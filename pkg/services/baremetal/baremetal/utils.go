/*
Copyright 2021 The Kubernetes Authors.
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

package baremetal

import (
	"context"
	"strings"

	// comment for go-lint

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// metal3SecretType defines the type of secret created by metal3
	metal3SecretType corev1.SecretType = "infrastructure.cluster.x-k8s.io/secret"
)

// Filter filters a list for a string.
func Filter(list []string, strToFilter string) (newList []string) {
	for _, item := range list {
		if item != strToFilter {
			newList = append(newList, item)
		}
	}
	return
}

// Contains returns true if a list contains a string.
func Contains(list []string, strToSearch string) bool {
	for _, item := range list {
		if item == strToSearch {
			return true
		}
	}
	return false
}

// NotFoundError represents that an object was not found
type NotFoundError struct {
}

// Error implements the error interface
func (e *NotFoundError) Error() string {
	return "Object not found"
}

func patchIfFound(ctx context.Context, helper *patch.Helper, host client.Object) error {
	err := helper.Patch(ctx, host)
	if err != nil {
		notFound := true
		if aggr, ok := err.(kerrors.Aggregate); ok {
			for _, kerr := range aggr.Errors() {
				if !apierrors.IsNotFound(kerr) {
					notFound = false
				}
				if apierrors.IsConflict(kerr) {
					return &RequeueAfterError{}
				}
			}
		} else {
			notFound = false
		}
		if notFound {
			return nil
		}
	}
	return err
}

func updateObject(cl client.Client, ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	err := cl.Update(ctx, obj.DeepCopyObject().(client.Object), opts...)
	if apierrors.IsConflict(err) {
		return &RequeueAfterError{}
	}
	return err
}

func createObject(cl client.Client, ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	err := cl.Create(ctx, obj.DeepCopyObject().(client.Object), opts...)
	if apierrors.IsAlreadyExists(err) {
		return &RequeueAfterError{}
	}
	return err
}

func deleteObject(cl client.Client, ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	err := cl.Delete(ctx, obj.DeepCopyObject().(client.Object), opts...)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func createSecret(cl client.Client, ctx context.Context, name string,
	namespace string, clusterName string,
	ownerRefs []metav1.OwnerReference, content map[string][]byte,
) error {
	bootstrapSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				capi.ClusterLabelName: clusterName,
			},
			OwnerReferences: ownerRefs,
		},
		Data: content,
		Type: metal3SecretType,
	}

	secret, err := checkSecretExists(cl, ctx, name, namespace)
	if err == nil {
		// Update the secret with user data
		secret.ObjectMeta.Labels = bootstrapSecret.ObjectMeta.Labels
		secret.ObjectMeta.OwnerReferences = bootstrapSecret.ObjectMeta.OwnerReferences
		bootstrapSecret.ObjectMeta = secret.ObjectMeta
		return updateObject(cl, ctx, bootstrapSecret)
	} else if apierrors.IsNotFound(err) {
		// Create the secret with user data
		return createObject(cl, ctx, bootstrapSecret)
	}
	return err
}

func checkSecretExists(cl client.Client, ctx context.Context, name string,
	namespace string,
) (corev1.Secret, error) {
	tmpBootstrapSecret := corev1.Secret{}
	key := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}
	err := cl.Get(ctx, key, &tmpBootstrapSecret)
	return tmpBootstrapSecret, err
}

func deleteSecret(cl client.Client, ctx context.Context, name string,
	namespace string,
) error {
	tmpBootstrapSecret, err := checkSecretExists(cl, ctx, name, namespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if err == nil {
		//unset the finalizers (remove all since we do not expect anything else
		// to control that object)
		tmpBootstrapSecret.Finalizers = []string{}
		err = updateObject(cl, ctx, &tmpBootstrapSecret)
		if err != nil {
			return err
		}
		// Delete the secret with metadata
		err = cl.Delete(ctx, &tmpBootstrapSecret)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func parseProviderID(providerID string) string {
	return strings.TrimPrefix(providerID, ProviderIDPrefix)
}
