package cluster

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
	"github.com/tilt-dev/localregistry-go"
	"gopkg.in/yaml.v3"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"

	"github.com/tilt-dev/ctlptl/pkg/api"
)

func kindNetworkName() string {
	networkName := "kind"
	if n := os.Getenv("KIND_EXPERIMENTAL_DOCKER_NETWORK"); n != "" {
		networkName = n
	}
	return networkName
}

// kindAdmin uses the kind CLI to manipulate a kind cluster,
// once the underlying machine has been setup.
type kindAdmin struct {
	iostreams    genericclioptions.IOStreams
	dockerClient dockerClient
}

func newKindAdmin(iostreams genericclioptions.IOStreams, dockerClient dockerClient) *kindAdmin {
	return &kindAdmin{
		iostreams:    iostreams,
		dockerClient: dockerClient,
	}
}

func (a *kindAdmin) EnsureInstalled(ctx context.Context) error {
	_, err := exec.LookPath("kind")
	if err != nil {
		return fmt.Errorf("kind not installed. Please install kind with these instructions: https://kind.sigs.k8s.io/")
	}
	return nil
}

func (a *kindAdmin) kindClusterConfig(desired *api.Cluster, registry *api.Registry) *v1alpha4.Cluster {
	kindConfig := desired.KindV1Alpha4Cluster
	if kindConfig == nil {
		kindConfig = &v1alpha4.Cluster{}
	} else {
		kindConfig = kindConfig.DeepCopy()
	}
	kindConfig.Kind = "Cluster"
	kindConfig.APIVersion = "kind.x-k8s.io/v1alpha4"

	if registry != nil {
		patch := fmt.Sprintf(`[plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:%d"]
  endpoint = ["http://%s:%d"]
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."%s:%d"]
  endpoint = ["http://%s:%d"]
`, registry.Status.HostPort, registry.Name, registry.Status.ContainerPort,
			registry.Name, registry.Status.ContainerPort, registry.Name, registry.Status.ContainerPort)
		kindConfig.ContainerdConfigPatches = append(kindConfig.ContainerdConfigPatches, patch)
	}
	return kindConfig
}

func (a *kindAdmin) Create(ctx context.Context, desired *api.Cluster, registry *api.Registry) error {
	klog.V(3).Infof("Creating cluster with config:\n%+v\n---\n", desired)
	if registry != nil {
		klog.V(3).Infof("Initializing cluster with registry config:\n%+v\n---\n", registry)
	}

	clusterName := desired.Name
	if !strings.HasPrefix(clusterName, "kind-") {
		return fmt.Errorf("all kind clusters must have a name with the prefix kind-*")
	}

	kindName := strings.TrimPrefix(clusterName, "kind-")

	// If a cluster has been registered with Kind, but deleted from our kubeconfig,
	// Kind will refuse to create a new cluster. The only way to salvage it is
	// to delete and recreate.
	exists, err := a.clusterExists(ctx, kindName)
	if err != nil {
		return err
	}

	if exists {
		klog.V(3).Infof("Deleting orphaned KIND cluster: %s", kindName)
		cmd := exec.CommandContext(ctx, "kind", "delete", "cluster", "--name", kindName)
		cmd.Stdout = a.iostreams.Out
		cmd.Stderr = a.iostreams.ErrOut
		err := cmd.Run()
		if err != nil {
			return errors.Wrap(err, "deleting orphaned kind cluster")
		}
	}

	args := []string{"create", "cluster", "--name", kindName}
	if desired.KubernetesVersion != "" {
		kindVersion, err := a.getKindVersion(ctx)
		if err != nil {
			return errors.Wrap(err, "creating cluster")
		}

		node, err := a.getNodeImage(ctx, kindVersion, desired.KubernetesVersion)
		if err != nil {
			return errors.Wrap(err, "creating cluster")
		}
		args = append(args, "--image", node)
	}

	kindConfig := a.kindClusterConfig(desired, registry)
	buf := bytes.NewBuffer(nil)
	encoder := yaml.NewEncoder(buf)
	err = encoder.Encode(kindConfig)
	if err != nil {
		return errors.Wrap(err, "creating kind cluster")
	}

	args = append(args, "--config", "-")

	cmd := exec.CommandContext(ctx, "kind", args...)
	cmd.Stdout = a.iostreams.Out
	cmd.Stderr = a.iostreams.ErrOut
	cmd.Stdin = buf
	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "creating kind cluster")
	}

	networkName := kindNetworkName()

	if registry != nil && !a.inKindNetwork(registry, networkName) {
		_, _ = fmt.Fprintf(a.iostreams.ErrOut, "   Connecting kind to registry %s\n", registry.Name)
		err := a.dockerClient.NetworkConnect(ctx, networkName, registry.Name, nil)
		if err != nil {
			return errors.Wrap(err, "connecting registry")
		}
	}

	return nil
}

