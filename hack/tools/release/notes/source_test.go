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
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestPullNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		subject string
		number  int
		ok      bool
	}{
		{subject: "🐛 Fix the thing (#12)", number: 12, ok: true},
		{subject: "Merge pull request #13 from someone/branch", number: 13, ok: true},
		{subject: ":seedling: Bump golang.org/x/text (#3048)", number: 3048, ok: true},
		{subject: "something (#12) more", ok: false},
		{subject: "no number", ok: false},
		{subject: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.subject, func(t *testing.T) {
			t.Parallel()
			got, ok := pullNumber(tt.subject)
			if ok != tt.ok || got != tt.number {
				t.Fatalf("pullNumber(%q) = (%d, %v), want (%d, %v)", tt.subject, got, ok, tt.number, tt.ok)
			}
		})
	}
}

func TestGitCommitLister(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=test@example.com", "-c", "user.name=test"}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", args, err, out)
		}
	}

	git("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("initial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "README")
	git("commit", "-m", "initial")
	start := gitOutput(t, dir, "rev-parse", "HEAD")

	git("commit", "--allow-empty", "-m", "🐛 Fix the thing (#12)")
	git("commit", "--allow-empty", "-m", "Merge pull request #13 from someone/branch")
	git("commit", "--allow-empty", "-m", "no pull request here")
	git("commit", "--allow-empty", "-m", "🐛 Fix the thing (#12)")
	end := gitOutput(t, dir, "rev-parse", "HEAD")

	numbers, count, err := gitCommitLister{}.List(context.Background(), dir, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatalf("commit count = %d, want 4", count)
	}
	slices.Sort(numbers)
	if !slices.Equal(numbers, []int{12, 13}) {
		t.Fatalf("pull numbers = %v, want [12 13]", numbers)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", args, err)
	}
	return string(trimNewline(out))
}

func trimNewline(in []byte) []byte {
	if len(in) > 0 && in[len(in)-1] == '\n' {
		return in[:len(in)-1]
	}
	return in
}
