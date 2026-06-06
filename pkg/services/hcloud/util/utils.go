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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	"sigs.k8s.io/cluster-api/util/record"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
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

// HandleRateLimitExceeded sets the rate-limit conditions on a v1beta2 resource if err is an HCloud
// rate-limit error, and reports whether it was. It is used by the controllers and services that
// reconcile v1beta2 resources. Controllers and services still on v1beta1 use
// HandleRateLimitExceededV1Beta1.
func HandleRateLimitExceeded(cluster *infrav2.HetznerCluster, err error, functionName string) bool {
	if !hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
		return false
	}

	msg := fmt.Sprintf("exceeded hcloud rate limit with calling function %q", functionName)

	deprecatedv1beta1conditions.MarkFalse(
		cluster,
		infrav2.HetznerAPIReachableV1Beta1Condition,
		infrav2.RateLimitExceededV1Beta1Reason,
		clusterv1.ConditionSeverityWarning,
		"%s",
		msg,
	)
	conditions.Set(cluster, metav1.Condition{
		Type:    infrav2.HCloudRateLimitExceededCondition,
		Status:  metav1.ConditionTrue,
		Reason:  infrav2.HCloudRateLimitExceededReason,
		Message: msg,
	})

	record.Warnf(cluster, "RateLimitExceeded", msg)
	return true
}

type runtimeObjectWithConditions interface {
	v1beta1conditions.Setter
	runtime.Object
}

// HandleRateLimitExceededV1Beta1 is the still-v1beta1 counterpart of HandleRateLimitExceeded, used by
// the resources that have not been switched to v1beta2 yet. It writes the deprecated v1beta1
// HetznerAPIReachable condition and, when the object supports the staged v1beta2 conditions, the
// v1beta2 HCloudRateLimitExceeded condition.
func HandleRateLimitExceededV1Beta1(obj runtimeObjectWithConditions, err error, functionName string) bool {
	if !hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
		return false
	}

	msg := fmt.Sprintf("exceeded hcloud rate limit with calling function %q", functionName)

	v1beta1conditions.MarkFalse(
		obj,
		infrav1.HetznerAPIReachableCondition,
		infrav1.RateLimitExceededReason,
		clusterv1beta1.ConditionSeverityWarning,
		"%s",
		msg,
	)
	if setter, ok := obj.(v1beta2conditions.Setter); ok {
		v1beta2conditions.Set(setter, metav1.Condition{
			Type:    infrav1.HCloudRateLimitExceededV1Beta2Condition,
			Status:  metav1.ConditionTrue,
			Reason:  infrav1.HCloudRateLimitExceededV1Beta2Reason,
			Message: msg,
		})
	}

	record.Warnf(obj, "RateLimitExceeded", msg)
	return true
}
