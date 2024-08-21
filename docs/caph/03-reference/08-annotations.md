---

title: Annotations

---

You can set [Kubernetes Annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/) to instruct the Syself CAPH Controller to modify its behavior.

## Overview of Annotations

### capi.syself.com/wipe-disk

| **Resource**    | [HetznerBareMetalHost](/docs/caph/03-reference/05-hetzner-bare-metal-host.md)                                                                                                                                                                                                                                       |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Description** | This annotation instructs the Syself CAPH Controller to wipe the disk before provisioning the machine.                                                                                                                                                                                                              |
| **Value**       | You can use the string `"all"` or a space-separated list of WWNs. For example, `"10:00:00:05:1e:7a:7a:00 eui.00253885910c8cec 0x500a07511bb48b25"`. If the value is empty, no disks will be wiped. The value `"all"` will wipe all disks on the bare-metal machine (not just the one given in the rootDeviceHints). |
| **Auto-Remove** | Enabled: The annotation is removed after the disks are wiped.                                                                                                                                                                                                                                                       |

### capi.syself.com/ignore-check-disk

| **Resource**    | [HetznerBareMetalHost](/docs/caph/03-reference/05-hetzner-bare-metal-host.md)                                                                                                                                                                                                                         |
| --------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Description** | This annotation instructs the Syself CAPH Controller to ignore the results of the check-disk step during machine provisioning. The check will be performed, and a Kubernetes Event will be created. However, if this annotation is set, provisioning will continue even if a faulty disk is detected. |
| **Value**       | The value is ignored. If the annotation exists, this feature is enabled.                                                                                                                                                                                                                              |
| **Auto-Remove** | Disabled: The annotation remains on the resource. It is up to the user to remove it.                                                                                                                                                                                                                  |

### capi.syself.com/allow-empty-control-plane-address

| **Resource**    | [HetznerCluster](/docs/caph/03-reference/02-hetzner-cluster.md)                                                            |
| --------------- | -------------------------------------------------------------------------------------------------------------------------- |
| **Description** | This annotation allows the Syself CAPH Controller to create HetznerCluster resources with an empty `controlPlaneEndpoint`. |
| **Value**       | `"true"` enables this feature. All other strings are considered `"false"`.                                                 |
| **Auto-Remove** | Disabled: The annotation remains on the resource.                                                                          |

### capi.syself.com/constant-bare-metal-hostname

| **Resource**    | Cluster (CRD of Cluster-API), HetznerBareMetalMachine                                                        |
| --------------- | ------------------------------------------------------------------------------------------------------------ |
| **Description** | See [Using constant hostnames](/docs/caph/02-topics/05-baremetal/04-constant-hostnames.md) for more details. |
| **Auto-Remove** | Disabled: The annotation remains on the resource.                                                            |

### capi.syself.com/reboot

| **Resource**    | [HetznerBareMetalHost](/docs/caph/03-reference/05-hetzner-bare-metal-host.md)                                                                                                                                                                                 |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Description** | If this annotation is present, the bare-metal machine will be rebooted. This annotation is used by `HetznerBareMetalRemediation` (see [Machine Health Checks with Custom Remediation Template](/docs/caph/02-topics/06-advanced/04-custom-templates-mhc.md)). |
| **Value**       | The value is ignored. If the annotation exists, this feature is enabled.                                                                                                                                                                                      |
| **Auto-Remove** | Enabled: The annotation is removed after the reboot.                                                                                                                                                                                                          |

### capi.syself.com/permanent-error

| **Resource**    | [HetznerBareMetalHost](/docs/caph/03-reference/05-hetzner-bare-metal-host.md)                                                                                                                                                                                                                                                                |
| --------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Description** | This annotation is set by the Syself CAPH Controller when a bare-metal machine enters the "permanent error" state. This indicates that human intervention is required (e.g., to fix a broken disk). After the root cause is resolved, the user must remove this annotation to allow the Controller to manage the HetznerBareMetalHost again. |
| **Auto-Remove** | Disabled: The annotation must be removed by the user.                                                                                                                                                                                                                                                                                        |
