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

package v1beta1

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrav1beta2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
)

func (src *HetznerCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HetznerCluster)
	return convertUnstructured(src, dst, convertFailureDomainsMapToSlice)
}

func (dst *HetznerCluster) ConvertFrom(srcRaw conversion.Hub) error {
	if err := convertFromUnstructured(srcRaw, dst, convertFailureDomainsSliceToMap); err != nil {
		return fmt.Errorf("HetznerCluster: %w", err)
	}
	return nil
}

func (src *HetznerClusterTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HetznerClusterTemplate)
	return convertUnstructured(src, dst, nil)
}

func (dst *HetznerClusterTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	if err := convertFromUnstructured(srcRaw, dst, nil); err != nil {
		return fmt.Errorf("HetznerClusterTemplate: %w", err)
	}
	return nil
}

func (src *HCloudMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HCloudMachine)
	return convertUnstructured(src, dst, nil)
}

func (dst *HCloudMachine) ConvertFrom(srcRaw conversion.Hub) error {
	if err := convertFromUnstructured(srcRaw, dst, nil); err != nil {
		return fmt.Errorf("HCloudMachine: %w", err)
	}
	return nil
}

func (src *HCloudMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HCloudMachineTemplate)
	return convertUnstructured(src, dst, nil)
}

func (dst *HCloudMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	if err := convertFromUnstructured(srcRaw, dst, nil); err != nil {
		return fmt.Errorf("HCloudMachineTemplate: %w", err)
	}
	return nil
}

func (src *HCloudRemediation) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HCloudRemediation)
	return convertUnstructured(src, dst, nil)
}

func (dst *HCloudRemediation) ConvertFrom(srcRaw conversion.Hub) error {
	if err := convertFromUnstructured(srcRaw, dst, nil); err != nil {
		return fmt.Errorf("HCloudRemediation: %w", err)
	}
	return nil
}

func (src *HCloudRemediationTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HCloudRemediationTemplate)
	return convertUnstructured(src, dst, nil)
}

func (dst *HCloudRemediationTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	if err := convertFromUnstructured(srcRaw, dst, nil); err != nil {
		return fmt.Errorf("HCloudRemediationTemplate: %w", err)
	}
	return nil
}

func (src *HetznerBareMetalHost) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HetznerBareMetalHost)
	return convertUnstructured(src, dst, nil)
}

func (dst *HetznerBareMetalHost) ConvertFrom(srcRaw conversion.Hub) error {
	if err := convertFromUnstructured(srcRaw, dst, nil); err != nil {
		return fmt.Errorf("HetznerBareMetalHost: %w", err)
	}
	return nil
}

func (src *HetznerBareMetalMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HetznerBareMetalMachine)
	return convertUnstructured(src, dst, nil)
}

func (dst *HetznerBareMetalMachine) ConvertFrom(srcRaw conversion.Hub) error {
	if err := convertFromUnstructured(srcRaw, dst, nil); err != nil {
		return fmt.Errorf("HetznerBareMetalMachine: %w", err)
	}
	return nil
}

func (src *HetznerBareMetalMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HetznerBareMetalMachineTemplate)
	return convertUnstructured(src, dst, nil)
}

func (dst *HetznerBareMetalMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	if err := convertFromUnstructured(srcRaw, dst, nil); err != nil {
		return fmt.Errorf("HetznerBareMetalMachineTemplate: %w", err)
	}
	return nil
}

func (src *HetznerBareMetalRemediation) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HetznerBareMetalRemediation)
	return convertUnstructured(src, dst, nil)
}

func (dst *HetznerBareMetalRemediation) ConvertFrom(srcRaw conversion.Hub) error {
	if err := convertFromUnstructured(srcRaw, dst, nil); err != nil {
		return fmt.Errorf("HetznerBareMetalRemediation: %w", err)
	}
	return nil
}

func (src *HetznerBareMetalRemediationTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HetznerBareMetalRemediationTemplate)
	return convertUnstructured(src, dst, nil)
}

func (dst *HetznerBareMetalRemediationTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	if err := convertFromUnstructured(srcRaw, dst, nil); err != nil {
		return fmt.Errorf("HetznerBareMetalRemediationTemplate: %w", err)
	}
	return nil
}

func convertUnstructured(src runtime.Object, dst runtime.Object, mutate func(map[string]interface{})) error {
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(src)
	if err != nil {
		return err
	}

	obj["apiVersion"] = infrav1beta2.GroupVersion.String()

	copyBeta1DeprecatedConditions(obj)

	if mutate != nil {
		mutate(obj)
	}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj, dst); err != nil {
		return err
	}

	return nil
}

