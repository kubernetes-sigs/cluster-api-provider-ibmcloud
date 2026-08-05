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

## Support and Guarantees

Cluster API Provider IBM Cloud maintains the most recent release/releases for all supported API
and contract versions. Support here refers to the ability to backport and release patch versions;
the standard [backport policy](https://github.com/kubernetes-sigs/cluster-api/blob/main/CONTRIBUTING.md#backporting-a-patch)
is defined upstream.

For the full version support matrix — including which CAPIBM releases are currently supported,
their EOL dates, CAPI compatibility, and Kubernetes version ranges — see the
**[Version Support Policy](../reference/versions.md)** reference page.

### Rules

CAPIBM follows the same N / N-1 / N-2 model as upstream CAPI:

- **N** and **N-1** receive standard support: bug fixes, backports, patch releases, and full CI
  signal.
- **N-2** is in **maintenance mode**: partial CI only, no proactive backports. Emergency patches
  may be considered case-by-case by maintainers.
- **N-3 and older** are EOL and receive no support.
- Test coverage is maintained for all non-EOL minor releases (N, N-1, N-2). When N+1 is
  released, tests for N-2 are removed (it becomes N-3 / EOL).
- The API version is determined from the GroupVersion defined in the top-level `api/` package.
- The EOL date of each API version is determined from the last release available once a new API
  version is published.

## Dependency Updates

- The CAPI, Kubernetes, and test packages receive regular updates for supported releases to
  ensure they remain synchronised with the CAPI release in use. This is ideally scheduled with
  every new CAPI n-1 and n-2 minor release.
- IBM Cloud SDK packages are monitored for updates alongside CAPI minor release activity, as
  long as there are no breaking changes that impact project stability.
- Exceptions can be filed with maintainers and considered on a case-by-case basis.
