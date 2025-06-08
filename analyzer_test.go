// Copyright 2025 Oliver Eikemeier. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package ptrequality_test

import (
	"testing"

	. "github.com/fillmore-labs/ptrequality"
	"golang.org/x/tools/go/analysis/analysistest"
)

//nolint:paralleltest
func TestAnalyzer(t *testing.T) {
	dir := analysistest.TestData()

	tests := []struct {
		name  string
		flags map[string]string
		pkg   string
	}{
		{
			name: "default",
			pkg:  "go.test/a",
		},
		{
			name: "check-is=false",
			flags: map[string]string{
				"check-is": "false",
			},
			pkg: "go.test/b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Analyzer
			for f, v := range tt.flags {
				if err := a.Flags.Set(f, v); err != nil {
					t.Fatalf("Can't set flag %s=%s: %v", f, v, err)
				}
			}

			analysistest.Run(t, dir, a, tt.pkg)
		})
	}
}
