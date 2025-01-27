/*
Copyright 2025 The Kubernetes Authors.

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

package parser

import "github.com/golang-jwt/jwt"

//go:generate ../../../../hack/tools/bin/mockgen -source=./parser.go -destination=./mock/parser_generated.go -package=mock
//go:generate /usr/bin/env bash -c "cat ../../../../hack/boilerplate/boilerplate.generatego.txt ./mock/parser_generated.go > ./mock/_parser_generated.go && mv ./mock/_parser_generated.go ./mock/parser_generated.go"

// TokenParser interface defines a method reuired to parse token.
type TokenParser interface {
	Parse(token string, fn jwt.Keyfunc) (*jwt.Token, error)
}
