# Reference of Tilt

    "allowed_contexts": [
        "kind-caph",
    ],
    "deploy_cert_manager": True,
    "deploy_observability": False,
    "preload_images_for_kind": True,
    "kind_cluster_name": "caph",
    "capi_version": "v1.6.3",
    "cabpt_version": "v0.5.5",
    "cacppt_version": "v0.4.10",
    "cert_manager_version": "v1.11.0",
    "kustomize_substitutions": {
        "HCLOUD_REGION": "fsn1",
        "CONTROL_PLANE_MACHINE_COUNT": "3",
        "WORKER_MACHINE_COUNT": "3",
        "KUBERNETES_VERSION": "v1.28.4",
        "HCLOUD_IMAGE_NAME": "test-image",
        "HCLOUD_CONTROL_PLANE_MACHINE_TYPE": "cpx31",
        "HCLOUD_WORKER_MACHINE_TYPE": "cpx31",
        "CLUSTER_NAME": "test",
        "HETZNER_SSH_PUB_PATH": "~/.ssh/test",
        "HETZNER_SSH_PRIV_PATH": "~/.ssh/test",
        "HETZNER_ROBOT_USER": "test",
        "HETZNER_ROBOT_PASSWORD": "pw"
    },
| Key | Type | Default | Required | Description |
|-----|-----|------|---------|-------------|
| allowed_contexts | []string | ["kind-caph"] | no | A list of kubeconfig contexts Tilt is allowed to use. See the Tilt documentation on
[allow_k8s_contexts](https://docs.tilt.dev/api.html#api.allow_k8s_contexts) for more details |
| deploy_cert_manager | bool | true | no | If true, deploys cert-manager into the cluster for use for webhook registration |
| deploy_observability | bool | false | no | If true, installs grafana, loki and promtail in the dev cluster. Grafana UI will be accessible via a link in the tilt console. Important! This feature requires the `helm` command to be available in the user's path |
| preload_images_for_kind | bool | true | no | If set to true, uses `kind load docker-image` to preload images into a kind cluster |
| kind_cluster_name | []object | "caph" | no | The name of the kind cluster to use when preloading images |
| capi_version | string | "v1.6.3" | no | Version of CAPI |
| cert_manager_version | string | "v1.11.0" | no | Version of cert manager |
| kustomize_substitutions | map[string]string | {
        "HCLOUD_REGION": "fsn1",
        "CONTROL_PLANE_MACHINE_COUNT": "3",
        "WORKER_MACHINE_COUNT": "3",
        "KUBERNETES_VERSION": "v1.28.4",
        "HCLOUD_IMAGE_NAME": "test-image",
        "HCLOUD_CONTROL_PLANE_MACHINE_TYPE": "cpx31",
        "HCLOUD_WORKER_MACHINE_TYPE": "cpx31",
        "CLUSTER_NAME": "test",
        "HETZNER_SSH_PUB_PATH": "~/.ssh/test",
        "HETZNER_SSH_PRIV_PATH": "~/.ssh/test",
        "HETZNER_ROBOT_USER": "test",
        "HETZNER_ROBOT_PASSWORD": "pw"
    }, | no | An optional map of substitutions for `${}`-style placeholders in the provider's yaml |
