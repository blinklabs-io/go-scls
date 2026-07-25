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

func TestVerifyOK(t *testing.T) {
	data := buildSCLS(t, 42, map[string][]kv{
		"utxo": {{[]byte("aaaa"), []byte("v1")}, {[]byte("bbbb"), []byte("v2")}},
	})
	path := tempSCLS(t, data)
	out, _, err := executeCommand("verify", path)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("expected OK, got %q", out)
	}
}

func TestVerifyJSON(t *testing.T) {
	data := buildSCLS(t, 42, map[string][]kv{
		"utxo": {{[]byte("aaaa"), []byte("v1")}},
	})
	path := tempSCLS(t, data)
	out, _, err := executeCommand("verify", "--json", path)
	if err != nil {
		t.Fatalf("verify --json: %v", err)
	}
	if !strings.Contains(out, `"ok": true`) || !strings.Contains(out, `"rootHash"`) {
		t.Errorf("unexpected json: %q", out)
	}
}

func TestVerifyJSONStructureOmitsRoot(t *testing.T) {
	data := buildSCLS(t, 42, map[string][]kv{
		"utxo": {{[]byte("aaaa"), []byte("v1")}},
	})
	path := tempSCLS(t, data)
	out, _, err := executeCommand("verify", "--json", "--level", "structure", path)
	if err != nil {
		t.Fatalf("verify --json --level structure: %v", err)
	}
	if strings.Contains(out, `"rootHash"`) {
		t.Errorf("expected rootHash to be omitted at structure level, got %q", out)
	}
	if strings.Contains(out, `"digest"`) {
		t.Errorf("expected digest to be omitted at structure level, got %q", out)
	}
}

func TestVerifyBadLevel(t *testing.T) {
	if _, _, err := executeCommand("verify", "--level", "bogus", "nofile"); err == nil {
		t.Fatal("expected error for bad level")
	}
}

func TestVerifyCorruptFails(t *testing.T) {
	data := buildSCLS(t, 1, map[string][]kv{"ns": {{[]byte("kkkk"), []byte("v")}}})
	data[len(data)-1] ^= 0xff // corrupt the manifest offset bookend
	path := tempSCLS(t, data)
	if _, _, err := executeCommand("verify", path); err == nil {
		t.Fatal("expected verification failure on corrupt file")
	}
}
