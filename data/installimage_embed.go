/*
Copyright 2026 The Kubernetes Authors.

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

// Package data registers embedded helper assets used by standalone tools.
//
// hbmh-provision-check runs outside the controller-manager pod, so it cannot rely
// on a local checkout of the installimage archive. Importing this package makes
// the embedded Hetzner installimage tarball available to sshclient.UntarTGZ.
package data

import (
	_ "embed"

	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
)

// installImageTGZ contains the vendored Hetzner installimage archive that gets
// uploaded into the rescue system before executing installimage.
//
//go:embed hetzner-installimage-v1.0.7.tgz
var installImageTGZ []byte

func init() {
	// Register the embedded archive once so blank-importing package data is
	// enough for standalone provisioning tools.
	sshclient.SetInstallImageTGZOverride(installImageTGZ)
}