func (a *kindAdmin) clusterExists(ctx context.Context, cluster string) (bool, error) {
	buf := bytes.NewBuffer(nil)
	cmd := exec.CommandContext(ctx, "kind", "get", "clusters")
	cmd.Stdout = buf
	cmd.Stderr = a.iostreams.ErrOut
	err := cmd.Run()
	if err != nil {
		return false, errors.Wrap(err, "kind get clusters")
	}

	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == cluster {
			return true, nil
		}
	}
	return false, nil
}

func (a *kindAdmin) inKindNetwork(registry *api.Registry, networkName string) bool {
	for _, n := range registry.Status.Networks {
		if n == networkName {
			return true
		}
	}
	return false
}

func (a *kindAdmin) LocalRegistryHosting(ctx context.Context, desired *api.Cluster, registry *api.Registry) (*localregistry.LocalRegistryHostingV1, error) {
	return &localregistry.LocalRegistryHostingV1{
		Host:                   fmt.Sprintf("localhost:%d", registry.Status.HostPort),
		HostFromClusterNetwork: fmt.Sprintf("%s:%d", registry.Name, registry.Status.ContainerPort),
		Help:                   "https://github.com/tilt-dev/ctlptl",
	}, nil
}

func (a *kindAdmin) Delete(ctx context.Context, config *api.Cluster) error {
	clusterName := config.Name
	if !strings.HasPrefix(clusterName, "kind-") {
		return fmt.Errorf("all kind clusters must have a name with the prefix kind-*")
	}

	kindName := strings.TrimPrefix(clusterName, "kind-")
	cmd := exec.CommandContext(ctx, "kind", "delete", "cluster", "--name", kindName)
	cmd.Stdout = a.iostreams.Out
	cmd.Stderr = a.iostreams.ErrOut
	cmd.Stdin = a.iostreams.In
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "deleting kind cluster")
	}
	return nil
}

func (a *kindAdmin) ModifyConfigInContainer(ctx context.Context, cluster *api.Cluster, containerID string, dockerClient dockerClient, configWriter configWriter) error {
	err := dockerClient.NetworkConnect(ctx, kindNetworkName(), containerID, nil)
	if err != nil {
		if !errdefs.IsForbidden(err) || !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("error connecting to cluster network: %w", err)
		}
	}

	kindName := strings.TrimPrefix(cluster.Name, "kind-")
	return configWriter.SetConfig(
		fmt.Sprintf("clusters.%s.server", cluster.Name),
		fmt.Sprintf("https://%s-control-plane:6443", kindName),
	)
}

func (a *kindAdmin) getNodeImage(ctx context.Context, kindVersion, k8sVersion string) (string, error) {
	nodeTable, ok := kindK8sNodeTable[kindVersion]
	if !ok {
		return "", fmt.Errorf("unsupported Kind version %s.\n"+
			"To set up a specific Kubernetes version in Kind, ctlptl needs an official Kubernetes image.\n"+
			"If you're running an unofficial version of Kind, remove 'kubernetesVersion' from your cluster config to use the default image.\n"+
			"If you're running a newly released version of Kind, please file an issue: https://github.com/tilt-dev/ctlptl/issues/new", kindVersion)
	}

	// Kind doesn't maintain Kubernetes nodes for every patch version, so just get the closest
	// major/minor patch.
	k8sVersionParsed, err := semver.ParseTolerant(k8sVersion)
	if err != nil {
		return "", fmt.Errorf("parsing kubernetesVersion: %v", err)
	}

	simplifiedK8sVersion := fmt.Sprintf("%d.%d", k8sVersionParsed.Major, k8sVersionParsed.Minor)
	node, ok := nodeTable[simplifiedK8sVersion]
	if !ok {
		return "", fmt.Errorf("Kind %s does not support Kubernetes v%s", kindVersion, simplifiedK8sVersion)
	}
	return node, nil
}

func (a *kindAdmin) getKindVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "kind", "version")
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "kind version")
	}

	parts := strings.Split(string(out), " ")
	if len(parts) < 2 {
		return "", fmt.Errorf("parsing kind version output: %s", string(out))
	}

	return parts[1], nil
}

