package v1beta1

import (
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrav1beta2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

func TestAllCRDConversionsRoundTrip(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		newSource func() conversion.Convertible
		newHub    func() conversion.Hub
	}{
		{
			name: "HetznerClusterTemplate",
			newSource: func() conversion.Convertible {
				return &HetznerClusterTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-template"},
					Spec: HetznerClusterTemplateSpec{
						Template: HetznerClusterTemplateResource{
							Spec: HetznerClusterSpec{
								ControlPlaneRegions: []Region{"nbg"},
								SSHKeys:             HetznerSSHKeys{},
							},
						},
					},
				}
			},
			newHub: func() conversion.Hub {
				return &infrav1beta2.HetznerClusterTemplate{}
			},
		},
		{
			name: "HCloudMachine",
			newSource: func() conversion.Convertible {
				return &HCloudMachine{
					ObjectMeta: metav1.ObjectMeta{Name: "machine"},
					Spec: HCloudMachineSpec{
						Type:      "cx11",
						ImageName: "image",
					},
					Status: HCloudMachineStatus{
						Ready: true,
					},
				}
			},
			newHub: func() conversion.Hub {
				return &infrav1beta2.HCloudMachine{}
			},
		},
		{
			name: "HCloudMachineTemplate",
			newSource: func() conversion.Convertible {
				return &HCloudMachineTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: "machine-template"},
					Spec: HCloudMachineTemplateSpec{
						Template: HCloudMachineTemplateResource{
							Spec: HCloudMachineSpec{
								Type:      "cx21",
								ImageName: "image",
							},
						},
					},
				}
			},
			newHub: func() conversion.Hub {
				return &infrav1beta2.HCloudMachineTemplate{}
			},
		},
		{
			name: "HCloudRemediation",
			newSource: func() conversion.Convertible {
				return &HCloudRemediation{
					ObjectMeta: metav1.ObjectMeta{Name: "remediation"},
					Spec: HCloudRemediationSpec{
						Strategy: &RemediationStrategy{
							Type:       RemediationTypeReboot,
							RetryLimit: 1,
							Timeout:    &metav1.Duration{Duration: time.Minute},
						},
					},
					Status: HCloudRemediationStatus{
						Phase: PhaseRunning,
						Conditions: clusterv1.Conditions{
							{
								Type:               clusterv1.ConditionType("Ready"),
								Status:             corev1.ConditionTrue,
								LastTransitionTime: metav1.Now(),
							},
						},
					},
				}
			},
			newHub: func() conversion.Hub {
				return &infrav1beta2.HCloudRemediation{}
			},
		},
		{
			name: "HCloudRemediationTemplate",
			newSource: func() conversion.Convertible {
				return &HCloudRemediationTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: "remediation-template"},
					Spec: HCloudRemediationTemplateSpec{
						Template: HCloudRemediationTemplateResource{
							Spec: HCloudRemediationSpec{
								Strategy: &RemediationStrategy{
									Type:    RemediationTypeReboot,
									Timeout: &metav1.Duration{Duration: time.Minute},
								},
							},
						},
					},
				}
			},
			newHub: func() conversion.Hub {
				return &infrav1beta2.HCloudRemediationTemplate{}
			},
		},
		{
			name: "HetznerBareMetalHost",
			newSource: func() conversion.Convertible {
				return &HetznerBareMetalHost{
					ObjectMeta: metav1.ObjectMeta{Name: "host"},
					Spec: HetznerBareMetalHostSpec{
						ServerID: 42,
						Status: ControllerGeneratedStatus{
							HetznerClusterRef: "cluster",
							Conditions: clusterv1.Conditions{
								{
									Type:               clusterv1.ConditionType("Ready"),
									Status:             corev1.ConditionTrue,
									LastTransitionTime: metav1.Now(),
								},
							},
						},
					},
				}
			},
			newHub: func() conversion.Hub {
				return &infrav1beta2.HetznerBareMetalHost{}
			},
		},
		{
			name: "HetznerBareMetalMachine",
			newSource: func() conversion.Convertible {
				return &HetznerBareMetalMachine{
					ObjectMeta: metav1.ObjectMeta{Name: "baremetal-machine"},
					Spec: HetznerBareMetalMachineSpec{
						InstallImage: InstallImage{
							Image: Image{Name: "image"},
						},
					},
				}
			},
			newHub: func() conversion.Hub {
				return &infrav1beta2.HetznerBareMetalMachine{}
			},
		},
		{
			name: "HetznerBareMetalMachineTemplate",
			newSource: func() conversion.Convertible {
				return &HetznerBareMetalMachineTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: "baremetal-machine-template"},
					Spec: HetznerBareMetalMachineTemplateSpec{
						Template: HetznerBareMetalMachineTemplateResource{
							Spec: HetznerBareMetalMachineSpec{
								InstallImage: InstallImage{
									Image: Image{Name: "template-image"},
								},
							},
						},
					},
				}
			},
			newHub: func() conversion.Hub {
				return &infrav1beta2.HetznerBareMetalMachineTemplate{}
			},
		},
		{
			name: "HetznerBareMetalRemediation",
			newSource: func() conversion.Convertible {
				return &HetznerBareMetalRemediation{
					ObjectMeta: metav1.ObjectMeta{Name: "baremetal-remediation"},
					Spec: HetznerBareMetalRemediationSpec{
						Strategy: &RemediationStrategy{
							Type:    RemediationTypeReboot,
							Timeout: &metav1.Duration{Duration: time.Minute},
						},
					},
					Status: HetznerBareMetalRemediationStatus{
						Phase: PhaseWaiting,
					},
				}
			},
			newHub: func() conversion.Hub {
				return &infrav1beta2.HetznerBareMetalRemediation{}
			},
		},
		{
			name: "HetznerBareMetalRemediationTemplate",
			newSource: func() conversion.Convertible {
				return &HetznerBareMetalRemediationTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: "baremetal-remediation-template"},
					Spec: HetznerBareMetalRemediationTemplateSpec{
						Template: HetznerBareMetalRemediationTemplateResource{
							Spec: HetznerBareMetalRemediationSpec{
								Strategy: &RemediationStrategy{
									Type:    RemediationTypeReboot,
									Timeout: &metav1.Duration{Duration: time.Minute},
								},
							},
						},
					},
					Status: HetznerBareMetalRemediationTemplateStatus{
						Status: HetznerBareMetalRemediationStatus{
							Phase: PhaseWaiting,
						},
					},
				}
			},
			newHub: func() conversion.Hub {
				return &infrav1beta2.HetznerBareMetalRemediationTemplate{}
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := tc.newSource()
			hub := tc.newHub()
			runConversionRoundTrip(t, src, hub)
		})
	}
}

func runConversionRoundTrip(t *testing.T, src conversion.Convertible, hub conversion.Hub) {
	t.Helper()

	if err := src.ConvertTo(hub); err != nil {
		t.Fatalf("ConvertTo failed: %v", err)
	}

	dst := reflect.New(reflect.TypeOf(src).Elem()).Interface().(conversion.Convertible)
	if reflect.TypeOf(dst) != reflect.TypeOf(src) {
		t.Fatalf("expected %T, got %T", src, dst)
	}

	if err := dst.ConvertFrom(hub); err != nil {
		t.Fatalf("ConvertFrom failed: %v", err)
	}
}
