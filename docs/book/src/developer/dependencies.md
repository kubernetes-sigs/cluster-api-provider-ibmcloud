# Modules and tools dependencies

| Package | Module name | Used by | 
| --- | ----------- | ------- |
| cluster-api | [sigs.k8s.io/cluster-api](https://github.com/kubernetes-sigs/cluster-api) | [go.mod][go.mod1] |
| cluster-api/test | [sigs.k8s.io/cluster-api/test](https://github.com/kubernetes-sigs/cluster-api/tree/main/test) | [go.mod][go.mod1]  |
| cluster-api/hack/tools | [sigs.k8s.io/cluster-api/hack/tools](https://github.com/kubernetes-sigs/cluster-api/tree/main/hack/tools) | [hack/tool/go.mod][go.mod2] |



#### Tools used by E2E tests.

| Package | Used by | GitHub |
| --- | ----------- | ------ |
| IBM Cloud CLI | [ci-e2e.sh](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/scripts/ci-e2e.sh) | [ibm-cloud-cli-release](https://github.com/IBM-Cloud/ibm-cloud-cli-release.git) |
| pvsadm | [ci-e2e.sh](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/scripts/ci-e2e.sh) | [pvsadm](https://github.com/ppc64le-cloud/pvsadm.git) |


[go.mod1]: https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/go.mod
[go.mod2]: https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/hack/tools/go.mod