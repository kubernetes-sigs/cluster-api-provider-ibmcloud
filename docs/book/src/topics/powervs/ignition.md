# Use ignition for IBM PowerVS cluster

## Steps

- Set ```powervs.cluster.x-k8s.io/create-infra:true``` annotation to IBMPowerVSCluster resource to auto create required resources.
- The cluster will be configured with IBM PowerVS external [cloud provider](https://kubernetes.io/docs/concepts/architecture/cloud-controller/)
- The [cluster-template-powervs-ignition](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/templates/cluster-template-powervs-ignition.yaml) will use [clusterresourceset](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-resource-set.html) and will create the necessary config map, secret and roles to run the cloud controller manager
- As a prerequisite set the `provider-id-fmt` [flag](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/5e7f80878f2252c6ab13c16102de90c784a2624d/main.go#L168-L173) with value v2.
- Set ```export EXP_KUBEADM_BOOTSTRAP_FORMAT_IGNITION=true``` if using clusterctl or set ```"EXP_KUBEADM_BOOTSTRAP_FORMAT_IGNITION": "true"``` when using tilt to update capi bootstrap provided to set ignition format.

### Deploy PowerVS cluster with IBM PowerVS cloud provider

  ```
IBMCLOUD_API_KEY=<api_key>> \
IBMPOWERVS_SSHKEY_NAME="karthik-ssh" \
COS_BUCKET_REGION="us-south" \
COS_BUCKET_NAME="power-oss-bucket" \
COS_OBJECT_NAME=capi-rhcos-openstack-4.5.ova.gz \
IBMACCOUNT_ID="<account_id>" \
IBMPOWERVS_REGION="wdc" \
IBMPOWERVS_ZONE="wdc06" \
IBMVPC_REGION="us-east" \
IBM_RESOURCE_GROUP="ibm-hypershift-dev" \
BASE64_API_KEY=$(echo -n $IBMCLOUD_API_KEY | base64) \
clusterctl generate cluster capi-powervs- --kubernetes-version v1.28.4 \
--target-namespace default \
--control-plane-machine-count=3 \
--worker-machine-count=1 \
--from ./cluster-template-powervs-ignition.yaml | kubectl apply -f -
  ```
