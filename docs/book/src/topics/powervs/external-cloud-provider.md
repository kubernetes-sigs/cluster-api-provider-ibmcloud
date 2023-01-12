# IBM Power VS External Cloud Provider
> ⚠️ **WARNING**: This feature is currently in experimental stage

## Steps

- To deploy a Power VS workload cluster with IBM Power VS external [cloud provider](https://kubernetes.io/docs/concepts/architecture/cloud-controller/), create a cluster configuration with the [external cloud provider template](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/templates/cluster-template-powervs-cloud-provider.yaml)
- The [external cloud provider template](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/templates/cluster-template-powervs-cloud-provider.yaml) will use [clusterresourceset](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-resource-set.html) and will create the necessary config map, secret and roles to run the cloud controller manager
- As a prerequisite set the `powervs-provider-id-fmt` [flag](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/64c9e1d17f1733c721f45a559edba3f4b712bcb0/main.go#L220) with value v2

### Deploy Power VS cluster with IBM Power VS cloud provider

  ```
  IBMPOWERVS_SSHKEY_NAME="my-pub-key" \
  IBMPOWERVS_VIP="192.168.151.22" \
  IBMPOWERVS_VIP_EXTERNAL="158.175.162.22" \
  IBMPOWERVS_VIP_CIDR="29" \
  IBMPOWERVS_IMAGE_NAME="capibm-powervs-centos-8-1-22-4" \
  IBMPOWERVS_SERVICE_INSTANCE_ID="7845d372-d4e1-46b8-91fc-41051c984601" \
  IBMPOWERVS_NETWORK_NAME="capi-test-3" \
  IBMACCOUNT_ID="ibm-accountid" \
  IBMPOWERVS_REGION="powervs-region" \
  IBMPOWERVS_ZONE="powervs-zone" \
  BASE64_API_KEY=$(echo -n $IBMCLOUD_API_KEY | base64) \
  clusterctl generate cluster ibm-powervs-1 --kubernetes-version v1.22.4 \
  --target-namespace default \
  --control-plane-machine-count=3 \
  --worker-machine-count=1 \
  --flavor=powervs-cloud-provider | kubectl apply -f -
  ```

When the cluster is created with above parameters, The IBM Power VS cloud provider will 
1. Initialize the node by fetching appropriate VM information such as IP, zone, region from Power Cloud.