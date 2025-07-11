---
name: Onboard new machine images for latest Kubernetes version
about: Create an issue to track tasks for onboarding new machine images of latest Kubernetes version 
title: Onboard new machine images for Kubernetes version v<> 

---

/area provider/ibmcloud

## Tasks

- [ ] Build images using automation in [image-builder](https://github.com/kubernetes-sigs/image-builder) repository
  - [ ] VPC
  - [ ] PowerVS
  - [ ] PowerVS with DHCP support

- Test the images
  - [ ] VPC
  - [ ] PowerVS
  - [ ] PowerVS with DHCP support

- [ ] Update [documentation](https://cluster-api-ibmcloud.sigs.k8s.io/machine-images/)

- [ ] Import the new images to VPC and PowerVS workspaces for CI

- [ ] Update Kubernetes version in E2E [config files](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/tree/main/test/e2e/config)

- [ ] Update [E2E script](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/scripts/ci-e2e.sh) with latest image names


**Notes**:
* The format of the new image name should be as follows:
  * VPC: capibm-vpc-{os-distribution}-
  {os-version}-kube-v{k8s-version}
    * ex: capibm-vpc-ubuntu-2404-kube-v1-32-3
  * PowerVS: capibm-powervs-{os-distribution}-
  {os-version}-{k8s-version}
    * ex: capibm-powervs-centos-streams9-1-32-3