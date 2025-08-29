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

// Package sshkeygen implements functions for ssh key creation.
package sshkeygen

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ssh"
)

// GenerateEd25519 creates an Ed25519 SSH key pair.
// Returns: private key PEM (PKCS#8), public key (authorized_keys format), and error.
func GenerateEd25519() (string, string, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate ed25519: %w", err)
	}

	// Private key in PKCS#8 (widely supported)
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", "", fmt.Errorf("marshal pkcs8: %w", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})

	sshPubKey, err := ssh.NewPublicKey(pub)
	if err != nil {
		return "", "", fmt.Errorf("ssh public key: %w", err)
	}
	pubAuthorized := string(ssh.MarshalAuthorizedKey(sshPubKey))

	return string(privPEM), pubAuthorized, nil
}
