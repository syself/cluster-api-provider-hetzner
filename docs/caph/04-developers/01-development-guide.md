---
title: Developing Cluster API Provider Hetzner
---

Developing our provider is quite easy. Please follow the steps mentioned below:

1. You need to install some base requirements.
2. You need to follow the [preparation document](/docs/caph/01-getting-started/01-preparation) to set up everything related to Hetzner.

## Install Base requirements

To develop with Tilt, there are a few requirements. You can use the command `make all-tools` to check whether the versions of the tools are up to date and to install the ones that are missing.

This ensures the following:

- clusterctl
- ctlptl (required)
- go (required)
- helm (required)
- helmfile
- kind (required)
- kubectl (required)
- packer
- tilt (required)
- hcloud

## Preparing Hetzner project

For more information, please see [here](/docs/caph/01-getting-started/01-preparation).

## Setting Tilt up

You need to create a `.envrc` file and specify the values you need. After the `.envrc` is loaded, invoke `direnv allow` to load the environment variables in your current shell session.

The complete reference can be found [here](/docs/caph/04-developers/02-tilt).

## Developing with Tilt

![tilt](https://syself.com/images/tilt.png)

Provider Integration development requires a lot of iteration, and the “build, tag, push, update deployment” workflow can be very tedious. Tilt makes this process much simpler by watching for updates and automatically building and deploying them. To build a kind cluster and to start Tilt, run:

```shell
make tilt-up
```

> To access the Tilt UI, please go to: `http://localhost:10351`

Once your kind management cluster is up and running, you can deploy a workload cluster. This could be done through the Tilt UI by pressing one of the buttons in the top right corner, e.g., **"Create Workload Cluster - without Packer"**. This triggers the `make create-workload-cluster` command, which uses the environment variables (we defined in the .envrc) and the cluster-template. Additionally, it installs cilium as CNI.

If you update the API in some way, you need to run `make generate` to generate everything related to kubebuilder and the CRDs.

To tear down the workload cluster, press the **"Delete Workload Cluster"** button. After a few minutes, the resources should be deleted.

To tear down the kind cluster, use:

```shell
$ make delete-mgt-cluster
```

To delete the registry, use `make delete-registry`. Use `make delete-mgt-cluster-registry` to delete both management cluster and associated registry.

If you have any trouble finding the right command, you can run the `make help` command to get a list of all available make targets.

## Submitting PRs and testing

Pull requests and issues are highly encouraged! For more information, please have a look at the [Contribution Guidelines](https://github.com/syself/cluster-api-provider-hetzner/blob/main/CONTRIBUTING)

There are two important commands that you should make use of before creating the PR.

With `make verify`, you can run all linting checks and others. Make sure that all of these checks pass - otherwise, the PR cannot be merged. Note that you need to commit all changes for the last checks to pass.

With `make test`, all unit tests are triggered. If they fail out of nowhere, then please re-run them. They are not 100% stable and sometimes there are tests failing due to something related to Kubernetes' `envtest`.

With `make generate`, new CRDs are generated. This is necessary if you change the API.

## Running local e2e test

If you are interested in running the E2E tests locally, then you can use the following commands:

```shell
export HCLOUD_TOKEN=<your-hcloud-token>
export CAPH_LATEST_VERSION=<latest-version>
export HETZNER_ROBOT_USER=<your robot user>
export HETZNER_ROBOT_PASSWORD=<your robot password>
export HETZNER_SSH_PUB=<your-ssh-pub-key>
export HETZNER_SSH_PRIV=<your-ssh-private-key>
make test-e2e
```

For the SSH public and private keys, you should use the following command to encode the keys. Note that the E2E test will not work if the ssh key is in any other format!

```shell
export HETZNER_SSH_PRIV=$(cat ~/.ssh/cluster | base64 -w0)
```

## Creating new user in Robot

To create new user in Robot, click on the `Create User` button in the Hetzner Robot console. Once you create the new user, a user ID will be provided to you via email from Hetzner Robot. The password will be the same that you used while creating the user.

![robot user](https://syself.com/images/robot-user.png)
