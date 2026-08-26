# Code Organization Improvements Aligned with CAPI Guidelines

## Metadata
* **Authors:** Karthik-K-N
* **Status:** Implemented
* **Creation Date:** 2026-07-14
* **Reference:** [CAPI Code Organization Proposal](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20260701-code-organization.md)

## Summary

This proposal documents a set of targeted code organization improvements for
`cluster-api-provider-ibmcloud`, inspired by the upstream CAPI code organization proposal.
The changes address misplaced packages, redundant indirection layers, and an oversized entry
point — all of which accumulated as the codebase grew. Every change is a pure structural
reorganization with no functional impact on reconciliation or webhook behaviour.

## Motivation

The codebase had accumulated several structural issues that made it harder to navigate,
maintain, and extend:

**Fat entry point.** `cmd/main.go` had grown to ~700 lines, mixing flag parsing, manager
construction, and ~150 lines of controller/webhook registration logic. The registration functions
were buried at the bottom of the file, making the file hard to reason about and the entry point
hard to read at a glance.

**Redundant controller facade.** The top-level `controllers/` package re-declared every
reconciler struct field and delegated `SetupWithManager` to the internal reconcilers. Its sole
consumer was `cmd/main.go`. Every field added to an internal reconciler required a matching
change in the alias — double maintenance with zero benefit.

**Webhooks unnecessarily locked in `internal/`.** `internal/webhooks/{powervs,vpc}/` were
registered directly from `cmd/main.go`, making them provider entry points rather than
internal-only code. The CAPI proposal explicitly warns against over-using `/internal` when it
causes improper placement. Kubebuilder's `controller-gen` scans `paths="./..."` for webhook
markers — the filesystem location is irrelevant to code generation.

**Misclassified packages.** Several packages were placed under `pkg/cloud/` despite having no
IBM Cloud SDK dependency:
- `pkg/cloud/pagingutils/` — a generic pagination helper using only `fmt` and `net/url`
- `pkg/cloud/ignition/` — Go type definitions for the Coreos Ignition v2 bootstrap format
- `pkg/cloud/options/` — a global var store for CLI flags (no cloud code)

Other packages were in the wrong part of the tree:
- `internal/genutil/` — a shared transit gateway utility locked behind `internal/` despite
  being used by both a scope and a webhook
- `pkg/version/` — repo-wide binary version info buried inside `pkg/` rather than at the
  module root where CAPI and other providers place it

**`utils` naming anti-pattern.** `pkg/cloud/pagingutils/` used the `utils` suffix, which is
idiomatic in Java/Python but not Go. The Go standard library, CAPI, and our own `pkg/util/record/`
all use the singular form.

## Goals

* Slim `cmd/main.go` by extracting registration logic into clearly-named unexported functions
  (`setupReconcilers`, `setupWebhooks`, `setupChecks`) — matching CAPI's `core/main.go` pattern.
* Remove the `controllers/alias.go` facade entirely.
* Move admission webhook packages out of `internal/` to reflect their true role as
  externally-registered provider entry points.
* Relocate misplaced packages to directories that accurately reflect their nature.
* Follow Go and CAPI naming conventions throughout.

## Non-Goals

* **Renaming `internal/controllers/` to `reconcilers/`.** The CAPI proposal uses a
  `reconcilers/` directory, but we use kubebuilder which scaffolds new controllers directly
  into `internal/controllers/{group}/` (recorded in the `PROJECT` file). Renaming would break
  `kubebuilder create api` for future APIs.

* **Splitting into multiple Go modules.** The upstream proposal introduces nested Go modules
  (`sigs.k8s.io/cluster-api/api`, `sigs.k8s.io/cluster-api/utils`) for its monorepo. As a
  single-provider repository we do not need this split.

* **Any functional changes** to reconciliation logic, webhook validation, or IBM Cloud API
  interactions.

* **Changes to `api/`**, `pkg/cloud/scope/`, or `pkg/cloud/services/` package structures.

---

## Proposal

### Target Directory Structure

After all changes the repository layout becomes:

