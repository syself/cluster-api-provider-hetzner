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
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
)

type autoSetupInput struct {
	osDevices []string
	hostName  string
	image     string
}

func buildAutoSetup(installImageSpec *infrav1.InstallImage, asi autoSetupInput) string {
	var drives string
	for i, osDevice := range asi.osDevices {
		if i > 0 {
			drives = fmt.Sprintf(`%s
DRIVE%v /dev/%s`, drives, i+1, osDevice)
		} else {
			drives = fmt.Sprintf(`DRIVE%v /dev/%s`, i+1, osDevice)
		}
	}

	hostName := fmt.Sprintf(`
HOSTNAME %s
SWRAID %v`, asi.hostName, installImageSpec.Swraid)
	if installImageSpec.Swraid == 1 {
		hostName = fmt.Sprintf(`%s
SWRAIDLEVEL %v`, hostName, installImageSpec.SwraidLevel)
	}

	var partitions string
	for _, partition := range installImageSpec.Partitions {
		partitions = fmt.Sprintf(`%s
PART %s %s %s`, partitions, partition.Mount, partition.FileSystem, partition.Size)
	}

	// e.g. PART / ext4 all
	// e.g. PART /boot ext4 1024M

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
IMAGE %s`, asi.image)

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

// splitHostKey splits "namespace/hbhm-name" into two parts. Note: The namespace gets ignored, as
// cross-namespace references are not allowed.
func splitHostKey(key string) (namespace, name string) {
	parts := strings.Split(key, "/")
	if len(parts) != 2 {
		panic("unexpected host key")
	}
	return parts[0], parts[1]
}

// GetAssociatedHost gets the associated host by looking for an annotation on the machine that
// contains a reference to the host. Returns nil if no annotation exist or the referenced hbmh is
// not found.
func GetAssociatedHost(ctx context.Context, crClient client.Client, hbmm *infrav1.HetznerBareMetalMachine) (*infrav1.HetznerBareMetalHost, error) {
	annotations := hbmm.GetAnnotations()
	// if no annotations exist on machine, no host can be associated
	if annotations == nil {
		return nil, nil
	}

	// check if host annotation is set and return if not
	hostKey, ok := annotations[infrav1.HostAnnotation]
	if !ok {
		return nil, nil
	}

	// find associated host object and return it
	_, hostName := splitHostKey(hostKey)

	host := &infrav1.HetznerBareMetalHost{}
	key := client.ObjectKey{
		Name:      hostName,
		Namespace: hbmm.Namespace,
	}

	err := crClient.Get(ctx, key, host)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get host object: %w", err)
	}

	return host, nil
}
