/*
Copyright 2025 The Kubernetes Authors.

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

// Package baremetalutils implements helper functions for working with baremetal.
package baremetalutils

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

func splitHostKey(key string) (namespace, name string) {
	parts := strings.Split(key, "/")
	if len(parts) != 2 {
		panic("unexpected host key")
	}
	return parts[0], parts[1]
}

// GetAssociatedHost gets the associated host by looking for an annotation on the
// machine that contains a reference to the host. Returns nil if not found. Assumes the host is in
// the same namespace as the machine.
func GetAssociatedHost(ctx context.Context, crClient client.Client, hbmm *infrav1.HetznerBareMetalMachine) (*infrav1.HetznerBareMetalHost, error) {
	annotations := hbmm.GetAnnotations()
	// if no annotations exist on machine, no host can be associated
	if annotations == nil {
		return nil, nil
	}

	// check if host annotation is set and return if not
	hostKey, ok := annotations[infrav1.HostAnnotation]
	if !ok {
		return nil, nil
	}

	// find associated host object and return it
	hostNamespace, hostName := splitHostKey(hostKey)

	host := &infrav1.HetznerBareMetalHost{}
	key := client.ObjectKey{
		Name:      hostName,
		Namespace: hostNamespace,
	}

	if err := crClient.Get(ctx, key, host); err != nil {
		return nil, fmt.Errorf("failed to get host object: %w", err)
	}
	return host, nil
}
