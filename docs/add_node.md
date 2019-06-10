<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Add additional nodes](#add-additional-nodes)
  - [How to add a new node after cluster creation completed](#how-to-add-a-new-node-after-cluster-creation-completed)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Add additional nodes
## How to add a new node after cluster creation completed
after cluster creation with command `clusterctl`, machines can be queried by:
```
# kubectl --kubeconfig=kubeconfig  get machines
NAME                  PROVIDERID              PHASE
jichen-master-g2x5d   ibmcloud:////xxxxxxxx   Running
jichen-node-s9s2b     ibmcloud:////xxxxxxxx   Running
```

In order to add additional node, need create a machine definition,
refer to `examples/ibmcloud/out/machines.yaml` for more info.

Here's a sample and we can put into `~/temp.yaml` for later usage.

```yaml
apiVersion: "cluster.k8s.io/v1alpha1"
kind: Machine
metadata:
  name: jichen-node-12345
  labels:
    set: node
    cluster.k8s.io/cluster-name: "test1"
spec:
  providerSpec:
    value:
      apiVersion: "ibmcloudproviderconfig/v1alpha1"
      kind: "IBMCloudMachineProviderSpec"
      domain: example.com
      flavor: B1_2X4X25
      dataCenter: wdc01
      osReferenceCode: UBUNTU_LATEST
      hourlyBillingFlag: true
      userDataSecret:
        name: worker-user-data
        namespace: ibmcloud-provider-system
  versions:
    kubelet: 1.14.0
```

***NOTE:*** The `generateName` in `examples/ibmcloud/out/machines.yaml` need to be changed
to a user define name because auto generate can't be achieved now.

Then use `kubectl --kubeconfig=kubeconfig apply -f ~/temp.yaml` to create the new machine:

at last, something like below will be created:

```
# kubectl --kubeconfig=kubeconfig  get machines
NAME                  PROVIDERID              PHASE
jichen-master-g2x5d   ibmcloud:////xxxxxxxx   Running
jichen-node-s9s2b     ibmcloud:////xxxxxxxx   Running
jichen-node-12345     ibmcloud:////xxxxxxxx   Running

# kubectl --kubeconfig=kubeconfig  get nodes
NAME                  STATUS   ROLES    AGE   VERSION
jichen-master-g2x5d   Ready    master   17h   v1.14.0
jichen-node-s9s2b     Ready    <none>   17h   v1.14.0
jichen-node-12345     Ready    <none>   33m   v1.14.0
```
