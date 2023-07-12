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
	"sigs.k8s.io/controller-runtime/pkg/client"

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

var _ = Describe("Test ObjectKeyFromAnnotations", func() {
	type testCaseObjectKeyFromAnnotations struct {
		annotations map[string]string
		expectError bool
		expectKey   client.ObjectKey
	}

	DescribeTable("Test ObjectKeyFromAnnotations",
		func(tc testCaseObjectKeyFromAnnotations) {
			key, err := objectKeyFromAnnotations(tc.annotations)

			if tc.expectError {
				Expect(err).ToNot(BeNil())
			} else {
				Expect(err).To(BeNil())
			}

			Expect(key).To(Equal(tc.expectKey))
		},
		Entry("correct host key", testCaseObjectKeyFromAnnotations{
			annotations: map[string]string{
				infrav1.HostAnnotation: "mynamespace/myname",
			},
			expectError: false,
			expectKey:   client.ObjectKey{Namespace: "mynamespace", Name: "myname"},
		}),
		Entry("nil annotations", testCaseObjectKeyFromAnnotations{
			annotations: nil,
			expectError: true,
			expectKey:   client.ObjectKey{},
		}),
		Entry("no host annotation", testCaseObjectKeyFromAnnotations{
			annotations: map[string]string{
				"other-key": "mynamespace/myname",
			},
			expectError: true,
			expectKey:   client.ObjectKey{},
		}),
		Entry("incorrect host key", testCaseObjectKeyFromAnnotations{
			annotations: map[string]string{
				infrav1.HostAnnotation: "mynamespace-myname",
			},
			expectError: true,
			expectKey:   client.ObjectKey{},
		}),
	)
})

var _ = Describe("Test SplitHostKey", func() {
	type testCaseSplitHostKey struct {
		hostKey         string
		expectNamespace string
		expectName      string
		expectError     bool
	}

	DescribeTable("Test SplitHostKey",
		func(tc testCaseSplitHostKey) {
			ns, name, err := splitHostKey(tc.hostKey)

			Expect(ns).To(Equal(tc.expectNamespace))
			Expect(name).To(Equal(tc.expectName))

			if tc.expectError {
				Expect(err).ToNot(BeNil())
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry("correct host key", testCaseSplitHostKey{
			hostKey:         "ns/name",
			expectNamespace: "ns",
			expectName:      "name",
			expectError:     false,
		}),
		Entry("incorrect host key 1", testCaseSplitHostKey{
			hostKey:         "ns-name",
			expectNamespace: "",
			expectName:      "",
			expectError:     true,
		}),
		Entry("incorrect host key 2", testCaseSplitHostKey{
			hostKey:         "ns/name/other",
			expectNamespace: "",
			expectName:      "",
			expectError:     true,
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
