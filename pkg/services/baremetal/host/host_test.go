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

package host

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"github.com/syself/hrobot-go/models"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	bmmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks"
	robotmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/robot"
	sshmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/ssh"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
)

var errTest = fmt.Errorf("test error")

var _ = Describe("SetError and ClearError", func() {
	type testCaseSetError struct {
		errorType             infrav2.ErrorType
		errorMessage          string
		expectedReason        string
		expectedV1Beta1Reason string
	}

	DescribeTable("SetError sets the error type and the ActionCompleted condition",
		func(tc testCaseSetError) {
			host := helpers.BareMetalHost("test-host", "default")

			host.SetError(tc.errorType, tc.errorMessage)

			Expect(host.Status.ErrorType).To(Equal(tc.errorType))

			actionCompleted := conditions.Get(host, infrav2.HetznerBareMetalHostActionCompletedCondition)
			Expect(actionCompleted).ToNot(BeNil())
			Expect(actionCompleted.Status).To(Equal(metav1.ConditionFalse))
			Expect(actionCompleted.Reason).To(Equal(tc.expectedReason))
			Expect(actionCompleted.Message).To(Equal(tc.errorMessage))

			actionCompletedV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.ActionCompletedV1Beta1Condition)
			Expect(actionCompletedV1Beta1).ToNot(BeNil())
			Expect(actionCompletedV1Beta1.Status).To(Equal(corev1.ConditionFalse))
			Expect(actionCompletedV1Beta1.Reason).To(Equal(tc.expectedV1Beta1Reason))
			Expect(actionCompletedV1Beta1.Message).To(Equal(tc.errorMessage))
		},
		Entry("fatal error", testCaseSetError{
			errorType:             infrav2.ErrorTypeFatal,
			errorMessage:          "fatal failure",
			expectedReason:        infrav2.HetznerBareMetalHostActionCompletedFatalErrorReason,
			expectedV1Beta1Reason: infrav2.ActionCompletedFatalErrorV1Beta1Reason,
		}),
	)

	type testCaseSetOngoingReboot struct {
		rebootType            infrav2.RebootType
		message               string
		expectedReason        string
		expectedV1Beta1Reason string
	}

	DescribeTable("setOngoingReboot sets the OngoingReboot and the ActionCompleted condition",
		func(tc testCaseSetOngoingReboot) {
			host := helpers.BareMetalHost("test-host", "default")

			setOngoingReboot(host, tc.rebootType, tc.message)

			Expect(host.Status.OngoingReboot).ToNot(BeNil())
			Expect(host.Status.OngoingReboot.Type).To(Equal(tc.rebootType))
			Expect(host.Status.OngoingReboot.TriggeredAt.Time).To(BeTemporally("~", time.Now(), 5*time.Second))
			Expect(host.Status.ErrorType).To(BeEmpty())

			actionCompleted := conditions.Get(host, infrav2.HetznerBareMetalHostActionCompletedCondition)
			Expect(actionCompleted).ToNot(BeNil())
			Expect(actionCompleted.Status).To(Equal(metav1.ConditionFalse))
			Expect(actionCompleted.Reason).To(Equal(tc.expectedReason))
			Expect(actionCompleted.Message).To(Equal(tc.message))

			actionCompletedV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.ActionCompletedV1Beta1Condition)
			Expect(actionCompletedV1Beta1).ToNot(BeNil())
			Expect(actionCompletedV1Beta1.Status).To(Equal(corev1.ConditionFalse))
			Expect(actionCompletedV1Beta1.Reason).To(Equal(tc.expectedV1Beta1Reason))
			Expect(actionCompletedV1Beta1.Message).To(Equal(tc.message))
		},
		Entry("ssh reboot triggered", testCaseSetOngoingReboot{
			rebootType:            infrav2.RebootTypeSSH,
			message:               "ssh reboot triggered",
			expectedReason:        infrav2.HetznerBareMetalHostActionCompletedSSHRebootOngoingReason,
			expectedV1Beta1Reason: infrav2.ActionCompletedSSHRebootTriggeredV1Beta1Reason,
		}),
		Entry("software reboot triggered", testCaseSetOngoingReboot{
			rebootType:            infrav2.RebootTypeSoftware,
			message:               "software reboot triggered",
			expectedReason:        infrav2.HetznerBareMetalHostActionCompletedSoftwareRebootOngoingReason,
			expectedV1Beta1Reason: infrav2.ActionCompletedSoftwareRebootTriggeredV1Beta1Reason,
		}),
		Entry("hardware reboot triggered", testCaseSetOngoingReboot{
			rebootType:            infrav2.RebootTypeHardware,
			message:               "hardware reboot triggered",
			expectedReason:        infrav2.HetznerBareMetalHostActionCompletedHardwareRebootOngoingReason,
			expectedV1Beta1Reason: infrav2.ActionCompletedHardwareRebootTriggeredV1Beta1Reason,
		}),
	)

	It("clearOngoingReboot removes the ActionCompleted condition that setOngoingReboot set", func() {
		host := helpers.BareMetalHost("test-host", "default")
		setOngoingReboot(host, infrav2.RebootTypeSSH, "reboot via ssh")

		clearOngoingReboot(host)

		Expect(host.Status.OngoingReboot).To(BeNil())
		Expect(conditions.Get(host, infrav2.HetznerBareMetalHostActionCompletedCondition)).To(BeNil())
		Expect(deprecatedv1beta1conditions.Get(host, infrav2.ActionCompletedV1Beta1Condition)).To(BeNil())
	})

	It("clearOngoingReboot keeps the ActionCompleted condition of a host with a fatal error", func() {
		host := helpers.BareMetalHost("test-host", "default",
			helpers.WithError(infrav2.ErrorTypeFatal, "fatal failure"),
			helpers.WithOngoingReboot(infrav2.RebootTypeSSH, metav1.Now()),
		)

		clearOngoingReboot(host)

		Expect(host.Status.OngoingReboot).To(BeNil())
		actionCompleted := conditions.Get(host, infrav2.HetznerBareMetalHostActionCompletedCondition)
		Expect(actionCompleted).ToNot(BeNil())
		Expect(actionCompleted.Reason).To(Equal(infrav2.HetznerBareMetalHostActionCompletedFatalErrorReason))
	})

	It("SetError clears the OngoingReboot", func() {
		host := helpers.BareMetalHost("test-host", "default",
			helpers.WithOngoingReboot(infrav2.RebootTypeHardware, metav1.Now()),
		)

		host.SetError(infrav2.ErrorTypeFatal, "fatal failure")

		Expect(host.Status.ErrorType).To(Equal(infrav2.ErrorTypeFatal))
		Expect(host.Status.OngoingReboot).To(BeNil())
	})

	It("sets an unrecognized error type with the UnknownError reason", func() {
		host := helpers.BareMetalHost("test-host", "default")

		host.SetError(infrav2.ErrorType("some unknown state"), "unexpected")

		Expect(host.Status.ErrorType).To(Equal(infrav2.ErrorType("some unknown state")))

		actionCompleted := conditions.Get(host, infrav2.HetznerBareMetalHostActionCompletedCondition)
		Expect(actionCompleted).ToNot(BeNil())
		Expect(actionCompleted.Reason).To(Equal(infrav2.HetznerBareMetalHostActionCompletedUnknownErrorReason))

		actionCompletedV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.ActionCompletedV1Beta1Condition)
		Expect(actionCompletedV1Beta1).ToNot(BeNil())
		Expect(actionCompletedV1Beta1.Reason).To(Equal(infrav2.ActionCompletedUnknownErrorV1Beta1Reason))
	})

	It("updates the ActionCompleted condition when moving from one ongoing reboot to the next", func() {
		host := helpers.BareMetalHost("test-host", "default")
		conditions.Set(host, metav1.Condition{
			Type:    infrav2.HetznerBareMetalHostActionCompletedCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerBareMetalHostActionCompletedHardwareRebootOngoingReason,
			Message: "reboot via hardware",
		})

		setOngoingReboot(host, infrav2.RebootTypeSSH, "reboot via ssh")

		Expect(host.Status.OngoingReboot).ToNot(BeNil())
		Expect(host.Status.OngoingReboot.Type).To(Equal(infrav2.RebootTypeSSH))
		actionCompleted := conditions.Get(host, infrav2.HetznerBareMetalHostActionCompletedCondition)
		Expect(actionCompleted).ToNot(BeNil())
		Expect(actionCompleted.Status).To(Equal(metav1.ConditionFalse))
		Expect(actionCompleted.Reason).To(Equal(infrav2.HetznerBareMetalHostActionCompletedSSHRebootOngoingReason))
	})

	It("overwrites an existing error", func() {
		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithError(infrav2.ErrorTypeFatal, "first message"),
		)

		host.SetError(infrav2.ErrorTypePermanent, "new message")

		Expect(host.Status.ErrorType).To(Equal(infrav2.ErrorTypePermanent))
		actionCompleted := conditions.Get(host, infrav2.HetznerBareMetalHostActionCompletedCondition)
		Expect(actionCompleted).ToNot(BeNil())
		Expect(actionCompleted.Reason).To(Equal(infrav2.HetznerBareMetalHostActionCompletedPermanentErrorReason))
		Expect(actionCompleted.Message).To(ContainSubstring("new message"))
	})

	It("sets the permanent error annotation and names it in both ActionCompleted conditions", func() {
		host := helpers.BareMetalHost("test-host", "default")

		host.SetError(infrav2.ErrorTypePermanent, "permanent failure")

		Expect(host.Status.ErrorType).To(Equal(infrav2.ErrorTypePermanent))
		Expect(host.Annotations).To(HaveKey(infrav2.PermanentErrorAnnotation))

		wantMessage := fmt.Sprintf("permanent failure. Remove annotation %q, if you want the controller to use the hbmh again.",
			infrav2.PermanentErrorAnnotation)

		actionCompleted := conditions.Get(host, infrav2.HetznerBareMetalHostActionCompletedCondition)
		Expect(actionCompleted).ToNot(BeNil())
		Expect(actionCompleted.Reason).To(Equal(infrav2.HetznerBareMetalHostActionCompletedPermanentErrorReason))
		Expect(actionCompleted.Message).To(Equal(wantMessage))

		actionCompletedV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.ActionCompletedV1Beta1Condition)
		Expect(actionCompletedV1Beta1).ToNot(BeNil())
		Expect(actionCompletedV1Beta1.Status).To(Equal(corev1.ConditionFalse))
		Expect(actionCompletedV1Beta1.Reason).To(Equal(infrav2.ActionCompletedPermanentErrorV1Beta1Reason))
		Expect(actionCompletedV1Beta1.Message).To(Equal(wantMessage))
	})

	It("returns permanentErrorSet with a message naming the error and the annotation", func() {
		host := helpers.BareMetalHost("test-host", "default")

		permanentErrorSet, message := host.SetError(infrav2.ErrorTypePermanent, "pre-provision command exited 1")

		Expect(permanentErrorSet).To(BeTrue())
		Expect(message).To(ContainSubstring("pre-provision command exited 1"))
		Expect(message).To(ContainSubstring(infrav2.PermanentErrorAnnotation))
	})

	It("returns permanentErrorSet false and an empty message for a non-permanent error", func() {
		host := helpers.BareMetalHost("test-host", "default")

		permanentErrorSet, message := host.SetError(infrav2.ErrorTypeFatal, "some error")

		Expect(permanentErrorSet).To(BeFalse())
		Expect(message).To(BeEmpty())
	})

	It("emits the PermanentErrorSet event when the Service sets a permanent error", func() {
		for len(testEventRecorder.Events) > 0 {
			<-testEventRecorder.Events
		}

		host := helpers.BareMetalHost("test-host", "default")
		svc := newTestService(host, nil, nil, nil, nil)

		svc.setHostError(infrav2.ErrorTypePermanent, "pre-provision command exited 1")

		Expect(testEventRecorder.Events).To(HaveLen(1))
		event := <-testEventRecorder.Events
		Expect(event).To(ContainSubstring("pre-provision command exited 1"))
		Expect(event).To(ContainSubstring(infrav2.PermanentErrorAnnotation))
	})

	It("ErrorMessage returns the message SetError recorded", func() {
		host := helpers.BareMetalHost("test-host", "default")

		Expect(host.ErrorMessage()).To(BeEmpty())

		host.SetError(infrav2.ErrorTypeFatal, "cloud init returned an error")

		Expect(host.ErrorMessage()).To(Equal("cloud init returned an error"))
	})

	It("ClearError removes the error type and both ActionCompleted conditions", func() {
		host := helpers.BareMetalHost("test-host", "default")
		host.SetError(infrav2.ErrorTypePermanent, "permanent failure")

		host.ClearError()

		Expect(host.Status.ErrorType).To(Equal(infrav2.ErrorType("")))
		Expect(conditions.Get(host, infrav2.HetznerBareMetalHostActionCompletedCondition)).To(BeNil())
		Expect(deprecatedv1beta1conditions.Get(host, infrav2.ActionCompletedV1Beta1Condition)).To(BeNil())
	})
})

