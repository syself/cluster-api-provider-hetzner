# Contributing Guidelines
<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Contributing Guidelines](#contributing-guidelines)
  - [Finding Things That Need Help](#finding-things-that-need-help)
  - [Contributing a Patch](#contributing-a-patch)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

Read the following guide if you're interested in contributing to cluster-api-provider-hetzner.

## Finding Things That Need Help

If you're new to the project and want to help, but don't know where to start, we have a semi-curated list of issues that should not need deep knowledge of the system. [Have a look and see if anything sounds interesting](https://github.com/syself/cluster-api-provider-hetzner/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22).

Alternatively, read some of the docs on other controllers and try to write your own, file and fix any/all issues that come up, including gaps in documentation!

Help and contributions are very welcome in the form of code contributions but also in helping to moderate office hours, triaging issues, fixing/investigating flaky tests, cutting releases, helping new contributors with their questions, reviewing proposals, etc.


## Contributing a Patch

1. If working on an issue, signal other contributors that you are actively working on it by commenting on it. Wait for approval in form of someone assigning you to the issue.
2. Fork the desired repo, develop and test your code changes.
    1. See the [Development Guide](/docs/caph/04-developers/01-development-guide.md) for more instructions on setting up your environment and testing changes locally.
3. Submit a pull request.
    1. All code PR must be created in "draft mode". This helps other contributors by not blocking E2E tests, which cannot run in parallel. After your PR is approved, you can mark it "ready for review".
    1.  All code PR must be have a title starting with one of
        - ⚠️ (`:warning:`, major or breaking changes)
        - ✨ (`:sparkles:`, feature additions)
        - 🐛 (`:bug:`, patch and bugfixes)
        - 📖 (`:book:`, documentation or proposals)
        - 🌱 (`:seedling:`, minor or other)
    2. If the PR requires additional action from users switching to a new release, include the string "action required" in the PR release-notes.
    3. All code changes must be covered by unit tests and E2E tests.
    4. All new features should come with user documentation.
    5. Before finishing up your PR and requesting a final review, use `make verify` and `make test` to check whether your code complies with all of our standards and ensures that no unit tests fail.
4. Once the PR has been reviewed and is ready to be merged, commits should be [squashed](https://github.com/kubernetes/community/blob/master/contributors/guide/github-workflow.md#squash-commits).
    1. Ensure that commit message(s) are be meaningful and commit history is readable.

All changes must be code reviewed. Coding conventions and standards are explained in the official [developer docs](https://github.com/kubernetes/community/tree/master/contributors/devel). Expect reviewers to request that you avoid common [go style mistakes](https://github.com/golang/go/wiki/CodeReviewComments) in your PRs.

In case you want to run our E2E tests locally, please refer to [Testing](/docs/caph/04-developers/01-development-guide.md#submitting-prs-and-testing) guide. 
