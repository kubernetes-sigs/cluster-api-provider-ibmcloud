# Create IBM PowerVS Cluster Using ClusterClass
## Steps

- To deploy PowerVS workload cluster using [ClusterClass](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/index.html), create a cluster configuration from the [clusterclass-template](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/templates/cluster-template-simple-powervs-clusterclass.yaml)
- The PowerVS cluster will use [external cloud provider](https://kubernetes.io/docs/concepts/architecture/cloud-controller/). As a prerequisite set the `powervs-provider-id-fmt` [flag](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/64c9e1d17f1733c721f45a559edba3f4b712bcb0/main.go#L220) with value v2
- The [clusterclass-template](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/templates/cluster-template-simple-powervs-clusterclass.yaml) will use [clusterresourceset](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-resource-set.html) and will create the necessary config map, secret and roles to run the cloud controller manager

### Deploy PowerVS cluster with IBM PowerVS cloud provider

  ```
  IBMPOWERVS_CLUSTER_CLASS_NAME="powervs-cc" \
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
  --flavor=simple-powervs-clusterclass | kubectl apply -f -
  ```