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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/syself/cluster-api-provider-hetzner/pkg/csr"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Validate Kubelet CSR", func() {
	var cr *x509.CertificateRequest
	var name string
	var addresses []corev1.NodeAddress
	BeforeEach(func() {

		name = "test2-control-plane-a0653-7w642"
		addresses = []corev1.NodeAddress{
			{
				Type:    corev1.NodeExternalIP,
				Address: "168.119.152.147",
			},
		}
		bytes, err := base64.StdEncoding.DecodeString("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0KTUlJQkNEQ0Jyd0lCQURCTk1SVXdFd1lEVlFRS0V3eHplWE4wWlcwNmJtOWtaWE14TkRBeUJnTlZCQU1USzNONQpjM1JsYlRwdWIyUmxPblJsYzNReUxXTnZiblJ5YjJ3dGNHeGhibVV0WVRBMk5UTXROM2MyTkRJd1dUQVRCZ2NxCmhrak9QUUlCQmdncWhrak9QUU1CQndOQ0FBVFpwcHp5bjQ2YU0xRE95L0xKMm4zK1hock1scmlteHZwV0E3dGwKcmdBRUtPekdqOUhzcWJqRnZ0eEFPdGNFQ2xCTDNWTUprRjhDMlpQaHlTV01xemRtb0FBd0NnWUlLb1pJemowRQpBd0lEU0FBd1JRSWhBTTJTTnhoU3dabEwwbnp4SE9JZmFDU2R1NFk5K1c4SlJVcWhQWFgxd0VyeEFpQWk0VE1PCmZvclIwTDVjV0xFYVYzVmE3aVVRYnFpSHJjK1lBZmZoSUZ6REhnPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUgUkVRVUVTVC0tLS0tCg==")
		Expect(err).To(BeNil())
		block, _ := pem.Decode(bytes)
		cr, err = x509.ParseCertificateRequest(block.Bytes)
		Expect(err).To(BeNil())
	})

	It("should not fail", func() {
		Expect(csr.ValidateKubeletCSR(cr, name, addresses)).To(Succeed())
	})
})
