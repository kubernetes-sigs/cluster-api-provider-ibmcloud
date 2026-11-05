# DHCP Subnet & Advanced Networking in PowerVS

This guide provides a comprehensive runbook and architectural overview of **DHCP Subnet Support** and **Advanced Networking** in Cluster API Provider IBMCloud (CAPIBM) for PowerVS clusters (`v1beta3` API).

---

## 1. Overview: Native DHCP Subnet vs Dedicated DHCP Server

When deploying a PowerVS cluster in `LoadBalancer` topology, virtual machines (VMs) obtain their IP configuration automatically via DHCP during bootstrapping. CAPIBM supports two provisioning mechanisms via a discriminated union under `spec.network.provision`:

```text
DHCPSubnet (Recommended)
┌───────────────────┐      ┌──────────────────────────┐      ┌───────────────────────────┐
│ CAPIBM Controller │ ───> │ PowerVS Private Network  │ ───> │ Control Plane & Worker VMs│
│                   │      │   (with native DHCP)     │      │     (Dynamic IP Leases)   │
└───────────────────┘      └──────────────────────────┘      └───────────────────────────┘

DHCPServer (Legacy)
┌───────────────────┐      ┌──────────────────────────┐      ┌───────────────────────────┐
│ CAPIBM Controller │ ───> │   PowerVS DHCP Server    │ ───> │ Control Plane & Worker VMs│
│                   │      │ (Dedicated Service / VM) │      │     (Dynamic IP Leases)   │
└───────────────────┘      └──────────────────────────┘      └───────────────────────────┘
                                         │
                                         ▼
                           ┌──────────────────────────┐
                           │ PowerVS Private Network  │
                           └──────────────────────────┘
```

### Key Differences

| Feature | `DHCPSubnet` (Native DHCP) | `DHCPServer` (Dedicated Service) |
|---|---|---|
| **API Type** | `network.provision.type: DHCPSubnet` | `network.provision.type: DHCPServer` |
| **Resources Provisioned** | PowerVS network with native DHCP | DHCP service and private network |
| **DHCP Server VM** | No | Yes |
| **SNAT Support** | Handled via external gateway | Optional built-in SNAT |
| **Recommended For** | New cluster installations | Legacies or environments requiring built-in SNAT |

---

## 2. Architecture & Networking Stack

In a full-stack automated deployment (`LoadBalancer` topology), CAPIBM coordinates resources across both IBM Power Systems Virtual Server and IBM Cloud VPC:

```text
+-------------------------------------------------------------+
|               Power Systems Virtual Server                  |
|                                                             |
|   +--------------------------+  +------------------------+  |
|   |    Control Plane VMs     |  |      Worker VMs        |  |
|   +--------------+-----------+  +-----------+------------+  |
|                  |                          |               |
|                  +------------+-------------+               |
|                               |                             |
|                               v                             |
|          +--------------------------------------+           |
|          | DHCP Subnet Network (Native DHCP)    |           |
|          +--------------------------------------+           |
+-------------------------------|-----------------------------+
                                |
                                v  (PowerVS Connection)
+-------------------------------------------------------------+
|                       Transit Gateway                       |
+-------------------------------|-----------------------------+
                                |
                                v  (VPC Connection)
+-------------------------------------------------------------+
|                        IBM Cloud VPC                        |
|                                                             |
|                    +-------------------+                    |
|                    | VPC Routing Table |                    |
|                    +---------+---------+                    |
|                              |                              |
|                              v                              |
|                    +-------------------+                    |
|                    |    VPC Subnet     |                    |
|                    +----+---------+----+                    |
|                         |         |                         |
|             +-----------+         +-----------+             |
|             |                         |       |             |
|             v                         v       v             |
|    +------------------+             +---+   +---+           |
|    | Public Gateway   |             |ALB|   |NLB|           |
|    +--------+---------+             +---+   +---+           |
+-------------|-------------------------|---------------------+
              |                         |
              v                         v
       +---------------------------------------+
       |          Internet / Clients           |
       +---------------------------------------+
```

### Core Components

1. **PowerVS DHCP Subnet (`DHCPSubnet`):**
   A private network created in the PowerVS workspace with native DHCP. Machines attached to this network automatically receive dynamic IP addresses.

2. **IBM Cloud Transit Gateway:**
   Bridges traffic between the PowerVS workspace subnet and the IBM Cloud VPC subnets.

3. **IBM Cloud VPC Load Balancer:**
   Provides high-availability access to the Kubernetes control plane:
   - **Application Load Balancer (`profile: application`):** Standard Layer 7/Layer 4 load balancing for external control plane traffic.
   - **Network Load Balancer (`profile: network-fixed`):** High-throughput Layer 4 load balancing.
   - **Route-Mode Network Load Balancer (`routeMode: Enabled`):** Enables the load balancer to act as a Layer 3 Virtual Network Function (VNF) next-hop router in VPC routing tables.