var _ = Describe("actionImageInstalling (customProvisioner)", func() {
	ctx := context.Background()

	// newBaseHost returns the host and the custom provisioner of the consuming
	// HetznerBareMetalMachine. The custom provisioner has to be set on the
	// HetznerBareMetalMachine of the scope after newTestService.
	newBaseHost := func() (*infrav2.HetznerBareMetalHost, *infrav2.CustomProvisioner) {
		commandDir := GinkgoT().TempDir()
		commandPath := filepath.Join(commandDir, "custom-provisioner-test.sh")
		Expect(os.WriteFile(commandPath, []byte("#!/usr/bin/env bash\n"), 0o600)).To(Succeed())
		oldCommandDir := baremetalCustomProvisionerDir
		baremetalCustomProvisionerDir = commandDir
		DeferCleanup(func() {
			baremetalCustomProvisionerDir = oldCommandDir
		})

		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithIPv4(),
			helpers.WithConsumerRef(),
			helpers.WithSSHStatus(),
		)
		// Custom provisioner mode.
		customProvisioner := &infrav2.CustomProvisioner{
			Command: filepath.Base(commandPath),
			URL:     "https://example.com/foo/image",
		}
		return host, customProvisioner
	}

	It("takes the installimage path when installImage is set instead of customProvisioner", func() {
		host, _ := newBaseHost()
		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{StdOut: "rescue"})
		sshMock.On("GetInstallImageState", mock.Anything).Return(sshclient.InstallImageStateRunning, nil)

		svc := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
		svc.scope.HetznerBareMetalMachine.Spec.InstallImage = &infrav2.InstallImage{
			Image: infrav2.Image{Name: "img", URL: "https://example.com/img.tar.gz"},
		}

		res := svc.actionImageInstalling(ctx)
		Expect(res).To(BeAssignableToTypeOf(actionContinue{}))
		// The installimage path checks GetInstallImageState; the custom-provisioner path is not taken.
		sshMock.AssertCalled(GinkgoT(), "GetInstallImageState", mock.Anything)
		sshMock.AssertNotCalled(GinkgoT(), "StateOfCustomProvisioner", mock.Anything)
	})

	It("returns continue when command is running", func() {
		host, customProvisioner := newBaseHost()
		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{StdOut: "rescue"})
		sshMock.On("StateOfCustomProvisioner", mock.Anything).Return(sshclient.CustomProvisionerStateRunning, "", nil)
		sshMock.On("ReadOutputJSON", mock.Anything).Return("", nil).Once()

		svc := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
		svc.scope.HetznerBareMetalMachine.Spec.CustomProvisioner = customProvisioner

		res := svc.actionImageInstalling(ctx)
		Expect(res).To(BeAssignableToTypeOf(actionContinue{}))
		// TODO(#2017): Remove the duplicated checks of deprecated v1beta1 conditions once all resources are native v1beta2.
		c := conditions.Get(host, infrav2.HetznerBareMetalHostProvisionSucceededCondition)
		Expect(c.Message).To(Equal(`custom provisioner running`))
		cV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.ProvisionSucceededV1Beta1Condition)
		Expect(cV1Beta1.Message).To(Equal(`custom provisioner running`))
	})

	It("reboots and completes when command finished successfully", func() {
		host, customProvisioner := newBaseHost()
		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{StdOut: "rescue"})
		sshMock.On("StateOfCustomProvisioner", mock.Anything).Return(sshclient.CustomProvisionerStateFinishedSuccessfully, "LOGFILE-CONTENT", nil)
		sshMock.On("ReadOutputJSON", mock.Anything).Return("", nil).Once()
		sshMock.On("Reboot", mock.Anything).Return(sshclient.Output{})

		robot := robotmock.Client{}
		robot.On("SetBMServerName", mock.Anything, infrav2.BareMetalHostNamePrefix+host.Spec.ConsumerRef.Name).Return(nil, nil)

		svc := newTestService(host, &robot, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
		svc.scope.HetznerBareMetalMachine.Spec.CustomProvisioner = customProvisioner

		res := svc.actionImageInstalling(ctx)
		Expect(res).To(BeAssignableToTypeOf(actionComplete{}))
		Expect(sshMock.AssertCalled(GinkgoT(), "Reboot", mock.Anything)).To(BeTrue())
		Expect(robot.AssertCalled(GinkgoT(), "SetBMServerName", mock.Anything, infrav2.BareMetalHostNamePrefix+host.Spec.ConsumerRef.Name)).To(BeTrue())
		// error should be cleared
		Expect(host.Status.ErrorType).To(BeEmpty())
		c := conditions.Get(host, infrav2.HetznerBareMetalHostProvisionSucceededCondition)
		Expect(c.Message).To(Equal(`host (test-host) is still provisioning - state "image-installing"`))
		cV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.ProvisionSucceededV1Beta1Condition)
		Expect(cV1Beta1.Message).To(Equal(`host (test-host) is still provisioning - state "image-installing"`))
	})

	It("retries when ReadOutputJSON fails during FinishedSuccessfully", func() {
		host, customProvisioner := newBaseHost()
		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{StdOut: "rescue"})
		sshMock.On("StateOfCustomProvisioner", mock.Anything).Return(sshclient.CustomProvisionerStateFinishedSuccessfully, "LOGFILE-CONTENT", nil)
		sshMock.On("ReadOutputJSON", mock.Anything).Return("", fmt.Errorf("ssh connection lost")).Once()

		svc := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
		svc.scope.HetznerBareMetalMachine.Spec.CustomProvisioner = customProvisioner

		res := svc.actionImageInstalling(ctx)
		Expect(res).To(BeAssignableToTypeOf(actionContinue{}))
	})

	It("stops when command failed", func() {
		host, customProvisioner := newBaseHost()
		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{StdOut: "rescue"})
		sshMock.On("StateOfCustomProvisioner", mock.Anything).Return(sshclient.CustomProvisionerStateFailed, "some logs", nil)
		sshMock.On("ReadOutputJSON", mock.Anything).Return("", nil).Once()

		svc := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
		svc.scope.HetznerBareMetalMachine.Spec.CustomProvisioner = customProvisioner
		res := svc.actionImageInstalling(ctx)
		Expect(res).To(BeAssignableToTypeOf(actionStop{}))
		Expect(host.Status.ErrorType).To(Equal(infrav2.ErrorTypeFatal))
		c := conditions.Get(host, infrav2.HetznerBareMetalHostProvisionSucceededCondition)
		Expect(c.Message).To(ContainSubstring("custom provisioner failed"))
		cV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.ProvisionSucceededV1Beta1Condition)
		Expect(cV1Beta1.Message).To(ContainSubstring("custom provisioner failed"))
	})

	It("completes successfully when CustomProvisionerStateFinishedSuccessfully", func() {
		host, customProvisioner := newBaseHost()
		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{StdOut: "rescue"})
		sshMock.On("StateOfCustomProvisioner", mock.Anything).Return(sshclient.CustomProvisionerStateFinishedSuccessfully, "LOGFILE-CONTENT", nil)
		sshMock.On("ReadOutputJSON", mock.Anything).Return(`{"status":"Succeeded"}`, nil).Once()
		sshMock.On("Reboot", mock.Anything).Return(sshclient.Output{})

		robot := robotmock.Client{}
		robot.On("SetBMServerName", mock.Anything, infrav2.BareMetalHostNamePrefix+host.Spec.ConsumerRef.Name).Return(nil, nil)

		svc := newTestService(host, &robot, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
		svc.scope.HetznerBareMetalMachine.Spec.CustomProvisioner = customProvisioner

		res := svc.actionImageInstalling(ctx)
		Expect(res).To(BeAssignableToTypeOf(actionComplete{}))

		// the ssh reboot after the install is recorded as the ongoing reboot
		Expect(host.Status.OngoingReboot).NotTo(BeNil())
		Expect(host.Status.OngoingReboot.Type).To(Equal(infrav2.RebootTypeSSH))
		actionCompleted := conditions.Get(host, infrav2.HetznerBareMetalHostActionCompletedCondition)
		Expect(actionCompleted).NotTo(BeNil())
		Expect(actionCompleted.Reason).To(Equal(infrav2.HetznerBareMetalHostActionCompletedSSHRebootOngoingReason))
	})

	It("starts the command on NotStarted and continues", func() {
		host, customProvisioner := newBaseHost()

		// Build service with fake client containing the bootstrap secret
		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{StdOut: "rescue"})
		sshMock.On("StateOfCustomProvisioner", mock.Anything).Return(sshclient.CustomProvisionerStateNotStarted, "", nil)

		svc := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
		svc.scope.HetznerBareMetalMachine.Spec.CustomProvisioner = customProvisioner
		commandPath := filepath.Join(baremetalCustomProvisionerDir, customProvisioner.Command)
		sshMock.On("StartCustomProvisioner", mock.Anything, commandPath, customProvisioner.URL, mock.Anything, svc.scope.Hostname(), []string{"nvme1n1"}).Return(0, "", nil)
		// Create the bootstrap secret referenced by the CAPI Machine in the fake client with key 'value'
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-secret", Namespace: host.Namespace}, Data: map[string][]byte{"value": []byte("#cloud-config")}}
		Expect(svc.scope.Client.Create(ctx, secret)).To(Succeed())

		sshMock.On("GetHardwareDetailsStorage", mock.Anything).Return(sshclient.Output{StdOut: `NAME="nvme1n1" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`})
		svc.scope.HetznerBareMetalHost.Spec.RootDeviceHints = &infrav2.RootDeviceHints{
			WWN: "eui.0025388801b4dff2",
		}

		res := svc.actionImageInstalling(ctx)
		Expect(res).To(BeAssignableToTypeOf(actionContinue{}))
		Expect(sshMock.AssertCalled(GinkgoT(), "StartCustomProvisioner", mock.Anything, commandPath, customProvisioner.URL, mock.Anything, svc.scope.Hostname(), []string{"nvme1n1"})).To(BeTrue())
		c := conditions.Get(host, infrav2.HetznerBareMetalHostProvisionSucceededCondition)
		Expect(c.Message).To(ContainSubstring(`custom provisioner started`))
		Expect(c.Reason).To(Equal("CustomProvisionerStarted"))
		cV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.ProvisionSucceededV1Beta1Condition)
		Expect(cV1Beta1.Message).To(ContainSubstring(`custom provisioner started`))
		Expect(cV1Beta1.Reason).To(Equal("CustomProvisionerStarted"))
	})

	It("passes WWN to StartCustomProvisioner when DeviceStringType is wwn", func() {
		host, customProvisioner := newBaseHost()

		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{StdOut: "rescue"})
		sshMock.On("StateOfCustomProvisioner", mock.Anything).Return(sshclient.CustomProvisionerStateNotStarted, "", nil)

		svc := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
		svc.scope.HetznerBareMetalMachine.Spec.CustomProvisioner = customProvisioner
		svc.scope.HetznerBareMetalMachine.Spec.CustomProvisioner.DeviceStringType = infrav2.DeviceStringTypeWWN
		svc.scope.HetznerBareMetalHost.Spec.RootDeviceHints = &infrav2.RootDeviceHints{
			WWN: "eui.0025388801b4dff2",
		}

		commandPath := filepath.Join(baremetalCustomProvisionerDir, customProvisioner.Command)
		sshMock.On("StartCustomProvisioner", mock.Anything, commandPath, customProvisioner.URL, mock.Anything, svc.scope.Hostname(), []string{"eui.0025388801b4dff2"}).Return(0, "", nil)

		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-secret", Namespace: host.Namespace}, Data: map[string][]byte{"value": []byte("#cloud-config")}}
		Expect(svc.scope.Client.Create(ctx, secret)).To(Succeed())

		res := svc.actionImageInstalling(ctx)
		Expect(res).To(BeAssignableToTypeOf(actionContinue{}))
		Expect(sshMock.AssertCalled(GinkgoT(), "StartCustomProvisioner", mock.Anything, commandPath, customProvisioner.URL, mock.Anything, svc.scope.Hostname(), []string{"eui.0025388801b4dff2"})).To(BeTrue())
		c := conditions.Get(host, infrav2.HetznerBareMetalHostProvisionSucceededCondition)
		Expect(c.Message).To(ContainSubstring(`custom provisioner started`))
		Expect(c.Reason).To(Equal("CustomProvisionerStarted"))
		cV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.ProvisionSucceededV1Beta1Condition)
		Expect(cV1Beta1.Message).To(ContainSubstring(`custom provisioner started`))
		Expect(cV1Beta1.Reason).To(Equal("CustomProvisionerStarted"))
	})

	It("returns error when DeviceStringType is wwn but no WWN is configured in rootDeviceHints", func() {
		host, customProvisioner := newBaseHost()

		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{StdOut: "rescue"})
		sshMock.On("StateOfCustomProvisioner", mock.Anything).Return(sshclient.CustomProvisionerStateNotStarted, "", nil)

		svc := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
		svc.scope.HetznerBareMetalMachine.Spec.CustomProvisioner = customProvisioner
		svc.scope.HetznerBareMetalMachine.Spec.CustomProvisioner.DeviceStringType = infrav2.DeviceStringTypeWWN
		// RootDeviceHints has no WWN set — empty list
		svc.scope.HetznerBareMetalHost.Spec.RootDeviceHints = &infrav2.RootDeviceHints{}

		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-secret", Namespace: host.Namespace}, Data: map[string][]byte{"value": []byte("#cloud-config")}}
		Expect(svc.scope.Client.Create(ctx, secret)).To(Succeed())

		res := svc.actionImageInstalling(ctx)
		Expect(res).To(BeAssignableToTypeOf(actionError{}))
		Expect(res.(actionError).err.Error()).To(ContainSubstring("no WWN is configured in rootDeviceHints"))
	})

	It("records failure when StartCustomProvisioner returns non-zero exit", func() {
		host, customProvisioner := newBaseHost()

		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{StdOut: "rescue"})
		sshMock.On("StateOfCustomProvisioner", mock.Anything).Return(sshclient.CustomProvisionerStateNotStarted, "", nil)

		svc := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
		svc.scope.HetznerBareMetalMachine.Spec.CustomProvisioner = customProvisioner
		commandPath := filepath.Join(baremetalCustomProvisionerDir, customProvisioner.Command)
		sshMock.On("StartCustomProvisioner", mock.Anything, commandPath, customProvisioner.URL, mock.Anything, svc.scope.Hostname(), []string{"nvme1n1"}).Return(7, "boom", nil)

		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-secret", Namespace: host.Namespace}, Data: map[string][]byte{"value": []byte("#cloud-config")}}
		Expect(svc.scope.Client.Create(ctx, secret)).To(Succeed())

		sshMock.On("GetHardwareDetailsStorage", mock.Anything).Return(sshclient.Output{StdOut: `NAME="nvme1n1" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`})
		svc.scope.HetznerBareMetalHost.Spec.RootDeviceHints = &infrav2.RootDeviceHints{
			WWN: "eui.0025388801b4dff2",
		}
		res := svc.actionImageInstalling(ctx)
		Expect(res).To(BeAssignableToTypeOf(actionContinue{}))
		result, err := res.Result()
		Expect(err).ToNot(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(time.Minute))
		Expect(host.Status.ErrorType).To(BeEmpty())
		Expect(conditions.Get(host, infrav2.HetznerBareMetalHostActionCompletedCondition)).To(BeNil())
		c := conditions.Get(host, infrav2.HetznerBareMetalHostProvisionSucceededCondition)
		Expect(c.Message).To(ContainSubstring("StartCustomProvisioner failed with non-zero exit status. Deleting machine"))
		Expect(c.Reason).To(Equal("CustomProvisionerFailedToStart"))
		cV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.ProvisionSucceededV1Beta1Condition)
		Expect(cV1Beta1.Message).To(ContainSubstring("StartCustomProvisioner failed with non-zero exit status. Deleting machine"))
		Expect(cV1Beta1.Reason).To(Equal("CustomProvisionerFailedToStart"))
	})

	It("times out after 20 minutes", func() {
		host, customProvisioner := newBaseHost()
		host.Status.OngoingReboot = &infrav2.OngoingReboot{
			Type:        infrav2.RebootTypeSSH,
			TriggeredAt: metav1.NewTime(time.Now().Add(-21 * time.Minute)),
		}

		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{StdOut: "rescue"})
		sshMock.On("StateOfCustomProvisioner", mock.Anything).Return(sshclient.CustomProvisionerStateRunning, "", nil)

		svc := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
		svc.scope.HetznerBareMetalMachine.Spec.CustomProvisioner = customProvisioner

		res := svc.actionImageInstalling(ctx)
		Expect(res).To(BeAssignableToTypeOf(actionStop{}))
		Expect(host.Status.ErrorType).To(Equal(infrav2.ErrorTypeFatal))
		c := conditions.Get(host, infrav2.HetznerBareMetalHostProvisionSucceededCondition)
		Expect(c.Message).To(ContainSubstring("custom provisioner timed out"))
		Expect(c.Reason).To(Equal("CustomProvisionerTimedOut"))
		cV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.ProvisionSucceededV1Beta1Condition)
		Expect(cV1Beta1.Message).To(ContainSubstring("custom provisioner timed out"))
		Expect(cV1Beta1.Reason).To(Equal("CustomProvisionerTimedOut"))
	})
})

var _ = Describe("test validateRootDeviceWwnsAreSubsetOfExistingWwns", func() {
	It("should return error when storageDevices is empty", func() {
		rootDeviceHints := &infrav2.RootDeviceHints{WWN: "wwn1"}
		storageDevices := []infrav2.Storage{}

		err := validateRootDeviceWwnsAreSubsetOfExistingWwns(rootDeviceHints, storageDevices)
		Expect(err).ToNot(BeNil())
		expectedError := fmt.Errorf(`%w for root device hint "wwn1". Known WWNs: []`, errMissingStorageDevice)
		Expect(err).To(Equal(expectedError))
	})
	It("should return nil when both rootDeviceHints and storageDevices are empty", func() {
		rootDeviceHints := &infrav2.RootDeviceHints{}
		storageDevices := []infrav2.Storage{}

		err := validateRootDeviceWwnsAreSubsetOfExistingWwns(rootDeviceHints, storageDevices)
		Expect(err).To(BeNil())
	})
	It("should return an error when rootDeviceHints contains WWNs not present in storageDevices", func() {
		rootDeviceHints := &infrav2.RootDeviceHints{WWN: "wwn3"}
		storageDevices := []infrav2.Storage{
			{WWN: "wwn1"},
			{WWN: "wwn2"},
		}

		err := validateRootDeviceWwnsAreSubsetOfExistingWwns(rootDeviceHints, storageDevices)
		Expect(err).NotTo(BeNil())
		expectedError := fmt.Errorf(`%w for root device hint "wwn3". Known WWNs: [wwn1 wwn2]`, errMissingStorageDevice)
		Expect(err).To(Equal(expectedError))
	})
	It("should return nil when rootDeviceHints contains WWNs present in storageDevices", func() {
		rootDeviceHints := &infrav2.RootDeviceHints{WWN: "wwn2"}
		storageDevices := []infrav2.Storage{
			{WWN: "wwn1"},
			{WWN: "wwn2"},
			{WWN: "wwn3"},
		}

		err := validateRootDeviceWwnsAreSubsetOfExistingWwns(rootDeviceHints, storageDevices)
		Expect(err).To(BeNil())
	})
})

