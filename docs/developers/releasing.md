# Release Process

## Create a tag

1. Create an annotated tag
   - `export RELEASE_TAG=<the tag of the release to be cut>` (eg. `export RELEASE_TAG=v1.0.1`)
   - `git tag -a ${RELEASE_TAG} -m ${RELEASE_TAG}`
2. Push the tag to the GitHub repository. This will automatically trigger a [Github Action](https://github.com/kubernetes-sigs/cluster-api/actions) to create a draft release.
   > NOTE: `origin` should be the name of the remote pointing to `github.com/syself/cluster-api-provider-hetzner`
   - `git push origin ${RELEASE_TAG}`
## Release in GitHub

1. Review the draft release on GitHub. Pay close attention to the `## :question: Sort these by hand` section, as it contains items that need to be manually sorted.
1. Publish the release

## Manual creation of images

1. Login to quay
2. Do:
   - `make release-image`


### Versioning

See the [versioning documentation](./../../CONTRIBUTING.md#versioning) for more information.

### Permissions

Releasing requires a particular set of permissions.

* Tag push access to the GitHub repository
* GitHub Release creation access