```
.
├── api/                              # Unchanged — type definitions, Hub/Spoke markers
├── cmd/
│   └── main.go                       # Thin: flag parsing, manager construction,
│                                     #   + setupChecks / setupReconcilers / setupWebhooks
├── config/                           # Unchanged
├── internal/
│   └── controllers/                  # Unchanged — kubebuilder scaffolds here
│       ├── powervs/
│       └── vpc/
├── pkg/
│   ├── bootstrap/
│   │   └── ignition/                 # Moved from pkg/cloud/ignition/
│   ├── cloud/
│   │   ├── endpoints/                # Unchanged
│   │   ├── options/                  # Unchanged
│   │   ├── scope/                    # Unchanged
│   │   ├── services/                 # Unchanged
│   │   └── util/                     # Moved from internal/genutil/
│   └── util/
│       ├── paging/                   # Moved from pkg/cloud/pagingutils/
│       └── record/                   # Unchanged
├── version/
│   └── version.go                    # Moved from pkg/version/
└── webhooks/                         # Moved from internal/webhooks/
    ├── powervs/
    │   └── admission/
    └── vpc/
        └── admission/
```

---

### Slim `cmd/main.go` — registration functions stay in-file

Rather than extracting setup logic into a separate `setup/` package, the three registration
functions remain as unexported functions in `cmd/main.go` itself:

```go
func setupChecks(mgr ctrl.Manager)
func setupReconcilers(ctx context.Context, mgr ctrl.Manager, serviceEndpoint []endpoints.ServiceEndpoint, ...)
func setupWebhooks(mgr ctrl.Manager)
```

This is **exactly what CAPI does** in `cluster-api/core/main.go` — unexported helpers at the
bottom of main, not a separate package. CAPI's `setup/` package contains only reusable manager
*option* helpers (`ManagerCacheOptions`, `ManagerClientOptions`), not registration logic.
Exporting the registration functions from a `setup` package would trigger the `revive` stutter
linter rule (`setup.SetupReconcilers` repeats "setup"). Keeping them unexported in `cmd/main.go`
is idiomatic, lint-clean, and mirrors the upstream reference directly.

`cmd/main.go` imports `internal/controllers/powervs`, `internal/controllers/vpc`, and
`webhooks/{powervs,vpc}/admission` directly — bypassing the old alias layer entirely.

---

### Remove `controllers/alias.go`

With `setup/setup.go` instantiating internal reconcilers directly, the `controllers/` package has
no remaining consumers. `controllers/alias.go` and `controllers/doc.go` are deleted. Any future
field added to a reconciler now needs to be changed in exactly one place.

---

### Move admission webhooks out of `internal/`

Webhook files move to a top-level `webhooks/` directory that clearly signals their role as
provider entry points:

| Before | After |
|---|---|
| `internal/webhooks/powervs/*.go` | `webhooks/powervs/admission/*.go` |
| `internal/webhooks/vpc/*.go` | `webhooks/vpc/admission/*.go` |

Package names and all `//+kubebuilder:webhook` markers stay exactly as-is. The `manifests`
Makefile target uses `paths="./..."` which scans every package regardless of location —
**no Makefile changes are needed**.

---

### Conversion webhooks — not applicable

CAPI separates conversion webhooks into `webhooks/{provider}/conversion/`. After investigation
this does not apply here: `ConvertTo`/`ConvertFrom` are Go methods defined on the `v1beta2`/
`v1beta1` types and must remain in `api/` — controller-runtime discovers conversion through
these method signatures. There are no shared conversion helpers to extract. Conversion logic
stays in `api/powervs/v1beta2/conversion.go` and `api/vpc/v1beta1/ibmvpc_conversion.go`
unchanged.

---

### Move `internal/genutil/` to `pkg/cloud/util/`

`internal/genutil/` contained one function — `GetTransitGatewayLocationAndRouting()` — consumed
by both the PowerVS cluster scope and the PowerVS cluster webhook. Placing it behind `internal/`
prevented reuse and gave it a vague name. Moving to `pkg/cloud/util/` places it alongside
`pkg/cloud/scope/`, `pkg/cloud/services/`, and `pkg/cloud/endpoints/`. Package name simplifies
from `genutil` to `util`.

