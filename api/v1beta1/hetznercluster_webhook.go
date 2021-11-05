/*
Copyright 2021.

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

package v1beta1

import (
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var hetznerclusterlog = logf.Log.WithName("hetznercluster-resource")

var regionsEUCentral = []string{
	"fsn1",
	"nbg1",
	"hel1",
}

// var regionsUSEast = []string{
// 	"ash",
// }

// SetupWebhookWithManager initializes webhook manager for HetznerCluster.
func (r *HetznerCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// Could go in own webhook file hetznerclusterlist_webhook

// SetupWebhookWithManager initializes webhook manager for HetznerClusterList.
func (r *HetznerClusterList) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hetznercluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerclusters,verbs=create;update,versions=v1beta1,name=mutation.hetznercluster.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &HetznerCluster{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *HetznerCluster) Default() {}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hetznercluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerclusters,verbs=create;update,versions=v1beta1,name=validation.hetznercluster.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &HetznerCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerCluster) ValidateCreate() error {
	// Check whether regions are all in same network zone
	return areRegionsInSameNetworkZone(r.Spec.ControlPlaneRegion)
}

func areRegionsInSameNetworkZone(regions []HCloudRegion) error {
	if len(regions) == 0 {
		return nil
	}

	var foundEUCentralRegion bool
	var foundUSEastRegion bool

	for _, region := range regions {
		if isStringinArray(regionsEUCentral, string(region)) {
			foundEUCentralRegion = true
		} else if isStringinArray(regionsEUCentral, string(region)) {
			foundUSEastRegion = true
		}
	}

	if foundEUCentralRegion && foundUSEastRegion {
		return errors.New("regions are not in one network zone")
	}

	return nil
}

func isStringinArray(arr []string, str string) bool {
	for _, s := range arr {
		if s == str {
			return true
		}
	}
	return false
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerCluster) ValidateUpdate(old runtime.Object) error {
	hetznerclusterlog.Info("validate update", "name", r.Name)

	var allErrs field.ErrorList

	oldC, ok := old.(*HetznerCluster)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected an HetznerCluster but got a %T", old))
	}

	oldRegion := sets.NewString()
	for _, l := range oldC.Spec.ControlPlaneRegion {
		oldRegion.Insert(string(l))
	}
	newRegion := sets.NewString()
	for _, l := range r.Spec.ControlPlaneRegion {
		newRegion.Insert(string(l))
	}

	if !oldRegion.Equal(newRegion) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "region"), r.Spec.ControlPlaneRegion, "field is immutable"),
		)
	}

	return aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
	// TODO: LoadBalancer
	// TODO: Network
	// TODO: TokenRef already set
	// TODO: validate SSH Key
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *HetznerCluster) ValidateDelete() error {
	return nil
}