var _ = Describe("obtainHardwareDetailsNics", func() {
	type testCaseObtainHardwareDetailsNics struct {
		stdout         string
		expectedOutput []infrav2.NIC
	}
	DescribeTable("Complete successfully",
		func(tc testCaseObtainHardwareDetailsNics) {
			sshMock := &sshmock.Client{}
			sshMock.On("GetHardwareDetailsNics", mock.Anything).Return(sshclient.Output{StdOut: tc.stdout})

			Expect(obtainHardwareDetailsNics(context.Background(), sshMock)).Should(Equal(tc.expectedOutput))
		},
		Entry("proper response", testCaseObtainHardwareDetailsNics{
			stdout: `name="eth0" model="Realtek Semiconductor Co." mac="a8:a1:59:94:19:42" ip="23.88.6.239/26" speedMbps="1000"
	name="eth0" model="Realtek Semiconductor Co." mac="a8:a1:59:94:19:42" ip="2a01:4f8:272:3e0f::2/64" speedMbps="1000"`,
			expectedOutput: []infrav2.NIC{
				{
					Name:      "eth0",
					Model:     "Realtek Semiconductor Co.",
					MAC:       "a8:a1:59:94:19:42",
					IP:        "23.88.6.239/26",
					SpeedMbps: 1000,
				}, {
					Name:      "eth0",
					Model:     "Realtek Semiconductor Co.",
					MAC:       "a8:a1:59:94:19:42",
					IP:        "2a01:4f8:272:3e0f::2/64",
					SpeedMbps: 1000,
				},
			},
		}),
	)
})

var _ = Describe("obtainHardwareDetailsStorage", func() {
	type testCaseObtainHardwareDetailsStorage struct {
		stdout               string
		expectedOutput       []infrav2.Storage
		expectedErrorMessage *string
	}
	DescribeTable("Complete successfully",
		func(tc testCaseObtainHardwareDetailsStorage) {
			sshMock := &sshmock.Client{}
			sshMock.On("GetHardwareDetailsStorage", mock.Anything).Return(sshclient.Output{StdOut: tc.stdout})

			storageDevices, err := obtainHardwareDetailsStorage(context.Background(), sshMock)
			Expect(storageDevices).Should(Equal(tc.expectedOutput))
			if tc.expectedErrorMessage != nil {
				Expect(err.Error()).Should(ContainSubstring(*tc.expectedErrorMessage))
			} else {
				Expect(err).To(Succeed())
			}
		},
		Entry("proper response", testCaseObtainHardwareDetailsStorage{
			stdout: `NAME="loop0" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
NAME="nvme2n1" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee8" ROTA="0"
NAME="nvme1n1" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			expectedOutput: []infrav2.Storage{
				{
					Name:         "nvme2n1",
					HCTL:         "",
					Model:        "SAMSUNG MZVL22T0HBLB-00B00",
					Vendor:       "",
					SerialNumber: "S677NF0R402742",
					SizeBytes:    2048408248320,
					SizeGB:       2048,
					WWN:          "eui.002538b411b2cee8",
					Rota:         false,
				},
				{
					Name:         "nvme1n1",
					HCTL:         "",
					Model:        "SAMSUNG MZVLB512HAJQ-00000",
					Vendor:       "",
					SerialNumber: "S3W8NX0N811178",
					SizeBytes:    512110190592,
					SizeGB:       512,
					WWN:          "eui.0025388801b4dff2",
					Rota:         false,
				},
			},
			expectedErrorMessage: nil,
		}),
		Entry("wrong rota", testCaseObtainHardwareDetailsStorage{
			stdout: `NAME="loop0" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="2"
	NAME="nvme2n1" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee8" ROTA="0"
	NAME="nvme1n1" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			expectedOutput:       nil,
			expectedErrorMessage: ptr.To("unknown rota"),
		}),
	)
})

var _ = Describe("handleIncompleteBoot", func() {
	Context("correct hostname == rescue", func() {
		type testCaseHandleIncompleteBootCorrectHostname struct {
			isRebootIntoRescue        bool
			isTimeOut                 bool
			isConnectionRefused       bool
			hostOngoingRebootType     infrav2.RebootType
			expectedReturnError       error
			expectedOngoingRebootType infrav2.RebootType
		}
		DescribeTable("hostName = rescue, varying ongoing reboot and ssh client response - robot client giving all positive results, no timeouts",
			func(tc testCaseHandleIncompleteBootCorrectHostname) {
				robotMock := robotmock.Client{}
				robotMock.On("SetBootRescue", mock.Anything, sshFingerprint).Return(nil, nil)
				robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: true}, nil)
				robotMock.On("RebootBMServer", mock.Anything, mock.Anything).Return(nil, nil)

				opts := []helpers.HostOpts{
					helpers.WithRebootTypes([]infrav2.RebootType{
						infrav2.RebootTypeSoftware,
						infrav2.RebootTypeHardware,
						infrav2.RebootTypePower,
					}),
					helpers.WithSSHStatus(),
				}
				if tc.hostOngoingRebootType != "" {
					opts = append(opts, helpers.WithOngoingReboot(tc.hostOngoingRebootType, metav1.Now()))
				}
				host := helpers.BareMetalHost("test-host", "default", opts...)
				service := newTestService(host, &robotMock, nil, nil, nil)
				ctx := context.Background()
				if tc.expectedReturnError == nil {
					_, err := service.handleIncompleteBoot(ctx, tc.isRebootIntoRescue, tc.isTimeOut, tc.isConnectionRefused)
					Expect(err).To(Succeed())
				} else {
					_, err := service.handleIncompleteBoot(ctx, tc.isRebootIntoRescue, tc.isTimeOut, tc.isConnectionRefused)
					Expect(err).Should(Equal(tc.expectedReturnError))
				}

				Expect(host.Status.OngoingReboot).ToNot(BeNil())
				Expect(host.Status.OngoingReboot.Type).To(Equal(tc.expectedOngoingRebootType))
			},
			Entry("timeout, no ongoing reboot", testCaseHandleIncompleteBootCorrectHostname{
				isRebootIntoRescue:        true,
				isTimeOut:                 true,
				isConnectionRefused:       false,
				hostOngoingRebootType:     infrav2.RebootType(""),
				expectedReturnError:       nil,
				expectedOngoingRebootType: infrav2.RebootTypeSSH,
			}),
			Entry("timeout, OngoingReboot == RebootTypeSoftware", testCaseHandleIncompleteBootCorrectHostname{
				isRebootIntoRescue:        true,
				isTimeOut:                 true,
				isConnectionRefused:       false,
				hostOngoingRebootType:     infrav2.RebootTypeSoftware,
				expectedReturnError:       nil,
				expectedOngoingRebootType: infrav2.RebootTypeSoftware,
			}),
			Entry("timeout, OngoingReboot == RebootTypeHardware", testCaseHandleIncompleteBootCorrectHostname{
				isRebootIntoRescue:        true,
				isTimeOut:                 true,
				isConnectionRefused:       false,
				hostOngoingRebootType:     infrav2.RebootTypeHardware,
				expectedReturnError:       nil,
				expectedOngoingRebootType: infrav2.RebootTypeHardware,
			}),
			Entry("timeout, OngoingReboot == RebootTypeSoftware", testCaseHandleIncompleteBootCorrectHostname{
				isRebootIntoRescue:        true,
				isTimeOut:                 true,
				isConnectionRefused:       false,
				hostOngoingRebootType:     infrav2.RebootTypeSoftware,
				expectedReturnError:       nil,
				expectedOngoingRebootType: infrav2.RebootTypeSoftware,
			}),
			Entry("timeout, OngoingReboot == RebootTypeHardware", testCaseHandleIncompleteBootCorrectHostname{
				isRebootIntoRescue:        true,
				isTimeOut:                 true,
				isConnectionRefused:       false,
				hostOngoingRebootType:     infrav2.RebootTypeHardware,
				expectedReturnError:       nil,
				expectedOngoingRebootType: infrav2.RebootTypeHardware,
			}),
			Entry("no timeout, OngoingReboot == RebootTypeSSH", testCaseHandleIncompleteBootCorrectHostname{
				isRebootIntoRescue:        true,
				isTimeOut:                 false,
				isConnectionRefused:       false,
				hostOngoingRebootType:     infrav2.RebootTypeSSH,
				expectedReturnError:       nil,
				expectedOngoingRebootType: infrav2.RebootTypeSoftware,
			}),
			Entry("wrong boot", testCaseHandleIncompleteBootCorrectHostname{
				isRebootIntoRescue:        false,
				isTimeOut:                 false,
				isConnectionRefused:       false,
				hostOngoingRebootType:     infrav2.RebootType(""),
				expectedReturnError:       nil,
				expectedOngoingRebootType: infrav2.RebootTypeSoftware,
			}),
		)

		type testCaseHandleIncompleteBootDifferentResetTypes struct {
			isTimeOut                 bool
			isConnectionRefused       bool
			rebootTypes               []infrav2.RebootType
			hostOngoingRebootType     infrav2.RebootType
			expectedOngoingRebootType infrav2.RebootType
			expectedRebootType        infrav2.RebootType
		}
		// Test with different reset type only software on machine
		DescribeTable("Different reset types",
			func(tc testCaseHandleIncompleteBootDifferentResetTypes) {
				robotMock := robotmock.Client{}
				robotMock.On("SetBootRescue", mock.Anything, sshFingerprint).Return(nil, nil)
				robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: true}, nil)
				robotMock.On("RebootBMServer", mock.Anything, mock.Anything).Return(nil, nil)

				opts := []helpers.HostOpts{
					helpers.WithSSHStatus(),
					helpers.WithRebootTypes(tc.rebootTypes),
				}
				if tc.hostOngoingRebootType != "" {
					// OngoingReboot.TriggeredAt is one hour ago. The reboot timeouts have passed for isTimeOut=true entries.
					opts = append(opts, helpers.WithOngoingReboot(tc.hostOngoingRebootType, metav1.NewTime(time.Now().Add(-time.Hour))))
				}
				host := helpers.BareMetalHost("test-host", "default", opts...)
				service := newTestService(host, &robotMock, nil, nil, nil)
				ctx := context.Background()
				_, err := service.handleIncompleteBoot(ctx, true, tc.isTimeOut, tc.isConnectionRefused)
				Expect(err).To(Succeed())
				Expect(host.Status.OngoingReboot).ToNot(BeNil())
				Expect(host.Status.OngoingReboot.Type).To(Equal(tc.expectedOngoingRebootType))
				if tc.expectedRebootType != infrav2.RebootType("") {
					Expect(robotMock.AssertCalled(GinkgoT(), "RebootBMServer", mock.Anything, tc.expectedRebootType)).To(BeTrue())
				} else {
					Expect(robotMock.AssertNotCalled(GinkgoT(), "RebootBMServer", mock.Anything, mock.Anything)).To(BeTrue())
				}
			},
			Entry("timeout, OngoingReboot == RebootTypeSSH, only hw reset", testCaseHandleIncompleteBootDifferentResetTypes{
				isTimeOut:                 true,
				isConnectionRefused:       false,
				rebootTypes:               []infrav2.RebootType{infrav2.RebootTypeHardware},
				hostOngoingRebootType:     infrav2.RebootTypeSSH,
				expectedOngoingRebootType: infrav2.RebootTypeHardware,
				expectedRebootType:        infrav2.RebootTypeHardware,
			}),
			Entry("wrong boot, only hw reset", testCaseHandleIncompleteBootDifferentResetTypes{
				isTimeOut:                 false,
				isConnectionRefused:       false,
				rebootTypes:               []infrav2.RebootType{infrav2.RebootTypeHardware},
				hostOngoingRebootType:     infrav2.RebootType(""),
				expectedOngoingRebootType: infrav2.RebootTypeHardware,
				expectedRebootType:        infrav2.RebootTypeHardware,
			}),
			Entry("wrong boot, only hw reset, OngoingReboot == RebootTypeSSH", testCaseHandleIncompleteBootDifferentResetTypes{
				isTimeOut:                 false,
				isConnectionRefused:       false,
				rebootTypes:               []infrav2.RebootType{infrav2.RebootTypeHardware},
				hostOngoingRebootType:     infrav2.RebootTypeSSH,
				expectedOngoingRebootType: infrav2.RebootTypeHardware,
				expectedRebootType:        infrav2.RebootTypeHardware,
			}),
			Entry("wrong boot, OngoingReboot == RebootTypeSSH", testCaseHandleIncompleteBootDifferentResetTypes{
				isTimeOut:                 false,
				isConnectionRefused:       false,
				rebootTypes:               []infrav2.RebootType{infrav2.RebootTypeSoftware, infrav2.RebootTypeHardware},
				hostOngoingRebootType:     infrav2.RebootTypeSSH,
				expectedOngoingRebootType: infrav2.RebootTypeSoftware,
				expectedRebootType:        infrav2.RebootTypeSoftware,
			}),
			Entry("wrong boot, OngoingReboot == RebootTypeSoftware", testCaseHandleIncompleteBootDifferentResetTypes{
				isTimeOut:                 false,
				isConnectionRefused:       false,
				rebootTypes:               []infrav2.RebootType{infrav2.RebootTypeSoftware, infrav2.RebootTypeHardware},
				hostOngoingRebootType:     infrav2.RebootTypeSoftware,
				expectedOngoingRebootType: infrav2.RebootTypeHardware,
				expectedRebootType:        infrav2.RebootTypeHardware,
			}),
			Entry("wrong boot, OngoingReboot == RebootTypeHardware", testCaseHandleIncompleteBootDifferentResetTypes{
				isTimeOut:                 false,
				isConnectionRefused:       false,
				rebootTypes:               []infrav2.RebootType{infrav2.RebootTypeSoftware, infrav2.RebootTypeHardware},
				hostOngoingRebootType:     infrav2.RebootTypeHardware,
				expectedOngoingRebootType: infrav2.RebootTypeHardware,
				expectedRebootType:        infrav2.RebootTypeHardware,
			}),
		)

		type testCaseHandleIncompleteBootDifferentTimeouts struct {
			hostOngoingRebootType     infrav2.RebootType
			rebootTriggeredAt         time.Time
			expectedOngoingRebootType infrav2.RebootType
			expectedRebootType        infrav2.RebootType
		}

		// Test with reached timeouts
		DescribeTable("Different timeouts",
			func(tc testCaseHandleIncompleteBootDifferentTimeouts) {
				robotMock := robotmock.Client{}
				robotMock.On("SetBootRescue", mock.Anything, sshFingerprint).Return(nil, nil)
				robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: true}, nil)
				robotMock.On("RebootBMServer", mock.Anything, mock.Anything).Return(nil, nil)

				host := helpers.BareMetalHost("test-host", "default",
					helpers.WithRebootTypes([]infrav2.RebootType{
						infrav2.RebootTypeSoftware,
						infrav2.RebootTypeHardware,
						infrav2.RebootTypePower,
					}),
					helpers.WithSSHStatus(),
					helpers.WithOngoingReboot(tc.hostOngoingRebootType, metav1.Time{Time: tc.rebootTriggeredAt}),
				)
				service := newTestService(host, &robotMock, nil, nil, nil)

				ctx := context.Background()
				_, err := service.handleIncompleteBoot(ctx, true, true, false)
				Expect(err).To(Succeed())
				Expect(host.Status.OngoingReboot).ToNot(BeNil())
				Expect(host.Status.OngoingReboot.Type).To(Equal(tc.expectedOngoingRebootType))
				if tc.expectedRebootType != infrav2.RebootType("") {
					Expect(robotMock.AssertCalled(GinkgoT(), "RebootBMServer", mock.Anything, tc.expectedRebootType)).To(BeTrue())
				} else {
					Expect(robotMock.AssertNotCalled(GinkgoT(), "RebootBMServer", mock.Anything, mock.Anything)).To(BeTrue())
				}
			},
			Entry("timed out sw reset", testCaseHandleIncompleteBootDifferentTimeouts{
				hostOngoingRebootType:     infrav2.RebootTypeSoftware,
				rebootTriggeredAt:         time.Now().Add(-15 * time.Minute),
				expectedOngoingRebootType: infrav2.RebootTypeHardware,
				expectedRebootType:        infrav2.RebootTypeHardware,
			}),
			Entry("not timed out hw reset", testCaseHandleIncompleteBootDifferentTimeouts{
				hostOngoingRebootType:     infrav2.RebootTypeHardware,
				rebootTriggeredAt:         time.Now().Add(-2 * time.Minute),
				expectedOngoingRebootType: infrav2.RebootTypeHardware,
				expectedRebootType:        infrav2.RebootType(""),
			}),
			Entry("not timed out sw reset", testCaseHandleIncompleteBootDifferentTimeouts{
				hostOngoingRebootType:     infrav2.RebootTypeSoftware,
				rebootTriggeredAt:         time.Now().Add(-3 * time.Minute),
				expectedOngoingRebootType: infrav2.RebootTypeSoftware,
				expectedRebootType:        infrav2.RebootType(""),
			}),
		)
		It("returns failed if connection error and timed out", func() {
			robotMock := robotmock.Client{}
			robotMock.On("SetBootRescue", mock.Anything, sshFingerprint).Return(nil, nil)
			robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: true}, nil)
			robotMock.On("RebootBMServer", mock.Anything, mock.Anything).Return(nil, nil)

			host := helpers.BareMetalHost("test-host", "default",
				helpers.WithRebootTypes([]infrav2.RebootType{
					infrav2.RebootTypeSoftware,
					infrav2.RebootTypeHardware,
					infrav2.RebootTypePower,
				}),
				helpers.WithSSHStatus(),
				helpers.WithOngoingReboot(infrav2.RebootTypeSSH, metav1.NewTime(time.Now().Add(-30*time.Minute))),
			)
			service := newTestService(host, &robotMock, nil, nil, nil)

			ctx := context.Background()
			failed, err := service.handleIncompleteBoot(ctx, true, false, true)
			Expect(err).ToNot(BeNil())
			Expect(failed).To(BeTrue())
			Expect(host.Status.OngoingReboot).ToNot(BeNil())
			Expect(host.Status.OngoingReboot.Type).To(Equal(infrav2.RebootTypeSSH))
			Expect(robotMock.AssertNotCalled(GinkgoT(), "RebootBMServer", mock.Anything, mock.Anything)).To(BeTrue())
		})

		It("fails if hardware reboot times out", func() {
			robotMock := robotmock.Client{}
			robotMock.On("SetBootRescue", mock.Anything, sshFingerprint).Return(nil, nil)
			robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: true}, nil)
			robotMock.On("RebootBMServer", mock.Anything, mock.Anything).Return(nil, nil)

			host := helpers.BareMetalHost("test-host", "default",
				helpers.WithRebootTypes([]infrav2.RebootType{
					infrav2.RebootTypeSoftware,
					infrav2.RebootTypeHardware,
					infrav2.RebootTypePower,
				}),
				helpers.WithSSHStatus(),
				helpers.WithOngoingReboot(infrav2.RebootTypeHardware, metav1.NewTime(time.Now().Add(-time.Hour))),
			)
			service := newTestService(host, &robotMock, nil, nil, nil)

			ctx := context.Background()
			_, err := service.handleIncompleteBoot(ctx, true, true, false)
			Expect(err).ToNot(Succeed())
			Expect(host.Status.OngoingReboot).ToNot(BeNil())
			Expect(host.Status.OngoingReboot.Type).To(Equal(infrav2.RebootTypeHardware))
			Expect(robotMock.AssertNotCalled(GinkgoT(), "RebootBMServer", mock.Anything, mock.Anything)).To(BeTrue())
		})
	})

	Context("hostname rescue vs machinename", func() {
		type testCaseHandleIncompleteBoot struct {
			isRebootIntoRescue        bool
			hostOngoingRebootType     infrav2.RebootType
			expectedReturnError       error
			expectedOngoingRebootType infrav2.RebootType
			expectsRescueCall         bool
		}

		DescribeTable("vary hostname and see whether rescue gets triggered",
			func(tc testCaseHandleIncompleteBoot) {
				robotMock := robotmock.Client{}
				robotMock.On("SetBootRescue", mock.Anything, sshFingerprint).Return(nil, nil)
				robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: true}, nil)
				robotMock.On("RebootBMServer", mock.Anything, mock.Anything).Return(nil, nil)

				opts := []helpers.HostOpts{
					helpers.WithRebootTypes([]infrav2.RebootType{
						infrav2.RebootTypeSoftware,
						infrav2.RebootTypeHardware,
						infrav2.RebootTypePower,
					}),
					helpers.WithSSHStatus(),
				}
				if tc.hostOngoingRebootType != "" {
					opts = append(opts, helpers.WithOngoingReboot(tc.hostOngoingRebootType, metav1.Now()))
				}
				host := helpers.BareMetalHost("test-host", "default", opts...)
				service := newTestService(host, &robotMock, nil, nil, nil)

				ctx := context.Background()
				if tc.expectedReturnError == nil {
					_, err := service.handleIncompleteBoot(ctx, tc.isRebootIntoRescue, false, false)
					Expect(err).To(Succeed())
				} else {
					_, err := service.handleIncompleteBoot(ctx, tc.isRebootIntoRescue, false, false)
					Expect(err).Should(Equal(tc.expectedReturnError))
				}
				Expect(host.Status.OngoingReboot).ToNot(BeNil())
				Expect(host.Status.OngoingReboot.Type).To(Equal(tc.expectedOngoingRebootType))
				if tc.expectsRescueCall {
					Expect(robotMock.AssertCalled(GinkgoT(), "GetBootRescue", mock.Anything)).To(BeTrue())
				} else {
					Expect(robotMock.AssertNotCalled(GinkgoT(), "GetBootRescue", mock.Anything)).To(BeTrue())
				}
			},
			Entry("hostname == rescue", testCaseHandleIncompleteBoot{
				isRebootIntoRescue:        true,
				hostOngoingRebootType:     infrav2.RebootType(""),
				expectedReturnError:       nil,
				expectedOngoingRebootType: infrav2.RebootTypeSoftware,
				expectsRescueCall:         true,
			}),
			Entry("hostname != rescue", testCaseHandleIncompleteBoot{
				isRebootIntoRescue:        false,
				hostOngoingRebootType:     infrav2.RebootType(""),
				expectedReturnError:       nil,
				expectedOngoingRebootType: infrav2.RebootTypeSoftware,
				expectsRescueCall:         false,
			}),
			Entry("hostname == rescue, OngoingReboot == RebootTypeSSH", testCaseHandleIncompleteBoot{
				isRebootIntoRescue:        true,
				hostOngoingRebootType:     infrav2.RebootTypeSSH,
				expectedReturnError:       nil,
				expectedOngoingRebootType: infrav2.RebootTypeSoftware,
				expectsRescueCall:         true,
			}),
			Entry("hostname != rescue, OngoingReboot == RebootTypeSSH", testCaseHandleIncompleteBoot{
				isRebootIntoRescue:        false,
				hostOngoingRebootType:     infrav2.RebootTypeSSH,
				expectedReturnError:       nil,
				expectedOngoingRebootType: infrav2.RebootTypeSoftware,
				expectsRescueCall:         false,
			}),
		)
	})

	Context("connection refused", func() {
		DescribeTable("keeps the reboot method in the host status while the timeout has not passed",
			func(rebootType infrav2.RebootType) {
				robotMock := robotmock.Client{}
				robotMock.On("RebootBMServer", mock.Anything, mock.Anything).Return(nil, nil)

				host := helpers.BareMetalHost("test-host", "default",
					helpers.WithRebootTypes([]infrav2.RebootType{
						infrav2.RebootTypeSoftware,
						infrav2.RebootTypeHardware,
					}),
					helpers.WithSSHStatus(),
					helpers.WithOngoingReboot(rebootType, metav1.NewTime(time.Now().Add(-time.Minute))),
				)
				service := newTestService(host, &robotMock, nil, nil, nil)

				failed, err := service.handleIncompleteBoot(context.Background(), true, false, true)
				Expect(err).To(Succeed())
				Expect(failed).To(BeFalse())
				Expect(host.Status.OngoingReboot).ToNot(BeNil())
				Expect(host.Status.OngoingReboot.Type).To(Equal(rebootType))
				Expect(robotMock.AssertNotCalled(GinkgoT(), "RebootBMServer", mock.Anything, mock.Anything)).To(BeTrue())
			},
			Entry("ssh reboot", infrav2.RebootTypeSSH),
			Entry("software reboot", infrav2.RebootTypeSoftware),
			Entry("hardware reboot", infrav2.RebootTypeHardware),
		)

	})
})

