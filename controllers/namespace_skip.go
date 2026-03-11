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

package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get

func shouldSkipReconciliationForNamespace(ctx context.Context, c client.Reader, namespace string) (bool, error) {
	if namespace == "" {
		return false, nil
	}

	ns := &corev1.Namespace{}
	if err := c.Get(ctx, client.ObjectKey{Name: namespace}, ns); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to fetch namespace %q: %w", namespace, err)
	}

	return ns.GetAnnotations()[infrav1.SkipNamespaceAnnotation] == "true", nil
}
