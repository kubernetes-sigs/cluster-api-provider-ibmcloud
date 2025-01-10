---
name: Release tracker
about: Create an issue to track tasks for a Cluster API version update
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
- [ ] Add new presubmit job for latest release kubernetes/test-infra for CAPIBM jobs
- [ ] Bump machine images in CI to use relevent Kubernetes version
