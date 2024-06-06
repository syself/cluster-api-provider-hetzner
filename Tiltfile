# -*- mode: Python -*-
load("ext://uibutton", "cmd_button", "location")
load("ext://restart_process", "docker_build_with_restart")

kustomize_cmd = "./hack/tools/bin/kustomize"
envsubst_cmd = "./hack/tools/bin/envsubst"
tools_bin = "./hack/tools/bin"

#Add tools to path
os.putenv("PATH", os.getenv("PATH") + ":" + tools_bin)

update_settings(k8s_upsert_timeout_secs = 60)  # on first tilt up, often can take longer than 30 seconds

# set defaults
settings = {
    "allowed_contexts": [
        "kind-caph",
    ],
    "deploy_cert_manager": True,
    "deploy_observability": False,
    "preload_images_for_kind": True,
    "kind_cluster_name": "caph",
    "capi_version": "v1.7.3",
    "cabpt_version": "v0.5.6",
    "cacppt_version": "v0.4.11",
    "cert_manager_version": "v1.11.0",
    "extra_args": {
        "hetzner": [
            "--log-level=debug",
            "--debug-hcloud-api-calls=true"
        ],
    },
    "kustomize_substitutions": {
    },
}

# global settings
settings.update(read_yaml(
    "tilt-settings.yaml",
    default = {},
))

if settings.get("trigger_mode") == "manual":
    trigger_mode(TRIGGER_MODE_MANUAL)

if "allowed_contexts" in settings:
    allow_k8s_contexts(settings.get("allowed_contexts"))

if "default_registry" in settings:
    default_registry(settings.get("default_registry"))

# deploy CAPI
def deploy_capi():
    version = settings.get("capi_version")
    capi_uri = "https://github.com/kubernetes-sigs/cluster-api/releases/download/{}/cluster-api-components.yaml".format(version)
    cmd = "curl -sSL {} | {} | kubectl apply -f -".format(capi_uri, envsubst_cmd)
    local(cmd, quiet = True)
    if settings.get("extra_args"):
        extra_args = settings.get("extra_args")
        if extra_args.get("core"):
            core_extra_args = extra_args.get("core")
            if core_extra_args:
                for namespace in ["capi-system", "capi-webhook-system"]:
                    patch_args_with_extra_args(namespace, "capi-controller-manager", core_extra_args)
        if extra_args.get("kubeadm-bootstrap"):
            kb_extra_args = extra_args.get("kubeadm-bootstrap")
            if kb_extra_args:
                patch_args_with_extra_args("capi-kubeadm-bootstrap-system", "capi-kubeadm-bootstrap-controller-manager", kb_extra_args)


def patch_args_with_extra_args(namespace, name, extra_args):
    args_str = str(local("kubectl get deployments {} -n {} -o jsonpath='{{.spec.template.spec.containers[0].args}}'".format(name, namespace)))
    args_to_add = [arg for arg in extra_args if arg not in args_str]
    if args_to_add:
        args = args_str[1:-1].split()
        args.extend(args_to_add)
        patch = [{
            "op": "replace",
            "path": "/spec/template/spec/containers/0/args",
            "value": args,
        }]
        local("kubectl patch deployment {} -n {} --type json -p='{}'".format(name, namespace, str(encode_json(patch)).replace("\n", "")))

# Users may define their own Tilt customizations in tilt.d. This directory is excluded from git and these files will
# not be checked in to version control.
def include_user_tilt_files():
    user_tiltfiles = listdir("tilt.d")
    for f in user_tiltfiles:
        include(f)

def append_arg_for_container_in_deployment(yaml_stream, name, namespace, contains_image_name, args):
    for item in yaml_stream:
        if item["kind"] == "Deployment" and item.get("metadata").get("name") == name and item.get("metadata").get("namespace") == namespace:
            containers = item.get("spec").get("template").get("spec").get("containers")
            for container in containers:
                if contains_image_name in container.get("name"):
                    container.get("args").extend(args)

def fixup_yaml_empty_arrays(yaml_str):
    yaml_str = yaml_str.replace("conditions: null", "conditions: []")
    return yaml_str.replace("storedVersions: null", "storedVersions: []")

def set_env_variables():
    substitutions = settings.get("kustomize_substitutions", {})
    print(substitutions)
    arr = [(key, val) for key, val in substitutions.items()]
    for key, val in arr:
        os.putenv(key, val)

## This should have the same versions as the Dockerfile
tilt_dockerfile_header = """
FROM gcr.io/distroless/base:debug as tilt
WORKDIR /

COPY installimage.tgz .

COPY manager .
"""

