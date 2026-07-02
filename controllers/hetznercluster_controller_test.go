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

package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
)

func getDefaultHetznerClusterSpec() infrav2.HetznerClusterSpec {
	return infrav2.HetznerClusterSpec{
		ControlPlaneLoadBalancer: infrav2.LoadBalancerSpec{
			Enabled:   true,
			Algorithm: "round_robin",
			ExtraServices: []infrav2.LoadBalancerServiceSpec{
				{
					DestinationPort: 8132,
					ListenPort:      8132,
					Protocol:        "tcp",
				},
				{
					DestinationPort: 8133,
					ListenPort:      8133,
					Protocol:        "tcp",
				},
			},
			Port:   6443,
			Region: "fsn1",
			Type:   "lb11",
		},
		ControlPlaneEndpoint: infrav2.APIEndpoint{},
		ControlPlaneRegions:  []infrav2.Region{"fsn1"},
		HCloudNetwork: infrav2.HCloudNetworkSpec{
			CIDRBlock:       "10.0.0.0/16",
			Enabled:         true,
			NetworkZone:     "eu-central",
			SubnetCIDRBlock: "10.0.0.0/24",
		},
		HCloudPlacementGroups: []infrav2.HCloudPlacementGroupSpec{
			{
				Name: defaultPlacementGroupName,
				Type: "spread",
			},
			{
				Name: "md-0",
				Type: "spread",
			},
		},
		HetznerSecret: infrav2.HetznerSecretRef{
			Key: infrav2.HetznerSecretKeyRef{
				HCloudToken:          "hcloud",
				HetznerRobotUser:     "robot-user",
				HetznerRobotPassword: "robot-password",
			},
			Name: "hetzner-secret",
		},
		SSHKeys: infrav2.HetznerSSHKeys{
			HCloud: []infrav2.SSHKey{
				{
					Name: "testsshkey",
				},
			},
			RescueSecretRef: infrav2.SSHSecretRef{
				Name: "rescue-ssh-secret",
				Key: infrav2.SSHSecretKeyRef{
					Name:       "sshkey-name",
					PublicKey:  "public-key",
					PrivateKey: "private-key",
				},
			},
		},
	}
}

func isHetznerClusterProvisioned(hetznerCluster *infrav2.HetznerCluster) bool {
	return ptr.Deref(hetznerCluster.Status.Initialization.Provisioned, false)
}

func TestIgnoreInsignificantClusterStatusUpdates(t *testing.T) {
	logger := klog.Background()
	predicate := IgnoreInsignificantClusterStatusUpdates(logger)

	testCases := []struct {
		name     string
		oldObj   *clusterv1.Cluster
		newObj   *clusterv1.Cluster
		expected bool
	}{
		{
			name: "No significant changes",
			oldObj: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Status: clusterv1.ClusterStatus{
					Phase: "Provisioned",
				},
			},
			newObj: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-cluster",
					Namespace:       "default",
					ResourceVersion: "2",
				},
				Status: clusterv1.ClusterStatus{
					Phase: "Provisioned",
				},
			},
			expected: false,
		},
		{
			name: "Significant changes in spec",
			oldObj: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						Pods: clusterv1.NetworkRanges{
							CIDRBlocks: []string{"192.168.0.0/16"},
						},
					},
				},
			},
			newObj: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						Pods: clusterv1.NetworkRanges{
							CIDRBlocks: []string{"10.0.0.0/16"},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Changes only in status",
			oldObj: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Status: clusterv1.ClusterStatus{
					Phase: "Provisioning",
				},
			},
			newObj: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Status: clusterv1.ClusterStatus{
					Phase: "Provisioned",
				},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			updateEvent := event.UpdateEvent{
				ObjectOld: tc.oldObj,
				ObjectNew: tc.newObj,
			}
			result := predicate.Update(updateEvent)
			if result != tc.expected {
				t.Errorf("Expected %v, but got %v", tc.expected, result)
			}
		})
	}
}

func TestIgnoreInsignificantHetznerClusterStatusUpdates(t *testing.T) {
	logger := klog.Background()
	predicate := IgnoreInsignificantHetznerClusterStatusUpdates(logger)

	testCases := []struct {
		name     string
		oldObj   *infrav2.HetznerCluster
		newObj   *infrav2.HetznerCluster
		expected bool
	}{
		{
			name: "No significant changes",
			oldObj: &infrav2.HetznerCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hetzner-cluster",
					Namespace: "default",
				},
				Status: infrav2.HetznerClusterStatus{
					Initialization: infrav2.HetznerClusterInitializationStatus{
						Provisioned: ptr.To(true),
					},
				},
			},
			newObj: &infrav2.HetznerCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-hetzner-cluster",
					Namespace:       "default",
					ResourceVersion: "2",
				},
				Status: infrav2.HetznerClusterStatus{
					Initialization: infrav2.HetznerClusterInitializationStatus{
						Provisioned: ptr.To(true),
					},
				},
			},
			expected: false,
		},
		{
			name: "Significant changes in spec",
			oldObj: &infrav2.HetznerCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hetzner-cluster",
					Namespace: "default",
				},
				Spec: infrav2.HetznerClusterSpec{
					ControlPlaneRegions: []infrav2.Region{"fsn1"},
				},
			},
			newObj: &infrav2.HetznerCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hetzner-cluster",
					Namespace: "default",
				},
				Spec: infrav2.HetznerClusterSpec{
					ControlPlaneRegions: []infrav2.Region{"nbg1"},
				},
			},
			expected: true,
		},
		{
			name: "Empty status in new object",
			oldObj: &infrav2.HetznerCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hetzner-cluster",
					Namespace: "default",
				},
				Status: infrav2.HetznerClusterStatus{
					Initialization: infrav2.HetznerClusterInitializationStatus{
						Provisioned: ptr.To(true),
					},
				},
			},
			newObj: &infrav2.HetznerCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hetzner-cluster",
					Namespace: "default",
				},
				Status: infrav2.HetznerClusterStatus{},
			},
			expected: true,
		},
		{
			name: "Changes only in status",
			oldObj: &infrav2.HetznerCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hetzner-cluster",
					Namespace: "default",
				},
				Status: infrav2.HetznerClusterStatus{
					Initialization: infrav2.HetznerClusterInitializationStatus{
						Provisioned: ptr.To(false),
					},
				},
			},
			newObj: &infrav2.HetznerCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hetzner-cluster",
					Namespace: "default",
				},
				Status: infrav2.HetznerClusterStatus{
					Initialization: infrav2.HetznerClusterInitializationStatus{
						Provisioned: ptr.To(true),
					},
				},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			updateEvent := event.UpdateEvent{
				ObjectOld: tc.oldObj,
				ObjectNew: tc.newObj,
			}
			result := predicate.Update(updateEvent)
			if result != tc.expected {
				t.Errorf("Expected %v, but got %v", tc.expected, result)
			}
		})
	}
}

// TestWorkloadClusterSecretNames verifies which workload-cluster secrets CAPH
// reconciles for the configured management-cluster secret name.
func TestControlPlaneMachineToHetznerClusterPredicate(t *testing.T) {
	predicate := controlPlaneMachineToHetznerClusterPredicate()

	withServerAvailable := func(m *infrav2.HCloudMachine, status metav1.ConditionStatus) *infrav2.HCloudMachine {
		conditions.Set(m, metav1.Condition{
			Type:   string(infrav1.HCloudMachineServerAvailableV1Beta2Condition),
			Status: status,
			Reason: "reason",
		})
		return m
	}

	controlPlaneHCloudMachine := func(name string) *infrav2.HCloudMachine {
		return &infrav2.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				Labels:    map[string]string{clusterv1.MachineControlPlaneLabel: ""},
			},
		}
	}

	workerHCloudMachine := func(name string) *infrav2.HCloudMachine {
		return &infrav2.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
		}
	}

	t.Run("update: control plane machine ServerAvailable transitions to True", func(t *testing.T) {
		oldObj := withServerAvailable(controlPlaneHCloudMachine("cp-0"), metav1.ConditionFalse)
		newObj := withServerAvailable(controlPlaneHCloudMachine("cp-0"), metav1.ConditionTrue)
		require.True(t, predicate.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}))
	})

	t.Run("update: worker machine ServerAvailable transitions to True", func(t *testing.T) {
		oldObj := withServerAvailable(workerHCloudMachine("md-0"), metav1.ConditionFalse)
		newObj := withServerAvailable(workerHCloudMachine("md-0"), metav1.ConditionTrue)
		require.False(t, predicate.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}))
	})

	t.Run("update: control plane machine ServerAvailable stays True", func(t *testing.T) {
		oldObj := withServerAvailable(controlPlaneHCloudMachine("cp-0"), metav1.ConditionTrue)
		newObj := withServerAvailable(controlPlaneHCloudMachine("cp-0"), metav1.ConditionTrue)
		require.False(t, predicate.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}))
	})

	t.Run("delete: control plane machine", func(t *testing.T) {
		require.True(t, predicate.Delete(event.DeleteEvent{Object: controlPlaneHCloudMachine("cp-0")}))
	})

	t.Run("delete: worker machine", func(t *testing.T) {
		require.False(t, predicate.Delete(event.DeleteEvent{Object: workerHCloudMachine("md-0")}))
	})

	t.Run("update: bare metal control plane machine uses its own condition type", func(t *testing.T) {
		oldObj := &infrav2.HetznerBareMetalMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cp-0",
				Namespace: "default",
				Labels:    map[string]string{clusterv1.MachineControlPlaneLabel: ""},
			},
		}
		newObj := oldObj.DeepCopy()
		conditions.Set(newObj, metav1.Condition{
			Type:   string(infrav1.HetznerBareMetalMachineServerAvailableV1Beta2Condition),
			Status: metav1.ConditionTrue,
			Reason: "reason",
		})
		require.True(t, predicate.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}))
	})
}

