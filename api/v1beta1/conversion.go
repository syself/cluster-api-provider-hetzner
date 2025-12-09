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
	"errors"
	"sort"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrav1beta2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
)

// Hub marks HetznerCluster as a conversion hub.
func (*HetznerCluster) Hub() {}

// Hub marks HetznerClusterList as a conversion hub.
func (*HetznerClusterList) Hub() {}

// Hub marks HCloudMachine as a conversion hub.
func (*HCloudMachine) Hub() {}

// Hub marks HCloudMachineList as a conversion hub.
func (*HCloudMachineList) Hub() {}

// Hub marks HCloudMachineTemplate as a conversion hub.
func (*HCloudMachineTemplate) Hub() {}

// Hub marks HCloudMachineTemplateList as a conversion hub.
func (*HCloudMachineTemplateList) Hub() {}

// Hub marks HCloudRemediation as a conversion hub.
func (*HCloudRemediation) Hub() {}

// Hub marks HCloudRemediationList as a conversion hub.
func (*HCloudRemediationList) Hub() {}

// Hub marks HCloudRemediationTemplate as a conversion hub.
func (*HCloudRemediationTemplate) Hub() {}

// Hub marks HCloudRemediationTemplateList as a conversion hub.
func (*HCloudRemediationTemplateList) Hub() {}

// Hub marks HetznerBareMetalHost as a conversion hub.
func (*HetznerBareMetalHost) Hub() {}

// Hub marks HetznerBareMetalHostList as a conversion hub.
func (*HetznerBareMetalHostList) Hub() {}

// Hub marks HetznerBareMetalMachine as a conversion hub.
func (*HetznerBareMetalMachine) Hub() {}

// Hub marks HetznerBareMetalMachineList as a conversion hub.
func (*HetznerBareMetalMachineList) Hub() {}

// Hub marks HetznerBareMetalMachineTemplate as a conversion hub.
func (*HetznerBareMetalMachineTemplate) Hub() {}

// Hub marks HetznerBareMetalMachineTemplateList as a conversion hub.
func (*HetznerBareMetalMachineTemplateList) Hub() {}

// Hub marks HetznerBareMetalRemediation as a conversion hub.
func (*HetznerBareMetalRemediation) Hub() {}

// Hub marks HetznerBareMetalRemediationList as a conversion hub.
func (*HetznerBareMetalRemediationList) Hub() {}

// Hub marks HetznerBareMetalRemediationTemplate as a conversion hub.
func (*HetznerBareMetalRemediationTemplate) Hub() {}

// Hub marks HetznerBareMetalRemediationTemplateList as a conversion hub.
func (*HetznerBareMetalRemediationTemplateList) Hub() {}

var errDownConversion = errors.New("conversion from v1beta2 to v1beta1 is not supported")

func (src *HetznerCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HetznerCluster)
	return convertUnstructured(src, dst, convertFailureDomainsMapToSlice)
}

func (dst *HetznerCluster) ConvertFrom(srcRaw conversion.Hub) error {
	return errDownConversion
}

func (src *HetznerClusterTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HetznerClusterTemplate)
	return convertUnstructured(src, dst, nil)
}

func (dst *HetznerClusterTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	return errDownConversion
}

func (src *HCloudMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HCloudMachine)
	return convertUnstructured(src, dst, nil)
}

func (dst *HCloudMachine) ConvertFrom(srcRaw conversion.Hub) error {
	return errDownConversion
}

func (src *HCloudMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HCloudMachineTemplate)
	return convertUnstructured(src, dst, nil)
}

func (dst *HCloudMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	return errDownConversion
}

func (src *HCloudRemediation) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HCloudRemediation)
	return convertUnstructured(src, dst, nil)
}

func (dst *HCloudRemediation) ConvertFrom(srcRaw conversion.Hub) error {
	return errDownConversion
}

func (src *HCloudRemediationTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HCloudRemediationTemplate)
	return convertUnstructured(src, dst, nil)
}

func (dst *HCloudRemediationTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	return errDownConversion
}

func (src *HetznerBareMetalHost) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HetznerBareMetalHost)
	return convertUnstructured(src, dst, nil)
}

func (dst *HetznerBareMetalHost) ConvertFrom(srcRaw conversion.Hub) error {
	return errDownConversion
}

func (src *HetznerBareMetalMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HetznerBareMetalMachine)
	return convertUnstructured(src, dst, nil)
}

func (dst *HetznerBareMetalMachine) ConvertFrom(srcRaw conversion.Hub) error {
	return errDownConversion
}

func (src *HetznerBareMetalMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HetznerBareMetalMachineTemplate)
	return convertUnstructured(src, dst, nil)
}

func (dst *HetznerBareMetalMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	return errDownConversion
}

func (src *HetznerBareMetalRemediation) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HetznerBareMetalRemediation)
	return convertUnstructured(src, dst, nil)
}

func (dst *HetznerBareMetalRemediation) ConvertFrom(srcRaw conversion.Hub) error {
	return errDownConversion
}

func (src *HetznerBareMetalRemediationTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.HetznerBareMetalRemediationTemplate)
	return convertUnstructured(src, dst, nil)
}

func (dst *HetznerBareMetalRemediationTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	return errDownConversion
}

func convertUnstructured(src runtime.Object, dst runtime.Object, mutate func(map[string]interface{})) error {
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(src)
	if err != nil {
		return err
	}

	if mutate != nil {
		mutate(obj)
	}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj, dst); err != nil {
		return err
	}

	return nil
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
