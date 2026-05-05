# API Modernization and Strict Kubernetes Compliance for PowerVS Resources (v1beta3)

## Metadata
* **Authors:** Karthik-K-N
* **Status:** Implementable
* **Creation Date:** 2026-04-21
* **Target Version:** v1beta3

## Summary
This proposal outlines a structural modernization of the `IBMPowerVSCluster`, `IBMPowerVSMachine`, and `IBMPowerVSImage` Custom Resource Definitions (CRDs) for the `v1beta3` API version. It aims to eliminate Kubernetes API anti-patterns—such as Status-driven lifecycle management, complex map types, ambiguous boolean pointers, and excessive nil-pointer checks—by aligning the API strictly with upstream Kubernetes API conventions and the Kube API Linter (KAL).

## Motivation
As the CAPIBM provider has evolved, the API has accumulated several patterns that make the controller brittle, difficult to maintain, and incompatible with modern Kubernetes features like Server-Side Apply (SSA). 

Currently, our APIs suffer from the following architectural drawbacks:
1. **Leaking Intent into Status:** The `Status` subresource contains a `ControllerCreated *bool` flag for nearly every infrastructure component. The Reconciler relies on this Status flag during the deletion phase to determine if it should delete an IBM Cloud resource. This violates the Kubernetes principle that `Spec` defines intent and `Status` merely reflects observed reality.
2. **The Map Anti-Pattern:** The `IBMPowerVSClusterStatus` uses native Go maps (e.g., `map[string]ResourceReference`) for complex objects like Subnets and Security Groups. Kubernetes explicitly warns against this, as it breaks Server-Side Apply (SSA) merging logic.
3. **The Pointer and Boolean Traps:** We heavily utilize `*bool` and `*string` to distinguish between "unset" and "zero" values. This leads to code littered with `nil` checks. Furthermore, using boolean toggles (like `Public *bool`) is an API design anti-pattern that severely limits future extensibility.
4. **Weak Validation:** User intent is often validated deep inside the Reconciler rather than at the API boundary using CEL (Common Expression Language) and explicit Union Types.

## Goals
* Shift infrastructure ownership intent from `Status.ControllerCreated` to explicit declarative definitions in the `Spec`.
* Eradicate illegal `map[string]T` usages in favor of Server-Side Apply compliant slices (`+listType=map`).
* Remove unnecessary pointers from the API using Go 1.24 `omitzero`.
* Replace restrictive `*bool` toggles with extensible string Enums.
* Enforce KAL-compliant docstrings and strict CEL validations.

## Non-Goals
* Introducing new infrastructure features (e.g., adding support for new IBM Cloud services).
* Modifying the core Cluster API (CAPI) machine contract.

---

## Proposal

### 1. Declarative Intent (Discriminated Unions)
**Current Drawback:** The controller guesses if it should create or adopt a resource (like a VPC) and then writes a `ControllerCreated: true` flag to the `Status` to remember to delete it later.

**Solution:** We introduce a Discriminated Union pattern using `SourceType`. The `Spec` now explicitly forces the user to declare their intent. The `ControllerCreated` flag is completely removed from the `Status`.

*Before:*
```go
// User just provides a name, controller guesses intent.
VPC *ResourceReference `json:"vpc,omitempty"`
```

*After:*
```go
// +kubebuilder:validation:Enum=Reference;Provision
Type SourceType `json:"type,omitempty"`
Reference ResourceIdentifier `json:"reference,omitempty,omitzero"`
Provision VPCProvisionConfig `json:"provision,omitempty,omitzero"`
```
*Deletion Logic Shift:* During deletion, the Reconciler simply checks `if Spec.VPC.Type == SourceTypeProvision` to decide whether to issue a DELETE call to the IBM Cloud API.

### 2. Eliminating Maps for Complex Types
**Current Drawback:** `VPCSubnets map[string]ResourceReference`. Kubernetes SSA cannot deterministically merge changes to complex objects stored in maps.

**Solution:** Convert maps to Arrays/Slices and utilize Kubernetes array-merging tags. We add a `Name` field to `ResourceReference` to act as the correlation key.

