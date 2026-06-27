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

// buildFileBytes builds the same file as buildFile without a *testing.T, for
// fuzz seeds. It panics on error (only reachable on a programming mistake).
func buildFileBytes() ([]byte, Hash) {
	var buf bytes.Buffer
	w, err := NewWriter(&buf, WithMaxChunkEntries(4)) // match buildFile: multiple chunks per namespace
	if err != nil {
		panic(err)
	}
	for _, ns := range []string{"aaa/v0", "bbb/v0"} {
		for i := 0; i < 10; i++ {
			if err := w.AddEntry(ns, []byte{byte(i >> 8), byte(i)}, []byte{0x10, byte(i)}); err != nil {
				panic(err)
			}
		}
	}
	root, err := w.Close(42)
	if err != nil {
		panic(err)
	}
	return buf.Bytes(), root
}

// buildFile writes a deterministic multi-namespace, multi-chunk SCLS file and
// returns its bytes plus the global root. Keys are 2 bytes, ascending.
func buildFile(t *testing.T) (data []byte, root Hash) {
	t.Helper()
	d, r := buildFileBytes()
	return d, r
}

func TestOpenManifestAndNamespaces(t *testing.T) {
	data, root := buildFile(t)
	s, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	m := s.Manifest()
	if m == nil {
		t.Fatal("nil manifest")
	}
	if m.RootHash != root {
		t.Fatalf("manifest root %x != writer root %x", m.RootHash, root)
	}
	names := make([]string, 0)
	for _, ns := range s.Namespaces() {
		names = append(names, ns.Name)
	}
	if len(names) != 2 {
		t.Fatalf("got %d namespaces, want 2: %v", len(names), names)
	}
	for _, ns := range []string{"aaa/v0", "bbb/v0"} {
		if lo, hi, ok := s.nsRange(ns); !ok || hi <= lo {
			t.Fatalf("nsRange(%q)=(%d,%d,%v) invalid", ns, lo, hi, ok)
		}
	}
}

func TestOpenGarbageNoPanic(t *testing.T) {
	for _, b := range [][]byte{nil, {0}, {0, 0, 0, 0}, bytes.Repeat([]byte{0xFF}, 64)} {
		if _, err := Open(bytes.NewReader(b), int64(len(b))); err == nil {
			t.Fatalf("Open(%x) succeeded, want error", b)
		}
	}
}

// TestOpenMalformedNoPanic feeds truncations and single-byte corruptions of a
// real file to Open. Inputs reach the record-walk and manifest parsing, so this
// exercises the bounds checks that the tiny-input cases skip. Open must never
// panic; it may return an error or, for a benign corruption, succeed.
func TestOpenMalformedNoPanic(t *testing.T) {
	data, _ := buildFile(t)
	for n := 0; n <= len(data); n++ {
		func(n int) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Open panicked on truncation len=%d: %v", n, r)
				}
			}()
			_, _ = Open(bytes.NewReader(data[:n]), int64(n))
		}(n)
	}
	for i := 0; i < len(data); i++ {
		func(i int) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Open panicked on corruption at byte %d: %v", i, r)
				}
			}()
			corrupt := append([]byte(nil), data...)
			corrupt[i] ^= 0xFF
			s, err := Open(bytes.NewReader(corrupt), int64(len(corrupt)))
			if err == nil {
				// If it still opened, exercising accessors must not panic either.
				for _, ns := range s.Namespaces() {
					_, _, _ = s.nsRange(ns.Name)
				}
			}
		}(i)
	}
}

