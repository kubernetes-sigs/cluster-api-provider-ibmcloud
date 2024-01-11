# Release Support Guidelines

## Branches

Cluster API Provider IBM Cloud has two types of branches: the *main* branch and
*release-X* branches.

The *main* branch is where development happens. All the latest and
greatest code, including breaking changes, happens on main.

The *release-X* branches contain stable, backwards compatible code. On every
major or minor release, a new branch is created. It is from these
branches that minor and patch releases are tagged. In some cases, it may
be necessary to open PRs for bugfixes directly against stable branches, but
this should generally not be the case.

### Support and guarantees

Cluster API Provider IBM Cloud maintains the most recent release/releases for all supported API and contract versions. Support for this section refers to the ability to backport and release patch versions;
standard [backport policy](https://github.com/kubernetes-sigs/cluster-api/blob/main/CONTRIBUTING.md#backporting-a-patch) is defined here.

- The API version is determined from the GroupVersion defined in the top-level `api/` package.
- The EOL date of each API Version is determined from the last release available once a new API version is published.

| API Version  | Supported Until      |
|--------------|----------------------|
| **v1beta2**  | TBD (current stable) |
| **v1beta1**  | EOL since 2023-02-09 |

- For the current stable API version (v1beta2) we support the two most recent minor releases; older minor releases are immediately unsupported when a new major/minor release is available.
- For older API versions we only support the most recent minor release until the API version reaches EOL.
- We will maintain test coverage for all supported minor releases for the current stable API version in case we have to do an emergency patch release.
  For example, if v0.5 and v0.6 are currently supported. When v0.7 is released, tests for v0.5 will be removed.

| Minor Release | API Version | Supported Until                                    |
|---------------|-------------|----------------------------------------------------|
| v0.6.x        | **v1beta2** | when v0.8.0 will be released                       |
| v0.5.x        | **v1beta2** | when v0.7.0 will be released, tentatively Nov 2023 |
| v0.4.x        | **v1beta2** | EOL since 2023-09-07 - v0.6.0 release date         |
| v0.3.x        | **v1beta1** | EOL since 2023-02-09 - API version EOL             |

- The CAPI, k8s and test packages will receive regular updates for supported releases to ensure they remain synchronized with the CAPI release being utilized as an integral component of the provider release. This activity is ideally scheduled to occur with every new n-1 and n-2 CAPI minor releases.
- The IBM packages will be monitored for latest updates in conjunction with CAPI minor release update activity, as long as there are no disruptive changes that impact the project stability.
- Exceptions can be filed with maintainers and taken into consideration on a case-by-case basis.
