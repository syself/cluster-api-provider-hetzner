package csr_test

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"

	corev1 "k8s.io/api/core/v1"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	p "github.com/syself/cluster-api-provider-hetzner/pkg/csr"
)

func newCSR() *x509.CertificateRequest {
	bytes, _ := base64.StdEncoding.DecodeString("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0KTUlJQlZUQ0IvQUlCQURCUE1SVXdFd1lEVlFRS0V3eHplWE4wWlcwNmJtOWtaWE14TmpBMEJnTlZCQU1UTFhONQpjM1JsYlRwdWIyUmxPbU5vY21semRHbGhiaTFrWlhZdFkyOXVkSEp2YkMxd2JHRnVaUzE2Tld4b2FEQlpNQk1HCkJ5cUdTTTQ5QWdFR0NDcUdTTTQ5QXdFSEEwSUFCQTVtb2xES0NoSElRZ2h2VEhmVk1JZWtITXlGVmU4MVIyb3IKbmh5cFE2Y2xUa3c1a0VBOGZCVlUzcHZXRlM4cG5JaG9nakZkQW9DV1lwT2FFUW50dzBDZ1N6QkpCZ2txaGtpRwo5dzBCQ1E0eFBEQTZNRGdHQTFVZEVRUXhNQytDSVdOb2NtbHpkR2xoYmkxa1pYWXRZMjl1ZEhKdmJDMXdiR0Z1ClpTMTZOV3hvYUljRUNnQUFBb2NFWG9MaW9EQUtCZ2dxaGtqT1BRUURBZ05JQURCRkFpRUFvUzhFeHJqNzhyRGIKNnQxWTUrc1BaaFFiQ09QeFpjLzVRZXp3SlNXZnpGZ0NJRy9rRGZ6VHp4ZEgvb1oxdEtFSHBvdTg0d21rZGFPOAoxbnVkWVRIb21SdTMKLS0tLS1FTkQgQ0VSVElGSUNBVEUgUkVRVUVTVC0tLS0tCg==")
	block, _ := pem.Decode(bytes)

	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		panic(err)
	}
	return csr
}

func newMachine() *infrav1.HCloudMachine {
	m := &infrav1.HCloudMachine{}
	m.Name = "christian-dev-control-plane-z5lhh"
	m.Status.Addresses = []corev1.NodeAddress{
		{
			Type:    corev1.NodeInternalIP,
			Address: "10.0.0.2",
		},
		{
			Type:    corev1.NodeExternalIP,
			Address: "94.130.226.160",
		},
	}
	return m
}

func TestValidateKubeletCSR(t *testing.T) {
	err := p.ValidateKubeletCSR(newCSR(), newMachine())
	if err != nil {
		t.Errorf("unexpected error: %q", err)
	}
}
