# E2E Testing

### Introduction

* The end-to-end tests for `VPC` and `PowerVS` run on an internal prow cluster on IBM Cloud.
* Resource management is handled via [boskos](https://github.com/kubernetes-sigs/boskos) which is an efficient way to lease infra and clean up after every run.
* The E2E tests use the Cluster API test framework. For more information on developing E2E tests, refer [here](https://cluster-api.sigs.k8s.io/developer/e2e).

### Jobs

The following periodic jobs are being run on main branch once every day.

1. [periodic-capi-provider-ibmcloud-e2e-powervs](https://prow.ppc64le-cloud.cis.ibm.net/job-history/gs/ppc64le-kubernetes/logs/periodic-capi-provider-ibmcloud-e2e-powervs)
2. [periodic-capi-provider-ibmcloud-e2e-vpc](https://prow.ppc64le-cloud.cis.ibm.net/job-history/gs/ppc64le-kubernetes/logs/periodic-capi-provider-ibmcloud-e2e-vpc)

We also test the last two releases, `release-0.5` and `release-0.6` once every week.

### Running the end-to-end tests locally

For development and debugging the E2E tests, they can be executed locally. 

1. Set the flavor you want to test. By default it is set to `powervs-md-remeditaion`.

```
export E2E_FLAVOR=<e2e-flavor>
```
2. Set the infra environment variables accrodingly based on the flavor being tested. Check the required variables for [VPC](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/scripts/ci-e2e.sh#L132-L145) and [PowerVS](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/scripts/ci-e2e.sh#L123-L130) being set in [ci-e2e.sh](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/scripts/ci-e2e.sh).
3. Run the e2e test 
```
./scripts/ci-e2e.sh
```