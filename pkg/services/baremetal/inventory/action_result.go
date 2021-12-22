package inventory

import (
	"math"
	"math/rand"
	"time"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// This is an upper limit for the ErrorCount, so that the max backoff
// timeout will not exceed (roughly) 8 hours
const maxBackOffCount = 9

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

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
	return
}

// actionDelayed it's the same of an actionContinue, but the requeue time
// is calculated using a fixed backoff with jitter
type actionDelayed struct {
	actionContinue
}

func (r actionDelayed) Result() (result reconcile.Result, err error) {
	result.RequeueAfter = calculateBackoff(1)
	result.Requeue = true
	return
}

// actionComplete is a result indicating that the current action has completed,
// and that the resource should transition to the next state.
type actionComplete struct {
}

func (r actionComplete) Result() (result reconcile.Result, err error) {
	result.Requeue = true
	return
}

// deleteComplete is a result indicating that the deletion action has
// completed, and that the resource has now been deleted.
type deleteComplete struct {
	actionComplete
}

func (r deleteComplete) Result() (result reconcile.Result, err error) {
	// Don't requeue, since the CR has been successfully deleted
	return
}

// actionError is a result indicating that an error occurred while attempting
// to advance the current action, and that reconciliation should be retried.
type actionError struct {
	err error
}

func (r actionError) Result() (result reconcile.Result, err error) {
	err = r.err
	return
}

// actionFailed is a result indicating that the current action has failed,
// and that the resource should be marked as in error.
type actionFailed struct {
	dirty      bool
	ErrorType  infrav1.ErrorType
	errorCount int
}

// Distribution sample for errorCount values:
// 1  [1m, 2m]
// 2  [2m, 4m]
// 3  [4m, 8m]
// 4  [8m, 16m]
// 5  [16m, 32m]
// 6  [32m, 1h4m]
// 7  [1h4m, 2h8m]
// 8  [2h8m, 4h16m]
// 9  [4h16m, 8h32m]
func calculateBackoff(errorCount int) time.Duration {

	if errorCount > maxBackOffCount {
		errorCount = maxBackOffCount
	}

	base := math.Exp2(float64(errorCount))
	backOff := base - (rand.Float64() * base * 0.5) // #nosec
	backOffDuration := time.Duration(float64(time.Minute) * backOff)
	return backOffDuration
}

func (r actionFailed) Result() (result reconcile.Result, err error) {
	result.RequeueAfter = calculateBackoff(r.errorCount)
	return
}
