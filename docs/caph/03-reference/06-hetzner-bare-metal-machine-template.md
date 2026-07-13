---
title: HetznerBareMetalMachineTemplate
description: In HetznerBareMetalMachineTemplate you can define all important properties for the HetznerBareMetalMachines.
metatitle: HetznerBareMetalMachineTemplate Object Reference
---

In `HetznerBareMetalMachineTemplate` you can define all important properties for the `HetznerBareMetalMachines`. `HetznerBareMetalMachines` are reconciled by the `HetznerBareMetalMachineController`, which DOES NOT create or delete Hetzner dedicated machines. Instead, it uses the inventory of `HetznerBareMetalHosts`. These hosts correspond to already existing bare metal servers, which get provisioned when selected by a `HetznerBareMetalMachine`.

## Lifecycle of a HetznerBareMetalMachine

### Creating a HetznerBareMetalMachine

Simply put, the specs of a `HetznerBareMetalMachine` consist of two parts. First, there is information about how the bare metal server is supposed to be provisioned. Second, there are properties where you can specify which host to select. If these selectors correspond to a host that is not consumed yet, then the `HetznerBareMetalMachine` transfers important information to the host object. This information is used to provision the host according to what you specified in the specs of `HetznerBareMetalMachineTemplate`. If a host has provisioned successfully, then the `HetznerBareMetalMachine` is considered to be ready.

### Deleting of a HetznerBareMetalMachine

When the `HetznerBareMetalMachine` object gets deleted, it removes the information from the host that the latter used for provisioning. The host then triggers the deprovisioning. As soon as this has been completed, the `HetznerBareMetalMachineController` removes the owner and consumer reference of the host and deletes the finalizer of the machine, so that it can be finally deleted.

### Updating a HetznerBareMetalMachine

Updating a `HetznerBareMetalMachineTemplate` is not possible. Instead, a new template should be created.

## cloud-init and installimage

