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

package v1beta2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestRemediationStrategyEffectiveCooldown(t *testing.T) {
	tests := []struct {
		name     string
		strategy *RemediationStrategy
		want     time.Duration
	}{
		{
			name:     "nil strategy",
			strategy: nil,
			want:     DefaultRemediationCooldown,
		},
		{
			name:     "nil cooldown",
			strategy: &RemediationStrategy{},
			want:     DefaultRemediationCooldown,
		},
		{
			name:     "zero cooldown",
			strategy: &RemediationStrategy{CooldownSeconds: ptr.To(int32(0))},
			want:     0,
		},
		{
			name:     "configured cooldown",
			strategy: &RemediationStrategy{CooldownSeconds: ptr.To(int32(600))},
			want:     10 * time.Minute,
		},
		{
			name:     "negative cooldown clamped to default",
			strategy: &RemediationStrategy{CooldownSeconds: ptr.To(int32(-1))},
			want:     DefaultRemediationCooldown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.strategy.EffectiveCooldown())
		})
	}
}
