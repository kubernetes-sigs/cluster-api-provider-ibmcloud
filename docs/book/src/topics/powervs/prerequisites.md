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
7. Install `capibmadm` tool (see [here](../capibmadm/index.md#install-capibmadm))

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
    ~ export IBMCLOUD_API_KEY=<API_KEY>
    ~ capibmadm powervs network create --name capi-test --service-instance-id 3229a94c-af54-4212-bf60-6202b6fd0a07 --zone osa21
    ```

    Output:
    ```console
    Creating PowerVS network service-instance-id="3229a94c-af54-4212-bf60-6202b6fd0a07" zone="osa21"
    Successfully created a network networkID="3ee5a1ca-19b4-48c7-a89d-44babdd18703"
    ```

## Import the machine boot image: 

```shell
$ export IBMCLOUD_API_KEY=<API_KEY>
$ pvsadm image import --pvs-instance-id <SERVICE_INSTANCE_ID> -b <BUCKETNAME> --object <OBJECT> --pvs-image-name <POWERVS_IMAGE_NAME> --bucket-region <REGION> --public-bucket
```

e.g:
```shell
$ pvsadm image import --pvs-instance-id 6d892c30-5387-4685-85d0-4999d9c22a8c -b power-oss-bucket --object capibm-powervs-centos-streams8-1-24-2.ova.gz --pvs-image-name capibm-powervs-centos-streams8-1-24-2 --bucket-region us-south --public-bucket
```

For more information about the images can be found at [machine-images](../../machine-images/powervs.md) section
