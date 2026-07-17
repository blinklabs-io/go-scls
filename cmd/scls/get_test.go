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
	"errors"
	"strings"
	"testing"
)

type getErrorWriter struct{ err error }

func (w getErrorWriter) Write([]byte) (int, error) { return 0, w.err }

func getFixture(t *testing.T) string {
	t.Helper()
	data := buildSCLS(t, 1, map[string][]kv{
		"ns": {{[]byte{1, 1, 1, 1}, []byte("hello")}, {[]byte{2, 2, 2, 2}, []byte("world")}},
	})
	return tempSCLS(t, data)
}

func TestGetRaw(t *testing.T) {
	out, _, err := executeCommand("get", getFixture(t), "ns", "02020202")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if out != "world" {
		t.Errorf("expected raw value %q, got %q", "world", out)
	}
}

func TestGetHex(t *testing.T) {
	out, _, err := executeCommand("get", "--hex", getFixture(t), "ns", "01010101")
	if err != nil {
		t.Fatalf("get --hex: %v", err)
	}
	if strings.TrimSpace(out) != "68656c6c6f" { // "hello"
		t.Errorf("expected hex of hello, got %q", out)
	}
}

func TestGetHexReturnsWriteError(t *testing.T) {
	writeErr := errors.New("closed output")
	cmd := newGetCmd()
	cmd.SetOut(getErrorWriter{err: writeErr})
	cmd.SetArgs([]string{"--hex", getFixture(t), "ns", "01010101"})
	if err := cmd.Execute(); !errors.Is(err, writeErr) {
		t.Fatalf("get --hex error = %v, want output error", err)
	}
}

func TestGetJSON(t *testing.T) {
	out, _, err := executeCommand("get", "--json", getFixture(t), "ns", "01010101")
	if err != nil {
		t.Fatalf("get --json: %v", err)
	}
	if !strings.Contains(out, `"value": "68656c6c6f"`) {
		t.Errorf("unexpected json: %q", out)
	}
}

func TestGetNotFound(t *testing.T) {
	if _, _, err := executeCommand("get", getFixture(t), "ns", "ffffffff"); err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestGetBadHex(t *testing.T) {
	if _, _, err := executeCommand("get", getFixture(t), "ns", "zz"); err == nil {
		t.Fatal("expected bad-hex error")
	}
}
