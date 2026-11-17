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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	"github.com/syself/cluster-api-provider-hetzner/pkg/csr"
)

var _ = Describe("Validate Kubelet CSR", func() {
	var cr *x509.CertificateRequest
	var name string
	var addresses []clusterv1.MachineAddress
	BeforeEach(func() {
		name = "hcloud-testing-control-plane-vgnlc"
		addresses = []clusterv1.MachineAddress{
			{
				Type:    clusterv1.MachineExternalIP,
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
		Expect(csr.ValidateKubeletCSR(cr, name, addresses)).To(Succeed())
	})

	It("should report every validation error, not only the last one", func() {
		// Build a CSR that violates several rules at once: two disallowed IP
		// addresses and a disallowed DNS name. The returned error has to mention
		// all of them so an operator can see every reason a kubelet CSR was denied.
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		Expect(err).To(BeNil())

		template := x509.CertificateRequest{
			Subject: pkix.Name{
				CommonName:   "system:node:" + name,
				Organization: []string{"system:nodes"},
			},
			DNSNames:    []string{"not-the-machine-name"},
			IPAddresses: []net.IP{net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.2")},
		}
		der, err := x509.CreateCertificateRequest(rand.Reader, &template, key)
		Expect(err).To(BeNil())

		request, err := x509.ParseCertificateRequest(der)
		Expect(err).To(BeNil())

		err = csr.ValidateKubeletCSR(request, name, addresses)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`the DNS name "not-the-machine-name" is not allowed`))
		Expect(err.Error()).To(ContainSubstring(`the IP address "10.0.0.1" is not allowed`))
		Expect(err.Error()).To(ContainSubstring(`the IP address "10.0.0.2" is not allowed`))
	})
})
