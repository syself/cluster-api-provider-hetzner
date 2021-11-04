# Contributing Guidelines
<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Contributing Guidelines](#contributing-guidelines)
  - [Finding Things That Need Help](#finding-things-that-need-help)
  - [Branches](#branches)
  - [Contributing a Patch](#contributing-a-patch)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

Read the following guide if you're interested in contributing to cluster-api-provider-hetzner.

## Finding Things That Need Help

If you're new to the project and want to help, but don't know where to start, we have a semi-curated list of issues that should not need deep knowledge of the system. [Have a look and see if anything sounds interesting](https://github.com/syself/cluster-api-provider-hetzner/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22). 
Before starting to work on the issue, make sure that it doesn't have a [lifecycle/active](https://github.com/kubernetes-sigs/cluster-api/labels/lifecycle%2Factive) label. If the issue has been assigned, reach out to the assignee.
Alternatively, read some of the docs on other controllers and try to write your own, file and fix any/all issues that come up, including gaps in documentation!

If you're a more experienced contributor, looking at unassigned issues in the next release milestone is a good way to find work that has been prioritized. For example, if the latest minor release is `v1.0`, the next release milestone is `v1.1`.

Help and contributions are very welcome in the form of code contributions but also in helping to moderate office hours, triaging issues, fixing/investigating flaky tests, cutting releases, helping new contributors with their questions, reviewing proposals, etc.


## Branches

Cluster API has two types of branches: the *main* branch and
*release-X* branches.

The *main* branch is where development happens. All the latest and
greatest code, including breaking changes, happens on main.

The *release-X* branches contain stable, backwards compatible code. On every
major or minor release, a new branch is created. It is from these
branches that minor and patch releases are tagged. In some cases, it may
be necessary to open PRs for bugfixes directly against stable branches, but
this should generally not be the case.

## Contributing a Patch

1. If working on an issue, signal other contributors that you are actively working on it using `/lifecycle active`.
2. Fork the desired repo, develop and test your code changes.
    1. See the [Development Guide](development.md) for more instructions on setting up your environment and testing changes locally.
3. Submit a pull request.
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
    2.  All code PR must be have a title starting with one of
        - ⚠️ (`:warning:`, major or breaking changes)
        - ✨ (`:sparkles:`, feature additions)
        - 🐛 (`:bug:`, patch and bugfixes)
        - 📖 (`:book:`, documentation or proposals)
        - 🌱 (`:seedling:`, minor or other)
    3. If the PR requires additional action from users switching to a new release, include the string "action required" in the PR release-notes.
    4. All code changes must be covered by unit tests and E2E tests.
    5. All new features should come with user documentation.
4. Once the PR has been reviewed and is ready to be merged, commits should be [squashed](https://github.com/kubernetes/community/blob/master/contributors/guide/github-workflow.md#squash-commits).
    1. Ensure that commit message(s) are be meaningful and commit history is readable.

All changes must be code reviewed. Coding conventions and standards are explained in the official [developer docs](https://github.com/kubernetes/community/tree/master/contributors/devel). Expect reviewers to request that you avoid common [go style mistakes](https://github.com/golang/go/wiki/CodeReviewComments) in your PRs.

In case you want to run our E2E tests locally, please refer to [Testing](development.md#submitting-prs-and-testing) guide. An overview of our e2e-test jobs (and also all our other jobs) can be found in [Jobs](jobs.md).
