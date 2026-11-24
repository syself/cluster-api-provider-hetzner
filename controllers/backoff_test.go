/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	backoffRequestA = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "a"}}
	backoffRequestB = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "b"}}
)

func newTestBackoff(t *testing.T, baseDelay time.Duration) workqueue.TypedRateLimiter[reconcile.Request] {
	t.Helper()
	limiter := NewReconcileBackoff(baseDelay)
	require.NotNil(t, limiter)
	return limiter
}

func failTimes(limiter workqueue.TypedRateLimiter[reconcile.Request], request reconcile.Request, n int) {
	for range n {
		limiter.When(request)
	}
}

func TestBackoffDoublesUpToTheMaximum(t *testing.T) {
	limiter := newTestBackoff(t, time.Second)

	want := []time.Duration{
		1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second,
		32 * time.Second, 64 * time.Second, 128 * time.Second, 256 * time.Second, 5 * time.Minute,
		5 * time.Minute,
	}
	got := make([]time.Duration, 0, len(want))
	for range want {
		got = append(got, limiter.When(backoffRequestA))
	}

	require.Equal(t, want, got)
	require.Equal(t, len(want), limiter.NumRequeues(backoffRequestA))
}

func TestBackoffStaysAtTheMaximumAfterManyFailures(t *testing.T) {
	limiter := newTestBackoff(t, time.Second)
	failTimes(limiter, backoffRequestA, 100)

	require.Equal(t, 5*time.Minute, limiter.When(backoffRequestA))
}

func TestBackoffWithBaseDelayAtTheMaximumIsConstant(t *testing.T) {
	limiter := newTestBackoff(t, MaxBackoffDelay)

	for range 3 {
		require.Equal(t, 5*time.Minute, limiter.When(backoffRequestA))
	}
}

func TestBackoffStartsAtACustomBaseDelay(t *testing.T) {
	limiter := newTestBackoff(t, 250*time.Millisecond)

	want := []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, time.Second}
	got := make([]time.Duration, 0, len(want))
	for range want {
		got = append(got, limiter.When(backoffRequestA))
	}

	require.Equal(t, want, got)
}

func TestBackoffForgetResetsTheDelay(t *testing.T) {
	limiter := newTestBackoff(t, time.Second)
	failTimes(limiter, backoffRequestA, 3)

	limiter.Forget(backoffRequestA)

	require.Equal(t, time.Second, limiter.When(backoffRequestA))
	require.Equal(t, 1, limiter.NumRequeues(backoffRequestA))
}

func TestBackoffObjectsHaveTheirOwnCounters(t *testing.T) {
	limiter := newTestBackoff(t, time.Second)
	failTimes(limiter, backoffRequestA, 3)

	require.Equal(t, time.Second, limiter.When(backoffRequestB))
}

func TestBackoffForgetOneObjectKeepsTheOther(t *testing.T) {
	limiter := newTestBackoff(t, time.Second)
	failTimes(limiter, backoffRequestA, 3)
	failTimes(limiter, backoffRequestB, 3)

	limiter.Forget(backoffRequestA)

	require.Equal(t, time.Second, limiter.When(backoffRequestA))
	require.Equal(t, 8*time.Second, limiter.When(backoffRequestB))
}

func TestBackoffInstancesDoNotShareACounter(t *testing.T) {
	first := newTestBackoff(t, time.Second)
	second := newTestBackoff(t, time.Second)
	failTimes(first, backoffRequestA, 3)

	require.Equal(t, time.Second, second.When(backoffRequestA))
}

func TestBackoffForgetInOneInstanceKeepsTheOther(t *testing.T) {
	first := newTestBackoff(t, time.Second)
	second := newTestBackoff(t, time.Second)
	failTimes(first, backoffRequestA, 3)
	failTimes(second, backoffRequestA, 3)

	first.Forget(backoffRequestA)

	require.Equal(t, 8*time.Second, second.When(backoffRequestA))
}

func TestValidateBackoffBaseDelayRejectsInvalidValues(t *testing.T) {
	testCases := []struct {
		name             string
		value            time.Duration
		wantDurationText string
	}{
		{name: "zero", value: 0, wantDurationText: "0s"},
		{name: "one nanosecond below zero", value: -time.Nanosecond, wantDurationText: "-1ns"},
		{name: "one second below zero", value: -time.Second, wantDurationText: "-1s"},
		{name: "one nanosecond above the maximum", value: 5*time.Minute + time.Nanosecond, wantDurationText: "5m0.000000001s"},
		{name: "above the maximum", value: 6 * time.Minute, wantDurationText: "6m0s"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateBackoffBaseDelay(tc.value)

			require.EqualError(t, err, "must be greater than 0 and at most 5m0s, got "+tc.wantDurationText)
		})
	}
}

func TestValidateBackoffBaseDelayAcceptsValidValues(t *testing.T) {
	for _, value := range []time.Duration{time.Nanosecond, time.Millisecond, time.Second, 5 * time.Minute} {
		require.NoError(t, ValidateBackoffBaseDelay(value), value.String())
	}
}

func TestDefaultBackoffBaseDelayIsOneSecondAndValid(t *testing.T) {
	require.Equal(t, time.Second, DefaultBackoffBaseDelay)
	require.NoError(t, ValidateBackoffBaseDelay(DefaultBackoffBaseDelay))
}
