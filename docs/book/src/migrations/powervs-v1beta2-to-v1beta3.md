# PowerVS v1beta2 to v1beta3 Migration Guide

## Overview

This guide helps you migrate from v1beta2 to v1beta3 PowerVS APIs. The v1beta3 API introduces significant improvements based on Kubernetes API best practices, including enhanced type safety, explicit intent declaration, and better GitOps compatibility.

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
| 9 | [VPC Security Groups Configuration](#9-vpc-security-groups-configuration) | `IBMPowerVSCluster` |
| 10 | [COS Instance Configuration](#10-cos-instance-configuration) | `IBMPowerVSCluster` |
| 11 | [Ignition Configuration](#11-ignition-configuration) | `IBMPowerVSCluster` |
| 12 | [IBMPowerVSMachine Configuration](#12-ibmpowervsmachine-configuration) | `IBMPowerVSMachine` |
| 13 | [IBMPowerVSImage Configuration](#13-ibmpowervsimage-configuration) | `IBMPowerVSImage` |
| 14 | [Status Field Changes](#14-status-field-changes) | All resources |
| 15 | [Conversion Webhook](#15-conversion-webhook) | All resources |

## What's Changed

The v1beta3 API introduces several major improvements across PowerVS resources. This guide documents the changes for:

- **IBMPowerVSCluster** - Topology, Zone, Resource Group, Workspace, Network, TransitGateway, VPC, VPC Subnets, LoadBalancers, VPC Security Groups, COS Instance, and Ignition configuration
- **IBMPowerVSMachine** - Workspace, Network, Image (Reference/Import), SSH Key, System Type, Processor Type, Processors, Memory, and ProviderID
- **IBMPowerVSImage** - Workspace, Bucket, Object, Region, Storage Type (typed enum), and Delete Policy (typed enum)

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
    type: Reference
    reference:
      name: "my-resource-group"  # Required for LoadBalancer topology
  vpc:
    type: Provision
    region: "us-east"
  # LoadBalancer will be automatically created
```

**Key Points:**
- The `topology` field replaces the annotation-based approach.
- `VirtualIP` topology: PowerVS network with Virtual IP.
- `LoadBalancer` topology: Integrates PowerVS with VPC and LoadBalancer.
- The topology is explicit, required, and discoverable via `kubectl explain`.
- **VirtualIP constraints (CEL-enforced):** When `topology: VirtualIP`, `workspace` must be `type: Reference`, `network` must be `type: Reference`, and `transitGateway` must not be set.
- **LoadBalancer constraints (CEL-enforced):** When `topology: LoadBalancer`, both `zone` and `resourceGroup` (with `id` or `name`) are required.

---

## 2. Zone and Resource Group (Data Type Enhancements)

In v1beta3, structural data types have been flattened to comply with standard Kubernetes API guidelines and prevent runtime errors.

### v1beta2 (Deprecated)
In v1beta2, `Zone` and `ResourceGroup` used Go pointers (`*string` and `*ResourceReference`). This occasionally caused nil-pointer panics in the controller and required complex webhook validations.
```yaml
# v1beta2 — zone as *string, resourceGroup as *IBMPowerVSResourceReference
spec:
  zone: "wdc06"
  resourceGroup:
    id: "my-rg-id"
    name: "my-resource-group"
```

### v1beta3 (New)
In v1beta3, pointers have been removed in favor of strict value types and a structured `ResourceGroupSource`.

```yaml
# v1beta3 — zone as plain string, resourceGroup as ResourceGroupSource
spec:
  zone: "wdc06"
  resourceGroup:
    type: Reference           # Only "Reference" is currently supported
    reference:
      id: "my-rg-id"
      # OR use name
      # name: "my-resource-group"
```

**Key Points:**
- **Pointer-Free Zone:** `zone` is now a standard `string`, preventing nil-pointer exceptions.
- **Zone is immutable:** Once set, `zone` cannot be changed (CEL immutability rule).
- **ResourceGroupSource:** `resourceGroup` now uses a structured `ResourceGroupSource` type with `type` and `reference` fields.
- Only `type: Reference` is supported for `resourceGroup` (provisioning a resource group via the API is not supported).
- **Conditional Validation:** If `topology: LoadBalancer`, CEL rules strictly enforce that both `zone` and `resourceGroup` are provided and non-empty.
- **Graceful Omission:** If `topology: VirtualIP`, `zone` and `resourceGroup` can be safely omitted.

---

## 3. Workspace Configuration

### v1beta2 (Deprecated)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  serviceInstanceID: "3229a94c-af54-4212-bf60-6202b6fd0a07"  # Deprecated flat field
  # OR the newer (but still v1beta2) form:
  serviceInstance:
    id: "3229a94c-af54-4212-bf60-6202b6fd0a07"
    # OR
    name: "my-existing-workspace"
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
- Both `serviceInstanceID` (deprecated flat field) and `serviceInstance` are replaced by the `workspace` field.
- The `type` field explicitly declares your intent (`Reference` or `Provision`).
- **Workspace type is immutable:** Once set, `workspace.type` cannot be changed.
- Use `reference.id` or `reference.name` to identify existing workspaces; exactly one must be specified (CEL-enforced).
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
    cidr: "192.168.0.0/24"      # *string pointer
    dnsServer: "8.8.8.8"        # *string pointer, default "1.1.1.1"
    snat: true                   # *bool pointer, default true
    id: "existing-dhcp-server-id"  # optional: reference existing DHCP server
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
        cidr: "192.168.0.0/24"  # Optional: plain string (no longer a pointer)
        dnsServer: "8.8.8.8"    # Optional: plain string (no longer a pointer)
        snat: Enabled           # Enum: Enabled or Disabled (default: Enabled)
```

**Key Points:**
- The `type` field explicitly declares whether to use an existing or create a new network.
- **Network type is immutable:** Once set, `network.type` cannot be changed.
- The top-level `dhcpServer` field is removed; DHCP configuration is nested under `network.provision.dhcpServer`.
- SNAT is now an enum (`Enabled`/`Disabled`) instead of a `*bool` pointer.
- All DHCP fields (`name`, `cidr`, `dnsServer`) are plain `string` values instead of pointers.
- The `id` field on DHCPServer (to reference an existing server) has been removed; use `network.type: Reference` instead.
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
      name: "my-transit-gateway"       # Optional: defaults to <cluster-name>-tgw
      globalRouting: Global            # Enum: Local or Global (auto-detected if omitted)
    # Optionally control how VPC/PowerVS connections are sourced
    vpcConnection:
      type: Provision
      provision:
        name: "my-vpc-connection"      # Optional: name for the VPC connection
    powerVSConnection:
      type: Provision
      provision:
        name: "my-powervs-connection"  # Optional: name for the PowerVS connection
```

**Option C: Reference Existing Connections within a Provisioned Transit Gateway**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  transitGateway:
    type: Provision
    provision:
      name: "my-transit-gateway"
      globalRouting: Local
    vpcConnection:
      type: Reference
      reference:
        id: "existing-vpc-connection-id"
    powerVSConnection:
      type: Reference
      reference:
        id: "existing-pvs-connection-id"
```

**Key Points:**
- The `type` field (`Reference` / `Provision`) replaces the flat `name`/`id` struct.
- `globalRouting` is now an enum (`Local` / `Global`) instead of a `*bool`.
- When `globalRouting` is omitted, the system automatically selects routing based on PowerVS and VPC regions.
- Individual connections (`vpcConnection`, `powerVSConnection`) can each independently reference an existing connection or provision a new one.
- `TransitGatewayConnectionSource` includes both `type`, `reference`, and `provision.name` fields.
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
    region: "us-east"     # Always required in v1beta3
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
    region: "us-east"     # Always required in v1beta3
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
      id: "subnet-id-123"    # *string pointer
      zone: "us-east-1"      # *string pointer
      cidr: "10.0.0.0/24"    # *string pointer (IPv4 CIDR block)
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
      zone: "us-east-1"    # Optional: plain string (no longer a pointer)
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
- All fields (`id`, `name`, `zone`) are plain value types, not pointers.
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
      id: "lb-id-123"          # *string pointer
      public: true             # *bool pointer — true = public, false = private
      additionalListeners:
        - port: 443
          protocol: TCP
          defaultPoolName: "my-pool"   # *string pointer
          selector:
            matchLabels:
              role: worker
      backendPools:
        - name: "my-pool"      # *string pointer
          algorithm: round_robin
          protocol: tcp
          healthMonitor:
            delay: 10
            retries: 3
            timeout: 5
            type: tcp
            port: 8080         # *int64 pointer (optional)
            urlPath: "/healthz" # *string pointer (optional)
      securityGroups:
        - id: "sg-id-123"
          name: "my-sg"        # VPCResource struct with *string fields
      subnets:
        - id: "subnet-id-123"
          name: "my-subnet"    # VPCResource struct with *string fields
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
            protocol: tcp
            defaultPoolName: "my-pool"  # plain string (no longer a pointer)
            selector:
              matchLabels:
                role: worker
        backendPools:
          - name: "my-pool"   # plain string (no longer a pointer)
            algorithm: round_robin
            protocol: tcp
            healthMonitor:
              delay: 10
              retries: 3
              timeout: 5
              type: tcp
              port: 8080       # plain int64 (no longer a pointer)
              urlPath: "/healthz" # plain string (no longer a pointer)
        securityGroups:
          - id: "sg-id-123"   # ResourceIdentifier: id or name (plain strings)
        subnets:
          - name: "my-subnet" # ResourceIdentifier: id or name (plain strings)
```

**Key Points:**
- `type` (`Reference` / `Provision`) is now required at the top level of each entry.
- The flat `id`/`name` fields on a LoadBalancer entry have moved into `reference` (when `type: Reference`).
- The `public` field (`*bool`) is replaced by `provision.type` enum (`Public` / `Private`), defaulting to `Public`.
- `securityGroups` and `subnets` now use `ResourceIdentifier` (plain `string` fields) instead of the v1beta2 `VPCResource` struct (pointer fields).
- `additionalListeners[].defaultPoolName` changed from `*string` (pointer) to a plain `string`.
- `additionalListeners[].protocol` changed from `*VPCLoadBalancerListenerProtocol` (pointer) to `LoadBalancerListenerProtocol` (value).
- `backendPools[].name` changed from `*string` (pointer) to a plain `string`.
- `healthMonitor.port` changed from `*int64` (pointer) to a plain `int64`.
- `healthMonitor.urlPath` changed from `*string` (pointer) to a plain `string`.
- The controller only deletes load balancers it created (`type: Provision`).

---

## 9. VPC Security Groups Configuration

### v1beta2 (Deprecated)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  vpcSecurityGroups:
    - id: "sg-id-123"          # *string pointer
      name: "my-sg"            # *string pointer
      rules:
        - direction: inbound
          destination:
            protocol: tcp
            portRange:
              minimumPort: 443
              maximumPort: 443
            remotes:
              - remoteType: cidr
                cidrSubnetName: "my-subnet"  # *string pointer
          securityGroupID: "sg-id-123"       # *string pointer
      tags:
        - "env:prod"                         # []*string slice of pointers
```

### v1beta3 (New)

**Option A: Reference an Existing Security Group**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  vpcSecurityGroups:
    - type: Reference
      reference:
        id: "sg-id-123"
        # OR use name
        # name: "my-sg"
```

**Option B: Provision a New Security Group**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  vpcSecurityGroups:
    - type: Provision
      provision:
        name: "my-sg"
        tags:
          - "env:prod"              # []string (no longer []*string)
        rules:
          - direction: inbound
            destination:
              protocol: tcp
              portRange:
                minimumPort: 443
                maximumPort: 443
              remotes:
                - remoteType: cidr
                  cidrSubnetName: "my-subnet"  # plain string (no longer a pointer)
            securityGroupID: "sg-id-123"       # plain string (no longer a pointer)
```

**Key Points:**
- `vpcSecurityGroups` now uses `VPCSecurityGroupSource` with `type` / `reference` / `provision` fields.
- `VPCSecurityGroup.id` and `VPCSecurityGroup.name` were `*string` pointers; now `ResourceIdentifier` uses plain `string` values.
- `VPCSecurityGroupProvision.tags` changed from `[]*string` to `[]string`.
- `VPCSecurityGroupRule.securityGroupID` changed from `*string` to a plain `string`.
- `VPCSecurityGroupRule.destination` / `.source` changed from `*VPCSecurityGroupRulePrototype` (pointer) to `VPCSecurityGroupRulePrototype` (value).
- `VPCSecurityGroupRuleRemote` string fields (`cidrSubnetName`, `address`, `securityGroupName`) changed from `*string` pointers to plain `string` values.

---

## 10. COS Instance Configuration

### v1beta2 (Deprecated)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  cosInstance:
    name: "my-cos-instance"    # Required when create-infra annotation is set and Ignition is used
    bucketName: "my-bucket"    # Required when create-infra annotation is set and Ignition is used
    bucketRegion: "us-south"   # Required when create-infra annotation is set and Ignition is used
```

### v1beta3 (New)

**Option A: Reference an Existing COS Instance**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  cosInstance:
    type: Reference
    bucketName: "my-bucket"    # Required in both Reference and Provision
    bucketRegion: "us-south"   # Required in both Reference and Provision
    reference:
      id: "cos-instance-id"
      # OR use name
      # name: "my-cos-instance"
```

**Option B: Provision a New COS Instance**
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  cosInstance:
    type: Provision
    bucketName: "my-bucket"
    bucketRegion: "us-south"
    provision:
      name: "my-cos-instance"  # Optional: name for the COS instance to create
```

**Key Points:**
- `cosInstance` now uses `COSInstanceSource` with a `type` / `reference` / `provision` structure, consistent with other resources.
- `bucketName` and `bucketRegion` are shared fields present at the top level of `COSInstanceSource` (required regardless of type).
- The flat v1beta2 `CosInstance` struct (with `name`, `bucketName`, `bucketRegion`) is replaced by this structured form.
- The controller only deletes COS instances it created (`type: Provision`).

---

## 11. Ignition Configuration

### v1beta2 (Deprecated)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  ignition:
    version: "3.4"  # +kubebuilder:default="2.3", enum: "2.3","2.4","3.0","3.1","3.2","3.3","3.4"
```

### v1beta3 (New)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSCluster
metadata:
  name: my-cluster
spec:
  ignition:
    version: "3.4"  # enum: "2.3","2.4","3.0","3.1","3.2","3.3","3.4"
```

**Key Points:**
- The `Ignition` struct has the same shape in both versions.
- The `version` field retains the same enum values.
- In v1beta3, the type is now a plain value (not a pointer), consistent with the pointer-free API philosophy.
- **CEL validation:** If `ignition` is set, `cosInstance` must also be configured (required by CEL rule).

---

## 12. IBMPowerVSMachine Configuration

The v1beta2 `IBMPowerVSMachine` spec had several structural issues: dual workspace references (`serviceInstanceID` + `serviceInstance`), dual image references (`image` + `imageRef`), and pointer-based identifiers. v1beta3 unifies all of these.

### 12.1 Complete Spec Comparison

#### v1beta2 (Deprecated)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: IBMPowerVSMachine
metadata:
  name: my-machine
spec:
  # Workspace — two redundant fields:
  serviceInstanceID: "workspace-id-123"   # Deprecated flat field
  serviceInstance:                         # Newer but still v1beta2 form
    id: "workspace-id-123"
    # OR: name: "my-workspace"
    # OR: regex: "workspace-.*"            # regex supported in v1beta2

  # Image — two redundant fields:
  image:                                   # Direct PowerVS image reference
    id: "image-id-123"
    # OR: name: "rhcos-4.14"
    # OR: regex: "rhcos-.*"                # regex supported in v1beta2
  imageRef:                                # Indirect reference via IBMPowerVSImage CRD
    name: "my-ibmpowervsimage"

  # Network
  network:
    id: "network-id-123"
    # OR: name: "my-network"
    # OR: regex: "network-.*"              # regex supported in v1beta2

  # SSH Key
  sshKey: "my-ssh-key"

  # System Type
  # +kubebuilder:validation:Enum:="s922";"e980";"s1022";"e1050";"e1080";""
  systemType: "s922"

  # Processor Type
  # +kubebuilder:validation:Enum:="Dedicated";"Shared";"Capped";""
  processorType: "Shared"

  # Processors (int or string for fractional values)
  processors: "0.25"

  # Memory (GiB)
  memoryGiB: 4

  # ProviderID
  providerID: "ibmpowervs://us-south/my-workspace/my-instance-id"  # *string pointer
```

#### v1beta3 (New)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: IBMPowerVSMachine
metadata:
  name: my-machine
spec:
  # Workspace — single unified field using ResourceIdentifier (id or name, exactly one)
  workspace:
    id: "workspace-id-123"
    # OR (not both): name: "my-workspace"
    # If omitted, workspace is inherited from IBMPowerVSCluster

  # Image — single unified field with explicit type
  image:
    type: Reference         # Use an existing PowerVS image
    reference:
      id: "image-id-123"
      # OR (not both): name: "rhcos-4.14"

  # OR: import via IBMPowerVSImage CRD
  # image:
  #   type: Import
  #   import:
  #     name: "my-ibmpowervsimage"  # name of the IBMPowerVSImage CRD resource

  # Network — uses ResourceIdentifier (id or name, exactly one)
  network:
    id: "network-id-123"
    # OR (not both): name: "my-network"
    # If omitted, network is inherited from IBMPowerVSCluster

  # SSH Key — unchanged
  sshKey: "my-ssh-key"

  # System Type — pattern-based validation (no longer a strict enum)
  # Pattern: ^[a-z][0-9]+$  (e.g. s922, e980, s1022, s1122, e1050, e1080)
  systemType: "s922"

  # Processor Type — same enum values
  # +kubebuilder:validation:Enum:="Dedicated";"Shared";"Capped";""
  processorType: "Shared"

  # Processors — unchanged (int or string for fractional values)
  processors: "0.25"

  # Memory — unchanged
  memoryGiB: 4

  # ProviderID — now a plain string (no longer a *string pointer)
  providerID: "ibmpowervs://us-south/my-workspace/my-instance-id"
```

### 12.2 Workspace Reference

#### v1beta2 (Deprecated)
```yaml
spec:
  serviceInstanceID: "workspace-id-123"  # Deprecated flat string
  # OR
  serviceInstance:
    id: "workspace-id-123"
    # OR: name: "my-workspace"
    # OR: regex: "workspace-.*"   # regex supported in v1beta2
```

#### v1beta3 (New)
```yaml
spec:
  workspace:
    id: "workspace-id-123"
    # OR (not both)
    # name: "my-workspace"
```

**Key Points:**
- Both `serviceInstanceID` and `serviceInstance` are replaced by the single `workspace` field.
- Uses `ResourceIdentifier` type with `id` or `name` (exactly one must be set; CEL-enforced).
- **`regex` is not supported** in v1beta3 `ResourceIdentifier`; use `id` or `name` only.
- If omitted, workspace is inherited from the associated IBMPowerVSCluster.

### 12.3 Image Reference

#### v1beta2 (Deprecated)
```yaml
spec:
  # Option 1: Reference an existing PowerVS image directly
  image:
    id: "image-id-123"
    name: "rhcos-4.14"

  # Option 2: Reference via IBMPowerVSImage CRD (mutually exclusive with image above)
  imageRef:
    name: "my-ibmpowervsimage"
```

#### v1beta3 (New)
```yaml
spec:
  # Option 1: Reference an existing PowerVS image (id or name, exactly one)
  image:
    type: Reference
    reference:
      id: "image-id-123"
      # OR (not both): name: "rhcos-4.14"

  # Option 2: Import via IBMPowerVSImage CRD
  image:
    type: Import
    import:
      name: "my-ibmpowervsimage"  # name of the IBMPowerVSImage CRD resource
```

**Key Points:**
- The dual `image` / `imageRef` fields are unified into a single `image` field with an explicit `type`.
- `type: Reference` replaces the v1beta2 `image` field.
- `type: Import` replaces the v1beta2 `imageRef` field.
- CEL validation ensures `reference` is present when `type: Reference` and `import` is present when `type: Import`.
- `image` is a **required** field in v1beta3.

### 12.4 Network Reference

#### v1beta2 (Deprecated)
```yaml
spec:
  network:                     # Required field in v1beta2
    id: "network-id-123"
    # OR: name: "my-network"
    # OR: regex: "network-.*"  # regex supported in v1beta2
```

#### v1beta3 (New)
```yaml
spec:
  network:                     # Optional field in v1beta3
    id: "network-id-123"
    # OR (not both): name: "my-network"
```

**Key Points:**
- The `network` field uses `ResourceIdentifier` — the same type as other identifiers in v1beta3.
- `network` is now **optional** in v1beta3 (was required in v1beta2); if omitted, the network is inherited from the cluster.
- Supports `id` or `name` (exactly one must be set; CEL-enforced). **`regex` is not supported** in v1beta3.

### 12.5 ProviderID

#### v1beta2 (Deprecated)
```yaml
spec:
  providerID: "ibmpowervs://us-south/my-workspace/my-instance-id"  # *string (pointer)
```

#### v1beta3 (New)
```yaml
spec:
  providerID: "ibmpowervs://us-south/my-workspace/my-instance-id"  # string (value)
```

**Key Points:**
- `providerID` changed from `*string` (pointer) to a plain `string`.

### 12.6 Machine Status Changes

#### v1beta2 Status
```yaml
status:
  ready: true                   # bool
  instanceID: "my-instance-id"
  addresses:
    - type: InternalIP
      address: "192.168.0.10"
  health: "OK"
  instanceState: "ACTIVE"
  fault: "some fault message"   # Removed in v1beta3
  failureReason: "..."          # Deprecated, removed in v1beta3
  failureMessage: "..."         # Deprecated, removed in v1beta3
  region: "us-south"            # *string pointer
  zone: "us-south-1"            # *string pointer
  conditions:
    - type: Ready
      status: "True"
  v1beta2:
    conditions:
      - type: Ready
        status: "True"
```

#### v1beta3 Status
```yaml
status:
  conditions:
    - type: Ready               # Promoted to top-level []metav1.Condition
      status: "True"
  initialization:
    provisioned: true           # Replaces the ready bool field
  instanceID: "my-instance-id"
  addresses:
    - type: InternalIP
      address: "192.168.0.10"
  health: "OK"
  instanceState: "ACTIVE"
  region: "us-south"            # plain string (no longer a pointer)
  zone: "us-south-1"            # plain string (no longer a pointer)
  deprecated:
    v1beta2:
      conditions:               # Deprecated v1beta1-style conditions
        - type: Ready
          status: "True"
```

**Key Points:**
- `ready bool` is replaced by `initialization.provisioned *bool`.
- `fault`, `failureReason`, `failureMessage` have been removed.
- `region` and `zone` changed from `*string` pointers to plain `string` values.
- Top-level `conditions` are now `[]metav1.Condition` (metav1 structured conditions).
- Old-style `Conditions` (clusterv1beta1) are moved to `deprecated.v1beta2.conditions`.

---

## 13. IBMPowerVSImage Configuration

### 13.1 Complete Spec Comparison

#### v1beta2 (Deprecated)
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: IBMPowerVSImage
metadata:
  name: my-image
  namespace: default
spec:
  clusterName: "my-cluster"

  # Workspace — two redundant fields:
  serviceInstanceID: "workspace-id-123"   # Deprecated flat field
  serviceInstance:                         # Newer but still v1beta2 form
    id: "workspace-id-123"
    # OR: name: "my-workspace"

  # COS Source
  bucket: "my-cos-bucket"        # *string pointer, required
  object: "rhcos-image.ova.gz"   # *string pointer, required
  region: "us-south"             # *string pointer, required

  # Storage Type — plain string with enum validation
  # +kubebuilder:default=tier1
  # +kubebuilder:validation:Enum=tier0;tier1;tier3
  storageType: "tier1"

  # Delete Policy — plain string with enum validation
  # +kubebuilder:default=delete
  # +kubebuilder:validation:Enum=delete;retain
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
  clusterName: "my-cluster"   # plain string with MinLength/MaxLength validation

  # Workspace — single unified ResourceIdentifier field
  workspace:
    id: "workspace-id-123"
    # OR: name: "my-workspace"
    # If omitted, workspace is derived from the IBMPowerVSCluster status

  # COS Source — now plain strings (no longer *string pointers)
  bucket: "my-cos-bucket"        # plain string, required
  object: "rhcos-image.ova.gz"   # plain string, required
  region: "us-south"             # plain string, required

  # Storage Type — now a typed enum (PowerVSStorageType)
  # Values: tier0, tier1, tier3
  storageType: tier1

  # Delete Policy — now a typed enum (PowerVSImageDeletePolicy)
  # Values: delete, retain
  deletePolicy: delete
```

### 13.2 Workspace Reference

#### v1beta2 (Deprecated)
```yaml
spec:
  serviceInstanceID: "workspace-id-123"  # Deprecated flat string
  # OR
  serviceInstance:
    id: "workspace-id-123"
    name: "my-workspace"
```

#### v1beta3 (New)
```yaml
spec:
  workspace:
    id: "workspace-id-123"
    # OR
    name: "my-workspace"
```

**Key Points:**
- Both `serviceInstanceID` and `serviceInstance` are replaced by the single `workspace` field using `ResourceIdentifier`.
- If `workspace` is omitted, the workspace is automatically derived from the associated IBMPowerVSCluster's status.

### 13.3 COS Source Fields

#### v1beta2 (Deprecated)
```yaml
spec:
  bucket: "my-cos-bucket"      # *string pointer
  object: "rhcos-image.ova.gz" # *string pointer
  region: "us-south"           # *string pointer
```

#### v1beta3 (New)
```yaml
spec:
  bucket: "my-cos-bucket"      # plain string, MinLength=1, MaxLength=63
  object: "rhcos-image.ova.gz" # plain string, MinLength=1, MaxLength=1024
  region: "us-south"           # plain string, MinLength=1, MaxLength=32
```

**Key Points:**
- All three COS fields changed from `*string` pointers to plain `string` values.
- v1beta3 adds explicit `MinLength` / `MaxLength` validation markers on each field.

### 13.4 StorageType and DeletePolicy

#### v1beta2 (Deprecated)
```yaml
spec:
  storageType: "tier1"   # plain string — validated by +kubebuilder:validation:Enum=tier0;tier1;tier3
  deletePolicy: "delete" # plain string — validated by +kubebuilder:validation:Enum=delete;retain
```

#### v1beta3 (New)
```yaml
spec:
  storageType: tier1   # PowerVSStorageType — a named Go type: tier0 | tier1 | tier3
  deletePolicy: delete # PowerVSImageDeletePolicy — a named Go type: delete | retain
```

**Key Points:**
- `storageType` changed from an untyped `string` to the named type `PowerVSStorageType`.
- `deletePolicy` changed from an untyped `string` to the named type `PowerVSImageDeletePolicy`.
- The allowed values remain the same (`tier0`, `tier1`, `tier3` and `delete`, `retain` respectively).
- Typed enums provide better discoverability via `kubectl explain` and improved code safety.

### 13.5 Image Status Changes

#### v1beta2 Status
```yaml
status:
  ready: true                    # bool
  imageID: "image-id-123"
  imageState: "active"
  jobID: "job-id-123"
  conditions:
    - type: Ready
      status: "True"
  v1beta2:
    conditions:
      - type: Ready
        status: "True"
```

#### v1beta3 Status
```yaml
status:
  conditions:
    - type: Ready               # Promoted to top-level []metav1.Condition
      status: "True"
  imageID: "image-id-123"       # with MinLength/MaxLength validation
  imageState: "active"          # with MinLength/MaxLength validation
  jobID: "job-id-123"           # with MinLength/MaxLength validation
  deprecated:
    v1beta2:
      conditions:               # Deprecated v1beta1-style conditions
        - type: Ready
          status: "True"
```

**Key Points:**
- `ready bool` is removed; readiness is conveyed through `conditions`.
- Top-level `conditions` are now `[]metav1.Condition`.
- Old-style `Conditions` moved to `deprecated.v1beta2.conditions`.
- `imageID`, `imageState`, and `jobID` now have explicit `MinLength` / `MaxLength` validation markers.

---

## 14. Status Field Changes

### IBMPowerVSCluster v1beta2 Status
```yaml
status:
  ready: false                              # bool
  resourceGroupID:
    id: "rg-id"
    controllerCreated: true                 # *bool pointer
  serviceInstance:
    id: "workspace-id"
    controllerCreated: true
  network:
    id: "network-id"
    controllerCreated: true
  dhcpServer:
    id: "dhcp-id"
    controllerCreated: true
  vpc:
    id: "vpc-id"
    controllerCreated: true
  vpcSubnet:                                # map[string]ResourceReference
    us-east-1:
      id: "subnet-id"
      controllerCreated: true
  vpcSecurityGroups:                        # map[string]VPCSecurityGroupStatus
    my-sg:
      id: "sg-id"
      ruleIDs: ["rule-id-1"]
      controllerCreated: true
  transitGateway:
    id: "tgw-id"
    controllerCreated: true
    vpcConnection:
      id: "conn-id"
      controllerCreated: true
    powerVSConnection:
      id: "conn-id"
      controllerCreated: true
  cosInstance:
    id: "cos-id"
    controllerCreated: true
  loadBalancers:                            # map[string]VPCLoadBalancerStatus
    my-lb:
      id: "lb-id"
      hostname: "my-lb.example.com"        # *string pointer
      controllerCreated: true
  conditions:
    - type: Ready
      status: "True"
  v1beta2:
    conditions:
      - type: Ready
        status: "True"
```

### IBMPowerVSCluster v1beta3 Status
```yaml
status:
  conditions:                               # Promoted to top-level []metav1.Condition
    - type: Ready
      status: "True"
  initialization:
    provisioned: true                       # Replaces ready bool
  workspace:                               # Renamed from serviceInstance; no controllerCreated
    id: "workspace-id"
    name: "my-workspace"
  network:                                 # No controllerCreated; DHCP nested here
    id: "network-id"
    name: "my-network"
    dhcpServer:
      id: "dhcp-id"
      name: "my-dhcp"
  resourceGroup:
    id: "rg-id"
    name: "my-resource-group"
  vpc:
    id: "vpc-id"
    name: "my-vpc"
    region: "us-east"                      # Added region to VPC status
  vpcSubnets:                              # list (renamed from vpcSubnet map)
    - id: "subnet-id"
      name: "my-subnet"
      zone: "us-east-1"
  vpcSecurityGroups:                       # list (was map[string]VPCSecurityGroupStatus)
    - id: "sg-id"
      name: "my-sg"
      rules:
        - id: "rule-id-1"
  transitGateway:
    id: "tgw-id"
    name: "my-tgw"
    vpcConnection:                         # Connection status includes name and state
      id: "conn-id"
      name: "my-vpc-conn"
      state: "attached"
    powerVSConnection:
      id: "conn-id"
      name: "my-pvs-conn"
      state: "attached"
  cosInstance:
    id: "cos-id"
    name: "my-cos-instance"
    bucketName: "my-bucket"
    bucketRegion: "us-south"
  loadBalancers:                           # list (was map[string]VPCLoadBalancerStatus)
    - name: "my-lb"
      id: "lb-id"
      state: "active"
      hostname: "my-lb.example.com"        # plain string (no longer a pointer)
  deprecated:
    v1beta2:
      conditions:
        - type: Ready
          status: "True"
```

**Key Points:**
- `controllerCreated` removed from all status fields; ownership is determined solely by the `type` field in Spec.
- `ready bool` replaced by `initialization.provisioned *bool`.
- `serviceInstance` renamed to `workspace` in status.
- `dhcpServer` status moved from a top-level field to nested under `network.dhcpServer`.
- `resourceGroupID` field key renamed to `resourceGroup` in status.
- VPC status now includes `region`.
- `vpcSubnet` (keyed map) renamed to `vpcSubnets` (ordered list); each entry includes `zone`.
- `vpcSecurityGroups` changed from `map[string]VPCSecurityGroupStatus` to `[]VPCSecurityGroupStatus`; each entry now exposes `name` and per-rule `id`.
- `TransitGateway` connection status now includes `name` and `state` (e.g., `attached`, `pending`).
- `loadBalancers` changed from `map[string]VPCLoadBalancerStatus` to `[]LoadBalancerStatus`; `hostname` is now a plain `string`.
- `COSInstance` status now exposes `name`, `bucketName`, and `bucketRegion`.
- Top-level `conditions` are `[]metav1.Condition`; old-style conditions moved to `deprecated.v1beta2.conditions`.

---

## 15. Conversion Webhook

The v1beta3 API includes automatic conversion webhooks that handle migration:

- **v1beta2 → v1beta3**: Automatically converts old format to new
  - `Status.ControllerCreated: true` → `Spec.Type: Provision`
  - `Status.ControllerCreated: false` → `Spec.Type: Reference`
  - `serviceInstanceID` / `serviceInstance` → `workspace`
  - `dhcpServer` (top-level) → `network.provision.dhcpServer`
  - Boolean SNAT (`*bool`) → Enum SNAT (`true` → `Enabled`, `false` → `Disabled`)
  - `*bool globalRouting` on TransitGateway → Enum routing (`true` → `Global`, `false` → `Local`)
  - `*bool public` on LoadBalancer → Enum type (`true` → `Public`, `false` → `Private`)
  - Annotation-based topology → Explicit `topology` field
  - `*string zone` → plain `string zone`
  - `*IBMPowerVSResourceReference resourceGroup` → `ResourceGroupSource resourceGroup`
  - `vpcSubnets[]` (flat `Subnet` struct with `*string` fields) → `subnets[]` with `type`/`reference`/`provision`
  - `loadBalancers` keyed map in status → `loadBalancers` list in status
  - `vpcSubnet` keyed map in status → `vpcSubnets` list in status
  - `vpcSecurityGroups` keyed map in status → `vpcSecurityGroups` list in status
  - `image` + `imageRef` (IBMPowerVSMachine) → unified `image` with `type: Reference` or `type: Import`
  - `*string bucket/object/region` (IBMPowerVSImage) → plain `string` fields
  - Untyped `storageType`/`deletePolicy` strings → typed `PowerVSStorageType`/`PowerVSImageDeletePolicy`
  - COS instance flat struct → `COSInstanceSource` with `type`/`reference`/`provision`

- **v1beta3 → v1beta2**: Converts back for compatibility
  - `Spec.Type: Provision` → `Status.ControllerCreated: true`
  - `Spec.Type: Reference` → `Status.ControllerCreated: false`
  - `workspace` → `serviceInstance`
  - `network.provision.dhcpServer` → top-level `dhcpServer`
  - Explicit `topology` field → Annotation-based configuration
  - Enum SNAT → `*bool` SNAT
  - Enum routing → `*bool globalRouting`
  - Enum LB type → `*bool public`
  - Unified `image` (Reference/Import) → `image` + `imageRef` split
  - Typed enums → plain strings for `storageType`/`deletePolicy`

**Note:** While conversion webhooks provide compatibility, it is recommended to migrate to v1beta3 explicitly for better maintainability.

---

## Additional Resources

- [PowerVS Prerequisites](../topics/powervs/prerequisites.md)
- [Creating a PowerVS Cluster](../topics/powervs/creating-a-cluster.md)
- [API References](../reference/api-references.md)
