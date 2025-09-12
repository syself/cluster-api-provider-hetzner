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

// Package errors provides types and constants are imported from CAPI and will be removed at some point once we
// implement the conditions that will be required in CAPI v1beta2
// See https://github.com/kubernetes-sigs/cluster-api/issues/10784
// See also proposal https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md
package errors

// MachineStatusError defines errors states for Machine objects.
type MachineStatusError string

const (
	// UpdateMachineError indicates an error while trying to update a Node that this
	// Machine represents. This may indicate a transient problem that will be
	// fixed automatically with time, such as a service outage,
	//
	// Example: error updating load balancers.
	UpdateMachineError MachineStatusError = "UpdateError"

	// CreateMachineError indicates an error while trying to create a Node to match this
	// Machine. This may indicate a transient problem that will be fixed
	// automatically with time, such as a service outage, or a terminal
	// error during creation that doesn't match a more specific
	// MachineStatusError value.
	//
	// Example: timeout trying to connect to GCE.
	CreateMachineError MachineStatusError = "CreateError"
)
