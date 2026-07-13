---
title: Reference of Tilt
description: Full list of available Tilt configuration values and their description.
metatitle: Reference For Available Tilt Configuration Options
---

```starlark
"allowed_contexts": [
    "kind-caph",
],
"deploy_cert_manager": True,
"deploy_observability": False,
"preload_images_for_kind": True,
"kind_cluster_name": "caph",
"capi_version": "v1.13.1",
"cabpt_version": "v0.5.6",
"cacppt_version": "v0.4.11",
"cert_manager_version": "v1.20.2",
"kustomize_substitutions": {
    "HCLOUD_REGION": "fsn1",
    "CONTROL_PLANE_MACHINE_COUNT": "3",
    "WORKER_MACHINE_COUNT": "3",
    "KUBERNETES_VERSION": "v1.36.0",
    "HCLOUD_CONTROL_PLANE_MACHINE_TYPE": "cpx32",
    "HCLOUD_WORKER_MACHINE_TYPE": "cpx32",
    "CLUSTER_NAME": "test",
    "HETZNER_SSH_PUB_PATH": "~/.ssh/test",
    "HETZNER_SSH_PRIV_PATH": "~/.ssh/test",
    "HETZNER_ROBOT_USER": "test",
    "HETZNER_ROBOT_PASSWORD": "pw"
}
```

<PropField name="allowed_contexts" type="[]string" defaultValue='["kind-caph"]' required={false}>

A list of kubeconfig contexts Tilt is allowed to use. See the Tilt documentation on [allow_k8s_contexts](https://docs.tilt.dev/api.html#api.allow_k8s_contexts) for more details.

</PropField>

<PropField name="deploy_cert_manager" type="bool" defaultValue="true" required={false}>
If true, deploys cert-manager into the cluster for use for webhook registration.
</PropField>

<PropField name="deploy_observability" type="bool" defaultValue="false" required={false}>

If true, installs grafana, loki and promtail in the dev cluster. Grafana UI will be accessible via a link in the tilt console. Important! This feature requires the `helm` command to be available in the user's path.

</PropField>

<PropField name="preload_images_for_kind" type="bool" defaultValue="true" required={false}>

If set to true, uses `kind load docker-image` to preload images into a kind cluster.

</PropField>

<PropField name="kind_cluster_name" type="string" defaultValue='"caph"' required={false}>
The name of the kind cluster to use when preloading images.
</PropField>

<PropField name="capi_version" type="string" defaultValue='"v1.13.1"' required={false}>
Version of CAPI.
</PropField>

<PropField name="cert_manager_version" type="string" defaultValue='"v1.20.2"' required={false}>
Version of cert manager.
</PropField>
