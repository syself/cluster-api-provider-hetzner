#!/bin/sh

set -o errexit
set -o nounset
set -o pipefail

# Prerequisites
modprobe overlay
modprobe br_netfilter

sysctl --system

## for cri-o
dnf module -y install go-toolset

dnf install -y \
  containers-common \
  device-mapper-devel \
  git \
  make \
  glib2-devel \
  glibc-devel \
  glibc-static \
  runc \
  go \
  gpgme-devel \
  libassuan \
  libassuan-devel \
  libgpg-error \
  libgpg-error-devel \
  libseccomp \
  libselinux \
  libseccomp-devel \
  libselinux-devel \
  pkgconfig \
  pkgconf-pkg-config 


go get github.com/cpuguy83/go-md2man  

RUNC=v1.0.2    # https://github.com/opencontainers/runc/releases
CRUN=1.2           # https://github.com/containers/crun/releases
CONMON=v2.0.30      # https://github.com/containers/conmon/releases
CRIO=v1.21.3        # https://github.com/cri-o/cri-o/releases
## Remember to check CRI-O Configuration updates when updating CRI-O
CRI_TOOLS=v1.22.0   # https://github.com/kubernetes-sigs/cri-tools/releases

# Install runc
wget https://github.com/opencontainers/runc/releases/download/$RUNC/runc.amd64 -O /usr/local/sbin/runc && chmod +x /usr/local/sbin/runc

# Install crun
wget https://github.com/containers/crun/releases/download/$CRUN/crun-$CRUN-linux-amd64 -O /usr/local/sbin/crun && chmod +x /usr/local/sbin/crun

# Install conmon
wget https://github.com/containers/conmon/releases/download/$CONMON/conmon-x86.zip -O conmon.zip && unzip conmon.zip -d conmon && mv conmon/bin/conmon /usr/local/bin/conmon && chmod +x /usr/local/bin/conmon
rm -rf conmon.zip conmon

# install cri-o
wget https://github.com/cri-o/cri-o/archive/$CRIO.tar.gz
mkdir /tmp/crio && tar zxvf $CRIO.tar.gz -C /tmp/crio --strip-components 1
(cd /tmp/crio && make)
(cd /tmp/crio && sudo make install)
(cd /tmp/crio && sudo make install.config)
(cd /tmp/crio && make install.systemd)
rm -f $CRIO.tar.gz
rm -rf /tmp/crio

# cri-tool https://github.com/kubernetes-sigs/cri-tools
# Install crictl
wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$CRI_TOOLS/crictl-$CRI_TOOLS-linux-amd64.tar.gz
tar zxvf crictl-$CRI_TOOLS-linux-amd64.tar.gz -C /usr/local/bin 
rm -f crictl-$CRI_TOOLS-linux-amd64.tar.gz

# Install critest
wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$CRI_TOOLS/critest-$CRI_TOOLS-linux-amd64.tar.gz
tar zxvf critest-$CRI_TOOLS-linux-amd64.tar.gz -C /usr/local/bin
rm -f critest-$CRI_TOOLS-linux-amd64.tar.gz

# remove default CNIs
rm -f /etc/cni/net.d/100-crio-bridge.conf /etc/cni/net.d/200-loopback.conf

# add default cni directory the config
perl -i -0pe 's#plugin_dirs\s*=\s*\[[^\]]*\]#plugin_dirs = [\n  "/opt/cni/bin",\n  "/usr/libexec/cni"\n]#g' /etc/crio/crio.conf


# CRI-O Configuration
# https://github.com/cri-o/cri-o/blob/master/docs/crio.conf.5.md
cat > /etc/crio/crio.conf <<"EOF"
# The CRI-O configuration file specifies all of the available configuration
# options and command-line flags for the crio(8) OCI Kubernetes Container Runtime
# daemon, but in a TOML format that can be more easily modified and versioned.
#
# Please refer to crio.conf(5) for details of all configuration options.

# CRI-O supports partial configuration reload during runtime, which can be
# done by sending SIGHUP to the running process. Currently supported options
# are explicitly mentioned with: 'This option supports live configuration
# reload'.

