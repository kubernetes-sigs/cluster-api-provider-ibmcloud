# Troubleshooting

### 1. Tilt stops working as not able to connect to kind cluster

   ```
   % kind get clusters
    enabling experimental podman provider
    ERROR: failed to list clusters: command "podman ps -a --filter label=io.x-k8s.kind.cluster --format '{{index .Labels "io.x-k8s.kind.cluster"}}'" failed with error: exit status 125
    Command Output: Cannot connect to Podman. Please verify your connection to the Linux system using `podman system connection list`, or try `podman machine init` and `podman machine start` to manage a new Linux VM
    Error: unable to connect to Podman socket: failed to connect: dial tcp 127.0.0.1:61514: connect: connection refused
   ```

1. Stop and start the Podman either via cli or from Podman Desktop.
   ```shell
   $ podman machine stop
   $ podman machine start
   ```
2. Run all the stopped containers like capi-test-control-plane, capi-test-worker, kind-registry.
   ```shell
   $ podman container list -a
     CONTAINER ID  IMAGE                                    NAMES
     512cee59230c  docker.io/library/registry:2             kind-registry
     5b99fd84c41e  docker.io/kindest/node@sha256            capi-test-worker
     94130af58929  docker.io/kindest/node@sha256            capi-test-control-plane

   $ podman container start 512cee59230c 5b99fd84c41e 94130af58929
   ```
3. Try re-running `tilt up` from `cluster-api` directory.


### 2. SSH into data/control plane node configured with DHCP network
1. Since the VM backing the node is configured with DHCP network which is private we can't directly SSH into it.
2. Create a public VM in the same workspace and attach the DHCP network to it.

   1. Create public network in PowerVS workspace if it does not exist using ibmcloud cli
   ```shell
   $ibmcloud pi subnet create publicnet1 --net-type public
   ```
   2. List the available images to create VM
   ```shell
   $ibmcloud pi image lc
   ```
   3. Create the VM with public and DHCP subnet.
   ```shell
   $ibmcloud pi instance create publicVM --image testrhel88 --subnets DHCPSERVERcapi-powervs-new_Private,publicnet1
   ```
   4. Get the public IP of created VM
   ```shell
   $ibmcloud pi ins get publicVM
   ```
4. SSH into the DHCP VM using public VM as a jump host.
    ```
   ssh -J root@<public_ip> root@<dhcp_ip>
   ```

### 3. Failed to apply a cluster template with release not found error

While trying to apply a cluster template from unreleased version like from main branch, we will run into error like `release not found for version vX.XX.XX`. In that case, instead of `--flavor` we need to use `--from=<path_to_cluster_template>`.


### 4. Debugging Machine struck in PROVISIONED phase

* A Machine's Running phase indicates that it has successfully created, initialised and has become a Kubernetes Node in a Ready state.

* Sometimes a machine will be in Provisioned phase forever indicating infrastructure has been created and configured but yet to become a Kubernetes node.

* Cloud controller manager(CCM) takes care of turning a machine into a node by fetching and initialising with appropriate data from cloud.