func TestWorkloadClusterSecretNames(t *testing.T) {
	testCases := []struct {
		name       string
		secretName string
		want       []string
	}{
		{
			name:       "keep hcloud as single secret",
			secretName: "hcloud",
			want:       []string{"hcloud"},
		},
		{
			name:       "add hcloud compatibility secret",
			secretName: "hetzner",
			want:       []string{"hetzner", "hcloud"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := workloadClusterSecretNames(tc.secretName)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestWorkloadClusterHCloudTokenKeys verifies when CAPH adds the upstream
// compatibility key alongside the configured management-cluster key.
func TestWorkloadClusterHCloudTokenKeys(t *testing.T) {
	testCases := []struct {
		name          string
		secretName    string
		configuredKey string
		want          []string
	}{
		{
			name:          "non-hcloud secrets only keep configured key",
			secretName:    "hetzner",
			configuredKey: "custom-token",
			want:          []string{"custom-token"},
		},
		{
			name:          "hcloud secret adds token compatibility key",
			secretName:    "hcloud",
			configuredKey: "custom-token",
			want:          []string{"custom-token", "token"},
		},
		{
			name:          "non-hcloud secrets keep hcloud configured key without duplication",
			secretName:    "hetzner",
			configuredKey: "hcloud",
			want:          []string{"hcloud"},
		},
		{
			name:          "existing token key is not duplicated",
			secretName:    "hcloud",
			configuredKey: "token",
			want:          []string{"token"},
		},
		{
			name:          "other secret names only keep configured key",
			secretName:    "custom-name",
			configuredKey: "custom-token",
			want:          []string{"custom-token"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := keysForWorkloadClusterSecret(tc.secretName, tc.configuredKey, "token")
			require.Equal(t, tc.want, got)
		})
	}
}

// TestReconcileOneWorkloadClusterSecretHetzner verifies that the configured
// workload-cluster secret keeps the configured key names without adding the
// upstream compatibility aliases.
func TestReconcileOneWorkloadClusterSecretHetzner(t *testing.T) {
	ctx := context.Background()

	testScheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(testScheme))
	utilruntime.Must(infrav2.AddToScheme(testScheme))

	hetznerCluster := &infrav2.HetznerCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-ns",
			UID:       "test-cluster-uid",
		},
		Spec: getDefaultHetznerClusterSpec(),
	}
	hetznerCluster.Spec.HetznerSecret.Name = "hetzner"
	hetznerCluster.Spec.HetznerSecret.Key.HCloudToken = "custom-token"
	hetznerCluster.Spec.HetznerSecret.Key.HetznerRobotUser = "custom-robot-user"
	hetznerCluster.Spec.HetznerSecret.Key.HetznerRobotPassword = "custom-robot-password"
	hetznerCluster.Spec.HCloudNetwork.Enabled = false
	hetznerCluster.Spec.ControlPlaneEndpoint = infrav2.APIEndpoint{
		Host: "198.51.100.10",
		Port: 6443,
	}

	mgtSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hetznerCluster.Spec.HetznerSecret.Name,
			Namespace: hetznerCluster.Namespace,
		},
		Data: map[string][]byte{
			"custom-token":          []byte("my-token"),
			"custom-robot-user":     []byte("my-user"),
			"custom-robot-password": []byte("my-password"),
		},
	}

	mgtClient := fakeclient.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(hetznerCluster.DeepCopy(), mgtSecret.DeepCopy()).
		Build()
	wlClient := fakeclient.NewClientBuilder().
		WithScheme(testScheme).
		Build()

	clusterScope := &scope.ClusterScope{
		Logger:         klog.Background(),
		Client:         mgtClient,
		APIReader:      mgtClient,
		HetznerCluster: hetznerCluster,
	}

	require.NoError(t, reconcileOneWorkloadClusterSecret(ctx, clusterScope, wlClient, "hetzner"))

	secret := &corev1.Secret{}
	require.NoError(t, wlClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: "hetzner"}, secret))
	require.Equal(t, "my-token", string(secret.Data["custom-token"]))
	require.NotContains(t, secret.Data, "hcloud")
	require.NotContains(t, secret.Data, "token")
	require.Equal(t, "my-user", string(secret.Data["custom-robot-user"]))
	require.Equal(t, "my-password", string(secret.Data["custom-robot-password"]))
	require.NotContains(t, secret.Data, "robot-user")
	require.NotContains(t, secret.Data, "robot-password")
	require.Equal(t, "198.51.100.10", string(secret.Data["apiserver-host"]))
	require.Equal(t, "6443", string(secret.Data["apiserver-port"]))

	require.NotContains(t, secret.Data, "note")
}

// TestReconcileOneWorkloadClusterSecretHCloud verifies that the "hcloud"
// compatibility secret exposes both the configured keys and the upstream alias
// keys expected by external consumers.
func TestReconcileOneWorkloadClusterSecretHCloud(t *testing.T) {
	ctx := context.Background()

	testScheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(testScheme))
	utilruntime.Must(infrav2.AddToScheme(testScheme))

	hetznerCluster := &infrav2.HetznerCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-ns",
			UID:       "test-cluster-uid",
		},
		Spec: getDefaultHetznerClusterSpec(),
	}
	hetznerCluster.Spec.HetznerSecret.Name = "hetzner"
	hetznerCluster.Spec.HetznerSecret.Key.HCloudToken = "custom-token"
	hetznerCluster.Spec.HetznerSecret.Key.HetznerRobotUser = "custom-robot-user"
	hetznerCluster.Spec.HetznerSecret.Key.HetznerRobotPassword = "custom-robot-password"
	hetznerCluster.Spec.HCloudNetwork.Enabled = false
	hetznerCluster.Spec.ControlPlaneEndpoint = infrav2.APIEndpoint{
		Host: "198.51.100.10",
		Port: 6443,
	}

	mgtSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hetznerCluster.Spec.HetznerSecret.Name,
			Namespace: hetznerCluster.Namespace,
		},
		Data: map[string][]byte{
			"custom-token":          []byte("my-token"),
			"custom-robot-user":     []byte("my-user"),
			"custom-robot-password": []byte("my-password"),
		},
	}

	mgtClient := fakeclient.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(hetznerCluster.DeepCopy(), mgtSecret.DeepCopy()).
		Build()
	wlClient := fakeclient.NewClientBuilder().
		WithScheme(testScheme).
		Build()

	clusterScope := &scope.ClusterScope{
		Logger:         klog.Background(),
		Client:         mgtClient,
		APIReader:      mgtClient,
		HetznerCluster: hetznerCluster,
	}

	require.NoError(t, reconcileOneWorkloadClusterSecret(ctx, clusterScope, wlClient, "hcloud"))

	secret := &corev1.Secret{}
	require.NoError(t, wlClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: "hcloud"}, secret))
	require.Equal(t, "my-token", string(secret.Data["custom-token"]))
	require.Equal(t, "my-token", string(secret.Data["token"]))
	require.NotContains(t, secret.Data, "hcloud")
	require.Equal(t, "my-user", string(secret.Data["custom-robot-user"]))
	require.Equal(t, "my-password", string(secret.Data["custom-robot-password"]))
	require.Equal(t, "my-user", string(secret.Data["robot-user"]))
	require.Equal(t, "my-password", string(secret.Data["robot-password"]))
	require.Equal(t, "198.51.100.10", string(secret.Data["apiserver-host"]))
	require.Equal(t, "6443", string(secret.Data["apiserver-port"]))

	require.NotContains(t, secret.Data, "note")
}

// TestReconcileAllWorkloadClusterSecretsCreatesCompatibilitySecret verifies
// that reconciling a non-default management secret creates both workload
// secrets with the expected per-secret data.
func TestReconcileAllWorkloadClusterSecretsCreatesCompatibilitySecret(t *testing.T) {
	ctx := context.Background()

	testScheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(testScheme))
	utilruntime.Must(infrav2.AddToScheme(testScheme))

	hetznerCluster := &infrav2.HetznerCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-ns",
			UID:       "test-cluster-uid",
		},
		Spec: getDefaultHetznerClusterSpec(),
	}
	hetznerCluster.Spec.HetznerSecret.Name = "hetzner"
	hetznerCluster.Spec.HetznerSecret.Key.HetznerRobotUser = "custom-robot-user"
	hetznerCluster.Spec.HetznerSecret.Key.HetznerRobotPassword = "custom-robot-password"
	hetznerCluster.Spec.HCloudNetwork.Enabled = false
	hetznerCluster.Spec.ControlPlaneEndpoint = infrav2.APIEndpoint{
		Host: "198.51.100.10",
		Port: 6443,
	}

	mgtSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hetznerCluster.Spec.HetznerSecret.Name,
			Namespace: hetznerCluster.Namespace,
		},
		Data: map[string][]byte{
			"hcloud":                []byte("my-token"),
			"custom-robot-user":     []byte("my-user"),
			"custom-robot-password": []byte("my-password"),
		},
	}

	mgtClient := fakeclient.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(hetznerCluster.DeepCopy(), mgtSecret.DeepCopy()).
		Build()
	wlClient := fakeclient.NewClientBuilder().
		WithScheme(testScheme).
		Build()

	clusterScope := &scope.ClusterScope{
		Logger:         klog.Background(),
		Client:         mgtClient,
		APIReader:      mgtClient,
		HetznerCluster: hetznerCluster,
	}

	require.NoError(t, reconcileAllWorkloadClusterSecrets(ctx, clusterScope, wlClient))

	for _, name := range []string{"hetzner", "hcloud"} {
		secret := &corev1.Secret{}
		require.NoError(t, wlClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: name}, secret))
		switch name {
		case "hetzner":
			require.Equal(t, "my-token", string(secret.Data["hcloud"]))
			require.NotContains(t, secret.Data, "token")
			require.Equal(t, "my-user", string(secret.Data["custom-robot-user"]))
			require.Equal(t, "my-password", string(secret.Data["custom-robot-password"]))
			require.NotContains(t, secret.Data, "robot-user")
			require.NotContains(t, secret.Data, "robot-password")
		case "hcloud":
			require.Equal(t, "my-token", string(secret.Data["hcloud"]))
			require.Equal(t, "my-token", string(secret.Data["token"]))
			require.Equal(t, "my-user", string(secret.Data["custom-robot-user"]))
			require.Equal(t, "my-password", string(secret.Data["custom-robot-password"]))
			require.Equal(t, "my-user", string(secret.Data["robot-user"]))
			require.Equal(t, "my-password", string(secret.Data["robot-password"]))
		}
	}
}

