## HetznerBareMetalMachineTemplate

In `HetznerBareMetalMachineTemplate` you can define all important properties for the `HetznerBareMetalMachines`. `HetznerBareMetalMachines` are reconciled by the `HetznerBareMetalMachineController`, which DOES NOT create or delete Hetzner dedicated machines. Instead, it uses the inventory of `HetznerBareMetalHosts`. These hosts correspond to already existing bare metal servers, which get provisioned when selected by a `HetznerBareMetalMachine`.

### Lifecycle of a HetznerBareMetalMachine

#### Creating a HetznerBareMetalMachine

Simply put, the specs of a `HetznerBareMetalMachine` consist of two parts. First, there is information about how the bare metal server is supposed to be provisioned. Second, there are properties where you can specify which host to select. If these selectors correspond to a host that is not consumed yet, then the `HetznerBareMetalMachine` transfers important information to the host object. This information is used to provision the host according to what you specified in the specs of `HetznerBareMetalMachineTemplate`. If a host has provisioned successfully, then the `HetznerBareMetalMachine` is considered to be ready.

#### Deleting of a HetznerBareMetalMachine

When the `HetznerBareMetalMachine` object gets deleted, it removes the information from the host that the latter used for provisioning. The host then triggers the deprovisioning. As soon as this has been completed, the `HetznerBareMetalMachineController` removes the owner and consumer reference of the host and deletes the finalizer of the machine, so that it can be finally deleted.

#### Updating a HetznerBareMetalMachine

Updating a `HetznerBareMetalMachineTemplate` is not possible. Instead, a new template should be created.

## cloud-init and installimage

