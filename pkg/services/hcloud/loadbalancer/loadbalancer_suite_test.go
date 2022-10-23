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

package loadbalancer

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/hetznercloud/hcloud-go/hcloud/schema"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

var lb *hcloud.LoadBalancer

const lbJSON = `{
	"algorithm": {
		"type": "round_robin"
	},
	"created": "2016-01-30T23:55:00+00:00",
	"id": 42,
	"included_traffic": 10000,
	"ingoing_traffic": null,
	"labels": {},
	"load_balancer_type": {
		"deprecated": "2016-01-30T23:50:00+00:00",
		"description": "LB11",
		"id": 1,
		"max_assigned_certificates": 10,
		"max_connections": 20000,
		"max_services": 5,
		"max_targets": 25,
		"name": "lb11",
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
		]
	},
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
	"name": "my-resource",
	"outgoing_traffic": null,
	"private_net": [
		{
			"ip": "10.0.0.2",
			"network": 4711
		}
	],
	"protection": {
		"delete": false
	},
	"public_net": {
		"enabled": false,
		"ipv4": {
			"dns_ptr": "lb1.example.com",
			"ip": "1.2.3.4"
		},
		"ipv6": {
			"dns_ptr": "lb1.example.com",
			"ip": "2001:db8::1"
		}
	},
	"services": [
		{
			"destination_port": 80,
			"health_check": {
				"http": {
					"domain": "example.com",
					"path": "/",
					"response": "{\"status\": \"ok\"}",
					"status_codes": [
						"2??",
						"3??"
					],
					"tls": false
				},
				"interval": 15,
				"port": 4711,
				"protocol": "http",
				"retries": 3,
				"timeout": 10
			},
			"http": {
				"certificates": [
					897
				],
				"cookie_lifetime": 300,
				"cookie_name": "HCLBSTICKY",
				"redirect_http": true,
				"sticky_sessions": true
			},
			"listen_port": 443,
			"protocol": "https",
			"proxyprotocol": false
		}
	],
	"targets": [
		{
			"health_status": [
				{
					"listen_port": 443,
					"status": "healthy"
				}
			],
			"ip": {
				"ip": "203.0.113.1"
			},
			"label_selector": {
				"selector": "env=prod"
			},
			"server": {
				"id": 80
			},
			"targets": [
				{
					"health_status": [
						{
							"listen_port": 443,
							"status": "healthy"
						}
					],
					"server": {
						"id": 85
					},
					"type": "server",
					"use_private_ip": false
				}
			],
			"type": "server",
			"use_private_ip": false
		},
		{
			"health_status": [
				{
					"listen_port": 444,
					"status": "healthy"
				}
			],
			"ip": {
				"ip": "203.0.114.1"
			},
			"label_selector": {
				"selector": "env=prod"
			},
			"server": {
				"id": 81
			},
			"targets": [
				{
					"health_status": [
						{
							"listen_port": 444,
							"status": "healthy"
						}
					],
					"server": {
						"id": 86
					},
					"type": "server",
					"use_private_ip": false
				}
			],
			"type": "server",
			"use_private_ip": false
		}
	]

}`

const ipv4 = "1.2.3.4"
const ipv6 = "2001:db8::1"
const protected = false
const internalIP = "10.0.0.2"

var targets = []infrav1.LoadBalancerTarget{
	{
		Type:     infrav1.LoadBalancerTargetTypeServer,
		ServerID: 80,
	},
	{
		Type:     infrav1.LoadBalancerTargetTypeServer,
		ServerID: 81,
	},
}

func TestLoadbalancer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Loadbalancer Suite")
}

var _ = BeforeSuite(func() {
	var lbSchema schema.LoadBalancer
	b := []byte(lbJSON)
	var buffer bytes.Buffer
	Expect(json.Compact(&buffer, b))
	Expect(json.Unmarshal(buffer.Bytes(), &lbSchema)).To(Succeed())
	lb = hcloud.LoadBalancerFromSchema(lbSchema)
})
