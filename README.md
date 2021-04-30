[![LICENSE](https://img.shields.io/badge/license-apache2.0-green.svg)](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/master/LICENSE)
[![Releases](https://img.shields.io/badge/version-v0.0.1-orange.svg)](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/releases "Cluster API provider IBM Cloud latest release")

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Kubernetes Cluster API Provider IBM Cloud](#kubernetes-cluster-api-provider-ibm-cloud)
  - [What is the Cluster API Provider IBM Cloud](#what-is-the-cluster-api-provider-ibm-cloud)
  - [Community, discussion, contribution, and support](#community-discussion-contribution-and-support)
    - [Code of conduct](#code-of-conduct)
  - [Getting Started](#getting-started)
    - [Prerequisites](#prerequisites)
    - [Cluster Creation](#cluster-creation)

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

### Cluster Creation

1. Create Management Cluster

    Using kind:

    ```
    kind create cluster
    ```

2. Apply CRDs

    ```
    kubectl apply -f config/crd/bases
    ```

    Output:
    ```
    customresourcedefinition.apiextensions.k8s.io/ibmvpcclusters.infrastructure.cluster.x-k8s.io created
    customresourcedefinition.apiextensions.k8s.io/ibmvpcmachines.infrastructure.cluster.x-k8s.io created
    customresourcedefinition.apiextensions.k8s.io/ibmvpcmachinetemplates.infrastructure.cluster.x-k8s.io created
    ```

3. Initialize Management Cluster

    ```
    clusterctl init
    ```

    Output:
    ```
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

4. Set Workload Cluster Environment Variables

   ```

   export IAM_ENDPOINT=https://iam.test.cloud.ibm.com/identity/token
   export SERVICE_ENDPOINT=https://us-east.iaas.cloud.ibm.com/v1
   export API_KEY=<YOUR_API_KEY>

   ```

5. Run Provider Controllers

    ```
    make run
    ```

6. Create a cluster:

    You can use clusterctl to render the yaml through templates.

    ```
    IBMVPC_REGION=us-south \
    IBMVPC_ZONE=us-south-1 \
    IBMVPC_RESOURCEGROUP=4f15679623607b855b1a27a67f20e1c7 \
    IBMVPC_NAME=ibm-vpc-1 \
    IBMVPC_IMAGE_ID=r134-ea84bbec-7986-4ff5-8489-d9ec34611dd4 \
    IBMVPC_PROFILE=bx2-4x16 \
    IBMVPC_SSHKEY_ID=r134-2a82b725-e570-43d3-8b23-9539e8641944 \
    clusterctl config cluster ibm-vpc-1 --kubernetes-version v1.14.3 \
    --target-namespace default \
    --control-plane-machine-count=1 \
    --worker-machine-count=2 \
    --from ./templates/cluster-template.yaml | kubectl apply -f -
    ```

    Output:
    ```
    cluster.cluster.x-k8s.io/ibm-vpc-5 created
    ibmvpccluster.infrastructure.cluster.x-k8s.io/ibm-vpc-5 created
    kubeadmcontrolplane.controlplane.cluster.x-k8s.io/ibm-vpc-5-control-plane created
    ibmvpcmachinetemplate.infrastructure.cluster.x-k8s.io/ibm-vpc-5-control-plane created
    machinedeployment.cluster.x-k8s.io/ibm-vpc-5-md-0 created
    ibmvpcmachinetemplate.infrastructure.cluster.x-k8s.io/ibm-vpc-5-md-0 created
    kubeadmconfigtemplate.bootstrap.cluster.x-k8s.io/ibm-vpc-5-md-0 created
    ```

    **Note:** the image ID field above must currently reference a custom vm that gets imported as a custom image for VSI provisioning in IBM Cloud VPC2.