# CRI-O reads its storage defaults from the containers-storage.conf(5) file
# located at /etc/containers/storage.conf. Modify this storage configuration if
# you want to change the system's defaults. If you want to modify storage just
# for CRI-O, you can change the storage configuration options here.
[crio]

# Path to the "root directory". CRI-O stores all of its data, including
# containers images, in this directory.
#root = "/var/lib/containers/storage"

# Path to the "run directory". CRI-O stores all of its state in this directory.
#runroot = "/var/run/containers/storage"

# Storage driver used to manage the storage of images and containers. Please
# refer to containers-storage.conf(5) to see all available storage drivers.
#storage_driver = "overlay"

# List to pass options to the storage driver. Please refer to
# containers-storage.conf(5) to see all available storage options.
#storage_option = [
#]

# The default log directory where all logs will go unless directly specified by
# the kubelet. The log directory specified must be an absolute directory.
log_dir = "/var/log/crio/pods"

# Location for CRI-O to lay down the temporary version file.
# It is used to check if crio wipe should wipe containers, which should
# always happen on a node reboot
version_file = "/var/run/crio/version"

# Location for CRI-O to lay down the persistent version file.
# It is used to check if crio wipe should wipe images, which should
# only happen when CRI-O has been upgraded
version_file_persist = "/var/lib/crio/version"

# The crio.api table contains settings for the kubelet/gRPC interface.
[crio.api]

# Path to AF_LOCAL socket on which CRI-O will listen.
listen = "/var/run/crio/crio.sock"

# IP address on which the stream server will listen.
stream_address = "127.0.0.1"

# The port on which the stream server will listen. If the port is set to "0", then
# CRI-O will allocate a random free port number.
stream_port = "0"

# Enable encrypted TLS transport of the stream server.
stream_enable_tls = false

# Path to the x509 certificate file used to serve the encrypted stream. This
# file can change, and CRI-O will automatically pick up the changes within 5
# minutes.
stream_tls_cert = ""

# Path to the key file used to serve the encrypted stream. This file can
# change and CRI-O will automatically pick up the changes within 5 minutes.
stream_tls_key = ""

# Path to the x509 CA(s) file used to verify and authenticate client
# communication with the encrypted stream. This file can change and CRI-O will
# automatically pick up the changes within 5 minutes.
stream_tls_ca = ""

# Maximum grpc send message size in bytes. If not set or <=0, then CRI-O will default to 16 * 1024 * 1024.
grpc_max_send_msg_size = 16777216

# Maximum grpc receive message size. If not set or <= 0, then CRI-O will default to 16 * 1024 * 1024.
grpc_max_recv_msg_size = 16777216

# The crio.runtime table contains settings pertaining to the OCI runtime used
# and options for how to set up and manage the OCI runtime.
[crio.runtime]

# A list of ulimits to be set in containers by default, specified as
# "<ulimit name>=<soft limit>:<hard limit>", for example:
# "nofile=1024:2048"
# If nothing is set here, settings will be inherited from the CRI-O daemon
#default_ulimits = [
#]

# default_runtime is the _name_ of the OCI runtime to be used as the default.
# The name is matched against the runtimes map below. If this value is changed,
# the corresponding existing entry from the runtimes map below will be ignored.
default_runtime = "crun"

# If true, the runtime will not use pivot_root, but instead use MS_MOVE.
no_pivot = false

# decryption_keys_path is the path where the keys required for
# image decryption are stored. This option supports live configuration reload.
decryption_keys_path = "/etc/crio/keys/"

# Path to the conmon binary, used for monitoring the OCI runtime.
# Will be searched for using $PATH if empty.
conmon = "/usr/local/bin/conmon"

# Cgroup setting for conmon
conmon_cgroup = "pod"

# Environment variable list for the conmon process, used for passing necessary
# environment variables to conmon or the runtime.
conmon_env = [
	"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
]

# Additional environment variables to set for all the
# containers. These are overridden if set in the
# container image spec or in the container runtime configuration.
default_env = [
]

