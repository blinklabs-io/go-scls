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

package scls

import (
	"bytes"
	"errors"
	"testing"
)

func TestVerifyProofRejectsExcessiveDepth(t *testing.T) {
	proof := Proof{nsPath: make([]proofStep, maxProofSteps+1)}
	if err := VerifyProof(Hash{}, "ns", nil, nil, proof); !errors.Is(err, ErrInvalidProof) {
		t.Fatalf("got %v, want ErrInvalidProof", err)
	}
}

func proofFixture(t *testing.T) (*Snapshot, [][]byte) {
	t.Helper()
	var buf bytes.Buffer
	w, err := NewWriter(&buf)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	keys := [][]byte{{1, 1, 1, 1}, {2, 2, 2, 2}, {3, 3, 3, 3}}
	for _, k := range keys {
		if err := w.AddEntry("ns", k, []byte("val")); err != nil {
			t.Fatalf("AddEntry: %v", err)
		}
	}
	if _, err := w.Close(7); err != nil {
		t.Fatalf("Close: %v", err)
	}
	data := buf.Bytes()
	s, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s, keys
}

func TestProofBinaryRoundTrip(t *testing.T) {
	s, keys := proofFixture(t)
	value, proof, err := s.Prove("ns", keys[1])
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}
	enc, err := proof.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	got, err := UnmarshalProofBinary(enc)
	if err != nil {
		t.Fatalf("UnmarshalProofBinary: %v", err)
	}
	reenc, err := got.MarshalBinary()
	if err != nil {
		t.Fatalf("re-MarshalBinary: %v", err)
	}
	if !bytes.Equal(enc, reenc) {
		t.Fatalf("round-trip mismatch:\n in=%x\nout=%x", enc, reenc)
	}
	if err := VerifyProof(s.Manifest().RootHash, "ns", keys[1], value, got); err != nil {
		t.Fatalf("decoded proof failed VerifyProof: %v", err)
	}
}

func TestUnmarshalProofBinaryRejectsBadInput(t *testing.T) {
	emptyProof, err := Proof{}.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal empty: %v", err)
	}
	cases := map[string]struct {
		input     []byte
		truncated bool
	}{
		"empty":            {input: []byte{}, truncated: true},
		"bad version":      {input: []byte{2, 0, 0, 0, 0, 0, 0, 0, 0}},
		"trailing bytes":   {input: append(append([]byte(nil), emptyProof...), 0xff)},
		"short step count": {input: []byte{1, 0, 0, 0, 1}, truncated: true}, // claims 1 ns step, no step data
	}
	for name, tc := range cases {
		_, err := UnmarshalProofBinary(tc.input)
		if !errors.Is(err, ErrInvalidProof) {
			t.Errorf("%s: got %v, want ErrInvalidProof", name, err)
		}
		if tc.truncated && !errors.Is(err, ErrTruncatedRecord) {
			t.Errorf("%s: got %v, want ErrTruncatedRecord", name, err)
		}
		if !tc.truncated && errors.Is(err, ErrTruncatedRecord) {
			t.Errorf("%s: got unexpected ErrTruncatedRecord: %v", name, err)
		}
	}
}
