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
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type pullRequest struct {
	Number int
	Title  string
	Author string
}

type commitLister interface {
	List(ctx context.Context, repoPath, start, end string) (numbers []int, commitCount int, err error)
}

type pullFetcher interface {
	Fetch(ctx context.Context, org, repo string, numbers []int) (map[int]pullRequest, error)
}

type gitCommitLister struct{}

var (
	mergePullRequest  = regexp.MustCompile(`^Merge pull request #(\d+)\b`)
	squashPullRequest = regexp.MustCompile(`\(#(\d+)\)$`)
	slugPattern       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

func (gitCommitLister) List(ctx context.Context, repoPath, start, end string) ([]int, int, error) {
	if err := validateRev(start); err != nil {
		return nil, 0, fmt.Errorf("start sha: %w", err)
	}
	if err := validateRev(end); err != nil {
		return nil, 0, fmt.Errorf("end sha: %w", err)
	}

	countOut, err := commandOutput(ctx, "git", "-C", repoPath, "rev-list", "--count", start+".."+end)
	if err != nil {
		return nil, 0, err
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(countOut)))
	if err != nil {
		return nil, 0, fmt.Errorf("parse commit count: %w", err)
	}

	logOut, err := commandOutput(ctx, "git", "-C", repoPath, "log", "--format=%s", start+".."+end)
	if err != nil {
		return nil, 0, err
	}

	var numbers []int
	seen := make(map[int]struct{})
	for _, line := range strings.Split(string(logOut), "\n") {
		number, ok := pullNumber(line)
		if !ok {
			continue
		}
		if _, dup := seen[number]; dup {
			continue
		}
		seen[number] = struct{}{}
		numbers = append(numbers, number)
	}
	return numbers, count, nil
}

func pullNumber(subject string) (int, bool) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return 0, false
	}
	if match := mergePullRequest.FindStringSubmatch(subject); len(match) == 2 {
		return atoiOK(match[1])
	}
	if match := squashPullRequest.FindStringSubmatch(subject); len(match) == 2 {
		return atoiOK(match[1])
	}
	return 0, false
}

func atoiOK(value string) (int, bool) {
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return number, true
}

func validateRev(rev string) error {
	if rev == "" {
		return errors.New("revision is empty")
	}
	if strings.ContainsAny(rev, " \t\n") {
		return fmt.Errorf("invalid revision %q", rev)
	}
	return nil
}

type ghPullFetcher struct{}

func (ghPullFetcher) Fetch(ctx context.Context, org, repo string, numbers []int) (map[int]pullRequest, error) {
	if len(numbers) == 0 {
		return map[int]pullRequest{}, nil
	}
	if err := validateSlug("org", org); err != nil {
		return nil, err
	}
	if err := validateSlug("repo", repo); err != nil {
		return nil, err
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, errors.New("gh GitHub CLI not available. Install it from https://cli.github.com/ and authenticate with GITHUB_TOKEN or gh auth login")
	}

	const batchSize = 40
	found := make(map[int]pullRequest, len(numbers))
	for start := 0; start < len(numbers); start += batchSize {
		end := start + batchSize
		if end > len(numbers) {
			end = len(numbers)
		}
		batch, err := fetchPullBatch(ctx, org, repo, numbers[start:end])
		if err != nil {
			return nil, err
		}
		for number, pr := range batch {
			found[number] = pr
		}
	}
	return found, nil
}

func fetchPullBatch(ctx context.Context, org, repo string, numbers []int) (map[int]pullRequest, error) {
	var query strings.Builder
	fmt.Fprintf(&query, "query { repository(owner: %q, name: %q) {", org, repo)
	for _, number := range numbers {
		fmt.Fprintf(&query, " pr_%d: pullRequest(number: %d) { number title author { login } }", number, number)
	}
	query.WriteString(" } }")

	out, err := commandOutput(ctx, "gh", "api", "graphql", "-f", "query="+query.String())
	if err != nil {
		return nil, err
	}

	var response struct {
		Data struct {
			Repository map[string]json.RawMessage `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(out, &response); err != nil {
		return nil, fmt.Errorf("decode github response: %w", err)
	}
	if response.Data.Repository == nil {
		return nil, fmt.Errorf("github graphql: %s", graphQLError(response.Errors))
	}

	found := make(map[int]pullRequest, len(numbers))
	for _, raw := range response.Data.Repository {
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		var pr struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
			Author *struct {
				Login string `json:"login"`
			} `json:"author"`
		}
		if err := json.Unmarshal(raw, &pr); err != nil {
			return nil, fmt.Errorf("decode pull request: %w", err)
		}
		author := ""
		if pr.Author != nil {
			author = pr.Author.Login
		}
		found[pr.Number] = pullRequest{Number: pr.Number, Title: pr.Title, Author: author}
	}
	return found, nil
}

func graphQLError(errs []struct {
	Message string `json:"message"`
}) string {
	if len(errs) == 0 {
		return "empty response"
	}
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		parts = append(parts, err.Message)
	}
	return strings.Join(parts, "; ")
}

func validateSlug(kind, value string) error {
	if !slugPattern.MatchString(value) {
		return fmt.Errorf("invalid github %s %q", kind, value)
	}
	return nil
}

func commandOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // args are validated revisions, slugs, or a constructed GraphQL query
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		if stderr != "" {
			return nil, fmt.Errorf("%s: %s: %w", name, stderr, err)
		}
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return out, nil
}

func isDependabot(login string) bool {
	login = strings.ToLower(strings.TrimSpace(login))
	return login == "dependabot" || login == "dependabot[bot]"
}
