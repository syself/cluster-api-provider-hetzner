// Package v1alpha1 implements the v1alpha1 apiVersion of ctlptl's cluster
// configuration
//
// Borrows the approach of clientcmd/api and KIND, maintaining an API similar to
// other Kubernetes APIs without pulling in the API machinery.
//
// +k8s:deepcopy-gen=package
package api
