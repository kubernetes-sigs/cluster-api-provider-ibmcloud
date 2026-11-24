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

// Command release-notes prints release notes for a CAPIBM tag.
// Pull requests are grouped by the impact emoji on the PR title
// (⚠️ ✨ 🐛 📖 🌱), following the Cluster API release notes tool.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("release-notes", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	cfg := notesConfig{}
	var output string
	fs.StringVar(&cfg.Org, "org", "kubernetes-sigs", "GitHub organization.")
	fs.StringVar(&cfg.Repo, "repo", "cluster-api-provider-ibmcloud", "GitHub repository.")
	fs.StringVar(&cfg.RepoPath, "repo-path", ".", "Path to the local git checkout.")
	fs.StringVar(&cfg.StartSHA, "start-sha", "", "Commit to start from, exclusive.")
	fs.StringVar(&cfg.EndSHA, "end-sha", "", "Commit to stop at, inclusive.")
	fs.StringVar(&cfg.PreviousTag, "previous-tag", "", "Previous release tag, used in the changelog heading and dependency diff.")
	fs.StringVar(&cfg.ReleaseTag, "release", "", "Tag for the release being cut.")
	fs.StringVar(&cfg.Image, "image", "registry.k8s.io/capi-ibmcloud/cluster-api-ibmcloud-controller", "Controller image repository, without the tag.")
	fs.BoolVar(&cfg.IncludeDependencies, "dependencies", true, "Append a module dependency diff. Disabled for pre-releases.")
	fs.StringVar(&output, "output", "", "Write notes to this file. Defaults to stdout.")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.StartSHA == "" || cfg.EndSHA == "" {
		return fmt.Errorf("--start-sha and --end-sha are required")
	}
	if cfg.ReleaseTag == "" {
		return fmt.Errorf("--release is required")
	}

	notes, err := generate(ctx, cfg, gitCommitLister{}, ghPullFetcher{}, modiffDependencies{})
	if err != nil {
		return err
	}
	if output == "" {
		fmt.Print(notes)
		return nil
	}
	if dir := filepath.Dir(output); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}
	if err := os.WriteFile(output, []byte(notes), 0o644); err != nil {
		return fmt.Errorf("write release notes: %w", err)
	}
	return nil
}