# If true, SELinux will be used for pod separation on the host.
selinux = false

# Path to the seccomp.json profile which is used as the default seccomp profile
# for the runtime. If not specified, then the internal default seccomp profile
# will be used. This option supports live configuration reload.
seccomp_profile = ""

# Used to change the name of the default AppArmor profile of CRI-O. The default
# profile name is "crio-default". This profile only takes effect if the user
# does not specify a profile via the Kubernetes Pod's metadata annotation. If
# the profile is set to "unconfined", then this equals to disabling AppArmor.
# This option supports live configuration reload.
apparmor_profile = "crio-default"

# Cgroup management implementation used for the runtime.
cgroup_manager = "cgroupfs"

# List of default capabilities for containers. If it is empty or commented out,
# only the capabilities defined in the containers json file by the user/kube
# will be added.
default_capabilities = [
	"CHOWN",
	"DAC_OVERRIDE",
	"FSETID",
	"FOWNER",
	"SETGID",
	"SETUID",
	"SETPCAP",
	"NET_BIND_SERVICE",
	"KILL",
    "MKNOD",
]

# List of default sysctls. If it is empty or commented out, only the sysctls
# defined in the container json file by the user/kube will be added.
default_sysctls = [
    "net.ipv4.ping_group_range=0 65535",
]

# List of additional devices. specified as
# "<device-on-host>:<device-on-container>:<permissions>", for example: "--device=/dev/sdc:/dev/xvdc:rwm".
#If it is empty or commented out, only the devices
# defined in the container json file by the user/kube will be added.
additional_devices = [
]

# Path to OCI hooks directories for automatically executed hooks. If one of the
# directories does not exist, then CRI-O will automatically skip them.
hooks_dir = [
	"/usr/share/containers/oci/hooks.d",
]

# List of default mounts for each container. **Deprecated:** this option will
# be removed in future versions in favor of default_mounts_file.
default_mounts = [
]

# Path to the file specifying the defaults mounts for each container. The
# format of the config is /SRC:/DST, one mount per line. Notice that CRI-O reads
# its default mounts from the following two files:
#
#   1) /etc/containers/mounts.conf (i.e., default_mounts_file): This is the
#      override file, where users can either add in their own default mounts, or
#      override the default mounts shipped with the package.
#
#   2) /usr/share/containers/mounts.conf: This is the default file read for
#      mounts. If you want CRI-O to read from a different, specific mounts file,
#      you can change the default_mounts_file. Note, if this is done, CRI-O will
#      only add mounts it finds in this file.
#
#default_mounts_file = ""

# Maximum number of processes allowed in a container.
pids_limit = 4096

# Maximum sized allowed for the container log file. Negative numbers indicate
# that no size limit is imposed. If it is positive, it must be >= 8192 to
# match/exceed conmon's read buffer. The file is truncated and re-opened so the
# limit is never exceeded.
log_size_max = -1

# Whether container output should be logged to journald in addition to the kuberentes log file
log_to_journald = false

# Path to directory in which container exit files are written to by conmon.
container_exits_dir = "/var/run/crio/exits"

# Path to directory for container attach sockets.
container_attach_socket_dir = "/var/run/crio"

# The prefix to use for the source of the bind mounts.
bind_mount_prefix = ""

# If set to true, all containers will run in read-only mode.
read_only = false

# Changes the verbosity of the logs based on the level it is set to. Options
# are fatal, panic, error, warn, info, debug and trace. This option supports
# live configuration reload.
log_level = "info"

# Filter the log messages by the provided regular expression.
# This option supports live configuration reload.
log_filter = ""

# The UID mappings for the user namespace of each container. A range is
# specified in the form containerUID:HostUID:Size. Multiple ranges must be
# separated by comma.
uid_mappings = ""

# The GID mappings for the user namespace of each container. A range is
# specified in the form containerGID:HostGID:Size. Multiple ranges must be
# separated by comma.
gid_mappings = ""

