# Prerequisites

## Common prerequisites

These are required for both topologies.

1. Install `kubectl` v1.14.0 or later — see [install guide](https://kubernetes.io/docs/tasks/tools/install-kubectl/#install-kubectl-binary-with-curl-on-linux).
2. A bootstrap cluster (pick one):
   - **kind** (preferred) — see [kind installation](https://github.com/kubernetes-sigs/kind#installation-and-usage)
   - **minikube** v0.30.0+ — see [minikube installation](https://kubernetes.io/docs/tasks/tools/install-minikube/). Requires a [driver](https://minikube.sigs.k8s.io/docs/drivers/); kvm2 on Linux, VirtualBox on macOS.
   - An existing Kubernetes cluster with a valid kubeconfig.
3. Install `clusterctl` — see [install guide](https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl).
4. Install `capibmadm` — see [install guide](../capibmadm/index.md#install-capibmadm).
5. An [IBM Cloud account](https://cloud.ibm.com) (paid, for PowerVS).
6. An [IBM Cloud API key](https://cloud.ibm.com/docs/iam?topic=iam-userapikey).

---

## Option 1 — VirtualIP: additional prerequisites

The VirtualIP topology requires you to create a PowerVS workspace, network, and a reserved port (VIP) before running `clusterctl`. Follow the steps below.

### 1. Create a PowerVS workspace

Create a Power Systems Virtual Server workspace in IBM Cloud. Note the **Workspace ID** — you will need it as `IBMPOWERVS_WORKSPACE_ID`.

> See [IBM Cloud docs](https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-power-iaas-cli-reference-v1#ibmcloud-pi-workspace) for how to create and list workspaces.

### 2. Create a public network

```console
export IBMCLOUD_API_KEY=<API_KEY>
capibmadm powervs network create --name capi-test \
  --service-instance-id <WORKSPACE_ID> \
  --zone <ZONE>
```

Output:
```console
Creating PowerVS network service-instance-id="3229a94c-af54-4212-bf60-6202b6fd0a07" zone="osa21"
Successfully created a network networkID="3ee5a1ca-19b4-48c7-a89d-44babdd18703"
```

The network name (`capi-test` above) becomes your `IBMPOWERVS_NETWORK_NAME`.

### 3. Reserve a port (VIP)

```console
capibmadm powervs port create \
  --network capi-test \
  --description capi-test-port \
  --service-instance-id <WORKSPACE_ID> \
  --zone <ZONE>
```

Then list the port to get the assigned IP addresses:

```console
capibmadm powervs port list \
  --network capi-test \
  --service-instance-id <WORKSPACE_ID> \
  --zone <ZONE>
```

Output:
```console
DESCRIPTION      EXTERNAL IP   IP ADDRESS      MAC ADDRESS         PORT ID                                STATUS
capi-test-port   163.68.65.6   192.168.167.6   fa:16:3e:89:c8:80   c7e7b6e0-0b0d-4a11-a90b-6ea293deb5ac   DOWN
```

From this output:
- `IP ADDRESS` (`192.168.167.6`) → `IBMPOWERVS_VIP`
- `EXTERNAL IP` (`163.68.65.6`) → `IBMPOWERVS_VIP_EXTERNAL`

The CIDR prefix length of the network (`IBMPOWERVS_VIP_CIDR`) can be found from the network details, e.g. `29` for a `/29` subnet.

### 4. Import the machine boot image

Use a **standard** (non-DHCP) image. See [PowerVS Images](../../machine-images/powervs.md) for the list of available images and COS bucket details.

```console
capibmadm powervs image import \
  --service-instance-id <WORKSPACE_ID> \
  --zone <ZONE> \
  --bucket-region <BUCKET_REGION> \
  --object <OBJECT_NAME> \
  --name <IMAGE_NAME> \
  --bucket power-oss-bucket \
  --public-bucket
```

Example:
```console
capibmadm powervs image import \
  --service-instance-id 3229a94c-af54-4212-bf60-6202b6fd0a07 \
  --zone osa21 \
  --bucket-region us-south \
  --object capibm-powervs-centos-streams10-1-34-7.ova.gz \
  --name capibm-powervs-centos-streams10-1-34-7 \
  --bucket power-oss-bucket \
  --public-bucket
```

The image name you choose becomes your `IBMPOWERVS_IMAGE_NAME`.

### 5. Upload an SSH key

```console
capibmadm powervs key create \
  --name my-pub-key \
  --key-path ~/.ssh/id_rsa.pub \
  --service-instance-id <WORKSPACE_ID> \
  --zone <ZONE>
```

The key name becomes your `IBMPOWERVS_SSHKEY_NAME`.

---

## Option 2 — LoadBalancer: additional prerequisites

The LoadBalancer topology only needs an SSH key and a COS bucket containing the DHCP-enabled machine boot image. CAPIBM creates the workspace and all other infrastructure automatically.

### 1. Identify your resource group

Go to **Manage → Account → Account resources → Resource groups** in the IBM Cloud console to get your resource group name. This becomes `IBM_RESOURCE_GROUP`.

### 2. Ensure a DHCP-enabled image is available in COS

Use a **DHCP-enabled** image. See [PowerVS Images with DHCP based network](../../machine-images/powervs.md#dhcp-enabled-images) for the list of available images and COS bucket details.

The COS details you need:
- `COS_BUCKET_NAME` — e.g. `power-oss-bucket`
- `COS_BUCKET_REGION` — e.g. `us-south`
- `COS_OBJECT_NAME` — e.g. `capibm-powervs-centos-streams10-1-34-7-dhcp.ova.gz`

> CAPIBM creates an `IBMPowerVSImage` resource that imports this image from COS into the newly provisioned workspace automatically during cluster creation. You do not need to import it manually.

### 3. Upload an SSH key to IBM Cloud

If you do not already have an SSH key registered in IBM Cloud, you can add one via the [IBM Cloud console](https://cloud.ibm.com/infrastructure/compute/sshKeys). The key name becomes `IBMPOWERVS_SSHKEY_NAME`.
