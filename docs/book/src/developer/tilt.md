# Rapid iterative development with Tilt

## Overview

This document describes how to use [kind](https://kind.sigs.k8s.io) and [Tilt](https://tilt.dev) for a simplified workflow that offers easy deployments and rapid iterative builds.

## Prerequisites

1. Container Runtime Interface
    * [Docker](https://docs.docker.com/install/)    - v19.03 or newer
    * [Podman](https://podman.io/docs/installation) - v3.0 or newer
2. [kind](https://kind.sigs.k8s.io) v0.9 or newer (other clusters can be
   used if `preload_images_for_kind` is set to false)
3. [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/)
4. [Tilt](https://docs.tilt.dev/install.html) v0.30.8 or newer
5. [envsubst](https://github.com/drone/envsubst) or similar to handle
   clusterctl var replacement
6. Clone the [Cluster API](https://github.com/kubernetes-sigs/cluster-api) repository
   locally
7. Clone the [cluster-api-provider-ibmcloud](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud) repository you want to deploy locally as well

---
If the user prefers to have Podman as the CRI, then follow the steps listed below:

1. Emulate Docker CLI with Podman: Instructions can be found [here](https://podman-desktop.io/docs/migrating-from-docker/emulating-docker-cli-with-podman)
2. On `Mac OS` migrate from Docker to Podman: Instructions can be found
 [here](https://podman-desktop.io/docs/migrating-from-docker/using-podman-mac-helper).

### 1. Create Podman machine

```shell
$ podman machine init
$ podman machine start
```

### 2. Configure podman to use local registry

```shell
$ podman machine ssh
$ sudo vi /etc/containers/registries.conf

## at the end of the file add below content

[[registry]]
location = "localhost:5001"
insecure = true
```
### 3. Restart Podman machine

```shell
podman machine stop
podman machine start
```
---

## Create a kind cluster

First, make sure you have a kind cluster and that your `KUBECONFIG` is set up correctly.
> **Note:** Execute the following from the `cluster-api-provider-ibmcloud` respository. 

``` bash
make kind-cluster
```

This local cluster will host the cluster-api controllers, which makes it the management cluster. The management cluster can be used to create and manage workload clusters on IBM Cloud.

---

## Create a tilt-settings.yaml file

Next, create a `tilt-settings.yaml` file and place it in your local copy of `cluster-api`. Here is an example:

**Example `tilt-settings.yaml` for CAPI-IBM clusters:**

Make sure to set a valid API key for the field `IBMCLOUD_API_KEY`.

```yaml
default_registry: "gcr.io/you-project-name-here"
provider_repos:
- ../cluster-api-provider-ibmcloud
enable_providers:
- ibmcloud
- kubeadm-bootstrap
- kubeadm-control-plane
kustomize_substitutions:
  IBMCLOUD_API_KEY: "XXXXXXXXXXXXXXXXXX"
```

Add following extra_args to log PowerVS REST API Requests/Responses

```yaml
extra_args:
  ibmcloud:
    - '-v=5'
```
---
## Different flavors of deploying workload clusters using CAPIBM.

> **Note:** Currently, both [ClusterClass](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/index.html) and [ClusterResourceset](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-resource-set.html) are experimental features.

### 1.  Configuration to deploy workload cluster with external cloud controller manager

To deploy workload cluster with cloud controller manager, set `PROVIDER_ID_FORMAT` to `v2` and enable cluster resourceset feature gate by setting `EXP_CLUSTER_RESOURCE_SET` to `true under kustomize_substitutions.

```yaml
default_registry: "gcr.io/you-project-name-here"
provider_repos:
- ../cluster-api-provider-ibmcloud
enable_providers:
- ibmcloud
- kubeadm-bootstrap
- kubeadm-control-plane
kustomize_substitutions:
  IBMCLOUD_API_KEY: "XXXXXXXXXXXXXXXXXX"
  PROVIDER_ID_FORMAT: "v2"
  EXP_CLUSTER_RESOURCE_SET: "true"
```

### 2.  Configuration to deploy workload cluster from ClusterClass template

To deploy workload cluster with [clusterclass-template](/topics/powervs/clusterclass-cluster.html), set the `PROVIDER_ID_FORMAT` to `v2` and enable the feature gates `EXP_CLUSTER_RESOURCE_SET` and `CLUSTER_TOPOLOGY` to `true`under kustomize_substitutions.

```yaml
default_registry: "gcr.io/you-project-name-here"
provider_repos:
- ../cluster-api-provider-ibmcloud
enable_providers:
- ibmcloud
- kubeadm-bootstrap
- kubeadm-control-plane
kustomize_substitutions:
  IBMCLOUD_API_KEY: "XXXXXXXXXXXXXXXXXX"
  PROVIDER_ID_FORMAT: "v2"
  EXP_CLUSTER_RESOURCE_SET: "true"
  CLUSTER_TOPOLOGY: "true"
```

### 3.  Configuration to deploy workload cluster with Custom Service Endpoint

To deploy workload cluster with Custom Service Endpoint, Set `SERVICE_ENDPOINT` environmental variable in semi-colon separated format: `${ServiceRegion}:${ServiceID1}=${URL1},${ServiceID2}=${URL2...}`
```yaml
default_registry: "gcr.io/you-project-name-here"
provider_repos:
- ../cluster-api-provider-ibmcloud
enable_providers:
- ibmcloud
- kubeadm-bootstrap
- kubeadm-control-plane
kustomize_substitutions:
  IBMCLOUD_API_KEY: "XXXXXXXXXXXXXXXXXX"
  SERVICE_ENDPOINT: "us-south:vpc=https://us-south-stage01.iaasdev.cloud.ibm.com,powervs=https://dal.power-iaas.test.cloud.ibm.com,rc=https://resource-controller.test.cloud.ibm.com"
  IBMCLOUD_AUTH_URL: "https://iam.test.cloud.ibm.com"
```

### 4.  Configuration to use observability tools

- cluster-api provides support for deploying observability tools, More information about it is available in cluster-api [book](https://cluster-api.sigs.k8s.io/developer/logging#developing-and-testing-logs).

```yaml
default_registry: "gcr.io/you-project-name-here"
deploy_observability:
   - promtail
   - loki
   - grafana
   - prometheus
provider_repos:
  - ../cluster-api-provider-ibmcloud
enable_providers:
  - ibmcloud
  - kubeadm-bootstrap
  - kubeadm-control-plane
kustomize_substitutions:
  IBMCLOUD_API_KEY: "XXXXXXXXXXXXXXXXXX"
  PROVIDER_ID_FORMAT: "v2"
  EXP_CLUSTER_RESOURCE_SET: "true"
extra_args:
   core:
      - "--logging-format=json"
      - "--v=5"
   kubeadm-bootstrap:
      - "--v=5"
      - "--logging-format=json"
   kubeadm-control-plane:
      - "--v=5"
      - "--logging-format=json"
   ibmcloud:
      - "--v=5"
      - "--logging-format=json"
```

**NOTE**: For information about all the fields that can be used in the `tilt-settings.yaml` file, check them [here](https://cluster-api.sigs.k8s.io/developer/tilt.html#tilt-settings-fields).

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
