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

package server

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/hetznercloud/hcloud-go/hcloud/schema"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	corev1 "k8s.io/api/core/v1"
)

const serverJSON = `
{
	"backup_window": "22-02",
	"created": "2016-01-30T23:55:00+00:00",
	"datacenter": {
		"description": "Falkenstein DC Park 8",
		"id": 42,
		"location": {
			"city": "Falkenstein",
			"country": "DE",
			"description": "Falkenstein DC Park 1",
			"id": 1,
			"latitude": 50.47612,
			"longitude": 12.370071,
			"name": "fsn1",
			"network_zone": "eu-central"
		},
		"name": "fsn1-dc8",
		"server_types": {
			"available": [
				1,
				2,
				3
			],
			"available_for_migration": [
				1,
				2,
				3
			],
			"supported": [
				1,
				2,
				3
			]
		}
	},
	"id": 42,
	"image": {
		"bound_to": null,
		"build_id": "c313fe40383af26094a5a92026054320ab55abc7",
		"created": "2016-01-30T23:55:00+00:00",
		"created_from": {
			"id": 1,
			"name": "Server"
		},
		"deleted": null,
		"deprecated": "2018-02-28T00:00:00+00:00",
		"description": "Ubuntu 20.04 Standard 64 bit",
		"disk_size": 10,
		"id": 42,
		"image_size": 2.3,
		"labels": {},
		"name": "ubuntu-20.04",
		"os_flavor": "ubuntu",
		"os_version": "20.04",
		"protection": {
			"delete": false
		},
		"rapid_deploy": false,
		"status": "available",
		"type": "snapshot"
	},
	"included_traffic": 654321,
	"ingoing_traffic": 123456,
	"iso": {
		"deprecated": "2018-02-28T00:00:00+00:00",
		"description": "FreeBSD 11.0 x64",
		"id": 42,
		"name": "FreeBSD-11.0-RELEASE-amd64-dvd1",
		"type": "public"
	},
	"labels": {},
	"load_balancers": [],
	"locked": false,
	"name": "my-resource",
	"outgoing_traffic": 123456,
	"placement_group": {
		"created": "2016-01-30T23:55:00+00:00",
		"id": 42,
		"labels": {},
		"name": "my-resource",
		"servers": [
			42
		],
		"type": "spread"
	},
	"primary_disk_size": 50,
	"private_net": [
		{
			"alias_ips": [],
			"ip": "10.0.0.2",
			"mac_address": "86:00:ff:2a:7d:e1",
			"network": 4711
		}
	],
	"protection": {
		"delete": false,
		"rebuild": false
	},
	"public_net": {
		"firewalls": [
			{
				"id": 42,
				"status": "applied"
			}
		],
		"floating_ips": [
			478
		],
		"ipv4": {
			"blocked": false,
			"dns_ptr": "server01.example.com",
			"id": 42,
			"ip": "1.2.3.4"
		},
		"ipv6": {
			"blocked": false,
			"dns_ptr": [
				{
					"dns_ptr": "server.example.com",
					"ip": "2001:db8::1"
				}
			],
			"id": 42,
			"ip": "2001:db8::/64"
		}
	},
	"rescue_enabled": false,
	"server_type": {
		"cores": 1,
		"cpu_type": "shared",
		"deprecated": false,
		"description": "CX11",
		"disk": 25,
		"id": 1,
		"memory": 1,
		"name": "cx11",
		"prices": [
			{
				"location": "fsn1",
				"price_hourly": {
					"gross": "1.1900000000000000",
					"net": "1.0000000000"
				},
				"price_monthly": {
					"gross": "1.1900000000000000",
					"net": "1.0000000000"
				}
			}
		],
		"storage_type": "local"
	},
	"status": "running",
	"volumes": []
}`

var server *hcloud.Server

const instanceState = hcloud.ServerStatusRunning

var ips = []string{"1.2.3.4", "2001:db8::3", "10.0.0.2"}
var addressTypes = []corev1.NodeAddressType{corev1.NodeExternalIP, corev1.NodeExternalIP, corev1.NodeInternalIP}

func TestServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Suite")
}

var _ = BeforeSuite(func() {
	var serverSchema schema.Server
	b := []byte(serverJSON)
	var buffer bytes.Buffer
	Expect(json.Compact(&buffer, b))
	Expect(json.Unmarshal(buffer.Bytes(), &serverSchema)).To(Succeed())

	server = hcloud.ServerFromSchema(serverSchema)
})

func newTestService(hcloudMachine *infrav1.HCloudMachine, hcloudClient hcloudclient.Client) *Service {
	return &Service{
		&scope.MachineScope{
			HCloudMachine: hcloudMachine,
			ClusterScope: scope.ClusterScope{
				HCloudClient: hcloudClient,
			},
		},
	}
}
