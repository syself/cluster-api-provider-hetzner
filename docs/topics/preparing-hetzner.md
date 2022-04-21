## Preparing Hetzner

There are several tasks that have to be completed, before a workload cluster can be created.

### Preparing Hetzner Cloud

1. Create a new [HCloud project](https://console.hetzner.cloud/projects). 
1. Generate an API token with read and write access. You'll find this if you click on the project and go to "security". Third, 
1. If you want to use it, generate an SSH key, upload the public key to HCloud (also via "security") and give it a name.

### Preparing Hetzner Robot

1. Create a new web service user. [Here](https://robot.your-server.de/preferences/index) you can define a password and copy your user name.
1. Generate an SSH key. You can either upload it via Hetzner Robot UI, or you can just rely on the controller to upload a key that it does not find in the robot API. This is possible, as you have to store the public and private key together with the ssh key's name in a secret that the controller reads.