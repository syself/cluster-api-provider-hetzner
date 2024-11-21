---
title: Updating Kubernetes Version
metatitle: Checklist For Kubernetes Version Update in CAPH Development
sidebar: Updating Kubernetes Version
description: All the steps needed when adding a new supported Kubernetes version to CAPH.
---

Please check the kubernetes version in the following files:

- `node-image` dependencies (cri-o, kubelet, kubeadm etc.).
- `cluster-template` (same as node-image but for cloud-init).
- quickstart Guide.
- [Supported Versions](/docs/caph/01-getting-started/01-introduction.md).
