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

package machinetemplate

import (
	"context"
	"testing"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/fake"
)

func TestGetCapacity(t *testing.T) {
	serverTypes := []*hcloud.ServerType{
		{Name: "cpx11", Cores: 2, Memory: 2},
		{Name: "cpx21", Cores: 3, Memory: 4},
	}

	t.Run("known server type", func(t *testing.T) {
		capacity, found, err := getCapacity(serverTypes, "cpx21")
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, "3", capacity.Cpu().String())
		require.Equal(t, "4G", capacity.Memory().String())
	})

	t.Run("unknown server type", func(t *testing.T) {
		capacity, found, err := getCapacity(serverTypes, "does-not-exist")
		require.NoError(t, err)
		require.False(t, found)
		require.Nil(t, capacity)
	})
}

func TestReconcileServerType(t *testing.T) {
	newService := func(serverType infrav1.HCloudMachineType) *Service {
		mt := &infrav1.HCloudMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec: infrav1.HCloudMachineTemplateSpec{
				Template: infrav1.HCloudMachineTemplateResource{
					Spec: infrav1.HCloudMachineSpec{Type: serverType},
				},
			},
		}
		s, err := scope.NewHCloudMachineTemplateScope(scope.HCloudMachineTemplateScopeParams{
			HCloudMachineTemplate: mt,
			HCloudClient:          fake.NewHCloudClientFactory().NewClient(""),
		})
		require.NoError(t, err)
		return NewService(s)
	}

	t.Run("known server type sets capacity and Available=True", func(t *testing.T) {
		s := newService("cpx32")
		_, err := s.Reconcile(context.Background())
		require.NoError(t, err)

		require.NotNil(t, s.scope.HCloudMachineTemplate.Status.Capacity.Cpu())
		cond := v1beta2conditions.Get(s.scope.HCloudMachineTemplate, infrav1.HCloudMachineTemplateAvailableV1Beta2Condition)
		require.NotNil(t, cond)
		require.Equal(t, metav1.ConditionTrue, cond.Status)
		require.Equal(t, infrav1.HCloudMachineTemplateAvailableV1Beta2Reason, cond.Reason)
	})

	t.Run("unknown server type sets Available=False with reason ServerTypeNotFound", func(t *testing.T) {
		s := newService("does-not-exist")
		_, err := s.Reconcile(context.Background())
		require.NoError(t, err)

		require.Nil(t, s.scope.HCloudMachineTemplate.Status.Capacity)
		cond := v1beta2conditions.Get(s.scope.HCloudMachineTemplate, infrav1.HCloudMachineTemplateAvailableV1Beta2Condition)
		require.NotNil(t, cond)
		require.Equal(t, metav1.ConditionFalse, cond.Status)
		require.Equal(t, infrav1.HCloudMachineTemplateServerTypeNotFoundV1Beta2Reason, cond.Reason)
	})
}