var _ = Describe("ensureSSHKey", func() {
	defaultFingerPrint := "my-fingerprint"

	It("sets an error error if a key that exists under another name is uploaded", func() {
		secret := helpers.GetDefaultSSHSecret("ssh-secret", "default")
		robotMock := robotmock.Client{}
		sshSecretKeyRef := infrav2.SSHSecretKeyRef{
			Name:       "sshkey-name",
			PublicKey:  "public-key",
			PrivateKey: "private-key",
		}
		robotMock.On("SetSSHKey", string(secret.Data[sshSecretKeyRef.Name]), mock.Anything).Return(
			nil, models.Error{Code: models.ErrorCodeKeyAlreadyExists, Message: "key already exists"},
		)
		robotMock.On("ListSSHKeys").Return([]models.Key{
			{
				Name:        "secret2",
				Fingerprint: "my fingerprint",
			},
			{
				Name:        "secret3",
				Fingerprint: "my fingerprint",
			},
		}, nil)

		host := helpers.BareMetalHost("test-host", "default")

		service := newTestService(host, &robotMock, nil, nil, nil)

		sshKey, actResult := service.ensureSSHKey(infrav2.SSHSecretRef{
			Name: "secret-name",
			Key:  sshSecretKeyRef,
		}, secret)

		emptySSHKey := infrav2.SSHKey{}
		Expect(sshKey).To(Equal(emptySSHKey))
		Expect(actResult).To(BeAssignableToTypeOf(actionContinue{}))
		result, err := actResult.Result()
		Expect(err).ToNot(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(5 * time.Minute))
		Expect(host.Status.ErrorType).To(BeEmpty())
		Expect(conditions.Get(host, infrav2.HetznerBareMetalHostActionCompletedCondition)).To(BeNil())
		sshKeysAvailable := conditions.Get(host, infrav2.HetznerBareMetalHostSSHKeysAvailableCondition)
		Expect(sshKeysAvailable).ToNot(BeNil())
		Expect(sshKeysAvailable.Status).To(Equal(metav1.ConditionFalse))
		Expect(sshKeysAvailable.Reason).To(Equal(infrav2.HetznerBareMetalHostSSHKeyAlreadyExistsReason))
	})

	type testCaseEnsureSSHKey struct {
		hetznerSSHKeys       []models.Key
		sshSecretKeyRef      infrav2.SSHSecretKeyRef
		expectedFingerprint  string
		expectedActionResult actionResult
		expectSetSSHKey      bool
	}

	DescribeTable("ensureSSHKey",
		func(tc testCaseEnsureSSHKey) {
			secret := helpers.GetDefaultSSHSecret("ssh-secret", "default")
			robotMock := robotmock.Client{}
			robotMock.On("SetSSHKey", string(secret.Data[tc.sshSecretKeyRef.Name]), mock.Anything).Return(
				&models.Key{Name: tc.sshSecretKeyRef.Name, Fingerprint: defaultFingerPrint}, nil,
			)
			robotMock.On("ListSSHKeys").Return(tc.hetznerSSHKeys, nil)

			host := helpers.BareMetalHost("test-host", "default")

			service := newTestService(host, &robotMock, nil, nil, nil)

			sshKey, actResult := service.ensureSSHKey(infrav2.SSHSecretRef{
				Name: "secret-name",
				Key:  tc.sshSecretKeyRef,
			}, secret)

			Expect(sshKey.Fingerprint).To(Equal(tc.expectedFingerprint))
			Expect(actResult).Should(BeAssignableToTypeOf(tc.expectedActionResult))
			if tc.expectSetSSHKey {
				Expect(robotMock.AssertCalled(GinkgoT(), "SetSSHKey", string(secret.Data[tc.sshSecretKeyRef.Name]), mock.Anything)).To(BeTrue())
			} else {
				Expect(robotMock.AssertNotCalled(GinkgoT(), "SetSSHKey", string(secret.Data[tc.sshSecretKeyRef.Name]), mock.Anything)).To(BeTrue())
			}
		},
		Entry("empty list", testCaseEnsureSSHKey{
			hetznerSSHKeys: nil,
			sshSecretKeyRef: infrav2.SSHSecretKeyRef{
				Name:       "sshkey-name",
				PublicKey:  "public-key",
				PrivateKey: "private-key",
			},
			expectedFingerprint:  defaultFingerPrint,
			expectedActionResult: actionComplete{},
			expectSetSSHKey:      true,
		}),
		Entry("secret in list", testCaseEnsureSSHKey{
			hetznerSSHKeys: []models.Key{
				{
					Name:        "my-name",
					Fingerprint: "my-fingerprint",
				},
			},
			sshSecretKeyRef: infrav2.SSHSecretKeyRef{
				Name:       "sshkey-name",
				PublicKey:  "public-key",
				PrivateKey: "private-key",
			},
			expectedFingerprint:  "my-fingerprint",
			expectedActionResult: actionComplete{},
			expectSetSSHKey:      false,
		}),
		Entry(
			"secret not in list", testCaseEnsureSSHKey{
				hetznerSSHKeys: []models.Key{
					{
						Name:        "secret2",
						Fingerprint: "my fingerprint",
					},
					{
						Name:        "secret3",
						Fingerprint: "my fingerprint",
					},
				},
				sshSecretKeyRef: infrav2.SSHSecretKeyRef{
					Name:       "sshkey-name",
					PublicKey:  "public-key",
					PrivateKey: "private-key",
				},
				expectedFingerprint:  defaultFingerPrint,
				expectedActionResult: actionComplete{},
				expectSetSSHKey:      true,
			}),
	)
})

