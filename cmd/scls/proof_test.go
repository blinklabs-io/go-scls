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
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProofEnvelopeRejectsInvalidUTF8Namespace(t *testing.T) {
	env := proofEnvelope{Namespace: string([]byte{0xff})}
	for _, format := range []string{"json", "binary"} {
		if _, err := marshalEnvelope(env, format); err == nil {
			t.Errorf("marshal %s: expected invalid UTF-8 error", format)
		}
	}

	raw := make([]byte, 10)
	copy(raw, proofMagic[:])
	raw[4] = proofEnvelopeVersion
	binary.BigEndian.PutUint32(raw[5:9], 1)
	raw[9] = 0xff
	// Add the four remaining empty length-prefixed fields.
	raw = append(raw, make([]byte, 16)...)
	if _, err := unmarshalEnvelopeBinary(raw); err == nil {
		t.Fatal("unmarshal binary: expected invalid UTF-8 error")
	}
}

func proofFixturePath(t *testing.T) string {
	t.Helper()
	data := buildSCLS(t, 5, map[string][]kv{
		"ns": {{[]byte{1, 1, 1, 1}, []byte("a")}, {[]byte{2, 2, 2, 2}, []byte("b")}},
	})
	return tempSCLS(t, data)
}

func genProof(t *testing.T, args ...string) string {
	t.Helper()
	out, _, err := executeCommand(append([]string{"proof", "generate"}, args...)...)
	if err != nil {
		t.Fatalf("proof generate %v: %v", args, err)
	}
	pf := filepath.Join(t.TempDir(), "proof")
	if err := os.WriteFile(pf, []byte(out), 0o600); err != nil {
		t.Fatalf("write proof: %v", err)
	}
	return pf
}

func TestProofRoundTripJSON(t *testing.T) {
	pf := genProof(t, proofFixturePath(t), "ns", "02020202")
	out, _, err := executeCommand("proof", "verify", pf)
	if err != nil {
		t.Fatalf("proof verify: %v", err)
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("expected OK, got %q", out)
	}
}

func TestProofRoundTripBinary(t *testing.T) {
	pf := genProof(t, "--format", "binary", proofFixturePath(t), "ns", "01010101")
	out, _, err := executeCommand("proof", "verify", pf)
	if err != nil {
		t.Fatalf("proof verify (binary): %v", err)
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("expected OK, got %q", out)
	}
}

func TestProofVerifyRootMatch(t *testing.T) {
	// Grab the real root from info --json, then pin it on verify.
	path := proofFixturePath(t)
	pf := genProof(t, path, "ns", "02020202")
	root := rootFromInfo(t, path)
	if _, _, err := executeCommand("proof", "verify", "--root", root, pf); err != nil {
		t.Fatalf("verify with correct --root: %v", err)
	}
	if _, _, err := executeCommand("proof", "verify", "--root", strings.Repeat("00", 28), pf); err == nil {
		t.Fatal("expected mismatch error with wrong --root")
	}
}

func rootFromInfo(t *testing.T, path string) string {
	t.Helper()
	out, _, err := executeCommand("info", "--json", path)
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	var info infoJSON
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		t.Fatalf("unmarshal info JSON: %v", err)
	}
	if info.RootHash == "" {
		t.Fatalf("empty rootHash in %q", out)
	}
	return info.RootHash
}

func TestProofVerifyTampered(t *testing.T) {
	pf := genProof(t, proofFixturePath(t), "ns", "02020202")
	raw, err := os.ReadFile(pf)
	if err != nil {
		t.Fatalf("read proof: %v", err)
	}
	// Corrupt the value hex so the proof no longer folds to the root.
	tampered := strings.Replace(string(raw), `"value": "62"`, `"value": "63"`, 1)
	pf2 := filepath.Join(t.TempDir(), "tampered")
	if err := os.WriteFile(pf2, []byte(tampered), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := executeCommand("proof", "verify", pf2); err == nil {
		t.Fatal("expected verification failure on tampered proof")
	}
}