# The minimal amount of time in seconds to wait before issuing a timeout
# regarding the proper termination of the container. The lowest possible
# value is 30s, whereas lower values are not considered by CRI-O.
ctr_stop_timeout = 30

# manage_ns_lifecycle determines whether we pin and remove namespaces
# and manage their lifecycle
manage_ns_lifecycle = true

# drop_infra_ctr determines whether CRI-O drops the infra container
# when a pod does not have a private PID namespace, and does not use
# a kernel separating runtime (like kata).
# It requires manage_ns_lifecycle to be true.
drop_infra_ctr = false

# The directory where the state of the managed namespaces gets tracked.
# Only used when manage_ns_lifecycle is true.
namespaces_dir = "/var/run"

# pinns_path is the path to find the pinns binary, which is needed to manage namespace lifecycle
pinns_path = ""

# The "crio.runtime.runtimes" table defines a list of OCI compatible runtimes.
# The runtime to use is picked based on the runtime_handler provided by the CRI.
# If no runtime_handler is provided, the runtime will be picked based on the level
# of trust of the workload. Each entry in the table should follow the format:
#
#[crio.runtime.runtimes.runtime-handler]
#  runtime_path = "/path/to/the/executable"
#  runtime_type = "oci"
#  runtime_root = "/path/to/the/root"
#
# Where:
# - runtime-handler: name used to identify the runtime
# - runtime_path (optional, string): absolute path to the runtime executable in
#   the host filesystem. If omitted, the runtime-handler identifier should match
#   the runtime executable name, and the runtime executable should be placed
#   in $PATH.
# - runtime_type (optional, string): type of runtime, one of: "oci", "vm". If
#   omitted, an "oci" runtime is assumed.
# - runtime_root (optional, string): root directory for storage of containers
#   state.


[crio.runtime.runtimes.runc]
runtime_path = ""
runtime_type = "oci"
runtime_root = "/run/runc"

[crio.runtime.runtimes.crun]
runtime_path = "/usr/local/sbin/crun"
runtime_type = "oci"
runtime_root = "/run/crun"

# crun is a fast and lightweight fully featured OCI runtime and C library for
# running containers
#[crio.runtime.runtimes.crun]

# Kata Containers is an OCI runtime, where containers are run inside lightweight
# VMs. Kata provides additional isolation towards the host, minimizing the host attack
# surface and mitigating the consequences of containers breakout.

# Kata Containers with the default configured VMM
#[crio.runtime.runtimes.kata-runtime]

# Kata Containers with the QEMU VMM
#[crio.runtime.runtimes.kata-qemu]

# Kata Containers with the Firecracker VMM
#[crio.runtime.runtimes.kata-fc]

# The crio.image table contains settings pertaining to the management of OCI images.
#
# CRI-O reads its configured registries defaults from the system wide
# containers-registries.conf(5) located in /etc/containers/registries.conf. If
# you want to modify just CRI-O, you can change the registries configuration in
# this file. Otherwise, leave insecure_registries and registries commented out to
# use the system's defaults from /etc/containers/registries.conf.
[crio.image]

# Default transport for pulling images from a remote container storage.
default_transport = "docker://"

# The path to a file containing credentials necessary for pulling images from
# secure registries. The file is similar to that of /var/lib/kubelet/config.json
global_auth_file = ""

# The image used to instantiate infra containers.
# This option supports live configuration reload.
pause_image = "k8s.gcr.io/pause:3.5"

# The path to a file containing credentials specific for pulling the pause_image from
# above. The file is similar to that of /var/lib/kubelet/config.json
# This option supports live configuration reload.
pause_image_auth_file = ""

# The command to run to have a container stay in the paused state.
# When explicitly set to "", it will fallback to the entrypoint and command
# specified in the pause image. When commented out, it will fallback to the
# default: "/pause". This option supports live configuration reload.
pause_command = "/pause"

# Path to the file which decides what sort of policy we use when deciding
# whether or not to trust an image that we've pulled. It is not recommended that
# this option be used, as the default behavior of using the system-wide default
# policy (i.e., /etc/containers/policy.json) is most often preferred. Please
# refer to containers-policy.json(5) for more details.
signature_policy = ""

