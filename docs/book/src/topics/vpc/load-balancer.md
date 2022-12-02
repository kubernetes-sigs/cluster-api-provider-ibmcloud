## Provision workload Cluster with Load Balancer in IBM Cloud VPC

> ⚠️ **WARNING**: This feature is currently in experimental stage

Using clusterctl, render the yaml through templates and deploy the cluster

```console
IBMVPC_REGION=us-south \
IBMVPC_ZONE=us-south-1 \
IBMVPC_RESOURCEGROUP=4f15679623607b855b1a27a67f20e1c7 \
IBMVPC_NAME=ibm-vpc-0 \
IBMVPC_IMAGE_ID=r134-ea84bbec-7986-4ff5-8489-d9ec34611dd4 \
IBMVPC_PROFILE=bx2-4x16 \
IBMVPC_SSHKEY_ID=r134-2a82b725-e570-43d3-8b23-9539e8641944 \
clusterctl generate cluster ibm-vpc-0 --kubernetes-version v1.22.0 \
--target-namespace default \
--control-plane-machine-count=3 \
--worker-machine-count=1 \
--flavor=load-balancer | kubectl apply -f -
```

**Change disk size for the boot volume**

There are two following variables for controlling the volume size for the boot disk.
- `IBMVPC_CONTROLPLANE_BOOT_VOLUME_SIZEGIB`: Size of the boot volume for the control plane nodes, default set to 20GiB
- `IBMVPC_WORKER_BOOT_VOLUME_SIZEGIB`: Size of the boot volume for the worker nodes, default set to 20GiB
> **Note**: Default value is set to 20GiB because the images published for testing are of size 20GiB(default size in the image-builder scripts as well).  