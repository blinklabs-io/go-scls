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
	"testing"
)

// FuzzSnapshot ensures Open + Get + Prove never panic on arbitrary input.
func FuzzSnapshot(f *testing.F) {
	valid, _ := buildFileBytes() // seed with a valid file
	f.Add(valid)
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		s, err := Open(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return
		}
		// Exercise lookups with a few arbitrary keys; must not panic.
		for _, ns := range s.Namespaces() {
			_, _ = s.Get(ns.Name, []byte{0x00})
			_, _, _ = s.Prove(ns.Name, []byte{0x00})
		}
	})
}

// FuzzVerifyProof ensures VerifyProof never panics on arbitrary proof bytes.
func FuzzVerifyProof(f *testing.F) {
	f.Add([]byte("seed"), uint8(0))
	f.Fuzz(func(t *testing.T, blob []byte, n uint8) {
		// Build a Proof from arbitrary bytes: chop blob into HashSize chunks.
		var steps []proofStep
		for off := 0; off+HashSize <= len(blob) && len(steps) < int(n); off += HashSize {
			var h Hash
			copy(h[:], blob[off:off+HashSize])
			steps = append(steps, proofStep{sibling: h, left: off%2 == 0})
		}
		var root Hash
		copy(root[:], blob)
		_ = VerifyProof(root, "ns", []byte("k"), blob, Proof{nsPath: steps, globalPath: steps})
	})
}
