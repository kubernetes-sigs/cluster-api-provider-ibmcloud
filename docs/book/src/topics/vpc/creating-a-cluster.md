### Provision workload Cluster in IBM Cloud VPC

Now that we have a management cluster ready, you can create your workload cluster by 
following the steps below. 

1. Using clusterctl, render the yaml through templates and deploy the cluster

    **Note:** the `IBMVPC_IMAGE_NAME` value below should reflect the Name of the custom qcow2 image

    ```console
    IBMVPC_REGION=us-south \
    IBMVPC_ZONE=us-south-1 \
    IBMVPC_RESOURCEGROUP=4f15679623607b855b1a27a67f20e1c7 \
    IBMVPC_NAME=ibm-vpc-0 \
    IBMVPC_IMAGE_NAME=capibm-vpc-ubuntu-2004-kube-v1-26-2 \
    IBMVPC_PROFILE=bx2-4x16 \
    IBMVPC_SSHKEY_NAME=capi-vpc-key \
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
    ```

    **Note:** Refer below for more detailed information on VPC variables.
    - [IBMVPC_REGION](/reference/regions-zones-mapping.html)
    - [IBMVPC_ZONE](/reference/regions-zones-mapping.html)
    - [IBMVPC_RESOURCEGROUP](https://cloud.ibm.com/docs/account?topic=account-rgs&interface=ui)
    - [IBMVPC_IMAGE_NAME](https://cloud.ibm.com/docs/vpc?topic=vpc-planning-custom-images)
    - [IBMVPC_PROFILE](https://cloud.ibm.com/docs/vpc?topic=vpc-profiles&interface=ui)
    - [IBMVPC_SSHKEY_NAME](https://cloud.ibm.com/docs/vpc?topic=vpc-managing-ssh-keys&interface=ui)

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