# List of registries to skip TLS verification for pulling images. Please
# consider configuring the registries via /etc/containers/registries.conf before
# changing them here.
#insecure_registries = "[]"

# Controls how image volumes are handled. The valid values are mkdir, bind and
# ignore; the latter will ignore volumes entirely.
image_volumes = "mkdir"

# List of registries to be used when pulling an unqualified image (e.g.,
# "alpine:latest"). By default, registries is set to "docker.io" for
# compatibility reasons. Depending on your workload and usecase you may add more
# registries (e.g., "quay.io", "registry.fedoraproject.org",
# "registry.opensuse.org", etc.).
#registries = [
# ]

# Temporary directory to use for storing big files
big_files_temporary_dir = ""

# The crio.network table containers settings pertaining to the management of
# CNI plugins.
[crio.network]

# The default CNI network name to be selected. If not set or "", then
# CRI-O will pick-up the first one found in network_dir.
# cni_default_network = ""

# Path to the directory where CNI configuration files are located.
network_dir = "/etc/cni/net.d/"

# Paths to directories where CNI plugin binaries are located.
plugin_dirs = [
  "/opt/cni/bin",
  "/usr/libexec/cni"
]

# A necessary configuration for Prometheus based metrics retrieval
[crio.metrics]

# Globally enable or disable metrics support.
enable_metrics = true

# The port on which the metrics server will listen.
metrics_port = 9090

# Local socket path to bind the metrics server to
metrics_socket = ""
EOF



#Registries
# https://github.com/containers/image/blob/master/docs/containers-registries.conf.5.md
cat > /etc/containers/registries.conf <<"EOF"

# For more information on this configuration file, see containers-registries.conf(5).
#
# There are multiple versions of the configuration syntax available, where the
# second iteration is backwards compatible to the first one. Mixing up both
# formats will result in an runtime error.
#
# The initial configuration format looks like this:
#
# NOTE: RISK OF USING UNQUALIFIED IMAGE NAMES
# Red Hat recommends always using fully qualified image names including the registry server (full dns name),
# namespace, image name, and tag (ex. registry.redhat.io/ubi8/ubu:latest). When using short names, there is
# always an inherent risk that the image being pulled could be spoofed. For example, a user wants to.
# pull an image named `foobar` from a registry and expects it to come from myregistry.com. If myregistry.com
# is not first in the search list, an attacker could place a different `foobar` image at a registry earlier
# in the search list. The user would accidentally pull and run the attacker's image and code rather than the
# intended content. Red Hat recommends only adding registries which are completely trusted, i.e. registries
# which don't allow unknown or anonymous users to create accounts with arbitrary names. This will prevent
# an image from being spoofed, squatted or otherwise made insecure.  If it is necessary to use one of these
# registries, it should be added at the end of the list.
#
# It is recommended to use fully-qualified images for pulling as the
# destination registry is unambiguous. Pulling by digest
# (i.e., quay.io/repository/name@digest) further eliminates the ambiguity of
# tags.

# The following registries are a set of secure defaults provided by Red Hat.
# Each of these registries provides container images curated, patched
# and maintained by Red Hat and its partners
#[registries.search]
#registries = ['registry.access.redhat.com', 'registry.redhat.io']

# To ensure compatibility with docker we've included docker.io in the default search list. However Red Hat
# does not curate, patch or maintain container images from the docker.io registry.
[registries.search]
registries = ['registry.access.redhat.com', 'registry.redhat.io', 'docker.io']

# The following registries entry can be used for convenience but includes
# container images built by the community. This set of content comes with all
# of the risks of any user generated content including security and performance
# issues. To use this list first comment out the default list, then uncomment
# the following list
#[registries.search]
#registries = ['registry.access.redhat.com', 'registry.redhat.io', 'docker.io', 'quay.io']

# Registries that do not use TLS when pulling images or uses self-signed
# certificates.
[registries.insecure]
registries = []

