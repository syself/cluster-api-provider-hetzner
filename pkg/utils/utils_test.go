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
	. "github.com/onsi/ginkgo/v2/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

var _ = DescribeTable("LabelsToLabelSelector",
	func(labels map[string]string, expectedOutput, expectedOutputInverse string) {
		Expect(utils.LabelsToLabelSelector(labels)).To(Or(Equal(expectedOutput), Equal(expectedOutputInverse)))
	},
	Entry("existing keys", map[string]string{
		"key1": "label1",
		"key2": "label2",
	}, "key2==label2,key1==label1", "key1==label1,key2==label2"),
	Entry("no keys", map[string]string{}, "", ""),
)

var _ = DescribeTable("LabelSelectorToLabels",
	func(str string, expectedOutput map[string]string) {
		Expect(utils.LabelSelectorToLabels(str)).To(Equal(expectedOutput))
	},
	Entry("existing keys", "key2==label2,key1==label1", map[string]string{
		"key1": "label1",
		"key2": "label2",
	}),
	Entry("no keys", "", map[string]string{}),
)

var _ = Describe("DifferenceOfStringSlices", func() {
	DescribeTable("Computing differences",
		func(a, b, onlyInA, onlyInB []string) {
			outA, outB := utils.DifferenceOfStringSlices(a, b)
			Expect(outA).To(Equal(onlyInA))
			Expect(outB).To(Equal(onlyInB))
		},
		Entry(
			"entry1",
			[]string{"string1", "string2", "string3", "string4"},
			[]string{"string1", "string2"},
			[]string{"string3", "string4"},
			nil,
		),
		Entry(
			"entry2",
			[]string{"string1", "string2"},
			[]string{"string1", "string2", "string3", "string4"},
			nil,
			[]string{"string3", "string4"},
		),
		Entry(
			"entry3",
			[]string{"string1", "string2"},
			nil,
			[]string{"string1", "string2"},
			nil),
		Entry(
			"entry4",
			nil,
			[]string{"string1", "string2"},
			nil,
			[]string{"string1", "string2"},
		),
	)
})

var _ = Describe("DifferenceOfIntSlices", func() {
	DescribeTable("Computing differences",
		func(a, b, onlyInA, onlyInB []int) {
			outA, outB := utils.DifferenceOfIntSlices(a, b)
			Expect(outA).To(Equal(onlyInA))
			Expect(outB).To(Equal(onlyInB))
		},
		Entry("entry1", []int{1, 2, 3, 4}, []int{1, 2}, []int{3, 4}, nil),
		Entry("entry2", []int{1, 2}, []int{1, 2, 3, 4}, nil, []int{3, 4}),
		Entry("entry3", []int{1, 2}, nil, []int{1, 2}, nil),
		Entry("entry4", nil, []int{1, 2}, nil, []int{1, 2}))
})

var _ = Describe("StringInList", func() {
	DescribeTable("Test string in list",
		func(list []string, str string, expectedOutcome bool) {
			out := utils.StringInList(list, str)
			Expect(out).To(Equal(expectedOutcome))
		},
		Entry("entry1", []string{"a", "b", "c"}, "a", true),
		Entry("entry2", []string{"a", "b", "c"}, "d", false))
})

var _ = Describe("FilterStringFromList", func() {
	DescribeTable("Test filter string from list",
		func(list []string, str string, expectedOutcome []string) {
			out := utils.FilterStringFromList(list, str)
			Expect(out).To(Equal(expectedOutcome))
		},
		Entry("entry1", []string{"a", "b", "c"}, "a", []string{"b", "c"}),
		Entry("entry2", []string{"a", "b", "c"}, "d", []string{"a", "b", "c"}))
})
