# Node Images

To use CAPH in production, it needs a node image. In Hetzner Cloud, it is not possible to upload your own images directly. However, a server can be created, configured, and then snapshotted. 
For this, Packer could be used, which already has support for Hetzner Cloud.
In this repository, there is also an example `Packer node-image`. To use it, do the following:
```shell
export HCLOUD_TOKEN=<your-token>

## Only build
packer build templates/node-image/1.28.4-ubuntu-22-04-containerd/image.json

## Debug and ability to ssh into the created server
packer build --debug --on-error=abort templates/node-image/1.28.4-ubuntu-22-04-containerd/image.json
```

The first command is necessary so that Packer is able to create a server in hcloud.
The second one creates the server with Packer. If you are developing your own packer image, the third command could be helpful to check what's going wrong. 

It is essential to know that if you create your own packer image, you need to set a label so that CAPH can find the specified image name. We use for this label the following key: `caph-image-name`
Please have a look at the image.json of the [example node-image](/templates/node-image/1.28.4-ubuntu-22-04-containerd/image.json).

If you use your own node image, make sure also to use a cluster flavor that has `packer` in its name. The default one uses preKubeadm commands to install all necessary things. This is very helpful for testing but is not recommended in a production system.
