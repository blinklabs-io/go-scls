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
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/blinklabs-io/go-scls"
	"github.com/spf13/cobra"
)

func TestDiffRootLevel(t *testing.T) {
	oldF := tempSCLS(t, buildSCLS(t, 10, map[string][]kv{
		"keep":   {{[]byte("k1"), []byte("v")}},
		"change": {{[]byte("k1"), []byte("old")}},
		"gone":   {{[]byte("k1"), []byte("v")}},
	}))
	newF := tempSCLS(t, buildSCLS(t, 11, map[string][]kv{
		"keep":   {{[]byte("k1"), []byte("v")}},
		"change": {{[]byte("k1"), []byte("new")}},
		"fresh":  {{[]byte("k1"), []byte("v")}},
	}))
	out, _, err := executeCommand("diff", oldF, newF)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	for _, want := range []string{"keep", "unchanged", "change", "changed", "gone", "removed", "fresh", "added"} {
		if !strings.Contains(out, want) {
			t.Errorf("diff output missing %q:\n%s", want, out)
		}
	}
}

func TestDiffDetailed(t *testing.T) {
	oldF := tempSCLS(t, buildSCLS(t, 1, map[string][]kv{
		"ns": {{[]byte("k1"), []byte("a")}, {[]byte("k2"), []byte("a")}},
	}))
	newF := tempSCLS(t, buildSCLS(t, 2, map[string][]kv{
		"ns": {{[]byte("k2"), []byte("b")}, {[]byte("k3"), []byte("a")}},
	}))
	out, _, err := executeCommand("diff", "--detailed", oldF, newF)
	if err != nil {
		t.Fatalf("diff --detailed: %v", err)
	}
	// k1 removed, k2 changed, k3 added (keys shown as hex: 6b31/6b32/6b33)
	if !strings.Contains(out, "- 6b31") || !strings.Contains(out, "~ 6b32") || !strings.Contains(out, "+ 6b33") {
		t.Errorf("unexpected detailed diff:\n%s", out)
	}
}

func TestDiffJSON(t *testing.T) {
	oldF := tempSCLS(t, buildSCLS(t, 1, map[string][]kv{"ns": {{[]byte("k1"), []byte("a")}}}))
	newF := tempSCLS(t, buildSCLS(t, 2, map[string][]kv{"ns": {{[]byte("k1"), []byte("b")}}}))
	out, _, err := executeCommand("diff", "--json", oldF, newF)
	if err != nil {
		t.Fatalf("diff --json: %v", err)
	}
	if !strings.Contains(out, `"status": "changed"`) || !strings.Contains(out, `"newSlot": 2`) {
		t.Errorf("unexpected json:\n%s", out)
	}
}

type failAfterWriter struct {
	writesBeforeError int
	err               error
}

func (w *failAfterWriter) Write(p []byte) (int, error) {
	if w.writesBeforeError == 0 {
		return 0, w.err
	}
	w.writesBeforeError--
	return len(p), nil
}

func TestRenderDiffReturnsTextWriteErrors(t *testing.T) {
	writeErr := errors.New("closed output")
	rep := &diffReport{
		OldSlot: 1,
		NewSlot: 2,
		Namespaces: []nsDiff{{
			Name:       "ns",
			Status:     "changed",
			OldEntries: 1,
			NewEntries: 1,
			KeyChanges: []keyChange{{Op: "~", Key: "01"}},
		}},
	}
	for _, writesBeforeError := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("write-%d", writesBeforeError+1), func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.SetOut(&failAfterWriter{
				writesBeforeError: writesBeforeError,
				err:               writeErr,
			})
			err := renderDiff(cmd, rep, true, false)
			if !errors.Is(err, writeErr) {
				t.Fatalf("renderDiff error = %v, want output error", err)
			}
		})
	}
}

func TestDiffStdinConsumesManifestBookend(t *testing.T) {
	data := buildSCLS(t, 1, map[string][]kv{"ns": {{[]byte("k1"), []byte("v")}}})
	path := tempSCLS(t, data)
	tests := []struct {
		name string
		args []string
	}{
		{name: "stdin as old", args: []string{"diff", "-", path}},
		{name: "stdin as new", args: []string{"diff", path, "-"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, _, err := executeCommandWithInput(data, tt.args...)
			if err != nil {
				t.Fatalf("%v: %v", tt.args, err)
			}
			if !strings.Contains(out, "unchanged") {
				t.Fatalf("unexpected diff output: %q", out)
			}
		})
	}
}

func TestPrimeEntryCursorClosesOnInitialReadError(t *testing.T) {
	data := buildSCLS(t, 1, map[string][]kv{"ns": {{[]byte("k1"), []byte("v")}}})
	headerSize := int(binary.BigEndian.Uint32(data[:4]))
	data = append(append([]byte(nil), data[:4+headerSize]...), 0, 0)
	sr, err := scls.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	closed := false
	c := &entryCursor{
		sr: sr,
		closeFn: func() error {
			closed = true
			return nil
		},
		want: map[string]bool{"ns": true},
	}
	got, err := primeEntryCursor(c)
	if err == nil {
		t.Fatal("primeEntryCursor succeeded on truncated stream")
	}
	if got != nil {
		t.Fatalf("primeEntryCursor returned cursor after error: %#v", got)
	}
	if !closed {
		t.Fatal("primeEntryCursor did not close cursor after initial read error")
	}
}
