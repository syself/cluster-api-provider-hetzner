---
title: image-url-command
metatitle: Cluster API Provider Hetzner Custom Command to Install Node Image via imageURL
sidebar: image-url-command
description: Documentation on the CAPH image-url-command
---

The hcloud `spec.imageURLCommand` field and the bare metal
`spec.installImage.imageURLCommand` field can be used to execute a custom command to
install the node image.

This provides you a flexible way to create nodes.

The script/binary will be copied into the rescue system and executed.

You need to enable two things:

* for hcloud: The HCloudMachine resource must set both `spec.imageURL` and
  `spec.imageURLCommand` (usually via a HCloudMachineTemplate)
* for baremetal: The HetznerBareMetalMachine must set
  `spec.installImage.imageURLCommand`, for example:

```yaml
spec:
  installImage:
    imageURLCommand: image-url-command-install-foo.sh
    image:
      url: oci://example.com/yourimage:v1
```

In bare metal custom-command mode, `image.name` and `image.path` must stay empty.

Example for hcloud:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: HCloudMachineTemplate
metadata:
  name: my-hcloud-template
spec:
  template:
    spec:
      type: cpx22
      imageURL: oci://example.com/yourimage:v1
      imageURLCommand: image-url-command-install-foo.sh
```

The command receives the following positional arguments:

1. `imageURL` — the OCI (or other) image URL
2. `/root/bootstrap.data` — path to the bootstrap data file written by CAPH
3. `machine-name` — name of the corresponding machine
4. `root-devices` — space-separated list of root device names (e.g. `sda sdb`)

Example:

```bash
/root/image-url-command oci://example.com/yourimage:v1 /root/bootstrap.data my-md-bm-kh57r-5z2v8-zdfc9 'sda sdb'
```

The image format — whole-disk image, root-filesystem tarball, or anything else — is entirely
your choice, as long as the `imageURLCommand` binary and the artifact at `imageURL` match each other.
Both are user-configurable; you are responsible for keeping them in sync.

The command must be accessible by the controller pod below `/shared`. You can use an initContainer to copy the
command to a shared emptyDir.
For both hcloud and bare metal, the command field is only the basename of a command below `/shared`
and must start with `image-url-command-`.

The env var OCI_REGISTRY_AUTH_TOKEN from the caph process will be set for the command, too.

The command must end with the last line containing `IMAGE_URL_DONE`. Otherwise the execution is
considered to have failed.

The controller uses url.ParseRequestURI (Go function) to validate the imageURL.

A Kubernetes event will be created in both (success, failure) cases containing the output (stdout
and stderr) of the script. If the script takes longer than 7 minutes, the controller cancels the
provisioning.

## output.json (optional)

The command may write `/root/output.json` at any point during execution. CAPH reads it
continuously to monitor provisioning progress. If the file does not exist, provisioning
still succeeds based on `IMAGE_URL_DONE` alone.

```json
{
  "status": "Succeeded",
  "phases": {
    "Preparation": {
      "status": "Succeeded",
      "steps": [
        {"name": "VerifyTools",        "status": "Succeeded", "message": ""},
        {"name": "CheckDeviceExists",  "status": "Succeeded", "message": ""}
      ]
    },
    "ImageDeployment": {
      "status": "Succeeded",
      "steps": [
        {"name": "PullImage",  "status": "Succeeded", "message": ""},
        {"name": "WriteImage", "status": "Succeeded", "message": ""}
      ]
    },
    "BootstrapDelivery": {
      "status": "Succeeded",
      "steps": [
        {"name": "ConfigureCloudInit", "status": "Succeeded", "message": ""}
      ]
    },
    "Handover": {
      "status": "Succeeded",
      "steps": [
        {"name": "UnmountDisk", "status": "Succeeded", "message": ""}
      ]
    }
  }
}
```

When the command finishes (success or failure), CAPH emits a Kubernetes event with reason `ImageURLCommandOutputJSON` containing the full JSON content. If the command failed, the event type is `Warning`; otherwise it is `Normal`. The content is also written to the controller log at key `outputJSON`.

When present, CAPH sets the `NodeProvisioningSucceeded` condition on the machine (HCloudMachine or HetznerBareMetalHost) based on the top-level `status` field.

## Measured durations for hcloud

| oldState | newState | avg(s) | min(s) | max(s) |
| -------- | -------- | -----: | -----: | -----: |
| | Initializing | 3.30 | 2.00 | 5.00 |
| Initializing | EnablingRescue | 19.20 | 11.00 | 21.00 |
| EnablingRescue | BootingToRescue | 14.20 | 9.00 | 23.00 |
| BootingToRescue | RunningImageCommand | 38.20 | 37.00 | 42.00 |
| RunningImageCommand | BootingToRealOS | 62.40 | 56.00 | 80.00 |
| BootingToRealOS | OperatingSystemRunning | 1.80 | 1.00 | 3.00 |

<!--
  the table was created by:

  k logs deployments/caph-controller-manager | python3 hack/hcloud-image-url-command-states-markdown-from-logs.py
-->

The duration of the state `RunningImageCommand` depends heavily on your script.
