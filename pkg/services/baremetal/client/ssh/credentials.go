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

package sshclient

import (
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client"
	corev1 "k8s.io/api/core/v1"
)

// Credentials defines the credentials for SSH calls specified in a secret.
type Credentials struct {
	Name       string
	PublicKey  string
	PrivateKey string
}

// Validate returns an error if the ssh credentials are invalid.
func (creds Credentials) Validate() error {
	if creds.Name == "" {
		return &client.CredentialsValidationError{Message: "Missing ssh name in SSH credentials"}
	}
	if creds.PublicKey == "" {
		return &client.CredentialsValidationError{Message: "Missing public key in SSH credentials"}
	}
	if creds.PrivateKey == "" {
		return &client.CredentialsValidationError{Message: "Missing private key in SSH credentials"}
	}

	return nil
}

// CredentialsFromSecret generates the credentials object from a secret and a secretRef.
func CredentialsFromSecret(secret *corev1.Secret, secretRef infrav1.SSHSecretRef) Credentials {
	return Credentials{
		Name:       string(secret.Data[secretRef.Key.Name]),
		PublicKey:  string(secret.Data[secretRef.Key.PublicKey]),
		PrivateKey: string(secret.Data[secretRef.Key.PrivateKey]),
	}
}
