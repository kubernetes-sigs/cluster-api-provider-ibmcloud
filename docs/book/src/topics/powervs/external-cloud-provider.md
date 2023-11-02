# IBM PowerVS External Cloud Provider
> ⚠️ **WARNING**: This feature is currently in experimental stage

## Steps

- To deploy a PowerVS workload cluster with IBM PowerVS external [cloud provider](https://kubernetes.io/docs/concepts/architecture/cloud-controller/), create a cluster configuration with the [external cloud provider template](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/templates/cluster-template-powervs-cloud-provider.yaml)
- The [external cloud provider template](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/templates/cluster-template-powervs-cloud-provider.yaml) will use [clusterresourceset](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-resource-set.html) and will create the necessary config map, secret and roles to run the cloud controller manager
- As a prerequisite set the `provider-id-fmt` [flag](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/5e7f80878f2252c6ab13c16102de90c784a2624d/main.go#L168-L173) with value v2

### Deploy PowerVS cluster with IBM PowerVS cloud provider

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
  --flavor=powervs-cloud-provider | kubectl apply -f -
  ```

When the cluster is created with above parameters, The IBM PowerVS cloud provider will 
1. Initialize the node by fetching appropriate VM information such as IP, zone, region from Power Cloud.