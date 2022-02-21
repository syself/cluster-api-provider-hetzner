package client

import (
	"fmt"
)

// CredentialsValidationError is returned when the provided Hetzner robot api credentials
// are invalid (e.g. null)
type CredentialsValidationError struct {
	Message string
}

func (e CredentialsValidationError) Error() string {
	return fmt.Sprintf("Validation error with credentials: %s",
		e.Message)
}
