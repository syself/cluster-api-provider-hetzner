---
title: Machine Health Checks with Custom Remediation Template
description: Learn how to configure Machine Health Checks and Bare Metal Server Remediation Templates for optimal server performance in your Kubernetes cluster.
---

Cluster API allows to [configure Machine Health Checks](https://cluster-api.sigs.k8s.io/tasks/automated-machine-management/healthchecking.html) with custom remediation strategies. This is helpful for our bare metal servers. If the health checks give an outcome that one server cannot be reached, the default strategy would be to delete it. In that case, it would need to be provisioned again. This takes, of course, longer for bare metal servers than for virtual cloud servers. Therefore, we want to try to avoid this with the help of our `HetznerBareMetalRemediationController` and `HCloudRemediationController`. Instead of deleting the object and deprovisioning it, we first try to reboot it and see whether this helps. If it solves the problem, we save a lot of time that is required for re-provisioning it.

If the MHC is configured to be used with the `HetznerBareMetalRemediationTemplate` (also see the [reference of the object](/docs/caph/03-reference/07-hetzner-bare-metal-remediation-template.md)) and `HCloudRemediationTemplate` (also see the [reference of the object](/docs/caph/03-reference/04-hcloud-remediation-template.md)), then such an object is created every time the MHC finds an unhealthy machine.

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
