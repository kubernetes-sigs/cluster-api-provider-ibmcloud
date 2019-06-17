<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Trouble shooting](#trouble-shooting)
  - [Get log of clusterapi-controller containers](#get-log-of-clusterapi-controller-containers)
  - [Cannot create bootstrap cluster if you are using kind](#cannot-create-bootstrap-cluster-if-you-are-using-kind)
  - [Calico node keeps CrashLoopBackOff](#calico-node-keeps-crashloopbackoff)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Trouble shooting

This guide (based on minikube and others should be similar) explains general info on how to debug issues if cluster failed to create.

## Get log of clusterapi-controller containers

1. Get ibmcloud container name, the output depends on the system you are running.
   the `minikube.kubeconfig` which is bootstrap cluster's kubeconfig by default locates at `cmd/clusterctl` folder.

   ```
   # kubectl --kubeconfig minikube.kubeconfig get pods -n ibmcloud-provider-system
   NAMESPACE                   NAME                                     READY   STATUS    RESTARTS   AGE
   ibmcloud-provider-system    clusterapi-controller-0                  1/1     Running   0          27m
   ```

2. Get log of clusterapi-controller-0

   ```
   # kubectl --kubeconfig minikube.kubeconfig log clusterapi-controller-0 -n ibmcloud-provider-system
   ```

## Cannot create bootstrap cluster if you are using kind
   
   Check if kind works well.

   ```
   # kind create cluster
   Creating cluster "kind" ...
   ✓ Ensuring node image (kindest/node:v1.14.2) 🖼
   ✓ Preparing nodes 📦
   ✓ Creating kubeadm config 📜
   ✓ Starting control-plane 🕹️
   ✓ Installing CNI 🔌
   ✓ Installing StorageClass 💾
   Cluster creation complete. You can now use the cluster with:

   export KUBECONFIG="$(kind get kubeconfig-path --name="kind")"
   kubectl cluster-info
   ```

   You may hit errors as below with kind v0.3.0 and docker-ce 18.09.6 on ubuntu.
   Follow the tickets to solve the problem.
   - https://github.com/kubernetes-sigs/kind/issues/567
   - https://github.com/moby/moby/issues/1871
   ```
   Creating cluster "kind" ...
   ✓ Ensuring node image (kindest/node:v1.14.2) 🖼
    ERRO[22:52:13] 0cd93e6e3b3a28c4216a3fa7b0d75337e83ca32f5e4095629c75a472b2ee89a6
    ERRO[22:52:13] docker: Error response from daemon: driver failed programming external connectivity on endpoint kind-control-plane (1229f3b0af4456532d4a8cf9ae274c0c03441da448de535ee94a1a6e25148d05):  (iptables failed: iptables --wait -t nat -A DOCKER -p tcp -d 127.0.0.1 --dport 46796 -j DNAT --to-destination 172.17.0.2:6443 ! -i docker0: iptables: No chain/target/match by that name.
    ERRO[22:52:13]  (exit status 1)).
    ✗ Preparing nodes 📦
    ERRO[22:52:13] docker run error: exit status 125
    Error: failed to create cluster: docker run error: exit status 125
   ```

## Calico node keeps CrashLoopBackOff

Check the pod CIDR and serivce CIDR you specified in `cluster.yaml` have no overlap with provisioned node CIDR, and each data center you specified in `machines.yaml` has different node CIDR setting, for complete node CIDR settings in all data centers, please refer to: https://control.softlayer.com/network/subnets

If the pod CIDR and serivce CIDR you specified in `cluster.yaml` have overlap with provisioned node CIDR, the `calico-node-xx` pod in worker node will failed to connect to tha api-server with the following logs:

   ```
   # kubectl --kubeconfig=kubeconfig -n kube-system logs -f calico-node-smfx8
   Threshold time for bird readiness check:  30s
   2019-05-27 11:14:48.306 [INFO][10] startup.go 256: Early log level set to info
   2019-05-27 11:14:48.306 [INFO][10] startup.go 272: Using NODENAME environment for node name
   2019-05-27 11:14:48.306 [INFO][10] startup.go 284: Determined node name: ibmcloud-node-jkb9p
   2019-05-27 11:14:48.307 [INFO][10] startup.go 316: Checking datastore connection
   2019-05-27 11:15:18.308 [INFO][10] startup.go 331: Hit error connecting to datastore - retry error=Get https://10.96.0.1:443/api/v1/nodes/foo: dial tcp 10.96.0.1:443: i/o timeout
   2019-05-27 11:15:49.309 [INFO][10] startup.go 331: Hit error connecting to datastore - retry error=Get https://10.96.0.1:443/api/v1/nodes/foo: dial tcp 10.96.0.1:443: i/o timeout
   ```

If this is the case you encounter, please change the pod CIDR and serivce CIDR you specified in `cluster.yaml` so that they have no overlap with provisioned node CIDR and then recreate the cluster.
