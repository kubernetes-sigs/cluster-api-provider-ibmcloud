# Rapid iterative development with Tilt

## Overview

This document describes how to use [kind](https://kind.sigs.k8s.io) and [Tilt](https://tilt.dev) for a simplified workflow that offers easy deployments and rapid iterative builds.

## Prerequisites

1. [Docker](https://docs.docker.com/install/) v19.03 or newer
2. [kind](https://kind.sigs.k8s.io) v0.9 or newer (other clusters can be
   used if `preload_images_for_kind` is set to false)
3. [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/)
4. [Tilt](https://docs.tilt.dev/install.html) v0.22.2 or newer
5. [envsubst](https://github.com/drone/envsubst) or similar to handle
   clusterctl var replacement
6. Clone the [Cluster API](https://github.com/kubernetes-sigs/cluster-api) repository
   locally
7. Clone the [cluster-api-provider-ibmcloud](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud) repository you want to deploy locally as well

## Create a kind cluster

First, make sure you have a kind cluster and that your `KUBECONFIG` is set up correctly:

``` bash
kind create cluster
```

This local cluster will be running all the cluster api controllers and become the management cluster which then can be used to spin up workload clusters on IBM Cloud.

## Create a tilt-settings.json file

Next, create a `tilt-settings.json` file and place it in your local copy of `cluster-api`. Here is an example:

**Example `tilt-settings.json` for CAPI-IBM clusters:**

Make sure to replace the parameter `IBMCLOUD_API_KEY` with a valid API key.

```json
{
    "default_registry": "gcr.io/you-project-name-here",
    "provider_repos": ["../cluster-api-provider-ibmcloud"],
    "enable_providers": ["ibmcloud", "kubeadm-bootstrap", "kubeadm-control-plane"],
    "kustomize_substitutions": {
      "IBMCLOUD_API_KEY": "XXXXXXXXXXXXXXXXXX"
    }
}
```

Add following extra_args to log Power VS REST API Requests/Responses

```json
{
   "extra_args": {
      "ibmcloud": ["-v=5"]
   }
}
```
**NOTE**: For information about all the fields that can be used in the `tilt-settings.json` file, check them [here](https://cluster-api.sigs.k8s.io/developer/tilt.html#tilt-settingsjson-fields).

## Run Tilt

To launch your development environment, run:

``` bash
tilt up
```

Kind cluster becomes a management cluster after this point, check the pods running on the kind cluster by running `kubectl get pods -A`.

## Create workload clusters

To provision your workload cluster, check the `Creating a cluster` section for [VPC](/topics/vpc/creating-a-cluster.html) and [PowerVS](/topics/powervs/creating-a-cluster.html). 

After deploying it, check the tilt logs and wait for the clusters to be created.

## Clean up

Before deleting the kind cluster, make sure you delete all the workload clusters.

```bash
kubectl delete cluster <clustername>
tilt up (ctrl-c)
kind delete cluster
```
