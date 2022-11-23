## Provision workload Cluster with Load Balancer in IBM Cloud VPC
### This feature is currently in experimental stage

### Deploy VPC cluster with Load Balancer

```
    IBMVPC_REGION=us-south \
    IBMVPC_ZONE=us-south-1 \
    IBMVPC_RESOURCEGROUP=4f15679623607b855b1a27a67f20e1c7 \
    IBMVPC_NAME=ibm-vpc-0 \
    IBMVPC_IMAGE_NAME=capibm-vpc-ubuntu-2004-kube-v1-25-2 \
    IBMVPC_PROFILE=bx2-4x16 \
    IBMVPC_SSHKEY_NAME=capibm-vpc-pub-key \
    clusterctl generate cluster ibm-vpc-0 --kubernetes-version v1.22.0 \
    --target-namespace default \
    --control-plane-machine-count=3 \
    --worker-machine-count=1 \
    --flavor=load-balancer | kubectl apply -f -
```
