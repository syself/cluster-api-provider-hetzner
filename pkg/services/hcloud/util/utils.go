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

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
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

// ServerIDFromProviderID returns the serverID from a providerID. This is used for hcloud machines
// only. The format must be "hcloud://NNN".
func ServerIDFromProviderID(providerID *string) (int64, error) {
	if providerID == nil {
		return 0, ErrNilProviderID
	}

	stringParts := strings.Split(*providerID, "://")
	if len(stringParts) != 2 || stringParts[0] == "" || stringParts[1] == "" {
		return 0, ErrInvalidProviderID
	}

	// Check that HCloud ProviderID starts with "hcloud"
	if stringParts[0] != "hcloud" {
		return 0, ErrInvalidProviderID
	}

	idString := stringParts[1]
	id, err := strconv.ParseInt(idString, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to convert serverID to int - %w: %w", ErrInvalidProviderID, err)
	}

	return id, nil
}

// conditionsObject is an API object that owns both the conditions and the deprecated v1beta1
// conditions.
type conditionsObject interface {
	conditions.Setter
	deprecatedv1beta1conditions.Setter
}

// HandleRateLimitExceeded sets the rate-limit conditions if err is an HCloud rate-limit error, and
// reports whether it was.
func HandleRateLimitExceeded(obj conditionsObject, recorder record.EventRecorder, err error, functionName string) bool {
	if !hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
		return false
	}

	msg := fmt.Sprintf("exceeded hcloud rate limit with calling function %q", functionName)

	deprecatedv1beta1conditions.MarkFalse(
		obj,
		infrav1.HetznerAPIReachableV1Beta1Condition,
		infrav1.RateLimitExceededV1Beta1Reason,
		clusterv1.ConditionSeverityWarning,
		"%s",
		msg,
	)
	conditions.Set(obj, metav1.Condition{
		Type:    infrav1.HCloudRateLimitExceededCondition,
		Status:  metav1.ConditionTrue,
		Reason:  infrav1.HCloudRateLimitExceededReason,
		Message: msg,
	})

	recorder.Event(
		obj,
		corev1.EventTypeWarning,
		"RateLimitExceeded",
		msg,
	)
	return true
}
