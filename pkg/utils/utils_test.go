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

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

var _ = DescribeTable("LabelsToLabelSelector",
	func(labels map[string]string, expectedOutput, expectedOutputInverse string) {
		Expect(utils.LabelsToLabelSelector(labels)).To(Or(Equal(expectedOutput), Equal(expectedOutputInverse)))
	},
	Entry(nil, map[string]string{
		"key1": "label1",
		"key2": "label2",
	}, "key2==label2,key1==label1", "key1==label1,key2==label2"),
	Entry(nil, map[string]string{}, "", ""),
)

var _ = DescribeTable("LabelSelectorToLabels",
	func(str string, expectedOutput map[string]string) {
		Expect(utils.LabelSelectorToLabels(str)).To(Equal(expectedOutput))
	},
	Entry(nil, "key2==label2,key1==label1", map[string]string{
		"key1": "label1",
		"key2": "label2",
	}),
	Entry(nil, "", map[string]string{}),
)

var _ = Describe("DifferenceOfStringSlices", func() {
	var a0 []string
	var a1 []string
	var a2 []string
	var a3 []string
	BeforeEach(func() {
		a0 = []string{}
		a1 = []string{
			"string1",
			"string2",
			"string3",
			"string4",
		}
		a2 = []string{
			"string1",
			"string2",
		}
		a2 = []string{
			"string3",
			"string4",
		}
	})
	DescribeTable("Computing differences",
		func(a, b, onlyInA, onlyInB []string) {
			outA, outB := utils.DifferenceOfStringSlices(a, b)
			Expect(outA).To(Equal(onlyInA))
			Expect(outB).To(Equal(onlyInB))
		},
		Entry(nil, a1, a2, a3, a0),
		Entry(nil, a2, a1, a0, a3),
		Entry(nil, a2, a0, a2, a0),
		Entry(nil, a0, a2, a0, a2),
	)
})

var _ = Describe("DifferenceOfIntSlices", func() {
	var a0 []int
	var a1 []int
	var a2 []int
	var a3 []int
	BeforeEach(func() {
		a0 = []int{}
		a1 = []int{
			1,
			2,
			3,
			4,
		}
		a2 = []int{
			1,
			2,
		}
		a2 = []int{
			3,
			4,
		}
	})
	DescribeTable("Computing differences",
		func(a, b, onlyInA, onlyInB []int) {
			outA, outB := utils.DifferenceOfIntSlices(a, b)
			Expect(outA).To(Equal(onlyInA))
			Expect(outB).To(Equal(onlyInB))
		},
		Entry(nil, a1, a2, a3, a0),
		Entry(nil, a2, a1, a0, a3),
		Entry(nil, a2, a0, a2, a0),
		Entry(nil, a0, a2, a0, a2),
	)
})
