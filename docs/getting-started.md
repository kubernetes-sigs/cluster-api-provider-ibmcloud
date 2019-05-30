<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Build clusterctl](#build-clusterctl)
  - [Prepare IBM Cloud info](#prepare-ibm-cloud-info)
  - [Cluster Creation](#cluster-creation)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Getting Started

## Prerequisites

1. Install `kubectl` (see [here](http://kubernetes.io/docs/user-guide/prereqs/)). Because `kustomize` was included into `kubectl` and it's used by `cluster-api-provider-ibmcloud` in generating yaml files, so version `1.14.0+` of `kubectl` is required, see [integrate kustomize into kubectl](https://github.com/kubernetes/enhancements/issues/633) for more info.
2. You can use either VM, container or existing Kubernetes cluster act as bootstrap cluster.
   - If you want to use container, install [kind](https://github.com/kubernetes-sigs/kind#installation-and-usage). This is preferred.
   - If you want to use VM, install [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/), version 0.30.0 or greater.
   - If you want to use existing Kubernetes cluster, prepare your kubeconfig.
3. Install a [driver](https://github.com/kubernetes/minikube/blob/master/docs/drivers.md) **if you are using minikube**. For Linux, we recommend kvm2. For MacOS, we recommend VirtualBox.
4. An appropriately configured [Go development environment](https://golang.org/doc/install)


## Build clusterctl

If you have docker installed, you can simply run following command to build clusterctl binary.

```shell
make build-clusterctl
```

If you have golang environment configured, run following command:

```shell
CGO_ENABLED=0 go build -o bin/clusterctl cmd/clusterctl/main.go
```

The clusterctl binary will be output to `bin/clusterctl`.


## Prepare IBM Cloud info

You need IBM Cloud `api username` and `authentication key` in order to create machines, you can follow [this guide](ibmcloud-get-credential.md) to get it.

You need IBM Cloud SSH key name in order to access the created machines, you can follow [this guide](ibmcloud-config-sshkey.md) to get it.

If you already have an SSH key in IBM Cloud, you need to put the SSH private key to `$HOME/.ssh/id_ibmcloud`.

If you don't have an SSH key, you can generate one with `make ssh-key` locally and create it on IBM Cloud.


## Cluster Creation

1. Create an environment file and fill in your IBM Cloud info

    ```shell
    cp env.sh.template env.sh
    vim env.sh
    ```

2. Generate the cluster-api yaml files

    This command will generate yaml files under `_output` directory, if they do not meet your requirements you can update them.

    ```shell
    source env.sh
    make generate-yaml
    ```

3. Create the cluster

    * If you are using kind:

        ```shell
        make create-with-kind

        or

        bin/clusterctl create cluster --provider ibmcloud \
            --bootstrap-type kind \
            -c _output/cluster.yaml \
            -m _output/machines.yaml \
            -p _output/provider-components.yaml
        ```

    * If you are using minikube:

        ```shell
        bin/clusterctl create cluster --provider ibmcloud \
            --bootstrap-type minikube \
            --bootstrap-flags kubernetes-version=v1.12.3 \
            -c _output/cluster.yaml \
            -m _output/machines.yaml \
            -p _output/provider-components.yaml
        ```

    * If you are using existing Kubernetes cluster:

        ```shell
        bin/clusterctl create cluster --provider ibmcloud \
            --bootstrap-cluster-kubeconfig ~/.kube/config \
            -c _output/cluster.yaml \
            -m _output/machines.yaml \
            -p _output/provider-components.yaml
        ```

    Optionally, add a addons.yaml can provide additional add ons in target cluster by append `-a /path/to/your/addons.yaml` in `clusterctl` command.

    Additional advanced flags can be found via help:

    ```shell
    bin/clusterctl create cluster --help
    ```
