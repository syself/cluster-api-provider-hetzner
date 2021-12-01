// Package csr contains functions to validate certificate signing requests
package csr

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"fmt"
	"reflect"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
)

// NodesPrefix defines the prefix name for a node.
const NodesPrefix = "system:node:"

// NodesGroup defines the group name for a node.
const NodesGroup = "system:nodes"

// ValidateKubeletCSR validates a CSR.
func ValidateKubeletCSR(csr *x509.CertificateRequest, machine *infrav1.HCloudMachine) error {
	// check signature and exist quickly
	if err := csr.CheckSignature(); err != nil {
		return err
	}

	var errs []error

	// validate subject
	username := fmt.Sprintf("%s%s", NodesPrefix, machine.ObjectMeta.Name)
	subjectExpected := pkix.Name{
		CommonName:   username,
		Organization: []string{NodesGroup},
		Names: []pkix.AttributeTypeAndValue{
			{Type: asn1.ObjectIdentifier{2, 5, 4, 10}, Value: NodesGroup},
			{Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: username},
		},
	}
	if !reflect.DeepEqual(subjectExpected, csr.Subject) {
		errs = append(errs, fmt.Errorf("unexpected subject actual=%+#v, expected=%+#v", csr.Subject, subjectExpected))
	}

	// check for DNS Names
	if len(csr.EmailAddresses) > 0 {
		errs = append(errs, fmt.Errorf("email addresses are not allow on the request: %v", csr.EmailAddresses))
	}

	// allow only certain DNS names
	allowedDNSNames := map[string]struct{}{
		machine.ObjectMeta.Name: {},
	}
	for _, name := range csr.DNSNames {
		if _, ok := allowedDNSNames[name]; !ok {
			errs = append(errs, fmt.Errorf("the DNS name '%s' is not allowed", name))
		}
	}

	// allow only certain IP addresses
	allowedIPAddresses := make(map[string]struct{})
	for _, address := range machine.Status.Addresses {
		switch address.Type {
		case corev1.NodeInternalIP, corev1.NodeExternalIP:
			allowedIPAddresses[address.Address] = struct{}{}
		}
	}
	for _, ip := range csr.IPAddresses {
		if _, ok := allowedIPAddresses[ip.String()]; !ok {
			errs = append(errs, fmt.Errorf("the IPAddress '%s' is not allowed", ip.String()))
		}
	}

	// allow only certain IP addresses

	return kerrors.NewAggregate(errs)
}
