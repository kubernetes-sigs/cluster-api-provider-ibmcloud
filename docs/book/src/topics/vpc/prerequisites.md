# Prerequisites

1. Install `kubectl` (see [here](https://kubernetes.io/docs/tasks/tools/install-kubectl/#install-kubectl-binary-with-curl-on-linux)). Because `kustomize` was included into `kubectl` and it's used by `cluster-api-provider-ibmcloud` in generating yaml files, so version `1.14.0+` of `kubectl` is required, see [integrate kustomize into kubectl](https://github.com/kubernetes/enhancements/issues/633) for more info.
2. You can use either VM, container or existing Kubernetes cluster act as the bootstrap cluster.
   - If you want to use container, install [kind](https://github.com/kubernetes-sigs/kind#installation-and-usage). This is preferred.
   - If you want to use VM, install [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/), version 0.30.0 or greater.
   - If you want to use existing Kubernetes cluster, prepare your kubeconfig.
3. Install a [driver](https://github.com/kubernetes/minikube/blob/master/docs/drivers.md) **if you are using minikube**. For Linux, we recommend kvm2. For MacOS, we recommend VirtualBox.
4. An appropriately configured [Go development environment](https://golang.org/doc/install)
5. Install `clusterctl` tool (see [here](https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl))

### Build workload cluster image:

1. Build a qcow2 image suitable for use as a Kubernetes cluster machine as detailed in the image builder [book](https://image-builder.sigs.k8s.io/capi/providers/raw.html).

    **Note:** Rename the output image to add the `.qcow2` extension. This is required by the next step.


2. Create a VPC Gen2 custom image based on the qcow2 image built in the previous step as detailed in the VPC [documentation](https://cloud.ibm.com/docs/vpc?topic=vpc-planning-custom-images).