# Build CAPH and add feature gates
def caph():
    # yaml = str(kustomizesub("./hack/observability")) # build an observable kind deployment by default
    yaml = str(kustomizesub("./config/default"))

    # add extra_args if they are defined
    if settings.get("extra_args"):
        hetzner_extra_args = settings.get("extra_args").get("hetzner")
        if hetzner_extra_args:
            yaml_dict = decode_yaml_stream(yaml)
            append_arg_for_container_in_deployment(yaml_dict, "caph-controller-manager", "caph-system", "manager", hetzner_extra_args)
            yaml = str(encode_yaml_stream(yaml_dict))
            yaml = fixup_yaml_empty_arrays(yaml)

    local("cp data/hetzner-installimage-v1.0.5.tgz .tiltbuild/installimage.tgz")

    # Set up a local_resource build of the provider's manager binary.

    # Forge the build command
    ldflags = "-extldflags \"-static\" " + str(local("hack/version.sh")).rstrip("\n")
    build_env = "CGO_ENABLED=0 GOOS=linux GOARCH=amd64"
    build_cmd = "{build_env} go build -ldflags '{ldflags}' -o .tiltbuild/manager".format(
        build_env = build_env,
        ldflags = ldflags,
    )
    local_resource(
        "manager",
        cmd = "mkdir -p .tiltbuild; " + build_cmd,
        deps = ["api", "config", "controllers", "pkg", "go.mod", "go.sum", "main.go"],
        labels = ["CAPH"],
    )

    entrypoint = ["/manager"]
    extra_args = settings.get("extra_args")
    if extra_args:
        entrypoint.extend(extra_args)

    # Set up an image build for the provider. The live update configuration syncs the output from the local_resource
    # build into the container.
    docker_build_with_restart(
        ref = "ghcr.io/syself/caph-staging",
        context = "./.tiltbuild/",
        dockerfile_contents = tilt_dockerfile_header,
        target = "tilt",
        entrypoint = entrypoint,
        only = ["manager", "installimage.tgz"],
        live_update = [
            sync(".tiltbuild/manager", "/manager"),
        ],
        ignore = ["templates"],
    )

    k8s_yaml(blob(yaml))
    k8s_resource(workload = "caph-controller-manager", labels = ["CAPH"])
    k8s_resource(
        objects = [
            "caph-system:namespace",
            "hetznerbaremetalhosts.infrastructure.cluster.x-k8s.io:customresourcedefinition",
            "hetznerbaremetalmachines.infrastructure.cluster.x-k8s.io:customresourcedefinition",
            "hetznerbaremetalmachinetemplates.infrastructure.cluster.x-k8s.io:customresourcedefinition",
            "hetznerbaremetalremediations.infrastructure.cluster.x-k8s.io:customresourcedefinition",
            "hetznerbaremetalremediationtemplates.infrastructure.cluster.x-k8s.io:customresourcedefinition",
            "hcloudmachines.infrastructure.cluster.x-k8s.io:customresourcedefinition",
            "hcloudmachinetemplates.infrastructure.cluster.x-k8s.io:customresourcedefinition",
            "hetznerclusters.infrastructure.cluster.x-k8s.io:customresourcedefinition",
            "hetznerclustertemplates.infrastructure.cluster.x-k8s.io:customresourcedefinition",
            "caph-mutating-webhook-configuration:mutatingwebhookconfiguration",
            "caph-controller-manager:serviceaccount",
            "caph-leader-election-role:role",
            "caph-manager-role:clusterrole",
            "caph-metrics-reader:clusterrole",
            "caph-leader-election-rolebinding:rolebinding",
            "caph-manager-rolebinding:clusterrolebinding",
            "caph-serving-cert:certificate",
            "caph-selfsigned-issuer:issuer",
            "caph-validating-webhook-configuration:validatingwebhookconfiguration",
        ],
        new_name = "caph-misc",
        labels = ["CAPH"],
    )

    if os.path.exists("baremetalhosts.yaml"):
        k8s_custom_deploy(
            "baremetal-hosts",
            deps = ["caph-controller-manager", "caph-misc"],
            apply_cmd = "kubectl apply -f baremetalhosts.yaml 1>&2",
            delete_cmd = "kubectl delete -f baremetalhosts.yaml",
        )
        k8s_resource(
            "baremetal-hosts",
            labels = ["CAPH"],
            resource_deps = ["caph-controller-manager", "caph-misc"],
        )

def base64_encode(to_encode):
    encode_blob = local("echo '{}' | tr -d '\n' | base64 - | tr -d '\n'".format(to_encode), quiet = True)
    return str(encode_blob)

