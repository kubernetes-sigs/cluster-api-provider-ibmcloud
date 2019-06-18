<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Add Taint to masters and nodes](#add-taint-to-masters-and-nodes)
  - [How to add taint to masters and nodes](#how-to-add-taint-to-masters-and-nodes)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Add Taint to masters and nodes
## How to add taint to masters and nodes

Refer to `taints` part below to `machines.yml` on either node or master or both, then start create cluster.

```yaml
apiVersion: "cluster.k8s.io/v1alpha1"
kind: Machine
metadata:
  name: jichen-node-xxxx
  labels:
    set: node
    cluster.k8s.io/cluster-name: "test1"
spec:
  taints:
    - effect: PreferNoSchedule
      key: key2
      value: value2
  providerSpec:
```
