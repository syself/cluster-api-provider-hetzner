---
title: image-url-command
metatitle: Cluster API Provider Hetzner Custom Command to Install Node Image via imageURL
sidebar: image-url-command
description: Documentation on the CAPH image-url-command
---

The hcloud `spec.imageURLCommand` field and the `--baremetal-image-url-command` controller argument
can be used to execute a custom command to install the node image.

This provides you a flexible way to create nodes.

The script/binary will be copied into the rescue system and executed.

You need to enable two things:

* for hcloud: The HCloudMachine resource must set both `spec.imageURL` and
  `spec.imageURLCommand` (usually via a HCloudMachineTemplate)
* for baremetal: The hetznerbaremetal resource must use `useCustomImageURLCommand: true`.
  The CAPH controller must still get `--baremetal-image-url-command=/shared/image-url-command.sh`.

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

The command will get the imageURL, bootstrap-data, machine-name of the corresponding
machine and the root devices (seperated by spaces) as argument.

Example:

```bash
/root/image-url-command oci://example.com/yourimage:v1 /root/bootstrap.data my-md-bm-kh57r-5z2v8-zdfc9 'sda sdb'
```

It is up to the command to download from that URL and provision the disk accordingly. This command
must be accessible by the controller pod below `/shared`. You can use an initContainer to copy the
command to a shared emptyDir. For hcloud, `spec.imageURLCommand` is only the basename and must
start with `image-url-command-`.

The env var OCI_REGISTRY_AUTH_TOKEN from the caph process will be set for the command, too.

The command must end with the last line containing IMAGE_URL_DONE. Otherwise the execution is
considered to have failed.

The controller uses url.ParseRequestURI (Go function) to validate the imageURL.

A Kubernetes event will be created in both (success, failure) cases containing the output (stdout
and stderr) of the script. If the script takes longer than 7 minutes, the controller cancels the
provisioning.

We measured these durations for hcloud:

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
