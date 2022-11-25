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

package csr_test

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/syself/cluster-api-provider-hetzner/pkg/csr"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Validate Kubelet CSR", func() {
	var cr *x509.CertificateRequest
	var name string
	var addresses []corev1.NodeAddress
	BeforeEach(func() {

		name = "hcloud-testing-control-plane-vgnlc"
		addresses = []corev1.NodeAddress{
			{
				Type:    corev1.NodeExternalIP,
				Address: "195.201.236.66",
			},
		}
		bytes, err := base64.StdEncoding.DecodeString("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0KTUlJQlVUQ0IrQUlCQURCUU1SVXdFd1lEVlFRS0V3eHplWE4wWlcwNmJtOWtaWE14TnpBMUJnTlZCQU1UTG5ONQpjM1JsYlRwdWIyUmxPbWhqYkc5MVpDMTBaWE4wYVc1bkxXTnZiblJ5YjJ3dGNHeGhibVV0ZG1kdWJHTXdXVEFUCkJnY3Foa2pPUFFJQkJnZ3Foa2pPUFFNQkJ3TkNBQVJrMmdPTkhyZi9keUxnd05qZXk5MWRJVHY0ai9tRDJDNTkKNkpzcGJuK3dydlR2eFBIWVBWUmJEZEZweGJodU81WnM2U3lGUUtJcjd2d2hJTjlYeTVhTm9FWXdSQVlKS29aSQpodmNOQVFrT01UY3dOVEF6QmdOVkhSRUVMREFxZ2lKb1kyeHZkV1F0ZEdWemRHbHVaeTFqYjI1MGNtOXNMWEJzCllXNWxMWFpuYm14amh3VER5ZXhDTUFvR0NDcUdTTTQ5QkFNQ0EwZ0FNRVVDSUg1bE9QNWZLYmV0eTlFRDI0VVkKWTdGb1N1eVJDYjFpNi9CRW14SGEvYjdtQWlFQWdlbEs4S0oyckJROUVMK0JHai9WTVNsc3BHVUdTRXlkMUthMQpISXVpU3NJPQotLS0tLUVORCBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0K")
		Expect(err).To(BeNil())
		block, _ := pem.Decode(bytes)
		cr, err = x509.ParseCertificateRequest(block.Bytes)
		Expect(err).To(BeNil())
	})

	It("should not fail", func() {
		Expect(csr.ValidateKubeletCSR(cr, name, true, addresses)).To(Succeed())
	})
})
