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

// Package secretutil contains functions to manage secrets and strategies to manage secret cache.
package secretutil

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

const (
	// SecretFinalizer is the finalizer for secrets.
	SecretFinalizer = infrav1.HetznerClusterFinalizer + "/secret"
)

// SecretManager is a type for fetching Secrets whether or not they are in the
// client cache, labelling so that they will be included in the client cache,
// and optionally setting an owner reference.
type SecretManager struct {
	log       logr.Logger
	client    client.Client
	apiReader client.Reader
}

// NewSecretManager returns a new SecretManager.
func NewSecretManager(log logr.Logger, cacheClient client.Client, apiReader client.Reader) *SecretManager {
	return &SecretManager{
		log:       log.WithName("secret_manager"),
		client:    cacheClient,
		apiReader: apiReader,
	}
}

// claimSecret ensures that the Secret has a label that will ensure it is
// present in the cache (and that we can watch for changes), and optionally
// that it has a particular owner reference.
func (sm *SecretManager) claimSecret(ctx context.Context, secret *corev1.Secret, owner client.Object, ownerIsController, addFinalizer bool) error {
	needsUpdate := false
	if !metav1.HasLabel(secret.ObjectMeta, LabelEnvironmentName) {
		metav1.SetMetaDataLabel(&secret.ObjectMeta, LabelEnvironmentName, LabelEnvironmentValue)
		needsUpdate = true
	}
	if owner != nil {
		if ownerIsController {
			if !metav1.IsControlledBy(secret, owner) {
				if err := controllerutil.SetControllerReference(owner, secret, sm.client.Scheme()); err != nil {
					return fmt.Errorf("failed to set secret controller reference: %w", err)
				}
				needsUpdate = true
			}
		} else {
			alreadyOwned := false
			ownerUID := owner.GetUID()
			for _, ref := range secret.GetOwnerReferences() {
				if ref.UID == ownerUID {
					alreadyOwned = true
					break
				}
			}
			if !alreadyOwned {
				if err := controllerutil.SetOwnerReference(owner, secret, sm.client.Scheme()); err != nil {
					return fmt.Errorf("failed to set secret owner reference: %w", err)
				}
				needsUpdate = true
			}
		}
	}

	if addFinalizer && !utils.StringInList(secret.Finalizers, SecretFinalizer) {
		secret.Finalizers = append(secret.Finalizers, SecretFinalizer)
		needsUpdate = true
	}

	if needsUpdate {
		if err := sm.client.Update(ctx, secret); err != nil {
			return fmt.Errorf("failed to update secret %s in namespace %s: %w", secret.ObjectMeta.Name, secret.ObjectMeta.Namespace, err)
		}
	}

	return nil
}

// findSecret retrieves a Secret from the cache if it is available, and from the
// k8s API if not.
func (sm *SecretManager) findSecret(ctx context.Context, key types.NamespacedName) (secret *corev1.Secret, err error) {
	secret = &corev1.Secret{}

	// Look for secret in the filtered cache
	err = sm.client.Get(ctx, key, secret)
	if err == nil {
		return secret, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}

	// Secret not in cache; check API directly for unlabelled Secret
	err = sm.apiReader.Get(ctx, key, secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

// ObtainSecret retrieves a Secret and ensures that it has a label that will
// ensure it is present in the cache (and that we can watch for changes).
func (sm *SecretManager) ObtainSecret(ctx context.Context, key types.NamespacedName) (*corev1.Secret, error) {
	secret, err := sm.findSecret(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch secret %s in namespace %s: %w", key.Name, key.Namespace, err)
	}
	err = sm.claimSecret(ctx, secret, nil, false, false)

	return secret, err
}

// AcquireSecret retrieves a Secret and ensures that it has a label that will
// ensure it is present in the cache (and that we can watch for changes), and
// that it has a particular owner reference. The owner reference may optionally
// be a controller reference.
func (sm *SecretManager) AcquireSecret(ctx context.Context, key types.NamespacedName, owner client.Object, ownerIsController, addFinalizer bool) (*corev1.Secret, error) {
	if owner == nil {
		panic("AcquireSecret called with no owner")
	}

	secret, err := sm.findSecret(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to find secret: %w", err)
	}

	err = sm.claimSecret(ctx, secret, owner, ownerIsController, addFinalizer)

	return secret, err
}

// ReleaseSecret removes secrets manager finalizer from specified secret when needed.
func (sm *SecretManager) ReleaseSecret(ctx context.Context, secret *corev1.Secret, owner client.Object) error {
	apiVersion, kind := owner.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	newOwnerRefs := utils.RemoveOwnerRefFromList(secret.OwnerReferences, owner.GetName(), kind, apiVersion)

	// return if nothing changed
	if len(secret.OwnerReferences) == len(newOwnerRefs) && !utils.StringInList(secret.Finalizers, SecretFinalizer) {
		return nil
	}

	// check whether there are other HetznerCluster objects owning the secret
	foundOtherHetznerClusterOwner := false
	for _, ownerRef := range newOwnerRefs {
		if ownerRef.Kind == "HetznerCluster" {
			foundOtherHetznerClusterOwner = true
			break
		}
	}

	// remove finalizer from secret to allow deletion if no other owner exists
	if !foundOtherHetznerClusterOwner {
		secret.Finalizers = utils.FilterStringFromList(
			secret.Finalizers, SecretFinalizer)
	}

	secret.OwnerReferences = newOwnerRefs
	if err := sm.client.Update(ctx, secret); err != nil {
		return fmt.Errorf("failed to remove finalizer from secret %s in namespace %s: %w",
			secret.ObjectMeta.Name, secret.ObjectMeta.Namespace, err)
	}

	return nil
}
