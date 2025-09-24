---
title: image-url-command
metatitle: Cluster API Provider Hetzner Custom Command to Install Node Image via imageURL
sidebar: image-url-command
description: Documentation on the CAPH image-url-command
---

The `--hcloud-image-url-command` for the caph controller can be used to execute a custom command to
install the node image.

This provides you a flexible way to create nodes.

The script/binary will be copied into the Hetzner Rescue System and executed.

You need to enable two things:

* The caph binary must get argument. Example:
  `--hcloud-image-url-command=/shared/image-url-command.sh`
* The hcloudmachine resource must have spec.imageURL set (usualy via a hcloudmachinetemplate)

The command will get the imageURL, bootstrap-data and machine-name of the corresponding
hcloudmachine as argument.

It is up to the command to download from that URL and provision the disk accordingly. This command
must be accessible by the controller pod. You can use an initContainer to copy the command to a
shared emptyDir.

The env var OCI_REGISTRY_AUTH_TOKEN from the caph process will be set for the command, too.

The command must end with the last line containing IMAGE_URL_DONE. Otherwise the execution is
considered to have failed.

We measured these durations:

| oldState | newState | avg(s) | min(s) | max(s) |
|----------|----------|-------:|-------:|-------:|
|  | Initializing | 3.30 | 2.00 | 5.00 |
| Initializing | EnablingRescue | 19.20 | 11.00 | 21.00 |
| EnablingRescue | BootingToRescue | 14.20 | 9.00 | 23.00 |
| BootingToRescue | RunningImageCommand | 38.20 | 37.00 | 42.00 |
| RunningImageCommand | BootToRealOS | 62.40 | 56.00 | 80.00 |
| BootToRealOS | OperatingSystemRunning | 1.80 | 1.00 | 3.00 |

<!--
  the table was created by:

  k logs deployments/caph-controller-manager | python3 hack/hcloud-image-url-command-states-markdown-from-logs.py
-->

The duration of the state `RunningImageCommand` depends heavily on your script.