var _ = Describe("Hetzner ClusterReconciler", func() {
	Context("cluster tests", func() {
		var (
			err       error
			namespace string
			testNs    *corev1.Namespace

			instance    *infrav2.HetznerCluster
			capiCluster *clusterv1.Cluster

			hetznerSecret *corev1.Secret

			key                client.ObjectKey
			lbName             string
			hetznerClusterName string
			hcloudClient       hcloudclient.Client
		)
		BeforeEach(func() {
			testNs, err = testEnv.ResetAndCreateNamespace(ctx, "cluster-tests")
			Expect(err).NotTo(HaveOccurred())
			hcloudClient = testEnv.HCloudClientFactory.NewClient("fake-token")

			namespace = testNs.Name

			lbName = utils.GenerateName(nil, "myloadbalancer")

			hetznerClusterName = utils.GenerateName(nil, "hetzner-test1")
			// Create capi cluster
			capiCluster = &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test1-",
					Namespace:    namespace,
					Finalizers:   []string{clusterv1.ClusterFinalizer},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						APIGroup: infrav2.GroupVersion.Group,
						Kind:     "HetznerCluster",
						Name:     hetznerClusterName,
					},
				},
			}
			Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())

			// Create the HetznerCluster object
			instance = &infrav2.HetznerCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      hetznerClusterName,
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Cluster",
							Name:       capiCluster.Name,
							UID:        capiCluster.UID,
						},
					},
				},
				Spec: getDefaultHetznerClusterSpec(),
			}

			hetznerSecret = getDefaultHetznerSecret(namespace)
			Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

			key = client.ObjectKey{Namespace: namespace, Name: hetznerClusterName}
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, testNs, capiCluster, instance, hetznerSecret)).To(Succeed())
		})

		It("should set the finalizer", func() {
			Expect(testEnv.Create(ctx, instance)).To(Succeed())

			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, instance); err != nil {
					return false
				}
				return len(instance.Finalizers) > 0
			}, timeout, time.Second).Should(BeTrue())
		})

		It("should set the Deleting condition when the HetznerCluster is being deleted", func() {
			// Add a blocking finalizer so the object is not immediately garbage-collected
			// after the controller removes its own finalizer. This gives us a stable window
			// to observe the Deleting condition.
			const testFinalizer = "test.caph.syself.com/block-deletion"
			instance.Finalizers = append(instance.Finalizers, testFinalizer)
			Expect(testEnv.Create(ctx, instance)).To(Succeed())

			// Wait for the controller to reconcile and add the CAPH finalizer.
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, instance); err != nil {
					return false
				}
				for _, f := range instance.Finalizers {
					if f == infrav2.HetznerClusterFinalizer {
						return true
					}
				}
				return false
			}, timeout, time.Second).Should(BeTrue())

			// Trigger deletion. The DeletionTimestamp is set but the object is not removed
			// because testFinalizer is still present.
			Expect(testEnv.Delete(ctx, instance)).To(Succeed())

			// The controller should set the Deleting condition to True once it
			// processes the deletion request.
			Eventually(func() bool {
				return isPresentAndTrueWithReason(key, instance, infrav2.HetznerClusterDeletingCondition, infrav2.HetznerClusterDeletingReason)
			}, timeout, time.Second).Should(BeTrue())

			// Remove the blocking finalizer so the object can be fully cleaned up.
			Eventually(func() error {
				if err := testEnv.Get(ctx, key, instance); err != nil {
					return err
				}
				newFinalizers := make([]string, 0, len(instance.Finalizers))
				for _, f := range instance.Finalizers {
					if f != testFinalizer {
						newFinalizers = append(newFinalizers, f)
					}
				}
				instance.Finalizers = newFinalizers
				return testEnv.Update(ctx, instance)
			}, timeout).Should(Succeed())
		})

		Context("load balancer", func() {
			It("should create load balancer and update it accordingly", func() {
				Expect(testEnv.Create(ctx, instance)).To(Succeed())

				Eventually(func() bool {
					return isPresentAndTrueDeprecatedV1Beta1(key, instance, infrav2.LoadBalancerReadyV1Beta1Condition) &&
						isPresentAndTrueWithReason(key, instance, infrav2.HetznerClusterLoadBalancerReadyCondition, string(infrav2.HetznerClusterLoadBalancerReadyReason))
				}, timeout, time.Second).Should(BeTrue())

				newLBName := "new-lb-name"
				newLBType := "lb31"

				By("updating load balancer type")

				ph, err := patch.NewHelper(instance, testEnv)
				Expect(err).ShouldNot(HaveOccurred())

				instance.Spec.ControlPlaneLoadBalancer.Type = newLBType

				Eventually(func() error {
					return ph.Patch(ctx, instance, patch.WithStatusObservedGeneration{})
				}, timeout).Should(BeNil())

				By("updating load balancer name")

				ph, err = patch.NewHelper(instance, testEnv)
				Expect(err).ShouldNot(HaveOccurred())

				instance.Spec.ControlPlaneLoadBalancer.Name = &newLBName

				Eventually(func() error {
					return ph.Patch(ctx, instance, patch.WithStatusObservedGeneration{})
				}, timeout).Should(BeNil())

				By("listing load balancers and checking spec")

				// Check in hetzner API
				Eventually(func() bool {
					loadBalancers, err := hcloudClient.ListLoadBalancers(ctx, hcloud.LoadBalancerListOpts{
						ListOpts: hcloud.ListOpts{
							LabelSelector: utils.LabelsToLabelSelector(map[string]string{instance.ClusterTagKey(): "owned"}),
						},
					})
					if err != nil {
						testEnv.GetLogger().Info("failed to list load balancers", "err", err)
						return false
					}
					if len(loadBalancers) > 1 {
						testEnv.GetLogger().Info("there are multiple load balancers found", "number of load balancers", loadBalancers)
						return false
					}
					if len(loadBalancers) == 0 {
						testEnv.GetLogger().Info("no load balancer found")
						return false
					}

					lb := loadBalancers[0]

					if lb.Name != newLBName {
						testEnv.GetLogger().Info("wrong name", "want", newLBName, "got", lb.Name)
						return false
					}
					if lb.LoadBalancerType.Name != newLBType {
						testEnv.GetLogger().Info("wrong type", "want", newLBType, "got", lb.LoadBalancerType.Name)
						return false
					}

					return true
				}, 2*timeout, 1*time.Second).Should(BeTrue())
			})

			It("should update extra targets", func() {
				Expect(testEnv.Create(ctx, instance)).To(Succeed())

				Eventually(func() bool {
					return isPresentAndTrueDeprecatedV1Beta1(key, instance, infrav2.LoadBalancerReadyV1Beta1Condition) &&
						isPresentAndTrueWithReason(key, instance, infrav2.HetznerClusterLoadBalancerReadyCondition, string(infrav2.HetznerClusterLoadBalancerReadyReason))
				}, timeout).Should(BeTrue())

				By("adding additional extra services")

				ph, err := patch.NewHelper(instance, testEnv)
				Expect(err).ShouldNot(HaveOccurred())

				instance.Spec.ControlPlaneLoadBalancer.ExtraServices = append(instance.Spec.ControlPlaneLoadBalancer.ExtraServices,
					infrav2.LoadBalancerServiceSpec{
						DestinationPort: 8134,
						ListenPort:      8134,
						Protocol:        "tcp",
					})

				Eventually(func() error {
					return ph.Patch(ctx, instance, patch.WithStatusObservedGeneration{})
				}, timeout).Should(BeNil())

				Eventually(func() int {
					loadBalancers, err := hcloudClient.ListLoadBalancers(ctx, hcloud.LoadBalancerListOpts{
						ListOpts: hcloud.ListOpts{
							LabelSelector: utils.LabelsToLabelSelector(map[string]string{instance.ClusterTagKey(): "owned"}),
						},
					})
					if err != nil {
						return -1
					}
					if len(loadBalancers) > 1 {
						return -2
					}
					if len(loadBalancers) == 0 {
						return -3
					}
					lb := loadBalancers[0]

					return len(lb.Services)
				}, timeout).Should(Equal(len(instance.Spec.ControlPlaneLoadBalancer.ExtraServices) + 1))

				By("reducing extra targets")

				ph, err = patch.NewHelper(instance, testEnv)
				Expect(err).ShouldNot(HaveOccurred())
				instance.Spec.ControlPlaneLoadBalancer.ExtraServices = []infrav2.LoadBalancerServiceSpec{
					{
						DestinationPort: 8134,
						ListenPort:      8134,
						Protocol:        "tcp",
					},
				}

				Eventually(func() error {
					return ph.Patch(ctx, instance, patch.WithStatusObservedGeneration{})
				}, timeout).Should(BeNil())

				Eventually(func() int {
					loadBalancers, err := hcloudClient.ListLoadBalancers(ctx, hcloud.LoadBalancerListOpts{
						ListOpts: hcloud.ListOpts{
							LabelSelector: utils.LabelsToLabelSelector(map[string]string{instance.ClusterTagKey(): "owned"}),
						},
					})
					if err != nil {
						return -1
					}
					if len(loadBalancers) > 1 {
						return -2
					}
					if len(loadBalancers) == 0 {
						return -3
					}
					lb := loadBalancers[0]

					return len(lb.Services)
				}, timeout).Should(Equal(len(instance.Spec.ControlPlaneLoadBalancer.ExtraServices) + 1))

				By("removing extra targets")

				ph, err = patch.NewHelper(instance, testEnv)
				Expect(err).ShouldNot(HaveOccurred())
				instance.Spec.ControlPlaneLoadBalancer.ExtraServices = nil

				Eventually(func() error {
					return ph.Patch(ctx, instance, patch.WithStatusObservedGeneration{})
				}, timeout).Should(BeNil())

				Eventually(func() int {
					loadBalancers, err := hcloudClient.ListLoadBalancers(ctx, hcloud.LoadBalancerListOpts{
						ListOpts: hcloud.ListOpts{
							LabelSelector: utils.LabelsToLabelSelector(map[string]string{instance.ClusterTagKey(): "owned"}),
						},
					})
					if err != nil {
						return -1
					}
					if len(loadBalancers) > 1 {
						return -2
					}
					if len(loadBalancers) == 0 {
						return -3
					}
					lb := loadBalancers[0]

					return len(lb.Services)
				}, timeout).Should(Equal(len(instance.Spec.ControlPlaneLoadBalancer.ExtraServices) + 1))
			})

			It("should not create load balancer if disabled and the cluster should get ready", func() {
				instance.Spec.ControlPlaneLoadBalancer.Enabled = false
				instance.Spec.ControlPlaneEndpoint = infrav2.APIEndpoint{
					Host: "my.test.host",
					Port: 6443,
				}
				Expect(testEnv.Create(ctx, instance)).To(Succeed())

				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, instance); err != nil {
						return false
					}

					return instance.Status.ControlPlaneLoadBalancer == nil &&
						instance.Status.Initialization.Provisioned != nil &&
						*instance.Status.Initialization.Provisioned
				}, timeout, time.Second).Should(BeTrue())

				By("making sure LoadBalancerReady condition is not set")
				Expect(isAbsent(key, instance, infrav2.HetznerClusterLoadBalancerReadyCondition)).To(BeTrue())
			})

			It("should take over an existing load balancer with correct name", func() {
				By("creating load balancer manually")

				opts := hcloud.LoadBalancerCreateOpts{
					Name:             lbName,
					Algorithm:        &hcloud.LoadBalancerAlgorithm{Type: hcloud.LoadBalancerAlgorithmTypeLeastConnections},
					LoadBalancerType: &hcloud.LoadBalancerType{Name: "mytype"},
				}

				_, err := hcloudClient.CreateLoadBalancer(ctx, opts)
				Expect(err).To(BeNil())

				By("making sure that there is no label set")
				loadBalancers, err := hcloudClient.ListLoadBalancers(ctx, hcloud.LoadBalancerListOpts{Name: lbName})
				Expect(err).To(BeNil())
				Expect(loadBalancers).To(HaveLen(1))

				_, found := loadBalancers[0].Labels[instance.ClusterTagKey()]
				Expect(found).To(BeFalse())

				By("creating cluster object")

				instance.Spec.ControlPlaneLoadBalancer.Name = &lbName
				Expect(testEnv.Create(ctx, instance)).To(Succeed())

				By("checking that cluster is ready")

				Eventually(func() bool {
					err := testEnv.Get(ctx, client.ObjectKeyFromObject(instance), instance)
					if err != nil {
						return false
					}
					c := deprecatedv1beta1conditions.Get(instance, infrav2.LoadBalancerReadyV1Beta1Condition)
					if c == nil {
						GinkgoLogr.Info("LoadBalancerReadyCondition is nil")
						return false
					}
					if c.Status == corev1.ConditionTrue {
						GinkgoLogr.Info("LoadBalancerReadyCondition is True now")
						return true
					}
					GinkgoLogr.Info("LoadBalancerReadyCondition is not True yet.",
						"reason", c.Reason,
						"message", c.Message,
					)
					return false
				}, 2*timeout, time.Second).Should(BeTrue())

				By("checking that load balancer has label set")

				loadBalancers, err = hcloudClient.ListLoadBalancers(ctx, hcloud.LoadBalancerListOpts{Name: lbName})
				Expect(err).To(BeNil())
				Expect(loadBalancers).To(HaveLen(1))

				value, found := loadBalancers[0].Labels[instance.ClusterTagKey()]
				Expect(found).To(BeTrue())
				Expect(value).To(Equal(string(infrav2.ResourceLifecycleOwned)))

				By("checking that kubeapi service is set on load balancer")

				var foundHetznerCluster infrav2.HetznerCluster

				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, &foundHetznerCluster); err != nil {
						testEnv.GetLogger().Error(err, "failed to fetch HetznerCluster")
						return false
					}

					// fetch load balancer again as reconcilement of additional services happens after the load balancer has been created
					loadBalancers, err := hcloudClient.ListLoadBalancers(ctx, hcloud.LoadBalancerListOpts{Name: lbName})
					if err != nil {
						testEnv.GetLogger().Error(err, "failed to list load balancers")
						return false
					}

					if len(loadBalancers) != 1 {
						testEnv.GetLogger().Info("expect 1 load balancer - but did not get it", "got", len(loadBalancers))
						return false
					}

					lb := loadBalancers[0]
					for _, service := range lb.Services {
						if service.ListenPort == int(foundHetznerCluster.Spec.ControlPlaneEndpoint.Port) {
							return true
						}
					}

					testEnv.GetLogger().Info(
						"Could not find listenPort of kubeapiserver in load balancer services",
						"load balancer services", lb.Services,
						"listenPort of kubeAPI service", foundHetznerCluster.Spec.ControlPlaneEndpoint.Port,
					)
					return false
				}, timeout, time.Second).Should(BeTrue())

				By("deleting the cluster and load balancer and testing that owned label is gone")

				Expect(testEnv.Delete(ctx, instance))

				Eventually(func() bool {
					loadBalancers, err := hcloudClient.ListLoadBalancers(ctx, hcloud.LoadBalancerListOpts{Name: lbName})
					// there should always be one load balancer, if not, then this is a problem where we can immediately return
					Expect(err).To(BeNil())
					Expect(loadBalancers).To(HaveLen(1))

					_, found := loadBalancers[0].Labels[instance.ClusterTagKey()]
					return found
				}, timeout, time.Second).Should(BeFalse())
			})

			It("should set the appropriate condition if a named load balancer is taken by another cluster", func() {
				By("creating load balancer manually")
				labelsOwnedByOtherCluster := map[string]string{instance.ClusterTagKey() + "s": string(infrav2.ResourceLifecycleOwned)}
				opts := hcloud.LoadBalancerCreateOpts{
					Name:             lbName,
					Algorithm:        &hcloud.LoadBalancerAlgorithm{Type: hcloud.LoadBalancerAlgorithmTypeLeastConnections},
					LoadBalancerType: &hcloud.LoadBalancerType{Name: "mytype"},
					Labels:           labelsOwnedByOtherCluster,
				}

				_, err := hcloudClient.CreateLoadBalancer(ctx, opts)
				Expect(err).To(BeNil())

				By("creating cluster object")

				instance.Spec.ControlPlaneLoadBalancer.Name = &lbName
				Expect(testEnv.Create(ctx, instance)).To(Succeed())

				By("checking that cluster is ready")

				Eventually(func() bool {
					return isPresentAndFalseWithReasonDeprecatedV1Beta1(key, instance, infrav2.LoadBalancerReadyV1Beta1Condition, infrav2.LoadBalancerFailedToOwnV1Beta1Reason) &&
						isPresentAndFalseWithReason(key, instance, infrav2.HetznerClusterLoadBalancerReadyCondition, infrav2.HetznerClusterLoadBalancerOwningFailedReason)
				}, timeout, time.Second).Should(BeTrue())
			})

			It("should set the appropriate condition if a named load balancer is not found", func() {
				By("creating cluster object")

				instance.Spec.ControlPlaneLoadBalancer.Name = &lbName
				Expect(testEnv.Create(ctx, instance)).To(Succeed())

				By("checking that cluster has condition set")

				Eventually(func() bool {
					return isPresentAndFalseWithReasonDeprecatedV1Beta1(key, instance, infrav2.LoadBalancerReadyV1Beta1Condition, infrav2.LoadBalancerFailedToOwnV1Beta1Reason) &&
						isPresentAndFalseWithReason(key, instance, infrav2.HetznerClusterLoadBalancerReadyCondition, infrav2.HetznerClusterLoadBalancerOwningFailedReason)
				}, timeout, time.Second).Should(BeTrue())
			})

			It("should work with capi.syself.com/allow-empty-control-plane-address annotation error condition", func() {
				instance.Annotations = make(map[string]string)
				instance.Annotations[infrav2.AllowEmptyControlPlaneAddressAnnotation] = "true"
				instance.Spec.ControlPlaneLoadBalancer.Enabled = false
				instance.Spec.ControlPlaneEndpoint = infrav2.APIEndpoint{}
				Expect(testEnv.Create(ctx, instance)).To(Succeed())

				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, instance); err != nil {
						return false
					}

					return isPresentAndFalseWithReasonDeprecatedV1Beta1(key, instance, infrav2.ControlPlaneEndpointSetV1Beta1Condition, infrav2.ControlPlaneEndpointNotSetV1Beta1Reason) &&
						isPresentAndFalseWithReason(key, instance, infrav2.HetznerClusterControlPlaneEndpointSetCondition, infrav2.HetznerClusterControlPlaneEndpointNotSetReason)
				}, timeout, time.Second).Should(BeTrue())
			})

			It("should work with capi.syself.com/allow-empty-control-plane-address annotation error condition custom port", func() {
				instance.Annotations = make(map[string]string)
				instance.Annotations[infrav2.AllowEmptyControlPlaneAddressAnnotation] = "true"
				instance.Spec.ControlPlaneLoadBalancer.Enabled = false
				instance.Spec.ControlPlaneEndpoint = infrav2.APIEndpoint{
					Host: "",
					Port: 1234,
				}
				Expect(testEnv.Create(ctx, instance)).To(Succeed())

				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, instance); err != nil {
						return false
					}

					return isPresentAndFalseWithReasonDeprecatedV1Beta1(key, instance, infrav2.ControlPlaneEndpointSetV1Beta1Condition, infrav2.ControlPlaneEndpointNotSetV1Beta1Reason) &&
						isPresentAndFalseWithReason(key, instance, infrav2.HetznerClusterControlPlaneEndpointSetCondition, infrav2.HetznerClusterControlPlaneEndpointNotSetReason)
				}, timeout, time.Second).Should(BeTrue())
			})

			It("should work with capi.syself.com/allow-empty-control-plane-address annotation success condition", func() {
				instance.Annotations = make(map[string]string)
				instance.Annotations[infrav2.AllowEmptyControlPlaneAddressAnnotation] = "true"
				instance.Spec.ControlPlaneLoadBalancer.Enabled = false
				instance.Spec.ControlPlaneEndpoint = infrav2.APIEndpoint{
					Host: "localhost",
					Port: 6443,
				}
				Expect(testEnv.Create(ctx, instance)).To(Succeed())

				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, instance); err != nil {
						return false
					}

					return isPresentAndTrueDeprecatedV1Beta1(key, instance, infrav2.ControlPlaneEndpointSetV1Beta1Condition) &&
						isPresentAndTrueWithReason(key, instance, infrav2.HetznerClusterControlPlaneEndpointSetCondition, infrav2.HetznerClusterControlPlaneEndpointSetReason)
				}, timeout, time.Second).Should(BeTrue())
			})

			It("should work with enabled load balancer success", func() {
				instance.Annotations = make(map[string]string)
				instance.Spec.ControlPlaneLoadBalancer.Enabled = true
				instance.Spec.ControlPlaneEndpoint = infrav2.APIEndpoint{
					Host: "localhost",
					Port: 6443,
				}
				Expect(testEnv.Create(ctx, instance)).To(Succeed())

				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, instance); err != nil {
						return false
					}

					return isPresentAndTrueDeprecatedV1Beta1(key, instance, infrav2.ControlPlaneEndpointSetV1Beta1Condition) &&
						isPresentAndTrueWithReason(key, instance, infrav2.HetznerClusterControlPlaneEndpointSetCondition, infrav2.HetznerClusterControlPlaneEndpointSetReason)
				}, timeout, time.Second).Should(BeTrue())
			})
		})

		Context("HetznerMachines belonging to the cluster", func() {
			var bootstrapSecret *corev1.Secret
			BeforeEach(func() {
				bootstrapSecret = getDefaultBootstrapSecret(namespace)
				Expect(testEnv.Create(ctx, bootstrapSecret)).To(Succeed())
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, bootstrapSecret)).To(Succeed())
			})

			It("sets owner references to those machines", func() {
				Expect(testEnv.Create(ctx, instance)).To(Succeed())

				By("creating hcloudmachine objects")

				machineCount := 3
				for i := 0; i < machineCount; i++ {
					Expect(createCapiAndHcloudMachines(ctx, testEnv, namespace, capiCluster.Name)).To(Succeed())
				}

				By("checking labels of HCloudMachine objects")

				Eventually(func() int {
					servers, err := hcloudClient.ListServers(ctx, hcloud.ServerListOpts{
						ListOpts: hcloud.ListOpts{
							LabelSelector: utils.LabelsToLabelSelector(map[string]string{instance.ClusterTagKey(): "owned"}),
						},
					})
					if err != nil {
						return -1
					}
					return len(servers)
				}, timeout).Should(Equal(machineCount))
			})
		})

		Context("Placement groups", func() {
			var bootstrapSecret *corev1.Secret

			BeforeEach(func() {
				// Create the bootstrap secret
				bootstrapSecret = getDefaultBootstrapSecret(namespace)
				Expect(testEnv.Create(ctx, bootstrapSecret)).To(Succeed())
			})

			AfterEach(func() {
				Expect(testEnv.Cleanup(ctx, bootstrapSecret)).To(Succeed())
			})

			DescribeTable("create and delete placement groups without error",
				func(placementGroups []infrav2.HCloudPlacementGroupSpec) {
					instance.Spec.HCloudPlacementGroups = placementGroups
					Expect(testEnv.Create(ctx, instance)).To(Succeed())

					Eventually(func() bool {
						return isPresentAndTrueDeprecatedV1Beta1(key, instance, infrav2.PlacementGroupsSyncedV1Beta1Condition) &&
							isPresentAndTrueWithReason(key, instance, infrav2.HetznerClusterPlacementGroupsSyncedCondition, infrav2.HetznerClusterPlacementGroupsSyncedReason)
					}, timeout).Should(BeTrue())

					By("checking for presence of HCloudPlacementGroup objects")

					Eventually(func() int {
						pgs, err := hcloudClient.ListPlacementGroups(ctx, hcloud.PlacementGroupListOpts{
							ListOpts: hcloud.ListOpts{
								LabelSelector: utils.LabelsToLabelSelector(map[string]string{instance.ClusterTagKey(): "owned"}),
							},
						})
						if err != nil {
							return -1
						}
						return len(pgs)
					}, timeout).Should(Equal(len(placementGroups)))
				},
				Entry("placement groups", []infrav2.HCloudPlacementGroupSpec{
					{
						Name: defaultPlacementGroupName,
						Type: "spread",
					},
					{
						Name: "md-0",
						Type: "spread",
					},
				}),
				Entry("no placement groups", []infrav2.HCloudPlacementGroupSpec{}),
			)

			Context("update placement groups", func() {
				BeforeEach(func() {
					Expect(testEnv.Create(ctx, instance)).To(Succeed())
				})

				DescribeTable("update placement groups",
					func(newPlacementGroupSpec []infrav2.HCloudPlacementGroupSpec) {
						ph, err := patch.NewHelper(instance, testEnv)
						Expect(err).ShouldNot(HaveOccurred())

						instance.Spec.HCloudPlacementGroups = newPlacementGroupSpec

						Eventually(func() error {
							return ph.Patch(ctx, instance, patch.WithStatusObservedGeneration{})
						}, timeout).Should(BeNil())

						Eventually(func() int {
							pgs, err := hcloudClient.ListPlacementGroups(ctx, hcloud.PlacementGroupListOpts{
								ListOpts: hcloud.ListOpts{
									LabelSelector: utils.LabelsToLabelSelector(map[string]string{instance.ClusterTagKey(): "owned"}),
								},
							})
							if err != nil {
								return -1
							}
							return len(pgs)
						}, timeout, time.Second).Should(Equal(len(newPlacementGroupSpec)))
					},
					Entry("one pg", []infrav2.HCloudPlacementGroupSpec{{Name: "md-0", Type: "spread"}}),
					Entry("no pgs", []infrav2.HCloudPlacementGroupSpec{}),
					Entry("three pgs", []infrav2.HCloudPlacementGroupSpec{
						{Name: "md-0", Type: "spread"},
						{Name: "md-1", Type: "spread"},
						{Name: "md-2", Type: "spread"},
					}),
				)
			})
		})

		Context("network", func() {
			var bootstrapSecret *corev1.Secret

			BeforeEach(func() {
				bootstrapSecret = getDefaultBootstrapSecret(namespace)
				Expect(testEnv.Create(ctx, bootstrapSecret)).To(Succeed())
			})

			AfterEach(func() {
				Expect(testEnv.Delete(ctx, bootstrapSecret)).To(Succeed())
			})

			It("creates a cluster with network and gets ready", func() {
				Expect(testEnv.Create(ctx, instance)).To(Succeed())

				Eventually(func() bool {
					return isPresentAndTrueDeprecatedV1Beta1(key, instance, infrav2.NetworkReadyV1Beta1Condition)
				}, timeout).Should(BeTrue())
			},
			)
		})
	})
})

