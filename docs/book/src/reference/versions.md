# Version Support Policy

This page documents the support policy for Cluster API Provider IBM Cloud (CAPIBM), including
which CAPIBM releases are supported, which versions of [Cluster API (CAPI)][capi-versions]
they are compatible with, and which Kubernetes versions can be used as management and workload
clusters.

> **Note:** CAPIBM ships a single provider binary but contains **two independent infrastructure
> providers** — [IBM Power Virtual Server (PowerVS)](#powervs-api-version-lifecycle) and
> [IBM VPC](#vpc-api-version-lifecycle) — each with their own API version track.

---

## Supported CAPIBM Releases

CAPIBM follows the same support model as upstream CAPI:

- **N** (latest minor) — Standard support: bug fixes, patch releases, full CI signal.
- **N-1** — Standard support: bug fixes, patch releases, full CI signal.
- **N-2** — **Maintenance mode**: partial CI only, no proactive backports; emergency patches
  considered case-by-case by maintainers.
- **N-3 and older** — EOL: no support.

| CAPIBM Release | PowerVS API | VPC API | Status |
|:---------------|:------------|:--------|:-------|
| **v0.15.x** (main) | v1beta3 | v1beta2 | ✅ N — Standard support |
| **v0.14.x** | v1beta3 | v1beta2 | ✅ N-1 — Standard support |
| **v0.13.x** | v1beta2 | v1beta2 | 🔧 N-2 — Maintenance mode (EOL when v0.16.0 is released) |

<details>
<summary>EOL releases (click to expand)</summary>

| CAPIBM Release | PowerVS API | VPC API | EOL Since |
|:---------------|:------------|:--------|:----------|
| v0.12.x | v1beta2 | v1beta2 | 2026-05-18 (v0.14.0 release) |
| v0.11.x | v1beta2 | v1beta2 | 2025-12-15 (v0.13.0 release) |
| v0.10.x | v1beta2 | v1beta2 | 2025-09-04 (v0.12.0 release) |
| v0.9.x | v1beta2 | v1beta2 | 2025-05-13 (v0.11.0 release) |
| v0.8.x | v1beta2 | v1beta2 | 2025-02-12 (v0.10.0 release) |
| v0.7.x | v1beta2 | v1beta2 | 2024-11-22 (v0.9.0 release) |
| v0.6.x | v1beta2 | v1beta2 | 2024-05-23 (v0.8.0 release) |
| v0.5.x | v1beta2 | v1beta2 | 2023-12-15 (v0.7.0 release) |
| v0.4.x | v1beta2 | v1beta2 | 2023-09-07 (v0.6.0 release) |
| v0.3.x | v1beta1 | v1beta1 | 2023-02-09 (API version EOL) |
| v0.2.x | v1beta1 | v1beta1 | 2023-02-09 (API version EOL) |
| v0.1.x | v1alpha4 | v1alpha4 | EOL |

</details>

---

## Cluster API (CAPI) Compatibility

The table below maps each CAPIBM release range to the CAPI contract version it implements.
Both providers (PowerVS and VPC) ship in the same binary and share the same CAPI contract.

| CAPIBM Release | Compatible CAPI Version |
|:---------------|:------------------------|
| v0.[14-15].x, main | CAPI v1beta2 (v1.11.x – v1.13.x+) |
| v0.[4-13].x | CAPI v1beta1 (v1.1.x – v1.10.x) |
| v0.2.x – v0.3.x | CAPI v1beta1 (v1.1.x – v1.10.x) |
| v0.1.x | CAPI v1alpha4 (v0.4) |

> **Current stable:** CAPIBM `main` / v0.15.x targets **CAPI v1beta2**, which corresponds to
> CAPI releases `v1.11.x`, `v1.12.x`, and `v1.13.x`.

See the official [Cluster API version support page][capi-versions] for CAPI's own support
lifecycle and its Kubernetes compatibility matrix.

---

## Kubernetes Version Compatibility

CAPIBM follows CAPI's Kubernetes support policy. The supported Kubernetes versions for both
providers are determined by the CAPI version the release depends on.

### Management Cluster

Each CAPI minor release supports **four Kubernetes minor versions** (N to N-3) for the
management cluster at initial cut. The table below shows the **combined range** covered by the
listed CAPI minors for each CAPIBM release line.

| CAPIBM Release | CAPI Version | Combined Management Cluster Kubernetes Range |
|:---------------|:-------------|:---------------------------------------------|
| v0.[14-15].x, main | v1.11.x – v1.13.x | Kubernetes v1.29 – v1.35 |
| v0.[4-13].x | v1.1.x – v1.10.x | Kubernetes v1.20 – v1.32 |

### Workload Cluster

The workload cluster Kubernetes version is **independent** of the management cluster version.
Management and workload clusters can be upgraded in any order. Both must fall within the range
that the CAPI version in use has been tested with.

Each CAPI minor release supports (at initial cut):

- **Management cluster:** Kubernetes N to N-3 (4 minor versions)
- **Workload cluster:** Kubernetes N to N-5 (6 minor versions)

As new Kubernetes minor releases ship, CAPI extends both ranges in patch releases. Refer to the
[Cluster API supported versions][capi-versions] page for the exact matrix tied to each CAPI
release.

---

## Upgrade and Downgrade Policy

### Upgrading CAPIBM

CAPIBM follows the same skip-upgrade limit as upstream CAPI: you can skip **at most N-3 minor
versions** in a single upgrade. For example, upgrading from v0.12.x directly to v0.15.x is the
maximum allowed skip. Skipping more than three minor versions may leave the management cluster
in a non-functional state.

Always upgrade **clusterctl first**, then use it to upgrade all other components.

### Downgrades

CAPIBM does **not** support version downgrades. Downgrading may leave the management cluster in
a non-functional state.

---

## PowerVS API Version Lifecycle

The PowerVS provider (`infrastructure.cluster.x-k8s.io/powervs`) has its own API version
track, currently ahead of the VPC provider.

| API Version | Provider Releases | Storage | Served | Status |
|:------------|:------------------|:--------|:-------|:-------|
| **v1beta3** | v0.[14-15].x, main | ✅ Yes (hub) | ✅ Yes | Current stable |
| **v1beta2** | v0.[4-14].x | No | ✅ Yes (until v0.17) | Deprecated — see removal roadmap below |
| **v1beta1** | v0.2.x – v0.3.x | No | No | EOL since 2023-02-09 |
| **v1alpha4** | v0.1.x | No | No | EOL |

- The current stable PowerVS API version is **v1beta3** (storage version since v0.14).
- `v1beta2` is still **served** by the API server and automatically converted to v1beta3 at
  admission. It will remain served until v0.17 per the Kubernetes deprecation policy
  (minimum 3 minor releases after v1beta3 was introduced in v0.14).
- See the [PowerVS v1beta2 → v1beta3 migration guide](../migrations/powervs-v1beta2-to-v1beta3.md)
  to migrate your manifests before v1beta2 is unserved.

### PowerVS v1beta2 Removal Roadmap

v1beta2 follows the [Kubernetes API deprecation policy](https://kubernetes.io/docs/reference/using-api/deprecation-policy/)
and must remain served for at least 3 minor CAPIBM releases after v1beta3 was introduced. The
planned removal schedule is:

| Date | CAPI | CAPIBM | v1beta2 | v1beta3 | Notes |
|:-----|:-----|:-------|:--------|:--------|:------|
| December 2025 | v1.12 | v0.13 | Served, not storage | — | Last release before v1beta3 |
| April 2026 | v1.13 | v0.14 | Served | **Storage** | v1beta3 introduced |
| August 2026 | v1.14 | v0.15 | Served | Storage | |
| December 2026 | v1.15 | v0.16 | Served | Storage | |
| April 2027 | v1.16 | v0.17 | **Unserved** | Storage | v1beta2 stops being served |
| August 2027 | v1.17 | v0.18 | Unserved | Storage | |
| December 2027 | v1.18 | v0.19 | Unserved | Storage | |
| April 2028 | v1.19 | v0.20 | Unserved | Storage | |
| August 2028 | v1.20 | v0.21 | **Removed** | Storage | v1beta2 fully removed |

> **Action required:** Migrate all PowerVS manifests and tooling from `v1beta2` to `v1beta3`
> before upgrading to CAPIBM v0.17 (expected April 2027), when v1beta2 will no longer be
> served by the API server.

The four-version gap between unserved (v0.17) and removed (v0.21) ensures that managedField
cleanup runs even for clusters that skip up to N-3 versions before upgrading.

---

## VPC API Version Lifecycle

The VPC provider (`infrastructure.cluster.x-k8s.io/vpc`) has an independent API version
track, currently at v1beta2.

| API Version | Provider Releases | Hub / Spoke | Supported Until |
|:------------|:------------------|:------------|:----------------|
| **v1beta2** | v0.[4-15].x, main | Hub (current storage version) | TBD (current stable) |
| **v1beta1** | v0.2.x – v0.3.x | Spoke (conversion to v1beta2) | EOL since 2023-02-09 |
| **v1alpha4** | v0.1.x | — | EOL |

- The current stable VPC API version is **v1beta2**.
- `v1beta1` resources are automatically converted to `v1beta2` (the hub version) at admission.

---

## Support Rules Summary

1. **N and N-1** (two most recent minor releases) receive **standard support**: bug fixes,
   backports, patch releases, and full CI signal.
2. **N-2** is in **maintenance mode**: partial CI preserved for emergency patch capability;
   no proactive backports; security scans may be disabled.
3. **N-3 and older** are **EOL**: no support, no backports.
4. Test coverage is maintained for N, N-1, and N-2. When a new release makes the former N-2
   into N-3, its tests are removed.
5. The CAPI, Kubernetes, and test package dependencies are kept in sync with supported CAPI
   minor releases. Updates are targeted with every new CAPI N-1 and N-2 minor release.
6. IBM Cloud SDK packages are updated alongside CAPI minor release updates, as long as there
   are no breaking changes that impact project stability.
7. Exceptions can be filed with maintainers and considered on a case-by-case basis.

---

## Further Reading

- [Cluster API Version Support Policy][capi-versions] — upstream CAPI version and Kubernetes
  compatibility matrix.
- [Release Support Guidelines](../developer/release-support-guidelines.md) — branch strategy,
  backport policy, and release cadence for CAPIBM.
- [Release Process](../developer/release.md) — how CAPIBM releases are cut.
- [CAPIBM Releases](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/releases) —
  full list of published releases on GitHub.

[capi-versions]: https://cluster-api.sigs.k8s.io/reference/versions
