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

package hcloudclient

import (
	"errors"
	"fmt"
	"testing"
)

func TestWrapUnauthorized(t *testing.T) {
	unauthorizedErr := fmt.Errorf("hcloud: server responded (unauthorized) with status 401")
	otherErr := fmt.Errorf("hcloud: server responded with status 500")

	if got := wrapUnauthorized(nil); got != nil {
		t.Errorf("wrapUnauthorized(nil) = %v, want nil", got)
	}

	if got := wrapUnauthorized(unauthorizedErr); !errors.Is(got, ErrUnauthorized) {
		t.Errorf("wrapUnauthorized(%v) = %v, want errors.Is match with ErrUnauthorized", unauthorizedErr, got)
	}

	// A caller-added message (as in EnableRescueSystem, Reboot, GetAction) must still match.
	wrappedWithContext := fmt.Errorf("EnableRescue failed for %d: %w", 42, unauthorizedErr)
	if got := wrapUnauthorized(wrappedWithContext); !errors.Is(got, ErrUnauthorized) {
		t.Errorf("wrapUnauthorized(%v) = %v, want errors.Is match with ErrUnauthorized", wrappedWithContext, got)
	}

	if got := wrapUnauthorized(otherErr); !errors.Is(got, otherErr) || errors.Is(got, ErrUnauthorized) {
		t.Errorf("wrapUnauthorized(%v) = %v, want err unchanged and no ErrUnauthorized match", otherErr, got)
	}
}