func createCapiAndHcloudMachines(ctx context.Context, env *helpers.TestEnvironment, namespace, clusterName string) error {
	hcloudMachineName := utils.GenerateName(nil, "hcloud-machine")
	capiMachine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "capi-machine-",
			Namespace:    namespace,
			Finalizers:   []string{clusterv1.MachineFinalizer},
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
		},
		Spec: clusterv1.MachineSpec{
			ClusterName: clusterName,
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: "infrastructure.cluster.x-k8s.io",
				Kind:     "HCloudMachine",
				Name:     hcloudMachineName,
			},
			FailureDomain: defaultFailureDomain,
			Bootstrap: clusterv1.Bootstrap{
				DataSecretName: ptr.To("bootstrap-secret"),
			},
		},
	}
	if err := env.Create(ctx, capiMachine); err != nil {
		return err
	}

	hcloudMachine := &infrav1.HCloudMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hcloudMachineName,
			Namespace: namespace,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: clusterName},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Machine",
					Name:       capiMachine.Name,
					UID:        capiMachine.UID,
				},
			},
		},
		Spec: infrav1.HCloudMachineSpec{
			ImageName: "my-control-plane",
			Type:      "cpx32",
		},
	}
	return env.Create(ctx, hcloudMachine)
}

var _ = Describe("Hetzner secret", func() {
	var (
		testNs         *corev1.Namespace
		hetznerCluster *infrav2.HetznerCluster
		capiCluster    *clusterv1.Cluster

		hetznerSecret *corev1.Secret

		key                client.ObjectKey
		hetznerClusterName string
	)

	BeforeEach(func() {
		var err error
		testNs, err = testEnv.ResetAndCreateNamespace(ctx, "hetzner-secret")
		Expect(err).NotTo(HaveOccurred())

		hetznerClusterName = utils.GenerateName(nil, "hetzner-cluster-test")
		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test1-",
				Namespace:    testNs.Name,
				Finalizers:   []string{clusterv1.ClusterFinalizer},
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: infrav2.GroupVersion.Group,
					Kind:     "HetznerCluster",
					Name:     hetznerClusterName,
				},
			},
		}
		Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())

		hetznerCluster = &infrav2.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hetznerClusterName,
				Namespace: testNs.Name,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       capiCluster.Name,
						UID:        capiCluster.UID,
					},
				},
			},
			Spec: getDefaultHetznerClusterSpec(),
		}
		Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())

		key = client.ObjectKey{Namespace: hetznerCluster.Namespace, Name: hetznerCluster.Name}
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, hetznerCluster, capiCluster, hetznerSecret)).To(Succeed())
	})

	DescribeTable("test different hetzner secret",
		func(secretFunc func() *corev1.Secret, expectedV1Beta1Reason string, expectedReason string) {
			hetznerSecret = secretFunc()
			Expect(testEnv.Create(ctx, hetznerSecret)).To(Succeed())

			Eventually(func() bool {
				return isPresentAndFalseWithReasonDeprecatedV1Beta1(key, hetznerCluster, infrav2.HCloudTokenAvailableV1Beta1Condition, expectedV1Beta1Reason) &&
					isPresentAndFalseWithReason(key, hetznerCluster, infrav2.HCloudTokenAvailableCondition, expectedReason)
			}, timeout, time.Second).Should(BeTrue())
			Expect(testEnv.Cleanup(ctx, hetznerSecret)).To(Succeed())
		},
		Entry("no Hetzner secret/wrong reference", func() *corev1.Secret {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wrong-name",
					Namespace: testNs.Name,
				},
				Data: map[string][]byte{
					"hcloud": []byte("my-token"),
				},
			}
		}, infrav2.HetznerSecretUnreachableV1Beta1Reason, infrav2.HCloudTokenSecretUnreachableReason),
		Entry("empty hcloud token", func() *corev1.Secret {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hetzner-secret",
					Namespace: testNs.Name,
				},
				Data: map[string][]byte{
					"hcloud": []byte(""),
				},
			}
		}, infrav2.HCloudCredentialsInvalidV1Beta1Reason, infrav2.HCloudTokenInvalidReason),
		Entry("wrong key in secret", func() *corev1.Secret {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hetzner-secret",
					Namespace: testNs.Name,
				},
				Data: map[string][]byte{
					"wrongkey": []byte("my-token"),
				},
			}
		}, infrav2.HCloudCredentialsInvalidV1Beta1Reason, infrav2.HCloudTokenInvalidReason),
	)
})

