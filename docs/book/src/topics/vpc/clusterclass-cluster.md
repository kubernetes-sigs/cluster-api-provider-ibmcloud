# Create IBM VPC Cluster Using ClusterClass

## Preface
- To deploy IBM Cloud VPC workload cluster using [ClusterClass](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/index.html), create a cluster configuration from the [clusterclass-template](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/templates/cluster-template-vpc-clusterclass.yaml).
- The [clusterclass-template](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/templates/cluster-template-vpc-clusterclass.yaml) will use [ClusterResourceSet](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-resource-set.html) and will create the necessary config map, secret and roles to run the cloud controller manager
- The flags EXP_CLUSTER_RESOURCE_SET and CLUSTER_TOPOLOGY need to be set to true.

A comprehensive list of IBM Cloud VPC Regions and Zones can be found [here](/reference/regions-zones-mapping.html)

## Deploy a cluster using IBM Cloud VPC infrastructure using ClusterClass
```shell
IBMVPC_CLUSTER_CLASS_NAME=ibmvpc-clusteclass \
IBMVPC_REGION= <IBM Cloud VPC region> \
IBMVPC_ZONE= <IBM Cloud VPC zone> \
IBMVPC_RESOURCEGROUP= <Resource Group of the associated IBM Cloud account> \
IBMVPC_NAME= <Name of VPC> \
IBMVPC_IMAGE_NAME=capibm-vpc-ubuntu-2004-kube-v1-26-2 \
IBMVPC_PROFILE=bx2-4x16 \
IBMVPC_SSHKEY_NAME= <SSH key to be used> \
BASE64_API_KEY=$(echo -n $IBMCLOUD_API_KEY | base64) \
IBMACCOUNT_ID= <IBM Account ID> \
clusterctl generate cluster ibm-mix-clusterclass --kubernetes-version v1.26.2 --target-namespace default --control-plane-machine-count=1 --worker-machine-count=2 --from=./templates/cluster-template-vpc-clusterclass.yaml | kubectl apply -f -
```
