### Provision workload cluster in IBM Cloud PowerVS

Now that we have a management cluster ready, you can create your workload cluster by 
following the steps below. 

1. Create PowerVS network port 

    ```console
    ~ pvsadm create port --description "capi-port" --network <NETWORK_NAME> --instance-id <SERVICE_INSTANCE_ID>
    ```

    Output:
    ```console
    I1125 15:24:20.581757 1548881 port.go:89] Successfully created a port, id: ac18ef17-8517-40e3-889d-4f246e9bd17e
    +---------------------+------------+------+-----------------+-------------------+--------------------------------------+-------------+--------+
    |     DESCRIPTION     | EXTERNALIP | HREF |    IPADDRESS    |    MACADDRESS     |                PORTID                | PVMINSTANCE | STATUS |
    +---------------------+------------+------+-----------------+-------------------+--------------------------------------+-------------+--------+
    | capi-port |            |      | 192.168.151.125 | fa:16:3e:34:2c:ef | 7eff02b5-040c-4934-957a-18209e65eca4 |             | DOWN   |
    +---------------------+------------+------+-----------------+-------------------+--------------------------------------+-------------+--------+
    ```

    ```console
    ~ pvsadm get ports --instance-id <SERVICE_INSTANCE_ID> --network <NETWORK_NAME>
    ```

    Output:
    ```console
    +-------------------+-----------------+-----------------+-------------------+--------------------------------------+--------+
    |    DESCRIPTION    |   EXTERNALIP    |    IPADDRESS    |    MACADDRESS     |                PORTID                | STATUS |
    +-------------------+-----------------+-----------------+-------------------+--------------------------------------+--------+
    | capi-port         | 158.175.162.125 | 192.168.151.125 | fa:16:3e:34:2c:ef | 7eff02b5-040c-4934-957a-18209e65eca4 | DOWN   |
    +-------------------+-----------------+-----------------+-------------------+--------------------------------------+--------+
    ```

2. Use clusterctl to render the yaml through templates and deploy the cluster

    **Note:** To deploy workload cluster with Power VS cloud controller manager which is currently in experimental stage follow [these](/topics/powervs/external-cloud-provider.html) steps.

    **Note:** the `IBMPOWERVS_IMAGE_ID` value below should reflect the ID of the custom qcow2 image, the `kubernetes-version` value below should reflect the kubernetes version of the custom qcow2 image.

    ```console
    IBMPOWERVS_SSHKEY_NAME="my-pub-key" \
    IBMPOWERVS_VIP="192.168.151.22" \
    IBMPOWERVS_VIP_EXTERNAL="158.175.162.22" \
    IBMPOWERVS_VIP_CIDR="29" \
    IBMPOWERVS_IMAGE_NAME="capibm-powervs-centos-8-1-22-4" \
    IBMPOWERVS_SERVICE_INSTANCE_ID="7845d372-d4e1-46b8-91fc-41051c984601" \
    IBMPOWERVS_NETWORK_NAME="capi-test-3" \
    clusterctl generate cluster ibm-powervs-1 --kubernetes-version v1.22.4 \
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

3. Deploy Container Network Interface (CNI)

    Example: calico
    ```console
    kubectl apply -f https://docs.projectcalico.org/v3.15/manifests/calico.yaml
    ```


4. Check the state of the provisioned cluster and machine objects within the local management cluster

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
    ibm-powervs-1-control-plane    true          true                   v1.22.4   1          1       1
    ```

    Machines
    ```console
    ~ kubectl get machines
    ibm-powervs-1-control-plane-vzz47     ibmpowervs://ibm-powervs-1/ibm-powervs-1-control-plane-rg6xv   Running        v1.22.4
    ibm-powervs-1-md-0-5444cfcbcd-6gg5z   ibmpowervs://ibm-powervs-1/ibm-powervs-1-md-0-dbxb7            Running        v1.22.4
    ibm-powervs-1-md-0-5444cfcbcd-7kr9x   ibmpowervs://ibm-powervs-1/ibm-powervs-1-md-0-k7blr            Running        v1.22.4
    ```

5.  Check the state of the newly provisioned cluster within IBM Cloud

    ```console
    ~ clusterctl get kubeconfig ibm-powervs-1 > ~/.kube/ibm-powervs-1
    ~ export KUBECONFIG=~/.kube/ibm-powervs-1
    ~ kubectl get nodes
    NAME                             STATUS   ROLES    AGE   VERSION
    ibm-powervs-1-control-plane-rg6xv    Ready    master   41h   v1.22.4
    ibm-powervs-1-md-0-4dc5c             Ready    <none>   41h   v1.22.4
    ibm-powervs-1-md-0-dbxb7             Ready    <none>   20h   v1.22.4
    ```