// This table must be built up manually from the Kind release notes each
// time a new Kind version is released :\
var kindK8sNodeTable = map[string]map[string]string{
	"v0.17.0": {
		"1.26": "kindest/node:v1.26.0@sha256:691e24bd2417609db7e589e1a479b902d2e209892a10ce375fab60a8407c7352",
		"1.25": "kindest/node:v1.25.3@sha256:f52781bc0d7a19fb6c405c2af83abfeb311f130707a0e219175677e366cc45d1",
		"1.24": "kindest/node:v1.24.7@sha256:577c630ce8e509131eab1aea12c022190978dd2f745aac5eb1fe65c0807eb315",
		"1.23": "kindest/node:v1.23.13@sha256:ef453bb7c79f0e3caba88d2067d4196f427794086a7d0df8df4f019d5e336b61",
		"1.22": "kindest/node:v1.22.15@sha256:7d9708c4b0873f0fe2e171e2b1b7f45ae89482617778c1c875f1053d4cef2e41",
		"1.21": "kindest/node:v1.21.14@sha256:9d9eb5fb26b4fbc0c6d95fa8c790414f9750dd583f5d7cee45d92e8c26670aa1",
		"1.20": "kindest/node:v1.20.15@sha256:a32bf55309294120616886b5338f95dd98a2f7231519c7dedcec32ba29699394",
		"1.19": "kindest/node:v1.19.16@sha256:476cb3269232888437b61deca013832fee41f9f074f9bed79f57e4280f7c48b7",
	},
	"v0.16.0": {
		"1.25": "kindest/node:v1.25.2@sha256:9be91e9e9cdf116809841fc77ebdb8845443c4c72fe5218f3ae9eb57fdb4bace",
		"1.24": "kindest/node:v1.24.6@sha256:97e8d00bc37a7598a0b32d1fabd155a96355c49fa0d4d4790aab0f161bf31be1",
		"1.23": "kindest/node:v1.23.12@sha256:9402cf1330bbd3a0d097d2033fa489b2abe40d479cc5ef47d0b6a6960613148a",
		"1.22": "kindest/node:v1.22.15@sha256:bfd5eaae36849bfb3c1e3b9442f3da17d730718248939d9d547e86bbac5da586",
		"1.21": "kindest/node:v1.21.14@sha256:ad5b7446dd8332439f22a1efdac73670f0da158c00f0a70b45716e7ef3fae20b",
		"1.20": "kindest/node:v1.20.15@sha256:45d0194a8069c46483a0e509088ab9249302af561ebee76a1281a1f08ecb4ed3",
		"1.19": "kindest/node:v1.19.16@sha256:a146f9819fece706b337d34125bbd5cb8ae4d25558427bf2fa3ee8ad231236f2",
	},
	"v0.15.0": {
		"1.25": "kindest/node:v1.25.0@sha256:428aaa17ec82ccde0131cb2d1ca6547d13cf5fdabcc0bbecf749baa935387cbf",
		"1.24": "kindest/node:v1.24.4@sha256:adfaebada924a26c2c9308edd53c6e33b3d4e453782c0063dc0028bdebaddf98",
		"1.23": "kindest/node:v1.23.10@sha256:f047448af6a656fae7bc909e2fab360c18c487ef3edc93f06d78cdfd864b2d12",
		"1.22": "kindest/node:v1.22.13@sha256:4904eda4d6e64b402169797805b8ec01f50133960ad6c19af45173a27eadf959",
		"1.21": "kindest/node:v1.21.14@sha256:f9b4d3d1112f24a7254d2ee296f177f628f9b4c1b32f0006567af11b91c1f301",
		"1.20": "kindest/node:v1.20.15@sha256:d67de8f84143adebe80a07672f370365ec7d23f93dc86866f0e29fa29ce026fe",
		"1.19": "kindest/node:v1.19.16@sha256:707469aac7e6805e52c3bde2a8a8050ce2b15decff60db6c5077ba9975d28b98",
		"1.18": "kindest/node:v1.18.20@sha256:61c9e1698c1cb19c3b1d8151a9135b379657aee23c59bde4a8d87923fcb43a91",
	},
	"v0.14.0": {
		"1.24": "kindest/node:v1.24.0@sha256:0866296e693efe1fed79d5e6c7af8df71fc73ae45e3679af05342239cdc5bc8e",
		"1.23": "kindest/node:v1.23.6@sha256:b1fa224cc6c7ff32455e0b1fd9cbfd3d3bc87ecaa8fcb06961ed1afb3db0f9ae",
		"1.22": "kindest/node:v1.22.9@sha256:8135260b959dfe320206eb36b3aeda9cffcb262f4b44cda6b33f7bb73f453105",
		"1.21": "kindest/node:v1.21.12@sha256:f316b33dd88f8196379f38feb80545ef3ed44d9197dca1bfd48bcb1583210207",
		"1.20": "kindest/node:v1.20.15@sha256:6f2d011dffe182bad80b85f6c00e8ca9d86b5b8922cdf433d53575c4c5212248",
		"1.19": "kindest/node:v1.19.16@sha256:d9c819e8668de8d5030708e484a9fdff44d95ec4675d136ef0a0a584e587f65c",
		"1.18": "kindest/node:v1.18.20@sha256:738cdc23ed4be6cc0b7ea277a2ebcc454c8373d7d8fb991a7fcdbd126188e6d7",
	},
	"v0.13.0": {
		"1.24": "kindest/node:v1.24.0@sha256:406fd86d48eaf4c04c7280cd1d2ca1d61e7d0d61ddef0125cb097bc7b82ed6a1",
		"1.23": "kindest/node:v1.23.6@sha256:1af0f1bee4c3c0fe9b07de5e5d3fafeb2eec7b4e1b268ae89fcab96ec67e8355",
		"1.22": "kindest/node:v1.22.9@sha256:6e57a6b0c493c7d7183a1151acff0bfa44bf37eb668826bf00da5637c55b6d5e",
		"1.21": "kindest/node:v1.21.12@sha256:ae05d44cc636ee961068399ea5123ae421790f472c309900c151a44ee35c3e3e",
		"1.20": "kindest/node:v1.20.15@sha256:a6ce604504db064c5e25921c6c0fffea64507109a1f2a512b1b562ac37d652f3",
		"1.19": "kindest/node:v1.19.16@sha256:dec41184d10deca01a08ea548197b77dc99eeacb56ff3e371af3193c86ca99f4",
		"1.18": "kindest/node:v1.18.20@sha256:38a8726ece5d7867fb0ede63d718d27ce2d41af519ce68be5ae7fcca563537ed",
	},
	"v0.12.0": {
		"1.23": "kindest/node:v1.23.4@sha256:0e34f0d0fd448aa2f2819cfd74e99fe5793a6e4938b328f657c8e3f81ee0dfb9",
		"1.22": "kindest/node:v1.22.7@sha256:1dfd72d193bf7da64765fd2f2898f78663b9ba366c2aa74be1fd7498a1873166",
		"1.21": "kindest/node:v1.21.10@sha256:84709f09756ba4f863769bdcabe5edafc2ada72d3c8c44d6515fc581b66b029c",
		"1.20": "kindest/node:v1.20.15@sha256:393bb9096c6c4d723bb17bceb0896407d7db581532d11ea2839c80b28e5d8deb",
		"1.19": "kindest/node:v1.19.16@sha256:81f552397c1e6c1f293f967ecb1344d8857613fb978f963c30e907c32f598467",
		"1.18": "kindest/node:v1.18.20@sha256:e3dca5e16116d11363e31639640042a9b1bd2c90f85717a7fc66be34089a8169",
		"1.17": "kindest/node:v1.17.17@sha256:e477ee64df5731aa4ef4deabbafc34e8d9a686b49178f726563598344a3898d5",
		"1.16": "kindest/node:v1.16.15@sha256:64bac16b83b6adfd04ea3fbcf6c9b5b893277120f2b2cbf9f5fa3e5d4c2260cc",
		"1.15": "kindest/node:v1.15.12@sha256:9dfc13db6d3fd5e5b275f8c4657ee6a62ef9cb405546664f2de2eabcfd6db778",
		"1.14": "kindest/node:v1.14.10@sha256:b693339da2a927949025869425e20daf80111ccabf020d4021a23c00bae29d82",
	},
	"v0.11.1": {
		"1.23": "kindest/node:v1.23.0@sha256:49824ab1727c04e56a21a5d8372a402fcd32ea51ac96a2706a12af38934f81ac",
		"1.22": "kindest/node:v1.22.0@sha256:b8bda84bb3a190e6e028b1760d277454a72267a5454b57db34437c34a588d047",
		"1.21": "kindest/node:v1.21.1@sha256:69860bda5563ac81e3c0057d654b5253219618a22ec3a346306239bba8cfa1a6",
		"1.20": "kindest/node:v1.20.7@sha256:cbeaf907fc78ac97ce7b625e4bf0de16e3ea725daf6b04f930bd14c67c671ff9",
		"1.19": "kindest/node:v1.19.11@sha256:07db187ae84b4b7de440a73886f008cf903fcf5764ba8106a9fd5243d6f32729",
		"1.18": "kindest/node:v1.18.19@sha256:7af1492e19b3192a79f606e43c35fb741e520d195f96399284515f077b3b622c",
		"1.17": "kindest/node:v1.17.17@sha256:66f1d0d91a88b8a001811e2f1054af60eef3b669a9a74f9b6db871f2f1eeed00",
		"1.16": "kindest/node:v1.16.15@sha256:83067ed51bf2a3395b24687094e283a7c7c865ccc12a8b1d7aa673ba0c5e8861",
		"1.15": "kindest/node:v1.15.12@sha256:b920920e1eda689d9936dfcf7332701e80be12566999152626b2c9d730397a95",
		"1.14": "kindest/node:v1.14.10@sha256:f8a66ef82822ab4f7569e91a5bccaf27bceee135c1457c512e54de8c6f7219f8",
	},
	"v0.11.0": {
		"1.21": "kindest/node:v1.21.1@sha256:fae9a58f17f18f06aeac9772ca8b5ac680ebbed985e266f711d936e91d113bad",
		"1.20": "kindest/node:v1.20.7@sha256:e645428988191fc824529fd0bb5c94244c12401cf5f5ea3bd875eb0a787f0fe9",
		"1.19": "kindest/node:v1.19.11@sha256:7664f21f9cb6ba2264437de0eb3fe99f201db7a3ac72329547ec4373ba5f5911",
		"1.18": "kindest/node:v1.18.19@sha256:530378628c7c518503ade70b1df698b5de5585dcdba4f349328d986b8849b1ee",
		"1.17": "kindest/node:v1.17.17@sha256:c581fbf67f720f70aaabc74b44c2332cc753df262b6c0bca5d26338492470c17",
		"1.16": "kindest/node:v1.16.15@sha256:430c03034cd856c1f1415d3e37faf35a3ea9c5aaa2812117b79e6903d1fc9651",
		"1.15": "kindest/node:v1.15.12@sha256:8d575f056493c7778935dd855ded0e95c48cb2fab90825792e8fc9af61536bf9",
		"1.14": "kindest/node:v1.14.10@sha256:6033e04bcfca7c5f2a9c4ce77551e1abf385bcd2709932ec2f6a9c8c0aff6d4f",
	},
	"v0.10.0": {
		"1.20": "kindest/node:v1.20.2@sha256:8f7ea6e7642c0da54f04a7ee10431549c0257315b3a634f6ef2fecaaedb19bab",
		"1.19": "kindest/node:v1.19.7@sha256:a70639454e97a4b733f9d9b67e12c01f6b0297449d5b9cbbef87473458e26dca",
		"1.18": "kindest/node:v1.18.15@sha256:5c1b980c4d0e0e8e7eb9f36f7df525d079a96169c8a8f20d8bd108c0d0889cc4",
		"1.17": "kindest/node:v1.17.17@sha256:7b6369d27eee99c7a85c48ffd60e11412dc3f373658bc59b7f4d530b7056823e",
		"1.16": "kindest/node:v1.16.15@sha256:c10a63a5bda231c0a379bf91aebf8ad3c79146daca59db816fb963f731852a99",
		"1.15": "kindest/node:v1.15.12@sha256:67181f94f0b3072fb56509107b380e38c55e23bf60e6f052fbd8052d26052fb5",
		"1.14": "kindest/node:v1.14.10@sha256:3fbed72bcac108055e46e7b4091eb6858ad628ec51bf693c21f5ec34578f6180",
	},
	"v0.9.0": {
		"1.19": "kindest/node:v1.19.1@sha256:98cf5288864662e37115e362b23e4369c8c4a408f99cbc06e58ac30ddc721600",
		"1.18": "kindest/node:v1.18.8@sha256:f4bcc97a0ad6e7abaf3f643d890add7efe6ee4ab90baeb374b4f41a4c95567eb",
		"1.17": "kindest/node:v1.17.11@sha256:5240a7a2c34bf241afb54ac05669f8a46661912eab05705d660971eeb12f6555",
		"1.16": "kindest/node:v1.16.15@sha256:a89c771f7de234e6547d43695c7ab047809ffc71a0c3b65aa54eda051c45ed20",
		"1.15": "kindest/node:v1.15.12@sha256:d9b939055c1e852fe3d86955ee24976cab46cba518abcb8b13ba70917e6547a6",
		"1.14": "kindest/node:v1.14.10@sha256:ce4355398a704fca68006f8a29f37aafb49f8fc2f64ede3ccd0d9198da910146",
		"1.13": "kindest/node:v1.13.12@sha256:1c1a48c2bfcbae4d5f4fa4310b5ed10756facad0b7a2ca93c7a4b5bae5db29f5",
	},
	"v0.8.1": {
		"1.18": "kindest/node:v1.18.2@sha256:7b27a6d0f2517ff88ba444025beae41491b016bc6af573ba467b70c5e8e0d85f",
		"1.17": "kindest/node:v1.17.5@sha256:ab3f9e6ec5ad8840eeb1f76c89bb7948c77bbf76bcebe1a8b59790b8ae9a283a",
		"1.16": "kindest/node:v1.16.9@sha256:7175872357bc85847ec4b1aba46ed1d12fa054c83ac7a8a11f5c268957fd5765",
		"1.15": "kindest/node:v1.15.11@sha256:6cc31f3533deb138792db2c7d1ffc36f7456a06f1db5556ad3b6927641016f50",
		"1.14": "kindest/node:v1.14.10@sha256:6cd43ff41ae9f02bb46c8f455d5323819aec858b99534a290517ebc181b443c6",
		"1.13": "kindest/node:v1.13.12@sha256:214476f1514e47fe3f6f54d0f9e24cfb1e4cda449529791286c7161b7f9c08e7",
		"1.12": "kindest/node:v1.12.10@sha256:faeb82453af2f9373447bb63f50bae02b8020968e0889c7fa308e19b348916cb",
	},
	"v0.8.0": {
		"1.18": "kindest/node:v1.18.2@sha256:7b27a6d0f2517ff88ba444025beae41491b016bc6af573ba467b70c5e8e0d85f",
		"1.17": "kindest/node:v1.17.5@sha256:ab3f9e6ec5ad8840eeb1f76c89bb7948c77bbf76bcebe1a8b59790b8ae9a283a",
		"1.16": "kindest/node:v1.16.9@sha256:7175872357bc85847ec4b1aba46ed1d12fa054c83ac7a8a11f5c268957fd5765",
		"1.15": "kindest/node:v1.15.11@sha256:6cc31f3533deb138792db2c7d1ffc36f7456a06f1db5556ad3b6927641016f50",
		"1.14": "kindest/node:v1.14.10@sha256:6cd43ff41ae9f02bb46c8f455d5323819aec858b99534a290517ebc181b443c6",
		"1.13": "kindest/node:v1.13.12@sha256:214476f1514e47fe3f6f54d0f9e24cfb1e4cda449529791286c7161b7f9c08e7",
		"1.12": "kindest/node:v1.12.10@sha256:faeb82453af2f9373447bb63f50bae02b8020968e0889c7fa308e19b348916cb",
	},
	"v0.7.0": {
		"1.18": "kindest/node:v1.18.0@sha256:0e20578828edd939d25eb98496a685c76c98d54084932f76069f886ec315d694",
		"1.17": "kindest/node:v1.17.0@sha256:9512edae126da271b66b990b6fff768fbb7cd786c7d39e86bdf55906352fdf62",
		"1.16": "kindest/node:v1.16.4@sha256:b91a2c2317a000f3a783489dfb755064177dbc3a0b2f4147d50f04825d016f55",
		"1.15": "kindest/node:v1.15.7@sha256:e2df133f80ef633c53c0200114fce2ed5e1f6947477dbc83261a6a921169488d",
		"1.14": "kindest/node:v1.14.10@sha256:81ae5a3237c779efc4dda43cc81c696f88a194abcc4f8fa34f86cf674aa14977",
		"1.13": "kindest/node:v1.13.12@sha256:5e8ae1a4e39f3d151d420ef912e18368745a2ede6d20ea87506920cd947a7e3a",
		"1.12": "kindest/node:v1.12.10@sha256:68a6581f64b54994b824708286fafc37f1227b7b54cbb8865182ce1e036ed1cc",
		"1.11": "kindest/node:v1.11.10@sha256:e6f3dade95b7cb74081c5b9f3291aaaa6026a90a977e0b990778b6adc9ea6248",
	},
}
