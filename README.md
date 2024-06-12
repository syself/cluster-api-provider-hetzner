<img align="center" src="https://cdn.syself.com/caph.png" alt="Kubernetes Cluster API Provider Hetzner">

<br>

<div align="center">
<a href="docs/topics/quickstart.md">Quickstart</a> |
<a href="docs/README.md">Docs</a> |
<a href="docs/developers/development.md">Contribution Guide</a><br><br>
<a href="https://cluster-api.sigs.k8s.io/">Cluster API Book</a>
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

The Kubernetes Cluster API Provider Hetzner (CAPH) enables declarative provisioning of multiple Kubernetes clusters on [Hetzner infrastructure](https://hetzner.cloud).

With CAPH, you can manage highly-available Kubernetes clusters on both bare metal and cloud instances, leveraging the Cluster API to handle creation, updates, and operations of production-ready, self-managed Kubernetes clusters at any scale.

> [!NOTE]
> The Cluster API Provider Hetzner is independently maintained by [Syself](https://syself.com) and the community. It is not an official Hetzner project.
>
> If you have any questions about this project, please start a conversation in the [Discussions](https://github.com/syself/cluster-api-provider-hetzner/discussions) tab or contact us at [contact@syself.com](mailto:contact@syself.com?subject=cluster-api-provider-hetzner).

## 📰 What is the Cluster API Provider Hetzner?

The [Cluster API][cluster_api] orchestrates infrastructure similarly to how Kubernetes manages containers. It implements a declarative API like Kubernetes does and extends the resources of the Kubernetes API server via CRDs.

The Cluster API consists of the CAPI controller, the control-plane provider, the bootstrap provider, and an infrastructure provider like CAPH, that translates resources in Hetzner to objects in the Kubernetes API.

The controllers ensure that the desired state of the infrastructure is achieved - just as Kubernetes ensures the desired state of containers. The concept of [Kubernetes Controller](https://kubernetes.io/docs/concepts/architecture/controller/) has significant advantages over traditional Infrastructure as Code (IaC) solutions because it can react automatically to changes and problems. The best example of this is the MachineHealthCheck, which replaces unhealthy nodes automatically.

Using CAPH unites the benefits of declarative infrastructure, cost-effectiveness, and GDPR-compliant European cloud, ensuring that your clusters can automatically adapt to changes and problems.

## ✨ Features of CAPH

- Native Kubernetes resources and API
- Works with your choice of Linux distribution
- Support for single and multi-node control plane clusters (HA Kubernetes)
- Support for Hetzner Cloud placement groups, network, and load balancer
- Complete day 2 operations - updating Kubernetes and nodes, scaling up and down, self-healing
- Custom CSR approver for approving [kubelet-serving certificate signing requests](https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-certs/#kubelet-serving-certs)
- Hetzner dedicated servers / bare metal (and GPUs)

## 👀 Clarifying Scope

Managing a production-grade Kubernetes system requires a **dedicated team of experts**.

The Cluster API Provider Hetzner (CAPH) handles the lifecycle management of machines and infrastructure, but certain aspects need to be managed separately:

- ❌ Production-ready node images
- ❌ Secured kubeadm configuration
- ❌ Incorporation of cluster add-ons, such as CNI (e.g. cilium), metrics-server, konnectivity-service, etc.
- ❌ Testing & update procedures of Kubernetes version, configuration
- ❌ Backup procedures
- ❌ Monitoring strategies
- ❌ Alerting systems
- ❌ Identity and Access Management (IAM)

If you don't have a dedicated team for managing Kubernetes, you can use [Syself Autopilot](https://syself.com) and enjoy a wide range of benefits, including:

- ✅ Consistent, regular updates that provide you with the latest features and improvements.
- ✅ Highly optimized defaults, reducing costs by up to 80% without performance impacts.
- ✅ Production-ready clusters working out of the box.
- ✅ Specialized expertise in Cluster API and Hetzner for quick issue resolution and 24/7 support.

## 🚀 Get Started

Ready to dive in? Here are some resources to get you started:

- [**Cluster API Provider Hetzner 15 Minute Tutorial**](docs/topics/quickstart.md): Set up a bootstrap cluster using Kind and deploy a Kubernetes cluster on Hetzner.
- [**Develop and test Kubernetes clusters with Tilt**](docs/developers/development.md): Start using Tilt for rapid testing of various cluster flavors, like with/without a private network or bare metal.
- [**Develop and test your own node-images**](docs/topics/node-image.md): Learn how to use your own machine images for production systems.

In addition to the pure creation and operation of Kubernetes clusters, this provider can also validate and approve certificate signing requests. This increases security as the kubelets of the nodes can be operated with signed certificates, and enables the metrics-server to run securely. [Click here](docs/topics/advanced-caph.md#csr-controller) to read more about the CSR controller.

## 🖇️ Compatibility with Cluster API and Kubernetes Versions

This provider's versions are compatible with the following versions of Cluster API:

|                                   | Cluster API `v1beta1` (`v1.6.x`) | Cluster API `v1beta1` (`v1.7.x`) |
| --------------------------------- | -------------------------------- | -------------------------------- |
| Hetzner Provider `v1.0.0-beta.33` | ✅                              | ❌                               |
| Hetzner Provider `v1.0.0-beta.34-35` | ❌                              | ✅                               |

This provider's versions can install and manage the following versions of Kubernetes:

|                   | Hetzner Provider `v1.0.x` |
| ----------------- | ------------------------- |
| Kubernetes 1.23.x | ✅                       |
| Kubernetes 1.24.x | ✅                       |
| Kubernetes 1.25.x | ✅                       |
| Kubernetes 1.26.x | ✅                       |
| Kubernetes 1.27.x | ✅                       |
| Kubernetes 1.28.x | ✅                       |
| Kubernetes 1.29.x | ✅                       |
| Kubernetes 1.30.x | ✅                       |

Test status:

- ✅ tested
- ❔ should work, but we weren't able to test it

Each version of Cluster API for Hetzner will attempt to support at least two Kubernetes versions.

**NOTE:** As the versioning for this project is tied to the versioning of Cluster API, future modifications to this policy may be made to more closely align with other providers in the Cluster API ecosystem.

---

## 💿 Operating System Images

> [!NOTE]
> Cluster API Provider Hetzner relies on a few prerequisites that must be already installed in the operating system images, such as a container runtime, kubelet, and Kubeadm.
>
> Reference images are available in kubernetes-sigs/image-builder and [templates/node-image](templates/node-image).
>
> If pre-installation of these prerequisites isn't possible, [custom scripts can be deployed](docs/topics/node-image through the Kubeadm config.md).

---

## 📖 Documentation

Documentation can be found in the `/docs` directory. [Here](docs/README.md) is an overview of our documentation.

## 👥 Getting Involved and Contributing

We, maintainers and the community, welcome any contributions to Cluster API Provider Hetzner. For suggestions, contributions, and assistance, contact the maintainers anytime.

To set up your environment, refer to the [development guide](docs/developers/development.md).

For new contributors, check out issues tagged as [`good first issue`][good_first_issue]. These are typically smaller in scope and great for getting familiar with the codebase.

We encourage **all** active community participants to act as if they were maintainers, even without "official" write permissions. This is a community effort serving the Kubernetes community.

If you have an active interest and you want to get involved, you have real power! Don't assume that the only people who can get things done around here are the "maintainers".

We would also love to add more "official" maintainers, so show us what you can
do!

## ⚖️ Code of Conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

## :shipit: GitHub Issues

### 🐛 Bugs

If you think you have found a bug, please follow these steps:

- Take some time to give due diligence to the issue tracker. Your issue might be a duplicate.
- Get the logs from the cluster controllers and paste them in your issue.
- Open a [bug report][bug_report].
- Give it a meaningful title to help others who might be searching for your issue in the future.
- For questions, reach out to the Cluster API community on the [Kubernetes Slack channel][slack_info].

### 🌟 Tracking New Features

We also use the issue tracker to track features. If you have an idea for a feature or think that you can help Cluster API Provider Hetzner become even more awesome, then follow these steps:

- Open a [feature request][feature_request].
- Give it a meaningful title to help others who might be searching for your issue in the future.
- Clearly define the use case with concrete examples, e.g. "I type `this` and Cluster API Provider Hetzner does `that`".
- Some of our larger features will require some design. If you would like to include a technical design for your feature, please include it in the issue.
- Once the new feature is well understood and the design is agreed upon, we can start coding. We would love for you to take part in this process, so we encourage you to take the lead and start coding it yourself. Please open a **WIP** _(work in progress)_ pull request. Happy coding!

## 📃 License

Published under the [Apache](https://github.com/syself/cluster-api-provider-hetzner/blob/main/LICENSE) license.

<!-- References -->

[good_first_issue]: https://github.com/syself/cluster-api-provider-hetzner/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc+label%3A%22good+first+issue%22
[bug_report]: https://github.com/syself/cluster-api-provider-hetzner/issues/new?template=bug_report.md
[feature_request]: https://github.com/syself/cluster-api-provider-hetzner/issues/new?template=feature_request.md
[slack_info]: https://github.com/kubernetes/community/tree/master/communication#slack
[cluster_api]: https://github.com/kubernetes-sigs/cluster-api
[quickstart]: https://cluster-api.sigs.k8s.io/user/quick-start.html
