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
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/blinklabs-io/go-scls"
	"github.com/spf13/cobra"
)

func lsFixture(t *testing.T) string {
	t.Helper()
	data := buildSCLS(t, 1, map[string][]kv{
		"aaa": {{[]byte("k1"), []byte("v")}},
		"bbb": {{[]byte("k1"), []byte("v")}, {[]byte("k2"), []byte("v")}},
	})
	return tempSCLS(t, data)
}

func TestLsNamespaces(t *testing.T) {
	out, _, err := executeCommand("ls", lsFixture(t))
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if !strings.Contains(out, "aaa") || !strings.Contains(out, "bbb") {
		t.Errorf("expected both namespaces, got %q", out)
	}
}

func TestRenderNamespacesSortsManifestOrder(t *testing.T) {
	manifest := &scls.Manifest{Namespaces: []scls.NamespaceInfo{
		{Name: "zebra", EntriesCount: 2, ChunksCount: 1},
		{Name: "alpha", EntriesCount: 1, ChunksCount: 1},
	}}
	for _, asJSON := range []bool{false, true} {
		name := "text"
		if asJSON {
			name = "json"
		}
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&buf)
			if err := renderNamespaces(cmd, manifest, asJSON); err != nil {
				t.Fatalf("renderNamespaces: %v", err)
			}
			if asJSON {
				var got struct {
					Namespaces []namespaceJSON `json:"namespaces"`
				}
				if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
					t.Fatalf("unmarshal output: %v", err)
				}
				if len(got.Namespaces) != 2 ||
					got.Namespaces[0].Name != "alpha" ||
					got.Namespaces[1].Name != "zebra" {
					t.Fatalf(
						"namespace order = %#v, want alpha, zebra",
						got.Namespaces,
					)
				}
				return
			}
			if strings.Index(buf.String(), "alpha") >
				strings.Index(buf.String(), "zebra") {
				t.Fatalf("namespaces are not sorted:\n%s", buf.String())
			}
		})
	}
	if manifest.Namespaces[0].Name != "zebra" {
		t.Fatal("renderNamespaces mutated manifest namespace order")
	}
}

func TestLsKeys(t *testing.T) {
	out, _, err := executeCommand("ls", lsFixture(t), "bbb")
	if err != nil {
		t.Fatalf("ls bbb: %v", err)
	}
	// "k1" and "k2" as hex
	if !strings.Contains(out, "6b31") || !strings.Contains(out, "6b32") {
		t.Errorf("expected hex keys for k1/k2, got %q", out)
	}
}

func TestLsKeysJSON(t *testing.T) {
	out, _, err := executeCommand("ls", "--json", lsFixture(t), "aaa")
	if err != nil {
		t.Fatalf("ls --json: %v", err)
	}
	if !strings.Contains(out, `"namespace": "aaa"`) || !strings.Contains(out, `"keys"`) {
		t.Errorf("unexpected json: %q", out)
	}
}

func TestLsStdinConsumesManifestBookend(t *testing.T) {
	t.Parallel()
	data := buildSCLS(t, 1, map[string][]kv{"aaa": {{[]byte("k1"), []byte("v")}}})
	out, _, err := executeCommandWithInput(data, "ls", "-")
	if err != nil {
		t.Fatalf("ls -: %v", err)
	}
	if !strings.Contains(out, "aaa") {
		t.Fatalf("unexpected ls output: %q", out)
	}
}

type errorWriter struct{ err error }

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }

func TestStreamKeysReturnsWriteErrorImmediately(t *testing.T) {
	data := buildSCLS(t, 1, map[string][]kv{"aaa": {{[]byte("k1"), []byte("v")}}})
	headerEnd := 4 + int(binary.BigEndian.Uint32(data[:4]))
	chunkEnd := headerEnd + 4 + int(binary.BigEndian.Uint32(data[headerEnd:headerEnd+4]))
	writeErr := errors.New("closed output")

	tests := []struct {
		name   string
		asJSON bool
		data   []byte
	}{
		{name: "writer construction", asJSON: true, data: append([]byte(nil), data[:headerEnd+2]...)},
		{name: "key write", data: append([]byte(nil), data[:chunkEnd+2]...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tempSCLS(t, tt.data)
			cmd := newLsCmd()
			cmd.SetOut(errorWriter{err: writeErr})
			err := streamKeys(cmd, path, "aaa", tt.asJSON)
			if !errors.Is(err, writeErr) {
				t.Fatalf("streamKeys error = %v, want output error", err)
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("reader error replaced output error: %v", err)
			}
		})
	}
}