var _ = Describe("actionPreparing", func() {
	It("continues when Robot omits rescue availability from the server response", func() {
		host := helpers.BareMetalHost(
			"test-host",
			"default",
		)

		robotMock := robotmock.Client{}
		robotMock.On("GetBMServer", mock.Anything).Return(&models.Server{
			ServerNumber:  1,
			ServerIP:      "1.2.3.4",
			ServerIPv6Net: "2a01:4f9:3051:12ce::",
			Rescue:        false,
		}, nil)
		robotMock.On("ListSSHKeys").Return([]models.Key{}, nil)
		robotMock.On("SetSSHKey", mock.Anything, mock.Anything).Return(
			&models.Key{Name: rescueSSHKeyName, Fingerprint: sshFingerprint},
			nil,
		)
		robotMock.On("GetReboot", mock.Anything).Return(&models.Reset{Type: []string{"sw", "hw"}}, nil)
		robotMock.On("DeleteBootRescue", mock.Anything).Return(&models.Rescue{Active: false}, nil)
		robotMock.On("SetBootRescue", mock.Anything, sshFingerprint).Return(&models.Rescue{Active: true}, nil)
		robotMock.On("RebootBMServer", mock.Anything, infrav2.RebootTypeSoftware).Return(&models.ResetPost{}, nil)

		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{})

		service := newTestService(
			host,
			&robotMock,
			bmmock.NewSSHFactory(sshMock, sshMock, sshMock),
			helpers.GetDefaultSSHSecret(osSSHKeyName, "default"),
			helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"),
		)

		actResult := service.actionPreparing(context.Background())

		Expect(actResult).To(BeAssignableToTypeOf(actionComplete{}))
		Expect(host.Status.OngoingReboot).ToNot(BeNil())
		Expect(host.Status.OngoingReboot.Type).To(Equal(infrav2.RebootTypeSoftware))
		Expect(host.Status.IPv4).To(Equal("1.2.3.4"))
		Expect(host.Status.IPv6).To(Equal("2a01:4f9:3051:12ce::1"))
		Expect(robotMock.AssertCalled(GinkgoT(), "DeleteBootRescue", mock.Anything)).To(BeTrue())
		Expect(robotMock.AssertCalled(GinkgoT(), "SetBootRescue", mock.Anything, sshFingerprint)).To(BeTrue())
	})

	It("sets a permanent error when the server has no IPv4", func() {
		host := helpers.BareMetalHost(
			"test-host",
			"default",
		)

		robotMock := robotmock.Client{}
		robotMock.On("GetBMServer", mock.Anything).Return(&models.Server{
			ServerNumber:  1,
			ServerIP:      "",
			ServerIPv6Net: "2a01:4f9:3051:12ce::",
		}, nil)

		service := newTestService(
			host,
			&robotMock,
			nil,
			helpers.GetDefaultSSHSecret(osSSHKeyName, "default"),
			helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"),
		)

		actResult := service.actionPreparing(context.Background())

		Expect(actResult).To(BeAssignableToTypeOf(actionStop{}))
		Expect(host.Status.ErrorType).To(Equal(infrav2.ErrorTypePermanent))
		Expect(host.Status.IPv4).To(BeEmpty())
		Expect(host.Annotations).To(HaveKey(infrav2.PermanentErrorAnnotation))
	})
})

var _ = Describe("analyzeSSHOutputInstallImage", func() {
	type testCaseAnalyzeSSHOutputInstallImageOutErr struct {
		err                         error
		rescueActive                bool
		expectedIsTimeout           bool
		expectedIsConnectionRefused bool
		expectedErrMessage          string
	}

	DescribeTable("analyzeSSHOutputInstallImage - out.Err",
		func(tc testCaseAnalyzeSSHOutputInstallImageOutErr) {
			host := helpers.BareMetalHost(
				"test-host",
				"default",
			)

			robotMock := robotmock.Client{}
			robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: tc.rescueActive}, nil)

			service := newTestService(host, &robotMock, nil, nil, nil)

			isTimeout, isConnectionRefused, err := service.analyzeSSHOutputRegistering(sshclient.Output{Err: tc.err})
			Expect(isTimeout).To(Equal(tc.expectedIsTimeout))
			Expect(isConnectionRefused).To(Equal(tc.expectedIsConnectionRefused))
			if tc.expectedErrMessage != "" {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(tc.expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry("timeout error", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         timeout,
			rescueActive:                true,
			expectedIsTimeout:           true,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          "",
		}),
		Entry("authenticationFailed error, rescue active", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         sshclient.ErrAuthenticationFailed,
			rescueActive:                true,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          "",
		}),
		Entry("authenticationFailed error, rescue not active", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         sshclient.ErrAuthenticationFailed,
			rescueActive:                false,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          "wrong ssh key",
		}),
		Entry("connectionRefused error, rescue active", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         syscall.ECONNREFUSED,
			rescueActive:                true,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: true,
			expectedErrMessage:          "",
		}),
		Entry("connectionRefused error, rescue not active", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         syscall.ECONNREFUSED,
			rescueActive:                false,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: true,
			expectedErrMessage:          "",
		}),
	)

	type testCaseAnalyzeSSHOutputInstallImageStdErr struct {
		hasNilErr          bool
		stdErr             string
		hostName           string
		expectedErrMessage string
	}

	DescribeTable("analyzeSSHOutputRegistering - toggle stdErr and hostName",
		func(tc testCaseAnalyzeSSHOutputInstallImageStdErr) {
			var err error
			if !tc.hasNilErr {
				err = errTest
			}

			out := sshclient.Output{
				StdOut: tc.hostName,
				StdErr: tc.stdErr,
				Err:    err,
			}

			host := helpers.BareMetalHost(
				"test-host",
				"default",
			)

			robotMock := robotmock.Client{}
			robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: true}, nil)

			service := newTestService(host, &robotMock, nil, nil, nil)

			isTimeout, isConnectionRefused, err := service.analyzeSSHOutputRegistering(out)
			Expect(isTimeout).To(Equal(false))
			Expect(isConnectionRefused).To(Equal(false))
			if tc.expectedErrMessage != "" {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(tc.expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry("stderr not empty", testCaseAnalyzeSSHOutputInstallImageStdErr{
			hasNilErr:          true,
			stdErr:             "command failed",
			hostName:           "hostName",
			expectedErrMessage: "failed to get hostname via ssh: StdErr:",
		}),
		Entry("stderr not empty - err != nil", testCaseAnalyzeSSHOutputInstallImageStdErr{
			hasNilErr:          false,
			stdErr:             "command failed",
			hostName:           "",
			expectedErrMessage: "unhandled ssh error while getting hostname",
		}),
		Entry("stderr not empty - wrong hostName", testCaseAnalyzeSSHOutputInstallImageStdErr{
			hasNilErr:          true,
			stdErr:             "command failed",
			hostName:           "",
			expectedErrMessage: "failed to get hostname via ssh: StdErr:",
		}),
		Entry("stderr empty - wrong hostName", testCaseAnalyzeSSHOutputInstallImageStdErr{
			hasNilErr:          true,
			stdErr:             "",
			hostName:           "",
			expectedErrMessage: "hostname is empty",
		}),
	)
})

