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
	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// SecretFinalizer is the finalizer for secrets.
	SecretFinalizer = infrav1.ClusterFinalizer + "/secret"
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
	log := sm.log.WithValues("secret", secret.Name, "secretNamespace", secret.Namespace)
	needsUpdate := false
	if !metav1.HasLabel(secret.ObjectMeta, LabelEnvironmentName) {
		log.Info("setting secret environment label")
		metav1.SetMetaDataLabel(&secret.ObjectMeta, LabelEnvironmentName, LabelEnvironmentValue)
		needsUpdate = true
	}
	if owner != nil {
		ownerLog := log.WithValues(
			"ownerKind", owner.GetObjectKind().GroupVersionKind().Kind,
			"owner", owner.GetNamespace()+"/"+owner.GetName(),
			"ownerUID", owner.GetUID())
		if ownerIsController {
			if !metav1.IsControlledBy(secret, owner) {
				ownerLog.Info("setting secret controller reference")
				if err := controllerutil.SetControllerReference(owner, secret, sm.client.Scheme()); err != nil {
					return errors.Wrap(err, "failed to set secret controller reference")
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
				ownerLog.Info("setting secret owner reference")
				if err := controllerutil.SetOwnerReference(owner, secret, sm.client.Scheme()); err != nil {
					return errors.Wrap(err, "failed to set secret owner reference")
				}
				needsUpdate = true
			}
		}
	}

	if addFinalizer && !utils.StringInList(secret.Finalizers, SecretFinalizer) {
		log.Info("setting secret finalizer")
		secret.Finalizers = append(secret.Finalizers, SecretFinalizer)
		needsUpdate = true
	}

	if needsUpdate {
		if err := sm.client.Update(ctx, secret); err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to update secret %s in namespace %s", secret.ObjectMeta.Name, secret.ObjectMeta.Namespace))
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
		return nil, errors.Wrap(err, fmt.Sprintf("failed to fetch secret %s in namespace %s", key.Name, key.Namespace))
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
		return nil, errors.Wrap(err, "failed to find secret")
	}

	err = sm.claimSecret(ctx, secret, owner, ownerIsController, addFinalizer)

	return secret, err
}

// ReleaseSecret removes secrets manager finalizer from specified secret when needed.
func (sm *SecretManager) ReleaseSecret(ctx context.Context, secret *corev1.Secret) error {
	if !utils.StringInList(secret.Finalizers, SecretFinalizer) {
		return nil
	}

	// Remove finalizer from secret to allow deletion
	secret.Finalizers = utils.FilterStringFromList(
		secret.Finalizers, SecretFinalizer)

	if err := sm.client.Update(ctx, secret); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to remove finalizer from secret %s in namespace %s",
			secret.ObjectMeta.Name, secret.ObjectMeta.Namespace))
	}

	sm.log.Info("removed secret finalizer",
		"remaining", secret.Finalizers)

	return nil
}
