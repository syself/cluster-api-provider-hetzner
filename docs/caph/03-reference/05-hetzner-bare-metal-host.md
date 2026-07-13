---
title: HetznerBareMetalHost
description: The HetznerBareMetalHost has a one-to-one relationship to a Hetzner dedicated server. It's used to make bare metal servers available to your clusters.
metatitle: HetznerBareMetalHost Object Reference
---

The `HetznerBareMetalHost` object has a one-to-one relationship to a Hetzner dedicated server. Its ID is specified in the specs. The host object does not belong to a certain `HetznerCluster`, but can be used by multiple clusters. This is useful, as one host object per server is enough and you can easily see whether a host is used by one of your clusters or not.

There are not many properties that are relevant to the host object. The WWN of the storage device that should be used for provisioning has to be specified in `rootDeviceHints` - but not right from the start. This property can be updated after the host starts the provisioning phase and writes all `hardwareDetails` in the host's status. From there, you can copy the WWN of the storage device that suits your needs and add it to your `HetznerBareMetalHost` object.

## Find the WWN

After you have started the provisioning, run the following on your management cluster to find the `hardwareDetails` of all of your bare metal hosts.

```shell
kubectl describe hetznerbaremetalhost
```

## Lifecycle of a HetznerBareMetalHost

A host object is available for consumption right after it has been created. When a `HetznerBareMetalMachine` chooses the host, it updates the host's status. This triggers the provisioning of the host. When the `HetznerBareMetalMachine` gets deleted, then the host deprovisions and returns to the state where it is available for new consumers.

`HetznerBareMetalHosts` can only be deleted when they are in the neutral state. In order to delete them, they should be first set to maintenance mode, so that no `HetznerBareMetalMachine` consumes it.

Host objects cannot be updated and have to be deleted and re-created if some of the properties change.

### Maintenance mode

Maintenance mode means that the host will not be consumed by any `HetznerBareMetalMachine`. If it is already consumed, then the corresponding `HetznerBareMetalMachine` will be deleted and the `HetznerBareMetalHost` deprovisioned.

## Overview of HetznerBareMetalHost.Spec

<PropField name="serverID" type="int" required={true}>
Server ID of the Hetzner dedicated server, you can find it on your Hetzner robot dashboard.
</PropField>

<PropField name="rootDeviceHints" type="object" required={false}>

It is important to find the correct root device. If none are specified, the host will stop provisioning in between to wait for the details to be specified. HardwareDetails in the host's status can be used to find the correct device. Currently, you can specify one disk or a raid setup.

<Collapsible title="properties">

<PropField name="rootDeviceHints.wwn" type="string" required={false}>
Unique storage identifier for non raid setups.
</PropField>

<PropField name="rootDeviceHints.raid" type="object" required={false}>

Used to provide the controller with information on which disks a raid can be established.

<Collapsible title="properties">

<PropField name="rootDeviceHints.raid.wwn" type="[]string" required={false}>
Defines a list of Unique storage identifiers used for raid setups.
</PropField>

</Collapsible>

</PropField>

</Collapsible>

</PropField>

<PropField name="consumerRef" type="object" required={false}>
Used by the controller and references the bare metal machine that consumes this host.
</PropField>

<PropField name="maintenanceMode" type="bool" required={false}>
If set to true, the host deprovisions and will not be consumed by any bare metal machine.
</PropField>

<PropField name="description" type="string" required={false}>
Description can be used to store some valuable information about this host.
</PropField>

<PropField name="status" type="object" required={false}>
The controller writes this status. As there are some that cannot be regenerated during any reconcilement, the status is in the specs of the object - not the actual status. DO NOT EDIT!!!
</PropField>

## Example of the HetznerBareMetalHost object

You should create one of these objects for each of your bare metal servers that you want to use for your deployment.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: HetznerBareMetalHost
metadata:
  name: "bm-0" #example
spec:
  serverID: 1682566 #change
  rootDeviceHints:
    wwn: "eui.0068475201b4egh2" #change
  maintenanceMode: false
  description: Test Machine 0 #example
```

If you want to create an object that will be used in a raid setup, the following can serve as an example.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: HetznerBareMetalHost
metadata:
  name: "bm-0" #example
spec:
  serverID: 1682566 #change
  rootDeviceHints:
    raid:
      wwn:
        - "eui.0068475201b4egh2" #change
        - "eui.0068475201b4egh3" #change
  maintenanceMode: false
  description: Test Machine 0 #example
```
