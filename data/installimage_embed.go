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

// Package data exposes embedded helper assets used by standalone tools.
//
// hbmh-provision-check runs outside the controller-manager pod, so it cannot rely
// on a local checkout of the installimage archive. Standalone tools can call
// RegisterEmbeddedInstallImageTGZ to make the embedded Hetzner installimage
// tarball available to sshclient.UntarTGZ.
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

// RegisterEmbeddedInstallImageTGZ makes the embedded installimage archive
// available to sshclient.UntarTGZ.
func RegisterEmbeddedInstallImageTGZ() {
	sshclient.SetInstallImageTGZOverride(installImageTGZ)
}