# Blocked Registries, blocks the `docker daemon` from pulling from the blocked registry.  If you specify
# "*", then the docker daemon will only be allowed to pull from registries listed above in the search
# registries.  Blocked Registries is deprecated because other container runtimes and tools will not use it.
# It is recommended that you use the trust policy file /etc/containers/policy.json to control which
# registries you want to allow users to pull and push from.  policy.json gives greater flexibility, and
# supports all container runtimes and tools including the docker daemon, cri-o, buildah ...
# The atomic CLI `atomic trust` can be used to easily configure the policy.json file.
[registries.block]
registries = []

# The second version of the configuration format allows to specify registry
# mirrors:
#
# # An array of host[:port] registries to try when pulling an unqualified image, in order.
# unqualified-search-registries = ["example.com"]
#
# [[registry]]
# # The "prefix" field is used to choose the relevant [[registry]] TOML table;
# # (only) the TOML table with the longest match for the input image name
# # (taking into account namespace/repo/tag/digest separators) is used.
# #
# # If the prefix field is missing, it defaults to be the same as the "location" field.
# prefix = "example.com/foo"
#
# # If true, unencrypted HTTP as well as TLS connections with untrusted
# # certificates are allowed.
# insecure = false
#
# # If true, pulling images with matching names is forbidden.
# blocked = false
#
# # The physical location of the "prefix"-rooted namespace.
# #
# # By default, this equal to "prefix" (in which case "prefix" can be omitted
# # and the [[registry]] TOML table can only specify "location").
# #
# # Example: Given
# #   prefix = "example.com/foo"
# #   location = "internal-registry-for-example.net/bar"
# # requests for the image example.com/foo/myimage:latest will actually work with the
# # internal-registry-for-example.net/bar/myimage:latest image.
# location = internal-registry-for-example.com/bar"
#
# # (Possibly-partial) mirrors for the "prefix"-rooted namespace.
# #
# # The mirrors are attempted in the specified order; the first one that can be
# # contacted and contains the image will be used (and if none of the mirrors contains the image,
# # the primary location specified by the "registry.location" field, or using the unmodified
# # user-specified reference, is tried last).
# #
# # Each TOML table in the "mirror" array can contain the following fields, with the same semantics
# # as if specified in the [[registry]] TOML table directly:
# # - location
# # - insecure
# [[registry.mirror]]
# location = "example-mirror-0.local/mirror-for-foo"
# [[registry.mirror]]
# location = "example-mirror-1.local/mirrors/foo"
# insecure = true
# # Given the above, a pull of example.com/foo/image:latest will try:
# # 1. example-mirror-0.local/mirror-for-foo/image:latest
# # 2. example-mirror-1.local/mirrors/foo/image:latest
# # 3. internal-registry-for-example.net/bar/myimage:latest
# # in order, and use the first one that exists.
EOF


# Policy for CRI-O
# https://github.com/containers/image/blob/master/docs/containers-policy.json.5.md
cat > /etc/containers/policy.json <<"EOF"
{
    "default": [
        {
            "type": "insecureAcceptAnything"
        }
    ],
    "transports":
        {
            "docker-daemon":
                {
                    "": [{"type":"insecureAcceptAnything"}]
                }
        }
}
EOF


# Storage Configuartion for CRI-O
# https://github.com/containers/storage/blob/master/docs/containers-storage.conf.5.md
cat > /etc/containers/storage.conf <<"EOF"
# This file is is the configuration file for all tools
# that use the containers/storage library.
# See man 5 containers-storage.conf for more information
# The "container storage" table contains all of the server options.
[storage]

# Default Storage Driver
driver = "overlay"

# Temporary storage location
runroot = "/var/run/containers/storage"

# Primary Read/Write location of container storage
graphroot = "/var/lib/containers/storage"

[storage.options]
# Storage options to be passed to underlying storage drivers

# AdditionalImageStores is used to pass paths to additional Read/Only image stores
# Must be comma separated list.
additionalimagestores = [
]

# Size is used to set a maximum size of the container image.  Only supported by
# certain container storage drivers.
size = ""

# Path to an helper program to use for mounting the file system instead of mounting it
# directly.
#mount_program = "/usr/bin/fuse-overlayfs"