func convertFromUnstructured(src runtime.Object, dst runtime.Object, mutate func(map[string]interface{})) error {
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(src)
	if err != nil {
		return err
	}

	obj["apiVersion"] = GroupVersion.String()

	copyConditionsToDeprecated(obj)

	if mutate != nil {
		mutate(obj)
	}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj, dst); err != nil {
		return err
	}

	return nil
}

func copyBeta1DeprecatedConditions(obj map[string]interface{}) {
	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return
	}
	deprecated, ok := status["deprecated"].(map[string]interface{})
	if !ok {
		return
	}
	v1beta1, ok := deprecated["v1beta1"].(map[string]interface{})
	if !ok {
		return
	}
	conds, ok := v1beta1["conditions"]
	if !ok {
		return
	}
	status["conditions"] = sanitizeConditions(conds)
}

func copyConditionsToDeprecated(obj map[string]interface{}) {
	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return
	}
	conds, ok := status["conditions"]
	if !ok {
		return
	}

	sanitized := sanitizeConditions(conds)
	if len(sanitized) == 0 {
		return
	}

	deprecated, ok := status["deprecated"].(map[string]interface{})
	if !ok {
		deprecated = make(map[string]interface{})
		status["deprecated"] = deprecated
	}
	v1beta1, ok := deprecated["v1beta1"].(map[string]interface{})
	if !ok {
		v1beta1 = make(map[string]interface{})
		deprecated["v1beta1"] = v1beta1
	}
	v1beta1["conditions"] = sanitized
}

func sanitizeConditions(raw interface{}) []interface{} {
	conds, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	if len(conds) == 0 {
		return nil
	}
	sanitized := make([]interface{}, 0, len(conds))
	for _, rawCond := range conds {
		condMap, ok := rawCond.(map[string]interface{})
		if !ok {
			continue
		}
		copied := make(map[string]interface{}, len(condMap))
		for k, v := range condMap {
			copied[k] = v
		}
		if reason, ok := copied["reason"].(string); ok && reason == "" {
			copied["reason"] = "Succeeded"
		}
		sanitized = append(sanitized, copied)
	}
	return sanitized
}

func convertFailureDomainsMapToSlice(obj map[string]interface{}) {
	status := extractStatus(obj)
	if status == nil {
		return
	}
	raw, ok := status["failureDomains"]
	if !ok {
		return
	}
	fdMap, ok := raw.(map[string]interface{})
	if !ok || len(fdMap) == 0 {
		delete(status, "failureDomains")
		return
	}

	names := make([]string, 0, len(fdMap))
	for name := range fdMap {
		names = append(names, name)
	}
	sort.Strings(names)

	var entries []interface{}
	for _, name := range names {
		entry := map[string]interface{}{"name": name}
		rawEntry, _ := fdMap[name].(map[string]interface{})
		if rawEntry != nil {
			if controlPlane, exists := rawEntry["controlPlane"]; exists {
				entry["controlPlane"] = controlPlane
			}
			if attributes, exists := rawEntry["attributes"]; exists {
				entry["attributes"] = attributes
			}
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		delete(status, "failureDomains")
		return
	}
	status["failureDomains"] = entries
}

func convertFailureDomainsSliceToMap(obj map[string]interface{}) {
	status := extractStatus(obj)
	if status == nil {
		return
	}
	raw, ok := status["failureDomains"]
	if !ok {
		return
	}
	fdSlice, ok := raw.([]interface{})
	if !ok || len(fdSlice) == 0 {
		delete(status, "failureDomains")
		return
	}

	fdMap := make(map[string]interface{}, len(fdSlice))
	for _, entryRaw := range fdSlice {
		entry, _ := entryRaw.(map[string]interface{})
		if entry == nil {
			continue
		}
		name, _ := entry["name"].(string)
		if name == "" {
			continue
		}
		spec := make(map[string]interface{})
		if controlPlane, exists := entry["controlPlane"]; exists {
			spec["controlPlane"] = controlPlane
		}
		if attributes, exists := entry["attributes"]; exists {
			spec["attributes"] = attributes
		}
		fdMap[name] = spec
	}
	if len(fdMap) == 0 {
		delete(status, "failureDomains")
		return
	}
	status["failureDomains"] = fdMap
}

func extractStatus(obj map[string]interface{}) map[string]interface{} {
	statusRaw, ok := obj["status"]
	if !ok {
		return nil
	}
	status, _ := statusRaw.(map[string]interface{})
	return status
}
