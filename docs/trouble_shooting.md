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
   ibmcloud-provider-system    clusterapi-controller-xxxxxxxxx-xxxxx   1/1     Running   0          27m
   ```

2. Get log of clusterapi-controller-xxxxxxxx-xxxxx

   ```
   # kubectl --kubeconfig minikube.kubeconfig log clusterapi-controller-xxxxxxxxx-xxxxx -n ibmcloud-provider-system
   ```
