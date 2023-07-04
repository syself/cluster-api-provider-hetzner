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

package host

import (
	"fmt"
	"strings"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

type autoSetupInput struct {
	osDevices []string
	hostName  string
	image     string
}

func buildAutoSetup(installImageSpec *infrav1.InstallImage, autoSetupInput autoSetupInput) string {
	var drives string
	for i, osDevice := range autoSetupInput.osDevices {
		if i > 0 {
			drives = fmt.Sprintf(`%s
DRIVE%v /dev/%s`, drives, i+1, osDevice)
		} else {
			drives = fmt.Sprintf(`DRIVE%v /dev/%s`, i+1, osDevice)
		}
	}

	hostName := fmt.Sprintf(`
HOSTNAME %s
SWRAID %v`, autoSetupInput.hostName, installImageSpec.Swraid)
	if installImageSpec.Swraid == 1 {
		hostName = fmt.Sprintf(`%s
SWRAIDLEVEL %v`, hostName, installImageSpec.SwraidLevel)
	}

	var partitions string
	for _, partition := range installImageSpec.Partitions {
		partitions = fmt.Sprintf(`%s
PART %s %s %s`, partitions, partition.Mount, partition.FileSystem, partition.Size)
	}

	var lvmDefinitions string
	for _, lvm := range installImageSpec.LVMDefinitions {
		lvmDefinitions = fmt.Sprintf(`%s
LV %s %s %s %s %s`, lvmDefinitions, lvm.VG, lvm.Name, lvm.Mount, lvm.FileSystem, lvm.Size)
	}

	var btrfsDefinitions string
	for _, btrfs := range installImageSpec.BTRFSDefinitions {
		btrfsDefinitions = fmt.Sprintf(`%s
SUBVOL %s %s %s`, btrfsDefinitions, btrfs.Volume, btrfs.SubVolume, btrfs.Mount)
	}

	image := fmt.Sprintf(`
IMAGE %s`, autoSetupInput.image)

	output := fmt.Sprintf(`%s
%s
%s
%s
%s
%s`, drives, hostName, partitions, lvmDefinitions, btrfsDefinitions, image)
	return output
}

func validJSONFromSSHOutput(str string) string {
	if str == "" {
		return "{}"
	}
	tempString1 := strings.ReplaceAll(str, `" `, `","`)
	tempString2 := strings.ReplaceAll(tempString1, `="`, `":"`)
	return fmt.Sprintf(`{"%s}`, strings.TrimSpace(tempString2))
}

func trimLineBreak(str string) string {
	return strings.TrimSuffix(str, "\n")
}
