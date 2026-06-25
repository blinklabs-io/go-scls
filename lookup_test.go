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
	"testing"
)

func TestLookupParity(t *testing.T) {
	data, root := buildFile(t)
	s, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for _, ns := range []string{"aaa/v0", "bbb/v0"} {
		for i := 0; i < 10; i++ {
			key := []byte{byte(i >> 8), byte(i)}

			gotVal, gerr := Lookup(bytes.NewReader(data), ns, key)
			wantVal, _ := s.Get(ns, key)
			if gerr != nil || !bytes.Equal(gotVal, wantVal) {
				t.Fatalf("Lookup(%q,%x)=%x,%v want %x", ns, key, gotVal, gerr, wantVal)
			}

			pVal, proof, perr := LookupProof(bytes.NewReader(data), ns, key)
			if perr != nil {
				t.Fatalf("LookupProof(%q,%x): %v", ns, key, perr)
			}
			if err := VerifyProof(root, ns, key, pVal, proof); err != nil {
				t.Fatalf("VerifyProof from streaming proof (%q,%x): %v", ns, key, err)
			}
		}
	}
	if _, err := Lookup(bytes.NewReader(data), "aaa/v0", []byte{0xFF, 0xFF}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("absent key: got %v want ErrNotFound", err)
	}
}

// ensure Lookup accepts a non-seeking reader
var _ io.Reader = (*bytes.Reader)(nil)
