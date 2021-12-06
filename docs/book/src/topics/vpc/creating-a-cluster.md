### Provision workload Cluster in IBM Cloud VPC

Now that we have a management cluster ready, you can create your workload cluster by 
following the steps below. 

1. Using clusterctl, render the yaml through templates and deploy the cluster

    **Note:** the `IBMVPC_IMAGE_ID` value below should reflect the ID of the custom qcow2 image

    ```console
    IBMVPC_REGION=us-south \
    IBMVPC_ZONE=us-south-1 \
    IBMVPC_RESOURCEGROUP=4f15679623607b855b1a27a67f20e1c7 \
    IBMVPC_NAME=ibm-vpc-0 \
    IBMVPC_IMAGE_ID=r134-ea84bbec-7986-4ff5-8489-d9ec34611dd4 \
    IBMVPC_PROFILE=bx2-4x16 \
    IBMVPC_SSHKEY_ID=r134-2a82b725-e570-43d3-8b23-9539e8641944 \
    clusterctl generate cluster ibm-vpc-0 --kubernetes-version v1.19.9 \
    --target-namespace default \
    --control-plane-machine-count=1 \
    --worker-machine-count=2 \
    --from ./templates/cluster-template.yaml | kubectl apply -f -
    ```

    Output:
    ```console
    cluster.cluster.x-k8s.io/ibm-vpc-5 created
    ibmvpccluster.infrastructure.cluster.x-k8s.io/ibm-vpc-5 created
    kubeadmcontrolplane.controlplane.cluster.x-k8s.io/ibm-vpc-5-control-plane created
    ibmvpcmachinetemplate.infrastructure.cluster.x-k8s.io/ibm-vpc-5-control-plane created
    machinedeployment.cluster.x-k8s.io/ibm-vpc-5-md-0 created
    ibmvpcmachinetemplate.infrastructure.cluster.x-k8s.io/ibm-vpc-5-md-0 created
    kubeadmconfigtemplate.bootstrap.cluster.x-k8s.io/ibm-vpc-5-md-0 created
    ```

2. Deploy Container Network Interface (CNI)

    Example: calico
    ```console
    kubectl apply -f https://docs.projectcalico.org/v3.15/manifests/calico.yaml
    ```


3. Check the state of the provisioned cluster and machine objects within the local management cluster

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
    ibm-vpc-0-control-plane    true          true                   v1.19.9   1          1       1
    ```

    Machines
    ```console
    ~ kubectl get machines
    ibm-vpc-0-control-plane-vzz47     ibmvpc://ibm-vpc-0/ibm-vpc-0-control-plane-rg6xv   Running        v1.19.9
    ibm-vpc-0-md-0-5444cfcbcd-6gg5z   ibmvpc://ibm-vpc-0/ibm-vpc-0-md-0-dbxb7            Running        v1.19.9
    ibm-vpc-0-md-0-5444cfcbcd-7kr9x   ibmvpc://ibm-vpc-0/ibm-vpc-0-md-0-k7blr            Running        v1.19.9
    ```

4.  Check the state of the newly provisioned cluster within IBM Cloud

    ```console
    ~ clusterctl get kubeconfig ibm-vpc-0 > ~/.kube/ibm-vpc-0
    ~ export KUBECONFIG=~/.kube/ibm-vpc-0
    ~ kubectl get nodes
    NAME                             STATUS   ROLES    AGE   VERSION
    ibm-vpc-0-control-plane-rg6xv    Ready    master   41h   v1.18.15
    ibm-vpc-0-md-0-4dc5c             Ready    <none>   41h   v1.18.15
    ibm-vpc-0-md-0-dbxb7             Ready    <none>   20h   v1.18.15
    ```

