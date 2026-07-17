// Copyright 2026 Blink Labs Software
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

package main

import (
	"strings"
	"testing"
)

func TestInfoText(t *testing.T) {
	data := buildSCLS(t, 99, map[string][]kv{
		"utxo": {{[]byte("aaaa"), []byte("v1")}, {[]byte("bbbb"), []byte("v2")}},
	})
	path := tempSCLS(t, data)
	out, _, err := executeCommand("info", path)
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	for _, want := range []string{"version", "slot", "99", "utxo", "root"} {
		if !strings.Contains(out, want) {
			t.Errorf("info output missing %q:\n%s", want, out)
		}
	}
}

func TestInfoJSON(t *testing.T) {
	data := buildSCLS(t, 99, map[string][]kv{"utxo": {{[]byte("aaaa"), []byte("v1")}}})
	path := tempSCLS(t, data)
	out, _, err := executeCommand("info", "--json", path)
	if err != nil {
		t.Fatalf("info --json: %v", err)
	}
	if !strings.Contains(out, `"slot": 99`) || !strings.Contains(out, `"tool": "scls-test"`) {
		t.Errorf("unexpected json: %q", out)
	}
}

func TestInfoStdinConsumesManifestBookend(t *testing.T) {
	data := buildSCLS(t, 99, map[string][]kv{"utxo": {{[]byte("aaaa"), []byte("v1")}}})
	withStdin(t, data, func() {
		out, _, err := executeCommand("info", "-")
		if err != nil {
			t.Fatalf("info -: %v", err)
		}
		if !strings.Contains(out, "slot:           99") {
			t.Fatalf("unexpected info output: %q", out)
		}
	})
}
