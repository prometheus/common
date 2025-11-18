# Releases

## What to do know before cutting a release

While `prometheus/common` does not have a formal release process. We strongly encourage you follow these steps:

1. Scan the list of available issues / PRs and make sure that You attempt to merge any pull requests that appear to be ready or almost ready
2. Notify the maintainers listed as part of [`MANTAINERS.md`](MAINTAINERS.md) that you're going to do a release.

With those steps done, you can proceed to cut a release.

## How to cut an individual release

There is no automated process for cutting a release in `prometheus/common`.
The primary trigger for announcing a release is pushing a new version tag.

NOTE: As soon as a new tag is created, many downstream projects will automatically create pull requests to update their dependency of prom/common.
Make sure the release is ready to go, with an updated changelog including notices of any breaking changes, before pushing a new tag.

Here are the basic steps:

1. Update CHANGELOG.md, applying the new version number (this time including the `v` prefix, e.g. `v0.53.0`) and date to the changes listed under ``## main / unreleased`, and commit those changes to the main branch.
2. Use GitHub's release feature via [this link](https://github.com/prometheus/prometheus/releases/new) to apply a new tag. The tag name must be prefixed with a `v` e.g. `v0.53.0` and then use the "Generate release notes" button to generate the release notes automagically ✨. No need to create a discussion or mark it a pre-release, please do make sure it is marked as the latest release.

## Versioning strategy

We aim to adhere to [Semantic Versioning](https://semver.org/) as much as possible. For example, patch version (e.g. v0.0.x) releases should contain bugfixes only and any sort of major or minor version bump should be a minor or major release respectively.
