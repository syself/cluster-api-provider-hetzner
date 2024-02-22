## Managing SSH keys

### In Hetzner Cloud
In pure HCloud clusters, without bare metal servers, there is no need for SSH keys. All keys that exist in HCloud API and are specified in ```HetznerCluster``` properties are included when provisioning machines. Therefore, they can be used to access those machines via SSH. Note that you have to upload those keys via Hetzner UI or API beforehand. 

The SSH keys can be either specified cluster-wide in the specs of the ```HetznerCluster``` object or scoped to one machine in the specs of ```HCloudMachine```.

If one SSH key is changed in the specs of the cluster, then keep in mind that the SSH key is still valid to access all servers that have been created with it. If it is a potential security vulnerability, then all of these servers should be removed and re-created with the new SSH keys.

### In Hetzner Robot
For bare metal servers, two SSH keys are required. One of them is used for the rescue system, and the other for the actual system. The two can, under the hood, of course, be the same. These SSH keys do not have to be uploaded into Robot API but have to be stored in two secrets (again, the same secret is also possible if the same reference is given twice). Not only the name of the SSH key but also the public and private key. The private key is necessary for provisioning the server with SSH. The SSH key for the actual system is specified in ```HetznerBareMetalMachineTemplate``` - there are no cluster-wide alternatives. The SSH key for the rescue system is defined in a cluster-wide manner in the specs of ```HetznerCluster```.

The secret reference to an SSH key cannot be changed - the secret data, i.e., the SSH key, can. The host that is consumed by the ```HetznerBareMetalMachine``` object reacts in different ways to the change of the secret data of the secret referenced in its specs, depending on its provisioning state. If the host is already provisioned, it will emit an event warning that provisioned hosts can't change SSH keys. The corresponding machine object should instead be deleted and recreated. When the host is provisioning, it restarts this process again if a change of the SSH key makes it necessary. This depends on whether it is the SSH key for the rescue or the actual system and the exact provisioning state.
