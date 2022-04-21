## HetznerBareMetalHost

The ```HetznerBareMetalHost``` object has a one-to-one relationship to a Hetzner dedicated server. Its ID is specified in the specs. The host object does not belong to a certain ```HetznerCluster```, but can be used by multiple clusters. This is useful, as one host object per server is enough and you can easily see whether a host is used by one of your clusters or not.

 There are not many properties that are relevant for the host object. The WWN of the storage device that should be used for provisioning has to be specified in ```rootDeviceHints``` - but not right from the start. This property can be updated after the host started the provisioning phase and wrote all ```hardwareDetails``` in the host's status. From there, you can copy the WWN of the storage device that suits your needs. 

### Lifecycle of a HetznerBareMetalHost

A host object is available for consumption right after it has been created. When a ```HetznerBareMetalMachine``` chooses the host, it updates the host's status. This triggers the provisioning of the host. When the ```HetznerBareMetalMachine``` gets deleted, then the host deprovisions and returns to the state where it is available for new consumers.

```HetznerBareMetalHosts``` can only be deleted when they are in the neutral state. In order to delete them, they should be first set to maintenance mode, so that no ```HetznerBareMetalMachine``` consumes it.

Host objects cannot be updated and have to be deleted and re-created if some of the properties change. 

#### Maintenance mode
Maintenance mode means that the host will not be consumed by any ```HetznerBareMetalMachine```. If it is already consumed, then the corresponding ```HetznerBareMetalMachine``` will be deleted and the ```HetznerBareMetalHost``` deprovisioned. 

### Overview of HetznerBareMetalHost.Spec
| Key | Type | Default | Required | Description |
|-----|-----|------|---------|-------------|
| serverID | int | | yes | Server ID of the Hetzner dedicated server |
| rootDeviceHints | object | | no | Important to find the correct root device. If none are specified, the host will stop provisioning in between to wait for the details to be specified. HardwareDetails in the host's status can be used to find the correct device. Currently, only WWN is supported |
| rootDeviceHints.wwn | string | | yes | Unique storage identifier |
| consumerRef | object | | no | Used by the controller and references the bare metal machine that consumes this host |
| maintenanceMode | bool | | no | If set to true, the host deprovisions and will not be consumed by any bare metal machine |
| description | string | | no | Description can be used to store some valuable information about this host |
| status | object | | no | The controller writes this status. As there are some that cannot be regenerated during any reconcilement, the status is in the specs of the object - not the actual status. DO NOT EDIT!!! |