var _ = Describe("HetznerCluster validation", func() {
	var (
		hetznerCluster *infrav2.HetznerCluster
		testNs         *corev1.Namespace
	)
	BeforeEach(func() {
		var err error
		testNs, err = testEnv.ResetAndCreateNamespace(ctx, "hcloudmachine-validation")
		Expect(err).NotTo(HaveOccurred())
	})
	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs, hetznerCluster)).To(Succeed())
	})

	Context("validate create", func() {
		BeforeEach(func() {
			hetznerCluster = &infrav2.HetznerCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hcloud-validation-machine",
					Namespace: testNs.Name,
				},
				Spec: getDefaultHetznerClusterSpec(),
			}
		})

		It("should succeed with valid spec", func() {
			Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())
		})

		It("should succeed with capi.syself.com/allow-empty-control-plane-address annotation", func() {
			hetznerCluster.Annotations = make(map[string]string)
			hetznerCluster.Annotations[infrav2.AllowEmptyControlPlaneAddressAnnotation] = "true"
			hetznerCluster.Spec.ControlPlaneRegions = []infrav2.Region{}
			hetznerCluster.Spec.ControlPlaneLoadBalancer.Enabled = false
			hetznerCluster.Spec.ControlPlaneEndpoint.Port = 443
			hetznerCluster.Spec.ControlPlaneEndpoint.Host = "localhost"
			Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())
		})

		It("should succeed with capi.syself.com/allow-empty-control-plane-address annotation empty host", func() {
			hetznerCluster.Annotations = make(map[string]string)
			hetznerCluster.Annotations[infrav2.AllowEmptyControlPlaneAddressAnnotation] = "true"
			hetznerCluster.Spec.ControlPlaneRegions = []infrav2.Region{}
			hetznerCluster.Spec.ControlPlaneLoadBalancer.Enabled = false
			hetznerCluster.Spec.ControlPlaneEndpoint.Port = 443
			hetznerCluster.Spec.ControlPlaneEndpoint.Host = ""
			Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())
		})

		It("should succeed with capi.syself.com/allow-empty-control-plane-address annotation empty ControlPlaneEndpoint", func() {
			hetznerCluster.Annotations = make(map[string]string)
			hetznerCluster.Annotations[infrav2.AllowEmptyControlPlaneAddressAnnotation] = "true"
			hetznerCluster.Spec.ControlPlaneRegions = []infrav2.Region{}
			hetznerCluster.Spec.ControlPlaneLoadBalancer.Enabled = false
			hetznerCluster.Spec.ControlPlaneEndpoint = infrav2.APIEndpoint{}
			Expect(testEnv.Create(ctx, hetznerCluster)).To(Succeed())
		})

		It("should fail without a wrong controlPlaneRegion name", func() {
			hetznerCluster.Spec.ControlPlaneRegions = append(hetznerCluster.Spec.ControlPlaneRegions, infrav2.Region("wrong-region"))
			Expect(testEnv.Create(ctx, hetznerCluster)).ToNot(Succeed())
		})

		It("should fail with an SSHKey without name", func() {
			hetznerCluster.Spec.SSHKeys.HCloud = append(hetznerCluster.Spec.SSHKeys.HCloud, infrav2.SSHKey{})
			Expect(testEnv.Create(ctx, hetznerCluster)).ToNot(Succeed())
		})

		It("should fail with an empty controlPlaneLoadBalancer region", func() {
			hetznerCluster.Spec.ControlPlaneLoadBalancer.Region = ""
			Expect(testEnv.Create(ctx, hetznerCluster)).ToNot(Succeed())
		})

		It("should fail with an empty placementGroup name", func() {
			hetznerCluster.Spec.HCloudPlacementGroups = append(hetznerCluster.Spec.HCloudPlacementGroups, infrav2.HCloudPlacementGroupSpec{})
			Expect(testEnv.Create(ctx, hetznerCluster)).ToNot(Succeed())
		})

		It("should fail with a wrong placementGroup type", func() {
			hetznerCluster.Spec.HCloudPlacementGroups = append(hetznerCluster.Spec.HCloudPlacementGroups, infrav2.HCloudPlacementGroupSpec{
				Name: "newName",
				Type: "wrong-type",
			})
			Expect(testEnv.Create(ctx, hetznerCluster)).ToNot(Succeed())
		})
	})
})

