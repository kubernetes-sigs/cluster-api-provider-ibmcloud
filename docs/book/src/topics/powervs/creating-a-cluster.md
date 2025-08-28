### Provision workload cluster in IBM Cloud PowerVS

> **Note:**
> A PowerVS cluster can be deployed with different customisations. Pick one of the following templates as per your need and fulfill the [prerequisites](prerequisites.md) before proceeding with cluster creation.
> - [PowerVS cluster with infrastructure creation](#deploy-a-powervs-cluster-with-infrastructure-creation)
> - [PowerVS cluster with cluster class](#deploy-a-powervs-cluster-with-cluster-class)

Now that we have a management cluster ready, you can create your workload cluster by 
following the steps below. 

1. Create PowerVS network port 

    ```console
    ~ export IBMCLOUD_API_KEY=<API_KEY>
    ~ capibmadm powervs port create --network capi-test --description capi-test-port --service-instance-id 3229a94c-af54-4212-bf60-6202b6fd0a07 --zone osa21
    ```

    Output:
    ```console
    Creating Port  Network ID/Name="capi-test" IP Address="" Description="capi-test-port" service-instance-id="3229a94c-af54-4212-bf60-6202b6fd0a07" zone="osa21"
    Successfully created a port portID="c7e7b6e0-0b0d-4a11-a90b-6ea293deb5ac"
    DESCRIPTION      EXTERNAL IP   IP ADDRESS      MAC ADDRESS         PORT ID                                STATUS
    capi-test-port                 192.168.167.6   fa:16:3e:89:c8:80   c7e7b6e0-0b0d-4a11-a90b-6ea293deb5ac   DOWN
    ```

    ```console
    ~ capibmadm powervs port list --network capi-test --service-instance-id 3229a94c-af54-4212-bf60-6202b6fd0a07 --zone osa21
    ```

    Output:
    ```console
    Listing PowerVS ports service-instance-id="3229a94c-af54-4212-bf60-6202b6fd0a07" network="capi-test"
    DESCRIPTION      EXTERNAL IP   IP ADDRESS      MAC ADDRESS         PORT ID                                STATUS
    capi-test-port   163.68.65.6   192.168.167.6   fa:16:3e:89:c8:80   c7e7b6e0-0b0d-4a11-a90b-6ea293deb5ac   DOWN
    ```

2. Use clusterctl to render the yaml through templates and deploy the cluster.
  **Replace the following snippet with the template of your choice.**

    > **Note:**
    > - Set `EXP_CLUSTER_RESOURCE_SET` to `true` as the cluster will be deployed with external cloud provider which will create the resources to run the cloud controller manager.
    > - The `IBMPOWERVS_IMAGE_NAME` value below should reflect the name of the custom image and the `kubernetes-version` value below should reflect the kubernetes version of the custom image.
    > - While working with unreleased versions like from main branch, instead of `--flavor=powervs` use `--from=./templates/cluster-template-powervs.yaml`.
    > - Refer detailed information on PowerVS variables [here](#note-refer-below-for-more-detailed-information-on-powervs-variables)

    ```console
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
    --flavor=powervs | kubectl apply -f -
    ```

    Output:
    ```console
    cluster.cluster.x-k8s.io/ibm-powervs-1 created
    ibmpowervscluster.infrastructure.cluster.x-k8s.io/ibm-powervs-1 created
    kubeadmcontrolplane.controlplane.cluster.x-k8s.io/ibm-powervs-1-control-plane created
    ibmpowervsmachinetemplate.infrastructure.cluster.x-k8s.io/ibm-powervs-1-control-plane created
    machinedeployment.cluster.x-k8s.io/ibm-powervs-1-md-0 created
    ibmpowervsmachinetemplate.infrastructure.cluster.x-k8s.io/ibm-powervs-1-md-0 created
    kubeadmconfigtemplate.bootstrap.cluster.x-k8s.io/ibm-powervs-1-md-0 created
    ```

    Additional parameters for modifying PowerVS Control-Plane spec
    ```
    IBMPOWERVS_CONTROL_PLANE_MEMORY
    IBMPOWERVS_CONTROL_PLANE_PROCESSORS
    IBMPOWERVS_CONTROL_PLANE_SYSTYPE
    IBMPOWERVS_CONTROL_PLANE_PROCTYPE
    ```

    Additional parameters for modifying PowerVS Compute node spec
    ```
    IBMPOWERVS_COMPUTE_MEMORY
    IBMPOWERVS_COMPUTE_PROCESSORS
    IBMPOWERVS_COMPUTE_SYSTYPE
    IBMPOWERVS_COMPUTE_PROCTYPE
    ```

    Additional parameters for modifying PowerVS Cluster API server port
    ```
    API_SERVER_PORT
    ```

3. Check the state of the provisioned cluster and machine objects within the local management cluster

    Clusters
    ```console
    ~ kubectl get clusters
    NAME         PHASE
    ibm-powervs-1    Provisioned
    ```

    Kubeadm Control Plane
    ```console
    ~ kubectl get kubeadmcontrolplane
    NAME                       INITIALIZED   API SERVER AVAILABLE   VERSION   REPLICAS   READY   UPDATED   UNAVAILABLE
    ibm-powervs-1-control-plane    true          true                   v1.26.2   1          1       1
    ```

    Machines
    ```console
    ~ kubectl get machines
    ibm-powervs-1-control-plane-vzz47     ibmpowervs://ibm-powervs-1/ibm-powervs-1-control-plane-rg6xv   Running        v1.26.2
    ibm-powervs-1-md-0-5444cfcbcd-6gg5z   ibmpowervs://ibm-powervs-1/ibm-powervs-1-md-0-dbxb7            Running        v1.26.2
    ibm-powervs-1-md-0-5444cfcbcd-7kr9x   ibmpowervs://ibm-powervs-1/ibm-powervs-1-md-0-k7blr            Running        v1.26.2
    ```

4. Deploy Container Network Interface (CNI)

    Example: calico
    ```console
    ~ clusterctl get kubeconfig ibm-powervs-1 > ~/.kube/ibm-powervs-1
    ~ export KUBECONFIG=~/.kube/ibm-powervs-1
    ~ kubectl apply -f https://docs.projectcalico.org/v3.15/manifests/calico.yaml
    ```

5.  Check the state of the newly provisioned cluster within IBM Cloud

    ```console
    ~ kubectl get nodes
    NAME                             STATUS   ROLES    AGE   VERSION
    ibm-powervs-1-control-plane-rg6xv    Ready    master   41h   v1.26.2
    ibm-powervs-1-md-0-4dc5c             Ready    <none>   41h   v1.26.2
    ibm-powervs-1-md-0-dbxb7             Ready    <none>   20h   v1.26.2

### Deploy a PowerVS cluster with infrastructure creation

#### Prerequisites: 
- Already existing infrasturcture resources can be used for cluster creation by setting either the ID or name in spec. If neither are specified, the cluster name will be used for constructing the resource name. For example, if cluster name is `capi-powervs`, PowerVS workspace will be created with name `capi-powervs-serviceInstance`.

**Note:** While working with unreleased versions like from main branch, instead of `--flavor=powervs-create-infra` use `--from=./templates/cluster-template-powervs-create-infra.yaml`.

  ```
    IBMCLOUD_API_KEY=XXXXXXXXXXXX \
    IBMPOWERVS_SSHKEY_NAME="my-ssh-key" \
    COS_BUCKET_REGION="us-south" \
    COS_BUCKET_NAME="power-oss-bucket" \
    COS_OBJECT_NAME=capibm-powervs-centos-streams8-1-28-4-1707287079.ova.gz \
    IBMACCOUNT_ID="<account_id>" \
    IBMPOWERVS_REGION="wdc" \
    IBMPOWERVS_ZONE="wdc06" \
    IBMVPC_REGION="us-east" \
    IBM_RESOURCE_GROUP="ibm-resource-group" \
    BASE64_API_KEY=$(echo -n $IBMCLOUD_API_KEY | base64) \
    clusterctl generate cluster capi-powervs --kubernetes-version v1.28.4 \
    --target-namespace default \
    --control-plane-machine-count=3 \
    --worker-machine-count=1 \
    --flavor=powervs-create-infra | kubectl apply -f -
  ```

### Deploy a PowerVS cluster with cluster class

#### Prerequisites:
- To deploy a cluster using [ClusterClass](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/index.html), set `CLUSTER_TOPOLOGY` environment variable to `true`.

**Note:** While working with unreleased versions like from main branch, instead of `--flavor=powervs-clusterclass` use `--from=./templates/cluster-template-powervs-clusterclass.yaml`.

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
  --flavor=powervs-clusterclass | kubectl apply -f -
  ```

#### Note: Refer below for more detailed information on PowerVS variables.
- IBMPOWERVS_CLUSTER_CLASS_NAME : Name of the cluster that user provides.
- IBMPOWERVS_SSHKEY_NAME : Name of the SSH Key. Refer [here](../../topics/capibmadm/powervs/key.md#3-capibmadm-powervs-key-list) how to get the keys.
- IBMPOWERVS_VIP, IBMPOWERVS_VIP_EXTERNAL and IBMPOWERVS_VIP_CIDR : Once we have created the network and port, the values will be available on listing the ports. Refer [here](../../topics/capibmadm/powervs/port.md#3-capibmadm-powervs-port-list)
- IBMPOWERVS_IMAGE_NAME : Name of the custom image. Refer [here](../../topics/capibmadm/powervs/image.md#2-capibmadm-powervs-image-list) to get image details.
- IBMPOWERVS_SERVICE_INSTANCE_ID : ID of the PowerVS workspace. Refer [here](https://cloud.ibm.com/docs/power-iaas-cli-plugin?topic=power-iaas-cli-plugin-power-iaas-cli-reference-v1#ibmcloud-pi-workspace)
- IBMACCOUNT_ID : Go to the Account settings page in the IBM Cloud console to view your account ID and type. The account ID is a 32 character, unique account identifier.Refer [here](https://cloud.ibm.com/account/settings)
- IBMPOWERVS_NETWORK_NAME : The name of the network. Refer [here](../../topics/capibmadm/powervs/network.md#3-capibmadm-powervs-network-list) how to get the network details.
- [IBMPOWERVS_REGION](../../reference/regions-zones-mapping.md)
- [IBMPOWERVS_ZONE](../../reference/regions-zones-mapping.md)
- IBM_RESOURCE_GROUP : Go to Manage > Account > Account resources > Resource groups in the IBM Cloud console to get the resource group Name.
- COS_BUCKET_REGION, COS_BUCKET_NAME and COS_OBJECT_NAME : Refer [here](../../machine-images/powervs.md#powervs-images-with-dhcp-based-network) to get the COS details.