def base64_encode_file(path_to_encode):
    encode_blob = local("cat {} | tr -d '\n' | base64 - | tr -d '\n'".format(path_to_encode), quiet = True)
    return str(encode_blob)

def read_file_from_path(path_to_read):
    str_blob = local("cat {} | tr -d '\n'".format(path_to_read), quiet = True)
    return str(str_blob)

def base64_decode(to_decode):
    decode_blob = local("echo '{}' | base64 --decode -".format(to_decode), quiet = True)
    return str(decode_blob)

def ensure_envsubst():
    if not os.path.exists(envsubst_cmd):
        local("make {}".format(os.path.abspath(envsubst_cmd)))

def ensure_kustomize():
    if not os.path.exists(kustomize_cmd):
        local("make {}".format(os.path.abspath(kustomize_cmd)))

def kustomizesub(folder):
    yaml = local("hack/kustomize-sub.sh {}".format(folder), quiet = True)
    return yaml

def waitforsystem():
    local("kubectl wait --for=condition=ready --timeout=300s pod --all -n capi-kubeadm-bootstrap-system")
    local("kubectl wait --for=condition=ready --timeout=300s pod --all -n capi-kubeadm-control-plane-system")
    local("kubectl wait --for=condition=ready --timeout=300s pod --all -n capi-system")

def deploy_observability():
    k8s_yaml(blob(str(local("{} build {}".format(kustomize_cmd, "./hack/observability/"), quiet = True))))

    k8s_resource(workload = "promtail", extra_pod_selectors = [{"app": "promtail"}], labels = ["observability"])
    k8s_resource(workload = "loki", extra_pod_selectors = [{"app": "loki"}], labels = ["observability"])
    k8s_resource(workload = "grafana", port_forwards = "3000", extra_pod_selectors = [{"app": "grafana"}], labels = ["observability"])

##############################
# Actual work happens here
##############################
ensure_envsubst()
ensure_kustomize()

include_user_tilt_files()

load("ext://cert_manager", "deploy_cert_manager")

if settings.get("deploy_cert_manager"):
    deploy_cert_manager()

if settings.get("deploy_observability"):
    deploy_observability()

deploy_capi()

set_env_variables()

caph()

waitforsystem()

cmd_button(
    "Create Hcloud Cluster",
    argv = ["make", "create-workload-cluster-hcloud"],
    location = location.NAV,
    icon_name = "switch_access_shortcut_outlined",
    text = "Create Hcloud Cluster",
)

cmd_button(
    "Create Hcloud Cluster - with Packer",
    argv = ["make", "create-workload-cluster-hcloud-packer"],
    location = location.NAV,
    icon_name = "cloud_upload",
    text = "Create Hcloud Cluster - with Packer",
)

cmd_button(
    "Create Hcloud Cluster Private Network",
    argv = ["make", "create-workload-cluster-hcloud-network"],
    location = location.NAV,
    icon_name = "switch_access_shortcut_add_outlined",
    text = "Create Hcloud Cluster Private Network",
)

cmd_button(
    "Create Hcloud Cluster Private Network - with Packer",
    argv = ["make", "create-workload-cluster-hcloud-network-packer"],
    location = location.NAV,
    icon_name = "lock_outlined",
    text = "Create Hcloud Cluster Private Network - with Packer",
)

cmd_button(
    "Create Baremetal Cluster - with hcloud control-planes",
    argv = ["make", "create-workload-cluster-hetzner-hcloud-control-plane"],
    location = location.NAV,
    icon_name = "dns_outline",
    text = "Create Baremetal Cluster - with hcloud control-planes",
)

cmd_button(
    "Create Hetzner Cluster - with baremetal control-planes",
    argv = ["make", "create-workload-cluster-hetzner-baremetal-control-plane"],
    location = location.NAV,
    icon_name = "storage",
    text = "Create Hetzner Cluster - with baremetal control-planes",
)

cmd_button(
    "Create Hetzner Cluster - with baremetal control-planes - remediation",
    argv = ["make", "create-workload-cluster-hetzner-baremetal-control-plane-remediation"],
    location = location.NAV,
    icon_name = "dvr",
    text = "Create Hetzner Cluster - remediation - baremetal control-planes",
)

cmd_button(
    "Delete Cluster",
    argv = ["make", "delete-workload-cluster"],
    location = location.NAV,
    icon_name = "cloud_download",
    text = "Delete Cluster",
)


cmd_button(
    "Add SSH Key to HCloud",
    argv=["make", "add-ssh-pub-key"],
    location=location.NAV,
    icon_name="trending_up",
    text="Add SSH Key to HCloud",
)