// TestOpenChunkHeaderFallback forces the probe-too-small fallback by shrinking
// chunkHeaderProbe so no namespace+key fits in the probe, then confirms Open
// still indexes chunks and extracts the correct first key via the full re-read.
func TestOpenChunkHeaderFallback(t *testing.T) {
	orig := chunkHeaderProbe
	chunkHeaderProbe = 1
	defer func() { chunkHeaderProbe = orig }()

	data, _ := buildFile(t)
	s, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Open with tiny probe: %v", err)
	}
	if lo, hi, ok := s.nsRange("aaa/v0"); !ok || hi <= lo {
		t.Fatalf("nsRange after fallback invalid: (%d,%d,%v)", lo, hi, ok)
	}
	if len(s.chunks) == 0 || !bytes.Equal(s.chunks[0].firstKey, []byte{0, 0}) {
		t.Fatalf("fallback parsed wrong first key: %x", s.chunks)
	}
}

func TestOpenRejectsUnsupportedChunkFormat(t *testing.T) {
	rec := chunkRecord(t, singleEntryChunk("a", 0, 0x01))
	rec.payload = append([]byte(nil), rec.payload...)
	rec.payload[8] = ChunkFormatZstd
	file := buildRawFile(t,
		rec,
		manifestRecord(t, &Manifest{
			TotalEntries: 1,
			TotalChunks:  1,
			Namespaces:   []NamespaceInfo{{Name: "a", EntriesCount: 1, ChunksCount: 1}},
		}),
	)
	if _, err := Open(bytes.NewReader(file), int64(len(file))); !errors.Is(err, ErrUnsupportedChunkFormat) {
		t.Fatalf("want ErrUnsupportedChunkFormat, got %v", err)
	}
}

func TestSnapshotGet(t *testing.T) {
	data, _ := buildFile(t)
	s, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for _, ns := range []string{"aaa/v0", "bbb/v0"} {
		for i := 0; i < 10; i++ {
			key := []byte{byte(i >> 8), byte(i)}
			want := []byte{0x10, byte(i)}
			got, err := s.Get(ns, key)
			if err != nil {
				t.Fatalf("Get(%q,%x): %v", ns, key, err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("Get(%q,%x)=%x want %x", ns, key, got, want)
			}
		}
	}
	if _, err := s.Get("aaa/v0", []byte{0xFF, 0xFF}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("absent key: got %v want ErrNotFound", err)
	}
	if _, err := s.Get("zzz/v0", []byte{0, 0}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("absent namespace: got %v want ErrNotFound", err)
	}
}

func TestSnapshotProve(t *testing.T) {
	data, root := buildFile(t)
	s, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for _, ns := range []string{"aaa/v0", "bbb/v0"} {
		for i := 0; i < 10; i++ {
			key := []byte{byte(i >> 8), byte(i)}
			val, proof, err := s.Prove(ns, key)
			if err != nil {
				t.Fatalf("Prove(%q,%x): %v", ns, key, err)
			}
			if err := VerifyProof(root, ns, key, val, proof); err != nil {
				t.Fatalf("VerifyProof(%q,%x): %v", ns, key, err)
			}
			// tampering the value must break verification
			bad := append([]byte{0xFF}, val...)
			if err := VerifyProof(root, ns, key, bad, proof); !errors.Is(err, ErrProofMismatch) {
				t.Fatalf("tampered value verified for %q,%x", ns, key)
			}
		}
	}
	if _, _, err := s.Prove("aaa/v0", []byte{0xFF, 0xFF}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("absent key: got %v want ErrNotFound", err)
	}
}

func TestSnapshotProveRejectsManifestGlobalRootMismatch(t *testing.T) {
	data, _ := buildFile(t)
	file := append([]byte(nil), data...)

	br := bytes.NewReader(file)
	offset := 0
	for {
		recType, payload, err := readRecord(br)
		if err != nil {
			t.Fatal("manifest not found")
		}
		if recType == RecordTypeManifest {
			file[offset+5+len(payload)-4-HashSize] ^= 0xff
			break
		}
		offset += 5 + len(payload)
	}

	s, err := Open(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, _, err := s.Prove("aaa/v0", []byte{0, 0}); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("want ErrInvalidManifest, got %v", err)
	}
}