---

### Move `pkg/version/` to root `version/`

Version info is a repo-wide concern — it describes the binary, not any cloud subsystem.
`pkg/version/` had no cloud or utility dependencies (only `fmt` and `runtime`). Moving to
`version/` at the project root matches CAPI (`sigs.k8s.io/cluster-api/version`), Kubernetes,
and the `hack/release/version.sh` ldflags script which already targeted
`sigs.k8s.io/cluster-api-provider-ibmcloud/version.*` — meaning the build script was already
correct and `pkg/version/` was the anomaly.

---

### Move `pkg/cloud/pagingutils/` to `pkg/util/paging/`

`pkg/cloud/pagingutils/` had no IBM Cloud SDK imports — only `fmt` and `net/url`. It is a
generic pagination helper, not a cloud-specific package. Moving it to `pkg/util/paging/` places
it alongside `pkg/util/record/` as a general-purpose utility. The package is also renamed from
`pagingutils` to `paging`, dropping the `utils` suffix in line with Go conventions and CAPI's
own `util/patch/`, `util/conditions/`, `util/collections/`.

---

### Move `pkg/cloud/ignition/` to `pkg/bootstrap/ignition/`

`pkg/cloud/ignition/` contained Go type definitions for the Coreos Ignition v2 machine bootstrap
config format. It had zero IBM Cloud SDK imports — it is a bootstrap data format specification,
not a cloud concept. Moving to `pkg/bootstrap/ignition/` correctly classifies it and mirrors
CAPI's own `bootstrap/` concept.

**Why a local copy is necessary:** `github.com/coreos/ignition/v2` only ships v3.x config types;
v2.x types were removed from the upstream library entirely. The local definition is the only
option for generating Ignition v2 redirect documents. A `doc.go` comment records this rationale
for future contributors.

---

## Kubebuilder Compatibility

| Concern | Impact |
|---|---|
| Webhook marker scanning | `controller-gen` uses `paths="./..."` — scans all packages. No Makefile change when moving webhook files. |
| Controller scaffolding | `kubebuilder create api` writes to `internal/controllers/{group}/` per `PROJECT` file. Unchanged. |
| Conversion code generation | `conversion-gen` targets explicit paths in Makefile. Unchanged since `ConvertTo`/`ConvertFrom` methods stay in `api/`. |

---

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Import churn from moving packages | `internal/` packages carry no stability guarantees; no external consumers exist. All import sites verified with grep before each move. |
| kubebuilder re-scaffolding into old paths | `PROJECT` file controller paths are unchanged; only webhook and utility locations change. |

---

## Dependency Injection for Event Recorders and Provider ID Format

Two packages used global variables as a substitute for proper dependency injection into scope
params — a pattern CAPI has moved away from entirely. Both were addressed in this PR.

### Recorder injection

**Before:** `pkg/util/record/` implemented a global singleton `EventRecorder` initialized once
from `cmd/main.go` and called directly from scope methods without passing a recorder explicitly.

**After:** Every `*ScopeParams` struct (`ClusterScopeParams`, `MachineScopeParams`,
`ImageScopeParams` — for both PowerVS and VPC) gains a `Recorder cgrecord.EventRecorder` field.
Each reconciler struct already held a `Recorder record.EventRecorder` field; `setupReconcilers`
in `cmd/main.go` now passes `mgr.GetEventRecorderFor(...)` into scope params at construction
time. All scope methods call `s.Recorder.Eventf(...)` directly. `pkg/util/record/` is deleted.

### ProviderIDFormat injection

**Before:** `pkg/cloud/options/` stored `ProviderIDFormat` as a global var set from the
`--provider-id-fmt` CLI flag and read directly by machine scopes.

**After:** `MachineScopeParams` and `MachineScope` for both PowerVS and VPC gain a
`ProviderIDFormat string` field. The machine reconciler structs gain the same field, populated
from `cmd/main.go` `setupReconcilers`. Machine scopes read `s.ProviderIDFormat` directly.
`pkg/cloud/options/` is deleted; the `"v2"` constant is inlined where needed.
