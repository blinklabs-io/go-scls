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
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/blinklabs-io/go-scls"
)

type kv struct{ k, v []byte }

// buildSCLS builds an in-memory SCLS file. entries maps namespace -> ordered
// key/value pairs; namespaces and keys must already be in ascending order
// within the map values (the helper sorts namespaces, not keys).
func buildSCLS(t *testing.T, slot uint64, entries map[string][]kv) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := scls.NewWriter(&buf, scls.WithSummary(scls.ManifestSummary{
		CreatedAt: "2026-07-17T00:00:00Z",
		Tool:      "scls-test",
		Comment:   "fixture",
	}))
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	names := make([]string, 0, len(entries))
	for ns := range entries {
		names = append(names, ns)
	}
	sort.Strings(names)
	for _, ns := range names {
		for _, e := range entries[ns] {
			if err := w.AddEntry(ns, e.k, e.v); err != nil {
				t.Fatalf("AddEntry(%q, %x): %v", ns, e.k, err)
			}
		}
	}
	if _, err := w.Close(slot); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return buf.Bytes()
}

// tempSCLS writes data to a temp .scls file and returns its path.
func tempSCLS(t *testing.T, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.scls")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return p
}

// executeCommand runs the CLI with args, capturing stdout and stderr.
func executeCommand(args ...string) (stdout, stderr string, err error) {
	root := newRootCmd()
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errBuf.String(), err
}

func withStdin(t *testing.T, data []byte, fn func()) {
	t.Helper()
	path := tempSCLS(t, data)
	f, err := os.Open(path) //nolint:gosec // test-owned temporary file
	if err != nil {
		t.Fatalf("open stdin fixture: %v", err)
	}
	oldStdin := os.Stdin
	os.Stdin = f
	defer func() {
		os.Stdin = oldStdin
		_ = f.Close()
	}()
	fn()
}
