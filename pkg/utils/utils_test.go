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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

type testCaseLabelsToLabelSelector struct {
	labels                map[string]string
	expectedOutput        string
	expectedOutputInverse string
}

var _ = DescribeTable("LabelsToLabelSelector",
	func(tc testCaseLabelsToLabelSelector) {
		Expect(utils.LabelsToLabelSelector(tc.labels)).To(Or(Equal(tc.expectedOutput), Equal(tc.expectedOutputInverse)))
	},
	Entry("existing keys", testCaseLabelsToLabelSelector{
		labels: map[string]string{
			"key1": "label1",
			"key2": "label2",
		},
		expectedOutput:        "key2==label2,key1==label1",
		expectedOutputInverse: "key1==label1,key2==label2",
	}),
	Entry("no keys", testCaseLabelsToLabelSelector{
		labels:                map[string]string{},
		expectedOutput:        "",
		expectedOutputInverse: "",
	}),
)

type testCaseLabelSelectorToLabels struct {
	str            string
	expectedOutput map[string]string
}

var _ = DescribeTable("LabelSelectorToLabels",
	func(tc testCaseLabelSelectorToLabels) {
		Expect(utils.LabelSelectorToLabels(tc.str)).To(Equal(tc.expectedOutput))
	},
	Entry("existing keys", testCaseLabelSelectorToLabels{
		str: "key2==label2,key1==label1",
		expectedOutput: map[string]string{
			"key1": "label1",
			"key2": "label2",
		},
	}),
	Entry("no keys", testCaseLabelSelectorToLabels{
		str:            "",
		expectedOutput: map[string]string{},
	}),
)

var _ = Describe("DifferenceOfStringSlices", func() {
	type testCaseDifferenceOfStringSlices struct {
		a       []string
		b       []string
		onlyInA []string
		onlyInB []string
	}

	DescribeTable("Computing differences",
		func(tc testCaseDifferenceOfStringSlices) {
			outA, outB := utils.DifferenceOfStringSlices(tc.a, tc.b)
			Expect(outA).To(Equal(tc.onlyInA))
			Expect(outB).To(Equal(tc.onlyInB))
		},
		Entry("entry1", testCaseDifferenceOfStringSlices{
			a:       []string{"string1", "string2", "string3", "string4"},
			b:       []string{"string1", "string2"},
			onlyInA: []string{"string3", "string4"},
			onlyInB: nil,
		}),
		Entry("entry2", testCaseDifferenceOfStringSlices{
			a:       []string{"string1", "string2"},
			b:       []string{"string1", "string2", "string3", "string4"},
			onlyInA: nil,
			onlyInB: []string{"string3", "string4"},
		}),
		Entry("entry3", testCaseDifferenceOfStringSlices{
			a:       []string{"string1", "string2"},
			b:       nil,
			onlyInA: []string{"string1", "string2"},
			onlyInB: nil,
		}),
		Entry("entry4", testCaseDifferenceOfStringSlices{
			a:       nil,
			b:       []string{"string1", "string2"},
			onlyInA: nil,
			onlyInB: []string{"string1", "string2"},
		}),
	)
})

var _ = Describe("DifferenceOfIntSlices", func() {
	type testCaseDifferenceOfIntSlices struct {
		a       []int
		b       []int
		onlyInA []int
		onlyInB []int
	}

	DescribeTable("Computing differences",
		func(tc testCaseDifferenceOfIntSlices) {
			outA, outB := utils.DifferenceOfIntSlices(tc.a, tc.b)
			Expect(outA).To(Equal(tc.onlyInA))
			Expect(outB).To(Equal(tc.onlyInB))
		},
		Entry("entry1", testCaseDifferenceOfIntSlices{
			a:       []int{1, 2, 3, 4},
			b:       []int{1, 2},
			onlyInA: []int{3, 4},
			onlyInB: nil,
		}),
		Entry("entry2", testCaseDifferenceOfIntSlices{
			a:       []int{1, 2},
			b:       []int{1, 2, 3, 4},
			onlyInA: nil,
			onlyInB: []int{3, 4},
		}),
		Entry("entry3", testCaseDifferenceOfIntSlices{
			a:       []int{1, 2},
			b:       nil,
			onlyInA: []int{1, 2},
			onlyInB: nil,
		}),
		Entry("entry4", testCaseDifferenceOfIntSlices{
			a:       nil,
			b:       []int{1, 2},
			onlyInA: nil,
			onlyInB: []int{1, 2},
		}))
})

