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

#### Network Provisioning Modes in Option 2

When provisioning a cluster in `LoadBalancer` topology, CAPIBM supports two network provisioning modes via a discriminated union in the API:

1. **`DHCPSubnet` (Recommended):** Creates a plain PowerVS private network with DHCP natively enabled (`EnableDHCP: true`) directly at the PowerVS network layer. This eliminates the need for a dedicated DHCP server virtual service instance, reducing resource consumption, operational complexity, and provisioning time.
2. **`DHCPServer`:** Provisions a dedicated PowerVS DHCP service instance and its managed private network.

In addition, CAPIBM supports advanced networking configurations including **VPC Routing Tables** (with custom ingress routing, next-hop resolution, and Transit Gateway route advertising), **Network Load Balancers** (NLB in Layer 3 `routeMode`), and **VPC Public Gateways** for outbound internet egress.

> **Note:** All provisioned resource names default to `<cluster-name>-<resource>`. You can override individual names by setting the corresponding environment variables. If a resource with the given name already exists in your account, the controller adopts it instead of creating a new one.

---

## SEE ALSO

- [Prerequisites](./prerequisites.md)
- [Creating a cluster](./creating-a-cluster.md)
- [DHCP Subnet & Advanced Networking](./dhcp-subnet-networking.md)
- [Using autoscaler with scaling from 0 machines](./autoscaler-scalling-from-0.md)
