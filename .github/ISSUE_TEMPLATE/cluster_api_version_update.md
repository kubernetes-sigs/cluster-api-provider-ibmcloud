---
name: Cluster API version update
about: Create an issue to track tasks for a Cluster API version update
title: Bump cluster-api to v<>

---

/area provider/ibmcloud

## Tasks for Cluster API major version update

Update cluster-api version
- [ ] [go.mod](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/go.mod)
- [ ] [hack/tools/go.mod](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/hack/tools/go.mod)
- [ ] [E2E config files](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/tree/main/test/e2e/config)
- [ ] [test/e2e/data/metadata.yaml](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/test/e2e/data/shared/metadata.yaml)
- [ ] run `make generate` to update the CRDs


Update Kubernetes version
- [ ] [go.mod](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/go.mod)
- [ ] [Kubebuilder version](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/Makefile#L84)
- [ ] [scripts](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/fetch_ext_bins.sh#L29)


If Go version is bumped, update it in the following files
- [ ] [go.mod](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/go.mod)
- [ ] [hack/tools/go.mod](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/hack/tools/go.mod)
- [ ] [ .golangci.yaml](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/.golangci.yml)
- [ ] [hack/ensure-go.sh](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/hack/ensure-go.sh)
- [ ] [netlify.toml](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/netlify.toml)
- [ ] [Makefile](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/Makefile#L66)
- [ ] [hack/ccm/Makefile](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/hack/ccm/Makefile#L16)
- [ ] [Update gcb-docker-gcloud image](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/cloudbuild.yaml#L7)

Previous PR: https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/pull/2069

## Tasks for Cluster API minor version update

Update cluster-api version
- [ ] [go.mod](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/go.mod)
- [ ] [hack/tools/go.mod](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/hack/tools/go.mod)
- [ ] [E2E config files](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/tree/main/test/e2e/config)


**Notes**:
* With every Cluster API release, update the version in the last two CAPIBM release branches also.
* Update the e2e CI to use machine images with corresponding kubernetes version with every Cluster API major version release and update the e2e files accordingly.