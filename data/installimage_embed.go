package data

import (
	_ "embed"

	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
)

//go:embed hetzner-installimage-v1.0.7.tgz
var installImageTGZ []byte

func init() {
	sshclient.SetInstallImageTGZOverride(installImageTGZ)
}