var _ = Describe("reconcileRateLimit", func() {
	var hetznerCluster *infrav2.HetznerCluster
	BeforeEach(func() {
		hetznerCluster = &infrav2.HetznerCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rate-limit-cluster",
				Namespace: "default",
			},
			Spec: getDefaultHetznerClusterSpec(),
		}
	})

	It("returns wait==true if rate limit exceeded is set and time is not over", func() {
		conditions.Set(hetznerCluster, metav1.Condition{
			Type:               infrav2.HCloudRateLimitExceededCondition,
			Status:             metav1.ConditionTrue,
			Reason:             infrav2.HCloudRateLimitExceededReason,
			LastTransitionTime: metav1.Now(),
		})
		Expect(reconcileRateLimit(hetznerCluster, testEnv.RateLimitWaitTime)).To(BeTrue())
	})

	It("returns wait==false if rate limit exceeded is set and time is over", func() {
		conditions.Set(hetznerCluster, metav1.Condition{
			Type:               infrav2.HCloudRateLimitExceededCondition,
			Status:             metav1.ConditionTrue,
			Reason:             infrav2.HCloudRateLimitExceededReason,
			LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour)),
		})
		Expect(reconcileRateLimit(hetznerCluster, testEnv.RateLimitWaitTime)).To(BeFalse())
		Expect(conditions.Has(hetznerCluster, infrav2.HCloudRateLimitExceededCondition)).To(BeFalse())
	})

	It("returns wait==true if HCloudRateLimitExceeded condition is True and time is not over (v1beta2)", func() {
		hcloudMachine := &infrav1.HCloudMachine{}
		v1beta1conditions.MarkFalse(hcloudMachine, infrav1.HetznerAPIReachableCondition, infrav1.RateLimitExceededReason, clusterv1beta1.ConditionSeverityWarning, "")
		v1beta2conditions.Set(hcloudMachine, metav1.Condition{
			Type:               infrav1.HCloudRateLimitExceededV1Beta2Condition,
			Status:             metav1.ConditionTrue,
			Reason:             infrav1.HCloudRateLimitExceededV1Beta2Reason,
			LastTransitionTime: metav1.Now(),
		})
		Expect(reconcileRateLimitV1Beta1(hcloudMachine, testEnv.RateLimitWaitTime)).To(BeTrue())
		rateLimitCond := v1beta2conditions.Get(hcloudMachine, infrav1.HCloudRateLimitExceededV1Beta2Condition)
		Expect(rateLimitCond).NotTo(BeNil())
		Expect(rateLimitCond.Status).To(Equal(metav1.ConditionTrue))
		Expect(rateLimitCond.Reason).To(Equal(infrav1.HCloudRateLimitExceededV1Beta2Reason))
	})

	It("removes HCloudRateLimitExceeded condition and returns wait==false when wait time is over (v1beta2)", func() {
		hcloudMachine := &infrav1.HCloudMachine{}
		v1beta1conditions.MarkFalse(hcloudMachine, infrav1.HetznerAPIReachableCondition, infrav1.RateLimitExceededReason, clusterv1beta1.ConditionSeverityWarning, "")
		conditionList := hcloudMachine.GetConditions()
		conditionList[0].LastTransitionTime = metav1.NewTime(time.Now().Add(-time.Hour))
		v1beta2conditions.Set(hcloudMachine, metav1.Condition{
			Type:               infrav1.HCloudRateLimitExceededV1Beta2Condition,
			Status:             metav1.ConditionTrue,
			Reason:             infrav1.HCloudRateLimitExceededV1Beta2Reason,
			LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour)),
		})
		Expect(reconcileRateLimitV1Beta1(hcloudMachine, testEnv.RateLimitWaitTime)).To(BeFalse())
		// Condition must be deleted (not just set to False) so the next API call
		// determines the real rate-limit status instead of assuming it is gone.
		Expect(v1beta2conditions.Has(hcloudMachine, infrav1.HCloudRateLimitExceededV1Beta2Condition)).To(BeFalse())
	})

	It("returns wait==false if rate limit condition is present but not exceeded", func() {
		conditions.Set(hetznerCluster, metav1.Condition{
			Type:               infrav2.HCloudRateLimitExceededCondition,
			Status:             metav1.ConditionFalse,
			Reason:             "NotExceeded",
			LastTransitionTime: metav1.Now(),
		})
		Expect(reconcileRateLimit(hetznerCluster, testEnv.RateLimitWaitTime)).To(BeFalse())
	})

	It("returns wait==false if rate limit condition is not set", func() {
		Expect(reconcileRateLimit(hetznerCluster, testEnv.RateLimitWaitTime)).To(BeFalse())
	})

	It("returns wait==true if HCloudRateLimitExceeded condition is True and time is not over (v1beta2)", func() {
		deprecatedv1beta1conditions.MarkFalse(hetznerCluster, infrav2.HetznerAPIReachableV1Beta1Condition, infrav2.RateLimitExceededV1Beta1Reason, clusterv1.ConditionSeverityWarning, "")
		conditions.Set(hetznerCluster, metav1.Condition{
			Type:               infrav2.HCloudRateLimitExceededCondition,
			Status:             metav1.ConditionTrue,
			Reason:             infrav2.HCloudRateLimitExceededReason,
			LastTransitionTime: metav1.Now(),
		})
		Expect(reconcileRateLimit(hetznerCluster, testEnv.RateLimitWaitTime)).To(BeTrue())
		rateLimitCond := conditions.Get(hetznerCluster, infrav2.HCloudRateLimitExceededCondition)
		Expect(rateLimitCond).NotTo(BeNil())
		Expect(rateLimitCond.Status).To(Equal(metav1.ConditionTrue))
		Expect(rateLimitCond.Reason).To(Equal(infrav2.HCloudRateLimitExceededReason))
	})

	It("removes HCloudRateLimitExceeded condition and returns wait==false when wait time is over (v1beta2)", func() {
		deprecatedv1beta1conditions.MarkFalse(hetznerCluster, infrav2.HetznerAPIReachableV1Beta1Condition, infrav2.RateLimitExceededV1Beta1Reason, clusterv1.ConditionSeverityWarning, "")
		conditions.Set(hetznerCluster, metav1.Condition{
			Type:               infrav2.HCloudRateLimitExceededCondition,
			Status:             metav1.ConditionTrue,
			Reason:             infrav2.HCloudRateLimitExceededReason,
			LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour)),
		})
		Expect(reconcileRateLimit(hetznerCluster, testEnv.RateLimitWaitTime)).To(BeFalse())
		// Condition must be deleted (not just set to False) so the next API call
		// determines the real rate-limit status instead of assuming it is gone.
		Expect(conditions.Has(hetznerCluster, infrav2.HCloudRateLimitExceededCondition)).To(BeFalse())
	})
})