var _ = Describe("analyzeSSHOutputInstallImage", func() {
	type testCaseAnalyzeSSHOutputInstallImageOutErr struct {
		err                         error
		errFromGetHostNameNil       bool
		port                        int
		expectedIsTimeout           bool
		expectedIsConnectionRefused bool
		expectedErrMessage          string
	}

	DescribeTable("analyzeSSHOutputInstallImage - out.Err",
		func(tc testCaseAnalyzeSSHOutputInstallImageOutErr) {
			sshMock := &sshmock.Client{}
			var errFromGetHostName error
			if !tc.errFromGetHostNameNil {
				errFromGetHostName = errTest
			}
			sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{Err: errFromGetHostName})

			isTimeout, isConnectionRefused, err := analyzeSSHOutputInstallImage(context.Background(), sshclient.Output{Err: tc.err}, sshMock, tc.port)
			Expect(isTimeout).To(Equal(tc.expectedIsTimeout))
			Expect(isConnectionRefused).To(Equal(tc.expectedIsConnectionRefused))
			if tc.expectedErrMessage != "" {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(tc.expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry("timeout error", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         timeout,
			errFromGetHostNameNil:       true,
			port:                        22,
			expectedIsTimeout:           true,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          "",
		}),
		Entry("authenticationFailed error, port 22, no hostName error", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         sshclient.ErrAuthenticationFailed,
			errFromGetHostNameNil:       true,
			port:                        22,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          "",
		}),
		Entry("authenticationFailed error, port 22, hostName error", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         sshclient.ErrAuthenticationFailed,
			errFromGetHostNameNil:       false,
			port:                        22,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          "wrong ssh key",
		}),
		Entry("authenticationFailed error, port != 22", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         sshclient.ErrAuthenticationFailed,
			errFromGetHostNameNil:       true,
			port:                        23,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          "wrong ssh key",
		}),
		Entry("connectionRefused error, port 22", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         syscall.ECONNREFUSED,
			errFromGetHostNameNil:       true,
			port:                        22,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: true,
			expectedErrMessage:          "",
		}),
		Entry("connectionRefused error, port != 22, hostname error", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         syscall.ECONNREFUSED,
			errFromGetHostNameNil:       false,
			port:                        23,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: true,
			expectedErrMessage:          "",
		}),
		Entry("connectionRefused error, port != 22, no hostname error", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         syscall.ECONNREFUSED,
			errFromGetHostNameNil:       true,
			port:                        23,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          "",
		}),
	)

	type testCaseAnalyzeSSHOutputInstallImageStdErr struct {
		hasNilErr          bool
		stdErr             string
		hasWrongHostName   bool
		expectedErrMessage string
	}

	DescribeTable("analyzeSSHOutputInstallImage - StdErr not empty",
		func(tc testCaseAnalyzeSSHOutputInstallImageStdErr) {
			var err error
			if !tc.hasNilErr {
				err = errTest
			}
			hostName := "rescue"
			if tc.hasWrongHostName {
				hostName = "wrongHostName"
			}

			out := sshclient.Output{
				StdOut: hostName,
				StdErr: tc.stdErr,
				Err:    err,
			}
			isTimeout, isConnectionRefused, err := analyzeSSHOutputInstallImage(context.Background(), out, nil, 22)
			Expect(isTimeout).To(Equal(false))
			Expect(isConnectionRefused).To(Equal(false))
			if tc.expectedErrMessage != "" {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(tc.expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry("stderr not empty", testCaseAnalyzeSSHOutputInstallImageStdErr{
			hasNilErr:          true,
			stdErr:             "command failed",
			hasWrongHostName:   false,
			expectedErrMessage: "failed to get hostname via ssh: StdErr:",
		}),
		Entry("stderr not empty - err != nil", testCaseAnalyzeSSHOutputInstallImageStdErr{
			hasNilErr:          false,
			stdErr:             "command failed",
			hasWrongHostName:   false,
			expectedErrMessage: "unhandled ssh error while getting hostname",
		}),
		Entry("stderr not empty - wrong hostName", testCaseAnalyzeSSHOutputInstallImageStdErr{
			hasNilErr:          true,
			stdErr:             "command failed",
			hasWrongHostName:   true,
			expectedErrMessage: "failed to get hostname via ssh: StdErr:",
		}),
	)

	type testCaseAnalyzeSSHOutputInstallImageWrongHostname struct {
		hasNilErr          bool
		stdErr             string
		hostName           string
		expectedErrMessage string
	}

	DescribeTable("analyzeSSHOutputInstallImage - wrong hostName",
		func(tc testCaseAnalyzeSSHOutputInstallImageWrongHostname) {
			var err error
			if !tc.hasNilErr {
				err = errTest
			}

			out := sshclient.Output{
				StdOut: tc.hostName,
				StdErr: tc.stdErr,
				Err:    err,
			}
			isTimeout, isConnectionRefused, err := analyzeSSHOutputInstallImage(context.Background(), out, nil, 22)
			Expect(isTimeout).To(Equal(false))
			Expect(isConnectionRefused).To(Equal(false))
			if tc.expectedErrMessage != "" {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(tc.expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry("empty hostname", testCaseAnalyzeSSHOutputInstallImageWrongHostname{
			hasNilErr:          true,
			stdErr:             "",
			hostName:           "",
			expectedErrMessage: "hostname is empty",
		}),
		Entry("empty hostname - err not empty", testCaseAnalyzeSSHOutputInstallImageWrongHostname{
			hasNilErr:          false,
			stdErr:             "",
			hostName:           "",
			expectedErrMessage: "unhandled ssh error while getting hostname",
		}),
		Entry("empty hostname stderr not empty", testCaseAnalyzeSSHOutputInstallImageWrongHostname{
			hasNilErr:          true,
			stdErr:             "command failed",
			hostName:           "",
			expectedErrMessage: "failed to get hostname via ssh: StdErr:",
		}),
		Entry("hostname == rescue", testCaseAnalyzeSSHOutputInstallImageWrongHostname{
			hasNilErr:          true,
			stdErr:             "",
			hostName:           "rescue",
			expectedErrMessage: "",
		}),
		Entry("hostname == otherHostName", testCaseAnalyzeSSHOutputInstallImageWrongHostname{
			hasNilErr:          true,
			stdErr:             "",
			hostName:           "otherHostName",
			expectedErrMessage: "unexpected hostname",
		}),
	)
})

var _ = Describe("analyzeSSHOutputProvisioned", func() {
	type testCaseAnalyzeSSHOutputProvisioned struct {
		out                         sshclient.Output
		expectedIsTimeout           bool
		expectedIsConnectionRefused bool
		expectedErrMessage          *string
	}

	DescribeTable("analyzeSSHOutputProvisioned",
		func(tc testCaseAnalyzeSSHOutputProvisioned) {
			isTimeout, isConnectionRefused, err := analyzeSSHOutputProvisioned(tc.out)
			Expect(isTimeout).To(Equal(tc.expectedIsTimeout))
			Expect(isConnectionRefused).To(Equal(tc.expectedIsConnectionRefused))
			if tc.expectedErrMessage != nil {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(*tc.expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry("incorrect boot", testCaseAnalyzeSSHOutputProvisioned{
			out:                         sshclient.Output{StdOut: "wrong_hostname"},
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          ptr.To("unexpected hostname"),
		}),
		Entry("timeout error", testCaseAnalyzeSSHOutputProvisioned{
			out:                         sshclient.Output{Err: timeout},
			expectedIsTimeout:           true,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          nil,
		}),
		Entry("stdErr non-empty", testCaseAnalyzeSSHOutputProvisioned{
			out:                         sshclient.Output{StdErr: "some error"},
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          ptr.To("failed to get hostname via ssh: StdErr: some error"),
		}),
		Entry("incorrect boot - empty hostname", testCaseAnalyzeSSHOutputProvisioned{
			out:                         sshclient.Output{StdOut: ""},
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          ptr.To("hostname is empty"),
		}),
		Entry("unable to authenticate", testCaseAnalyzeSSHOutputProvisioned{
			out:                         sshclient.Output{Err: sshclient.ErrAuthenticationFailed},
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          ptr.To("wrong ssh key"),
		}),
		Entry("connection refused", testCaseAnalyzeSSHOutputProvisioned{
			out:                         sshclient.Output{Err: syscall.ECONNREFUSED},
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: true,
			expectedErrMessage:          nil,
		}),
	)
})

var _ = Describe("actionRegistering", func() {
	type testCaseActionRegistering struct {
		storageStdOut             string
		includeRootDeviceHintWWN  bool
		includeRootDeviceHintRaid bool
		expectedActionResult      actionResult
		expectedErrorMessage      *string
		swRaid                    bool
		customProvisioner         bool
	}
	ctx := context.Background()
	DescribeTable("actionRegistering",
		func(tc testCaseActionRegistering) {
			var host *infrav2.HetznerBareMetalHost
			if tc.includeRootDeviceHintWWN {
				host = helpers.BareMetalHost(
					"test-host",
					"default",
					helpers.WithRootDeviceHintWWN(),
					helpers.WithIPv4(),
					helpers.WithConsumerRef(),
				)
			} else if tc.includeRootDeviceHintRaid {
				host = helpers.BareMetalHost(
					"test-host",
					"default",
					helpers.WithRootDeviceHintRaid(),
					helpers.WithIPv4(),
					helpers.WithConsumerRef(),
				)
			} else {
				host = helpers.BareMetalHost(
					"test-host",
					"default",
					helpers.WithIPv4(),
					helpers.WithConsumerRef(),
				)
			}
			sshMock := registeringSSHMock(tc.storageStdOut)
			service := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
			if tc.customProvisioner {
				service.scope.HetznerBareMetalMachine.Spec.InstallImage = nil
				service.scope.HetznerBareMetalMachine.Spec.CustomProvisioner = &infrav2.CustomProvisioner{Swraid: 1}
			} else if tc.swRaid {
				service.scope.HetznerBareMetalMachine.Spec.InstallImage = &infrav2.InstallImage{Swraid: 1}
			}

			actResult := service.actionRegistering(ctx)
			Expect(host.Status.HardwareDetails).ToNot(BeNil())
			if tc.expectedErrorMessage != nil {
				Expect(host.Status.ErrorType).To(BeEmpty())
				Expect(conditions.Get(host, infrav2.HetznerBareMetalHostActionCompletedCondition)).To(BeNil())
				rootDeviceHintsValidated := conditions.Get(host, infrav2.HetznerBareMetalHostRootDeviceHintsValidatedCondition)
				Expect(rootDeviceHintsValidated).ToNot(BeNil())
				Expect(rootDeviceHintsValidated.Status).To(Equal(metav1.ConditionFalse))
				Expect(rootDeviceHintsValidated.Reason).To(Equal(infrav2.HetznerBareMetalHostRootDeviceHintsValidationFailedReason))
				Expect(rootDeviceHintsValidated.Message).To(Equal(*tc.expectedErrorMessage))
			}
			if _, ok := tc.expectedActionResult.(actionComplete); ok {
				Expect(host.Status.ErrorType).To(BeEmpty())
			}
			Expect(actResult).Should(BeAssignableToTypeOf(tc.expectedActionResult))
		},
		Entry("working example", testCaseActionRegistering{
			storageStdOut: `NAME="loop0" LABEL="" FSTYPE="ext2" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
		NAME="nvme2n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee8" ROTA="0"
		NAME="nvme1n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			includeRootDeviceHintWWN:  true,
			includeRootDeviceHintRaid: false,
			expectedActionResult:      actionComplete{},
			expectedErrorMessage:      nil,
		}),
		Entry("working example - rootDeviceHints raid", testCaseActionRegistering{
			storageStdOut: `NAME="loop0" LABEL="" FSTYPE="ext2" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
		NAME="nvme2n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee8" ROTA="0"
		NAME="nvme1n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			includeRootDeviceHintWWN:  false,
			includeRootDeviceHintRaid: true,
			expectedActionResult:      actionComplete{},
			expectedErrorMessage:      nil,
			swRaid:                    true,
		}),
		Entry("working example - rootDeviceHints raid with a custom provisioner", testCaseActionRegistering{
			storageStdOut: `NAME="loop0" LABEL="" FSTYPE="ext2" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
		NAME="nvme2n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee8" ROTA="0"
		NAME="nvme1n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			includeRootDeviceHintWWN:  false,
			includeRootDeviceHintRaid: true,
			expectedActionResult:      actionComplete{},
			expectedErrorMessage:      nil,
			swRaid:                    true,
			customProvisioner:         true,
		}),
		Entry("wwn does not fit to storage devices", testCaseActionRegistering{
			storageStdOut: `NAME="loop0" LABEL="" FSTYPE="ext2" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
			NAME="nvme2n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee2" ROTA="0"
			NAME="nvme1n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			includeRootDeviceHintWWN:  true,
			includeRootDeviceHintRaid: false,
			// The user has to correct spec.rootDeviceHints. We reconcile again when the spec
			// changes, so there is no requeue.
			expectedActionResult: actionStop{},
			expectedErrorMessage: ptr.To(`missing storage device for root device hint "eui.002538b411b2cee8". Known WWNs: [eui.002538b411b2cee2 eui.0025388801b4dff2]`),
		}),
		Entry("no root device hints", testCaseActionRegistering{
			storageStdOut: `NAME="loop0" LABEL="" FSTYPE="ext2" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
			NAME="nvme2n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee2" ROTA="0"
			NAME="nvme1n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			includeRootDeviceHintWWN:  false,
			includeRootDeviceHintRaid: false,
			// The user has to set spec.rootDeviceHints. We reconcile again when the spec changes,
			// so there is no requeue.
			expectedActionResult: actionStop{},
			expectedErrorMessage: ptr.To(infrav2.ErrorMessageMissingRootDeviceHints),
		}),
	)

	type testCaseActionRegisteringIncompleteBoot struct {
		getHostNameOutput         sshclient.Output
		expectedOngoingRebootType infrav2.RebootType
	}

	DescribeTable("actionRegistering - incomplete reboot",
		func(tc testCaseActionRegisteringIncompleteBoot) {
			host := helpers.BareMetalHost(
				"test-host",
				"default",
				helpers.WithRebootTypes([]infrav2.RebootType{infrav2.RebootTypeHardware}),
				helpers.WithRootDeviceHintWWN(),
				helpers.WithIPv4(),
				helpers.WithConsumerRef(),
			)

			sshMock := &sshmock.Client{}
			sshMock.On("GetHostName", mock.Anything).Return(tc.getHostNameOutput)

			robotMock := robotmock.Client{}
			robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: false}, nil)

			service := newTestService(host, &robotMock, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

			actResult := service.actionRegistering(ctx)
			Expect(actResult).Should(BeAssignableToTypeOf(actionContinue{}))
			if tc.expectedOngoingRebootType == "" {
				Expect(host.Status.OngoingReboot).To(BeNil())
			} else {
				Expect(host.Status.OngoingReboot).ToNot(BeNil())
				Expect(host.Status.OngoingReboot.Type).To(Equal(tc.expectedOngoingRebootType))
			}
		},
		Entry("timeout", testCaseActionRegisteringIncompleteBoot{
			getHostNameOutput:         sshclient.Output{Err: timeout},
			expectedOngoingRebootType: infrav2.RebootTypeSSH,
		}),
		Entry("connectionRefused", testCaseActionRegisteringIncompleteBoot{
			getHostNameOutput:         sshclient.Output{Err: syscall.ECONNREFUSED},
			expectedOngoingRebootType: infrav2.RebootType(""),
		}),
	)

	It("sets a fatal error when the reboot into rescue times out", func() {
		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithRootDeviceHintWWN(),
			helpers.WithIPv4(),
			helpers.WithConsumerRef(),
			helpers.WithOngoingReboot(infrav2.RebootTypeHardware, metav1.NewTime(time.Now().Add(-time.Hour))),
		)

		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{Err: timeout})

		service := newTestService(host, &robotmock.Client{}, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

		actResult := service.actionRegistering(ctx)

		Expect(actResult).To(BeAssignableToTypeOf(actionStop{}))
		Expect(host.Status.ErrorType).To(Equal(infrav2.ErrorTypeFatal))
		ac := conditions.Get(host, infrav2.HetznerBareMetalHostActionCompletedCondition)
		Expect(ac).NotTo(BeNil())
		Expect(ac.Message).To(ContainSubstring("hardware reboot (to rescue mode) timed out"))

		acV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.ActionCompletedV1Beta1Condition)
		Expect(acV1Beta1).NotTo(BeNil())
		Expect(acV1Beta1.Message).To(ContainSubstring("hardware reboot (to rescue mode) timed out"))
	})

	// SetError clears the ongoing reboot. This checks that the next reconcile of a host with a fatal
	// error returns actionStop without sending a reboot.
	It("does not send a reboot when the host already has a fatal error", func() {
		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithRootDeviceHintWWN(),
			helpers.WithIPv4(),
			helpers.WithConsumerRef(),
			helpers.WithError(infrav2.ErrorTypeFatal, "hardware reboot (to rescue mode) timed out"),
		)

		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{StdOut: "not-rescue"})
		robotMock := &robotmock.Client{}

		service := newTestService(host, robotMock, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

		Expect(service.actionRegistering(ctx)).To(BeAssignableToTypeOf(actionStop{}))

		// errorType is still fatal error and RebootBMServer was not called
		Expect(host.Status.ErrorType).To(Equal(infrav2.ErrorTypeFatal))
		Expect(host.Status.OngoingReboot).To(BeNil())
		robotMock.AssertNotCalled(GinkgoT(), "RebootBMServer", mock.Anything, mock.Anything)
	})
})

func registeringSSHMock(storageStdOut string) *sshmock.Client {
	sshMock := &sshmock.Client{}
	sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{StdOut: "rescue"})
	sshMock.On("GetHardwareDetailsRAM", mock.Anything).Return(sshclient.Output{StdOut: "10000"})
	sshMock.On("GetHardwareDetailsStorage", mock.Anything).Return(sshclient.Output{
		StdOut: storageStdOut,
	})
	sshMock.On("GetHardwareDetailsNics", mock.Anything).Return(sshclient.Output{
		StdOut: `name="eth0" model="Realtek Semiconductor Co., Ltd. RTL8111/8168/8411 PCI Express Gigabit Ethernet Controller (rev 15)" mac="a8:a1:59:94:19:42" ipv4="23.88.6.239/26" speedMbps="1000"
name="eth0" model="Realtek Semiconductor Co., Ltd. RTL8111/8168/8411 PCI Express Gigabit Ethernet Controller (rev 15)" mac="a8:a1:59:94:19:42" ip="2a01:4f8:272:3e0f::2/64" speedMbps="1000"`,
	})
	sshMock.On("GetHardwareDetailsCPUArch", mock.Anything).Return(sshclient.Output{StdOut: "myarch"})
	sshMock.On("GetHardwareDetailsCPUModel", mock.Anything).Return(sshclient.Output{StdOut: "mymodel"})
	sshMock.On("GetHardwareDetailsCPUClockGigahertz", mock.Anything).Return(sshclient.Output{StdOut: "42654"})
	sshMock.On("GetHardwareDetailsCPUThreads", mock.Anything).Return(sshclient.Output{StdOut: "123"})
	sshMock.On("GetHardwareDetailsCPUCores", mock.Anything).Return(sshclient.Output{StdOut: "12"})
	sshMock.On("GetHardwareDetailsDebug", mock.Anything).Return(sshclient.Output{StdOut: "Dummy output"})
	return sshMock
}

var _ = Describe("actionRegistering check RAID", func() {
	ctx := context.Background()
	It("check RAID", func() {
		sshMock := registeringSSHMock(`NAME="nvme2n1" TYPE="disk" MODEL="mymode." VENDOR="" SIZE="3068773888" WWN="wwn1" ROTA="0"
		NAME="nvme2n2" TYPE="disk" MODEL="mymodel" VENDOR="" SIZE="3068773888" WWN="wwn2" ROTA="0"`)
		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithRootDeviceHintWWN(),
			helpers.WithConsumerRef(),
		)
		host.Spec.RootDeviceHints.WWN = "wwn1"
		service := newTestService(host, nil, bmmock.NewSSHFactory(
			sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
		service.scope.HetznerBareMetalMachine.Spec.InstallImage = &infrav2.InstallImage{
			Swraid: 1,
		}
		actResult := service.actionRegistering(ctx)

		_, err := actResult.Result()
		Expect(err).Should(BeNil())
		Expect(host.Status.ErrorType).To(Equal(infrav2.ErrorTypeFatal))

		service.scope.HetznerBareMetalMachine.Spec.InstallImage.Swraid = 0
		host.Spec.RootDeviceHints.WWN = ""
		host.Spec.RootDeviceHints.Raid.WWN = []string{"wwn1", "wwn2"}
		actResult = service.actionRegistering(ctx)

		_, err = actResult.Result()
		Expect(err).Should(BeNil())
		Expect(host.Status.ErrorType).To(Equal(infrav2.ErrorTypeFatal))
	})
})

var _ = Describe("actionRegistering emits event on hardwareDetails change", func() {
	const storageStdOut = `NAME="nvme2n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee8" ROTA="0"
NAME="nvme1n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`

	ctx := context.Background()

	It("emits HardwareDetails Changed event when existing details differ", func() {
		// drain events from previous tests
		for len(testEventRecorder.Events) > 0 {
			<-testEventRecorder.Events
		}

		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithRootDeviceHintWWN(),
			helpers.WithIPv4(),
			helpers.WithConsumerRef(),
		)
		host.Status.HardwareDetails = &infrav2.HardwareDetails{
			CPU: infrav2.CPU{Model: "old-model"},
		}

		sshMock := registeringSSHMock(storageStdOut)
		service := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

		service.actionRegistering(ctx)

		var events []string
		for len(testEventRecorder.Events) > 0 {
			events = append(events, <-testEventRecorder.Events)
		}
		Expect(events).To(ContainElement(ContainSubstring("HardwareDetails changed")))
	})

	It("invalidates RootDeviceHints in cases where a hardware change leads to different wwns", func() {
		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithRootDeviceHintWWN(),
			helpers.WithIPv4(),
			helpers.WithConsumerRef(),
		)
		// The previously read hardware had a disk matching the configured root device hint WWN.
		host.Status.HardwareDetails = &infrav2.HardwareDetails{
			Storage: []infrav2.Storage{
				{WWN: helpers.DefaultWWN},
			},
		}

		// The disk with the WWN referenced by RootDeviceHints is gone, e.g. because it was replaced.
		const newStorageStdOut = `NAME="nvme2n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee2" ROTA="0"
NAME="nvme1n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`

		sshMock := registeringSSHMock(newStorageStdOut)
		service := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

		actResult := service.actionRegistering(ctx)

		// The user has to point spec.rootDeviceHints at one of the new wwns. We reconcile again
		// when the spec changes, so there is no requeue.
		Expect(actResult).To(BeAssignableToTypeOf(actionStop{}))
		c := conditions.Get(host, infrav2.HetznerBareMetalHostRootDeviceHintsValidatedCondition)
		Expect(c.Message).To(ContainSubstring("missing storage device for root device hint"))

		cV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.RootDeviceHintsValidatedV1Beta1Condition)
		Expect(cV1Beta1).ToNot(BeNil())
		Expect(cV1Beta1.Message).To(ContainSubstring("missing storage device for root device hint"))

		// Even though the action did not complete, the freshly read hardware details must be
		// persisted, so the object still reports the new storage layout.
		Expect(host.Status.HardwareDetails).ToNot(BeNil())
		Expect(host.Status.HardwareDetails.Storage).To(ConsistOf(
			infrav2.Storage{Model: "SAMSUNG MZVL22T0HBLB-00B00", SerialNumber: "S677NF0R402742", SizeBytes: 2048408248320, SizeGB: 2048, WWN: "eui.002538b411b2cee2"},
			infrav2.Storage{Model: "SAMSUNG MZVLB512HAJQ-00000", SerialNumber: "S3W8NX0N811178", SizeBytes: 512110190592, SizeGB: 512, WWN: "eui.0025388801b4dff2"},
		))
	})
})

var _ = Describe("getImageDetails", func() {
	type testCaseGetImageDetails struct {
		image                 infrav2.Image
		expectedImagePath     string
		expectedNeedsDownload bool
		expectedErrorMessage  string
	}

	DescribeTable("getImageDetails",
		func(tc testCaseGetImageDetails) {
			imagePath, needsDownload, errorMessage := tc.image.GetDetails()
			Expect(imagePath).To(Equal(tc.expectedImagePath))
			Expect(needsDownload).To(Equal(tc.expectedNeedsDownload))
			Expect(errorMessage).To(Equal(tc.expectedErrorMessage))
		},
		Entry("name and url specified, tar.gz suffix", testCaseGetImageDetails{
			image: infrav2.Image{
				Name: "imageName",
				URL:  "https://mytargz.tar.gz",
				Path: "",
			},
			expectedImagePath:     "/root/imageName.tar.gz",
			expectedNeedsDownload: true,
			expectedErrorMessage:  "",
		}),
		Entry("name and url specified, tgz suffix", testCaseGetImageDetails{
			image: infrav2.Image{
				Name: "imageName",
				URL:  "https://mytargz.tgz",
				Path: "",
			},
			expectedImagePath:     "/root/imageName.tgz",
			expectedNeedsDownload: true,
			expectedErrorMessage:  "",
		}),
		Entry("name and url specified, wrong suffix", testCaseGetImageDetails{
			image: infrav2.Image{
				Name: "imageName",
				URL:  "https://mytargz.tgx",
				Path: "",
			},
			expectedImagePath:     "",
			expectedNeedsDownload: false,
			expectedErrorMessage:  "wrong image url suffix",
		}),
		Entry("path specified", testCaseGetImageDetails{
			image: infrav2.Image{
				Name: "",
				URL:  "",
				Path: "imagePath",
			},
			expectedImagePath:     "imagePath",
			expectedNeedsDownload: false,
			expectedErrorMessage:  "",
		}),
		Entry("neither specified", testCaseGetImageDetails{
			image: infrav2.Image{
				Name: "imageName",
				URL:  "",
				Path: "",
			},
			expectedImagePath:     "",
			expectedNeedsDownload: false,
			expectedErrorMessage:  "invalid image - need to specify either name and url or path",
		}),
	)
})

var _ = Describe("actionEnsureProvisioned", func() {
	type testCaseActionEnsureProvisioned struct {
		outSSHClientGetHostName                sshclient.Output
		outSSHClientCloudInitStatus            sshclient.Output
		outSSHClientCheckSigterm               sshclient.Output
		outOldSSHClientCloudInitStatus         sshclient.Output
		outOldSSHClientCheckSigterm            sshclient.Output
		expectedActionResult                   actionResult
		expectedErrorType                      infrav2.ErrorType
		expectedOngoingRebootType              infrav2.RebootType
		expectsSSHClientCallCloudInitStatus    bool
		expectsSSHClientCallCheckSigterm       bool
		expectsSSHClientCallReboot             bool
		expectsOldSSHClientCallCloudInitStatus bool
		expectsOldSSHClientCallCheckSigterm    bool
		expectsOldSSHClientCallReboot          bool
	}

	DescribeTable("actionEnsureProvisioned",
		func(in testCaseActionEnsureProvisioned) {
			ctx := context.Background()
			portAfterInstallImage := 24

			host := helpers.BareMetalHost(
				"test-host",
				"default",
				helpers.WithIPv4(),
				helpers.WithConsumerRef(),
			)
			sshMock := &sshmock.Client{}
			sshMock.On("GetHostName", mock.Anything).Return(in.outSSHClientGetHostName)
			sshMock.On("CloudInitStatus", mock.Anything).Return(in.outSSHClientCloudInitStatus)
			sshMock.On("CheckCloudInitLogsForSigTerm", mock.Anything).Return(in.outSSHClientCheckSigterm)
			sshMock.On("CleanCloudInitLogs", mock.Anything).Return(sshclient.Output{})
			sshMock.On("CleanCloudInitInstances", mock.Anything).Return(sshclient.Output{})
			sshMock.On("Reboot", mock.Anything).Return(sshclient.Output{})
			sshMock.On("GetCloudInitOutput", mock.Anything).Return(sshclient.Output{StdOut: "dummy content of /var/log/cloud-init-output.log"})

			oldSSHMock := &sshmock.Client{}
			oldSSHMock.On("CloudInitStatus", mock.Anything).Return(in.outOldSSHClientCloudInitStatus)
			oldSSHMock.On("CheckCloudInitLogsForSigTerm", mock.Anything).Return(in.outOldSSHClientCheckSigterm)
			oldSSHMock.On("CleanCloudInitLogs", mock.Anything).Return(sshclient.Output{})
			oldSSHMock.On("CleanCloudInitInstances", mock.Anything).Return(sshclient.Output{})
			oldSSHMock.On("Reboot", mock.Anything).Return(sshclient.Output{})

			robotMock := robotmock.Client{}
			robotMock.On("SetBMServerName", mock.Anything, infrav2.BareMetalHostNamePrefix+host.Spec.ConsumerRef.Name).Return(nil, nil)

			service := newTestService(host, &robotMock, bmmock.NewSSHFactory(sshMock, oldSSHMock, sshMock), helpers.GetDefaultSSHSecret(osSSHKeyName, "default"), nil)
			service.scope.HetznerBareMetalMachine.Spec.SSHSpec.PortAfterInstallImage = portAfterInstallImage

			actResult := service.actionEnsureProvisioned(ctx)
			Expect(actResult).Should(BeAssignableToTypeOf(in.expectedActionResult))
			Expect(host.Status.ErrorType).To(Equal(in.expectedErrorType))
			if in.expectedOngoingRebootType == "" {
				Expect(host.Status.OngoingReboot).To(BeNil())
			} else {
				Expect(host.Status.OngoingReboot).ToNot(BeNil())
				Expect(host.Status.OngoingReboot.Type).To(Equal(in.expectedOngoingRebootType))
			}
			if in.expectsSSHClientCallCloudInitStatus {
				Expect(sshMock.AssertCalled(GinkgoT(), "CloudInitStatus", mock.Anything)).To(BeTrue())
			} else {
				Expect(sshMock.AssertNotCalled(GinkgoT(), "CloudInitStatus", mock.Anything)).To(BeTrue())
			}
			if in.expectsSSHClientCallCheckSigterm {
				Expect(sshMock.AssertCalled(GinkgoT(), "CheckCloudInitLogsForSigTerm", mock.Anything)).To(BeTrue())
			} else {
				Expect(sshMock.AssertNotCalled(GinkgoT(), "CheckCloudInitLogsForSigTerm", mock.Anything)).To(BeTrue())
			}
			if in.expectsSSHClientCallReboot {
				Expect(sshMock.AssertCalled(GinkgoT(), "Reboot", mock.Anything)).To(BeTrue())
			} else {
				Expect(sshMock.AssertNotCalled(GinkgoT(), "Reboot", mock.Anything)).To(BeTrue())
			}
			if in.expectsOldSSHClientCallCloudInitStatus {
				Expect(oldSSHMock.AssertCalled(GinkgoT(), "CloudInitStatus", mock.Anything)).To(BeTrue())
			} else {
				Expect(oldSSHMock.AssertNotCalled(GinkgoT(), "CloudInitStatus", mock.Anything)).To(BeTrue())
			}
			if in.expectsOldSSHClientCallCheckSigterm {
				Expect(oldSSHMock.AssertCalled(GinkgoT(), "CheckCloudInitLogsForSigTerm", mock.Anything)).To(BeTrue())
			} else {
				Expect(oldSSHMock.AssertNotCalled(GinkgoT(), "CheckCloudInitLogsForSigTerm", mock.Anything)).To(BeTrue())
			}
			if in.expectsOldSSHClientCallReboot {
				Expect(oldSSHMock.AssertCalled(GinkgoT(), "Reboot", mock.Anything)).To(BeTrue())
			} else {
				Expect(oldSSHMock.AssertNotCalled(GinkgoT(), "Reboot", mock.Anything)).To(BeTrue())
			}
		},
		Entry("correct hostname, cloud init running",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{StdOut: infrav2.BareMetalHostNamePrefix + "bm-machine"},
				outSSHClientCloudInitStatus:            sshclient.Output{StdOut: "status: running"},
				outSSHClientCheckSigterm:               sshclient.Output{},
				outOldSSHClientCloudInitStatus:         sshclient.Output{},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				expectedActionResult:                   actionContinue{},
				expectedErrorType:                      infrav2.ErrorType(""),
				expectsSSHClientCallCloudInitStatus:    true,
				expectsSSHClientCallCheckSigterm:       false,
				expectsSSHClientCallReboot:             false,
				expectsOldSSHClientCallCloudInitStatus: false,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
		Entry("correct hostname, cloud init done, no SIGTERM",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{StdOut: infrav2.BareMetalHostNamePrefix + "bm-machine"},
				outSSHClientCloudInitStatus:            sshclient.Output{StdOut: "status: done"},
				outSSHClientCheckSigterm:               sshclient.Output{StdOut: ""},
				outOldSSHClientCloudInitStatus:         sshclient.Output{},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				expectedActionResult:                   actionComplete{},
				expectedErrorType:                      infrav2.ErrorType(""),
				expectsSSHClientCallCloudInitStatus:    true,
				expectsSSHClientCallCheckSigterm:       true,
				expectsSSHClientCallReboot:             false,
				expectsOldSSHClientCallCloudInitStatus: false,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
		Entry("correct hostname, cloud init done, SIGTERM",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{StdOut: infrav2.BareMetalHostNamePrefix + "bm-machine"},
				outSSHClientCloudInitStatus:            sshclient.Output{StdOut: "status: done"},
				outSSHClientCheckSigterm:               sshclient.Output{StdOut: "found SIGTERM in cloud init output logs"},
				outOldSSHClientCloudInitStatus:         sshclient.Output{},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				expectedActionResult:                   actionContinue{},
				expectedErrorType:                      infrav2.ErrorType(""),
				expectedOngoingRebootType:              infrav2.RebootTypeSSH,
				expectsSSHClientCallCloudInitStatus:    true,
				expectsSSHClientCallCheckSigterm:       true,
				expectsSSHClientCallReboot:             true,
				expectsOldSSHClientCallCloudInitStatus: false,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
		Entry("correct hostname, cloud init error",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{StdOut: infrav2.BareMetalHostNamePrefix + "bm-machine"},
				outSSHClientCloudInitStatus:            sshclient.Output{StdOut: "status: error"},
				outSSHClientCheckSigterm:               sshclient.Output{},
				outOldSSHClientCloudInitStatus:         sshclient.Output{},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				expectedActionResult:                   actionStop{},
				expectedErrorType:                      infrav2.ErrorTypeFatal,
				expectsSSHClientCallCloudInitStatus:    true,
				expectsSSHClientCallCheckSigterm:       false,
				expectsSSHClientCallReboot:             false,
				expectsOldSSHClientCallCloudInitStatus: false,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
		Entry("correct hostname, cloud init disabled",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{StdOut: infrav2.BareMetalHostNamePrefix + "bm-machine"},
				outSSHClientCloudInitStatus:            sshclient.Output{StdOut: "status: disabled"},
				outSSHClientCheckSigterm:               sshclient.Output{},
				outOldSSHClientCloudInitStatus:         sshclient.Output{},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				expectedActionResult:                   actionContinue{},
				expectedErrorType:                      infrav2.ErrorType(""),
				expectedOngoingRebootType:              infrav2.RebootTypeSSH,
				expectsSSHClientCallCloudInitStatus:    true,
				expectsSSHClientCallCheckSigterm:       false,
				expectsSSHClientCallReboot:             true,
				expectsOldSSHClientCallCloudInitStatus: false,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
		Entry("connectionFailed, same ports",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{Err: syscall.ECONNREFUSED},
				outSSHClientCloudInitStatus:            sshclient.Output{},
				outSSHClientCheckSigterm:               sshclient.Output{},
				outOldSSHClientCloudInitStatus:         sshclient.Output{},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				expectedActionResult:                   actionContinue{},
				expectedErrorType:                      infrav2.ErrorType(""),
				expectsSSHClientCallCloudInitStatus:    false,
				expectsSSHClientCallCheckSigterm:       false,
				expectsSSHClientCallReboot:             false,
				expectsOldSSHClientCallCloudInitStatus: false,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
	)

	It("records the CloudInitOutput event when the cloud-init status check returned an error", func() {
		ctx := context.Background()

		// drain events from previous tests
		for len(testEventRecorder.Events) > 0 {
			<-testEventRecorder.Events
		}

		portAfterInstallImage := 24
		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithIPv4(),
			helpers.WithConsumerRef(),
		)

		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{StdOut: infrav2.BareMetalHostNamePrefix + "bm-machine"})
		// an unknown cloud-init status makes checkCloudInitStatus return an error
		sshMock.On("CloudInitStatus", mock.Anything).Return(sshclient.Output{StdOut: "status: broken"})
		sshMock.On("GetCloudInitOutput", mock.Anything).Return(sshclient.Output{StdOut: "dummy content of /var/log/cloud-init-output.log"})

		robotMock := robotmock.Client{}
		robotMock.On("SetBMServerName", mock.Anything, infrav2.BareMetalHostNamePrefix+host.Spec.ConsumerRef.Name).Return(nil, nil)

		service := newTestService(host, &robotMock, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), helpers.GetDefaultSSHSecret(osSSHKeyName, "default"), nil)
		service.scope.HetznerBareMetalMachine.Spec.SSHSpec.PortAfterInstallImage = portAfterInstallImage

		// the error is still returned, and the cloud-init output is recorded in an event
		Expect(service.actionEnsureProvisioned(ctx)).To(BeAssignableToTypeOf(actionError{}))

		var events []string
		for len(testEventRecorder.Events) > 0 {
			events = append(events, <-testEventRecorder.Events)
		}
		Expect(events).To(ContainElement(ContainSubstring("dummy content of /var/log/cloud-init-output.log")))
		Expect(events).NotTo(ContainElement(ContainSubstring("GetCloudInitOutputFailed")))
	})

	It("sets a fatal error when the reboot into the OS times out", func() {
		ctx := context.Background()
		portAfterInstallImage := 24

		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithIPv4(),
			helpers.WithConsumerRef(),
			helpers.WithOngoingReboot(infrav2.RebootTypeHardware, metav1.NewTime(time.Now().Add(-time.Hour))),
		)

		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{Err: timeout})

		service := newTestService(host, &robotmock.Client{}, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), helpers.GetDefaultSSHSecret(osSSHKeyName, "default"), nil)
		service.scope.HetznerBareMetalMachine.Spec.SSHSpec.PortAfterInstallImage = portAfterInstallImage

		actResult := service.actionEnsureProvisioned(ctx)

		Expect(actResult).To(BeAssignableToTypeOf(actionStop{}))
		Expect(host.Status.ErrorType).To(Equal(infrav2.ErrorTypeFatal))
		ac := conditions.Get(host, infrav2.HetznerBareMetalHostActionCompletedCondition)
		Expect(ac).NotTo(BeNil())
		Expect(ac.Message).To(ContainSubstring("hardware reboot (to node) timed out"))

		acV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.ActionCompletedV1Beta1Condition)
		Expect(acV1Beta1).NotTo(BeNil())
		Expect(acV1Beta1.Message).To(ContainSubstring("hardware reboot (to node) timed out"))
	})

	It("reports the host as still provisioning while the connection refused timeout has not passed", func() {
		ctx := context.Background()
		portAfterInstallImage := 24

		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithIPv4(),
			helpers.WithConsumerRef(),
			helpers.WithOngoingReboot(infrav2.RebootTypeSoftware, metav1.NewTime(time.Now().Add(-time.Minute))),
		)

		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{Err: syscall.ECONNREFUSED})

		service := newTestService(host, &robotmock.Client{}, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), helpers.GetDefaultSSHSecret(osSSHKeyName, "default"), nil)
		service.scope.HetznerBareMetalMachine.Spec.SSHSpec.PortAfterInstallImage = portAfterInstallImage

		Expect(service.actionEnsureProvisioned(ctx)).To(BeAssignableToTypeOf(actionContinue{}))
		Expect(host.Status.OngoingReboot).ToNot(BeNil())
		Expect(host.Status.OngoingReboot.Type).To(Equal(infrav2.RebootTypeSoftware))

		conditionV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.ProvisionSucceededV1Beta1Condition)
		Expect(conditionV1Beta1).ToNot(BeNil())
		Expect(conditionV1Beta1.Reason).To(Equal(infrav2.StillProvisioningV1Beta1Reason))

		condition := conditions.Get(host, infrav2.HetznerBareMetalHostProvisionSucceededCondition)
		Expect(condition).ToNot(BeNil())
		Expect(condition.Reason).To(Equal(infrav2.HetznerBareMetalHostProvisioningReason))
	})

	It("keeps failing with the same fatal error when the provisioned server keeps refusing the ssh connection", func() {
		ctx := context.Background()
		portAfterInstallImage := 24

		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithIPv4(),
			helpers.WithConsumerRef(),
			helpers.WithOngoingReboot(infrav2.RebootTypeSoftware, metav1.NewTime(time.Now().Add(-time.Hour))),
		)

		sshMock := &sshmock.Client{}
		sshMock.On("GetHostName", mock.Anything).Return(sshclient.Output{Err: syscall.ECONNREFUSED})

		service := newTestService(host, &robotmock.Client{}, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), helpers.GetDefaultSSHSecret(osSSHKeyName, "default"), nil)
		service.scope.HetznerBareMetalMachine.Spec.SSHSpec.PortAfterInstallImage = portAfterInstallImage

		for run := 1; run <= 3; run++ {
			Expect(service.actionEnsureProvisioned(ctx)).To(BeAssignableToTypeOf(actionStop{}), "run %d", run)
			Expect(host.Status.ErrorType).To(Equal(infrav2.ErrorTypeFatal), "run %d", run)

			conditionV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.ProvisionSucceededV1Beta1Condition)
			Expect(conditionV1Beta1).ToNot(BeNil(), "run %d", run)
			Expect(conditionV1Beta1.Reason).To(Equal(infrav2.SSHConnectionRefusedV1Beta1Reason), "run %d", run)
			Expect(conditionV1Beta1.Message).To(ContainSubstring("wrong ssh port"), "run %d", run)

			condition := conditions.Get(host, infrav2.HetznerBareMetalHostProvisionSucceededCondition)
			Expect(condition).ToNot(BeNil(), "run %d", run)
			Expect(condition.Reason).To(Equal(infrav2.HetznerBareMetalHostSSHConnectionRefusedReason), "run %d", run)
			Expect(condition.Message).To(ContainSubstring("wrong ssh port"), "run %d", run)
		}
	})
})

