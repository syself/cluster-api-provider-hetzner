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

package placementgroup

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/hetznercloud/hcloud-go/hcloud/schema"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var placementGroups []*hcloud.PlacementGroup

const pgJSON = `{
  "placement_groups": [
    {
      "created": "2019-01-08T12:10:00+00:00",
      "id": 897,
      "labels": {
        "key": "value"
      },
      "name": "cluster-name-my Placement Group1",
      "servers": [
        4711,
        4712
      ],
      "type": "spread"
    },
		{
      "created": "2019-01-08T12:10:00+00:00",
      "id": 898,
      "labels": {
        "key": "value1"
      },
      "name": "cluster-name-my Placement Group2",
      "servers": [
        4713
      ],
      "type": "spread"
    },
		{
      "created": "2019-01-08T12:10:00+00:00",
      "id": 899,
      "labels": {
        "key": "value2"
      },
      "name": "cluster-name-my Placement Group3",
      "servers": [
      ],
      "type": "spread"
    }
  ]
}`

var ids = []int{897, 898, 899}

var names = []string{"my Placement Group1", "my Placement Group2", "my Placement Group3"}

const pgtype = "spread"

var pgServerMap = map[int][]int{
	897: {4711, 4712},
	898: {4713},
	899: {},
}

func TestPlacementgroup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Placementgroup Suite")
}

var _ = BeforeSuite(func() {
	var placementGroupsSchema struct {
		PlacementGroups []schema.PlacementGroup `json:"placement_groups"`
	}
	b := []byte(pgJSON)
	var buffer bytes.Buffer
	Expect(json.Compact(&buffer, b))
	Expect(json.Unmarshal(buffer.Bytes(), &placementGroupsSchema)).To(Succeed())

	schemas := placementGroupsSchema.PlacementGroups
	placementGroups = make([]*hcloud.PlacementGroup, len(schemas))
	for i := range schemas {
		placementGroups[i] = hcloud.PlacementGroupFromSchema(schemas[i])
	}
})
