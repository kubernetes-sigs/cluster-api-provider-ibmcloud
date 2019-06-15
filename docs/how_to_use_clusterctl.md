<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [IBM Cloud](#ibm-cloud)
  - [How to use clusterctl image in existing kubernetes cluster?](#how-to-use-clusterctl-image-in-existing-kubernetes-cluster)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# IBM Cloud
## How to use clusterctl image in existing kubernetes cluster?
The `clusterctl` image is designed to run independently to provision ibmcloud cluster. We have embedded the `kind` and `kubectl` into `clusterctl` image.

1. Follow [this readme](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/#cluster-creation) to generate `cluster.yaml`, `machines.yaml`, `provider-components.yaml`, and `addons.yaml`

2. Create `serviceAccount` for clusterctl running
    ```
    kubectl apply -f examples/clusterctl/serviceaccount.yaml
    ```
3. Create `configmap` with `cluster.yaml`, `machines.yaml`, `provider-components.yaml`.
    ```
    kubectl create configmap ibmcloud-config --from-file=./cluster.yaml \
        --from-file=./machines.yaml --from-file=./provider-components.yaml
    ```
    Optionally, add a `addons.yaml` can provide additional add-ons in target cluster. You need to create `configmap` with `addons.yaml`
    ```
    kubectl create configmap ibmcloud-config --from-file=./cluster.yaml \
        --from-file=./machines.yaml --from-file=./provider-components.yaml \
        --from-file=./addons.yaml
    ```
    and modify the `examples/clusterctl/job.yaml` to append `-a /examples/addons.yaml` for `clusterctl create` command.

4. Create `secret` to store the ssh private key
    ```
    kubectl create secret generic ibmcloud-ssh-key-secret \ 
        --from-file=id_ibmcloud=/root/.ssh/id_ibmcloud
    ```
5. Create clusterrole and clusterrolebinding by apply the examples/clusterrole.yaml and examples/clusterrolebinding.yaml
    ```
    kubectl apply -f examples/clusterctl/clusterrole.yaml
    kubectl apply -f examples/clusterctl/clusterrolebinding.yaml
    ```
6. Create job to generate ibmcloud cluster
    ```
    kubectl apply -f examples/clusterctl/job.yaml
    ```

7. After the job completes successfully, it creates a secret to store the `kubeconfig`. You can save it to access the remote cluster. for example:
    ```
    kubectl get secret kubeconfig -ojsonpath={.data.kubeconfig} \
        | base64 -d > /tmp/kubeconfig
    ```
8. Now it is ready to access the remote cluster
    ```
    kubectl --kubeconfig /tmp/kubeconfig get ns
    NAME                       STATUS   AGE
    default                    Active   4m11s
    ibmcloud-provider-system   Active   3m52s
    kube-node-lease            Active   4m14s
    kube-public                Active   4m14s
    kube-system                Active   4m14s
    system                     Active   4m11s
    ```