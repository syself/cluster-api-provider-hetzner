# Developing Cluster API Provider Hetzner with Tilt

## Getting started

### Create a tilt-settings.json file

Create a `tilt-settings.json` file and place it in your local copy of `cluster-api-provider-hetzner`. Here is an example:

```json
{
  "trigger_mode": "manual",
  "allowed_contexts": ["kind-caph"],
  "deploy_cert_manager": "True",
  "deploy_observability": "False",
  "preload_images_for_kind": "True",
  "kind_cluster_name": "caph",
  "capi_version": "v1.0.1",
  "cabpt_version": "v0.5.0",
  "cacppt_version": "v0.4.0-alpha.0",
  "cert_manager_version": "v1.1.0",
  "kubernetes_version": "v1.21.1",
  "kustomize_substitutions": {
      "HCLOUD_TOKEN": "<Your-Token>",
      "SSH_KEY": "<SSH-KEY-NAME-IN-HCLOUD>",
      "REGION": "fsn1",
      "CONTROL_PLANE_MACHINE_COUNT": "3",
      "WORKER_MACHINE_COUNT": "3",
      "KUBERNETES_VERSION": "v1.21.1",
      "HCLOUD_IMAGE_NAME": "test-image",
      "HCLOUD_CONTROL_PLANE_MACHINE_TYPE": "cpx31",
      "HCLOUD_NODE_MACHINE_TYPE": "cpx31",
      "CLUSTER_NAME": "test"
  },
  "talos-bootstrap": "false"
}
```

#### tilt-settings.json fields

**allowed_contexts** (Array, default=[kind-caph]): A list of kubeconfig contexts Tilt is allowed to use. See the Tilt documentation on
[allow_k8s_contexts](https://docs.tilt.dev/api.html#api.allow_k8s_contexts) for more details.

**kind_cluster_name** (String, default="caph"): The name of the kind cluster to use when preloading images.

**kustomize_substitutions** (Map{String: String}, default=see Tiltfile): An optional map of substitutions for `${}`-style placeholders in the
provider's yaml.

**deploy_observability** (Bool, default=false): If set to true, it will instrall grafana, loki and promtail in the dev
cluster. Grafana UI will be accessible via a link in the tilt console.
Important! This feature requires the `helm` command to be available in the user's path.

**deploy_cert_manager** (Boolean, default=`true`): Deploys cert-manager into the cluster for use for webhook registration.

**preload_images_for_kind** (Boolean, default=`true`): Uses `kind load docker-image` to preload images into a kind cluster.

**trigger_mode** (String, default=`auto`): Optional setting to configure if tilt should automatically rebuild on changes.
Set to `manual` to disable auto-rebuilding and require users to trigger rebuilds of individual changed components through the UI.

**extra_args** (Object, default={}): A mapping of provider to additional arguments to pass to the main binary configured
for this provider. Each item in the array will be passed in to the manager for the given provider.

Example:

```json
{
    "extra_args": {
        "core": ["--feature-gates=MachinePool=true"],
        "kubeadm-bootstrap": ["--feature-gates=MachinePool=true"],
    }
}
```

With this config, the respective managers will be invoked with:

```bash
manager --feature-gates=MachinePool=true
```
