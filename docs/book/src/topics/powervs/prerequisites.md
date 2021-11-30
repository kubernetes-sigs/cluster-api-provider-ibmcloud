# Prerequisites

1. Install `kubectl` (see [here](https://kubernetes.io/docs/tasks/tools/install-kubectl/#install-kubectl-binary-with-curl-on-linux)). Because `kustomize` was included into `kubectl` and it's used by `cluster-api-provider-ibmcloud` in generating yaml files, so version `1.14.0+` of `kubectl` is required, see [integrate kustomize into kubectl](https://github.com/kubernetes/enhancements/issues/633) for more info.
2. You can use either VM, container or existing Kubernetes cluster act as the bootstrap cluster.
   - If you want to use container, install [kind](https://github.com/kubernetes-sigs/kind#installation-and-usage). This is preferred.
   - If you want to use VM, install [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/), version 0.30.0 or greater.
   - If you want to use existing Kubernetes cluster, prepare your kubeconfig.
3. Install a [driver](https://github.com/kubernetes/minikube/blob/master/docs/drivers.md) **if you are using minikube**. For Linux, we recommend kvm2. For MacOS, we recommend VirtualBox.
4. An appropriately configured [Go development environment](https://golang.org/doc/install)
5. Install `clusterctl` tool (see [here](https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl))
6. Install `pvsadm` tool (see [here](https://github.com/ppc64le-cloud/pvsadm#installation))
6. Install `ibmcloud` tool (see [here](https://github.com/IBM-Cloud/ibm-cloud-cli-release#downloads))


## **PowerVS Prerequisites**

###	Create an IBM Cloud account.

If you don’t already have one, you need a paid IBM Cloud account to create your Power Systems Virtual Server instance.
To create an account, go to: [cloud.ibm.com](https://cloud.ibm.com).

###	Create an IBM Cloud account API key

Please refer to the following [documentation](https://cloud.ibm.com/docs/account?topic=account-userapikey) to create an API key.


### Create Power Systems Virtual Server Service Instance

After you have an active IBM Cloud account, you can create a Power Systems Virtual Server service. To do so, perform the following steps:

1. ***TO-DO***

## Create Network

A public network is required for your kubernetes cluster. Perform the following steps to create a public network for the Power Systems Virtual Server service instance created in the previous step.

1. Create Public Network

    ```console
    ~ ibmcloud pi network-create-public capi-test --dns-servers "8.8.8.8 9.9.9.9"
    ```

    Output:
    ```console
    Network capi-test created.
                        
    ID                fea9ac26-693d-402b-b22f-aa3d90ed0a31   
    Name              capi-test  
    Type              pub-vlan   
    VLAN              2008   
    CIDR Block        192.168.150.96/29   
    IP Range          [192.168.150.98 192.168.150.102]   
    Public IP Range   [158.175.161.98 158.175.161.102]   
    Gateway           192.168.150.97   
    DNS               8.8.8.8, 9.9.9.9
    ```

## Build workload cluster image: 

1. ***TO-DO***
