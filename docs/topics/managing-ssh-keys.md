## Managing SSH keys

This section provides details about SSH keys and its importance with regards to CAPH.

### What are SSH keys?

SSH keys are a crucial component of secured network communication. They provide a secure and convenient method for authenticating to and communicating with remote servers over unsecured networks. They are used as an access credential in the SSH (Secure Shell) protocol, which is used for logging in remotely from one system to another. SSH keys come in pairs with a public and a private key and its strong encryption is used for executing remote commands and remotely managing vital system components.

### SSH keys in CAPH

In CAPH, SSH keys help in establishing secure communication remotely with Kubernetes nodes running on Hetzner cloud. They help you get complete access to the underlying Kubernetes nodes that are machines provisioned in Hetzner cloud and retrieve required information related to the system. With the help of these keys, you can SSH into the nodes in case of troubleshooting. 

### In Hetzner Cloud
NOTE: You are responsible for uploading your public ssh key to hetzner cloud. This can be done using `hcloud` CLI or hetznercloud console.
All keys that exist in Hetzner Cloud and are specified in `HetznerCluster` spec are included when provisioning machines. Therefore, they can be used to access those machines via SSH. 

```bash
hcloud ssh-key create --name caph --public-key-from-file ~/.ssh/hetzner-cluster.pub
```
Once this is done, you'll have to reference it while creating your cluster.

For example, if you've specified four keys in your hetzner cloud project and you reference all of them while creating your cluster in `HetznerCluster.spec.sshKeys.hcloud` then you can access the machines with all the four keys.
```yaml
  sshKeys:
    hcloud:
    - name: testing
    - name: test
    - name: hello
    - name: another
```

The SSH keys can be either specified cluster-wide in the `HetznerCluster.spec.sshKeys` or scoped to one machine via `HCloudMachine.spec.sshKeys`. The HCloudMachine sshkey overrides the cluster-wide sshkey.

If one SSH key is changed in the specs of the cluster, then keep in mind that the SSH key is still valid to access all servers that have been created with it. If it is a potential security vulnerability, then all of these servers should be removed and re-created with the new SSH keys.

### In Hetzner Robot
For bare metal servers, two SSH keys are required. One of them is used for the rescue system, and the other for the actual system. The two can, under the hood, of course, be the same. These SSH keys do not have to be uploaded into Robot API but have to be stored in two secrets (again, the same secret is also possible if the same reference is given twice). Not only the name of the SSH key but also the public and private key. The private key is necessary for provisioning the server with SSH. The SSH key for the actual system is specified in `HetznerBareMetalMachineTemplate` - there are no cluster-wide alternatives. The SSH key for the rescue system is defined in a cluster-wide manner in the specs of `HetznerCluster`.

The secret reference to an SSH key cannot be changed - the secret data, i.e., the SSH key, can. The host that is consumed by the `HetznerBareMetalMachine` object reacts in different ways to the change of the secret data of the secret referenced in its specs, depending on its provisioning state. If the host is already provisioned, it will emit an event warning that provisioned hosts can't change SSH keys. The corresponding machine object should instead be deleted and recreated. When the host is provisioning, it restarts this process again if a change of the SSH key makes it necessary. This depends on whether it is the SSH key for the rescue or the actual system and the exact provisioning state.
