# Release Process

## Alpha/Beta releases
- Create a tag and push
    ```shell
    git clone git@github.com:kubernetes-sigs/cluster-api-provider-ibmcloud.git
    git tag -s -m "v0.2.0-alpha.3" v0.2.0-alpha.3
    git push origin v0.2.0-alpha.3
    ```
- Wait for the google cloud build to be finished 
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
