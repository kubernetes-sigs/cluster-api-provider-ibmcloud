### Provision workload Cluster in IBM Cloud VPC

Now that we have a management cluster ready, you can create your workload cluster by 
following the steps below.

> **Note**:
> 1. The cluster will be deployed with [cloud controller manager](https://kubernetes.io/docs/concepts/architecture/cloud-controller/)
> 2. The [template](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/templates/cluster-template.yaml) uses the experimental feature gate [clusterresourceset](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-resource-set.html) which will create the necessary config map, secret and roles to run the cloud controller manager. Set `EXP_CLUSTER_RESOURCE_SET` to true.
> 3. As a prerequisite, set the `provider-id-fmt` [flag](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/bfb33f159d5edd87dcbbb45942a6ffdc3aedb067/main.go#L137) to `v2` via `PROVIDER_ID_FORMAT` environment variable.
> 4. To deploy a cluster using [ClusterClass](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/index.html), refer [here](#deploy-a-cluster-using-ibm-cloud-vpc-infrastructure-using-clusterclass). In additional to the above flags, set `CLUSTER_TOPOLOGY` environment variable to `true`. 
> 5. The list of IBM Cloud VPC Regions and Zones can be found [here](../../reference/regions-zones-mapping.md).


1. Using clusterctl, render the yaml through templates and deploy the cluster


    **Note:** the `IBMVPC_IMAGE_NAME` value below should reflect the name of the custom qcow2 image

    ```console
    IBMCLOUD_API_KEY="XXXXXXXXXXXXXXXXXX" \
    IBMVPC_REGION=us-south \
    IBMVPC_ZONE=us-south-1 \
    IBMVPC_RESOURCEGROUP=4f15679623607b855b1a27a67f20e1c7 \
    IBMVPC_NAME=ibm-vpc-0 \
    IBMVPC_IMAGE_NAME=capibm-vpc-ubuntu-2004-kube-v1-26-2 \
    IBMVPC_PROFILE=bx2-4x16 \
    IBMVPC_SSHKEY_NAME=capi-vpc-key \
    IBMACCOUNT_ID="ibm-accountid" \
    BASE64_API_KEY=$(echo -n $IBMCLOUD_API_KEY | base64) \
    clusterctl generate cluster ibm-vpc-0 --kubernetes-version v1.26.2 \
    --target-namespace default \
    --control-plane-machine-count=1 \
    --worker-machine-count=2 | kubectl apply -f -
    ```

    Output:
    ```console
    cluster.cluster.x-k8s.io/ibm-vpc-0 created
    ibmvpccluster.infrastructure.cluster.x-k8s.io/ibm-vpc-0 created
    kubeadmcontrolplane.controlplane.cluster.x-k8s.io/ibm-vpc-0-control-plane created
    ibmvpcmachinetemplate.infrastructure.cluster.x-k8s.io/ibm-vpc-0-control-plane created
    machinedeployment.cluster.x-k8s.io/ibm-vpc-0-md-0 created
    ibmvpcmachinetemplate.infrastructure.cluster.x-k8s.io/ibm-vpc-0-md-0 created
    kubeadmconfigtemplate.bootstrap.cluster.x-k8s.io/ibm-vpc-0-md-0 created
    clusterresourceset.addons.cluster.x-k8s.io/crs-cloud-conf created
    configmap/ibm-cfg created
    secret/ibm-credential created
    configmap/cloud-controller-manager-addon created
    ```

    **Note:** Refer below for more detailed information on VPC variables.
    - [IBMVPC_REGION](/reference/regions-zones-mapping.html)
    - [IBMVPC_ZONE](/reference/regions-zones-mapping.html)
    - [IBMVPC_RESOURCEGROUP](https://cloud.ibm.com/docs/account?topic=account-rgs&interface=ui)
    - [IBMVPC_IMAGE_NAME](https://cloud.ibm.com/docs/vpc?topic=vpc-planning-custom-images)
    - [IBMVPC_PROFILE](https://cloud.ibm.com/docs/vpc?topic=vpc-profiles&interface=ui)
    - [IBMVPC_SSHKEY_NAME](https://cloud.ibm.com/docs/vpc?topic=vpc-managing-ssh-keys&interface=ui)
    - [IBMACCOUNT_ID](https://cloud.ibm.com/docs/account?topic=account-accountfaqs#account-details)

2. Check the state of the provisioned cluster and machine objects within the local management cluster

    Clusters
    ```console
    ~ kubectl get clusters
    NAME         PHASE
    ibm-vpc-0    Provisioned
    ```

    Kubeadm Control Plane
    ```console
    ~ kubectl get kubeadmcontrolplane
    NAME                       INITIALIZED   API SERVER AVAILABLE   VERSION   REPLICAS   READY   UPDATED   UNAVAILABLE
    ibm-vpc-0-control-plane    true          true                   v1.26.2   1          1       1
    ```

    Machines
    ```console
    ~ kubectl get machines
    ibm-vpc-0-control-plane-vzz47     ibmvpc://ibm-vpc-0/ibm-vpc-0-control-plane-rg6xv   Running        v1.26.2
    ibm-vpc-0-md-0-5444cfcbcd-6gg5z   ibmvpc://ibm-vpc-0/ibm-vpc-0-md-0-dbxb7            Running        v1.26.2
    ibm-vpc-0-md-0-5444cfcbcd-7kr9x   ibmvpc://ibm-vpc-0/ibm-vpc-0-md-0-k7blr            Running        v1.26.2
    ```

3. Deploy Container Network Interface (CNI)

    Example: calico
    ```console
    ~ clusterctl get kubeconfig ibm-vpc-0 > ~/.kube/ibm-vpc-0
    ~ export KUBECONFIG=~/.kube/ibm-vpc-0
    ~ kubectl apply -f https://docs.projectcalico.org/v3.15/manifests/calico.yaml
    ```

4.  Check the state of the newly provisioned cluster within IBM Cloud

    ```console
    ~ kubectl get nodes
    NAME                             STATUS   ROLES    AGE   VERSION
    ibm-vpc-0-control-plane-rg6xv    Ready    master   41h   v1.26.2
    ibm-vpc-0-md-0-4dc5c             Ready    <none>   41h   v1.26.2
    ibm-vpc-0-md-0-dbxb7             Ready    <none>   20h   v1.26.2
    ```

**Change disk size for the boot volume**

There are two following variables for controlling the volume size for the boot disk.
- `IBMVPC_CONTROLPLANE_BOOT_VOLUME_SIZEGIB`: Size of the boot volume for the control plane nodes, default set to 20GiB
- `IBMVPC_WORKER_BOOT_VOLUME_SIZEGIB`: Size of the boot volume for the worker nodes, default set to 20GiB
> **Note**: Default value is set to 20GiB because the images published for testing are of size 20GiB(default size in the image-builder scripts as well).


### Deploy a VPC cluster using ClusterClass

    IBMVPC_CLUSTER_CLASS_NAME=ibmvpc-clusterclass \
    IBMCLOUD_API_KEY="XXXXXXXXXXXXXXXXXX" \
    IBMVPC_REGION=us-south \
    IBMVPC_ZONE=us-south-1 \
    IBMVPC_RESOURCEGROUP=4f15679623607b855b1a27a67f20e1c7 \
    IBMVPC_NAME=ibm-vpc-0 \
    IBMVPC_IMAGE_NAME=capibm-vpc-ubuntu-2004-kube-v1-26-2 \
    IBMVPC_PROFILE=bx2-4x16 \
    IBMVPC_SSHKEY_NAME=capi-vpc-key \
    IBMACCOUNT_ID="ibm-accountid" \
    BASE64_API_KEY=$(echo -n $IBMCLOUD_API_KEY | base64) \
    clusterctl generate cluster ibm-vpc-clusterclass --kubernetes-version v1.26.2 --target-namespace default --control-plane-machine-count=1 --worker-machine-count=2 --from=./templates/cluster-template-vpc-clusterclass.yaml | kubectl apply -f -
  