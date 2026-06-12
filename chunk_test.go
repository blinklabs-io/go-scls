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

func testChunk() *Chunk {
	return &Chunk{
		Seq:       0,
		Format:    ChunkFormatRaw,
		Namespace: "utxo/v0",
		KeyLen:    2,
		Entries: []Entry{
			{Key: []byte{0x01, 0x02}, Value: []byte{0x03, 0x04}},
			{Key: []byte{0x01, 0x03}, Value: []byte{0x05}},
		},
	}
}

func TestChunkRoundTrip(t *testing.T) {
	in := testChunk()
	payload, err := encodeChunk(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := decodeChunk(payload)
	if err != nil {
		t.Fatal(err)
	}
	if out.Seq != in.Seq || out.Format != in.Format ||
		out.Namespace != in.Namespace || out.KeyLen != in.KeyLen {
		t.Fatalf("chunk fields mismatch: %+v", out)
	}
	if len(out.Entries) != 2 ||
		!bytes.Equal(out.Entries[0].Key, in.Entries[0].Key) ||
		!bytes.Equal(out.Entries[1].Value, in.Entries[1].Value) {
		t.Fatalf("entries mismatch: %+v", out.Entries)
	}
}

// Wire layout (RECONCILIATION #3/#4/#6):
// seq u64 || format u8 || len_ns u32 || ns || len_key u32 || entries || count u32 || digest
func TestChunkWireLayout(t *testing.T) {
	c := testChunk()
	payload, err := encodeChunk(c)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // seq
		0x00,                   // format raw
		0x00, 0x00, 0x00, 0x07, // len_ns
		'u', 't', 'x', 'o', '/', 'v', '0', // ns
		0x00, 0x00, 0x00, 0x02, // len_key
		0x00, 0x00, 0x00, 0x04, 0x01, 0x02, 0x03, 0x04, // entry 0
		0x00, 0x00, 0x00, 0x03, 0x01, 0x03, 0x05, // entry 1
		0x00, 0x00, 0x00, 0x02, // footer: entries_count
	}
	if !bytes.HasPrefix(payload, wantPrefix) || len(payload) != len(wantPrefix)+HashSize {
		t.Fatalf("wire %x, want prefix %x + 28-byte digest", payload, wantPrefix)
	}
}

// chunk_hash = Blake2b-224(concat(EntryDigest(ns, key, value))) — VERIFY(#6): answered
func TestChunkHash(t *testing.T) {
	c := testChunk()
	expected := hashFromHex(t, "285f2339f2320e962c46bd46bfbbe024ac5530ea2b35a335ea8eb59d")
	if got := c.ComputeHash(); got != expected {
		t.Fatalf("got %x, want %x", got, expected)
	}
	payload, err := encodeChunk(c)
	if err != nil {
		t.Fatal(err)
	}
	out, err := decodeChunk(payload)
	if err != nil {
		t.Fatal(err)
	}
	if out.DeclaredHash() != expected {
		t.Fatalf("declared hash %x, want %x", out.DeclaredHash(), expected)
	}
}

func TestChunkUnsupportedFormat(t *testing.T) {
	c := testChunk()
	c.Format = ChunkFormatZstd
	if _, err := encodeChunk(c); !errors.Is(err, ErrUnsupportedChunkFormat) {
		t.Fatalf("encode: want ErrUnsupportedChunkFormat, got %v", err)
	}
}

func TestChunkInconsistentKeyLength(t *testing.T) {
	c := testChunk()
	c.KeyLen = 3
	if _, err := encodeChunk(c); !errors.Is(err, ErrKeyLength) {
		t.Fatalf("encode: want ErrKeyLength, got %v", err)
	}
}

func TestChunkCorruptedEntryDetectedByHash(t *testing.T) {
	payload, err := encodeChunk(testChunk())
	if err != nil {
		t.Fatal(err)
	}
	// flip a byte inside the first entry's value region
	payload[len(payload)-40] ^= 0xff
	out, err := decodeChunk(payload)
	if err != nil {
		// corruption may also break parsing; either failure mode is acceptable
		return
	}
	if out.ComputeHash() == out.DeclaredHash() {
		t.Fatal("corruption not detected by chunk hash")
	}
}
