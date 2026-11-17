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
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// section is the release note group for a pull request, chosen from the PR title emoji.
type section int

const (
	sectionSkip section = iota
	sectionProposals
	sectionWarning
	sectionFeature
	sectionBug
	sectionOther
	sectionDocumentation
	sectionUnknown
)

var sectionOrder = []section{
	sectionProposals,
	sectionWarning,
	sectionFeature,
	sectionBug,
	sectionOther,
	sectionDocumentation,
	sectionUnknown,
}

var sectionHeading = map[section]string{
	sectionProposals:     ":memo: Proposals",
	sectionWarning:       ":warning: Breaking Changes",
	sectionFeature:       ":sparkles: New Features",
	sectionBug:           ":bug: Bug Fixes",
	sectionOther:         ":seedling: Others",
	sectionDocumentation: ":book: Documentation",
	sectionUnknown:       ":question: Sort these by hand",
}

// emojiPrefixes are matched in order. Longer marks come first so a variation
// selector on ⚠️ is consumed with the emoji instead of leaking into the title.
var emojiPrefixes = []struct {
	marker  string
	section section
}{
	{marker: "⚠️", section: sectionWarning},
	{marker: "⚠", section: sectionWarning},
	{marker: "✨", section: sectionFeature},
	{marker: "🐛", section: sectionBug},
	{marker: "📖", section: sectionDocumentation},
	{marker: "🌱", section: sectionOther},
	{marker: "🚀", section: sectionSkip},
}

var leadingShortcodes = []struct {
	from string
	to   string
}{
	{from: ":warning:", to: "⚠"},
	{from: ":sparkles:", to: "✨"},
	{from: ":bug:", to: "🐛"},
	{from: ":book:", to: "📖"},
	{from: ":rocket:", to: "🚀"},
	{from: ":seedling:", to: "🌱"},
}

var (
	leadingMarker = regexp.MustCompile(`(?i)^(\[WIP\]|WIP:?)(\s+|$)`)
	leadingTag    = regexp.MustCompile(`^\[[[:alnum:]._-]*\]\s*`)
)

// classify maps a pull request title onto a release note section and returns the
// title with the emoji, release tag, and WIP prefix removed.
func classify(title string) (section, string) {
	title = strings.TrimSpace(title)
	title = stripLeadingMarkers(title)
	title = replaceLeadingShortcode(title)
	title = strings.TrimSpace(title)

	sec, rest, ok := splitEmoji(title)
	if !ok {
		if title == "" {
			return sectionSkip, ""
		}
		return sectionUnknown, capitalize(title)
	}

	rest = strings.TrimPrefix(rest, "\uFE0F")
	rest = strings.TrimSpace(rest)
	if rest == "" || sec == sectionSkip {
		return sectionSkip, ""
	}
	if sec == sectionDocumentation && (strings.Contains(strings.ToLower(rest), "proposal") || strings.Contains(rest, "CAEP")) {
		sec = sectionProposals
	}
	return sec, capitalize(rest)
}

func stripLeadingMarkers(title string) string {
	for {
		next := strings.TrimSpace(leadingMarker.ReplaceAllString(title, ""))
		next = strings.TrimSpace(leadingTag.ReplaceAllString(next, ""))
		if next == title {
			return title
		}
		title = next
	}
}

func replaceLeadingShortcode(title string) string {
	for _, shortcode := range leadingShortcodes {
		if strings.HasPrefix(title, shortcode.from) {
			return shortcode.to + strings.TrimPrefix(title, shortcode.from)
		}
	}
	return title
}

func splitEmoji(title string) (section, string, bool) {
	for _, prefix := range emojiPrefixes {
		if strings.HasPrefix(title, prefix.marker) {
			return prefix.section, strings.TrimPrefix(title, prefix.marker), true
		}
	}
	return 0, title, false
}

func capitalize(value string) string {
	r, size := utf8.DecodeRuneInString(value)
	if r == utf8.RuneError && size == 0 {
		return value
	}
	upper := unicode.ToUpper(r)
	if upper == r {
		return value
	}
	return string(upper) + value[size:]
}