*After:*
```go
// vpcSubnets is a list of references to IBM Cloud VPC subnets.
// +optional
// +listType=map
// +listMapKey=name
VPCSubnets []ResourceReference `json:"vpcSubnets,omitempty,omitzero"`
```

### 3. Replacing Booleans with Enums
**Current Drawback:** We use `*bool` fields like `Public` (LoadBalancer) and `Snat` (DHCPServer) to handle 3 states: true, false, and unset (auto). Upstream API best practices strictly prohibit booleans for configuration values because they cannot be easily extended (e.g., if IBM Cloud later adds an "Internal" LoadBalancer type, a boolean breaks).

**Solution:** Convert to explicit string enumerations.

*Before:* `Public *bool`

*After:* 
```go
// +kubebuilder:validation:Enum=Public;Private
Visibility string `json:"visibility,omitempty"`

```

### 4. Eradicating Pointers with `omitzero`
**Current Drawback:** The API is full of `*string` and `*ResourceReference`. This causes `nil` pointer panics in the Reconciler if developers forget to check them. Historically, pointers were required to prevent Go from serializing empty structs into JSON (`{}`).

**Solution:** Leverage Go's `omitzero` JSON tag combined with Kube-API-Linter (KAL) standards. Structs and strings become value types, resulting in much cleaner controller logic.

*Before Controller Logic:*
```go
if cluster.Status.VPC != nil && cluster.Status.VPC.ID != nil {
    return *cluster.Status.VPC.ID
}
```
*After Controller Logic:*
```go
if cluster.Status.VPC.ID != "" {
    return cluster.Status.VPC.ID
}
```

### 5. Explicit Resource Inheritance
**Current Drawback:** For `IBMPowerVSMachine` and `IBMPowerVSImage` if a `ServiceInstance` was omitted, the controller would try to check for ServiceInstance with predifined name (<cluster-name>-serviceInstance). This is functionally dangerous at scale.

**Solution:** We clarify the documentation and Reconciler behavior to explicitly implement CAPI inheritance. If `Network` or `ServiceInstance` is omitted on a Machine/Image, it directly inherits the ID from the parent `IBMPowerVSCluster.Status`.

---

## 6. User Experience (UX) / YAML Examples

### 6.1. Spec: Creating a New VPC vs. Using Existing
**Before (v1beta2):**
```yaml
spec:
  vpc:
    name: "my-vpc" # Controller must guess if it creates this or uses it.
```

**After (v1beta3):**
```yaml
spec:
  vpc:
    type: Provision
    provision:
      name: "my-vpc" # Intent is clear: Create this.
```

### 6.2. Status: SSA-Compliant Subnets
**Before (v1beta2):**
```yaml
status:
  vpcSubnets: # Native map
    subnet-alpha:
      id: "id-1"
```

**After (v1beta3):**
```yaml
status:
  vpcSubnets: # SSA-compliant list
    - name: "subnet-alpha"
      id: "id-1"
```

---

## Upgrade Strategy

Because these changes alter the structural schema of the CRDs (moving from Maps to Arrays, renaming fields, changing Types):

1. **API Version Bump:** These changes will be introduced exclusively in the `v1beta3` API version.
2. **Conversion Webhooks:** We will write a Kubernetes conversion webhook to translate `v1beta2` objects to `v1beta3`. 
    * *Map to Slice:* The webhook will iterate over `v1beta2` maps, extract the string key, map it to the new `Name` field in the slice, and append it.
    * *Boolean to Enum:* `Public: true` will convert to `Visibility: Public`.
    * *Intent:* If `v1beta2` `Status.ControllerCreated` is true, the webhook will set `v1beta3` `Spec.Type` to `Provision`. If false, it will set it to `Reference`.

## Alternatives Considered
* **Keeping Maps in Status:** We considered keeping `map[string]T` to avoid writing a complex conversion webhook. This was rejected because Server-Side Apply is becoming the standard for modern GitOps tools (like ArgoCD and Flux), and maps for complex types are fundamentally incompatible with it.
* **Flattening `ResourceReference`:** We considered flattening `Status.VPC` to just `Status.VPCID string`. This was rejected to maintain forward compatibility; keeping a struct allows us to easily add fields like `CRN` or `State` in the future without breaking the API.