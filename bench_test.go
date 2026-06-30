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
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

const benchEntries = 10_000

// benchFile writes benchEntries 32-byte-key/64-byte-value entries.
func benchFile(b *testing.B) []byte {
	b.Helper()
	var buf bytes.Buffer
	w, err := NewWriter(&buf)
	if err != nil {
		b.Fatal(err)
	}
	key := make([]byte, 32)
	value := bytes.Repeat([]byte{0xab}, 64)
	for i := range benchEntries {
		binary.BigEndian.PutUint64(key, uint64(i))
		if err := w.AddEntry("utxo/v0", key, value); err != nil {
			b.Fatal(err)
		}
	}
	if _, err := w.Close(1); err != nil {
		b.Fatal(err)
	}
	return buf.Bytes()
}

func BenchmarkWriter(b *testing.B) {
	key := make([]byte, 32)
	value := bytes.Repeat([]byte{0xab}, 64)
	b.SetBytes(benchEntries * int64(4+len(key)+len(value)))
	for b.Loop() {
		w, err := NewWriter(io.Discard)
		if err != nil {
			b.Fatal(err)
		}
		for i := range benchEntries {
			binary.BigEndian.PutUint64(key, uint64(i))
			if err := w.AddEntry("utxo/v0", key, value); err != nil {
				b.Fatal(err)
			}
		}
		if _, err := w.Close(1); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReader(b *testing.B) {
	file := benchFile(b)
	b.SetBytes(int64(len(file)))
	for b.Loop() {
		r, err := NewReader(bytes.NewReader(file))
		if err != nil {
			b.Fatal(err)
		}
		for {
			_, err := r.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkVerifyFull(b *testing.B) {
	file := benchFile(b)
	b.SetBytes(int64(len(file)))
	for b.Loop() {
		if _, err := Verify(bytes.NewReader(file), VerifyFull); err != nil {
			b.Fatal(err)
		}
	}
}

func benchKey(i uint64) []byte {
	key := make([]byte, 32)
	binary.BigEndian.PutUint64(key, i)
	return key
}

func BenchmarkGet(b *testing.B) {
	data := benchFile(b)
	s, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		b.Fatal(err)
	}
	key := benchKey(benchEntries / 2)
	for b.Loop() {
		if _, err := s.Get("utxo/v0", key); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProve(b *testing.B) {
	data := benchFile(b)
	s, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		b.Fatal(err)
	}
	key := benchKey(benchEntries / 2)
	for b.Loop() {
		if _, _, err := s.Prove("utxo/v0", key); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVerifyProof(b *testing.B) {
	data := benchFile(b)
	s, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		b.Fatal(err)
	}
	key := benchKey(benchEntries / 2)
	root := s.Manifest().RootHash
	val, proof, err := s.Prove("utxo/v0", key)
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if err := VerifyProof(root, "utxo/v0", key, val, proof); err != nil {
			b.Fatal(err)
		}
	}
}
