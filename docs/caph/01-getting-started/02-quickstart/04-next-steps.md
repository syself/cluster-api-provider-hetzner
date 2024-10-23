---
title: Next steps
description: Install components with the latest version using clusterctl command, then switch back to the management cluster and move objects into the new cluster easily.
---

## Switching to the workload cluster

As a next step, you need to switch to the workload cluster and the below command will do it:

```shell
export KUBECONFIG=/tmp/workload-kubeconfig
```

## Moving components

To move the Cluster API objects from your bootstrap cluster to the new management cluster, firstly you need to install the Cluster API controllers. To install the components with the latest version, run the below command:

```shell
clusterctl init --core cluster-api --bootstrap kubeadm --control-plane kubeadm --infrastructure hetzner
```

{% callout %}

For a specific version, use the flag `--infrastructure hetzner:vX.X.X` with the above command.

{% /callout %}

You can switch back to the management cluster with the following command:

```shell
export KUBECONFIG=~/.kube/config
```

Move the objects into the new cluster by using:

```shell
clusterctl move --to-kubeconfig $CAPH_WORKER_CLUSTER_KUBECONFIG
```

Clusterctl Flags:

| Flag                      | Description                                                                                                                       |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| _--namespace_             | The namespace where the workload cluster is hosted. If unspecified, the current context's namespace is used.                      |
| _--kubeconfig_            | Path to the kubeconfig file for the source management cluster. If unspecified, default discovery rules apply.                     |
| _--kubeconfig-context_    | Context to be used within the kubeconfig file for the source management cluster. If empty, the current context will be used.      |
| _--to-kubeconfig_         | Path to the kubeconfig file to use for the destination management cluster.                                                        |
| _--to-kubeconfig-context_ | Context to be used within the kubeconfig file for the destination management cluster. If empty, the current context will be used. |
