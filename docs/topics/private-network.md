You can also use different flavors, e.g., to create a cluster with the private network:

```shell
clusterctl generate cluster my-cluster --kubernetes-version v1.28.4 --control-plane-machine-count=3 --worker-machine-count=3  --flavor hcloud-network > my-cluster.yaml
```

All pre-configured flavors can be found on the [release page](https://github.com/syself/cluster-api-provider-hetzner/releases). The cluster-templates start with `cluster-template-`. The flavor name is the suffix.