# Using Autoscaler to scale machines from 0 machine

The autoscaler project supports [cluster-api](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/cloudprovider/clusterapi/README.md). With this [enhancement](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20210310-opt-in-autoscaling-from-zero.md#upgrade-strategy) now the user can use cluster-api feature to scaling from 0 machine.

## Settinng up the workload cluster

While creating a workload cluster, We need to set the below annotations to machinedeployment inorder to enable the autoscaling, This is one of the [prerequisites](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/cloudprovider/clusterapi/README.md#enabling-autoscaling) for autoscaler.
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

1. Clone the autoscaler repository
```
git clone https://github.com/kubernetes/autoscaler.git
```
2. Build the autoscaler binary
```
cd cluster-autoscaler 
go build .
```
3. Start the autoscaler
```
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

Note:
1. Autoscaler can be run in different ways, the possible ways are described [here](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/cloudprovider/clusterapi/README.md#connecting-cluster-autoscaler-to-cluster-api-management-and-workload-clusters).
2. Autoscaler supports various command line flags and more details about it can be found [here](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#what-are-the-parameters-to-ca).

## Use case of cluster-autoscaler 

1. Create a workload cluster with 0 worker machines
2. Create a sample workload
```
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: busybox
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
        - command:
            - sh
            - -c
            - echo Container 1 is Running ; sleep 3600
          image: busybox
          imagePullPolicy: IfNotPresent
          name: busybox
          resources:
            requests:
              cpu: "0.2"
              memory: 3G
```
3. Scale the deployment to create addition pods
```
kubectl scale --replicas=2 deployment/busybox-deployment 
```
4. Obeserve the status of new pods 
```
kubectl get pods                                        
NAME                                  READY   STATUS    RESTARTS   AGE
busybox-deployment-7c87788568-qhqdb   1/1     Running   0          48s
busybox-deployment-7c87788568-t26bb   0/1     Pending   0          5s
```
5. On the management cluster verify that the new machine creation is being triggered by autoscaler
```
NAME                                        CLUSTER               NODENAME                                  PROVIDERID                                                                                      PHASE          AGE     VERSION
ibm-powervs-control-plane-smvf7     ibm-powervs   ibm-powervs-control-plane-pgwmz   ibmpowervs://osa/osa21/3229a-af54-4212-bf60-6202b6fd0a07/809cd0f2-7502-4112-bf44-84d178020d8a   Running        82m     v1.24.2
ibm-powervs-md-0-6b4d67ccf4-npdbm   ibm-powervs   ibm-powervs-md-0-qch8f            ibmpowervs://osa/osa21/3229a-af54-4212-bf60-6202b6fd0a07/50f841e5-f58c-4569-894d-b40ba0d2696e   Running        76m     v1.24.2
ibm-powervs-md-0-6b4d67ccf4-v7xv9   ibm-powervs                                                                                                                                             Provisioning   3m19s   v1.24.2
```
6. After sometime verify that the new node being added to the cluster and pod is in running state
```
kubectl get nodes
NAME                                      STATUS   ROLES           AGE   VERSION
ibm-powervs-control-plane-pgwmz   Ready    control-plane   92m   v1.24.2
ibm-powervs-md-0-n8c6d            Ready    <none>          42s   v1.24.2
ibm-powervs-md-0-qch8f            Ready    <none>          85m   v1.24.2

kubectl get pods
NAME                                  READY   STATUS    RESTARTS   AGE
busybox-deployment-7c87788568-qhqdb   1/1     Running   0          19m
busybox-deployment-7c87788568-t26bb   1/1     Running   0          18m
```
7. Delete the deployment to observe the scale down of nodes by autoscaler
```
kubectl delete deployment/busybox-deployment

kubectl get nodes
NAME                                      STATUS   ROLES           AGE    VERSION
ibm-powervs-control-plane-pgwmz   Ready    control-plane   105m   v1.24.2
ibm-powervs-md-0-qch8f            Ready    <none>          98m    v1.24.2
```

