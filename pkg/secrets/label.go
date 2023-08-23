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

package secretutil

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// LabelEnvironmentName is used as key of the label for secrets that should be included in cache.
	LabelEnvironmentName = "caph.environment"
	// LabelEnvironmentValue is the value of the label for secrets that should be included in cache.
	LabelEnvironmentValue = "owned"
)

// AddSecretSelector adds a selector to a cache.SelectorsByObject that filters
// Secrets so that only those labelled as part of the environment get
// cached. The input may be nil.
func AddSecretSelector() map[client.Object]cache.ByObject {
	secret := &corev1.Secret{}
	byObject := cache.ByObject{
		Label: labels.SelectorFromSet(
			labels.Set{
				LabelEnvironmentName: LabelEnvironmentValue,
			}),
	}
	m := make(map[client.Object]cache.ByObject)
	m[secret] = byObject
	return m
}
