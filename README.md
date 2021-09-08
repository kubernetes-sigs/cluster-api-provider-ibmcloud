[![LICENSE](https://img.shields.io/badge/license-apache2.0-green.svg)](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/master/LICENSE)
[![Releases](https://img.shields.io/badge/version-v0.0.1-orange.svg)](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/releases "Cluster API provider IBM Cloud latest release")
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-sigs/cluster-api-provider-ibmcloud)](https://goreportcard.com/report/github.com/kubernetes-sigs/cluster-api-provider-ibmcloud)

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Kubernetes Cluster API Provider IBM Cloud](#kubernetes-cluster-api-provider-ibm-cloud)
  - [What is the Cluster API Provider IBM Cloud](#what-is-the-cluster-api-provider-ibm-cloud)
  - [Community, discussion, contribution, and support](#community-discussion-contribution-and-support)
    - [Code of conduct](#code-of-conduct)
  - [Getting Started](#getting-started)
    - [Prerequisites](#prerequisites)
  - [How to provision a simple workload cluster in IBM Cloud VPC Gen2 from local bootstrap cluster](#how-to-provision-a-simple-workload-cluster-in-ibm-cloud-vpc-gen2-from-local-bootstrap-cluster)
    - [Build workload cluster image:](#build-workload-cluster-image)
    - [Provision local boostrap management cluster:](#provision-local-boostrap-management-cluster)
    - [Provision Workload Cluster in IBM Cloud VPC](#provision-workload-cluster-in-ibm-cloud-vpc)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Kubernetes Cluster API Provider IBM Cloud

<a href="https://github.com/kubernetes-sigs/cluster-api"><img src="https://github.com/kubernetes/kubernetes/raw/master/logo/logo.png"  width="100"></a><a href="https://www.ibm.com/cloud/"><img hspace="90px" src="./docs/images/ibm-cloud.svg" alt="Powered by IBM Cloud" height="100"></a>

------

This repository hosts a concrete implementation of an IBM Cloud provider for the [cluster-api project](https://github.com/kubernetes-sigs/cluster-api).

## What is the Cluster API Provider IBM Cloud

The [Cluster API](https://github.com/kubernetes-sigs/cluster-api) brings declarative, Kubernetes-style APIs to cluster creation, configuration and management. The API itself is shared across multiple cloud providers allowing for true IBM Cloud hybrid deployments of Kubernetes.

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page](http://kubernetes.io/community/).

You can reach the maintainers of this project at:

- [#provider-ibmcloud on Kubernetes Slack](https://kubernetes.slack.com/messages/provider-ibmcloud)
- [SIG-Cluster-Lifecycle Mailing List](https://groups.google.com/forum/#!forum/kubernetes-sig-cluster-lifecycle)

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

------

## Getting Started

### Prerequisites

1. Install `kubectl` (see [here](https://kubernetes.io/docs/tasks/tools/install-kubectl/#install-kubectl-binary-with-curl-on-linux)). Because `kustomize` was included into `kubectl` and it's used by `cluster-api-provider-ibmcloud` in generating yaml files, so version `1.14.0+` of `kubectl` is required, see [integrate kustomize into kubectl](https://github.com/kubernetes/enhancements/issues/633) for more info.
2. You can use either VM, container or existing Kubernetes cluster act as the bootstrap cluster.
   - If you want to use container, install [kind](https://github.com/kubernetes-sigs/kind#installation-and-usage). This is preferred.
   - If you want to use VM, install [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/), version 0.30.0 or greater.
   - If you want to use existing Kubernetes cluster, prepare your kubeconfig.
3. Install a [driver](https://github.com/kubernetes/minikube/blob/master/docs/drivers.md) **if you are using minikube**. For Linux, we recommend kvm2. For MacOS, we recommend VirtualBox.
4. An appropriately configured [Go development environment](https://golang.org/doc/install)
5. Install `clusterctl` tool (see [here](https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl))

## How to provision a simple workload cluster in IBM Cloud VPC Gen2 from local bootstrap cluster

### Build workload cluster image:

1. Build a qcow2 image suitable for use as a Kubernetes cluster machine as detailed in the image builder [book](https://image-builder.sigs.k8s.io/capi/providers/raw.html).

    **Note:** Rename the output image to add the `.qcow2` extension. This is required by the next step.


2. Create a VPC Gen2 custom image based on the qcow2 image built in the previous step as detailed in the VPC [documentation](https://cloud.ibm.com/docs/vpc?topic=vpc-planning-custom-images).

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

2. Apply IBM VPC CAPI CRDs

    ```console
    ~ kubectl apply -f config/crd/bases
    ```

    Output:
    ```console
    customresourcedefinition.apiextensions.k8s.io/ibmvpcclusters.infrastructure.cluster.x-k8s.io created
    customresourcedefinition.apiextensions.k8s.io/ibmvpcmachines.infrastructure.cluster.x-k8s.io created
    customresourcedefinition.apiextensions.k8s.io/ibmvpcmachinetemplates.infrastructure.cluster.x-k8s.io created
    ```

3. Initialize local bootstrap cluter as a management cluster

    This cluster will be used to provision a workload cluster in IBM Cloud.

    ```console
    ~ clusterctl init
    ```

    Output:
    ```console
    Fetching providers
    Installing cert-manager Version="v1.1.0"
    Waiting for cert-manager to be available...
    Installing Provider="cluster-api" Version="v0.3.16" TargetNamespace="capi-system"
    Installing Provider="bootstrap-kubeadm" Version="v0.3.16" TargetNamespace="capi-kubeadm-bootstrap-system"
    Installing Provider="control-plane-kubeadm" Version="v0.3.16" TargetNamespace="capi-kubeadm-control-plane-system"

    Your management cluster has been initialized successfully!

    You can now create your first workload cluster by running the following:

      clusterctl config cluster [name] --kubernetes-version [version] | kubectl apply -f -
    ```

### Provision Workload Cluster in IBM Cloud VPC

1. Set workload cluster environment variables

    The sample IAM_ENDPOINT below points to Production and the SERVICE_ENDPOINT points to the `us-east` VPC region. Make sure these values reflect your target VPC environment in IBM Cloud.

    ```console
    export IAM_ENDPOINT=https://iam.cloud.ibm.com/identity/token
    export SERVICE_ENDPOINT=https://us-south.iaas.cloud.ibm.com/v1
    export API_KEY=<YOUR_API_KEY>
    ```

2. Run IBM provider controllers

    The controllers will run against your local management bootstrap cluster.

    ```console
    ~ make run
    ```

3. Provision workload cluster in IBM Cloud

    You can use clusterctl to render the yaml through templates.

    **Note:** the `IBMVPC_IMAGE_ID` value below should reflect the ID of the custom qcow2 image

    ```console
    IBMVPC_REGION=us-south \
    IBMVPC_ZONE=us-south-1 \
    IBMVPC_RESOURCEGROUP=4f15679623607b855b1a27a67f20e1c7 \
    IBMVPC_NAME=ibm-vpc-0 \
    IBMVPC_IMAGE_ID=r134-ea84bbec-7986-4ff5-8489-d9ec34611dd4 \
    IBMVPC_PROFILE=bx2-4x16 \
    IBMVPC_SSHKEY_ID=r134-2a82b725-e570-43d3-8b23-9539e8641944 \
    clusterctl config cluster ibm-vpc-0 --kubernetes-version v1.19.9 \
    --target-namespace default \
    --control-plane-machine-count=1 \
    --worker-machine-count=2 \
    --from ./templates/cluster-template.yaml | kubectl apply -f -
    ```

    Output:
    ```console
    cluster.cluster.x-k8s.io/ibm-vpc-5 created
    ibmvpccluster.infrastructure.cluster.x-k8s.io/ibm-vpc-5 created
    kubeadmcontrolplane.controlplane.cluster.x-k8s.io/ibm-vpc-5-control-plane created
    ibmvpcmachinetemplate.infrastructure.cluster.x-k8s.io/ibm-vpc-5-control-plane created
    machinedeployment.cluster.x-k8s.io/ibm-vpc-5-md-0 created
    ibmvpcmachinetemplate.infrastructure.cluster.x-k8s.io/ibm-vpc-5-md-0 created
    kubeadmconfigtemplate.bootstrap.cluster.x-k8s.io/ibm-vpc-5-md-0 created
    ```

4. Deploy Container Network Interface (CNI)

    Example: calico
    ```console
    kubectl apply -f https://docs.projectcalico.org/v3.15/manifests/calico.yaml
    ```


5. Check the state of the provisioned cluster and machine objects within the local management cluster

    Clusters
    ```console
    ~ kubectl get clusters
    NAME         PHASE
    ibm-vpc-0    Provisioned
    ```

    Kubeadm Control Plane
    ```console
    ~ kubectl get kubeadmcontrolplane
    NAME                       INITIALIZED   API SERVER AVAILABLE   VERSION   REPLICAS   READY   UPDATED   UNAVAILABLE
    ibm-vpc-0-control-plane    true          true                   v1.19.9   1          1       1
    ```

    Machines
    ```console
    ~ kubectl get machines
    ibm-vpc-0-control-plane-vzz47     ibmvpc://ibm-vpc-0/ibm-vpc-0-control-plane-rg6xv   Running        v1.19.9
    ibm-vpc-0-md-0-5444cfcbcd-6gg5z   ibmvpc://ibm-vpc-0/ibm-vpc-0-md-0-dbxb7            Running        v1.19.9
    ibm-vpc-0-md-0-5444cfcbcd-7kr9x   ibmvpc://ibm-vpc-0/ibm-vpc-0-md-0-k7blr            Running        v1.19.9
    ```

6.  Check the state of the newly provisioned cluster within IBM Cloud

    ```console
    ~ clusterctl get kubeconfig ibm-vpc-0 > ~/.kube/ibm-vpc-0
    ~ export KUBECONFIG=~/.kube/ibm-vpc-0
    ~ kubectl get nodes
    NAME                             STATUS   ROLES    AGE   VERSION
    ibm-vpc-0-control-plane-rg6xv    Ready    master   41h   v1.18.15
    ibm-vpc-0-md-0-4dc5c             Ready    <none>   41h   v1.18.15
    ibm-vpc-0-md-0-dbxb7             Ready    <none>   20h   v1.18.15
    ```

7. Experiment with machinedeployment alterations in your management cluster

    With your management *(local)* and workload *(IBM Cloud)* clusters successfully provisioned, you can now experiment with altering the number of machine deployment replicas in your management cluster and see the replica counts reconciled in your workload cluster.

    ```console
    ~ kubectl get machinedeployments
    NAME              PHASE       REPLICAS   READY   UPDATED   UNAVAILABLE
    ibm-vpc-0-md-0    Running     2          2       2

    ~ kubectl scale machinedeployment ibm-vpc-0-md-0 --replicas 3
    ```

    Increase / decrease the `replicas: 2` count in the spec section to see the machine replicas reconciled within the workload cluster.
