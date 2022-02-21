package sshclient

import "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client"

type Credentials struct {
	Name       string
	PublicKey  string
	PrivateKey string
}

// Validate returns an error if the ssh credentials are invalid
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
