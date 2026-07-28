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

package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LoadBalancerSpec target address family", func() {
	type expectation struct {
		family LoadBalancerTargetAddressFamily
		ipv4   bool
		ipv6   bool
	}

	DescribeTable("resolves the configured value",
		func(configured LoadBalancerTargetAddressFamily, expected expectation) {
			spec := LoadBalancerSpec{TargetAddressFamily: configured}

			Expect(spec.TargetAddressFamilyOrDefault()).To(Equal(expected.family))
			Expect(spec.WantsIPv4()).To(Equal(expected.ipv4))
			Expect(spec.WantsIPv6()).To(Equal(expected.ipv6))
		},
		Entry("an unset field defaults to ipv4",
			LoadBalancerTargetAddressFamily(""),
			expectation{family: LoadBalancerTargetAddressFamilyIPv4, ipv4: true, ipv6: false},
		),
		Entry("ipv4 selects the IPv4 address only",
			LoadBalancerTargetAddressFamilyIPv4,
			expectation{family: LoadBalancerTargetAddressFamilyIPv4, ipv4: true, ipv6: false},
		),
		Entry("ipv6 selects the IPv6 address only",
			LoadBalancerTargetAddressFamilyIPv6,
			expectation{family: LoadBalancerTargetAddressFamilyIPv6, ipv4: false, ipv6: true},
		),
		Entry("dualstack selects both addresses",
			LoadBalancerTargetAddressFamilyDualStack,
			expectation{family: LoadBalancerTargetAddressFamilyDualStack, ipv4: true, ipv6: true},
		),
		// The enum only guards writes through the API server. A value that reaches the
		// code from elsewhere, for instance from an object stored before the field
		// existed, has to resolve to the default rather than to nothing at all.
		Entry("an unknown value falls back to the default",
			LoadBalancerTargetAddressFamily("ipv5"),
			expectation{family: LoadBalancerTargetAddressFamilyIPv4, ipv4: true, ipv6: false},
		),
	)
})
