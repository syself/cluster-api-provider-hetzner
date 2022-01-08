# CSR Controller

For the secure operation of Kubernetes it is necessary to sign the kubelet serving certificates. By default these are self-signed by kubeadm. By using the kubelet flag under initConfiguration/joinConfiguration.nodeRegistration.kubeletExtraArgs `rotate-server-certificates: "true"` The kubelet will do a certificate signing request (CSR) to the certificates api of kubernetes. 


These CSRs are not approved by default for security reasons, as described in the documents, this should be done manually by the cloud provider or with a custom approval controller. Since the provider integration is in a way the responsible cloud provider, it makes sense to implement such a controller directly here, which checks the DNS name and the IP address and thus also ensures that only the correct nodes receive a signed certificate.

For error-free operation the following kubelet flags should not be set 
```
tls-cert-file: "/var/lib/kubelet/pki/kubelet-client-current.pem"
tls-private-key-file: "/var/lib/kubelet/pki/kubelet-client-current.pem" 
```

For more inforamtion see: 
* https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-certs/
* https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet-tls-bootstrapping/#client-and-serving-certificates
