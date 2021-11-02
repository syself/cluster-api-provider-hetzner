# -*- mode: Python -*-
load("ext://uibutton", "cmd_button", "location")

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
    "preload_images_for_kind": True,
    "kind_cluster_name": "caph",
    "capi_version": "v1.0.0",
    "cabpt_version": "v0.5.0-alpha.0",
    "cacppt_version": "v0.3.0",
    "cert_manager_version": "v1.1.0",
    "kubernetes_version": "v1.21.1",
    "kustomize_substitutions": {
        "HETZNER_REGION": "fsn1",
        "CONTROL_PLANE_MACHINE_COUNT": "3",
        "WORKER_MACHINE_COUNT": "3",
        "KUBERNETES_VERSION": "v1.21.1",
        "IMAGE": "test",
        "HETZNER_CONTROL_PLANE_MACHINE_TYPE": "cpx31",
        "HETZNER_NODE_MACHINE_TYPE": "cpx31",
        "CLUSTER_NAME": "test",
    },
}

keys = ["TOKEN", "HETZNER_SSH_KEY"]

# global settings
settings.update(read_json(
    "tilt-settings.json",
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
    if settings.get("talos-bootstrap") == "true":
        deploy_talos_bootstrap()
        deploy_talos_controlplane()

def deploy_talos_bootstrap():
    version = settings.get("cabpt_version")
    cabpt_uri = "https://github.com/talos-systems/cluster-api-bootstrap-provider-talos/releases/download/{}/bootstrap-components.yaml".format(version)
    cmd = "curl -sSL {} | {} | kubectl apply -f -".format(cabpt_uri, envsubst_cmd)
    local(cmd, quiet = True)

def deploy_talos_controlplane():
    version = settings.get("cacppt_version")
    cacppt_uri = "https://github.com/talos-systems/cluster-api-control-plane-provider-talos/releases/download/{}/control-plane-components.yaml".format(version)
    cmd = "curl -sSL {} | {} | kubectl apply -f -".format(cacppt_uri, envsubst_cmd)
    local(cmd, quiet = True)

def patch_args_with_extra_args(namespace, name, extra_args):
    args_str = str(local("kubectl get deployments {} -n {} -o jsonpath={{.spec.template.spec.containers[1].args}}".format(name, namespace)))
    args_to_add = [arg for arg in extra_args if arg not in args_str]
    if args_to_add:
        args = args_str[1:-1].split()
        args.extend(args_to_add)
        patch = [{
            "op": "replace",
            "path": "/spec/template/spec/containers/1/args",
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
                if contains_image_name in container.get("image"):
                    container.get("args").extend(args)

def fixup_yaml_empty_arrays(yaml_str):
    yaml_str = yaml_str.replace("conditions: null", "conditions: []")
    return yaml_str.replace("storedVersions: null", "storedVersions: []")

def validate_needed_keys():
    substitutions = settings.get("kustomize_substitutions", {})
    missing = [k for k in keys if k not in substitutions]
    if missing:
        fail("missing kustomize_substitutions keys {} in tilt-setting.json".format(missing))

def set_env_variables():
    substitutions = settings.get("kustomize_substitutions", {})
    print(substitutions)
    arr = [(key, val) for key, val in substitutions.items()]
    for key, val in arr:
        os.putenv(key, val)

def deploy_hetzner_token():
    substitutions = settings.get("kustomize_substitutions", {})
    token = substitutions.get("TOKEN")
    local("kubectl create secret generic hetzner-token --from-literal=token=%s --dry-run=client -o yaml | kubectl apply -f -" % token)

tilt_helper_dockerfile_header = """
# Tilt image
FROM golang:1.16 as tilt-helper
# Support live reloading with Tilt
RUN wget --output-document /restart.sh --quiet https://raw.githubusercontent.com/windmilleng/rerun-process-wrapper/master/restart.sh  && \
    wget --output-document /start.sh --quiet https://raw.githubusercontent.com/windmilleng/rerun-process-wrapper/master/start.sh && \
    chmod +x /start.sh && chmod +x /restart.sh
"""

## This should have the same versions as the Dockerfile
tilt_dockerfile_header = """
FROM alpine:3.13.6 as tilt
WORKDIR /
COPY --from=tilt-helper /start.sh .
COPY --from=tilt-helper /restart.sh .
COPY manager .
ENV PACKER_VERSION=1.7.7
RUN apk add --update git bash wget
ADD https://releases.hashicorp.com/packer/${PACKER_VERSION}/packer_${PACKER_VERSION}_linux_amd64.zip ./
RUN unzip packer_${PACKER_VERSION}_linux_amd64.zip -d /bin
RUN rm -f packer_${PACKER_VERSION}_linux_amd64.zip
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
            append_arg_for_container_in_deployment(yaml_dict, "caph-controller-manager", "caph-system", hetzner_extra_args)
            yaml = str(encode_yaml_stream(yaml_dict))
            yaml = fixup_yaml_empty_arrays(yaml)

    # Set up a local_resource build of the provider's manager binary.
    local_resource(
        "manager",
        cmd = 'mkdir -p .tiltbuild;CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags \'-extldflags "-static"\' -o .tiltbuild/manager',
        deps = ["api", "config", "controllers", "pkg", "go.mod", "go.sum", "main.go"],
    )

    dockerfile_contents = "\n".join([
        tilt_helper_dockerfile_header,
        tilt_dockerfile_header,
    ])

    entrypoint = ["sh", "/start.sh", "/manager"]
    extra_args = settings.get("extra_args")
    if extra_args:
        entrypoint.extend(extra_args)

    # Set up an image build for the provider. The live update configuration syncs the output from the local_resource
    # build into the container.
    docker_build(
        ref = "quay.io/syself/cluster-api-provider-hetzner",
        context = "./.tiltbuild/",
        dockerfile_contents = dockerfile_contents,
        target = "tilt",
        entrypoint = entrypoint,
        only = "manager",
        live_update = [
            sync(".tiltbuild/manager", "/manager"),
            run("sh /restart.sh"),
        ],
        ignore = ["templates"],
    )

    k8s_yaml(blob(yaml))

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

def kustomizesub(folder):
    yaml = local("hack/kustomize-sub.sh {}".format(folder), quiet = True)
    return yaml

def waitforsystem():
    local("kubectl wait --for=condition=ready --timeout=300s pod --all -n capi-kubeadm-bootstrap-system")
    local("kubectl wait --for=condition=ready --timeout=300s pod --all -n capi-kubeadm-control-plane-system")
    local("kubectl wait --for=condition=ready --timeout=300s pod --all -n capi-system")

##############################
# Actual work happens here
##############################

include_user_tilt_files()

validate_needed_keys()

load("ext://cert_manager", "deploy_cert_manager")

if settings.get("deploy_cert_manager"):
    deploy_cert_manager()

deploy_capi()

set_env_variables()

deploy_hetzner_token()

caph()

waitforsystem()

cmd_button(
    "Create Workload Cluster",
    argv = ["make", "create-workload-cluster"],
    location = location.NAV,
    icon_name = "cloud_upload",
    text = "Create Workload Cluster",
)

cmd_button(
    "Delete Workload Cluster",
    argv = ["make", "delete-workload-cluster"],
    location = location.NAV,
    icon_name = "cloud_download",
    text = "Delete Workload Cluster",
)
