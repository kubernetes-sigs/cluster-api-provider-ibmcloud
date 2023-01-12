## Provision workload Cluster with Load Balancer and external cloud provider in IBM Cloud VPC

> ⚠️ **WARNING**: This feature is currently in experimental stage

## Steps

- To deploy a VPC workload cluster with Load Balancer IBM external [cloud provider](https://kubernetes.io/docs/concepts/architecture/cloud-controller/), create a cluster configuration with the [external cloud provider template](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/templates/cluster-template-load-balancer.yaml)
- The [external cloud provider template](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/templates/cluster-template-load-balancer.yaml) will use [clusterresourceset](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-resource-set.html) and will create the necessary config map, secret and roles to run the cloud controller manager
- As a prerequisite set the `provider-id-fmt` [flag](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/ee70591709ac5ddaeed23222ccbfa78335d984a1/main.go#L183) with value v2

### Deploy VPC cluster with Load Balancer and IBM cloud provider

```console
  IBMCLOUD_API_KEY: "XXXXXXXXXXXXXXXXXX" \
  IBMVPC_REGION=us-south \
  IBMVPC_ZONE=us-south-1 \
  IBMVPC_RESOURCEGROUP_NAME="ibm-hypershift-dev" \
  IBMVPC_RESOURCEGROUP=4f15679623607b855b1a27a67f20e1c7 \
  IBMVPC_NAME=ibm-vpc-0 \
  IBMVPC_IMAGE_ID=r134-ea84bbec-7986-4ff5-8489-d9ec34611dd4 \
  IBMVPC_PROFILE=bx2-4x16 \
  IBMVPC_SSHKEY_ID=r134-2a82b725-e570-43d3-8b23-9539e8641944 \
  IBMACCOUNT_ID="ibm-accountid" \
  BASE64_API_KEY=$(echo -n $IBMCLOUD_API_KEY | base64) \
  clusterctl generate cluster ibm-vpc-0 --kubernetes-version v1.25.2 \
  --target-namespace default \
  --control-plane-machine-count=1 \
  --worker-machine-count=2 \
  --flavor=load-balancer | kubectl apply -f -
```

**Change disk size for the boot volume**

There are two following variables for controlling the volume size for the boot disk.
- `IBMVPC_CONTROLPLANE_BOOT_VOLUME_SIZEGIB`: Size of the boot volume for the control plane nodes, default set to 20GiB
- `IBMVPC_WORKER_BOOT_VOLUME_SIZEGIB`: Size of the boot volume for the worker nodes, default set to 20GiB
> **Note**: Default value is set to 20GiB because the images published for testing are of size 20GiB(default size in the image-builder scripts as well).  