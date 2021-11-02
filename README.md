# <img alt="capi" src="docs/pics/cluster-api.png" height="48x" /> Kubernetes Cluster API Provider Hetzner

[![Go Report Card](https://goreportcard.com/badge/syself/cluster-api-provider-hetzner)](https://goreportcard.com/report/syself/cluster-api-provider-hetzner)

<p align="center">
<img alt="hcloud" src="docs/pics/hetzner.png"/>
</p>

Kubernetes-native declarative infrastructure for [Hetzner](https://hetzner.cloud).

## What is the Cluster API Provider Hetzner

The [Cluster API][cluster_api] brings declarative, Kubernetes-style APIs to cluster creation, configuration and management.

The API itself is shared across multiple cloud providers allowing for true Hetzner
hybrid deployments of Kubernetes.

> This is no official Hetzner Project! It's maintained by the folks of the cloud-native startup Syself.

## Quick Start

Check out the [Cluster API Quick Start][quickstart] to create your first Kubernetes cluster on Hetzner using Cluster API.
Then please check out the [Quickstart Guide](docs/quickstart.md).*cooming soon*

------

## Support Policy

This provider's versions are compatible with the following versions of Cluster API:

|  | Cluster API `v1beta1` (`v1.0.x`) |
|---|---|
|Hetzner Provider `v1.0.x` | ✓ |

This provider's versions are able to install and manage the following versions of Kubernetes:

|  | Hetzner Provider `v1.0.x` |
|---|---|
| Kubernetes 1.21 | ✓ |
| Kubernetes 1.22 | ✓ |

Each version of Cluster API for Hetzner will attempt to support at least two Kubernetes versions 

**NOTE:** As the versioning for this project is tied to the versioning of Cluster API, future modifications to this policy may be made to more closely align with other providers in the Cluster API ecosystem.

------

## Documentation

Docs can be found in the `/docs` directory. Index could be found [here](docs/README.md).

## Getting involved and contributing

Are you interested in contributing to cluster-api-provider-hetzner? We, the
maintainers and community, would love your suggestions, contributions, and help!
Also, the maintainers can be contacted at any time to learn more about how to get
involved.

To set up your environment checkout the development guide.

In the interest of getting more new people involved, we tag issues with
[`good first issue`][good_first_issue].
These are typically issues that have smaller scope but are good ways to start
to get acquainted with the codebase.

We also encourage ALL active community participants to act as if they are
maintainers, even if you don't have "official" write permissions. This is a
community effort, we are here to serve the Kubernetes community. If you have an
active interest and you want to get involved, you have real power! Don't assume
that the only people who can get things done around here are the "maintainers".

We also would love to add more "official" maintainers, so show us what you can
do!

## Github issues

### Bugs

If you think you have found a bug please follow the instructions below.

- Please spend a small amount of time giving due diligence to the issue tracker. Your issue might be a duplicate.
- Get the logs from the cluster controllers. Please paste this into your issue.
- Open a [bug report][bug_report].
- Remember users might be searching for your issue in the future, so please give it a meaningful title to helps others.
- Feel free to reach out to the cluster-api community on [kubernetes slack][slack_info].

### Tracking new features

We also use the issue tracker to track features. If you have an idea for a feature, or think you can help Cluster API Provider Hetzner become even more awesome, then follow the steps below.

- Open a [feature request][feature_request].
- Remember users might be searching for your issue in the future, so please
  give it a meaningful title to helps others.
- Clearly define the use case, using concrete examples. EG: I type `this` and
  cluster-api-provider-hetzner does `that`.
- Some of our larger features will require some design. If you would like to
  include a technical design for your feature please include it in the issue.
- After the new feature is well understood, and the design agreed upon we can
  start coding the feature. We would love for you to code it. So please open
  up a **WIP** *(work in progress)* pull request, and happy coding.

<!-- References -->

[good_first_issue]: https://github.com/syself/cluster-api-provider-hetzner/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc+label%3A%22good+first+issue%22
[bug_report]: https://github.com/syself/cluster-api-provider-hetzner/issues/new?template=bug_report.md
[feature_request]: https://github.com/syself/cluster-api-provider-hetzner/issues/new?template=feature_request.md
[slack_info]: https://github.com/kubernetes/community/tree/master/communication#slack
[cluster_api]: https://github.com/kubernetes-sigs/cluster-api
[quickstart]: https://cluster-api.sigs.k8s.io/user/quick-start.html