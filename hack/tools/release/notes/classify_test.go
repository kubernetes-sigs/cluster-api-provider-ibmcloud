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

import "testing"

func TestClassify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		title   string
		section section
		cleaned string
	}{
		{title: ":warning: Breaking API", section: sectionWarning, cleaned: "Breaking API"},
		{title: "⚠️ Breaking API", section: sectionWarning, cleaned: "Breaking API"},
		{title: "⚠ Breaking API", section: sectionWarning, cleaned: "Breaking API"},
		{title: ":sparkles: add feature", section: sectionFeature, cleaned: "Add feature"},
		{title: "✨ add feature", section: sectionFeature, cleaned: "Add feature"},
		{title: ":bug: fix bug", section: sectionBug, cleaned: "Fix bug"},
		{title: "🐛 fix bug", section: sectionBug, cleaned: "Fix bug"},
		{title: ":book: update docs", section: sectionDocumentation, cleaned: "Update docs"},
		{title: "📖  Updating the run book", section: sectionDocumentation, cleaned: "Updating the run book"},
		{title: "📖 proposal for tags", section: sectionProposals, cleaned: "Proposal for tags"},
		{title: "📖 Add a CAEP", section: sectionProposals, cleaned: "Add a CAEP"},
		{title: ":seedling: bump modules", section: sectionOther, cleaned: "Bump modules"},
		{title: "🌱 Update owners", section: sectionOther, cleaned: "Update owners"},
		{title: "🚀 Release v0.2.0", section: sectionSkip, cleaned: ""},
		{title: ":rocket: Release v0.2.0", section: sectionSkip, cleaned: ""},
		{title: "[release-0.15] 🐛 Fix DHCP", section: sectionBug, cleaned: "Fix DHCP"},
		{title: "[release-0.15] :seedling: Bump golang", section: sectionOther, cleaned: "Bump golang"},
		{title: "WIP 🐛 Fix deletion", section: sectionBug, cleaned: "Fix deletion"},
		{title: "Update owners", section: sectionUnknown, cleaned: "Update owners"},
		{title: "   ", section: sectionSkip, cleaned: ""},
		{title: "🌱", section: sectionSkip, cleaned: ""},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			t.Parallel()
			gotSection, gotTitle := classify(tt.title)
			if gotSection != tt.section || gotTitle != tt.cleaned {
				t.Fatalf("classify(%q) = (%v, %q), want (%v, %q)", tt.title, gotSection, gotTitle, tt.section, tt.cleaned)
			}
		})
	}
}
