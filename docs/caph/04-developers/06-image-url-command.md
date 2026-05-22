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

When `imageURLCommandAPIVersion: v2` is set, CAPH also passes `--api-version=v2` as a flag
(appended after the positional arguments).

Example (v1):

```bash
/root/image-url-command oci://example.com/yourimage:v1 /root/bootstrap.data my-md-bm-kh57r-5z2v8-zdfc9 'sda sdb'
```

Example (v2):

```bash
/root/image-url-command oci://example.com/yourimage:v1 /root/bootstrap.data my-md-bm-kh57r-5z2v8-zdfc9 'sda sdb' --api-version=v2
```

It is up to the command to download from that URL and provision the disk accordingly. The command
must be accessible by the controller pod below `/shared`. You can use an initContainer to copy the
command to a shared emptyDir.
For both hcloud and bare metal, the command field is only the basename of a command below `/shared`
and must start with `image-url-command-`.

The env var OCI_REGISTRY_AUTH_TOKEN from the caph process will be set for the command, too.

The controller uses url.ParseRequestURI (Go function) to validate the imageURL.

A Kubernetes event will be created in both (success, failure) cases containing the output (stdout
and stderr) of the script. If the script takes longer than 7 minutes, the controller cancels the
provisioning.

## API versions

There are two output protocol versions, selected by `spec.imageURLCommandAPIVersion` on
HCloudMachine (hcloud only). Once set, this field is immutable.

### v1 (deprecated)

Leave `imageURLCommandAPIVersion` empty (the default) to use v1.

The command must end with the last line containing `IMAGE_URL_DONE`. Otherwise the execution is
considered to have failed.

### v2

Set `imageURLCommandAPIVersion: v2` on the HCloudMachineTemplate to opt in:

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
      imageURLCommandAPIVersion: v2
```

With v2, the command signals completion by writing `/root/output.json` before it exits.
If the process exits without writing that file, CAPH fails the machine immediately.

#### output.json format

```json
{
  "apiVersion": "v2",
  "status": "Succeeded",
  "phases": {
    "Preparation": {
      "status": "Succeeded",
      "steps": [
        {"name": "partition-disks",   "status": "Succeeded", "message": ""},
        {"name": "mount-filesystems", "status": "Succeeded", "message": ""}
      ]
    },
    "ImageDeployment": {
      "status": "Succeeded",
      "steps": [
        {"name": "pull-oci-image", "status": "Succeeded", "message": ""},
        {"name": "write-to-disk",  "status": "Succeeded", "message": ""}
      ]
    },
    "BootstrapDelivery": {
      "status": "Succeeded",
      "steps": [
        {"name": "write-cloud-init", "status": "Succeeded", "message": ""}
      ]
    },
    "Handover": {
      "status": "Succeeded",
      "steps": [
        {"name": "reboot", "status": "Succeeded", "message": ""}
      ]
    }
  }
}
```

The top-level `status` must be `"Succeeded"`, `"Failed"`, or `"Running"`. A missing or empty
`status` means the file is not yet complete (binary still running or was interrupted before writing
the file).

On failure, set `status: "Failed"` and mark the failed phase and step accordingly:

```json
{
  "apiVersion": "v2",
  "status": "Failed",
  "phases": {
    "Preparation": {"status": "Succeeded", "steps": [...]},
    "ImageDeployment": {
      "status": "Failed",
      "failedStep": "pull-oci-image",
      "steps": [
        {"name": "pull-oci-image", "status": "Failed", "message": "registry returned 403"}
      ]
    }
  }
}
```

#### Conditions set by v2

CAPH maps each phase to a condition on the HCloudMachine:

| Condition | Phase |
|-----------|-------|
| `PreparationSucceeded` | Preparation — disks partitioned, filesystems mounted, required binaries verified |
| `ImageDeploymentSucceeded` | ImageDeployment — OCI tarball pulled, checksum/signature verified, image written to disk |
| `BootstrapDeliverySucceeded` | BootstrapDelivery — cloud-init/CAPI bootstrap data written into the deployed image |
| `HandoverSucceeded` | Handover — reboot initiated; binary completed, controller takes over |
| `NodeProvisioningSucceeded` | Aggregate of all four phases above |

Each condition uses the reason `Succeeded`, `Failed`, or `NotStarted`.

## Measured durations for hcloud

| oldState | newState | avg(s) | min(s) | max(s) |
|----------|----------|-------:|-------:|-------:|
|  | Initializing | 3.30 | 2.00 | 5.00 |
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
