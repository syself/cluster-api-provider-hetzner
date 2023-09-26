/*
Copyright 2022 The Kubernetes Authors.

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

package host

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

// This is an upper limit for the ErrorCount, so that the max backoff
// timeout will not exceed (roughly) 8 hours.
const maxBackOffCount = 9

// actionResult is an interface that encapsulates the result of a Reconcile
// call, as returned by the action corresponding to the current state.
type actionResult interface {
	Result() (reconcile.Result, error)
}

// actionContinue is a result indicating that the current action is still
// in progress, and that the resource should remain in the same provisioning
// state.
type actionContinue struct {
	delay time.Duration
}

func (r actionContinue) Result() (result reconcile.Result, err error) {
	result.RequeueAfter = r.delay
	// Set Requeue true as well as RequeueAfter in case the delay is 0.
	result.Requeue = true
	return result, nil
}

// actionComplete is a result indicating that the current action has completed,
// and that the resource should transition to the next state.
type actionComplete struct{}

func (actionComplete) Result() (result reconcile.Result, err error) {
	result.Requeue = true
	return result, nil
}

// deleteComplete is a result indicating that the deletion action has
// completed, and that the resource has now been deleted.
type deleteComplete struct{}

func (deleteComplete) Result() (result reconcile.Result, err error) {
	// Don't requeue, since the CR has been successfully deleted
	return result, nil
}

// actionError is a result indicating that an error occurred while attempting
// to advance the current action, and that reconciliation should be retried.
type actionError struct {
	err error
}

func (r actionError) Result() (result reconcile.Result, err error) {
	return result, r.err
}

// actionStop is a result indicating that there is a permanent error and we have to stop reconcilement.
type actionStop struct{}

func (r actionStop) Result() (result reconcile.Result, err error) {
	return result, err
}

// actionFailed is a result indicating that the current action has failed,
// and that the resource should be marked as in error.
type actionFailed struct {
	ErrorType  infrav1.ErrorType
	errorCount int
}

// CalculateBackoff calculates the reconciliation backoff.
// Distribution sample for errorCount values:
// 1  [1m, 2m]
// 2  [2m, 4m]
// 3  [4m, 8m]
// 4  [8m, 16m]
// 5  [16m, 32m]
// 6  [32m, 1h4m]
// 7  [1h4m, 2h8m]
// 8  [2h8m, 4h16m]
// 9  [4h16m, 8h32m].
func CalculateBackoff(errorCount int) (time.Duration, error) {
	if errorCount > maxBackOffCount {
		errorCount = maxBackOffCount
	}

	base := math.Exp2(float64(errorCount))
	randInt, err := rand.Int(rand.Reader, big.NewInt(1<<53))
	if err != nil {
		return 0, fmt.Errorf("failed to create random number: %w", err)
	}
	randFloat := float64(randInt.Int64()) / (1 << 53)
	backOff := base - (randFloat * base * 0.5)
	backOffDuration := time.Duration(float64(time.Minute) * backOff)
	return backOffDuration, nil
}

func (r actionFailed) Result() (result reconcile.Result, err error) {
	result.RequeueAfter, err = CalculateBackoff(r.errorCount)
	return result, err
}
