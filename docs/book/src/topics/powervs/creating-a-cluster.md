# Creating a PowerVS cluster

Before running any of the commands below, make sure your management cluster is initialised:

```console
clusterctl init --infrastructure ibmcloud
```

> See [Getting Started](../../getting-started.md) for full management cluster setup.

---

## Option 1 — VirtualIP topology

Uses `kube-vip` on the PowerVS network for control-plane HA. Requires a pre-existing workspace, network, and reserved VIP port — see [VirtualIP prerequisites](./prerequisites.md#option-1--virtualip-additional-prerequisites).

> **Working from main branch?** Replace `--flavor=powervs` with `--from=./templates/cluster-template-powervs.yaml`.

### Required variables

| Variable | Description |
|---|---|
| `IBMPOWERVS_SSHKEY_NAME` | SSH key name registered in the PowerVS workspace |
| `IBMPOWERVS_VIP` | Internal IP address of the reserved port (e.g. `192.168.167.6`) |
| `IBMPOWERVS_VIP_EXTERNAL` | External/floating IP of the reserved port (e.g. `163.68.65.6`) |
| `IBMPOWERVS_VIP_CIDR` | Prefix length of the network subnet (e.g. `29`) |
| `IBMPOWERVS_IMAGE_NAME` | Name of the imported boot image in the workspace |
| `IBMPOWERVS_WORKSPACE_ID` | ID of the existing PowerVS workspace |
| `IBMPOWERVS_NETWORK_NAME` | Name of the existing public network |
| `IBMACCOUNT_ID` | Your IBM Cloud account ID — see [Account settings](https://cloud.ibm.com/account/settings) |
| `IBMPOWERVS_REGION` | PowerVS region (e.g. `osa`) — see [Regions-Zones Mapping](../../reference/regions-zones-mapping.md) |
| `IBMPOWERVS_ZONE` | PowerVS zone (e.g. `osa21`) — see [Regions-Zones Mapping](../../reference/regions-zones-mapping.md) |
| `BASE64_API_KEY` | Base64-encoded IBM Cloud API key: `$(echo -n $IBMCLOUD_API_KEY \| base64)` |

### Deploy the cluster

```console
IBMPOWERVS_SSHKEY_NAME="my-pub-key" \
IBMPOWERVS_VIP="192.168.167.6" \
IBMPOWERVS_VIP_EXTERNAL="163.68.65.6" \
IBMPOWERVS_VIP_CIDR="29" \
IBMPOWERVS_IMAGE_NAME="capibm-powervs-centos-streams10-1-34-7" \
IBMPOWERVS_WORKSPACE_ID="3229a94c-af54-4212-bf60-6202b6fd0a07" \
IBMPOWERVS_NETWORK_NAME="capi-test" \
IBMACCOUNT_ID="ibm-accountid" \
IBMPOWERVS_REGION="osa" \
IBMPOWERVS_ZONE="osa21" \
BASE64_API_KEY=$(echo -n $IBMCLOUD_API_KEY | base64) \
clusterctl generate cluster ibm-powervs-1 \
  --kubernetes-version v1.34.7 \
  --target-namespace default \
  --control-plane-machine-count=3 \
  --worker-machine-count=1 \
  --flavor=powervs | kubectl apply -f -
```

### Optional variables

**Control-plane machine sizing:**
```
IBMPOWERVS_CONTROL_PLANE_MEMORY      (default: 4 GiB)
IBMPOWERVS_CONTROL_PLANE_PROCESSORS  (default: 0.25)
IBMPOWERVS_CONTROL_PLANE_SYSTYPE     (default: s1022)
IBMPOWERVS_CONTROL_PLANE_PROCTYPE    (default: Shared)
```

**Worker machine sizing:**
```
IBMPOWERVS_COMPUTE_MEMORY      (default: 4 GiB)
IBMPOWERVS_COMPUTE_PROCESSORS  (default: 0.25)
IBMPOWERVS_COMPUTE_SYSTYPE     (default: s1022)
IBMPOWERVS_COMPUTE_PROCTYPE    (default: Shared)
```

**API server port:**
```
API_SERVER_PORT  (default: 6443)
```

### ClusterClass variant

To use the [ClusterClass](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/index.html) approach, set `CLUSTER_TOPOLOGY=true` and use `--flavor=powervs-clusterclass`. All the same variables apply, with one addition:

| Variable | Description |
|---|---|
| `IBMPOWERVS_CLUSTER_CLASS_NAME` | Name for the ClusterClass resource (e.g. `powervs-cc`) |

```console
CLUSTER_TOPOLOGY=true \
IBMPOWERVS_CLUSTER_CLASS_NAME="powervs-cc" \
IBMPOWERVS_SSHKEY_NAME="my-pub-key" \
IBMPOWERVS_VIP="192.168.167.6" \
IBMPOWERVS_VIP_EXTERNAL="163.68.65.6" \
IBMPOWERVS_VIP_CIDR="29" \
IBMPOWERVS_IMAGE_NAME="capibm-powervs-centos-streams10-1-34-7" \
IBMPOWERVS_WORKSPACE_ID="3229a94c-af54-4212-bf60-6202b6fd0a07" \
IBMPOWERVS_NETWORK_NAME="capi-test" \
IBMACCOUNT_ID="ibm-accountid" \
IBMPOWERVS_REGION="osa" \
IBMPOWERVS_ZONE="osa21" \
BASE64_API_KEY=$(echo -n $IBMCLOUD_API_KEY | base64) \
clusterctl generate cluster ibm-powervs-1 \
  --kubernetes-version v1.34.7 \
  --target-namespace default \
  --control-plane-machine-count=3 \
  --worker-machine-count=1 \
  --flavor=powervs-clusterclass | kubectl apply -f -
```

> **Working from main branch?** Replace `--flavor=powervs-clusterclass` with `--from=./templates/cluster-template-powervs-clusterclass.yaml`.

---

## Option 2 — LoadBalancer topology

CAPIBM provisions the full infrastructure stack and uses a VPC Load Balancer for control-plane HA. Requires a COS bucket with a DHCP-enabled image — see [LoadBalancer prerequisites](./prerequisites.md#option-2--loadbalancer-additional-prerequisites).

> **Working from main branch?** Replace `--flavor=powervs-create-infra` with `--from=./templates/cluster-template-powervs-create-infra.yaml`.

### Required variables

| Variable | Description |
|---|---|
| `IBMCLOUD_API_KEY` | IBM Cloud API key (also used for `BASE64_API_KEY`) |
| `IBMPOWERVS_SSHKEY_NAME` | SSH key name to inject into VMs |
| `COS_BUCKET_REGION` | Region of the COS bucket containing the boot image (e.g. `us-south`) |
| `COS_BUCKET_NAME` | COS bucket name (e.g. `power-oss-bucket`) |
| `COS_OBJECT_NAME` | DHCP-enabled image object name (e.g. `capibm-powervs-centos-streams10-1-34-7-dhcp.ova.gz`) |
| `IBMACCOUNT_ID` | Your IBM Cloud account ID — see [Account settings](https://cloud.ibm.com/account/settings) |
| `IBMPOWERVS_REGION` | PowerVS region (e.g. `wdc`) — see [Regions-Zones Mapping](../../reference/regions-zones-mapping.md) |
| `IBMPOWERVS_ZONE` | PowerVS zone (e.g. `wdc06`) — see [Regions-Zones Mapping](../../reference/regions-zones-mapping.md) |
| `IBMVPC_REGION` | VPC region to provision into (e.g. `us-east`) |
| `IBM_RESOURCE_GROUP` | IBM Cloud resource group name — see [Resource groups](https://cloud.ibm.com/account/resource-groups) |
| `BASE64_API_KEY` | Base64-encoded IBM Cloud API key: `$(echo -n $IBMCLOUD_API_KEY \| base64)` |

### Deploy the cluster

```console
IBMCLOUD_API_KEY=<API_KEY> \
IBMPOWERVS_SSHKEY_NAME="my-ssh-key" \
COS_BUCKET_REGION="us-south" \
COS_BUCKET_NAME="power-oss-bucket" \
COS_OBJECT_NAME="capibm-powervs-centos-streams10-1-34-7-dhcp.ova.gz" \
IBMACCOUNT_ID="<account_id>" \
IBMPOWERVS_REGION="wdc" \
IBMPOWERVS_ZONE="wdc06" \
IBMVPC_REGION="us-east" \
IBM_RESOURCE_GROUP="ibm-resource-group" \
BASE64_API_KEY=$(echo -n $IBMCLOUD_API_KEY | base64) \
clusterctl generate cluster capi-powervs \
  --kubernetes-version v1.34.7 \
  --target-namespace default \
  --control-plane-machine-count=3 \
  --worker-machine-count=1 \
  --flavor=powervs-create-infra | kubectl apply -f -
```

All infrastructure resources are named `<cluster-name>-<resource>` by default (e.g. `capi-powervs-workspace`, `capi-powervs-vpc`). To override any name, set the corresponding variable before running the command.

---

## Verify the cluster

Once applied, check the status on the management cluster:

**Clusters**
```console
kubectl get clusters
NAME            PHASE
ibm-powervs-1   Provisioned
```

**Control plane**
```console
kubectl get kubeadmcontrolplane
NAME                          INITIALIZED   API SERVER AVAILABLE   VERSION   REPLICAS   READY   UPDATED   UNAVAILABLE
ibm-powervs-1-control-plane   true          true                   v1.34.7   3          3       3
```

**Machines**
```console
kubectl get machines
NAME                                   PROVIDERID                                                         PHASE     VERSION
ibm-powervs-1-control-plane-vzz47      ibmpowervs://ibm-powervs-1/ibm-powervs-1-control-plane-rg6xv      Running   v1.34.7
ibm-powervs-1-md-0-5444cfcbcd-6gg5z   ibmpowervs://ibm-powervs-1/ibm-powervs-1-md-0-dbxb7               Running   v1.34.7
ibm-powervs-1-md-0-5444cfcbcd-7kr9x   ibmpowervs://ibm-powervs-1/ibm-powervs-1-md-0-k7blr               Running   v1.34.7
```

## Deploy a CNI

Retrieve the workload cluster kubeconfig and apply a CNI. Example using Calico:

```console
clusterctl get kubeconfig ibm-powervs-1 > ~/.kube/ibm-powervs-1
export KUBECONFIG=~/.kube/ibm-powervs-1
kubectl apply -f https://docs.projectcalico.org/v3.15/manifests/calico.yaml
```

## Verify workload cluster nodes

```console
kubectl get nodes
NAME                                STATUS   ROLES           AGE   VERSION
ibm-powervs-1-control-plane-rg6xv   Ready    control-plane   41h   v1.34.7
ibm-powervs-1-md-0-4dc5c            Ready    <none>          41h   v1.34.7
ibm-powervs-1-md-0-dbxb7            Ready    <none>          20h   v1.34.7
```