var _ = Describe("actionProvisioned NoSSHAfterInstallImage=false", func() {
	type testCaseActionProvisioned struct {
		shouldHaveRebootAnnotation bool
		rebootFinished             bool
		rebooted                   bool
		// storedBootID sets RebootAnnotationNodeBootID in host status.
		// Empty (default) means Phase 1 has not run yet — Phase 1 will execute.
		// Non-empty means Phase 1 already ran — Phase 2 will execute.
		// Set to a value different from fakeBootID to simulate a completed BootID change.
		storedBootID                string
		expectedActionResult        actionResult
		expectRebootAnnotation      bool
		expectRebootInStatus        bool
		expectOngoingRebootInStatus bool
		expectedNodeBootID          string
	}

	DescribeTable("actionProvisioned",
		func(tc testCaseActionProvisioned) {
			ctx := context.Background()
			host := helpers.BareMetalHost(
				"test-host",
				"default",
				helpers.WithIPv4(),
				helpers.WithConsumerRef(),
			)
			if tc.shouldHaveRebootAnnotation {
				host.SetAnnotations(map[string]string{infrav2.RebootAnnotation: "reboot"})
				if tc.storedBootID != "" {
					host.Status.NodeBootID = tc.storedBootID
				}
			}

			if tc.rebooted {
				host.Status.Rebooted = tc.rebooted
				host.Status.OngoingReboot = &infrav2.OngoingReboot{
					Type:        infrav2.RebootTypeSSH,
					TriggeredAt: metav1.Now(),
				}
			}

			sshMock := &sshmock.Client{}
			var hostNameOutput sshclient.Output
			if tc.rebootFinished {
				hostNameOutput = sshclient.Output{StdOut: infrav2.BareMetalHostNamePrefix + host.Spec.ConsumerRef.Name}
			} else {
				hostNameOutput = sshclient.Output{Err: timeout}
			}
			sshMock.On("GetHostName", mock.Anything).Return(hostNameOutput)
			sshMock.On("Reboot", mock.Anything).Return(sshclient.Output{})

			service := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), helpers.GetDefaultSSHSecret(osSSHKeyName, "default"), helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
			service.scope.HetznerBareMetalMachine.Spec.SSHSpec.PortAfterInstallImage = 23

			actResult := service.actionProvisioned(ctx)
			Expect(actResult).Should(BeAssignableToTypeOf(tc.expectedActionResult))
			Expect(host.HasRebootAnnotation()).To(Equal(tc.expectRebootAnnotation))
			Expect(host.Status.Rebooted).To(Equal(tc.expectRebootInStatus))
			Expect(host.Status.OngoingReboot != nil).To(Equal(tc.expectOngoingRebootInStatus))
			Expect(host.Status.NodeBootID).To(Equal(tc.expectedNodeBootID))

			// Phase 1 (storedBootID == ""): SSH Reboot should be called.
			// Phase 2 (storedBootID != ""): SSH Reboot should not be called again.
			if tc.shouldHaveRebootAnnotation && !tc.rebooted {
				Expect(sshMock.AssertCalled(GinkgoT(), "Reboot", mock.Anything)).To(BeTrue())
				Expect(host.Status.OngoingReboot.Type).To(Equal(infrav2.RebootTypeSSH))
				actionCompleted := conditions.Get(host, infrav2.HetznerBareMetalHostActionCompletedCondition)
				Expect(actionCompleted).ToNot(BeNil())
				Expect(actionCompleted.Reason).To(Equal(infrav2.HetznerBareMetalHostActionCompletedSSHRebootOngoingReason))
			} else {
				Expect(sshMock.AssertNotCalled(GinkgoT(), "Reboot", mock.Anything)).To(BeTrue())
			}
		},
		Entry("reboot desired, but not performed yet", testCaseActionProvisioned{
			shouldHaveRebootAnnotation:  true,
			rebooted:                    false,
			storedBootID:                fakeBootID,
			rebootFinished:              false,
			expectedActionResult:        actionContinue{},
			expectRebootAnnotation:      true,
			expectRebootInStatus:        true,
			expectOngoingRebootInStatus: true,
			expectedNodeBootID:          fakeBootID,
		}),
		Entry("reboot desired, and already performed, not finished", testCaseActionProvisioned{
			shouldHaveRebootAnnotation:  true,
			rebooted:                    true,
			storedBootID:                fakeBootID, // Phase 2: same as node BootID, still waiting
			rebootFinished:              false,
			expectedActionResult:        actionContinue{},
			expectRebootAnnotation:      true,
			expectRebootInStatus:        true,
			expectOngoingRebootInStatus: true,
			expectedNodeBootID:          fakeBootID,
		}),
		// BootID changed in the workload cluster: the sole signal that the reboot completed.
		Entry("reboot desired, performed, BootID changed in workload cluster", testCaseActionProvisioned{
			shouldHaveRebootAnnotation:  true,
			rebooted:                    true,
			storedBootID:                "old-boot-id", // Phase 2: differs from fakeBootID the node reports
			expectedActionResult:        actionFinished{},
			expectRebootAnnotation:      false,
			expectRebootInStatus:        false,
			expectOngoingRebootInStatus: false,
			expectedNodeBootID:          "old-boot-id",
		}),
		Entry("no reboot desired", testCaseActionProvisioned{
			shouldHaveRebootAnnotation:  false,
			rebooted:                    false,
			storedBootID:                fakeBootID,
			expectedActionResult:        actionFinished{},
			expectRebootAnnotation:      false,
			expectRebootInStatus:        false,
			expectOngoingRebootInStatus: false,
			expectedNodeBootID:          fakeBootID,
		}),
	)

	// SetError clears the ongoing reboot. This checks that the next reconcile after the reboot timed
	// out keeps the fatal error and the TimedOut reason, even when the BootID changes later.
	It("keeps the fatal error after the reboot via annotation timed out", func() {
		ctx := context.Background()
		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithIPv4(),
			helpers.WithConsumerRef(),
			helpers.WithOngoingReboot(infrav2.RebootTypeSSH, metav1.NewTime(time.Now().Add(-6*time.Minute))),
		)
		host.SetAnnotations(map[string]string{infrav2.RebootAnnotation: "reboot"})
		host.Status.Rebooted = true
		host.Status.NodeBootID = fakeBootID

		sshMock := &sshmock.Client{}
		service := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), helpers.GetDefaultSSHSecret(osSSHKeyName, "default"), helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

		Expect(service.actionProvisioned(ctx)).To(BeAssignableToTypeOf(actionStop{}))
		Expect(host.Status.ErrorType).To(Equal(infrav2.ErrorTypeFatal))
		Expect(host.Status.OngoingReboot).To(BeNil())

		// the node came back with a new BootID after the timeout
		host.Status.NodeBootID = "old-boot-id"

		Expect(service.actionProvisioned(ctx)).To(BeAssignableToTypeOf(actionStop{}))
		Expect(host.Status.ErrorType).To(Equal(infrav2.ErrorTypeFatal))
		rebootSucceeded := conditions.Get(host, infrav2.HetznerBareMetalHostRebootSucceededCondition)
		Expect(rebootSucceeded).NotTo(BeNil())
		Expect(rebootSucceeded.Reason).To(Equal(infrav2.HetznerBareMetalHostRebootSucceededTimeoutReachedReason))
	})
})

