---
name: Release tracker
about: Create an issue to track tasks to be done after CAPIBM major version release
title: Release tracker for v<>

---

/area provider/ibmcloud

**Tasks:**

After every CAPIBM major version release:
- [ ] Update Infrastructure Provider version in [metadata.yaml](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/metadata.yaml) and [e2e test config files](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/tree/main/test/e2e/config)
- [ ] [Update release branch versions for weekly security scan](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/.github/workflows/weekly-security-scan.yaml#L16)
- [ ] [Update release support data in docs](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/docs/book/src/developer/release-support-guidelines.md)
- [ ] [Update docs with reference to latest release](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/README.md#compatibility-with-cluster-api-and-kubernetes-versions)
- [ ] Update and add documentation link for new release branch in Netlify
- [ ] Update capibmadm tool to the latest version after each new release
- [ ] Add new presubmit job for latest release branch in [kubernetes/test-infra](https://github.com/kubernetes/test-infra/tree/master/config/jobs/kubernetes-sigs/cluster-api-provider-ibmcloud)
    - [ ] Update kubekins-e2e image to relevent Kubernetes version
- [ ] Add E2E CI jobs for latest release branch in [ppc64le-cloud/test-infra](https://github.com/ppc64le-cloud/test-infra/blob/master/config/jobs/periodic/cluster-api-provider-ibmcloud/test-e2e-capi-ibmcloud-periodics.yaml)
    - [ ] Bump machine images in CI to relevent Kubernetes version
    - [ ] Update kubekins-e2e image to relevent Kubernetes version

> Note:
> 1. An example for infrastructure provider version upgrade, if we cut a release for version
> 0.9.0, update the infratructure provider version to 0.10.0 on the main branch.
> 2. Keep the version upgrades in check for the main branch and the two latest releases.