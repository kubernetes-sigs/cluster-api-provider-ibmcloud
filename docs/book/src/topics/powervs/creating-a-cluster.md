# Creating a cluster

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

    Make sure these value reflects your API Key for PowerVS environment in IBM Cloud.

    ```console
    export IBMCLOUD_API_KEY=<YOUR_API_KEY>
    ```

3. Initialize local bootstrap cluter as a management cluster

    This cluster will be used to provision a workload cluster in IBM Cloud.

    ```console
    ~ clusterctl init --infrastructure ibmcloud:<TAG>
    ```

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

4. Create PowerVS network port

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

4. Provision workload cluster in IBM Cloud PowerVS

    You can use clusterctl to render the yaml through templates.

    **Note:** the `IBMPOWERVS_IMAGE_ID` value below should reflect the ID of the custom qcow2 image, the `kubernetes-version` value below should reflect the kubernetes version of the custom qcow2 image.

    ```console
    IBMPOWERVS_SSHKEY_NAME="my-pub-key" \
    IBMPOWERVS_VIP="192.168.151.22" \
    IBMPOWERVS_VIP_EXTERNAL="158.175.162.22" \
    IBMPOWERVS_VIP_CIDR="29" \
    IBMPOWERVS_IMAGE_ID="505f57d8-1143-4a99-b67f-7e82d73342bf" \
    IBMPOWERVS_SERVICE_INSTANCE_ID="7845d372-d4e1-46b8-91fc-41051c984601" \
    IBMPOWERVS_NETWORK_ID="0ad342f5-f461-414a-a870-e2f2a2b7fa0c" \
    clusterctl generate cluster ibm-powervs-1 --kubernetes-version v1.22.4 \
    --target-namespace default \
    --control-plane-machine-count=3 \
    --worker-machine-count=1 \
    --from ./cluster-template-powervs.yaml | kubectl apply -f -
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

5. Deploy Container Network Interface (CNI)

    Example: calico
    ```console
    kubectl apply -f https://docs.projectcalico.org/v3.15/manifests/calico.yaml
    ```


6. Check the state of the provisioned cluster and machine objects within the local management cluster

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

7.  Check the state of the newly provisioned cluster within IBM Cloud

    ```console
    ~ clusterctl get kubeconfig ibm-powervs-1 > ~/.kube/ibm-powervs-1
    ~ export KUBECONFIG=~/.kube/ibm-powervs-1
    ~ kubectl get nodes
    NAME                             STATUS   ROLES    AGE   VERSION
    ibm-powervs-1-control-plane-rg6xv    Ready    master   41h   v1.22.4
    ibm-powervs-1-md-0-4dc5c             Ready    <none>   41h   v1.22.4
    ibm-powervs-1-md-0-dbxb7             Ready    <none>   20h   v1.22.4
    ```