4. **VPC Routing Tables (`vpcRoutingTables`):**
   Custom VPC routing tables configured with routes targeting the cluster's subnets or load balancers, with ingress policies and automatic next-hop resolution.

5. **VPC Public Gateways (`vpcPublicGateways`):**
   Attached to VPC subnets to provide outbound internet connectivity for cluster workloads and control plane components.

---

## 3. API Specification Reference

### 3.1 Network Provisioning: `DHCPSubnet`

To configure native DHCP subnet provisioning, set `type: DHCPSubnet` under `spec.network.provision`:

```yaml
spec:
  topology: LoadBalancer
  network:
    type: Provision
    provision:
      type: DHCPSubnet
      dhcpSubnet:
        name: "my-cluster-dhcpsubnet"  # Optional: defaults to <cluster-name>-dhcpsubnet
        cidr: "192.168.0.0/24"         # Optional: defaults to PowerVS zone default
        dnsServers:                     # Optional: defaults to ["127.0.0.1"]
        - "8.8.8.8"
        - "9.9.9.9"
```

#### Field Reference for `dhcpSubnet`

| Field | Type | Description |
|---|---|---|
| `name` | string | Name of the PowerVS network. Defaults to `<cluster-name>-dhcpsubnet`. |
| `cidr` | string | IPv4 CIDR block (e.g. `192.168.0.0/24`). |
| `dnsServers` | []string | Up to 2 DNS server IP addresses to assign via DHCP. |

---

### 3.2 Network Provisioning: `DHCPServer` (Alternative)

To provision a dedicated DHCP service instance instead:

```yaml
spec:
  topology: LoadBalancer
  network:
    type: Provision
    provision:
      type: DHCPServer
      dhcpServer:
        name: "my-dhcp-server"         # Optional: defaults to DHCPSERVER<cluster-name>_Private
        cidr: "192.168.0.0/24"         # Optional
        dnsServer: "8.8.8.8"           # Optional: single DNS server IP
        snat: Enabled                  # Optional: "Enabled" or "Disabled" (default: Enabled)
```

---

### 3.3 VPC Routing Tables & Next-Hop Routing

CAPIBM allows configuring VPC Routing Tables with ingress policies and custom routes:

```yaml
spec:
  vpcRoutingTables:
    - type: Provision
      provision:
        name: "my-cluster-routing-table"
        routeTransitGatewayIngress: Enabled   # Route traffic from Transit Gateway
        routeDirectLinkIngress: Disabled
        routeVPCZoneIngress: Disabled
        advertiseRoutesTo:
          - transit_gateway                  # Advertise routes to Transit Gateway
        routes:
          - name: "powervs-to-vpc-egress"
            destination: "10.0.0.0/16"
            zone: "us-east-1"
            action: deliver
            # If nextHop is omitted when action is deliver, CAPIBM automatically resolves
            # the next-hop IP from the first private IP of a provisioned load balancer!
            nextHop: "10.240.0.4"
            advertise: Enabled
```

#### Route Actions
- `deliver`: Delivers the packet to the specified `nextHop` IP address.
- `delegate`: Delegates packet routing to system built-in routing.
- `delegate_vpc`: Delegates packet routing to the VPC default routing table.
- `drop`: Drops the packet.

---

### 3.4 Network Load Balancer (NLB) & Route Mode

You can configure VPC Network Load Balancers with optional Layer 3 route mode:

```yaml
spec:
  loadBalancers:
    - type: Provision
      provision:
        name: "my-cluster-nlb"
        type: Private                        # Public or Private
        profile: network-fixed               # NLB profile
        routeMode: Enabled                   # Enabled: acts as Layer 3 VNF router
        backendPools:
          - name: "egress-pool"
            algorithm: round_robin
            protocol: tcp
            failsafePolicy: bypass           # Send traffic directly to member IPs when unhealthy
            healthMonitor:
              delay: 10
              retries: 3
              timeout: 5
              type: tcp
              port: 80
```

> **Note on Route Mode:** When `routeMode: Enabled`, CAPIBM skips registering VM instances as backend pool members because the load balancer is operating as a routing gateway next hop.

---

### 3.5 VPC Public Gateways

To provide outbound internet access for subnets in a VPC zone:

```yaml
spec:
  vpcPublicGateways:
    - type: Provision
      provision:
        name: "my-cluster-pubgw-zone1"
        zone: "us-east-1"
  subnets:
    - type: Provision
      zone: "us-east-1"
      provision:
        name: "my-cluster-subnet-1"
      publicGateway:
        name: "my-cluster-pubgw-zone1"
```

