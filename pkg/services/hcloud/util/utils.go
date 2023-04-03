/*
Copyright 2023 The Kubernetes Authors.

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

// Package hcloudutil contains utility functions for hcloud servers.
package hcloudutil

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hetznercloud/hcloud-go/hcloud"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/record"
)

const providerIDPrefix = "hcloud://"

var (
	// ErrInvalidProviderID indicates that the providerID is invalid.
	ErrInvalidProviderID = fmt.Errorf("invalid providerID")
	// ErrNilProviderID indicates that the providerID is nil.
	ErrNilProviderID = fmt.Errorf("nil providerID")
)

// ProviderIDFromServerID returns the providerID of a hcloud server from a serverID.
func ProviderIDFromServerID(serverID int) string {
	return fmt.Sprintf("%s%v", providerIDPrefix, serverID)
}

// ServerIDFromProviderID returns the serverID from a providerID.
func ServerIDFromProviderID(providerID *string) (int, error) {
	if providerID == nil {
		return 0, ErrNilProviderID
	}
	stringParts := strings.Split(*providerID, "://")
	if len(stringParts) != 2 || stringParts[0] == "" || stringParts[1] == "" {
		return 0, ErrInvalidProviderID
	}
	idString := stringParts[1]
	id, err := strconv.Atoi(idString)
	if err != nil {
		return 0, fmt.Errorf("failed to convert serverID to int - %w: %w", ErrInvalidProviderID, err)
	}

	return id, nil
}

type runtimeObjectWithConditions interface {
	conditions.Setter
	runtime.Object
}

// HandleRateLimitExceeded handles rate limit exceeded errors.
func HandleRateLimitExceeded(obj runtimeObjectWithConditions, err error, functionName string) {
	if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
		conditions.MarkTrue(obj, infrav1.RateLimitExceeded)
		record.Warnf(obj, "RateLimitExceeded", "exceeded rate limit with calling function %q", functionName)
	}
}