Both in [installimage](https://docs.hetzner.com/robot/dedicated-server/operating-systems/installimage/) and cloud-init the ports used for SSH can be changed, e.g. with the following code snippet:

```shell
sed -i -e '/^\(#\|\)Port/s/^.*$/Port 2223/' /etc/ssh/sshd_config
```

As the controller needs to know this to be able to successfully provision the server, these ports can be specified in `SSHSpec` of `HetznerBareMetalMachineTemplate`.

When the port is changed in cloud-init, then we additionally need to use the following command to make sure that the change of ports takes immediate effect:
`systemctl restart sshd`

## Choosing the right host

Via MatchLabels you can specify a certain label (key and value) that identifies the host. You get more flexibility with MatchExpressions. This allows decisions like "take any host that has the key "mykey" and let this key have either one of the values "val1", "val2", and "val3".

## Overview of HetznerBareMetalMachineTemplate.Spec

<PropField name="template.spec.providerID" type="string" required={false}>
Provider ID set by controller.
</PropField>

<PropField name="template.spec.installImage" type="object" required={true}>

Configuration used in autosetup.

<Collapsible title="properties">

<PropField name="template.spec.installImage.image" type="object" required={true}>

Defines image for bm machine. See below for details.

<Collapsible title="properties">

<PropField name="template.spec.installImage.image.url" type="string" required={false}>
Remote URL of image. Can be tar, tar.gz, tar.bz, tar.bz2, tar.xz, tgz, tbz, txz.
</PropField>

<PropField name="template.spec.installImage.imageURLCommand" type="string" required={false}>

Basename of a command below `/shared` on the controller pod that CAPH copies into the rescue system and executes instead of `installimage`. Requires `template.spec.installImage.image.url`.

</PropField>

<PropField name="template.spec.installImage.image.name" type="string" required={false}>
Name of the image.
</PropField>

<PropField name="template.spec.installImage.image.path" type="string" required={false}>
Local path of a pre-installed image.
</PropField>

</Collapsible>

</PropField>

<PropField name="template.spec.installImage.postInstallScript" type="string" required={false}>
PostInstallScript that is used for commands that will be executed after installing image.
</PropField>

<PropField name="template.spec.installImage.swraid" type="int" defaultValue="0" required={false}>
Enables or disables raid. Set 1 to enable.
</PropField>

<PropField name="template.spec.installImage.swraidLevel" type="int" defaultValue="1" required={false}>
Defines the software raid levels. Only relevant if raid is enabled. Pick one of 0,1,5,6,10.
</PropField>

<PropField name="template.spec.installImage.partitions" type="[]object" required={true}>

Partitions that should be created in installimage.

<Collapsible title="properties">

<PropField name="template.spec.installImage.partitions.mount" type="string" required={true}>
Mount defines the mount path of the filesystem.
</PropField>

<PropField name="template.spec.installImage.partitions.fileSystem" type="string" required={true}>
Filesystem that should be used. Can be ext2, ext3, ext4, btrfs, reiserfs, xfs, swap, or the name of the LVM volume group, if the partition is a VG.
</PropField>

<PropField name="template.spec.installImage.partitions.size" type="string" required={true}>
Size of the partition. Use 'all' to use all remaining space of the drive. M/G/T can be used as unit specifications for MiB, GiB, TiB.
</PropField>

</Collapsible>

</PropField>

<PropField name="template.spec.installImage.logicalVolumeDefinitions" type="[]object" required={false}>

Defines the logical volume definitions that should be created.

<Collapsible title="properties">

<PropField name="template.spec.installImage.logicalVolumeDefinitions.vg" type="string" required={true}>
Defines the vg name.
</PropField>

<PropField name="template.spec.installImage.logicalVolumeDefinitions.name" type="string" required={true}>
Defines the volume name.
</PropField>

<PropField name="template.spec.installImage.logicalVolumeDefinitions.mount" type="string" required={true}>
Defines the mount path.
</PropField>

<PropField name="template.spec.installImage.logicalVolumeDefinitions.fileSystem" type="string" required={true}>
Defines the file system.
</PropField>

<PropField name="template.spec.installImage.logicalVolumeDefinitions.size" type="string" required={true}>
Defines size with unit M/G/T or MiB/GiB/TiB.
</PropField>

</Collapsible>

</PropField>

<PropField name="template.spec.installImage.btrfsDefinitions" type="[]object" required={false}>

Defines the btrfs sub-volume definitions that should be created.

<Collapsible title="properties">

<PropField name="template.spec.installImage.btrfsDefinitions.volume" type="string" required={true}>
Defines the btrfs volume name.
</PropField>

<PropField name="template.spec.installImage.btrfsDefinitions.subvolume" type="string" required={true}>
Defines the btrfs sub-volume name.
</PropField>

<PropField name="template.spec.installImage.btrfsDefinitions.mount" type="string" required={true}>
Defines the btrfs mount path.
</PropField>

</Collapsible>

</PropField>

</Collapsible>

</PropField>

<PropField name="template.spec.hostSelector" type="object" required={false}>

Options to select hosts with.

<Collapsible title="properties">

<PropField name="template.spec.hostSelector.matchLabels" type="map[string][string]" required={false}>
Specify labels as key-value pairs that should be there in host object to select it.
</PropField>

<PropField name="template.spec.hostSelector.matchExpressions" type="[]object" required={false}>

Requirements using Kubernetes MatchExpressions.

<Collapsible title="properties">

<PropField name="template.spec.hostSelector.matchExpressions.key" type="string" required={true}>
Key of label that should be matched in host object.
</PropField>

<PropField name="template.spec.hostSelector.matchExpressions.operator" type="string" required={true}>

[Selection operator](https://pkg.go.dev/k8s.io/apimachinery@v0.23.4/pkg/selection?utm_source=gopls#Operator).

</PropField>

<PropField name="template.spec.hostSelector.matchExpressions.values" type="[]string" required={true}>
Values whose relation to the label value in the host machine is defined by the selection operator.
</PropField>

</Collapsible>

</PropField>

</Collapsible>

</PropField>

<PropField name="template.spec.sshSpec" type="object" required={true}>

SSH specs.

<Collapsible title="properties">

<PropField name="template.spec.sshSpec.secretRef" type="object" required={true}>

Reference to the secret where SSH key is stored.

<Collapsible title="properties">

<PropField name="template.spec.sshSpec.secretRef.name" type="string" required={true}>
Name of the secret.
</PropField>

<PropField name="template.spec.sshSpec.secretRef.key" type="object" required={true}>

Details about the keys used in the data of the secret.

<Collapsible title="properties">

<PropField name="template.spec.sshSpec.secretRef.key.name" type="string" required={true}>
Name is the key in the secret's data where the SSH key's name is stored.
</PropField>

<PropField name="template.spec.sshSpec.secretRef.key.publicKey" type="string" required={true}>
PublicKey is the key in the secret's data where the SSH key's public key is stored.
</PropField>

<PropField name="template.spec.sshSpec.secretRef.key.privateKey" type="string" required={true}>
PrivateKey is the key in the secret's data where the SSH key's private key is stored.
</PropField>

</Collapsible>

</PropField>

</Collapsible>

</PropField>

<PropField name="template.spec.sshSpec.portAfterInstallImage" type="int" defaultValue="22" required={false}>
PortAfterInstallImage specifies the port that can be used to reach the server via SSH after install image completed successfully.
</PropField>

<PropField name="template.spec.sshSpec.portAfterCloudInit" type="int" defaultValue="22 (install image port)" required={false}>
PortAfterCloudInit specifies the port that can be used to reach the server via SSH after cloud init completed successfully.
</PropField>

</Collapsible>

</PropField>

## installImage.image

You must specify either:

- `name` and `url`
- `path`
- `url` and `imageURLCommand`

Example of an image provided by Hetzner via NFS:

```yaml
image:
  path: /root/.oldroot/nfs//images/Ubuntu-2404-noble-amd64-base.tar.zst
```

Example of an image provided by you via https. The script installimage of Hetzner parses the name to detect the version. It is
recommended to follow their naming pattern.

```yaml
image:
  name: Ubuntu-2404-noble-amd64-custom
  url: https://user:pwd@example.com/images/Ubuntu-2404-noble-amd64-custom.tar.gz
```

Example of pulling an image from an oci-registry:

```yaml
image:
  name: Ubuntu-2404-noble-amd64-custom
  url: oci://ghcr.io/myorg/images/Ubuntu-2404-noble-amd64-custom:1.0.1
```

If you need credentials to pull the image, then provide the environment variable `OCI_REGISTRY_AUTH_TOKEN` to the controller.

You can provide the variable via a secret of the deployment `caph-controller-manager`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  # ...
spec:
  # ...
  template:
    spec:
      containers:
        - command:
            - /manager
          image: ghcr.io/syself/caph:vXXX
          env:
            - name: OCI_REGISTRY_AUTH_TOKEN
              valueFrom:
                secretKeyRef:
                  name: my-oci-registry-secret # The name of the secret
                  key: OCI_REGISTRY_AUTH_TOKEN # The key in the secret. Format: "user:pwd" or just "token"
      # ... other container specs
```

You can push an image to an oci-registry with a tool like [oras](https://oras.land):

```shell
oras push ghcr.io/myorg/images/Ubuntu-2404-noble-amd64-custom:1.0.1 \
    --artifact-type application/vnd.myorg.machine-image.v1 Ubuntu-2404-noble-amd64-custom.tar.gz
```

Example of provisioning a bare metal machine via a custom image-url-command:

```yaml
imageURLCommand: image-url-command-install-foo.sh
image:
  url: oci://ghcr.io/myorg/images/Ubuntu-2404-noble-amd64-custom:1.0.1
```

In this mode, `name` and `path` must be empty.
