# Development with Tilt

## Overview

This guide sets up a local development environment using [kind](https://kind.sigs.k8s.io) and [Tilt](https://tilt.dev) that hot-reloads controller changes without having to rebuild and redeploy manually.

The setup involves two repositories:

- **`cluster-api-provider-ibmcloud`** — this repository. The `make kind-cluster` command and controller code live here.
- **`cluster-api`** — the upstream CAPI repo. The `tilt-settings.yaml` file and `tilt up` command are run from here.

---

## Prerequisites

1. Container runtime — one of:
   - [Docker](https://docs.docker.com/install/) v19.03 or newer
   - [Podman](https://podman.io/docs/installation) v3.0 or newer (see [Podman setup](#podman-setup) below)
2. [kind](https://kind.sigs.k8s.io) v0.9 or newer
3. [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/)
4. [Tilt](https://docs.tilt.dev/install.html) v0.30.8 or newer
5. [envsubst](https://github.com/drone/envsubst) or similar (for `clusterctl` variable substitution)
6. Both repositories cloned side-by-side:
   ```console
   git clone https://github.com/kubernetes-sigs/cluster-api.git
   git clone https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud.git
   ```

---

## Setup steps

### 1. Create the kind management cluster

Run from the **`cluster-api-provider-ibmcloud`** directory:

```bash
make kind-cluster
```

This creates a local kind cluster and sets it as your active `KUBECONFIG` context. This cluster acts as the CAPI management cluster — it runs the controllers and is used to provision workload clusters on IBM Cloud.

### 2. Create a tilt-settings.yaml

Create `tilt-settings.yaml` in your local **`cluster-api`** directory. This file tells Tilt where to find the IBM Cloud provider and which credentials to use.

Minimum configuration:

```yaml
default_registry: "localhost:5001"
provider_repos:
  - ../cluster-api-provider-ibmcloud
enable_providers:
  - ibmcloud
  - kubeadm-bootstrap
  - kubeadm-control-plane
kustomize_substitutions:
  IBMCLOUD_API_KEY: "<YOUR_API_KEY>"
```

> **Note:** The path `../cluster-api-provider-ibmcloud` assumes both repositories are cloned side-by-side in the same parent directory.

### 3. Start Tilt

Run from the **`cluster-api`** directory:

```bash
tilt up
```

Tilt builds and deploys the controllers onto the kind cluster, then watches for source changes and hot-reloads automatically. Verify the controllers are running:

```bash
kubectl get pods -A
```

---

## Optional configurations

All of the following are additions to the base `tilt-settings.yaml` shown above. Only include the fields relevant to your use case.

### Enable verbose API logging

Logs all PowerVS REST API requests and responses:

```yaml
extra_args:
  ibmcloud:
    - '--v=5'
```

### Enable ClusterClass support

Required when deploying a cluster using `--flavor=powervs-clusterclass`. See [ClusterClass variant](../topics/powervs/creating-a-cluster.md#clusterclass-variant).

```yaml
kustomize_substitutions:
  IBMCLOUD_API_KEY: "<YOUR_API_KEY>"
  CLUSTER_TOPOLOGY: "true"
```

### Use custom service endpoints (staging/test environments)

Set `SERVICE_ENDPOINT` in semi-colon separated format: `${ServiceRegion}:${ServiceID1}=${URL1},${ServiceID2}=${URL2}`:

```yaml
kustomize_substitutions:
  IBMCLOUD_API_KEY: "<YOUR_API_KEY>"
  SERVICE_ENDPOINT: "us-south:vpc=https://us-south-stage01.iaasdev.cloud.ibm.com,powervs=https://dal.power-iaas.test.cloud.ibm.com,rc=https://resource-controller.test.cloud.ibm.com"
  IBMCLOUD_AUTH_URL: "https://iam.test.cloud.ibm.com"
```

### Enable observability tools

Deploys Prometheus, Grafana, Loki, and Promtail alongside the controllers. See [CAPI observability docs](https://cluster-api.sigs.k8s.io/developer/core/logging#developing-and-testing-logs) for more detail.

```yaml
deploy_observability:
  - promtail
  - loki
  - grafana
  - prometheus
extra_args:
  core:
    - "--logging-format=json"
    - "--v=5"
  kubeadm-bootstrap:
    - "--v=5"
    - "--logging-format=json"
  kubeadm-control-plane:
    - "--v=5"
    - "--logging-format=json"
  ibmcloud:
    - "--v=5"
    - "--logging-format=json"
```

> For a full list of supported `tilt-settings.yaml` fields, see the [CAPI Tilt settings reference](https://cluster-api.sigs.k8s.io/developer/core/tilt.html#tilt-settings-fields).

---

## Create workload clusters

With Tilt running, provision a workload cluster using `clusterctl`. See:

- [PowerVS cluster](../topics/powervs/creating-a-cluster.md)
- [VPC cluster](../topics/vpc/creating-a-cluster.md)

---

## Clean up

Delete all workload clusters before tearing down the management cluster:

```bash
# Delete workload clusters first
kubectl delete cluster <cluster-name>

# Stop Tilt (Ctrl-C in the tilt terminal, then)
tilt down

# Delete the kind management cluster
kind delete cluster
```

---

## Podman setup

If you prefer Podman over Docker, complete these steps before running `make kind-cluster`.

**Emulate the Docker CLI** (required for kind to work with Podman):
- [General instructions](https://podman-desktop.io/docs/migrating-from-docker/emulating-docker-cli-with-podman)
- [macOS-specific instructions](https://podman-desktop.io/docs/migrating-from-docker/using-podman-mac-helper)

**1. Initialise and start the Podman machine:**

```bash
podman machine init
podman machine start
```

**2. Configure the local registry as insecure:**

```bash
podman machine ssh
sudo vi /etc/containers/registries.conf
```

Add at the end of the file:

```toml
[[registry]]
location = "localhost:5001"
insecure = true
```

**3. Restart the Podman machine to apply the config:**

```bash
podman machine stop
podman machine start
```
