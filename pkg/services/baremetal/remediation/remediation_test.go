/*
Copyright 2023 The Kubernetes Authors.

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

package remediation

import (
	"encoding/json"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
)

func TestBMRemediation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "BMRemediation Suite")
}

var _ = Describe("Test TimeUntilNextRemediation", func() {
	type testCaseTimeUntilNextRemediation struct {
		lastRemediated                 time.Time
		expectTimeUntilLastRemediation time.Duration
	}

	now := time.Now()
	nullTime := time.Time{}

	DescribeTable("Test TimeUntilNextRemediation",
		func(tc testCaseTimeUntilNextRemediation) {
			var bmRemediation infrav1.HetznerBareMetalRemediation

			bmRemediation.Spec.Strategy = &infrav1.RemediationStrategy{Timeout: &metav1.Duration{Duration: time.Minute}}

			if tc.lastRemediated != nullTime {
				bmRemediation.Status.LastRemediated = &metav1.Time{Time: tc.lastRemediated}
			}

			service := Service{scope: &scope.BareMetalRemediationScope{
				BareMetalRemediation: &bmRemediation,
			}}

			timeUntilNextRemediation := service.timeUntilNextRemediation(now)

			Expect(timeUntilNextRemediation).To(Equal(tc.expectTimeUntilLastRemediation))
		},
		Entry("first remediation", testCaseTimeUntilNextRemediation{
			lastRemediated:                 nullTime,
			expectTimeUntilLastRemediation: time.Minute,
		}),
		Entry("remediation timed out", testCaseTimeUntilNextRemediation{
			lastRemediated:                 now.Add(-2 * time.Minute),
			expectTimeUntilLastRemediation: time.Duration(0),
		}),
		Entry("remediation not timed out", testCaseTimeUntilNextRemediation{
			lastRemediated:                 now.Add(-30 * time.Second),
			expectTimeUntilLastRemediation: 31 * time.Second,
		}),
	)
})

var _ = Describe("Test AddRebootAnnotation", func() {
	type testCaseAddRebootAnnotation struct {
		annotations       map[string]string
		expectAnnotations map[string]string
	}

	rebootAnnotationArguments := infrav1.RebootAnnotationArguments{Type: infrav1.RebootTypeHardware}

	b, err := json.Marshal(rebootAnnotationArguments)
	Expect(err).To(BeNil())

	rebootAnnotationString := string(b)

	DescribeTable("Test AddRebootAnnotation",
		func(tc testCaseAddRebootAnnotation) {
			annotations, err := addRebootAnnotation(tc.annotations)

			Expect(annotations).To(Equal(tc.expectAnnotations))
			Expect(err).To(BeNil())
		},
		Entry("nil annotations", testCaseAddRebootAnnotation{
			annotations:       nil,
			expectAnnotations: map[string]string{infrav1.RebootAnnotation: rebootAnnotationString},
		}),
		Entry("existing annotations", testCaseAddRebootAnnotation{
			annotations:       map[string]string{"key": "value"},
			expectAnnotations: map[string]string{"key": "value", infrav1.RebootAnnotation: rebootAnnotationString},
		}),
		Entry("reboot annotation already present", testCaseAddRebootAnnotation{
			annotations:       map[string]string{"key": "value", infrav1.RebootAnnotation: rebootAnnotationString},
			expectAnnotations: map[string]string{"key": "value", infrav1.RebootAnnotation: rebootAnnotationString},
		}),
	)
})
