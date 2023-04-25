# Getting Started

For prerequisites, check the respective sections for [VPC](topics/vpc/prerequisites.md) and [PowerVS](topics/powervs/prerequisites.md)

Now that we’ve got all the prerequisites in place, let’s create a Kubernetes cluster and transform 
it into a management cluster using `clusterctl`.

### Provision local boostrap management cluster:

1. Create simple, local bootstrap cluster with a control-plane and worker node

    Using [kind](https://kind.sigs.k8s.io/docs/user/quick-start/):

    ```console
    ~ kind create cluster --name my-bootstrap --config bootstrap.yaml
    ```

    Example bootstrap.yaml:
    ```yaml
    kind: Cluster
    apiVersion: kind.x-k8s.io/v1alpha4
    nodes:
       - role: control-plane
       - role: worker
    ```

    Make sure the nodes are in `Ready` state before moving on.

    ```console
    ~ kubectl get nodes
    NAME                         STATUS   ROLES                  AGE   VERSION
    my-bootstrap-control-plane   Ready    control-plane,master   46h   v1.20.2
    my-bootstrap-worker          Ready    <none>                 46h   v1.20.2
    ```

2. Set workload cluster environment variables

    Make sure these value reflects your API Key for your target VPC environment 
    or PowerVS environment in IBM Cloud.

    ```console
    export IBMCLOUD_API_KEY=<YOUR_API_KEY>
    ```
    For enabling debug level logs for the controller, set the `LOGLEVEL` environment variable(defaults to 0).
    ```console
    export LOGLEVEL=5
    ```

    > Note: Refer [Regions-Zones Mapping](/reference/regions-zones-mapping.html) for more information.

    > Note: To deploy VPC workload cluster with [IBM cloud controller manager](/topics/vpc/load-balancer.html) which is currently in experimental stage. Set the `PROVIDER_ID_FORMAT` environmental variable.
    Currently, [ClusterResourceset](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-resource-set.html) is experimental feature, so we need to enable the feature gate by setting `EXP_CLUSTER_RESOURCE_SET` environmental variables.
    ```console
       export PROVIDER_ID_FORMAT=v2
       export EXP_CLUSTER_RESOURCE_SET=true
     ```

    > Note: To deploy workload cluster with [PowerVS cloud controller manager](/topics/powervs/external-cloud-provider.html) which is currently in experimental stage. Set the `POWERVS_PROVIDER_ID_FORMAT` environmental variable.
    Currently, [ClusterResourceset](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-resource-set.html) is experimental feature so we need to enable the feature gate by setting `EXP_CLUSTER_RESOURCE_SET` environmental variables.
    ```console
       export POWERVS_PROVIDER_ID_FORMAT=v2
       export EXP_CLUSTER_RESOURCE_SET=true
     ```
    > Note: To deploy workload cluster with [PowerVS clusterclass-template](/topics/powervs/clusterclass-cluster.html). Set the `POWERVS_PROVIDER_ID_FORMAT` environmental variable.
      Currently, both [ClusterClass](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/index.html) and [ClusterResourceset](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-resource-set.html) are experimental feature so we need to enable the feature gate by setting `EXP_CLUSTER_RESOURCE_SET`, `CLUSTER_TOPOLOGY` environmental variables.
     ```console
        export POWERVS_PROVIDER_ID_FORMAT=v2
        export EXP_CLUSTER_RESOURCE_SET=true
        export CLUSTER_TOPOLOGY=true
     ```

    > Note: To deploy workload cluster with Custom Service Endpoint, Set `SERVICE_ENDPOINT` environmental variable in semi-colon separated format:
     ```console
        `${ServiceRegion1}:${ServiceID1}=${URL1},${ServiceID2}=${URL2};${ServiceRegion2}:${ServiceID1}=${URL1...}`.
     ```
    Supported ServiceIDs include - `vpc, powervs, rc`
     ```console
        export SERVICE_ENDPOINT=us-south:vpc=https://us-south-stage01.iaasdev.cloud.ibm.com,powervs=https://dal.power-iaas.test.cloud.ibm.com,rc=https://resource-controller.test.cloud.ibm.com
     ```

2. Initialize local bootstrap cluster as a management cluster
    
    When executed for the first time, the following command accepts the infrastructure provider as an input to install. `clusterctl init` automatically adds to the list the cluster-api core provider, and if unspecified, it also adds the kubeadm bootstrap and kubeadm control-plane providers, thereby converting it into a management cluster which will be used to provision a workload cluster in IBM Cloud.

    ```console
    ~ clusterctl init --infrastructure ibmcloud:<TAG>
    ```
    > Note: If the latest release version of the provider is available, specifying TAG can be avoided.
    In other cases, you can specify any prerelease version compatible with the supported API contract as the TAG.  
    Example: clusterctl init --infrastructure ibmcloud:v0.2.0-alpha.5

    Output:
    ```console
    Fetching providers
    Installing cert-manager Version="v1.5.3"
    Waiting for cert-manager to be available...
    Installing Provider="cluster-api" Version="v0.4.4" TargetNamespace="capi-system"
    Installing Provider="bootstrap-kubeadm" Version="v0.4.4" TargetNamespace="capi-kubeadm-bootstrap-system"
    Installing Provider="control-plane-kubeadm" Version="v0.4.4" TargetNamespace="capi-kubeadm-control-plane-system"
    Installing Provider="infrastructure-ibmcloud" Version="v0.1.0-alpha.2" TargetNamespace="capi-ibmcloud-system"

    Your management cluster has been initialized successfully!

    You can now create your first workload cluster by running the following:

    clusterctl generate cluster [name] --kubernetes-version [version] | kubectl apply -f -
    ```

5. Once the management cluster is ready with the required providers up and running, proceed to provisioning the workload cluster. Check the respective sections for [VPC](/topics/vpc/creating-a-cluster.html) and [PowerVS](/topics/powervs/creating-a-cluster.html) to deploy the cluster. 