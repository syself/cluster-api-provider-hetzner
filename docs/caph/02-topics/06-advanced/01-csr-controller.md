---
title: CSR controller
description: Explore advanced Certificate Signing Request (CSR) controller topics for efficient management insights.
---

For the secure operation of Kubernetes, it is necessary to sign the kubelet serving certificates. By default, these are self-signed by kubeadm. By using the kubelet flag `rotate-server-certificates: "true"`, which can be found in initConfiguration/joinConfiguration.nodeRegistration.kubeletExtraArgs, the kubelet will do a certificate signing request (CSR) to the certificates API of Kubernetes.

These CSRs are not approved by default for security reasons. As described in the docs, this should be done manually by the cloud provider or with a custom approval controller.

## Default CSR controller

Since the provider integration is the responsible cloud provider in a way, it makes sense to implement such a controller directly here. The CSR controller that we implemented checks the DNS name and the IP address and thus ensures that only those nodes receive the signed certificate that are supposed to.

For error-free operation, the following kubelet flags should not be set:

```shell
tls-cert-file: "/var/lib/kubelet/pki/kubelet-client-current.pem"
tls-private-key-file: "/var/lib/kubelet/pki/kubelet-client-current.pem"
```

For more information, see:

- [https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-certs/](https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-certs/)
- [https://kubernetes.io/docs/reference/access-authn-authz/kubelet-tls-bootstrapping/#client-and-serving-certificates](https://kubernetes.io/docs/reference/access-authn-authz/kubelet-tls-bootstrapping/#client-and-serving-certificates)

## Custom CSR controller

It is possible to disable the CSR controller using the flag `--disable-csr-approval`. However, this flag disables this feature globally, for all `HetznerCluster` objects in the management cluster. There is currently no way to toggle this on or off for just a single cluster.

This is useful for cases where the validation or approval logic of the default CSR controller is insufficient for your use cases.

If you disable the default CSR controller, you'll need to deploy an equivalent controller that can validate and approve CSRs securely. This is a security critical process and needs to be handled with care.

For more information, see:

- [https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/](https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/)
- [https://kubernetes.io/docs/reference/access-authn-authz/kubelet-tls-bootstrapping/#certificate-rotation](https://kubernetes.io/docs/reference/access-authn-authz/kubelet-tls-bootstrapping/#certificate-rotation)