---
title: HCloudMachineTemplate
description: In HCloudMachineTemplate you can define all important properties for HCloudMachines, which are reconciled by the `HCloudMachineController`, responsible for creating and deleting servers in Hetzner Cloud.
metatitle: HCloudMachineController Object Reference
---

In `HCloudMachineTemplate` you can define all important properties for `HCloudMachines`. `HCloudMachines` are reconciled by the `HCloudMachineController`, which creates and deletes servers in Hetzner Cloud.

## Overview of HCloudMachineTemplate.Spec

<PropField name="template.spec.providerID" type="string" required={false}>
ProviderID set by controller.
</PropField>

<PropField name="template.spec.type" type="string" required={true}>
Desired server type of server in Hetzner's Cloud API. Example: cpx11.
</PropField>

<PropField name="template.spec.imageName" type="string" required={true}>

Specifies desired image of server. ImageName can reference an image uploaded to Hetzner API in two ways: either directly as name of an image, or as label of an image (see [here](/docs/caph/topics/node-image) for more details).

</PropField>

<PropField name="template.spec.sshKeys" type="object" required={false}>

SSHKeys that are scoped to this machine.

<Collapsible title="properties">

<PropField name="template.spec.sshKeys.hcloud" type="[]object" required={false}>

SSH keys for HCloud.

<Collapsible title="properties">

<PropField name="template.spec.sshKeys.hcloud.name" type="string" required={true}>
Name of SSH key.
</PropField>

<PropField name="template.spec.sshKeys.hcloud.fingerprint" type="string" required={false}>
Fingerprint of SSH key - used by the controller.
</PropField>

</Collapsible>

</PropField>

</Collapsible>

</PropField>

<PropField name="template.spec.placementGroupName" type="string" required={false}>
Placement group of the machine in HCloud API, must be referencing an existing placement group.
</PropField>

<PropField name="template.spec.publicNetwork" type="object" defaultValue={"{enableIPv4: true, enabledIPv6: true}"} required={false}>

Specs about primary IP address of server. If both IPv4 and IPv6 are disabled, then the private network has to be enabled.

<Collapsible title="properties">

<PropField name="template.spec.publicNetwork.enableIPv4" type="bool" defaultValue="true" required={false}>
Defines whether server has IPv4 address enabled. As Hetzner load balancers require an IPv4 address, this setting will be ignored and set to true if there is no private net.
</PropField>

<PropField name="template.spec.publicNetwork.enableIPv6" type="bool" defaultValue="true" required={false}>
Defines whether server has IPv6 address enabled.
</PropField>

</Collapsible>

</PropField>
