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
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

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

func (r actionContinue) Result() (reconcile.Result, error) {
	if r.delay != 0 {
		return reconcile.Result{RequeueAfter: r.delay}, nil
	}
	return reconcile.Result{RequeueAfter: time.Second}, nil
}

// actionComplete is a result indicating that the current action has completed, and that the
// resource should transition to the next state. If you want to not reconcile again (end of last
// phase), use actionFinished instead.
type actionComplete struct{}

func (actionComplete) Result() (reconcile.Result, error) {
	// One phase is done. Go to the next phase in the next reconcile.
	return reconcile.Result{RequeueAfter: time.Second}, nil
}

// actionFinished is a result indicating that the host has reached a stable
// steady state and no further reconciliation is needed until an external
// event (e.g. a watch) triggers a new reconcile.
type actionFinished struct{}

func (actionFinished) Result() (reconcile.Result, error) {
	return reconcile.Result{}, nil
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