var _ = Describe("StringInList", func() {
	type testCaseStringInList struct {
		list            []string
		str             string
		expectedOutcome bool
	}

	DescribeTable("Test string in list",
		func(tc testCaseStringInList) {
			out := utils.StringInList(tc.list, tc.str)
			Expect(out).To(Equal(tc.expectedOutcome))
		},
		Entry("entry1", testCaseStringInList{
			list:            []string{"a", "b", "c"},
			str:             "a",
			expectedOutcome: true,
		}),
		Entry("entry2", testCaseStringInList{
			list:            []string{"a", "b", "c"},
			str:             "d",
			expectedOutcome: false,
		}))
})

var _ = Describe("FilterStringFromList", func() {
	type testCaseFilterStringFromList struct {
		list            []string
		str             string
		expectedOutcome []string
	}
	DescribeTable("Test filter string from list",
		func(tc testCaseFilterStringFromList) {
			out := utils.FilterStringFromList(tc.list, tc.str)
			Expect(out).To(Equal(tc.expectedOutcome))
		},
		Entry("entry1", testCaseFilterStringFromList{
			list:            []string{"a", "b", "c"},
			str:             "a",
			expectedOutcome: []string{"b", "c"},
		}),
		Entry("entry2", testCaseFilterStringFromList{
			list:            []string{"a", "b", "c"},
			str:             "d",
			expectedOutcome: []string{"a", "b", "c"},
		}))
})

var _ = Describe("Test removeOwnerRefFromList", func() {
	type testCaseRemoveOwnerRefFromList struct {
		RefList         []metav1.OwnerReference
		ExpectedRefList []metav1.OwnerReference
	}

	name := "bm-machine"
	kind := "HetznerBareMetalMachine"
	apiVersion := "v1beta1"

	expectedRefList3 := make([]metav1.OwnerReference, 0, 2)
	expectedRefList3 = append(expectedRefList3, metav1.OwnerReference{
		Name:       "bm-machine2",
		Kind:       "HetznerBareMetalMachine",
		APIVersion: "v1beta1",
	})

	DescribeTable("Test RemoveOwnerRefFromList",
		func(tc testCaseRemoveOwnerRefFromList) {
			refList := utils.RemoveOwnerRefFromList(tc.RefList, name, kind, apiVersion)
			Expect(refList).To(Equal(tc.ExpectedRefList))
		},
		Entry("List of one matching entry", testCaseRemoveOwnerRefFromList{
			RefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
			},
			ExpectedRefList: []metav1.OwnerReference{},
		}),
		Entry("List of one non-matching entry", testCaseRemoveOwnerRefFromList{
			RefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine2",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
			},
			ExpectedRefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine2",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
			},
		}),
		Entry("Two entries with matching", testCaseRemoveOwnerRefFromList{
			RefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine2",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
			},
			ExpectedRefList: expectedRefList3,
		}),
	)
})

var _ = Describe("Test FindOwnerRefFromList", func() {
	type testCaseFindOwnerRefFromList struct {
		RefList          []metav1.OwnerReference
		ExpectedPosition *int
	}

	name := "bm-machine"
	kind := "HetznerBareMetalMachine"
	apiVersion := "v1beta1"

	DescribeTable("Test FindOwnerRefFromList",
		func(tc testCaseFindOwnerRefFromList) {
			position, found := utils.FindOwnerRefFromList(tc.RefList, name, kind, apiVersion)

			if tc.ExpectedPosition != nil {
				Expect(found).To(BeTrue())
				Expect(position).To(Equal(*tc.ExpectedPosition))
			} else {
				Expect(found).To(BeFalse())
			}
		},
		Entry("Matching consumer", testCaseFindOwnerRefFromList{
			RefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
			},
			ExpectedPosition: pointer.Int(0),
		}),
		Entry("Matching consumer position 1", testCaseFindOwnerRefFromList{
			RefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine2",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
			},
			ExpectedPosition: pointer.Int(1),
		}),
		Entry("Matching consumer position 1a", testCaseFindOwnerRefFromList{
			RefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine",
					Kind:       "OtherBareMetalMachine",
					APIVersion: "v1beta1",
				},
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
			},
			ExpectedPosition: pointer.Int(1),
		}),
		Entry("Matching consumer position 1b", testCaseFindOwnerRefFromList{
			RefList: []metav1.OwnerReference{
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "hetzner/v1beta1",
				},
				{
					Name:       "bm-machine",
					Kind:       "HetznerBareMetalMachine",
					APIVersion: "v1beta1",
				},
			},
			ExpectedPosition: pointer.Int(1),
		}),
	)
})
