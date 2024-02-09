## HetznerBareMetalHost

The `HetznerBareMetalHost` object has a one-to-one relationship to a Hetzner dedicated server. Its ID is specified in the specs. The host object does not belong to a certain `HetznerCluster`, but can be used by multiple clusters. This is useful, as one host object per server is enough and you can easily see whether a host is used by one of your clusters or not.

There are not many properties that are relevant to the host object. The WWN of the storage device that should be used for provisioning has to be specified in `rootDeviceHints` - but not right from the start. This property can be updated after the host starts the provisioning phase and writes all `hardwareDetails` in the host's status. From there, you can copy the WWN of the storage device that suits your needs and add it to your `HetznerBareMetalHost` object.

#### Find the WWN

After you have started the provisioning, run the following on your management cluster to find the `hardwareDetails` of all of your bare metal hosts.

```shell
kubectl describe hetznerbaremetalhost
```

### Lifecycle of a HetznerBareMetalHost

A host object is available for consumption right after it has been created. When a `HetznerBareMetalMachine` chooses the host, it updates the host's status. This triggers the provisioning of the host. When the `HetznerBareMetalMachine` gets deleted, then the host deprovisions and returns to the state where it is available for new consumers.

`HetznerBareMetalHosts` can only be deleted when they are in the neutral state. In order to delete them, they should be first set to maintenance mode, so that no `HetznerBareMetalMachine` consumes it.

Host objects cannot be updated and have to be deleted and re-created if some of the properties change.

#### Maintenance mode

Maintenance mode means that the host will not be consumed by any `HetznerBareMetalMachine`. If it is already consumed, then the corresponding `HetznerBareMetalMachine` will be deleted and the `HetznerBareMetalHost` deprovisioned.

### Overview of HetznerBareMetalHost.Spec

| Key                      | Type      | Default | Required | Description                                                                                                                                                                                                                                                                            |
| ------------------------ | --------- | ------- | -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| serverID                 | int       |         | yes      | Server ID of the Hetzner dedicated server, you can find it on your Hetzner robot dashboard                                                                                                                                                                                             |
| rootDeviceHints          | object    |         | no       | It is important to find the correct root device. If none are specified, the host will stop provisioning in between to wait for the details to be specified. HardwareDetails in the host's status can be used to find the correct device. Currently, you can specify one disk or a raid setup |
| rootDeviceHints.wwn      | string    |         | no       | Unique storage identifier for non raid setups                                                                                                                                                                                                                                          |
| rootDeviceHints.raid     | object    |         | no       | Used to provide the controller with information on which disks a raid can be established                                                                                                                                                                                               |
| rootDeviceHints.raid.wwn | []string |         | no       | Defines a list of Unique storage identifiers used for raid setups                                                                                                                                                                                                                       |
| consumerRef              | object    |         | no       | Used by the controller and references the bare metal machine that consumes this host                                                                                                                                                                                                   |
| maintenanceMode          | bool      |         | no       | If set to true, the host deprovisions and will not be consumed by any bare metal machine                                                                                                                                                                                               |
| description              | string    |         | no       | Description can be used to store some valuable information about this host                                                                                                                                                                                                             |
| status                   | object    |         | no       | The controller writes this status. As there are some that cannot be regenerated during any reconcilement, the status is in the specs of the object - not the actual status. DO NOT EDIT!!!                                                                                             |

### Example of the HetznerBareMetalHost object

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
