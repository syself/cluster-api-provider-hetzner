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
	osDevice string
	hostName string
	image    string
}

func buildAutoSetup(installImageSpec infrav1.InstallImage, autoSetupInput autoSetupInput) string {
	drive := fmt.Sprintf(`DRIVE1 /dev/%s
HOSTNAME %s
SWRAID 0`, autoSetupInput.osDevice, autoSetupInput.hostName)

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
%s`, drive, partitions, lvmDefinitions, btrfsDefinitions, image)
	fmt.Printf("Output: \n%s", output)
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
