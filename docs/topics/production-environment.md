# Production Environment Best Practices

## HA Cluster API Components
The clusterctl CLI will create all four needed components, such as Cluster API (CAPI), cluster-api-bootstrap-provider-kubeadm (CAPBK), cluster-api-control-plane-kubeadm (KCP), and cluster-api-provider-hetzner (CAPH).
It uses the respective *-components.yaml from the releases. However, these are not highly available. By scaling the components, we can at least reduce the probability of failure. If this is not enough, add anti-affinity rules and PDBs.

Scale up the deployments
```shell
kubectl -n capi-system scale deployment capi-controller-manager --replicas=2

kubectl -n capi-kubeadm-bootstrap-system scale deployment capi-kubeadm-bootstrap-controller-manager --replicas=2

kubectl -n capi-kubeadm-control-plane-system scale deployment capi-kubeadm-control-plane-controller-manager --replicas=2

kubectl -n caph-system scale deployment caph-controller-manager --replicas=2

```
