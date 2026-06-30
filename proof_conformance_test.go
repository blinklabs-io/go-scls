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
	"io"
	"os"
	"testing"
)

func firstEntry(t *testing.T, data []byte) (ns string, key, val []byte, hasEntry bool) {
	t.Helper()
	sr, err := NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	for {
		c, err := sr.Next()
		if errors.Is(err, io.EOF) {
			return "", nil, nil, false
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if len(c.Entries) > 0 {
			e := c.Entries[0]
			return c.Namespace, append([]byte(nil), e.Key...), append([]byte(nil), e.Value...), true
		}
	}
}

func TestProofConformance(t *testing.T) {
	fixtures := []string{
		"testdata/minimal-raw.scls",
		"testdata/multi-ns.scls",
		"testdata/multi-chunk.scls",
		"testdata/multi-ns-multi-chunk.scls",
	}
	for _, f := range fixtures {
		f := f
		t.Run(f, func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			ns, key, val, ok := firstEntry(t, data)
			if !ok {
				t.Skip("fixture has no entries")
			}
			s, err := Open(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			root := s.Manifest().RootHash

			gotVal, proof, err := s.Prove(ns, key)
			if err != nil {
				t.Fatalf("Prove: %v", err)
			}
			if !bytes.Equal(gotVal, val) {
				t.Fatalf("Prove value %x != %x", gotVal, val)
			}
			if err := VerifyProof(root, ns, key, gotVal, proof); err != nil {
				t.Fatalf("VerifyProof: %v", err)
			}

			sVal, sProof, err := LookupProof(bytes.NewReader(data), ns, key)
			if err != nil {
				t.Fatalf("LookupProof: %v", err)
			}
			if err := VerifyProof(root, ns, key, sVal, sProof); err != nil {
				t.Fatalf("VerifyProof (streaming): %v", err)
			}

			gv, err := s.Get(ns, key)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			lv, err := Lookup(bytes.NewReader(data), ns, key)
			if err != nil {
				t.Fatalf("Lookup: %v", err)
			}
			if !bytes.Equal(gv, lv) || !bytes.Equal(gv, val) {
				t.Fatalf("value mismatch Get=%x Lookup=%x want=%x", gv, lv, val)
			}
		})
	}
}