* As a part of cluster create template we make use of [ClusterResourceSet](https://cluster-api.sigs.k8s.io/tasks/cluster-resource-set) to apply the CCM [resources](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/cbdb2550ab3e326c95d075a6dc852c81c15b1189/templates/cluster-template-powervs.yaml#L300-L315) into the workload cluster.

* Check the machine's current status

    ```shell
    $ kubectl get machines
    NAME                          CLUSTER   NODENAME   PROVIDERID                                                                                         PHASE          AGE     VERSION
    powervs-control-plane-pqnt4   powervs              ibmpowervs://osa/osa21/10b1000b-da8d-4e18-ad1f-6b2a56a8c130/bc0c9621-12d2-47f1-932e-a18ff041aba2   Provisioned    5m36s   v1.31.0
    ```

* Verify that the ClusterResourceSet is applied to the workload cluster

    ```shell
    $ kubectl get clusterresourceset
    NAME             AGE
    crs-cloud-conf   10m
    
    $ kubectl describe clusterresourceset crs-cloud-conf
    .
    .
    Status:
      Conditions:
        Last Transition Time:  2025-05-06T08:36:40Z
        Message:
        Observed Generation:   1
        Reason:                Applied
        Status:                True
        Type:                  ResourcesApplied
        Last Transition Time:  2025-05-06T08:31:27Z
        Message:
        Observed Generation:   1
        Reason:                NotPaused
        Status:                False
        Type:                  Paused
    ```

* Verify that the CCM resources are created in the workload cluster

   * Get the workload cluster kubeconfig

      ```
      $ clusterctl get kubeconfig powervs > workload.conf
      ```
   
   * Check the CCM daemonset's status

      ```
      $ kubectl get daemonset -n kube-system --kubeconfig=workload.conf
      NAME                                  DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR                            AGE
      ibmpowervs-cloud-controller-manager   2         2         2       2            2           node-role.kubernetes.io/control-plane=   45m
      ```
   
   * Check the logs of CCM

      ```
      $ kubectl -n kube-system get pods --kubeconfig=workload.conf
      ibmpowervs-cloud-controller-manager-472lq                     1/1     Running   1 (45m ago)   46m
      ibmpowervs-cloud-controller-manager-fw47h                     1/1     Running   1 (38m ago)   38m
      
      $ kubectl -n kube-system logs ibmpowervs-cloud-controller-manager-472lq --kubeconfig=workload.conf
      I0506 09:23:51.420992       1 ibm_metadata_service.go:206] Retrieving information for node=powervs-control-plane-ftd8j from Power VS
      I0506 09:23:51.421003       1 ibm_powervs_client.go:270] Node powervs-control-plane-ftd8j found metadata &{InternalIP:192.168.236.114 ExternalIP:163.68.98.114 WorkerID:001275c5-f454-4944-8419-61c16f16f8b7 InstanceType:s922 FailureDomain:osa21 Region:osa ProviderID:ibmpowervs://osa/osa21/10b1000b-da8d-4e18-ad1f-6b2a56a8c130/001275c5-f454-4944-8419-61c16f16f8b7} from DHCP cache
      I0506 09:23:51.421038       1 node_controller.go:271] Update 3 nodes status took 7.03624ms.
      ```
     
  * Check the cloud-conf config map

     ```
     $ kubectl -n kube-system get cm ibmpowervs-cloud-config -o yaml --kubeconfig=workload.conf
     apiVersion: v1
     kind: ConfigMap
     metadata:
        creationTimestamp: "2025-05-06T08:36:39Z"
        name: ibmpowervs-cloud-config
        namespace: kube-system
        resourceVersion: "329"
        uid: ae2bd436-0b1e-4534-9c6c-48f717f6f47e
    data:
     ibmpowervs.conf: |
       [global]
       version = 1.1.0
       [kubernetes]
        config-file = ""
        [provider]
        cluster-default-provider = g2
        .
        .
      ```
    
   * Check whether the secret is configured with correct IBM Cloud API key.
      
     ```
     $ kubectl -n kube-system get secret ibmpowervs-cloud-credential -o yaml --kubeconfig=workload.conf
     ```
* Check whether the node is initialised correctly and does not have taint `node.cloudprovider.kubernetes.io/uninitialized` taint

    ```shell
    $ kubectl get nodes --kubeconfig=workload.conf
    NAME                          STATUS     ROLES           AGE   VERSION
    powervs-control-plane-ftd8j   NotReady   control-plane   53m   v1.31.0
    powervs-control-plane-pqnt4   NotReady   control-plane   61m   v1.31.0
    powervs-md-0-2dnrm-8658c      NotReady   <none>          56m   v1.31.0
    
    
    $ kubectl get node powervs-control-plane-ftd8j -o yaml --kubeconfig=workload.conf
    apiVersion: v1
    kind: Node
    metadata:
      annotations:
        cluster.x-k8s.io/annotations-from-machine: ""
        cluster.x-k8s.io/cluster-name: powervs
        cluster.x-k8s.io/cluster-namespace: default
        cluster.x-k8s.io/labels-from-machine: ""
        cluster.x-k8s.io/machine: powervs-control-plane-ftd8j
        cluster.x-k8s.io/owner-kind: KubeadmControlPlane
        cluster.x-k8s.io/owner-name: powervs-control-plane
        kubeadm.alpha.kubernetes.io/cri-socket: unix:///var/run/containerd/containerd.sock
        node.alpha.kubernetes.io/ttl: "0"
        volumes.kubernetes.io/controller-managed-attach-detach: "true"
    ```

* On the successful CCM initialisation the machine will turn into Running phase and corresponding NODENAME field will be populated.
    ```shell
    NAME                          CLUSTER   NODENAME                      PROVIDERID                                                                                         PHASE          AGE     VERSION
    powervs-control-plane-pqnt4   powervs   powervs-control-plane-pqnt4   ibmpowervs://osa/osa21/10b1000b-da8d-4e18-ad1f-6b2a56a8c130/bc0c9621-12d2-47f1-932e-a18ff041aba2   Running        8m52s   v1.31.0
    ```