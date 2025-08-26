# Modules and tools dependencies

#### CAPI Packages
| Package | Module name | Used by |
| ------- | ----------- | ------- |
| cluster-api | [sigs.k8s.io/cluster-api](https://github.com/kubernetes-sigs/cluster-api) | [go.mod][go.mod1] |
| cluster-api/test | [sigs.k8s.io/cluster-api/test](https://github.com/kubernetes-sigs/cluster-api/tree/main/test) | [go.mod][go.mod1]  |
| cluster-api/hack/tools | [sigs.k8s.io/cluster-api/hack/tools](https://github.com/kubernetes-sigs/cluster-api/tree/main/hack/tools) | [hack/tools/go.mod][go.mod2] |

- ##### K8s Packages
| Package | Module name | Used by |
| ------- | ----------- | ------- |
| api | [api](https://github.com/kubernetes/api) | [go.mod][go.mod1] |
| apiextensions-apiserver | [apiextensions-apiserver](https://github.com/kubernetes/apiextensions-apiserver) | [go.mod][go.mod1] |
| apimachinery | [apimachinery](https://github.com/kubernetes/apimachinery) | [go.mod][go.mod1] |
| cli-runtime | [cli-runtime](https://github.com/kubernetes/cli-runtime) | [go.mod][go.mod1] |
| client-go | [client-go](https://github.com/kubernetes/client-go) | [go.mod][go.mod1] |
| utils | [utils](https://github.com/kubernetes/utils) | [go.mod][go.mod1] |
| controller-runtime | [sigs.k8s.io/controller-runtime](https://sigs.k8s.io/controller-runtime) | [go.mod][go.mod1] |
| controller-runtime/tools/setup-envtest | [sigs.k8s.io/controller-runtime/tools/setup-envtest](https://sigs.k8s.io/controller-runtime/tools/setup-envtest) | [hack/tools/go.mod][go.mod2] |
| controller-tools | [sigs.k8s.io/controller-tools](https://sigs.k8s.io/controller-tools) | [hack/tools/go.mod][go.mod2] |

- ##### Test Packages
| Package | Module name | Used by |
| ------- | ----------- | ------- |
| onsi/ginkgo/v2 | [github.com/onsi/ginkgo/v2](https://github.com/onsi/ginkgo) | [go.mod][go.mod1] [hack/tools/go.mod][go.mod2] |
| onsi/gomega | [github.com/onsi/gomega](https://github.com/onsi/gomega) | [go.mod][go.mod1] |

> Note: The K8s and Test packages are subject to updates with each new CAPI package release.

#### IBM Packages
| Package | Module name | Used by |
| ------- | ----------- | ------- |
| IBM-Cloud/power-go-client | [github.com/IBM-Cloud/power-go-client](https://github.com/IBM-Cloud/power-go-client) | [go.mod][go.mod1] |
| IBM/go-sdk-core/v5 | [github.com/IBM/go-sdk-core/v5](https://github.com/IBM/go-sdk-core) | [go.mod][go.mod1] |
| IBM/platform-services-go-sdk | [github.com/IBM/platform-services-go-sdk](https://github.com/IBM/platform-services-go-sdk) | [go.mod][go.mod1] |
| IBM/vpc-go-sdk | [github.com/IBM/vpc-go-sdk](https://github.com/IBM/vpc-go-sdk) | [go.mod][go.mod1] |

</br>

---
#### Tools used by E2E tests.

| Package | Used by | GitHub |
| --- | ----------- | ------ |
| IBM Cloud CLI | [ci-e2e.sh](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/scripts/ci-e2e.sh) | [ibm-cloud-cli-release](https://github.com/IBM-Cloud/ibm-cloud-cli-release.git) |
| capibmadm | [ci-e2e.sh](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/scripts/ci-e2e.sh) | [capibmadm](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/tree/main/cmd/capibmadm) |

#### Other Tools
| Package | Used by | Source |
| --- | ----------- | ------ |
| kind | [ensure-kind.sh](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/hack/ensure-kind.sh) | [kind](https://github.com/kubernetes-sigs/kind) |
| kubebuilder | [Makefile](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/Makefile) | [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) |

[go.mod1]: https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/go.mod
[go.mod2]: https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/hack/tools/go.mod