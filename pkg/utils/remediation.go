/*
Copyright 2026 The Kubernetes Authors.

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

package utils

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
)

// MachineRecoveredAfterReboot reports whether a reboot fixed the Machine. The Node has to be
// healthy, and the MachineHealthCheck must no longer mark the Machine unhealthy. A health
// check on a custom Node condition keeps HealthCheckSucceeded False while the Node is Ready,
// so a Ready Node alone does not mean the reboot worked.
func MachineRecoveredAfterReboot(machine *clusterv1.Machine) bool {
	return conditions.IsTrue(machine, clusterv1.MachineNodeHealthyCondition) &&
		!conditions.IsFalse(machine, clusterv1.MachineHealthCheckSucceededCondition)
}
