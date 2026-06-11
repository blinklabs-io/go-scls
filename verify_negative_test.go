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

// rawRecord is a pre-encoded record payload for hand-assembling test files.
type rawRecord struct {
	typ     byte
	payload []byte
}

// buildRawFile assembles an SCLS byte stream from a HDR record followed by
// the given records.
func buildRawFile(t *testing.T, recs ...rawRecord) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := writeRecord(&buf, RecordTypeHdr, (&Header{Version: FormatVersion}).encode()); err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		if err := writeRecord(&buf, r.typ, r.payload); err != nil {
			t.Fatal(err)
		}
	}
	return buf.Bytes()
}

func chunkRecord(t *testing.T, c *Chunk) rawRecord {
	t.Helper()
	payload, err := encodeChunk(c)
	if err != nil {
		t.Fatal(err)
	}
	return rawRecord{typ: RecordTypeChunk, payload: payload}
}

func manifestRecord(t *testing.T, m *Manifest) rawRecord {
	t.Helper()
	payload, err := encodeManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	return rawRecord{typ: RecordTypeManifest, payload: payload}
}

// singleEntryChunk builds a one-entry chunk for namespace ns with the given
// sequence number and single-byte key.
func singleEntryChunk(ns string, seq uint64, key byte) *Chunk {
	return &Chunk{
		Seq:       seq,
		Format:    ChunkFormatRaw,
		Namespace: ns,
		KeyLen:    1,
		Entries:   []Entry{{Key: []byte{key}, Value: []byte{0x05}}},
	}
}

// The spec does not pin the starting sequence number (the Haskell reference
// writes from 1, RECONCILIATION #9) — only strict increase is required.
func TestVerifyAcceptsSeqStartingAtOne(t *testing.T) {
	file := buildRawFile(t,
		chunkRecord(t, singleEntryChunk("a", 1, 0x01)),
		manifestRecord(t, &Manifest{
			TotalEntries: 1,
			TotalChunks:  1,
			Namespaces:   []NamespaceInfo{{Name: "a", EntriesCount: 1, ChunksCount: 1}},
		}),
	)
	if _, err := Verify(bytes.NewReader(file), VerifyStructure); err != nil {
		t.Fatalf("seq starting at 1 should pass VerifyStructure: %v", err)
	}
}

func TestVerifyRejectsNonIncreasingSeq(t *testing.T) {
	file := buildRawFile(t,
		chunkRecord(t, singleEntryChunk("a", 1, 0x01)),
		chunkRecord(t, singleEntryChunk("a", 1, 0x02)), // same seq again
		manifestRecord(t, &Manifest{
			TotalEntries: 2,
			TotalChunks:  2,
			Namespaces:   []NamespaceInfo{{Name: "a", EntriesCount: 2, ChunksCount: 2}},
		}),
	)
	if _, err := Verify(bytes.NewReader(file), VerifyStructure); !errors.Is(err, ErrChunkSequence) {
		t.Fatalf("want ErrChunkSequence, got %v", err)
	}
}

// Seq resets on namespace change: 0,1 in "a" then 0 again in "b" is valid.
func TestVerifyAcceptsSeqResetAcrossNamespaces(t *testing.T) {
	file := buildRawFile(t,
		chunkRecord(t, singleEntryChunk("a", 0, 0x01)),
		chunkRecord(t, singleEntryChunk("a", 1, 0x02)),
		chunkRecord(t, singleEntryChunk("b", 0, 0x01)),
		manifestRecord(t, &Manifest{
			TotalEntries: 3,
			TotalChunks:  3,
			Namespaces: []NamespaceInfo{
				{Name: "a", EntriesCount: 2, ChunksCount: 2},
				{Name: "b", EntriesCount: 1, ChunksCount: 1},
			},
		}),
	)
	if _, err := Verify(bytes.NewReader(file), VerifyStructure); err != nil {
		t.Fatalf("per-namespace seq reset should pass: %v", err)
	}
}

// Keys must keep ascending across chunk boundaries within a namespace.
func TestVerifyRejectsDescendingKeysAcrossChunks(t *testing.T) {
	file := buildRawFile(t,
		chunkRecord(t, singleEntryChunk("a", 0, 0x02)),
		chunkRecord(t, singleEntryChunk("a", 1, 0x01)), // key goes backwards
		manifestRecord(t, &Manifest{
			TotalEntries: 2,
			TotalChunks:  2,
			Namespaces:   []NamespaceInfo{{Name: "a", EntriesCount: 2, ChunksCount: 2}},
		}),
	)
	if _, err := Verify(bytes.NewReader(file), VerifyStructure); !errors.Is(err, ErrEntryOrder) {
		t.Fatalf("want ErrEntryOrder, got %v", err)
	}
}

// The set of namespaces in chunks must match the manifest exactly.
func TestVerifyRejectsNamespaceSetMismatch(t *testing.T) {
	file := buildRawFile(t,
		chunkRecord(t, singleEntryChunk("a", 0, 0x01)),
		manifestRecord(t, &Manifest{
			TotalEntries: 1,
			TotalChunks:  1,
			Namespaces:   []NamespaceInfo{{Name: "b", EntriesCount: 1, ChunksCount: 1}},
		}),
	)
	if _, err := Verify(bytes.NewReader(file), VerifyStructure); !errors.Is(err, ErrCountMismatch) {
		t.Fatalf("want ErrCountMismatch, got %v", err)
	}
}

func TestVerifyRejectsDuplicateManifestNamespace(t *testing.T) {
	file := buildRawFile(t,
		chunkRecord(t, singleEntryChunk("a", 0, 0x01)),
		manifestRecord(t, &Manifest{
			TotalEntries: 1,
			TotalChunks:  1,
			Namespaces: []NamespaceInfo{
				{Name: "a", EntriesCount: 1, ChunksCount: 1},
				{Name: "a", EntriesCount: 1, ChunksCount: 1},
			},
		}),
	)
	if _, err := Verify(bytes.NewReader(file), VerifyStructure); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("want ErrInvalidManifest, got %v", err)
	}
}

func TestReaderRejectsDelta(t *testing.T) {
	file := buildRawFile(t, rawRecord{typ: RecordTypeDelta, payload: []byte{0x00}})
	r, err := NewReader(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Next(); !errors.Is(err, ErrUnexpectedRecord) {
		t.Fatalf("want ErrUnexpectedRecord, got %v", err)
	}
}

func TestReaderRejectsMultipleManifests(t *testing.T) {
	m := &Manifest{}
	file := buildRawFile(t, manifestRecord(t, m), manifestRecord(t, m))
	r, err := NewReader(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Next(); !errors.Is(err, ErrUnexpectedRecord) {
		t.Fatalf("want ErrUnexpectedRecord, got %v", err)
	}
}

func TestReaderRejectsChunkAfterManifest(t *testing.T) {
	file := buildRawFile(t,
		manifestRecord(t, &Manifest{}),
		chunkRecord(t, singleEntryChunk("a", 0, 0x01)),
	)
	r, err := NewReader(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Next(); !errors.Is(err, ErrUnexpectedRecord) {
		t.Fatalf("want ErrUnexpectedRecord, got %v", err)
	}
}

func TestReaderRejectsDuplicateHeader(t *testing.T) {
	file := buildRawFile(t,
		rawRecord{typ: RecordTypeHdr, payload: (&Header{Version: FormatVersion}).encode()},
	)
	r, err := NewReader(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Next(); !errors.Is(err, ErrMissingHeader) {
		t.Fatalf("want ErrMissingHeader, got %v", err)
	}
}
