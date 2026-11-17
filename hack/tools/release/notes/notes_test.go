/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"strings"
	"testing"
)

type fakeLister struct {
	numbers []int
	commits int
}

func (f fakeLister) List(context.Context, string, string, string) ([]int, int, error) {
	return f.numbers, f.commits, nil
}

type fakeFetcher struct {
	prs map[int]pullRequest
}

func (f fakeFetcher) Fetch(context.Context, string, string, []int) (map[int]pullRequest, error) {
	return f.prs, nil
}

type fakeDeps struct {
	out string
}

func (f fakeDeps) Changes(context.Context, string, string, string) (string, error) {
	return f.out, nil
}

func TestGenerateGroupsByEmoji(t *testing.T) {
	t.Parallel()

	notes, err := generate(context.Background(), notesConfig{
		Org:         "kubernetes-sigs",
		Repo:        "cluster-api-provider-ibmcloud",
		PreviousTag: "v0.1.0",
		ReleaseTag:  "v0.2.0",
		Image:       "registry.k8s.io/capi-ibmcloud/cluster-api-ibmcloud-controller",
	}, fakeLister{
		numbers: []int{1, 2, 3, 9, 4, 5, 6, 7, 8},
		commits: 10,
	}, fakeFetcher{prs: map[int]pullRequest{
		1: {Number: 1, Title: "⚠️ Remove v1beta1", Author: "alice"},
		2: {Number: 2, Title: ":sparkles: add VPC routing", Author: "bob"},
		3: {Number: 3, Title: "🐛 Fix DHCP", Author: "carol"},
		9: {Number: 9, Title: "🐛 adjust logs", Author: "carol"},
		4: {Number: 4, Title: "[release-0.2] 📖 proposal for tags", Author: "dave"},
		5: {Number: 5, Title: "🌱 Update owners", Author: "erin"},
		6: {Number: 6, Title: "🚀 Release v0.2.0", Author: "release"},
		7: {Number: 7, Title: "Missing emoji", Author: "frank"},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}

	want := `# What's Changed

## FIXME: Consistency errors!
- PR number 8 identified from commits, not found on GitHub

## Highlights

* REPLACE ME

## Changes since v0.1.0

## :chart_with_upwards_trend: Overview
- 10 new commits merged
- 1 breaking change :warning:
- 1 feature addition ✨
- 2 bugs fixed 🐛

## :memo: Proposals
- Proposal for tags ([#4](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/pull/4), [@dave](https://github.com/dave))

## :warning: Breaking Changes
- Remove v1beta1 ([#1](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/pull/1), [@alice](https://github.com/alice))

## :sparkles: New Features
- Add VPC routing ([#2](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/pull/2), [@bob](https://github.com/bob))

## :bug: Bug Fixes
- Adjust logs ([#9](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/pull/9), [@carol](https://github.com/carol))
- Fix DHCP ([#3](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/pull/3), [@carol](https://github.com/carol))

## :seedling: Others
- Update owners ([#5](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/pull/5), [@erin](https://github.com/erin))

## :question: Sort these by hand
- Missing emoji ([#7](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/pull/7), [@frank](https://github.com/frank))

## The image for this release is:
registry.k8s.io/capi-ibmcloud/cluster-api-ibmcloud-controller:v0.2.0

<!-- markdown-link-check-disable-next-line -->
Full Changelog: https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/compare/v0.1.0...v0.2.0

**_Thanks to all our contributors!_** 😊
`
	if notes != want {
		t.Fatalf("notes mismatch\n got:\n%s\nwant:\n%s", notes, want)
	}
	if strings.Contains(notes, "Release v0.2.0 ([#6]") {
		t.Fatal("release trigger PR was included")
	}
}

func TestGeneratePreReleaseSkipsDependencySection(t *testing.T) {
	t.Parallel()

	notes, err := generate(context.Background(), notesConfig{
		Org:                 "kubernetes-sigs",
		Repo:                "cluster-api-provider-ibmcloud",
		PreviousTag:         "v0.2.0-beta.0",
		ReleaseTag:          "v0.2.0-rc.1",
		Image:               "registry.k8s.io/capi-ibmcloud/cluster-api-ibmcloud-controller",
		IncludeDependencies: false,
	}, fakeLister{
		numbers: []int{10, 11},
		commits: 2,
	}, fakeFetcher{prs: map[int]pullRequest{
		10: {Number: 10, Title: "🌱 Bump something", Author: "dependabot"},
		11: {Number: 11, Title: "📖 Update the book", Author: "erin"},
	}}, fakeDeps{out: "## Dependencies\nshould not appear"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(notes, "This is a RELEASE CANDIDATE.") {
		t.Fatalf("missing release candidate banner:\n%s", notes)
	}
	if !strings.Contains(notes, "Bump something") {
		t.Fatal("dependabot PR should be listed when the dependency section is disabled")
	}
	if strings.Contains(notes, "should not appear") {
		t.Fatal("dependency section was included for a pre-release")
	}
	if !strings.Contains(notes, "## :book: Documentation") {
		t.Fatalf("missing documentation section:\n%s", notes)
	}
}

func TestGenerateSkipsDependabotWhenDependenciesEnabled(t *testing.T) {
	t.Parallel()

	notes, err := generate(context.Background(), notesConfig{
		Org:                 "kubernetes-sigs",
		Repo:                "cluster-api-provider-ibmcloud",
		PreviousTag:         "v0.1.0",
		ReleaseTag:          "v0.2.0",
		IncludeDependencies: true,
	}, fakeLister{
		numbers: []int{10, 11},
		commits: 2,
	}, fakeFetcher{prs: map[int]pullRequest{
		10: {Number: 10, Title: "🌱 Bump something", Author: "dependabot"},
		11: {Number: 11, Title: "🌱 Bump golang", Author: "alice"},
	}}, fakeDeps{out: "## Dependencies\n\n### Changed\n- example.com/mod: v1 → v2"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(notes, "Bump something") {
		t.Fatal("dependabot PR should be omitted when the dependency section is enabled")
	}
	if !strings.Contains(notes, "Bump golang") {
		t.Fatal("non-dependabot bump should still be listed")
	}
	if !strings.Contains(notes, "## Dependencies") || !strings.Contains(notes, "example.com/mod") {
		t.Fatalf("missing dependency section:\n%s", notes)
	}
}

func TestReleaseKind(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"v0.2.0":         "",
		"v0.2.0-alpha.3": "ALPHA RELEASE",
		"v0.2.0-beta.1":  "BETA RELEASE",
		"v0.2.0-rc.1":    "RELEASE CANDIDATE",
		"v0.2.0-rc0":     "RELEASE CANDIDATE",
	}
	for tag, want := range tests {
		if got := releaseKind(tag); got != want {
			t.Errorf("releaseKind(%q) = %q, want %q", tag, got, want)
		}
	}
}
