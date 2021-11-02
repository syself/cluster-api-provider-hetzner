# Contributing Guidelines
<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Contributing Guidelines](#contributing-guidelines)
  - [Finding Things That Need Help](#finding-things-that-need-help)
  - [Contributing a Patch](#contributing-a-patch)
  - [Backporting a Patch](#backporting-a-patch)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

Read the following guide if you're interested in contributing to cluster-api-provider-hetzner.

## Finding Things That Need Help

If you're new to the project and want to help, but don't know where to start, we have a semi-curated list of issues that should not need deep knowledge of the system. [Have a look and see if anything sounds interesting](https://github.com/syself/cluster-api-provider-hetzner/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22). Alternatively, read some of the docs on other controllers and try to write your own, file and fix any/all issues that come up, including gaps in documentation!

## Contributing a Patch

1. Fork the desired repo, develop and test your code changes.
    1. See the [Development Guide](development.md) for more instructions on setting up your environment and testing changes locally.
2. Submit a pull request.
    1. All PRs should be labeled with one of the following kinds
         - `/kind feature` for PRs releated to adding new features/tests
         - `/kind bug` for PRs releated to bug fixes and patches
         - `/kind api-change` for PRs releated to adding, removing, or otherwise changing an API
         - `/kind cleanup` for PRs releated to code refactoring and cleanup
         - `/kind deprecation` for PRs related to a feature/enhancement marked for deprecation.
         - `/kind design` for PRs releated to design proposals
         - `/kind documentation` for PRs releated to documentation
         - `/kind failing-test` for PRs releated to to a consistently or frequently failing test.
         - `/kind flake` for PRs related to a flaky test.
         - `/kind other` for PRs releated to updating dependencies, minor changes or other
     1. If the PR requires additional action from users switching to a new release, include the string "action required" in the PR release-notes.
     2. All code changes must be covered by unit tests and E2E tests.
     3. All new features should come with user documentation.
 1. Once the PR has been reviewed and is ready to be merged, commits should be [squashed](https://github.com/kubernetes/community/blob/master/contributors/guide/github-workflow.md#squash-commits). 
    1. Ensure that commit message(s) are be meaningful and commit history is readable.

All changes must be code reviewed. Coding conventions and standards are explained in the official [developer docs](https://github.com/kubernetes/community/tree/master/contributors/devel). Expect reviewers to request that you avoid common [go style mistakes](https://github.com/golang/go/wiki/CodeReviewComments) in your PRs.

In case you want to run our E2E tests locally, please refer to [Testing](development.md#submitting-prs-and-testing) guide. An overview of our e2e-test jobs (and also all our other jobs) can be found in [Jobs](jobs.md).

## Backporting a Patch

Cluster API ships older versions through `release-X.X` branches, usually backports are reserved to critical bug-fixes.
Some release branches might ship with both Go modules and dep (e.g. `release-0.1`), users backporting patches should always make sure
that the vendored Go modules dependencies match the Gopkg.lock and Gopkg.toml ones by running `dep ensure`