---

## 4. CLI Operations with `capibmadm`

The `capibmadm` CLI supports creating PowerVS networks with native DHCP enabled using the `--enable-dhcp` flag.

### Create a Native DHCP Subnet via CLI

```console
export IBMCLOUD_API_KEY=<API_KEY>

capibmadm powervs network create \
  --name my-dhcp-net \
  --private \
  --cidr 192.168.0.0/24 \
  --enable-dhcp \
  --dns-servers 8.8.8.8,9.9.9.9 \
  --service-instance-id <WORKSPACE_ID> \
  --zone <ZONE>
```

### List Networks

```console
capibmadm powervs network list \
  --service-instance-id <WORKSPACE_ID> \
  --zone <ZONE>
```

---

## 5. Complete Cluster Example: DHCPSubnet Installation

Below is a complete `IBMPowerVSCluster` manifest demonstrating automated infrastructure provisioning with native `DHCPSubnet`, Transit Gateway, VPC, and Load Balancer:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: powervs-dhcp-cluster
  namespace: default
  labels:
    cluster.x-k8s.io/cluster-name: powervs-dhcp-cluster
spec:
  topology: LoadBalancer
  zone: "wdc06"
  resourceGroup:
    type: Reference
    reference:
      name: "my-resource-group"
  workspace:
    type: Provision
    provision:
      name: "powervs-dhcp-cluster-workspace"
  network:
    type: Provision
    provision:
      type: DHCPSubnet
      dhcpSubnet:
        name: "powervs-dhcp-cluster-dhcpsubnet"
        cidr: "192.168.0.0/24"
        dnsServers:
        - "8.8.8.8"
        - "9.9.9.9"
  vpc:
    type: Provision
    region: "us-east"
    provision:
      name: "powervs-dhcp-cluster-vpc"
  subnets:
  - type: Provision
    provision:
      name: "powervs-dhcp-cluster-vpcsubnet"
  transitGateway:
    type: Provision
    provision:
      name: "powervs-dhcp-cluster-tg"
    vpcConnection:
      type: Provision
    powerVSConnection:
      type: Provision
  loadBalancers:
  - type: Provision
    provision:
      name: "powervs-dhcp-cluster-lb"
      type: Public
---
apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: powervs-dhcp-cluster
  namespace: default
spec:
  clusterNetwork:
    pods:
      cidrBlocks: ["192.168.0.0/16"]
    services:
      cidrBlocks: ["10.128.0.0/12"]
  controlPlaneRef:
    apiGroup: controlplane.cluster.x-k8s.io
    kind: KubeadmControlPlane
    name: powervs-dhcp-cluster-control-plane
  infrastructureRef:
    apiGroup: infrastructure.cluster.x-k8s.io
    kind: IBMPowerVSCluster
    name: powervs-dhcp-cluster
```

---

## 6. VM Bootstrapping & Default Route Handling

When instances boot in a DHCP subnet network, their network interface acquires an IP and subnet configuration from the DHCP service. In some PowerVS DHCP environments, default routes may need to be explicitly ensured during pre-kubeadm bootstrapping if they are not set automatically by the network configuration.

For custom or non-standard templates, you can include the following `preKubeadmCommands` in your `KubeadmControlPlane` and `KubeadmConfigTemplate` resources to ensure the default gateway route is correctly established:

```yaml
preKubeadmCommands:
  - 'GW=$(nmcli -t -f DHCP4.OPTION device show | sed -n "s/.*]:routers = //p" | head -1); [ -z "$GW" ] || ip route show default | grep -q . || ip route add default via "$GW"'
```

---

## 7. Verification & Troubleshooting

### Check Cluster Infrastructure Status

```console
kubectl get ibmpowervscluster powervs-dhcp-cluster -o yaml
```

Verify that `status.network` and `status.initialization.provisioned` indicate readiness:

```yaml
status:
  initialization:
    provisioned: true
  network:
    id: "3ee5a1ca-19b4-48c7-a89d-44babdd18703"
    name: "powervs-dhcp-cluster-dhcpsubnet"
  workspace:
    id: "3229a94c-af54-4212-bf60-6202b6fd0a07"
    name: "powervs-dhcp-cluster-workspace"
```

### Idempotency & Teardown Behavior

- **Idempotency:** When reconciling `DHCPSubnet`, the CAPIBM controller checks if a network matching the requested name already exists. If found, it automatically adopts and records its ID in status.
- **Teardown:** When deleting the cluster (`kubectl delete cluster <name>`), the controller calls `DeleteDHCPNetwork`, deleting the provisioned DHCP subnet network automatically while preserving any referenced external networks.
