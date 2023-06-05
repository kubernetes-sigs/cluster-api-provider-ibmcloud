Render the template via clusterctl
==================================

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