# OverrideKernelCheck tells the driver to ignore kernel checks based on kernel version
override_kernel_check = "true"

# mountopt specifies comma separated list of extra mount options
# mountopt = "nodev,metacopy=on"

# Remap-UIDs/GIDs is the mapping from UIDs/GIDs as they should appear inside of
# a container, to UIDs/GIDs as they should appear outside of the container, and
# the length of the range of UIDs/GIDs.  Additional mapped sets can be listed
# and will be heeded by libraries, but there are limits to the number of
# mappings which the kernel will allow when you later attempt to run a
# container.
#
# remap-uids = 0:1668442479:65536
# remap-gids = 0:1668442479:65536

# Remap-User/Group is a name which can be used to look up one or more UID/GID
# ranges in the /etc/subuid or /etc/subgid file.  Mappings are set up starting
# with an in-container ID of 0 and the a host-level ID taken from the lowest
# range that matches the specified name, and using the length of that range.
# Additional ranges are then assigned, using the ranges which specify the
# lowest host-level IDs first, to the lowest not-yet-mapped container-level ID,
# until all of the entries have been used for maps.
#
# remap-user = "storage"
# remap-group = "storage"

[storage.options.thinpool]
# Storage Options for thinpool

# autoextend_percent determines the amount by which pool needs to be
# grown. This is specified in terms of % of pool size. So a value of 20 means
# that when threshold is hit, pool will be grown by 20% of existing
# pool size.
# autoextend_percent = "20"

# autoextend_threshold determines the pool extension threshold in terms
# of percentage of pool size. For example, if threshold is 60, that means when
# pool is 60% full, threshold has been hit.
# autoextend_threshold = "80"

# basesize specifies the size to use when creating the base device, which
# limits the size of images and containers.
# basesize = "10G"

# blocksize specifies a custom blocksize to use for the thin pool.
# blocksize="64k"

# directlvm_device specifies a custom block storage device to use for the
# thin pool. Required if you setup devicemapper.
# directlvm_device = ""

# directlvm_device_force wipes device even if device already has a filesystem.
# directlvm_device_force = "True"

# fs specifies the filesystem type to use for the base device.
# fs="xfs"

# log_level sets the log level of devicemapper.
# 0: LogLevelSuppress 0 (Default)
# 2: LogLevelFatal
# 3: LogLevelErr
# 4: LogLevelWarn
# 5: LogLevelNotice
# 6: LogLevelInfo
# 7: LogLevelDebug
# log_level = "7"

# min_free_space specifies the min free space percent in a thin pool require for
# new device creation to succeed. Valid values are from 0% - 99%.
# Value 0% disables
# min_free_space = "10%"

# mkfsarg specifies extra mkfs arguments to be used when creating the base.
# device.
# mkfsarg = ""

# use_deferred_removal marks devicemapper block device for deferred removal.
# If the thinpool is in use when the driver attempts to remove it, the driver 
# tells the kernel to remove it as soon as possible. Note this does not free
# up the disk space, use deferred deletion to fully remove the thinpool.
# use_deferred_removal = "True"

# use_deferred_deletion marks thinpool device for deferred deletion.
# If the device is busy when the driver attempts to delete it, the driver
# will attempt to delete device every 30 seconds until successful.
# If the program using the driver exits, the driver will continue attempting
# to cleanup the next time the driver is used. Deferred deletion permanently
# deletes the device and all data stored in device will be lost.
# use_deferred_deletion = "True"

# xfs_nospace_max_retries specifies the maximum number of retries XFS should
# attempt to complete IO when ENOSPC (no space) error is returned by
# underlying storage device.
# xfs_nospace_max_retries = "0"

# If specified, use OSTree to deduplicate files with the overlay backend
ostree_repo = ""

# Set to skip a PRIVATE bind mount on the storage home directory.  Only supported by
# certain container storage drivers
skip_mount_home = "false"
EOF




# enable systemd service after next boot
systemctl enable crio.service
systemctl daemon-reload
systemctl enable crio
