Render the template via clusterctl
==================================

> **Note:**
> Set `EXP_CLUSTER_RESOURCE_SET` to `true` as the cluster will be deployed with external cloud provider for both VPC and PowerVS, which will create the resources to run the cloud controller manager.

## VPC

```
IBMVPC_REGION=us-south-1 \
IBMVPC_ZONE=us-south-1 \
IBMVPC_RESOURCEGROUP=4f15679623607b855b1a27a67f20e1c7 \
IBMVPC_NAME=ibm-vpc-1 \
IBMVPC_IMAGE_NAME=capibm-vpc-ubuntu-2004-kube-v1-25-2 \
IBMVPC_PROFILE=bx2-4x16 \
IBMVPC_SSHKEY_NAME=capi-vpc-key \
clusterctl generate cluster ibm-vpc-1 --kubernetes-version v1.14.3 \
--target-namespace default \
--control-plane-machine-count=1 \
--worker-machine-count=2 \
--from ./cluster-template.yaml
```

## Power VS

```
IBMPOWERVS_SSHKEY_NAME="my-pub-key" \
IBMPOWERVS_VIP="192.168.167.6" \
IBMPOWERVS_VIP_EXTERNAL="163.68.65.6" \
IBMPOWERVS_VIP_CIDR="29" \
IBMPOWERVS_IMAGE_NAME="capibm-powervs-centos-streams8-1-26-2" \
IBMPOWERVS_SERVICE_INSTANCE_ID="3229a94c-af54-4212-bf60-6202b6fd0a07" \
IBMPOWERVS_NETWORK_NAME="capi-test" \
IBMACCOUNT_ID="ibm-accountid" \
IBMPOWERVS_REGION="osa" \
IBMPOWERVS_ZONE="osa21" \
BASE64_API_KEY=$(echo -n $IBMCLOUD_API_KEY | base64) \
clusterctl generate cluster ibm-powervs-1 --kubernetes-version v1.26.2 \
--target-namespace default \
--control-plane-machine-count=3 \
--worker-machine-count=1 \
--from ./cluster-template-powervs.yaml
```

### Additional parameters for modifying PowerVS Control-Plane spec
```
IBMPOWERVS_CONTROL_PLANE_MEMORY
IBMPOWERVS_CONTROL_PLANE_PROCESSORS
IBMPOWERVS_CONTROL_PLANE_SYSTYPE
IBMPOWERVS_CONTROL_PLANE_PROCTYPE
```

### Additional parameters for modifying PowerVS Compute node spec
```
IBMPOWERVS_COMPUTE_MEMORY
IBMPOWERVS_COMPUTE_PROCESSORS
IBMPOWERVS_COMPUTE_SYSTYPE
IBMPOWERVS_COMPUTE_PROCTYPE
```

### Additional parameters for modifying PowerVS Cluster API server port
```
API_SERVER_PORT
```
