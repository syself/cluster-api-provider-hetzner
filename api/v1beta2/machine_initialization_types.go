package v1beta2

// MachineInitializationStatus holds provisioning signals consumed by CAPI for Machines.
// Fields and json tags must follow the contract at
// https://cluster-api.sigs.k8s.io/developer/providers/contracts/infra-machine.html?highlight=Initialization#inframachine-initialization-completed
// (status.initialization.provisioned).
type MachineInitializationStatus struct {
	// Provisioned is true when the infrastructure provider reports the machine is fully provisioned.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`
}
