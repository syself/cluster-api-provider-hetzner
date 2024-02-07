# Upgrading the Kubernetes Cluster API Provider Hetzner

This guide explains how to upgrade Cluster API Provider Hetzner (aka CAPH).

## Set matching kubeconfig

Connect `kubectl` to the management cluster.

We use `.envrc` files with [direnv](https://direnv.net/),
but this is optional.

```
❯ cd mgm-cluster/
direnv: loading ~/mgm-cluster/.envrc
direnv: export +KUBECONFIG +STARSHIP_CONFIG
```

Check, that you are connected to the correct cluster:

```
❯ k config current-context 
mgm-cluster-admin@mgm-cluster
```

OK, looks good.

# Update clusterctl

Is clusterctl still up to date?

```
❯ clusterctl version
clusterctl version: &version.Info{Major:"1", Minor:"3", GitVersion:"v1.3.2", GitCommit:"18c6e8e6cda0eaf71d509258186fa8db30a8fa62", GitTreeState:"clean", BuildDate:"2023-01-10T13:20:59Z", GoVersion:"go1.19.4", Compiler:"gc", Platform:"linux/amd64"}
```

You can see the current version here:

https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl

If your clusterctl is outdated, then upgrade it. See the above URL for details.

# clusterctl upgrade plan

Have a look at what could get upgraded:

```
❯ clusterctl upgrade plan
Checking cert-manager version...
Cert-Manager will be upgraded from "v1.10.1" to "v1.11.0"

Checking new release availability...

Latest release available for the v1beta1 API Version of Cluster API (contract):

NAME                     NAMESPACE                             TYPE                     CURRENT VERSION   NEXT VERSION
bootstrap-kubeadm        capi-kubeadm-bootstrap-system         BootstrapProvider        v1.3.2            v1.4.1
control-plane-kubeadm    capi-kubeadm-control-plane-system     ControlPlaneProvider     v1.3.2            v1.4.1
cluster-api              capi-system                           CoreProvider             v1.3.2            v1.4.1
infrastructure-hetzner   cluster-api-provider-hetzner-system   InfrastructureProvider   v1.0.0-beta.14    Already up to date

You can now apply the upgrade by executing the following command:

clusterctl upgrade apply --contract v1beta1
```

Docs: [clusterctl upgrade plan](https://cluster-api.sigs.k8s.io/clusterctl/commands/upgrade.html)

You might be surprised that for `infrastructure-hetzner`, you see the "Already up to date" message below "NEXT VERSION".

`clusterctl upgrade plan` does not display pre-release versions by default.

# Upgrade cluster-API

Use the command, which you saw in the plan:

```
❯ clusterctl upgrade apply --contract v1beta1
Checking cert-manager version...
Deleting cert-manager Version="v1.10.1"
Installing cert-manager Version="v1.11.0"
Waiting for cert-manager to be available...
Performing upgrade...
Scaling down Provider="cluster-api" Version="v1.3.2" Namespace="capi-system"
Scaling down Provider="bootstrap-kubeadm" Version="v1.3.2" Namespace="capi-kubeadm-bootstrap-system"
Scaling down Provider="control-plane-kubeadm" Version="v1.3.2" Namespace="capi-kubeadm-control-plane-system"
Deleting Provider="cluster-api" Version="v1.3.2" Namespace="capi-system"
Installing Provider="cluster-api" Version="v1.4.1" TargetNamespace="capi-system"
Deleting Provider="bootstrap-kubeadm" Version="v1.3.2" Namespace="capi-kubeadm-bootstrap-system"
Installing Provider="bootstrap-kubeadm" Version="v1.4.1" TargetNamespace="capi-kubeadm-bootstrap-system"
Deleting Provider="control-plane-kubeadm" Version="v1.3.2" Namespace="capi-kubeadm-control-plane-system"
Installing Provider="control-plane-kubeadm" Version="v1.4.1" TargetNamespace="capi-kubeadm-control-plane-system"
```


Great, cluster-API was upgraded. 


# Upgrade CAPH

You can find the latest version of CAPH here:

https://github.com/syself/cluster-api-provider-hetzner/tags

```
❯ clusterctl upgrade apply --infrastructure cluster-api-provider-hetzner-system/hetzner:v1.0.0-beta.16
Checking cert-manager version...
Cert-manager is already up to date
Performing upgrade...
Scaling down Provider="infrastructure-hetzner" Version="" Namespace="cluster-api-provider-hetzner-system"
Deleting Provider="infrastructure-hetzner" Version="" Namespace="cluster-api-provider-hetzner-system"
Installing Provider="infrastructure-hetzner" Version="v1.0.0-beta.16" TargetNamespace="cluster-api-provider-hetzner-system"
```

# Check your cluster

Check the health of your cluster with your preferred tools. For example `kubectl`.

```
❯ k get pods -A --sort-by=metadata.creationTimestamp
NAMESPACE                             NAME                                                             READY   STATUS    RESTARTS        AGE
kube-system                           coredns-565d847f94-ppj8z                                         1/1     Running   685 (33d ago)   79d
kube-system                           kube-proxy-6p7lt                                                 1/1     Running   2 (33d ago)     79d
kube-system                           coredns-565d847f94-nrgsk                                         1/1     Running   686 (33d ago)   79d
kube-system                           kube-apiserver-host-cluster-control-plane-64j47                  1/1     Running   970 (33d ago)   79d
kube-system                           kube-scheduler-host-cluster-control-plane-64j47                  1/1     Running   484 (33d ago)   79d
kube-system                           kube-controller-manager-host-cluster-control-plane-64j47         1/1     Running   493 (33d ago)   79d
kube-system                           etcd-host-cluster-control-plane-64j47                            1/1     Running   813 (33d ago)   79d
kube-system                           cilium-operator-6f64975cf7-5489z                                 1/1     Running   524 (33d ago)   79d
kube-system                           cilium-qk7v7                                                     1/1     Running   644 (33d ago)   79d
kube-system                           cilium-operator-6f64975cf7-z9m72                                 1/1     Running   538 (33d ago)   79d
kube-system                           ccm-ccm-hcloud-655cf4fdcc-xjszz                                  1/1     Running   3 (33d ago)     79d
kube-system                           kube-proxy-hbtnt                                                 1/1     Running   1 (35d ago)     79d
kube-system                           cilium-gtvfw                                                     1/1     Running   643 (35d ago)   79d
kube-system                           kube-scheduler-host-cluster-control-plane-t97fn                  1/1     Running   492 (33d ago)   79d
kube-system                           etcd-host-cluster-control-plane-t97fn                            1/1     Running   491 (33d ago)   79d
kube-system                           kube-apiserver-host-cluster-control-plane-t97fn                  1/1     Running   560 (33d ago)   79d
kube-system                           kube-controller-manager-host-cluster-control-plane-t97fn         1/1     Running   492 (33d ago)   79d
kube-system                           hubble-relay-6676b755f6-l7vcd                                    1/1     Running   0               33d
kube-system                           hubble-ui-55f87db549-q4bxb                                       2/2     Running   0               33d
default                               netshoot                                                         1/1     Running   2 (21d ago)     21d
kube-system                           cilium-l8p7s                                                     1/1     Running   0               20d
kube-system                           kube-proxy-n7pwh                                                 1/1     Running   0               20d
kube-system                           kube-scheduler-host-cluster-control-plane-2r25q                  1/1     Running   0               20d
kube-system                           etcd-host-cluster-control-plane-2r25q                            1/1     Running   0               20d
kube-system                           kube-apiserver-host-cluster-control-plane-2r25q                  1/1     Running   0               20d
kube-system                           kube-controller-manager-host-cluster-control-plane-2r25q         1/1     Running   0               20d
cert-manager                          cert-manager-cainjector-ffb4747bb-bt2l7                          1/1     Running   0               10m
cert-manager                          cert-manager-99bb69456-ntvz6                                     1/1     Running   0               10m
cert-manager                          cert-manager-webhook-545bd5d7d8-6cxxk                            1/1     Running   0               10m
capi-system                           capi-controller-manager-746b4f5db4-zzbv9                         1/1     Running   0               9m17s
capi-kubeadm-bootstrap-system         capi-kubeadm-bootstrap-controller-manager-8654485994-tpvkf       1/1     Running   0               9m14s
capi-kubeadm-control-plane-system     capi-kubeadm-control-plane-controller-manager-5d9d9494d5-2mqlc   1/1     Running   0               9m11s
cluster-api-provider-hetzner-system   caph-controller-manager-566f996fbd-jrqc4                         1/1     Running   0               2m30s
```

