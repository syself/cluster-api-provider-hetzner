package robotclient

import "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client"

// Credentials holds the information for authenticating with the Hetzner robot api.
type Credentials struct {
	Username string
	Password string
}

// Validate returns an error if the credentials are invalid
func (creds Credentials) Validate() error {
	if creds.Username == "" {
		return &client.CredentialsValidationError{Message: "Missing Hetzner robot api connection detail 'username' in credentials"}
	}
	if creds.Password == "" {
		return &client.CredentialsValidationError{Message: "Missing Hetzner robot api connection detail 'password' in credentials"}
	}
	return nil
}
