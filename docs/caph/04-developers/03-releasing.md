---
title: Release Process
metatitle: Cluster API Provider Hetzner Release Process
sidebar: Release Process
description: Documentation on the CAPH release process, describing the necessary steps and how to version.
---

## Create a tag

1. Create an annotated tag
   - `git switch main`
   - `git pull`
   - Have a look at the current (old) version: [Github Releases](https://github.com/syself/cluster-api-provider-hetzner/releases)
   - `export RELEASE_TAG=<the tag of the release to be cut>` (eg. `export RELEASE_TAG=v1.0.1`)
   - `git tag -a ${RELEASE_TAG} -m ${RELEASE_TAG}`
2. Push the tag to the GitHub repository.
   {% callout %}

   `origin` should be the name of the remote pointing to `github.com/syself/cluster-api-provider-hetzner`

   {% /callout %}

   - `git push origin ${RELEASE_TAG}`
   - This will automatically trigger a [Github Action](https://github.com/syself/cluster-api-provider-hetzner/actions) to create a draft release (this will take roughly 6 minutes).

## Release in GitHub

1. Review the draft release on GitHub: [Releases](https://github.com/syself/cluster-api-provider-hetzner/releases). Use the pencil-icon to edit the draft release. Then use the button "Generate release notes". Pay close attention to the `## :question: Sort these by hand` section, as it contains items that need to be manually sorted. Feel free to move less important PRs, like version upgrades (from renovate bot), to the bottom.
1. Double checkt that the assets got created. There should be one zip file, one tgz file, and 12 yaml files.
1. If it is pre-release, activate the corresponding check at the bottom of the page. And add `:rotating_light: This is a RELEASE CANDIDATE. If you find any bugs, file an [issue](https://github.com/syself/cluster-api-provider-hetzner/issues/new).` at the top of the release notes.
1. Before publishing you can check the [Recent tagged image versions](https://github.com/syself/cluster-api-provider-hetzner/pkgs/container/caph): "latest" should be some seconds old and the new version number.
1. Publish the release
1. Write to the corresponding channels: "FYI: .... was released, (add hyperlink). A big "thank you" to all contributors!"

Done 🥳

## Manual creation of images

This is only needed if you want to manually release images.

1. Login to ghcr
2. Execute `make release-image`

## Versioning

See the [versioning documentation](https://github.com/syself/cluster-api-provider-hetzner/blob/main/CONTRIBUTING.md#versioning) for more information.

## Permissions

Releasing requires a particular set of permissions.

- Tag push access to the GitHub repository
- GitHub Release creation access
