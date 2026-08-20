# Using Autoscaler to scale from 0 machines

The [cluster-autoscaler](https://github.com/kubernetes/autoscaler) project supports [Cluster API](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/cloudprovider/clusterapi/README.md#cluster-autoscaler-on-cluster-api). With the [scale-from-zero enhancement](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20210310-opt-in-autoscaling-from-zero.md#upgrade-strategy), worker nodes can be scaled down to 0 and provisioned on demand.

## Setting up the workload cluster

Add the following annotations to your `MachineDeployment` to opt in to autoscaling. These are [required by the autoscaler](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/cloudprovider/clusterapi/README.md#enabling-autoscaling) to know the min/max bounds when scaling from 0.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: "${CLUSTER_NAME}-md-0"
  annotations:
    cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size: "5"
    cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size: "0"
```

## Setting up the cluster-autoscaler

1. Clone the autoscaler repository:
   ```console
   git clone https://github.com/kubernetes/autoscaler.git
   ```

2. Build the autoscaler binary:
   ```console
   cd autoscaler/cluster-autoscaler
   go build .
   ```

3. Start the autoscaler:
   ```console
   ./cluster-autoscaler \
     --cloud-provider=clusterapi \
     --v=2 \
     --namespace=default \
     --max-nodes-total=30 \
     --scale-down-delay-after-add=10s \
     --scale-down-delay-after-delete=10s \
     --scale-down-delay-after-failure=10s \
     --scale-down-unneeded-time=5m \
     --max-node-provision-time=30m \
     --balance-similar-node-groups \
     --expander=random \
     --kubeconfig=<workload_cluster_kubeconfig> \
     --cloud-config=<management_cluster_kubeconfig>
   ```

> **Note:** The autoscaler can be run in several ways — see [connecting to management and workload clusters](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/cloudprovider/clusterapi/README.md#connecting-cluster-autoscaler-to-cluster-api-management-and-workload-clusters) for alternatives. A full list of command-line flags is available in the [FAQ](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#what-are-the-parameters-to-ca).

## Walkthrough: scale up and scale down

1. Create a workload cluster with 0 worker machines.

2. Apply a sample workload:
   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: busybox-deployment
     namespace: default
   spec:
     replicas: 1
     selector:
       matchLabels:
         app: busybox
     template:
       metadata:
         labels:
           app: busybox
       spec:
         containers:
           - name: busybox
             image: busybox
             imagePullPolicy: IfNotPresent
             command: ["sh", "-c", "echo Running; sleep 3600"]
             resources:
               requests:
                 cpu: "0.2"
                 memory: 3G
   ```

3. Scale the deployment to trigger pending pods:
   ```console
   kubectl scale --replicas=2 deployment/busybox-deployment
   ```

4. Observe that the second pod is pending (no nodes available yet):
   ```console
   kubectl get pods
   NAME                                  READY   STATUS    RESTARTS   AGE
   busybox-deployment-7c87788568-qhqdb   1/1     Running   0          48s
   busybox-deployment-7c87788568-t26bb   0/1     Pending   0          5s
   ```

5. On the management cluster, watch the autoscaler provision a new machine:
   ```console
   kubectl get machines
   NAME                                  CLUSTER         PHASE          VERSION
   ibm-powervs-control-plane-smvf7       ibm-powervs     Running        v1.34.7
   ibm-powervs-md-0-6b4d67ccf4-npdbm    ibm-powervs     Running        v1.34.7
   ibm-powervs-md-0-6b4d67ccf4-v7xv9    ibm-powervs     Provisioning   v1.34.7
   ```

6. Once the new node joins, both pods should be running:
   ```console
   kubectl get nodes
   NAME                               STATUS   ROLES           AGE   VERSION
   ibm-powervs-control-plane-pgwmz   Ready    control-plane   92m   v1.34.7
   ibm-powervs-md-0-n8c6d            Ready    <none>          42s   v1.34.7
   ibm-powervs-md-0-qch8f            Ready    <none>          85m   v1.34.7

   kubectl get pods
   NAME                                  READY   STATUS    RESTARTS   AGE
   busybox-deployment-7c87788568-qhqdb   1/1     Running   0          19m
   busybox-deployment-7c87788568-t26bb   1/1     Running   0          18m
   ```

7. Delete the deployment and observe the autoscaler scale the node back down:
   ```console
   kubectl delete deployment/busybox-deployment

   kubectl get nodes
   NAME                               STATUS   ROLES           AGE    VERSION
   ibm-powervs-control-plane-pgwmz   Ready    control-plane   105m   v1.34.7
   ibm-powervs-md-0-qch8f            Ready    <none>          98m    v1.34.7
   ```
