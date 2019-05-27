<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Trouble shooting](#trouble-shooting)
  - [Get log of clusterapi-controller containers](#get-log-of-clusterapi-controller-containers)

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
   
1. Check if kind works well.

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

2. Check docker version.

   You may hit errors with kind v0.3.0 and docker 18.09.6 on ubuntu. Use docker version < 18.09.6.
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