Both in [installimage](https://docs.hetzner.com/robot/dedicated-server/operating-systems/installimage/) and cloud-init the ports used for SSH can be changed, e.g. with the following code snippet:

```
sed -i -e '/^\(#\|\)Port/s/^.*$/Port 2223/' /etc/ssh/sshd_config
```

As the controller needs to know this to be able to successfully provision the server, these ports can be specified in `SSHSpec` of `HetznerBareMetalMachineTemplate`.

When the port is changed in cloud-init, then we additionally need to use the following command to make sure that the change of ports takes immediate effect:
`systemctl restart sshd`

## Choosing the right host

Via MatchLabels you can specify a certain label (key and value) that identifies the host. You get more flexibility with MatchExpressions. This allows decisions like "take any host that has the key "mykey" and let this key have either one of the values "val1", "val2", and "val3".

### Overview of HetznerBareMetalMachineTemplate.Spec

| Key                                                            | Type                | Default                 | Required | Description                                                                                                                                        |
| -------------------------------------------------------------- | ------------------- | ----------------------- | -------- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| template.spec.providerID                                       | string              |                         | no       | Provider ID set by controller                                                                                                                      |
| template.spec.installImage                                     | object              |                         | yes      | Configuration used in autosetup                                                                                                                    |
| template.spec.installImage.image                               | object              |                         | yes      | Defines image for bm machine. See below for details.                                                                 |
| template.spec.installImage.image.url                           | string              |                         | no       | Remote URL of image. Can be tar, tar.gz, tar.bz, tar.bz2, tar.xz, tgz, tbz, txz                                                                    |
| template.spec.installImage.image.name                          | string              |                         | no       | Name of the image                                                                                                                                  |
| template.spec.installImage.image.path                          | string              |                         | no       | Local path of a pre-installed image                                                                                                                |
| template.spec.installImage.postInstallScript                   | string              |                         | no       | PostInstallScript that is used for commands that will be executed after installing image                                                              |
| template.spec.installImage.swraid                              | int                 | 0                       | no       | Enables or disables raid. Set 1 to enable                                                                                                          |
| template.spec.installImage.swraidLevel                         | int                 | 1                       | no       | Defines the software raid levels. Only relevant if raid is enabled. Pick one of 0,1,5,6,10                                                                                           |
| template.spec.installImage.partitions                          | []object            |                         | yes      | Partitions that should be created in installimage                                                                                                  |
| template.spec.installImage.partitions.mount                    | string              |                         | yes      | Mount defines the mount path of the filesystem                                                                                                     |
| template.spec.installImage.partitions.fileSystem               | string              |                         | yes      | Filesystem that should be used. Can be ext2, ext3, ext4, btrfs, reiserfs, xfs, swap, or the name of the LVM volume group, if the partition is a VG |
| template.spec.installImage.partitions.size                     | string              |                         | yes      | Size of the partition. Use 'all' to use all remaining space of the drive. M/G/T can be used as unit specifications for MiB, GiB, TiB               |
| template.spec.installImage.logicalVolumeDefinitions            | []object            |                         | no       | Defines the logical volume definitions that should be created                                                                                      |
| template.spec.installImage.logicalVolumeDefinitions.vg         | string              |                         | yes      | Defines the vg name                                                                                                                                |
| template.spec.installImage.logicalVolumeDefinitions.name       | string              |                         | yes      | Defines the volume name                                                                                                                            |
| template.spec.installImage.logicalVolumeDefinitions.mount      | string              |                         | yes      | Defines the mount path                                                                                                                             |
| template.spec.installImage.logicalVolumeDefinitions.fileSystem | string              |                         | yes      | Defines the file system                                                                                                                            |
| template.spec.installImage.logicalVolumeDefinitions.size       | string              |                         | yes      | Defines size with unit M/G/T or MiB/GiB/TiB                                                                                                        |
| template.spec.installImage.btrfsDefinitions                    | []object            |                         | no       | Defines the btrfs sub-volume definitions that should be created                                                                                    |
| template.spec.installImage.btrfsDefinitions.volume             | string              |                         | yes      | Defines the btrfs volume name                                                                                                                      |
| template.spec.installImage.btrfsDefinitions.subvolume          | string              |                         | yes      | Defines the btrfs sub-volume name                                                                                                                  |
| template.spec.installImage.btrfsDefinitions.mount              | string              |                         | yes      | Defines the btrfs mount path                                                                                                                       |
| template.spec.hostSelector                                     | object              |                         | no       | Options to select hosts with                                                                                                                       |
| template.spec.hostSelector.matchLabels                         | map[string][string] |                         | no       | Specify labels as key-value pairs that should be there in host object to select it                                                                 |
| template.spec.hostSelector.matchExpressions                    | []object            |                         | no       | Requirements using Kubernetes MatchExpressions                                                                                                     |
| template.spec.hostSelector.matchExpressions.key                | string              |                         | yes      | Key of label that should be matched in host object                                                                                                 |
| template.spec.hostSelector.matchExpressions.operator           | string              |                         | yes      | [Selection operator](https://pkg.go.dev/k8s.io/apimachinery@v0.23.4/pkg/selection?utm_source=gopls#Operator)                                       |
| template.spec.hostSelector.matchExpressions.values             | []string            |                         | yes      | Values whose relation to the label value in the host machine is defined by the selection operator                                                  |
| template.spec.sshSpec                                          | object              |                         | yes      | SSH specs                                                                                                                                          |
| template.spec.sshSpec.secretRef                                | object              |                         | yes      | Reference to the secret where SSH key is stored                                                                                                    |
| template.spec.sshSpec.secretRef.name                           | string              |                         | yes      | Name of the secret                                                                                                                                 |
| template.spec.sshSpec.secretRef.key                            | object              |                         | yes      | Details about the keys used in the data of the secret                                                                                              |
| template.spec.sshSpec.secretRef.key.name                       | string              |                         | yes      | Name is the key in the secret's data where the SSH key's name is stored                                                                            |
| template.spec.sshSpec.secretRef.key.publicKey    | string              |                         | yes      | PublicKey is the key in the secret's data where the SSH key's public key is stored                                                                 |
| template.spec.sshSpec.secretRef.key.privateKey                 | string              |                         | yes      | PrivateKey is the key in the secret's data where the SSH key's private key is stored                                                               |
| template.spec.sshSpec.portAfterInstallImage                    | int                 | 22                      | no       | PortAfterInstallImage specifies the port that can be used to reach the server via SSH after install image completed successfully                   |
| template.spec.sshSpec.portAfterCloudInit                       | int                 | 22 (install image port) | no       | PortAfterCloudInit specifies the port that can be used to reach the server via SSH after cloud init completed successfully                         |

### installImage.image

You must specify either name and url, or a local path.

Example of an image provided by Hetzner via NFS:

```
image:
  path: /root/.oldroot/nfs//images/Ubuntu-2204-jammy-amd64-base.tar.gz
```

Example of an image provided by you via https. The script installimage of Hetzner parses the name to detect the version. It is
recommended to follow their naming pattern.

```
image:
  name: Ubuntu-2204-jammy-amd64-custom
  url: https://user:pwd@example.com/images/Ubuntu-2204-jammy-amd64-custom.tar.gz

```

Example of pulling an image from an oci-registry:

```
image:
  name: Ubuntu-2204-jammy-amd64-custom
  url: oci://ghcr.io/myorg/images/Ubuntu-2204-jammy-amd64-custom:1.0.0-beta.2
```

If you need credentials to pull the image, then provide the environment variable `OCI_REGISTRY_AUTH_TOKEN` to the controller.

You can provide the variable via a secret of the deployment `caph-controller-manager`:

```
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
                name: my-oci-registry-secret    # The name of the secret
                key: OCI_REGISTRY_AUTH_TOKEN    # The key in the secret. Format: "user:pwd" or just "token"
      # ... other container specs
```

You can push an image to an oci-registry with a tool like [oras](https://oras.land):

```
oras push ghcr.io/myorg/images/Ubuntu-2204-jammy-amd64-custom:1.0.0-beta.2 \
    --artifact-type application/vnd.myorg.machine-image.v1 Ubuntu-2204-jammy-amd64-custom.tar.gz
```