var _ = Describe("actionProvisioned NoSSHAfterInstallImage=true", func() {
	It("test reboot annotation for NoSSHAfterInstallImage=true, Reboot should be triggered", func() {
		ctx := context.Background()
		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithIPv4(),
			helpers.WithConsumerRef(),
		)

		host.SetAnnotations(map[string]string{infrav2.RebootAnnotation: "reboot"})
		// NodeBootID left empty: Phase 1 runs and sends the hardware reboot.

		robotMock := robotmock.Client{}
		robotMock.On("RebootBMServer", mock.Anything, mock.Anything).Return(nil, nil).Once()

		service := newTestService(host, &robotMock, nil, helpers.GetDefaultSSHSecret(osSSHKeyName, "default"), helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
		service.scope.HetznerBareMetalMachine.Spec.SSHSpec.NoSSHAfterInstallImage = true
		Expect(service.scope.RobotClient).ToNot(BeNil())

		actResult := service.actionProvisioned(ctx)
		Expect(actResult).Should(BeAssignableToTypeOf(actionContinue{}))
		Expect(robotMock.AssertNumberOfCalls(GinkgoT(), "RebootBMServer", 1)).To(BeTrue())
		Expect(host.Status.OngoingReboot).NotTo(BeNil())
		Expect(host.Status.OngoingReboot.Type).To(Equal(infrav2.RebootTypeHardware))
		c := conditions.Get(host, infrav2.HetznerBareMetalHostRebootSucceededCondition)
		Expect(c.Message).To(ContainSubstring("Rebooting because annotation was set"))
		cV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.RebootSucceededV1Beta1Condition)
		Expect(cV1Beta1.Message).To(ContainSubstring("Rebooting because annotation was set"))
	})

	It("test reboot annotation for NoSSHAfterInstallImage=true, reach: Waiting for BootID of Node", func() {
		ctx := context.Background()
		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithIPv4(),
			helpers.WithConsumerRef(),
		)

		host.SetAnnotations(map[string]string{infrav2.RebootAnnotation: "reboot"})
		host.Status.NodeBootID = fakeBootID

		service := newTestService(host, nil, nil, helpers.GetDefaultSSHSecret(osSSHKeyName, "default"), helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
		service.scope.HetznerBareMetalMachine.Spec.SSHSpec.NoSSHAfterInstallImage = true
		host.Status.Rebooted = true

		actResult := service.actionProvisioned(ctx)
		Expect(actResult).Should(BeAssignableToTypeOf(actionContinue{}))
		c := conditions.Get(host, infrav2.HetznerBareMetalHostRebootSucceededCondition)
		Expect(c.Message).To(ContainSubstring("Waiting for the node to be rebooted"))
		cV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.RebootSucceededV1Beta1Condition)
		Expect(cV1Beta1.Message).To(ContainSubstring("Waiting for the node to be rebooted"))
	})

	It("test reboot annotation for NoSSHAfterInstallImage=true, finished with healthy Condition", func() {
		// Change BootID
		ctx := context.Background()
		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithIPv4(),
			helpers.WithConsumerRef(),
		)

		host.SetAnnotations(map[string]string{infrav2.RebootAnnotation: "reboot"})
		host.Status.NodeBootID = fakeBootID

		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: host.Name,
			},
		}

		service := newTestService(host, nil, nil, helpers.GetDefaultSSHSecret(osSSHKeyName, "default"), helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
		service.scope.HetznerBareMetalMachine.Spec.SSHSpec.NoSSHAfterInstallImage = true
		host.Status.Rebooted = true

		err := service.scope.Client.Get(ctx, client.ObjectKeyFromObject(node), node)
		Expect(err).ToNot(HaveOccurred())

		node.Status.NodeInfo.BootID = "98765"
		err = service.scope.Client.Status().Update(ctx, node)
		Expect(err).ToNot(HaveOccurred())

		// Call actionProvisioned
		actResult := service.actionProvisioned(ctx)
		Expect(actResult).Should(BeAssignableToTypeOf(actionFinished{}))

		// Condition should be fine
		c := conditions.Get(host, infrav2.HetznerBareMetalHostRebootSucceededCondition)
		Expect(c.Message).To(Equal(""))
		Expect(c.Status).To(Equal(metav1.ConditionTrue))
		cV1Beta1 := deprecatedv1beta1conditions.Get(host, infrav2.RebootSucceededV1Beta1Condition)
		Expect(cV1Beta1.Message).To(Equal(""))
		Expect(cV1Beta1.Status).To(Equal(corev1.ConditionTrue))
		Expect(host.GetAnnotations()).To(BeEmpty())
	})
})

var _ = Describe("actionProvisioned when the Node is missing in the workload cluster", func() {
	It("stops reconciling instead of erroring forever", func() {
		ctx := context.Background()

		// drain events from previous tests
		for len(testEventRecorder.Events) > 0 {
			<-testEventRecorder.Events
		}

		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithIPv4(),
			helpers.WithConsumerRef(),
		)

		service := newTestService(host, nil, nil,
			helpers.GetDefaultSSHSecret(osSSHKeyName, "default"),
			helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

		// newTestService seeds a Node named after the host. Remove it so the
		// workload-cluster Get returns NotFound.
		Expect(service.scope.Client.Delete(ctx, &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: host.Name},
		})).To(Succeed())

		actResult := service.actionProvisioned(ctx)

		// actionStop returns no error and schedules no requeue.
		Expect(actResult).Should(BeAssignableToTypeOf(actionStop{}))
		res, err := actResult.Result()
		Expect(err).ToNot(HaveOccurred())
		Expect(res.RequeueAfter).To(BeZero())

		c := deprecatedv1beta1conditions.Get(host, infrav2.NodeBootIDRetrievedV1Beta1Condition)
		Expect(c).ToNot(BeNil())
		Expect(c.Status).To(Equal(corev1.ConditionFalse))
		Expect(c.Reason).To(Equal(infrav2.NodeNotFoundV1Beta1Reason))
		Expect(c.Message).To(ContainSubstring(host.Name))

		c2 := conditions.Get(host, infrav2.HetznerBareMetalHostNodeBootIDRetrievedCondition)
		Expect(c2).ToNot(BeNil())
		Expect(c2.Status).To(Equal(metav1.ConditionFalse))
		Expect(c2.Reason).To(Equal(infrav2.HetznerBareMetalHostNodeNotFoundReason))

		var events []string
		for len(testEventRecorder.Events) > 0 {
			events = append(events, <-testEventRecorder.Events)
		}
		Expect(events).To(ContainElement(ContainSubstring(infrav2.HetznerBareMetalHostNodeNotFoundReason)))
	})
})
