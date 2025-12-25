/*
Copyright 2024 The Kubernetes Authors.

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

package v1beta1

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HCloudMachine.SetBootState", func() {
	It("updates the timestamp only when the state changes", func() {
		machine := &HCloudMachine{}

		machine.SetBootState(HCloudBootStateInitializing)
		initialTimestamp := machine.Status.BootStateSince

		machine.SetBootState(HCloudBootStateInitializing)
		Expect(machine.Status.BootStateSince).To(Equal(initialTimestamp))

		time.Sleep(5 * time.Millisecond)
		machine.SetBootState(HCloudBootStateBootingToRescue)

		Expect(machine.Status.BootState).To(Equal(HCloudBootStateBootingToRescue))
		Expect(machine.Status.BootStateSince.Time.After(initialTimestamp.Time)).To(BeTrue())
	})
})
