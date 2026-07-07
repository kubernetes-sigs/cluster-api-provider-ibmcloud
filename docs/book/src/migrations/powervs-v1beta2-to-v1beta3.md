# PowerVS v1beta2 to v1beta3 Migration Guide

## Overview

This guide helps you migrate from v1beta2 to v1beta3 PowerVS APIs. The v1beta3 API introduces significant improvements based on Kubernetes API best practices, including enhanced type safety, explicit intent declaration, and better GitOps compatibility.

> **Note**: This migration guide covers the currently implemented changes. Additional API improvements will be documented as they are completed.

## Table of Contents

| # | Section | Resource |
|---|---------|----------|
| 1 | [Cluster Topology](#1-cluster-topology) | `IBMPowerVSCluster` |
| 2 | [Zone and Resource Group](#2-zone-and-resource-group-data-type-enhancements) | `IBMPowerVSCluster` |
| 3 | [Workspace Configuration](#3-workspace-configuration) | `IBMPowerVSCluster` |
| 4 | [Network Configuration](#4-network-configuration) | `IBMPowerVSCluster` |
| 5 | [TransitGateway Configuration](#5-transitgateway-configuration) | `IBMPowerVSCluster` |
| 6 | [VPC Configuration](#6-vpc-configuration) | `IBMPowerVSCluster` |
| 7 | [VPC Subnet Configuration](#7-vpc-subnet-configuration) | `IBMPowerVSCluster` |
| 8 | [LoadBalancer Configuration](#8-loadbalancer-configuration) | `IBMPowerVSCluster` |
| 9 | [IBMPowerVSMachine Configuration](#9-ibmpowervsmachine-configuration) | `IBMPowerVSMachine` |
| 10 | [IBMPowerVSImage Configuration](#10-ibmpowervsimage-configuration) | `IBMPowerVSImage` |
| 11 | [Status Field Changes](#status-field-changes) | All resources |
| 12 | [Conversion Webhook](#conversion-webhook) | All resources |

## What's Changed

The v1beta3 API introduces several major improvements across PowerVS resources. This guide documents the changes for:

- **IBMPowerVSCluster** - Topology, Zone, Resource Group, Workspace, Network, TransitGateway, VPC, VPC Subnets, and LoadBalancer configuration
- **IBMPowerVSMachine** - Workspace and Network references
- **IBMPowerVSImage** - Workspace reference

Each section below provides detailed before/after examples and migration guidance.

---

## 1. Cluster Topology

### v1beta2 (Deprecated)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
  annotations:
    powervs.cluster.x-k8s.io/create-infra: "true"  # Annotation-based
spec:
  # Configuration implied by annotation
```

### v1beta3 (New)

**Option A: VirtualIP Topology (PowerVS)**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  topology: VirtualIP  # Explicit topology declaration
  # No VPC/LoadBalancer configuration needed
```

**Option B: LoadBalancer Topology (PowerVS + VPC)**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  topology: LoadBalancer  # Explicit topology declaration
  zone: "wdc06"           # Required for LoadBalancer topology
  resourceGroup:
    name: "my-resource-group"  # Required for LoadBalancer topology
  vpc:
    name: "my-vpc"
    region: "us-east"
  # LoadBalancer will be automatically created
```

**Key Points:**
- The `topology` field replaces the annotation-based approach.
- `VirtualIP` topology: PowerVS network with Virtual IP.
- `LoadBalancer` topology: Integrates PowerVS with VPC and LoadBalancer.
- The topology is explicit, required, and discoverable via `kubectl explain`.

---

## 2. Zone and Resource Group (Data Type Enhancements)

In v1beta3, structural data types have been flattened to comply with standard Kubernetes API guidelines and prevent runtime errors.

### v1beta2 (Deprecated)
In v1beta2, `Zone` and `ResourceGroup` used Go pointers (`*string` and `*ResourceReference`). This occasionally caused nil-pointer panics in the controller and required complex webhook validations.
```yaml
# v1beta2 internal representation allowed nulls
zone: "wdc06" # evaluated as *string
```

### v1beta3 (New)
In v1beta3, pointers have been removed in favor of strict value types (`string`).

**Key Points:**
- **Pointer-Free API:** `zone` is now a standard string, preventing nil-pointer exceptions.
- **Conditional Validation:** If `topology: LoadBalancer` is selected, Kubernetes native CEL (Common Expression Language) rules will strictly enforce that both `zone` and `resourceGroup` are provided and not empty.
- **Graceful Omission:** If `topology: VirtualIP` is selected, `zone` and `resourceGroup` can be safely omitted from the YAML without causing schema errors.

---

## 3. Workspace Configuration

### v1beta2 (Deprecated)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  serviceInstanceID: "3229a94c-af54-4212-bf60-6202b6fd0a07"
```

### v1beta3 (New)

**Option A: Reference an Existing Workspace**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  workspace:
    type: Reference
    reference:
      id: "3229a94c-af54-4212-bf60-6202b6fd0a07"
      # OR use name instead of id
      # name: "my-existing-workspace"
```

**Option B: Provision a New Workspace**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  workspace:
    type: Provision
    provision:
      name: "my-new-workspace"  # Optional: defaults to <cluster-name>-workspace
```

**Key Points:**
- The `type` field explicitly declares your intent (Reference or Provision).
- Use `reference.id` or `reference.name` to identify existing workspaces.
- When provisioning, the workspace name is optional and defaults to `<cluster-name>-workspace`.
- The controller will only delete workspaces it created (when `type: Provision`).

---

## 4. Network Configuration

### v1beta2 (Deprecated)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  network:
    name: "capi-test"
  dhcpServer:
    name: "DHCPSERVER-capi-test"
    snat: true  # Boolean pointer
```

### v1beta3 (New)

**Option A: Reference an Existing Network**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  network:
    type: Reference
    reference:
      name: "capi-test"
      # OR use id instead of name
      # id: "network-id-12345"
```

**Option B: Provision a New Network with DHCP Server**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  network:
    type: Provision
    provision:
      dhcpServer:
        name: "my-dhcp-server"  # Optional: defaults to DHCPSERVER<cluster-name>_Private
        cidr: "192.168.0.0/24"  # Optional
        dnsServer: "8.8.8.8"    # Optional
        snat: Enabled           # Enum: Enabled or Disabled (default: Enabled)
```

**Key Points:**
- The `type` field explicitly declares whether to use an existing or create a new network.
- SNAT is now an enum (`Enabled`/`Disabled`) instead of a boolean pointer.
- DHCP server configuration is only valid when `type: Provision`.
- The controller will only delete networks it created.

---

## 5. TransitGateway Configuration

### v1beta2 (Deprecated)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  transitGateway:
    name: "my-transit-gateway"   # OR use id
    id: "tgw-id-123"
    globalRouting: true          # *bool pointer — true = Global, false = Local
```

### v1beta3 (New)

**Option A: Reference an Existing Transit Gateway**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  transitGateway:
    type: Reference
    reference:
      id: "tgw-id-123"
      # OR use name instead of id
      # name: "my-transit-gateway"
```

**Option B: Provision a New Transit Gateway**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  transitGateway:
    type: Provision
    provision:
      name: "my-transit-gateway"   # Optional: defaults to <cluster-name>-tgw
      globalRouting: Global        # Enum: Local or Global (auto-detected if omitted)
    # Optionally control how VPC/PowerVS connections are sourced
    vpcConnection:
      type: Provision
    powerVSConnection:
      type: Provision
```

**Key Points:**
- The `type` field (`Reference` / `Provision`) replaces the flat `name`/`id` struct.
- `globalRouting` is now an enum (`Local` / `Global`) instead of a `*bool`.
- When omitted, the system automatically selects routing based on PowerVS and VPC regions.
- Individual connections (`vpcConnection`, `powerVSConnection`) can each independently reference an existing connection or provision a new one.
- The controller only deletes Transit Gateways and connections it created (`type: Provision`).

---

## 6. VPC Configuration

### v1beta2 (Deprecated)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  vpc:
    id: "vpc-id-123"      # OR use name
    name: "my-vpc"
    region: "us-east"     # Required only when create-infra annotation is set
```

### v1beta3 (New)

**Option A: Reference an Existing VPC**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  vpc:
    type: Reference
    region: "us-east"     # Required
    reference:
      id: "vpc-id-123"
      # OR use name instead of id
      # name: "my-vpc"
```

**Option B: Provision a New VPC**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  vpc:
    type: Provision
    region: "us-east"     # Required
    provision:
      name: "my-new-vpc"  # Optional: defaults to <cluster-name>-vpc
```

**Key Points:**
- `type` (`Reference` / `Provision`) is now required and replaces the implicit behavior of v1beta2.
- `region` is always required in v1beta3 (was only required under the create-infra annotation in v1beta2).
- The `type` field is immutable once set.
- The controller only deletes VPCs it created (`type: Provision`).

---

## 7. VPC Subnet Configuration

### v1beta2 (Deprecated)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  vpcSubnets:
    - name: "my-subnet"
      id: "subnet-id-123"  # OR use name
      zone: "us-east-1"
      cidr: "10.0.0.0/24"  # IPv4 CIDR block
```

### v1beta3 (New)

**Option A: Reference Existing Subnets**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  subnets:                  # Field renamed from vpcSubnets to subnets
    - type: Reference
      zone: "us-east-1"
      reference:
        id: "subnet-id-123"
        # OR use name instead of id
        # name: "my-subnet"
```

**Option B: Provision New Subnets**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  subnets:
    - type: Provision
      zone: "us-east-1"      # Optional: random zone picked if omitted
      provision:
        name: "my-subnet"    # Optional: defaults to <cluster-name>-vpcsubnet-<INDEX>
```

**Key Points:**
- The field was **renamed** from `vpcSubnets` to `subnets`.
- Each entry now requires a `type` field (`Reference` / `Provision`).
- The `cidr` field from the v1beta2 `Subnet` struct has been removed in v1beta3.
- When `type: Provision` and `zone` is omitted, a random zone is selected from those available in the VPC region.
- The controller only deletes subnets it created (`type: Provision`).

---

## 8. LoadBalancer Configuration

### v1beta2 (Deprecated)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  loadBalancers:
    - name: "my-lb"
      id: "lb-id-123"        # OR use name
      public: true           # *bool pointer — true = public, false = private
      additionalListeners:
        - port: 443
          protocol: TCP
          defaultPoolName: "my-pool"  # *string pointer
      backendPools:
        - name: "my-pool"    # *string pointer
          algorithm: round_robin
          protocol: tcp
          healthMonitor:
            delay: 10
            retries: 3
            timeout: 5
            type: tcp
      securityGroups:
        - id: "sg-id-123"
          name: "my-sg"      # VPCResource struct
      subnets:
        - id: "subnet-id-123"
          name: "my-subnet"  # VPCResource struct
```

### v1beta3 (New)

**Option A: Reference an Existing Load Balancer**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  loadBalancers:
    - type: Reference
      reference:
        id: "lb-id-123"
        # OR use name instead of id
        # name: "my-lb"
```

**Option B: Provision a New Load Balancer**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  loadBalancers:
    - type: Provision
      provision:
        name: "my-lb"         # Optional: defaults to <cluster-name>-loadbalancer
        type: Public          # Enum: Public or Private (default: Public)
        additionalListeners:
          - port: 443
            protocol: TCP
            defaultPoolName: "my-pool"  # plain string (no longer a pointer)
        backendPools:
          - name: "my-pool"   # plain string (no longer a pointer)
            algorithm: round_robin
            protocol: tcp
            healthMonitor:
              delay: 10
              retries: 3
              timeout: 5
              type: tcp
        securityGroups:
          - id: "sg-id-123"   # ResourceIdentifier: id or name
        subnets:
          - name: "my-subnet" # ResourceIdentifier: id or name
```

**Key Points:**
- `type` (`Reference` / `Provision`) is now required at the top level of each entry.
- The flat `id`/`name` fields on a LoadBalancer entry have moved into `reference` (when `type: Reference`).
- The `public` field (`*bool`) is replaced by `provision.type` enum (`Public` / `Private`), defaulting to `Public`.
- `securityGroups` and `subnets` now use `ResourceIdentifier` instead of the v1beta2 `VPCResource` struct.
- `additionalListeners[].defaultPoolName` changed from `*string` (pointer) to a plain `string`.
- `backendPools[].name` changed from `*string` (pointer) to a plain `string`.
- The controller only deletes load balancers it created (`type: Provision`).

---

## 9. IBMPowerVSMachine Configuration

### Workspace Reference

#### v1beta2 (Deprecated)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: IBMPowerVSMachine
metadata:
  name: my-machine
spec:
  serviceInstanceID: "workspace-id-123"
  # OR
  serviceInstance:
    name: "my-workspace"
```

#### v1beta3 (New)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSMachine
metadata:
  name: my-machine
spec:
  workspace:
    id: "workspace-id-123"
    # OR use name
    # name: "my-workspace"
```

**Key Points:**
- The `workspace` field uses `ResourceIdentifier` type.
- Supports both `id` and `name` identifiers.
- If omitted, workspace is inherited from the associated IBMPowerVSCluster.
- Simplified from the v1beta2 `serviceInstance` structure.

### Network Reference

#### v1beta2 (Deprecated)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: IBMPowerVSMachine
metadata:
  name: my-machine
spec:
  network:
    name: "my-network"
```

#### v1beta3 (New)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSMachine
metadata:
  name: my-machine
spec:
  network:
    id: "network-id-123"
    # OR use name
    # name: "my-network"
```

**Key Points:**
- The `network` field uses `ResourceIdentifier` type.
- Supports `id` and `name` identifiers.
- Simplified and consistent network reference pattern.

---

## 10. IBMPowerVSImage Configuration

### Workspace Reference

#### v1beta2 (Deprecated)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: IBMPowerVSImage
metadata:
  name: my-image
  namespace: default
spec:
  clusterName: "my-cluster"
  serviceInstanceID: "workspace-id-123"
  # OR
  serviceInstance:
    name: "my-workspace"
  bucket: "my-cos-bucket"
  object: "rhcos-image.ova.gz"
  region: "us-south"
  storageType: "tier1"
  deletePolicy: "delete"
```

#### v1beta3 (New)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSImage
metadata:
  name: my-image
  namespace: default
spec:
  clusterName: "my-cluster"
  workspace:
    id: "workspace-id-123"
    # OR use name
    # name: "my-workspace"
  bucket: "my-cos-bucket"
  object: "rhcos-image.ova.gz"
  region: "us-south"
  storageType: "tier1"  # Options: tier0, tier1, tier3
  deletePolicy: "delete"  # Options: delete, retain
```

**Key Points:**
- Workspace reference changed from `serviceInstanceID`/`serviceInstance` to `workspace` using ResourceIdentifier.
- Workspace can be specified using `id` or `name`.
- If workspace is omitted, it's automatically derived from the associated IBMPowerVSCluster's status.
- Simplified and consistent with other v1beta3 resources.

---

## Status Field Changes

### v1beta2 Status
```yaml
status:
  serviceInstance:
    id: "workspace-id"
    controllerCreated: true     # Boolean pointer in status
  network:
    id: "network-id"
    controllerCreated: true
  vpc:
    id: "vpc-id"
    controllerCreated: true
  vpcSubnet:                    # keyed map
    us-east-1: { id: "subnet-id", controllerCreated: true }
  transitGateway:
    id: "tgw-id"
    controllerCreated: true
    vpcConnection:
      id: "conn-id"
      controllerCreated: true
    powerVSConnection:
      id: "conn-id"
      controllerCreated: true
  loadBalancers:                # keyed map
    my-lb:
      id: "lb-id"
      hostname: "my-lb.example.com"
      controllerCreated: true
```

### v1beta3 Status
```yaml
status:
  workspace:
    id: "workspace-id"
    name: "my-workspace"
  network:
    id: "network-id"
    name: "my-network"
    dhcpServer:
      id: "dhcp-id"
      name: "my-dhcp"
  vpc:
    id: "vpc-id"
    name: "my-vpc"
    region: "us-east"
  vpcSubnets:                   # renamed from vpcSubnet (map) to vpcSubnets (list)
    - id: "subnet-id"
      name: "my-subnet"
      zone: "us-east-1"
  transitGateway:
    id: "tgw-id"
    name: "my-tgw"
    vpcConnection:              # connection status includes state
      id: "conn-id"
      name: "my-vpc-conn"
      state: "attached"
    powerVSConnection:
      id: "conn-id"
      name: "my-pvs-conn"
      state: "attached"
  loadBalancers:                # changed from map to list; hostname is a plain string
    - name: "my-lb"
      id: "lb-id"
      state: "active"
      hostname: "my-lb.example.com"
```

**Key Points:**
- `controllerCreated` removed from all Status fields. Ownership is determined solely by the `type` field in Spec.
- All Status resources expose both `id` and `name` for better observability.
- DHCP server status is nested under network status.
- VPC Status now includes `region`.
- `vpcSubnet` (keyed map) renamed to `vpcSubnets` (ordered list), with `zone` included per entry.
- TransitGateway connection status now includes `state` (e.g., `attached`, `pending`).
- `loadBalancers` changed from a keyed map (`map[string]VPCLoadBalancerStatus`) to an ordered list (`[]LoadBalancerStatus`).

---

## Conversion Webhook

The v1beta3 API includes automatic conversion webhooks that handle migration:

- **v1beta2 → v1beta3**: Automatically converts old format to new
  - `Status.ControllerCreated: true` → `Spec.Type: Provision`
  - `Status.ControllerCreated: false` → `Spec.Type: Reference`
  - Boolean SNAT → Enum SNAT (`true` → `Enabled`, `false` → `Disabled`)
  - `*bool globalRouting` on TransitGateway → Enum routing (`true` → `Global`, `false` → `Local`)
  - `*bool public` on LoadBalancer → Enum type (`true` → `Public`, `false` → `Private`)
  - Annotation-based topology → Explicit `topology` field
  - Pointer strings → Value types (for `zone` and `resourceGroup`)
  - `vpcSubnets[]` (flat Subnet struct) → `subnets[]` with `type`/`reference`/`provision` shape
  - `loadBalancers` keyed map in status → `loadBalancers` list in status
  - `vpcSubnet` keyed map in status → `vpcSubnets` list in status

- **v1beta3 → v1beta2**: Converts back for compatibility
  - `Spec.Type: Provision` → `Status.ControllerCreated: true`
  - `Spec.Type: Reference` → `Status.ControllerCreated: false`
  - Explicit `topology` field → Annotation-based configuration
  - Enum routing → `*bool globalRouting`
  - Enum LB type → `*bool public`

**Note:** While conversion webhooks provide compatibility, it's recommended to migrate to v1beta3 explicitly for better maintainability.

---

## Additional Resources

- [PowerVS Prerequisites](../topics/powervs/prerequisites.md)
- [Creating a PowerVS Cluster](../topics/powervs/creating-a-cluster.md)
- [API References](../reference/api-references.md)