### Provision workload cluster in IBM Cloud PowerVS

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

2. Use clusterctl to render the yaml through templates and deploy the cluster

    **Note:** To deploy workload cluster with PowerVS cloud controller manager which is currently in experimental stage follow [these](/topics/powervs/external-cloud-provider.html) steps.

    **Note:** the `IBMPOWERVS_IMAGE_ID` value below should reflect the ID of the custom qcow2 image, the `kubernetes-version` value below should reflect the kubernetes version of the custom qcow2 image.

    ```console
    IBMPOWERVS_SSHKEY_NAME="my-pub-key" \
    IBMPOWERVS_VIP="192.168.167.6" \
    IBMPOWERVS_VIP_EXTERNAL="163.68.65.6" \
    IBMPOWERVS_VIP_CIDR="29" \
    IBMPOWERVS_IMAGE_NAME="capibm-powervs-centos-streams8-1-26-2" \
    IBMPOWERVS_SERVICE_INSTANCE_ID="3229a94c-af54-4212-bf60-6202b6fd0a07" \
    IBMPOWERVS_NETWORK_NAME="capi-test" \
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
    ```
