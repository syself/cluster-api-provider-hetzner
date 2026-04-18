[![Kubernetes Cluster API Provider Hetzner](https://cdn.syself.com/caph.png)](https://syself.com)

<br>

<div align="center">
<a href="https://syself.com/docs/caph/getting-started/quickstart/prerequisites">Quickstart</a> |
<a href="https://syself.com/docs/caph/getting-started/introduction">Docs</a> |
<a href="https://cluster-api.sigs.k8s.io/">Cluster API Book</a><br><br>
<p>⭐ Consider leaving a star — it motivates us a lot! ⭐</p>
</div>

---

<div align="center">
<a href="https://github.com/syself/cluster-api-provider-hetzner/releases"><img src="https://img.shields.io/github/release/syself/cluster-api-provider-hetzner/all.svg?style=flat-square" alt="GitHub release"></a>
<a href="https://pkg.go.dev/github.com/syself/cluster-api-provider-hetzner?tab=overview"><img src="https://godoc.org/github.com/syself/cluster-api-provider-hetzner?status.svg" alt="GoDoc"></a>
<a href="https://goreportcard.com/report/github.com/syself/cluster-api-provider-hetzner"><img src="https://goreportcard.com/badge/github.com/syself/cluster-api-provider-hetzner" alt="Go Report Card"></a>
<a href="https://bestpractices.coreinfrastructure.org/projects/5682"><img src="https://bestpractices.coreinfrastructure.org/projects/5682/badge" alt="CII Best Practices"></a>
<a href="https://opensource.org/license/apache-2-0/"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License"></a>
<a href="https://quay.io/repository/syself/cluster-api-provider-hetzner?tab=tags"><img src="https://img.shields.io/github/v/tag/syself/cluster-api-provider-hetzner?include_prereleases&label=quay.io" alt="Latest quay.io image tags"></a>
</div>

<br>

## Table of Contents

- [What is CAPH](#-what-is-the-cluster-api-provider-hetzner)
- [Documentation](#-documentation)
- [Getting Started](#-getting-started)
- [CLI Usage](#-cli-usage)
- [Version Compatibility](#%EF%B8%8F-compatibility-with-cluster-api-and-kubernetes-versions)
- [Node Images](#-operating-system-images)
- [Contributing](#-getting-involved-and-contributing)
- [Contact](#-contact)

## 📰 What is the Cluster API Provider Hetzner?

> [!NOTE]
> The Cluster API Provider Hetzner is independently maintained by [Syself](https://syself.com) and the community. It is not an official Hetzner project.
>
> If you have any questions about this project, please start a conversation in the [Discussions](https://github.com/syself/cluster-api-provider-hetzner/discussions) tab or contact us at [contact@syself.com](mailto:contact@syself.com?subject=cluster-api-provider-hetzner).

The Cluster API Provider Hetzner (CAPH) provides a way to declaratively create and manage infrastructure on Hetzner, in a Kubernetes-native way. It extends the Kubernetes API with Custom Resource Definitions (CRDs) allowing you to interact with clusters in the same fashion you interact with workload.

Key benefits include:

- **Self-healing**: CAPH and CAPI controllers react to every change in your infrastructure, identifying and resolving issues without human intervention
- **Declarative**: Specify the desired state of your infrastructure and let the operators do the rest, ensuring repeatability and idempotency
- **Kubernetes native**: Everything is a Kubernetes resource, meaning you can use tools you're already familiar with while working with CAPH

CAPH enables you to have DIY Kubernetes on Hetzner at any scale, with full control over your infrastructure and clusters configuration.

If you want a batteries-included solution instead, you can try [Syself](https://syself.com) free for 14 days.

## 📖 Documentation

Documentation can be found at [caph.syself.com](https://caph.syself.com). You can contribute to it by modifying the contents of the `/docs` directory.

## 🚀 Getting Started

The best way to get started with CAPH is to spin up a cluster. For that you can follow our [**Managing Kubernetes on Hetzner with Cluster API**](https://community.hetzner.com/tutorials/kubernetes-on-hetzner-with-cluster-api) article featured in the Hetzner Community Tutorials.

Additional resources from the documentation:

- [**Cluster API Provider Hetzner 15 Minute Tutorial**](https://syself.com/docs/caph/getting-started/quickstart/prerequisites): Set up a bootstrap cluster using Kind and deploy a Kubernetes cluster on Hetzner.
- [**Develop and test Kubernetes clusters with Tilt**](https://syself.com/docs/caph/developers/development-guide): Start using Tilt for rapid testing of various cluster flavors, like with/without a private network or bare metal.
- [**Develop and test your own node-images**](https://syself.com/docs/caph/topics/node-image): Learn how to use your own machine images for production systems.

In addition to the pure creation and operation of Kubernetes clusters, this provider can also validate and approve certificate signing requests. This increases security as the kubelets of the nodes can be operated with signed certificates, and enables the metrics-server to run securely. [Click here](https://syself.com/docs/caph/topics/advanced/csr-controller) to read more about the CSR controller.

## 🧭 CLI Usage

The main binary prints its usage text before trying to connect to a Kubernetes cluster. The block below is updated from the binary itself.

<!-- BEGIN MAIN BINARY USAGE -->
```console
$ cluster-api-provider-hetzner --help
Usage:
  cluster-api-provider-hetzner [flags]

Flags:
      --baremetal-image-url-command string        Command to run (in rescue-system) to provision an baremetal machine. Docs: https://syself.com/docs/caph/developers/image-url-command
      --baremetal-ssh-after-install-image         Connect to the baremetal machine after install-image and ensure it is provisioned. Current default is true, but we might change that to false. Background: Users might not want the controller to be able to ssh onto the servers (default true)
      --debug-hcloud-api-calls                    Debug all calls to the hcloud API.
      --disable-csr-approval                      Disables builtin workload cluster CSR validation and approval.
      --hcloudmachine-concurrency int             Number of HcloudMachines to process simultaneously (default 1)
      --health-probe-bind-address string          The address the probe endpoint binds to. (default ":9440")
      --hetznerbaremetalhost-concurrency int      Number of HetznerBareMetalHosts to process simultaneously (default 1)
      --hetznerbaremetalmachine-concurrency int   Number of HetznerBareMetalMachines to process simultaneously (default 1)
      --hetznercluster-concurrency int            Number of HetznerClusters to process simultaneously (default 1)
      --kubeconfig string                         Paths to a kubeconfig. Only required if out-of-cluster.
      --leader-elect                              Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager. (default true)
      --leader-elect-namespace string             Namespace that the controller performs leader election in. If unspecified, the controller will discover which namespace it is running in.
      --log-level string                          Specifies log level. Options are 'debug', 'info' and 'error' (default "info")
      --metrics-bind-address string               The address the metric endpoint binds to. (default "localhost:8080")
      --namespace string                          Namespace that the controller watches to reconcile cluster-api objects. If unspecified, the controller watches for cluster-api objects across all namespaces.
      --pre-provision-command string              Command to run (in rescue-system) before installing the image on bare metal servers. You can use that to check if the machine is healthy before installing the image. If the exit value is non-zero, the machine is considered unhealthy. This command must be accessible by the controller pod. You can use an initContainer to copy the command to a shared emptyDir.
      --rate-limit duration                       The rate limiting for HCloud controller (e.g. 5m) (default 5m0s)
      --skip-crd-migration-phases strings         List of CRD migration phases to skip. Valid values are: StorageVersionMigration, CleanupManagedFields.
      --skip-webhooks                             Skip setting up webhooks. Together with --leader-elect=false, this lets you run CAPH in a cluster connected via KUBECONFIG. You should scale down the deployed CAPH controller to 0 before doing that. This is only for testing.
      --sync-period duration                      The minimum interval at which watched resources are reconciled (e.g. 3m) (default 3m0s)
      --watch-filter string                       Label value that the controller watches to reconcile cluster-api objects. Label key is always cluster.x-k8s.io/watch-filter. If unspecified, the controller watches for all cluster-api objects.
```
<!-- END MAIN BINARY USAGE -->

## 🖇️ Compatibility with Cluster API and Kubernetes Versions

This provider's versions are compatible with the following versions of Cluster API:

|                                      | CAPI `v1beta1` (`v1.8.x`) | CAPI `v1beta1` (`v1.9.x`) | CAPI `v1beta1` (`v1.10.x`) |
| ------------------------------------ | ------------------------- | ------------------------- | -------------------------- |
| Hetzner Provider `v1.0.x`            | ✅                        | ✅                        | ✅                         |


This provider's versions can install and manage the following versions of Kubernetes:

|                   | Hetzner Provider `v1.0.x` |
| ----------------- | ------------------------- |
| Kubernetes 1.31.x | ✅                        |
| Kubernetes 1.32.x | ✅                        |
| Kubernetes 1.33.x | ✅                        |
| Kubernetes 1.34.x | ❔                        |

Test status:

- ✅ tested
- ❔ should work, but we weren't able to test it

Each version of Cluster API for Hetzner will attempt to support at least two Kubernetes versions.

**NOTE:** As the versioning for this project is tied to the versioning of Cluster API, future modifications to this policy may be made to more closely align with other providers in the Cluster API ecosystem.

## 💿 Operating System Images

Cluster API Provider Hetzner relies on a few prerequisites that must be already installed in the operating system images, such as a container runtime, kubelet, and kubeadm.

Reference images are available in kubernetes-sigs/image-builder.

If it's not possible to pre-install these prerequisites, [custom scripts can be deployed](/docs/caph/02-topics/03-node-image.md) through the kubeadm config.

In case you want a solution with managed node images, [Syself](https://syself.com) might be interesting for you.

## 🤝 Getting Involved and Contributing

We, the maintainers and the community, welcome any contributions to Cluster API Provider Hetzner. Feel free to contact the maintainers for suggestions, contributions and help.

To set up your environment, refer to the [development guide](https://syself.com/docs/caph/developers/development-guide).

For new contributors, check out issues tagged as [`good first issue`][good_first_issue]. These are typically smaller in scope and great for getting familiar with the codebase.

We encourage **all** active community members to act as if they were maintainers, even without "official" write permissions. This is a collaborative effort serving the Kubernetes community.

If you have an active interest and you want to get involved, you have real power! Don't assume that the only people who can get things done around here are the "maintainers".

We would also love to add more "official" maintainers, so show us what you can do!

### ⚖️ Code of Conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](/code-of-conduct.md).

### :shipit: GitHub Issues

#### 🐛 Bugs

If you think you have found a bug, please follow these steps:

- Take some time to give due diligence to the issue tracker. Your issue might be a duplicate.
- Get the logs from the cluster controllers and paste them in your issue.
- Open a [bug report][bug_report].
- Give it a meaningful title to help others who might be searching for your issue in the future.
- For questions, reach out to the Cluster API community on the [Kubernetes Slack channel][slack_info].

#### 🌟 Tracking New Features

We also use the issue tracker to track features. If you have an idea for a feature or think that you can help Cluster API Provider Hetzner become even more awesome, then follow these steps:

- Open a [feature request][feature_request].
- Give it a meaningful title to help others who might be searching for your issue in the future.
- Clearly define the use case with concrete examples, e.g. "I type `this` and Cluster API Provider Hetzner does `that`".
- Some of our larger features will require some design. If you would like to include a technical design for your feature, please include it in the issue.
- Once the new feature is well understood and the design is agreed upon, we can start coding. We would love for you to take part in this process, so we encourage you to take the lead and start coding it yourself. Please open a **WIP** _(work in progress)_ pull request. Happy coding!

## 💬 Contact

For more information about Syself, our platform, or any generall information about the Cluster API Provider Hetzner, feel free to reach out to us. Below are some ways to contact our team:

- **Email**: Send us questions at <contact@syself.com>
- **Website**: Visit [our website](https://syself.com) for more information about Syself
- **LinkedIn**: Follow us on [LinkedIn](https://www.linkedin.com/company/syself/) for announcements
- **Newsletter**: Consider subscribing to [our LinkedIn newsletter](https://www.linkedin.com/newsletters/the-syselfer-7223788357485543424/) for regular news about CAPH

[![Kubernetes Cluster API Provider Hetzner](https://cdn.syself.com/caph-alt.png)](https://syself.com/demo)

<!-- References -->

[good_first_issue]: https://github.com/syself/cluster-api-provider-hetzner/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc+label%3A%22good+first+issue%22
[bug_report]: https://github.com/syself/cluster-api-provider-hetzner/issues/new?template=bug_report.md
[feature_request]: https://github.com/syself/cluster-api-provider-hetzner/issues/new?template=feature_request.md
[slack_info]: https://github.com/kubernetes/community/tree/master/communication#slack
