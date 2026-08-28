# PowerVS Cluster

IBM Power Systems Virtual Server (PowerVS) is IBM's cloud offering for running workloads on IBM Power hardware. CAPIBM supports provisioning Kubernetes clusters on PowerVS.

## Overview

A PowerVS cluster can be deployed with two different control-plane topologies. The topology determines how the Kubernetes API server endpoint is made highly available, which infrastructure you are responsible for, and which `clusterctl` flavor to use.

---

**Choose your topology before reading further — prerequisites and steps differ between them.**

---

### Option 1 — VirtualIP

> Control-plane HA via **kube-vip** running on the PowerVS network.

You pre-create all PowerVS infrastructure (workspace, public network, port/VIP) and import the machine boot image. CAPIBM only provisions the virtual machines; it references your existing resources.

- Template flavor: `--flavor=powervs`
- ClusterClass variant: `--flavor=powervs-clusterclass`
- Image type required: [standard PowerVS image](../../machine-images/powervs.md)

### Option 2 — LoadBalancer

> Control-plane HA via a **VPC Load Balancer**.

CAPIBM provisions the complete infrastructure stack: PowerVS workspace, VPC, transit gateway, VPC subnets, load balancer, and virtual machines. The machine boot image is imported automatically from a COS bucket you specify.

- Template flavor: `--flavor=powervs-create-infra`
- Image type required: [DHCP-enabled PowerVS image](../../machine-images/powervs.md#dhcp-enabled-images)

> **Note:** All provisioned resource names default to `<cluster-name>-<resource>`. You can override individual names by setting the corresponding environment variables. If a resource with the given name already exists in your account, the controller adopts it instead of creating a new one.

---

## SEE ALSO

- [Prerequisites](./prerequisites.md)
- [Creating a cluster](./creating-a-cluster.md)
- [Using autoscaler with scaling from 0 machines](./autoscaler-scalling-from-0.md)
