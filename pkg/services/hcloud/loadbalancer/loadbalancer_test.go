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

package loadbalancer

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

var _ = Describe("Loadbalancer", func() {
	Context("hcloud cluster has network attached", func() {
		var sts infrav1.LoadBalancerStatus
		BeforeEach(func() {
			var err error
			sts, err = apiToStatus(lb, true)
			Expect(err).To(Succeed())
		})

		It("should have two targets", func() {
			Expect(sts.Target).To(Equal(targets))
		})
		It("should have the right ip addresses", func() {
			Expect(sts.IPv4).To(Equal(ipv4))
			Expect(sts.IPv6).To(Equal(ipv6))
		})
		It("should have the right internal IP", func() {
			Expect(sts.InternalIP).To(Equal(internalIP))
		})
		It("should be unprotected", func() {
			Expect(sts.Protected).To(Equal(protected))
		})
	})
	Context("hcloud cluster has no network attached", func() {
		var sts infrav1.LoadBalancerStatus
		BeforeEach(func() {
			var err error
			sts, err = apiToStatus(lb, false)
			Expect(err).To(Succeed())
		})

		It("should have two targets", func() {
			Expect(sts.Target).To(Equal(targets))
		})
		It("should have the right ip addresses", func() {
			Expect(sts.IPv4).To(Equal(ipv4))
			Expect(sts.IPv6).To(Equal(ipv6))
		})
		It("should have no internal IP", func() {
			Expect(sts.InternalIP).To(Equal(""))
		})
		It("should be unprotected", func() {
			Expect(sts.Protected).To(Equal(protected))
		})
	})
})
