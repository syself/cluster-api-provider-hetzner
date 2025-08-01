package errors

// The following types and constants are imported from CAPI and will be removed at some point once we
// implement the conditions that will be required in CAPI v1beta2
// See https://github.com/kubernetes-sigs/cluster-api/issues/10784
// See also proposal https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md

// DeprecatedCAPIMachineStatusError defines errors states for Machine objects.
type DeprecatedCAPIMachineStatusError string

const (
	// DeprecatedCAPIUpdateMachineError indicates an error while trying to update a Node that this
	// Machine represents. This may indicate a transient problem that will be
	// fixed automatically with time, such as a service outage,
	//
	// Example: error updating load balancers.
	DeprecatedCAPIUpdateMachineError DeprecatedCAPIMachineStatusError = "UpdateError"

	// DeprecatedCAPICreateMachineError indicates an error while trying to create a Node to match this
	// Machine. This may indicate a transient problem that will be fixed
	// automatically with time, such as a service outage, or a terminal
	// error during creation that doesn't match a more specific
	// MachineStatusError value.
	//
	// Example: timeout trying to connect to GCE.
	DeprecatedCAPICreateMachineError DeprecatedCAPIMachineStatusError = "CreateError"
)
