# Release Process

## Alpha/Beta releases
- Create a tag and push
    ```shell
    git clone git@github.com:kubernetes-sigs/cluster-api-provider-ibmcloud.git
    git tag -s -m "v0.2.0-alpha.3" v0.2.0-alpha.3
    git push origin v0.2.0-alpha.3
    ```
- Wait for the google cloud build to be finished 
- [Prepare release notes](#prepare-release-notes)
- Create a draft release with release notes for the tag
- Tick the prerelease checkbox
- Download the artifacts once cloud build is finished
     ```shell
    gsutil -m cp \
      "gs://artifacts.k8s-staging-capi-ibmcloud.appspot.com/components/v0.2.0-alpha.3/cluster-template-powervs.yaml" \
      "gs://artifacts.k8s-staging-capi-ibmcloud.appspot.com/components/v0.2.0-alpha.3/cluster-template.yaml" \
      "gs://artifacts.k8s-staging-capi-ibmcloud.appspot.com/components/v0.2.0-alpha.3/infrastructure-components.yaml" \
      "gs://artifacts.k8s-staging-capi-ibmcloud.appspot.com/components/v0.2.0-alpha.3/metadata.yaml" \
      .
    ```
- Upload the downloaded artifacts into the release asset
- Publish the drafted release
> Note: In the above instructions, `v0.2.0-alpha.3` is the version/tag is being released

## GA Releases
- Create a tag and push
    ```shell
    git clone git@github.com:kubernetes-sigs/cluster-api-provider-ibmcloud.git
    git tag -s -m "v0.1.0" v0.1.0
    git push origin v0.1.0
    ```
- Wait for the google cloud build to be finished
- [Prepare release notes](#prepare-release-notes)
- Create a draft release with release notes for the tag
- Perform the [image promotion process](https://github.com/kubernetes/k8s.io/tree/main/k8s.gcr.io#image-promoter):
  - Clone and pull down the latest from [kubernetes/k8s.io](https://github.com/kubernetes/k8s.io)
  - Create a new branch in your fork of `kubernetes/k8s.io`. 
  - The staging repository is [here](https://console.cloud.google.com/gcr/images/k8s-staging-capi-ibmcloud/GLOBAL).
  - Once image is present in the above staging repository, find the sha256 tag for the image by following instructions
  ```shell
  $ manifest-tool inspect --raw gcr.io/k8s-staging-capi-ibmcloud/cluster-api-ibmcloud-controller:v0.1.0 | jq '.[0].Digest'
  "sha256:6c92a6a337ca5152eda855ac27c9e4ca1f30bba0aa4de5c3a0b937270ead4363"
  ```
  - In your `kubernetes/k8s.io` branch edit `k8s.gcr.io/images/k8s-staging-capi-ibmcloud/images.yaml` and add an entry for the version using the sha256 value got from the above command. For example: `"sha256:6c92a6a337ca5152eda855ac27c9e4ca1f30bba0aa4de5c3a0b937270ead4363": ["v0.1.0"]`
  - You can use [this PR](https://github.com/kubernetes/k8s.io/pull/3185) as example 
  - Wait for the PR to be approved and merged
  - Run `make release` command
  - Copy the content from `out` directory to release asset 
  - Publish the drafted release

> Note: In the above instructions, `v0.1.0` is the version/tag is being released

### Prepare release notes

1. If you don't have a GitHub token, create one by going to your GitHub settings, in [Personal access tokens](https://github.com/settings/tokens). Make sure you give the token the `repo` scope.

2. Fetch the latest changes from upstream and check out the `main` branch:

    ```sh
    git fetch upstream
    git checkout main
    ```

3. Generate release notes by running the following commands on the `main` branch:

    ```sh
    export GITHUB_TOKEN=<your GH token>
    export RELEASE_TAG=v1.2.3 # change this to the tag of the release to be cut
    make release-notes
    ```

4. Review the release notes file generated at `CHANGELOG/<RELEASE_TAG>.md` and make any necessary changes:

  - Move items out of "Uncategorized" into an appropriate section.
  - Change anything attributed to "k8s-cherrypick-robot" to credit the original author.
  - Fix any typos or other errors.
  - Add the following section with a link to the full diff:
      ```md
      ## The image for this release is:
      registry.k8s.io/capi-ibmcloud/cluster-api-ibmcloud-controller:<RELEASE_TAG>

      <!-- markdown-link-check-disable-next-line -->
      Full Changelog: https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/compare/v0.9.0...v0.10.0
      ```
    Be sure to replace the versions in the URL with the appropriate tags.
---
### Post release tasks: 

Create a tracker issue using the [`release`](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/.github/ISSUE_TEMPLATE/release.md) template to have a check-list that covers through all tasks that are expected to be done after the release.
