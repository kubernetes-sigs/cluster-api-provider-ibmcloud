---
name: Onboard new machine images for latest Kubernetes version
about: Create an issue to track tasks for onboarding new machine images of latest Kubernetes version 
title: Onboard new machine images for Kubernetes version v<> 

---

/area provider/ibmcloud

## Tasks


### PowerVS Image
  - [ ] Build images using automation in [image-builder](https://github.com/kubernetes-sigs/image-builder) repository
  - [ ] Test the image using the [cluster-template-powervs.yaml](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/templates/cluster-template-powervs.yaml)
  - Bump to the new image in CI
    - [ ] Import the new images to PowerVS workspaces for CI
      - Account: `Upstream CI`
      - Resource group: `prow-upstream`
    - [ ] Update Kubernetes version in E2E [config file](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/811cf285d371b4d9fffdff027a3f3c90b17e6719/test/e2e/config/ibmcloud-e2e-powervs.yaml#L45)
    - [ ] Update [E2E script](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/811cf285d371b4d9fffdff027a3f3c90b17e6719/scripts/ci-e2e.sh#L127) with latest PowerVS image name
  - [ ] Upload the built image to the public COS bucket `power-oss-bucket`
  - [ ] Update [documentation](https://cluster-api-ibmcloud.sigs.k8s.io/machine-images/powervs#powervs-images) with the latest image details
  - [ ] Update the [documentation](https://cluster-api-ibmcloud.sigs.k8s.io/topics/powervs/creating-a-cluster) with latest image details and Kubernetes version in the cluster deployment instructions.

### PowerVS image with DHCP support for cluster deployment with infrastruction creation 
  - [ ] Build images using automation in [image-builder](https://github.com/kubernetes-sigs/image-builder) repository
  - Test the image with [cluster-template-powervs-create-infra.yaml](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/templates/cluster-template-powervs-create-infra.yaml)
  - [ ] Upload the built image to the public COS bucket `power-oss-bucket`
  - [ ] Update [documentation](https://cluster-api-ibmcloud.sigs.k8s.io/machine-images/powervs#powervs-images-with-dhcp-based-network) with the latest image details
  - [ ] Update the [documentation](https://cluster-api-ibmcloud.sigs.k8s.io/topics/powervs/creating-a-cluster) with latest image details and Kubernetes version in the cluster deployment instructions.

### VPC image
  - [ ] Build images using automation in [image-builder](https://github.com/kubernetes-sigs/image-builder) repository
  - [ ] Test the image using the [cluster-template.yaml](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/templates/cluster-template.yaml)
  - Bump to the new image in CI
    - [ ] Import the new images to VPC regions for CI
      - Account: Upstream CI
      - Resource group: prow-upstream
    - [ ] Update Kubernetes version in E2E [config file](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/test/e2e/config/ibmcloud-e2e-vpc.yaml#L45)
    - [ ] Update [E2E script](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/scripts/ci-e2e.sh#L141) with latest VPC image name
  - [ ] Upload the built image to the public COS bucket `power-oss-bucket`
  - [ ] Update [documentation](https://cluster-api-ibmcloud.sigs.k8s.io/machine-images/vpc/) with the latest image details
  - [ ] Update the [documentation](https://cluster-api-ibmcloud.sigs.k8s.io/topics/vpc/creating-a-cluster) with latest image details and Kubernetes version in the cluster deployment instructions.

**Notes**:
* The format of the new image name should be as follows:
  * VPC: capibm-vpc-{os-distribution}-
  {os-version}-kube-v{k8s-version}
    * ex: capibm-vpc-ubuntu-2404-kube-v1-32-3
  * PowerVS: capibm-powervs-{os-distribution}-
  {os-version}-{k8s-version}
    * ex: capibm-powervs-centos-streams9-1-32-3