func TestSetControlPlaneEndpoint(t *testing.T) {
	t.Run("return false and don't make changes to ControlPlaneEndpoint if load balancer is not enabled and ControlPlaneEndpoint is nil", func(t *testing.T) {
		hetznerCluster := &infrav2.HetznerCluster{
			Spec: infrav2.HetznerClusterSpec{
				ControlPlaneLoadBalancer: infrav2.LoadBalancerSpec{
					Enabled: false,
				},
				ControlPlaneEndpoint: infrav2.APIEndpoint{},
			},
		}

		processControlPlaneEndpoint(hetznerCluster)

		if hetznerCluster.Spec.ControlPlaneEndpoint != (infrav2.APIEndpoint{}) {
			t.Fatalf("ControlPlaneEndpoint must be nil")
		}

		if isHetznerClusterProvisioned(hetznerCluster) != false {
			t.Fatal("return value should be false")
		}
	})

	t.Run("return true and don't make changes to ControlPlaneEndpoint if load balancer is not enabled and ControlPlaneEndpoint is not nil", func(t *testing.T) {
		hetznerCluster := &infrav2.HetznerCluster{
			Spec: infrav2.HetznerClusterSpec{
				ControlPlaneLoadBalancer: infrav2.LoadBalancerSpec{
					Enabled: false,
				},
				ControlPlaneEndpoint: infrav2.APIEndpoint{
					Host: "xyz",
					Port: 1234,
				},
			},
		}

		processControlPlaneEndpoint(hetznerCluster)

		if hetznerCluster.Spec.ControlPlaneEndpoint == (infrav2.APIEndpoint{}) {
			t.Fatalf("ControlPlaneEndpoint must not be nil")
		}

		if hetznerCluster.Spec.ControlPlaneEndpoint.Host != "xyz" {
			t.Fatalf("Wrong input for host. Got: %s, Want: 'xyz'", hetznerCluster.Spec.ControlPlaneEndpoint.Host)
		}

		if hetznerCluster.Spec.ControlPlaneEndpoint.Port != 1234 {
			t.Fatalf("Value of Port should not change. Got: %d, Want: 1234", hetznerCluster.Spec.ControlPlaneEndpoint.Port)
		}

		if isHetznerClusterProvisioned(hetznerCluster) != true {
			t.Fatalf("return value should be true")
		}
	})

	t.Run("return false if load balancer is enabled and IPv4 is '<nil>'. ControlPlaneEndpoint should not change", func(t *testing.T) {
		hetznerCluster := &infrav2.HetznerCluster{
			Spec: infrav2.HetznerClusterSpec{
				ControlPlaneLoadBalancer: infrav2.LoadBalancerSpec{
					Enabled: true,
				},
				ControlPlaneEndpoint: infrav2.APIEndpoint{},
			},
			Status: infrav2.HetznerClusterStatus{
				ControlPlaneLoadBalancer: &infrav2.LoadBalancerStatus{
					IPv4: "<nil>",
				},
			},
		}

		processControlPlaneEndpoint(hetznerCluster)

		if hetznerCluster.Spec.ControlPlaneEndpoint != (infrav2.APIEndpoint{}) {
			t.Fatalf("ControlPlaneEndpoint should not change. It should remain nil")
		}

		if isHetznerClusterProvisioned(hetznerCluster) != false {
			t.Fatalf("return value should be false")
		}

		if !deprecatedv1beta1conditions.Has(hetznerCluster, infrav2.ControlPlaneEndpointSetV1Beta1Condition) {
			t.Fatalf("ControlPlaneEndpointSetCondition should exist")
		}

		condition := deprecatedv1beta1conditions.Get(hetznerCluster, infrav2.ControlPlaneEndpointSetV1Beta1Condition)
		if condition.Status != corev1.ConditionFalse {
			t.Fatalf("condition status should be false")
		}
	})

	t.Run("return true if load balancer is enabled, IPv4 is not nil, and ControlPlaneEndpoint is nil. Values of ControlPlaneEndpoint.Host and ControlPlaneEndpoint.Port will get updated", func(t *testing.T) {
		hetznerCluster := &infrav2.HetznerCluster{
			Spec: infrav2.HetznerClusterSpec{
				ControlPlaneLoadBalancer: infrav2.LoadBalancerSpec{
					Enabled: true,
					Port:    11,
				},
				ControlPlaneEndpoint: infrav2.APIEndpoint{},
			},
			Status: infrav2.HetznerClusterStatus{
				ControlPlaneLoadBalancer: &infrav2.LoadBalancerStatus{
					IPv4: "xyz",
				},
			},
		}

		processControlPlaneEndpoint(hetznerCluster)

		if hetznerCluster.Status.ControlPlaneLoadBalancer.IPv4 != "xyz" {
			t.Fatalf("Wrong input for hetznerCluster.Status.ControlPlaneLoadBalancer.IPv4. Got: %s, Want: 'xyz'", hetznerCluster.Status.ControlPlaneLoadBalancer.IPv4)
		}

		if hetznerCluster.Spec.ControlPlaneEndpoint == (infrav2.APIEndpoint{}) {
			t.Fatal("Value of ControlPlaneEndpoint should have been changed. It should not remain nil")
		}

		// Values of hetznerCluster.Spec.ControlPlaneEndpoint.Host and hetznerCluster.Spec.ControlPlaneEndpoint.Port should change after execution of the function SetControlPlaneEndpoint()
		// They should be the same as hetznerCluster.Status.ControlPlaneLoadBalancer.IPv4 for Host (Spec.ControlPlaneEndpoint.Host) and hetznerCluster.Spec.ControlPlaneLoadBalancer.Port for Port (Spec.ControlPlaneEndpoint.Port)
		if hetznerCluster.Spec.ControlPlaneEndpoint.Host != hetznerCluster.Status.ControlPlaneLoadBalancer.IPv4 {
			t.Fatalf("Wrong value for Host set. Got: %s, Want: %s", hetznerCluster.Spec.ControlPlaneEndpoint.Host, hetznerCluster.Status.ControlPlaneLoadBalancer.IPv4)
		}

		if hetznerCluster.Spec.ControlPlaneEndpoint.Port != int32(hetznerCluster.Spec.ControlPlaneLoadBalancer.Port) { //nolint:gosec // Validation for the port range (1 to 65535) is already done via kubebuilder.
			t.Fatalf("Wrong value for Port set. Got: %d, Want: %d", hetznerCluster.Spec.ControlPlaneEndpoint.Port, int32(hetznerCluster.Spec.ControlPlaneLoadBalancer.Port)) //nolint:gosec // Validation for the port range (1 to 65535) is already done via kubebuilder.
		}

		if isHetznerClusterProvisioned(hetznerCluster) != true {
			t.Fatalf("return value should be true")
		}
	})

	t.Run("return true if load balancer is enabled and IPv4 is not nil, ControlPlaneEndpoint.Host is an empty string and ControlPlaneEndpoint.Port is 0. Values of ControlPlaneEndpoint.Host and ControlPlaneEndpoint.Port should update", func(t *testing.T) {
		hetznerCluster := &infrav2.HetznerCluster{
			Spec: infrav2.HetznerClusterSpec{
				ControlPlaneLoadBalancer: infrav2.LoadBalancerSpec{
					Enabled: true,
					Port:    21,
				},
				ControlPlaneEndpoint: infrav2.APIEndpoint{
					Host: "",
					Port: 0,
				},
			},
			Status: infrav2.HetznerClusterStatus{
				ControlPlaneLoadBalancer: &infrav2.LoadBalancerStatus{
					IPv4: "xyz",
				},
			},
		}

		processControlPlaneEndpoint(hetznerCluster)

		if hetznerCluster.Spec.ControlPlaneEndpoint.Host != hetznerCluster.Status.ControlPlaneLoadBalancer.IPv4 {
			t.Fatalf("Wrong value for Host set. Got: %s, Want: %s", hetznerCluster.Spec.ControlPlaneEndpoint.Host, hetznerCluster.Status.ControlPlaneLoadBalancer.IPv4)
		}

		if hetznerCluster.Spec.ControlPlaneEndpoint.Port != int32(hetznerCluster.Spec.ControlPlaneLoadBalancer.Port) { //nolint:gosec // Validation for the port range (1 to 65535) is already done via kubebuilder.
			t.Fatalf("Wrong value for Port set. Got: %d, Want: %d", hetznerCluster.Spec.ControlPlaneEndpoint.Port, int32(hetznerCluster.Spec.ControlPlaneLoadBalancer.Port)) //nolint:gosec // Validation for the port range (1 to 65535) is already done via kubebuilder.
		}

		if isHetznerClusterProvisioned(hetznerCluster) != true {
			t.Fatalf("return value should be true")
		}
	})

	t.Run("return true if load balancer is enabled and IPv4 is not nil, ControlPlaneEndpoint.Host is 'xyz' and ControlPlaneEndpoint.Port is 0. Value of ControlPlaneEndpoint.Host will not change and ControlPlaneEndpoint.Port should update", func(t *testing.T) {
		hetznerCluster := &infrav2.HetznerCluster{
			Spec: infrav2.HetznerClusterSpec{
				ControlPlaneLoadBalancer: infrav2.LoadBalancerSpec{
					Enabled: true,
					Port:    21,
				},
				ControlPlaneEndpoint: infrav2.APIEndpoint{
					Host: "xyz",
					Port: 0,
				},
			},
			Status: infrav2.HetznerClusterStatus{
				ControlPlaneLoadBalancer: &infrav2.LoadBalancerStatus{
					IPv4: "xyz",
				},
			},
		}

		processControlPlaneEndpoint(hetznerCluster)

		if hetznerCluster.Spec.ControlPlaneEndpoint.Host != "xyz" {
			t.Fatalf("Wrong value for Host set. Got: %s, Want: 'xyz'", hetznerCluster.Spec.ControlPlaneEndpoint.Host)
		}

		if hetznerCluster.Spec.ControlPlaneEndpoint.Port != int32(hetznerCluster.Spec.ControlPlaneLoadBalancer.Port) { //nolint:gosec // Validation for the port range (1 to 65535) is already done via kubebuilder.
			t.Fatalf("Wrong value for Port set. Got: %d, Want: %d", hetznerCluster.Spec.ControlPlaneEndpoint.Port, int32(hetznerCluster.Spec.ControlPlaneLoadBalancer.Port)) //nolint:gosec // Validation for the port range (1 to 65535) is already done via kubebuilder.
		}

		if isHetznerClusterProvisioned(hetznerCluster) != true {
			t.Fatalf("return value should be true")
		}
	})

	t.Run("return true if load balancer is enabled and IPv4 is not nil, ControlPlaneEndpoint.Host is an empty string and ControlPlaneEndpoint.Port is 21. Value of ControlPlaneEndpoint.Host will change and ControlPlaneEndpoint.Port should remain same", func(t *testing.T) {
		hetznerCluster := &infrav2.HetznerCluster{
			Spec: infrav2.HetznerClusterSpec{
				ControlPlaneLoadBalancer: infrav2.LoadBalancerSpec{
					Enabled: true,
					Port:    21,
				},
				ControlPlaneEndpoint: infrav2.APIEndpoint{
					Host: "",
					Port: 21,
				},
			},
			Status: infrav2.HetznerClusterStatus{
				ControlPlaneLoadBalancer: &infrav2.LoadBalancerStatus{
					IPv4: "xyz",
				},
			},
		}

		processControlPlaneEndpoint(hetznerCluster)

		if hetznerCluster.Spec.ControlPlaneEndpoint.Host != hetznerCluster.Status.ControlPlaneLoadBalancer.IPv4 {
			t.Fatalf("Wrong value for Host set. Got: %s, Want: %s", hetznerCluster.Spec.ControlPlaneEndpoint.Host, hetznerCluster.Status.ControlPlaneLoadBalancer.IPv4)
		}

		if hetznerCluster.Spec.ControlPlaneEndpoint.Port != 21 {
			t.Fatalf("Wrong value for Port set. Got: %d, Want: 21", hetznerCluster.Spec.ControlPlaneEndpoint.Port)
		}

		if isHetznerClusterProvisioned(hetznerCluster) != true {
			t.Fatalf("return value should be true")
		}
	})

	t.Run("return true if load balancer is enabled and IPv4 is not nil, ControlPlaneEndpoint.Host is 'xyz' and ControlPlaneEndpoint.Port is 21. Value of ControlPlaneEndpoint.Host and ControlPlaneEndpoint.Port should remain unchanged", func(t *testing.T) {
		hetznerCluster := &infrav2.HetznerCluster{
			Spec: infrav2.HetznerClusterSpec{
				ControlPlaneLoadBalancer: infrav2.LoadBalancerSpec{
					Enabled: true,
					Port:    21,
				},
				ControlPlaneEndpoint: infrav2.APIEndpoint{
					Host: "xyz",
					Port: 21,
				},
			},
			Status: infrav2.HetznerClusterStatus{
				ControlPlaneLoadBalancer: &infrav2.LoadBalancerStatus{
					IPv4: "xyz",
				},
			},
		}

		processControlPlaneEndpoint(hetznerCluster)

		if hetznerCluster.Spec.ControlPlaneEndpoint.Host != "xyz" {
			t.Fatalf("Wrong value for Host set. Got: %s, Want: 'xyz'", hetznerCluster.Spec.ControlPlaneEndpoint.Host)
		}

		if hetznerCluster.Spec.ControlPlaneEndpoint.Port != 21 {
			t.Fatalf("Wrong value for Port set. Got: %d, Want: 21", hetznerCluster.Spec.ControlPlaneEndpoint.Port)
		}

		if isHetznerClusterProvisioned(hetznerCluster) != true {
			t.Fatalf("return value should be true")
		}
	})
}
