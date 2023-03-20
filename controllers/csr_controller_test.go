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

package controllers

import (
	"testing"

	certificatesv1 "k8s.io/api/certificates/v1"
)

func Test_isCSRFromNode(t *testing.T) {
	testIsCSRFromNode := []struct {
		name        string
		csrUserName string
		expectBool  bool
	}{
		{
			name:        "csr from node",
			csrUserName: "system:node:testnode",
			expectBool:  true,
		},
		{
			name:        "csr not from node",
			csrUserName: "system:object:otherobject",
			expectBool:  false,
		},
	}

	for _, tt := range testIsCSRFromNode {
		gotBool := isCSRFromNode(&certificatesv1.CertificateSigningRequest{
			Spec: certificatesv1.CertificateSigningRequestSpec{Username: tt.csrUserName},
		})
		if gotBool != tt.expectBool {
			t.Fatalf("Testcase %q: got %v, want %v", tt.name, gotBool, tt.expectBool)
		}
	}
}

func Test_hcloudMachineNameFromCSR(t *testing.T) {
	testHCloudMachineNameFromCSR := []struct {
		name                    string
		csrUserName             string
		expectHCloudMachineName string
	}{
		{
			name:                    "first hcloud machine name",
			csrUserName:             "system:node:testnode",
			expectHCloudMachineName: "testnode",
		},
		{
			name:                    "second hcloud machine name",
			csrUserName:             "system:node:otherobject",
			expectHCloudMachineName: "otherobject",
		},
	}

	for _, tt := range testHCloudMachineNameFromCSR {
		gotName := hcloudMachineNameFromCSR(&certificatesv1.CertificateSigningRequest{
			Spec: certificatesv1.CertificateSigningRequestSpec{Username: tt.csrUserName},
		})
		if gotName != tt.expectHCloudMachineName {
			t.Fatalf("Testcase %q: got %v, want %v", tt.name, gotName, tt.expectHCloudMachineName)
		}
	}
}

func Test_bmMachineNameFromCSR(t *testing.T) {
	testBMMachineNameFromCSR := []struct {
		name                string
		csrUserName         string
		expectBMMachineName string
	}{
		{
			name:                "first bm machine name",
			csrUserName:         "system:node:bm-testnode",
			expectBMMachineName: "testnode",
		},
		{
			name:                "second bm machine name",
			csrUserName:         "system:node:bm-otherobject",
			expectBMMachineName: "otherobject",
		},
	}

	for _, tt := range testBMMachineNameFromCSR {
		gotName := bmMachineNameFromCSR(&certificatesv1.CertificateSigningRequest{
			Spec: certificatesv1.CertificateSigningRequestSpec{Username: tt.csrUserName},
		})
		if gotName != tt.expectBMMachineName {
			t.Fatalf("Testcase %q: got %v, want %v", tt.name, gotName, tt.expectBMMachineName)
		}
	}
}

func Test_machineNameFromCSR(t *testing.T) {
	testMachineNameFromCSR := []struct {
		name              string
		csrUserName       string
		isHCloudMachine   bool
		expectMachineName string
	}{
		{
			name:              "first hcloud machine name",
			csrUserName:       "system:node:testnode",
			isHCloudMachine:   true,
			expectMachineName: "testnode",
		},
		{
			name:              "first bm machine name",
			csrUserName:       "system:node:bm-testnode",
			isHCloudMachine:   false,
			expectMachineName: "testnode",
		},
		{
			name:              "second hcloud machine name",
			csrUserName:       "system:node:otherobject",
			isHCloudMachine:   true,
			expectMachineName: "otherobject",
		},
		{
			name:              "second bm machine name",
			csrUserName:       "system:node:bm-otherobject",
			isHCloudMachine:   false,
			expectMachineName: "otherobject",
		},
	}

	for _, tt := range testMachineNameFromCSR {
		gotName := machineNameFromCSR(&certificatesv1.CertificateSigningRequest{
			Spec: certificatesv1.CertificateSigningRequestSpec{Username: tt.csrUserName},
		}, tt.isHCloudMachine)
		if gotName != tt.expectMachineName {
			t.Fatalf("Testcase %q: got %v, want %v", tt.name, gotName, tt.expectMachineName)
		}
	}
}

func Test_getx509CSR(t *testing.T) {
	testGetx509CSR := []struct {
		name        string
		csrRequest  []byte
		expectError bool
	}{
		{
			name:        "invalid request",
			csrRequest:  []byte("invalid request"),
			expectError: true,
		},
		{
			name: "correct request",
			csrRequest: []byte(`-----BEGIN CERTIFICATE REQUEST-----
MIIBODCB3gIBADBDMRUwEwYDVQQKEwxzeXN0ZW06bm9kZXMxKjAoBgNVBAMTIXN5
c3RlbTpub2RlOmJtLXRlc3RpbmctbWQtMS1tbGx3aDBZMBMGByqGSM49AgEGCCqG
SM49AwEHA0IABCqjw5YPkFiK2AHSxmdYTXIDAwl6YrOwixAHwl6W3sqcjt+C9xqG
lNcGj4PxuTr+VtSDa15FJyTT4gttEbOYhiigOTA3BgkqhkiG9w0BCQ4xKjAoMCYG
A1UdEQQfMB2CFWJtLXRlc3RpbmctbWQtMS1tbGx3aIcEiPNFpzAKBggqhkjOPQQD
AgNJADBGAiEAjMlzuDr3YddabxkKF5Wm/xZgmAN8IbZMoqP7vvrl0mkCIQDrda1J
+F6glIbLmASRT9ar3jcVLLcHjaqZFy6quhCSsQ==
-----END CERTIFICATE REQUEST-----`),
			expectError: false,
		},
	}

	for _, tt := range testGetx509CSR {
		_, gotError := getx509CSR(&certificatesv1.CertificateSigningRequest{
			Spec: certificatesv1.CertificateSigningRequestSpec{Request: tt.csrRequest},
		})
		if (gotError != nil) != tt.expectError {
			t.Fatalf("Testcase %q: got %v (%v), want %v.", tt.name, gotError != nil, gotError, tt.expectError)
		}
	}
}
