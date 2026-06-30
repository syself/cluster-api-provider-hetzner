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

The command must be accessible by the controller pod below `/shared`. You can use an initContainer
to copy the command to a shared emptyDir. For both hcloud and bare metal, the command field is only
the basename of a command below `/shared`.

The env var OCI_REGISTRY_AUTH_TOKEN from the caph process will be set for the command, too.

By default, CAPH passes short device names (e.g. `sda`) as the last argument to the command.
For bare metal machines you can set `spec.installImage.deviceStringType` to control this:

* `"short"` (or empty): passes the short device name, e.g. `sda`
* `"wwn"`: passes the WWN from the `rootDeviceHints`, e.g. `eui.00253885910c8cec`

Example:

```yaml
spec:
  installImage:
    imageURLCommand: image-url-command-install-foo.sh
    deviceStringType: wwn
    image:
      url: oci://example.com/yourimage:v1
```

Using `deviceStringType: wwn` avoids fragile device-name lookups, because device names like `sda`
can change across reboots while WWNs are stable identifiers. The `deviceStringType` field is not
used for hcloud machines (hcloud VMs always boot from `sda` and disks have no WWN).

When multiple devices are configured (e.g. RAID via `rootDeviceHints.raid.wwn`), all device
strings are passed as a single space-separated `$4` argument. Scripts should split on whitespace.

The command must end with the last line on stdout containing `IMAGE_URL_DONE`. Otherwise the
execution is considered to have failed.

  Implementation detail: CAPH executes the command in the rescue system via `ssh` and `nohup`. Stdout
  and stderr are redirected to a file. CAPH continuously connects to the rescue system to see if the
  process is still running.

The controller uses url.ParseRequestURI (Go function) to validate the imageURL.

A Kubernetes event will be created in both (success, failure) cases containing the output (stdout
and stderr) of the script. If the script takes longer than 7 minutes, the controller cancels the
provisioning.

## output.json (optional)

The command may write `/root/output.json` at any point during execution. If the file does not
exist, provisioning still succeeds based on `IMAGE_URL_DONE` alone.

CAPH reads the `status` field from this file to update the provisioning condition on the machine
(HCloudMachine or HetznerBareMetalHost). The `message` field is forwarded verbatim into the
condition message.

## Outcome summary

CAPH waits until the provisioning process in the rescue system has terminated. Then the captured
stdout gets examined. If it does not contain `IMAGE_URL_DONE`, then the process has failed. Optionally
`output.json` can be created by the process.

| **`IMAGE_URL_DONE` in stdout** | **`output.json` exists** | **`status` in `output.json`** | **Result** |
| :------------------------: | :------------------: | :-----------------------: | ------ |
| yes | no | — | **success** |
| yes | yes | `"Succeeded"` | **success** |
| yes | yes | `"Failed"` | **failure**, provisioning cancelled |
| yes | yes | any other string | **failure**, provisioning cancelled |
| no | any | any | **failure**, provisioning cancelled |

Implemented in `handleBootStateRunningImageCommand` (hcloud) and
`actionImageInstallingImageURLCommand` (baremetal).

### Fields CAPH reads

CAPH only reads two top-level fields:

| Field     | Required               | Values                                    | Purpose                                    |
|-----------|------------------------|-------------------------------------------|--------------------------------------------|
| `status`  | yes (to set condition) | `"Succeeded"`, `"Failed"`, `"InProgress"` | Updates the provisioning condition         |
| `message` | no                     | free-form string                          | Included in the condition message          |

While the command is **running**, write `"InProgress"` to update the provisioning condition
(indicating provisioning has not succeeded yet). Any other unrecognised string is also
treated as in-progress.
Once `IMAGE_URL_DONE` appears in stdout (command finished), only `"Succeeded"` allows
provisioning to proceed; any other value cancels it.

Minimal success example:

```json
{"status": "Succeeded"}
```

Minimal failure example:

```json
{"status": "Failed", "message": "failed to pull image: disk full"}
```

Any other fields in the JSON are **ignored by CAPH** but are forwarded as-is via the Kubernetes
event (see below). You can use them for your own structured debugging output.

### Kubernetes event on completion

When the command finishes (success or failure), CAPH emits a Kubernetes event with reason
`ImageURLCommandOutputJSON` containing the **full JSON content** of the file. If the command
failed, the event type is `Warning`; otherwise it is `Normal`. The content is also written to
the controller log at key `outputJSON`.

### Extended example

Your command can include arbitrary extra fields for its own structured debug output. CAPH
passes the whole JSON through untouched:

```json
{
  "status": "Succeeded",
  "phases": {
    "Preparation": {
      "status": "Succeeded",
      "duration": "45.2s",
      "steps": [
        {"name": "VerifyTools",       "status": "Succeeded", "duration": "0.3s", "percentOfTimeout": 1},
        {"name": "CheckDeviceExists", "status": "Succeeded", "duration": "0.1s", "percentOfTimeout": 0}
      ]
    },
    "ImageDeployment": {
      "status": "Succeeded",
      "duration": "62.1s",
      "steps": [
        {"name": "PullImage",  "status": "Succeeded", "duration": "58.4s", "percentOfTimeout": 14},
        {"name": "WriteImage", "status": "Succeeded", "duration": "3.5s",  "percentOfTimeout": 1}
      ]
    }
  }
}
```

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
