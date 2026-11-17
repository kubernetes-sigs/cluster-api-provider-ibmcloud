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
	"fmt"
	"net/url"
	"sort"
	"strings"
)

type notesConfig struct {
	Org                 string
	Repo                string
	RepoPath            string
	StartSHA            string
	EndSHA              string
	PreviousTag         string
	ReleaseTag          string
	Image               string
	IncludeDependencies bool
}

type noteEntry struct {
	section section
	line    string
}

// generate builds release notes for the commits between StartSHA and EndSHA.
// Pull requests are grouped by the emoji prefix on the PR title.
func generate(ctx context.Context, cfg notesConfig, lister commitLister, fetcher pullFetcher, deps dependencyChanges) (string, error) {
	numbers, commitCount, err := lister.List(ctx, cfg.RepoPath, cfg.StartSHA, cfg.EndSHA)
	if err != nil {
		return "", err
	}
	pulls, err := fetcher.Fetch(ctx, cfg.Org, cfg.Repo, numbers)
	if err != nil {
		return "", err
	}

	repo := cfg.Org + "/" + cfg.Repo
	entries := make([]noteEntry, 0, len(numbers))
	var missing []int
	for _, number := range numbers {
		pr, ok := pulls[number]
		if !ok {
			missing = append(missing, number)
			continue
		}
		// Dependabot bumps are covered by the module diff when that section is enabled.
		if cfg.IncludeDependencies && isDependabot(pr.Author) {
			continue
		}
		sec, title := classify(pr.Title)
		if sec == sectionSkip || title == "" {
			continue
		}
		entries = append(entries, noteEntry{section: sec, line: formatLine(repo, pr, title)})
	}

	var dependencies string
	if cfg.IncludeDependencies {
		if deps == nil {
			return "", fmt.Errorf("dependency diff requested but no differ is configured")
		}
		dependencies, err = deps.Changes(ctx, "github.com/"+repo, cfg.PreviousTag, cfg.ReleaseTag)
		if err != nil {
			return "", fmt.Errorf("dependency changes: %w", err)
		}
	}

	return render(cfg, repo, commitCount, entries, missing, dependencies), nil
}

func formatLine(repo string, pr pullRequest, title string) string {
	line := fmt.Sprintf("- %s ([#%d](https://github.com/%s/pull/%d)", title, pr.Number, repo, pr.Number)
	if pr.Author != "" {
		line += fmt.Sprintf(", [@%s](https://github.com/%s)", pr.Author, url.PathEscape(pr.Author))
	}
	line += ")"
	return line
}

func render(cfg notesConfig, repo string, commitCount int, entries []noteEntry, missing []int, dependencies string) string {
	grouped := map[section][]string{}
	for _, entry := range entries {
		grouped[entry.section] = append(grouped[entry.section], entry.line)
	}
	for _, lines := range grouped {
		sort.SliceStable(lines, func(i, j int) bool {
			return strings.ToLower(lines[i]) < strings.ToLower(lines[j])
		})
	}

	var b strings.Builder
	b.WriteString("# What's Changed\n\n")
	if banner := preReleaseBanner(repo, cfg.ReleaseTag); banner != "" {
		b.WriteString(banner)
		b.WriteString("\n\n")
	}
	if len(missing) > 0 {
		sort.Ints(missing)
		b.WriteString("## FIXME: Consistency errors!\n")
		for _, number := range missing {
			fmt.Fprintf(&b, "- PR number %d identified from commits, not found on GitHub\n", number)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Highlights\n\n* REPLACE ME\n\n")
	fmt.Fprintf(&b, "## Changes since %s\n\n", sinceLabel(cfg))
	b.WriteString("## :chart_with_upwards_trend: Overview\n")
	writeCount(&b, commitCount, "new commit merged", "new commits merged")
	writeCount(&b, len(grouped[sectionWarning]), "breaking change :warning:", "breaking changes :warning:")
	writeCount(&b, len(grouped[sectionFeature]), "feature addition ✨", "feature additions ✨")
	writeCount(&b, len(grouped[sectionBug]), "bug fixed 🐛", "bugs fixed 🐛")
	if commitCount == 0 && len(entries) == 0 {
		b.WriteString("- No changes in this range\n")
	}
	b.WriteString("\n")

	for _, sec := range sectionOrder {
		lines := grouped[sec]
		if len(lines) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n", sectionHeading[sec])
		for _, line := range lines {
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if strings.TrimSpace(dependencies) != "" {
		b.WriteString(strings.TrimSpace(dependencies))
		b.WriteString("\n\n")
	}

	if cfg.Image != "" && cfg.ReleaseTag != "" {
		fmt.Fprintf(&b, "## The image for this release is:\n%s:%s\n\n", cfg.Image, cfg.ReleaseTag)
	}
	if cfg.PreviousTag != "" && cfg.ReleaseTag != "" {
		b.WriteString("<!-- markdown-link-check-disable-next-line -->\n")
		fmt.Fprintf(&b, "Full Changelog: https://github.com/%s/compare/%s...%s\n\n", repo, cfg.PreviousTag, cfg.ReleaseTag)
	}
	b.WriteString("**_Thanks to all our contributors!_** 😊\n")
	return b.String()
}

func sinceLabel(cfg notesConfig) string {
	if cfg.PreviousTag != "" {
		return cfg.PreviousTag
	}
	return cfg.StartSHA
}

func writeCount(b *strings.Builder, count int, singular, plural string) {
	switch count {
	case 0:
		return
	case 1:
		fmt.Fprintf(b, "- 1 %s\n", singular)
	default:
		fmt.Fprintf(b, "- %d %s\n", count, plural)
	}
}

func preReleaseBanner(repo, tag string) string {
	kind := releaseKind(tag)
	if kind == "" {
		return ""
	}
	article := "a"
	if strings.HasPrefix(kind, "A") {
		article = "an"
	}
	return fmt.Sprintf("🚨 This is %s %s. Use it only for testing purposes. If you find any bugs, file an [issue](https://github.com/%s/issues/new).", article, kind, repo)
}

func releaseKind(tag string) string {
	version := strings.TrimPrefix(tag, "v")
	_, pre, ok := strings.Cut(version, "-")
	if !ok || pre == "" {
		return ""
	}
	pre = strings.ToLower(pre)
	switch {
	case strings.HasPrefix(pre, "alpha"):
		return "ALPHA RELEASE"
	case strings.HasPrefix(pre, "beta"):
		return "BETA RELEASE"
	default:
		return "RELEASE CANDIDATE"
	}
}
