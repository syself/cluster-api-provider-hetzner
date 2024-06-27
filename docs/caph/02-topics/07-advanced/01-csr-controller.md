---
title: CSR controller
---

For the secure operation of Kubernetes, it is necessary to sign the kubelet serving certificates. By default, these are self-signed by kubeadm. By using the kubelet flag `rotate-server-certificates: "true"`, which can be found in initConfiguration/joinConfiguration.nodeRegistration.kubeletExtraArgs, the kubelet will do a certificate signing request (CSR) to the certificates API of Kubernetes.

These CSRs are not approved by default for security reasons. As described in the docs, this should be done manually by the cloud provider or with a custom approval controller. Since the provider integration is the responsible cloud provider in a way, it makes sense to implement such a controller directly here. The CSR controller that we implemented checks the DNS name and the IP address and thus ensures that only those nodes receive the signed certificate that are supposed to.

For error-free operation, the following kubelet flags should not be set:

```shell
tls-cert-file: "/var/lib/kubelet/pki/kubelet-client-current.pem"
tls-private-key-file: "/var/lib/kubelet/pki/kubelet-client-current.pem"
```

For more information, see:

- [https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-certs/](https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-certs/)
- [https://kubernetes.io/docs/reference/access-authn-authz/kubelet-tls-bootstrapping/#client-and-serving-certificates](https://kubernetes.io/docs/reference/access-authn-authz/kubelet-tls-bootstrapping/#client-and-serving-certificates)
