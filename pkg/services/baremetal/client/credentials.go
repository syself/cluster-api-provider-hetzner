package robotclient

import "fmt"

// Credentials holds the information for authenticating with the Hetzner robot api.
type Credentials struct {
	Username string
	Password string
}

// CredentialsValidationError is returned when the provided Hetzner robot api credentials
// are invalid (e.g. null)
type CredentialsValidationError struct {
	message string
}

func (e CredentialsValidationError) Error() string {
	return fmt.Sprintf("Validation error with Hetzner robot api credentials: %s",
		e.message)
}

// Validate returns an error if the credentials are invalid
func (creds Credentials) Validate() error {
	if creds.Username == "" {
		return &CredentialsValidationError{message: "Missing Hetzner robot api connection detail 'username' in credentials"}
	}
	if creds.Password == "" {
		return &CredentialsValidationError{message: "Missing Hetzner robot api connection detail 'password' in credentials"}
	}
	return nil
}
