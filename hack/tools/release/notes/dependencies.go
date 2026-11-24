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
	"strings"

	"github.com/saschagrunert/go-modiff/pkg/modiff"
)

// dependencyChanges reports module changes between two revisions.
type dependencyChanges interface {
	Changes(ctx context.Context, repository, from, to string) (string, error)
}

// modiffDependencies uses go-modiff, the same differ Cluster API and the
// Kubernetes release notes tool use for the Dependencies section.
type modiffDependencies struct{}

func (modiffDependencies) Changes(ctx context.Context, repository, from, to string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if from == "" || to == "" || from == to {
		return "", fmt.Errorf("cannot diff dependencies from %q to %q", from, to)
	}
	out, err := modiff.Run(modiff.NewConfig(repository, from, to, true, 2))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
