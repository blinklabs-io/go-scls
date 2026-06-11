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

func TestVerifyFullPassesOnWriterOutput(t *testing.T) {
	file := writeTestFile(t) // from reader_test.go
	res, err := Verify(bytes.NewReader(file), VerifyFull)
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalEntries != 4 || res.TotalChunks != 3 {
		t.Fatalf("result %+v", res)
	}
}

func TestVerifyDetectsCorruptEntry(t *testing.T) {
	file := writeTestFile(t)
	// locate the first CHUNK record and flip the first key byte of its
	// first entry (entries start after the fixed chunk header + namespace;
	// skip the entry's own 4-byte size prefix)
	br := bytes.NewReader(file)
	offset := 0
	for {
		recType, payload, err := readRecord(br)
		if err != nil {
			t.Fatal("chunk not found")
		}
		if recType == RecordTypeChunk {
			c, err := decodeChunk(payload)
			if err != nil {
				t.Fatal(err)
			}
			entryOff := chunkHeaderFixedSize + len(c.Namespace)
			file[offset+5+entryOff+4] ^= 0xff
			break
		}
		offset += 5 + len(payload)
	}
	_, err := Verify(bytes.NewReader(file), VerifyChunks)
	if err == nil {
		t.Fatal("corruption not detected at VerifyChunks")
	}
}

func TestVerifyDetectsTamperedManifestRoot(t *testing.T) {
	file := writeTestFile(t)
	// locate the manifest payload and flip a byte of root_hash, which sits
	// immediately before the trailing u32 offset field (RECONCILIATION #7)
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
	_, err := Verify(bytes.NewReader(file), VerifyFull)
	if !errors.Is(err, ErrHashMismatch) {
		t.Fatalf("want ErrHashMismatch, got %v", err)
	}
	// structural-only verification must still pass
	if _, err := Verify(bytes.NewReader(file), VerifyStructure); err != nil {
		t.Fatalf("VerifyStructure should pass: %v", err)
	}
}

func TestVerifyDetectsCountMismatch(t *testing.T) {
	file := writeTestFile(t)
	// tamper total_entries (u64 at payload offset 8) in the manifest
	br := bytes.NewReader(file)
	offset := 0
	for {
		recType, payload, err := readRecord(br)
		if err != nil {
			t.Fatal("manifest not found")
		}
		if recType == RecordTypeManifest {
			file[offset+5+15] ^= 0xff
			break
		}
		offset += 5 + len(payload)
	}
	_, err := Verify(bytes.NewReader(file), VerifyStructure)
	if !errors.Is(err, ErrCountMismatch) {
		t.Fatalf("want ErrCountMismatch, got %v", err)
	}
}

func FuzzVerify(f *testing.F) {
	if seed, err := buildTestFile(); err == nil {
		f.Add(seed)
	}
	f.Add([]byte{})
	f.Add([]byte{'S', 'C', 'L', 'S'})
	f.Fuzz(func(t *testing.T, data []byte) {
		// must never panic
		_, _ = Verify(bytes.NewReader(data), VerifyFull)
	})
}

// FuzzRoundTrip asserts the writer/verifier contract: any file the Writer
// produces passes VerifyFull with a matching root, and its manifest is
// locatable via the trailing offset bookend.
func FuzzRoundTrip(f *testing.F) {
	f.Add([]byte("seed data for values"), uint8(3), uint8(5))
	f.Add([]byte{}, uint8(0), uint8(1))
	f.Fuzz(func(t *testing.T, data []byte, rot, n uint8) {
		var buf bytes.Buffer
		w, err := NewWriter(&buf,
			WithMaxChunkEntries(int(rot%7)+1),
			WithMaxChunkBytes(int(rot)*16+1),
		)
		if err != nil {
			t.Fatal(err)
		}
		entries := int(n % 64)
		var want uint64
		for _, ns := range []string{"a/v0", "b/v0"} {
			for i := range entries {
				key := []byte{byte(i)} // strictly ascending, fixed length
				var value []byte
				if len(data) > 0 {
					value = data[i*len(data)/(entries+1):]
				}
				if err := w.AddEntry(ns, key, value); err != nil {
					t.Fatal(err)
				}
				want++
			}
		}
		root, err := w.Close(uint64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		res, err := Verify(bytes.NewReader(buf.Bytes()), VerifyFull)
		if err != nil {
			t.Fatalf("writer output does not verify: %v", err)
		}
		if res.TotalEntries != want {
			t.Fatalf("entries %d, want %d", res.TotalEntries, want)
		}
		if entries > 0 && res.RootHash != root {
			t.Fatalf("verify root %x != writer root %x", res.RootHash, root)
		}
		m, err := ReadManifest(bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatalf("manifest not locatable: %v", err)
		}
		if m.RootHash != root {
			t.Fatalf("manifest root %x != writer root %x", m.RootHash, root)
		}
	})
}
