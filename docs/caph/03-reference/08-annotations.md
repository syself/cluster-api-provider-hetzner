---
title: Annotations
---

You can set [Kubernetes Annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/) to make the Syself CAPH Controller modify its behavior.

## Overview of Annotations

### capi.syself.com/wipedisk

Resource: [HetznerBareMetalHost](/docs/caph/03-reference/05-hetzner-bare-metal-host.md)

Description: You can instruct the Syself CAPH Controller to wipe the disk before provisioning the machine.

Value: You can use the string "all" or a space speperated list of WWNS. For example "10:00:00:05:1e:7a:7a:00 eui.00253885910c8cec 0x500a07511bb48b25". If the value is empty, not disks will be wiped.

Auto-Remove: Enabled: The annotation gets removed after the disks got wiped.

### capi.syself.com/ignore-check-disk

Resource:[HetznerBareMetalHost](/docs/caph/03-reference/05-hetzner-bare-metal-host.md)

Description: You can instruct the Syself CAPH Controller to ignore the result of the check-disk step during provisioning the machine. The check will be done and an Event will be created. But if the annotation is set (the value does not matter), the provisioning will be done even if the check detected a faulty disk.

Value: The value gets ignored. If the annotation exists, this feature will be enabled.

Auto-Remove: Disabled: The annotation stays on the resource. It is up to the user to remove it.

### capi.syself.com/allow-empty-control-plane-address

Resource: [HetznerCluster](docs/caph/03-reference/02-hetzner-cluster.md)

Description: This annotation makes the Syself CAPH Controller allow the creation of HetznerCluster resources with "controlPlaneEndpoint" being empty.
TODO: use-case?

Value: "true" all other strings are considered "false".

Auto-Remove: Disabled: The annotation stays on the resource.

### capi.syself.com/constant-bare-metal-hostname

Resource: Cluster (CRD of Cluster-API), HetznerBareMetalMachine

Description: [Using constant hostnames](/docs/caph/02-topics/05-baremetal/04-constant-hostnames.md)

Auto-Remove: Disabled: The annotation stays on the resource.

### capi.syself.com/reboot

Resource: [HetznerBareMetalHost](/docs/caph/03-reference/05-hetzner-bare-metal-host.md)

Description: If the annotation is present, the bare-metal machine will be rebooted. This annotation gets used by the `HetznerBareMetalRemediation` (see [Machine Health Checks with Custom Remediation Template](docs/caph/02-topics/06-advanced/04-custom-templates-mhc.md))

Value: The value gets ignored. If the annotation exists, this feature will be enabled.

Auto-Remove: Enabled: The annotation gets removed the reboot.

### capi.syself.com/permanent-error

Resource: [HetznerBareMetalHost](/docs/caph/03-reference/05-hetzner-bare-metal-host.md)

Description: The annoation gets set by the Syself CAPH Controller, when a bare-metal machine gets set into the "permanent error" state. This means a human need to handle the error (for example a broken disk). After the root cause was fixed by a user, you remove this annotation to make the Controller pick the HetznerBareMetalHost again.

Auto-Remove: Disabled: Needs to be removed by a user.
