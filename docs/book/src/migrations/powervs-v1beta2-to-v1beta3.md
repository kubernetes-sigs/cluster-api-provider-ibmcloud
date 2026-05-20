# PowerVS v1beta2 to v1beta3 Migration Guide

## Overview

This guide helps you migrate from v1beta2 to v1beta3 PowerVS APIs. The v1beta3 API introduces significant improvements based on Kubernetes API best practices, including enhanced type safety, explicit intent declaration, and better GitOps compatibility.

> **Note**: This migration guide covers the currently implemented changes. Additional API improvements will be documented as they are completed.

## What's Changed

The v1beta3 API introduces several major improvements across PowerVS resources. This guide documents the changes for:

- **IBMPowerVSCluster** - Topology, Zone, Resource Group, Workspace, and Network configuration
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

## 5. IBMPowerVSMachine Configuration

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

## 6. IBMPowerVSImage Configuration

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
    controllerCreated: true  # Boolean pointer in status
  network:
    id: "network-id"
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
```

**Key Points:**
- `controllerCreated` flag removed from Status.
- Ownership is now determined by the `type` field in Spec.
- Status includes both `id` and `name` for better observability.
- DHCP server status is nested under network status.

---

## Conversion Webhook

The v1beta3 API includes automatic conversion webhooks that handle migration:

- **v1beta2 → v1beta3**: Automatically converts old format to new
  - `Status.ControllerCreated: true` → `Spec.Type: Provision`
  - `Status.ControllerCreated: false` → `Spec.Type: Reference`
  - Boolean SNAT → Enum SNAT
  - Annotation-based topology → Explicit `topology` field
  - Pointer strings → Value types (for `zone` and `resourceGroup`)

- **v1beta3 → v1beta2**: Converts back for compatibility
  - `Spec.Type: Provision` → `Status.ControllerCreated: true`
  - `Spec.Type: Reference` → `Status.ControllerCreated: false`
  - Explicit `topology` field → Annotation-based configuration

**Note:** While conversion webhooks provide compatibility, it's recommended to migrate to v1beta3 explicitly for better maintainability.

---

## Additional Resources

- [PowerVS Prerequisites](../topics/powervs/prerequisites.md)
- [Creating a PowerVS Cluster](../topics/powervs/creating-a-cluster.md)
- [API References](../reference/api-references.md)