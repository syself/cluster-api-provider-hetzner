## HCloudMachineTemplate

In ```HCloudMachineTemplate``` you can define all important properties for ```HCloudMachines```. ```HCloudMachines``` are reconciled by the ```HCloudMachineController```, which creates and deletes servers in Hetzner Cloud. 

### Overview of HCloudMachineTemplate.Spec
| Key | Type | Default | Required | Description |
|-----|-----|------|---------|-------------|
| template.spec.providerID | string |  | no | ProviderID set by controller |
| template.spec.type | string |  | yes | Desired server type of server in Hetzner's Cloud API. Example: cpx11 |
| template.spec.imageName | string | | yes | Specifies desired image of server. ImageName can reference an image uploaded to Hetzner API in two ways: either directly as name of an image, or as label of an image (see [here](https://github.com/syself/cluster-api-provider-hetzner/blob/main/docs/topics/node-image.md) for more details) |
| template.spec.sshKeys | object | | no | SSHKeys that are scoped to this machine |
| template.spec.sshKeys.hcloud | []object | | no | SSH keys for HCloud |
| template.spec.sshKeys.hcloud.name | string | | yes | Name of SSH key |
| template.spec.sshKeys.hcloud.fingerprint | string | | no| Fingerprint of SSH key - used by the controller |
| template.spec.placementGroupName | string | | no | Placement group of the machine in HCloud API, must be referencing an existing placement group |
