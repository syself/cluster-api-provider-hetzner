# Advanced CAPH

## CSR Controller

For the secure operation of Kubernetes, it is necessary to sign the kubelet serving certificates. By default, these are self-signed by kubeadm. By using the kubelet flag `rotate-server-certificates: "true"`, which can be found in initConfiguration/joinConfiguration.nodeRegistration.kubeletExtraArgs, the kubelet will do a certificate signing request (CSR) to the certificates API of Kubernetes. 

These CSRs are not approved by default for security reasons. As described in the docs, this should be done manually by the cloud provider or with a custom approval controller. Since the provider integration is the responsible cloud provider in a way, it makes sense to implement such a controller directly here. The CSR controller that we implemented checks the DNS name and the IP address and thus ensures that only those nodes receive the signed certificate that are supposed to.

For error-free operation, the following kubelet flags should not be set: 
```
tls-cert-file: "/var/lib/kubelet/pki/kubelet-client-current.pem"
tls-private-key-file: "/var/lib/kubelet/pki/kubelet-client-current.pem" 
```

For more information, see: 
* https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-certs/
* https://kubernetes.io/docs/reference/access-authn-authz/kubelet-tls-bootstrapping/#client-and-serving-certificates

## Rate Limits

Hetzner Cloud and Hetzner Robot both implement rate limits. As a brute-force method, we implemented some logic that prevents the controller from reconciling a specific object for some defined time period if a rate limit was hit during reconcilement of that object. We set the condition on true, that a rate limit was hit. Of course, this only affects one object so that another `HCloudMachine` still reconciles normally, even though one hits the rate limit. There is a chance that it will also hit the rate limit (which is defined per function so that it does not necessarily need to happen). In that case, the controller also stops reconciling this object for some time.

## Multi-tenancy

We support multi-tenancy. You can start multiple clusters in one Hetzner project at the same time. As the resources all have a label with the cluster name, the controller is able to handle them perfectly.

## Machine Health Checks with Custom Remediation Template

Cluster API allows to [configure Machine Health Checks](https://cluster-api.sigs.k8s.io/tasks/automated-machine-management/healthchecking.html) with custom remediation strategies. This is helpful for our bare metal servers. If the health checks give an outcome that one server cannot be reached, the default strategy would be to delete it. In that case, it would need to be provisioned again. This takes, of course, longer for bare metal servers than for virtual cloud servers. Therefore, we want to try to avoid this with the help of our `HetznerBareMetalRemediationController` and `HCloudRemediationController`. Instead of deleting the object and deprovisioning it, we first try to reboot it and see whether this helps. If it solves the problem, we save a lot of time that is required for re-provisioning it.

If the MHC is configured to be used with the `HetznerBareMetalRemediationTemplate` (also see the [reference of the object](/docs/reference/hetzner-bare-metal-remediation-template.md)) and `HCloudRemediationTemplate` (also see the [reference of the object](/docs/reference/hcloud-remediation-template.md)), then such an object is created every time the MHC finds an unhealthy machine. 

The `HetznerBareMetalRemediationController` reconciles this object and then sets an annotation in the relevant `HetznerBareMetalHost` object specifying the desired remediation strategy. At the moment, only "reboot" is supported.
The `HCloudRemediationController` reboots the HCloudMachine directly via the HCloud API. For HCloud servers, there is no other strategy than "reboot" either.

Here is an example of how to configure the Machine Health Check and `HetznerBareMetalRemediationTemplate`:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineHealthCheck
metadata:
  name: "cluster123-control-plane-unhealthy-5m"
spec:
  clusterName: "cluster123"
  maxUnhealthy: 100%
  nodeStartupTimeout: 20m
  selector:
    matchLabels:
      cluster.x-k8s.io/control-plane: ""
  unhealthyConditions:
    - type: Ready
      status: Unknown
      timeout: 300s
    - type: Ready
      status: "False"
      timeout: 300s
  remediationTemplate: # added infrastructure reference
    kind: HetznerBareMetalRemediationTemplate
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    name: control-plane-remediation-request
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: HetznerBareMetalRemediationTemplate
metadata:
  name: control-plane-remediation-request
spec:
  template:
    spec:
      strategy:
        type: "Reboot"
        retryLimit: 2
        timeout: 300s

```
