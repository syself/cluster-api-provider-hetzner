---
title: pre-provision-command
metatitle: Cluster API Provider Hetzner Check Bare Metal Server before Provisioning
sidebar: pre-provision-command
description: Documentation on the CAPH pre-provision-command.
---

The `--pre-provision-command` for the caph controller can be used to execute a custom command
before install-image starts.

This provides you a flexible way to check if the bare metal server is healthy.

Example:

```sh
--pre-provision-command=/shared/my-pre-provision-command.sh
```

The script/binary will be copied into the Hetzner Rescue System and executed.

If the exit code is zero, then all is fine.

If the exit code is non-zero, then provisioning of that machine will be stopped.

The CAPH controller runs in a Kubernetes Pod. The container of that pod needs access to the file.

There are several ways to make this command available:

* You could mount a configMap/secret.
* You create an container image, and use that as init-container.
* You build a custom image of CAPH. We do not recommend that.

In this example we use an init-container to provide the script.

In the directory `images/pre-provision-command/` you see these files:

* my-pre-provision-command.sh: A simple Bash script which creates a message and exists with 0.
* Dockerfile: Needed to create an container image.
* build-and-push.sh: A script to build and upload the script to a container registry.

When the container images was uploaded, you need to adapt the CAPH deployment:

```yaml

      # Init container, which makes the command available to caph.
      initContainers:
      - command:
        - /bin/sh
        - -c
        - cp /my-pre-provision-command.sh /shared/
        image: ghcr.io/syself/caph-staging:pre-provision-command
        imagePullPolicy: Always
        name: init-container
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /shared
          name: shared


      # Add this to the args of the caph container:
        args:
        - --pre-provision-command=/shared/my-pre-provision-command.sh

        # Add this to the caph container
        volumeMounts:
        - mountPath: /shared
          name: shared

      # Add this after "container"
      volumes:
      - emptyDir: {}
        name: shared
```
