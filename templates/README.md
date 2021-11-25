Render the template via clusterctl
==================================

## VPC

```
IBMVPC_REGION=us-south-1 \
IBMVPC_ZONE=us-south-1 \
IBMVPC_RESOURCEGROUP=4f15679623607b855b1a27a67f20e1c7 \
IBMVPC_NAME=ibm-vpc-1 \
IBMVPC_IMAGE_ID=r134-ea84bbec-7986-4ff5-8489-d9ec34611dd4 \
IBMVPC_PROFILE=bx2-4x16 \
IBMVPC_SSHKEY_ID=r134-2a82b725-e570-43d3-8b23-9539e8641944 \
clusterctl generate cluster ibm-vpc-1 --kubernetes-version v1.14.3 \
--target-namespace default \
--control-plane-machine-count=1 \
--worker-machine-count=2 \
--from ~/.cluster-api/dev-repository/infrastructure-ibmvpccloud/v0.3.8/cluster-template.yaml
```

## Power VS

```shell
IBMPOWERVS_SSHKEY_NAME="mkumatag-pub-key" \
IBMPOWERVS_VIP="192.168.150.125" \
IBMPOWERVS_VIP_EXTERNAL="158.175.161.125" \
IBMPOWERVS_VIP_CIDR="29" \
IBMPOWERVS_IMAGE_NAME="capibm-powervs-centos-8-1-22-4" \
IBMPOWERVS_SERVICE_INSTANCE_ID="e449d86e-c3a0-4c07-959e-8557fdf55482" \
IBMPOWERVS_NETWORK_NAME="capi-test-3" \
clusterctl generate cluster ibm-powervs-1 --kubernetes-version v1.21.2 \
--target-namespace default \
--control-plane-machine-count=3 \
--worker-machine-count=1 \
--from ./cluster-template-powervs.yaml
```