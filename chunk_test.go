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
	"fmt"
	"reflect"
	"strings"
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
	payload, err := encodeChunkLevel(in, 0)
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
	payload, err := encodeChunkLevel(c, 0)
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
	payload, err := encodeChunkLevel(c, 0)
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
	c.Format = 0xff
	if _, err := encodeChunkLevel(c, 0); !errors.Is(err, ErrUnsupportedChunkFormat) {
		t.Fatalf("encode: want ErrUnsupportedChunkFormat, got %v", err)
	}
}

func TestChunkCompressedFormatsRoundTrip(t *testing.T) {
	t.Parallel()
	want := testChunk()
	wantHash := want.ComputeHash()
	for _, format := range []byte{ChunkFormatZstd, ChunkFormatZstdEntries} {
		format := format
		t.Run(fmt.Sprintf("0x%02x", format), func(t *testing.T) {
			in := *want
			in.Format = format
			payload, err := encodeChunkLevel(&in, 0)
			if err != nil {
				t.Fatal(err)
			}
			got, err := decodeChunk(payload)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Entries, want.Entries) {
				t.Fatalf("entries differ: %#v", got.Entries)
			}
			if got.ComputeHash() != wantHash {
				t.Fatal("compression changed chunk hash")
			}
		})
	}
}

func TestChunkCompressedFormatsRejectOversizedDecodedEntries(t *testing.T) {
	value := bytes.Repeat([]byte{0}, maxRecordSize)
	for _, format := range []byte{ChunkFormatZstd, ChunkFormatZstdEntries} {
		c := &Chunk{
			Format:    format,
			Namespace: "a",
			KeyLen:    1,
			Entries:   []Entry{{Key: []byte{0}, Value: value}},
		}
		if _, err := encodeChunkLevel(c, 0); !errors.Is(err, ErrRecordTooLarge) {
			t.Fatalf("format 0x%02x: want ErrRecordTooLarge, got %v", format, err)
		}
	}
}

func TestChunkZstdRejectsOutOfRangeSeekFrame(t *testing.T) {
	c := testChunk()
	c.Format = ChunkFormatZstd
	payload, err := encodeChunkLevel(c, 0)
	if err != nil {
		t.Fatal(err)
	}
	lenNS := int(binary.BigEndian.Uint32(payload[9:13]))
	dataStart := chunkHeaderFixedSize + lenNS
	data := payload[dataStart : len(payload)-chunkFooterSize]
	seekTable := bytes.LastIndex(data, []byte{0x5e, 0x2a, 0x4d, 0x18})
	if seekTable < 0 {
		t.Fatal("seek table not found")
	}
	binary.LittleEndian.PutUint32(payload[dataStart+seekTable+8:], uint32(len(data)+1))
	_, err = decodeChunk(payload)
	if !errors.Is(err, ErrCompressedChunk) || !strings.Contains(err.Error(), "exceeds compressed data") {
		t.Fatalf("want bounded seek-table ErrCompressedChunk, got %v", err)
	}
}

func TestDecodeCompressedEntriesAllowsSmallRemainingBudget(t *testing.T) {
	enc, err := newZstdEncoder(0)
	if err != nil {
		t.Fatal(err)
	}
	defer enc.Close()
	first := encodeEntry(Entry{Key: []byte{0}, Value: enc.EncodeAll(bytes.Repeat([]byte{1}, 80), nil)})
	second := encodeEntry(Entry{Key: []byte{1}, Value: enc.EncodeAll([]byte{2}, nil)})
	data := append(first, second...)
	entries, err := decodeCompressedEntriesLimit(data, 2, 1, 91)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || len(entries[0].Value) != 80 || !bytes.Equal(entries[1].Value, []byte{2}) {
		t.Fatalf("unexpected decoded entries: %#v", entries)
	}
}

func TestChunkCompressedFormatsRejectTruncation(t *testing.T) {
	t.Parallel()
	for _, format := range []byte{ChunkFormatZstd, ChunkFormatZstdEntries} {
		c := testChunk()
		c.Format = format
		payload, err := encodeChunkLevel(c, 0)
		if err != nil {
			t.Fatal(err)
		}
		footer := bytes.Clone(payload[len(payload)-chunkFooterSize:])
		bad := append(bytes.Clone(payload[:len(payload)-chunkFooterSize-1]), footer...)
		if _, err := decodeChunk(bad); !errors.Is(err, ErrCompressedChunk) {
			t.Fatalf("format 0x%02x: want ErrCompressedChunk, got %v", format, err)
		}
		rec := chunkRecord(t, c)
		rec.payload = bad
		file := buildRawFile(t, rec, manifestRecord(t, &Manifest{TotalEntries: 2, TotalChunks: 1}))
		if _, err := Verify(bytes.NewReader(file), VerifyFull); !errors.Is(err, ErrCompressedChunk) {
			t.Fatalf("format 0x%02x Verify: want ErrCompressedChunk, got %v", format, err)
		}
	}
}

func TestChunkCompressedFormatsDetectCorruption(t *testing.T) {
	t.Parallel()
	for _, format := range []byte{ChunkFormatZstd, ChunkFormatZstdEntries} {
		c := testChunk()
		c.Format = format
		rec := chunkRecord(t, c)
		rec.payload[len(rec.payload)-chunkFooterSize-1] ^= 0x40
		file := buildRawFile(t, rec, manifestRecord(t, &Manifest{TotalEntries: 2, TotalChunks: 1}))
		if _, err := Verify(bytes.NewReader(file), VerifyFull); !errors.Is(err, ErrCompressedChunk) {
			t.Fatalf("format 0x%02x: want ErrCompressedChunk, got %v", format, err)
		}
	}
}

func FuzzCompressedChunk(f *testing.F) {
	for _, format := range []byte{ChunkFormatZstd, ChunkFormatZstdEntries} {
		c := testChunk()
		c.Format = format
		payload, err := encodeChunkLevel(c, 0)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(payload)
	}
	f.Fuzz(func(t *testing.T, payload []byte) { _, _ = decodeChunk(payload) })
}

func TestChunkInconsistentKeyLength(t *testing.T) {
	c := testChunk()
	c.KeyLen = 3
	if _, err := encodeChunkLevel(c, 0); !errors.Is(err, ErrKeyLength) {
		t.Fatalf("encode: want ErrKeyLength, got %v", err)
	}
}

func TestChunkCorruptedEntryDetectedByHash(t *testing.T) {
	payload, err := encodeChunkLevel(testChunk(), 0)
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

func TestCompressedEntryWindowAndOutputBudgets(t *testing.T) {
	enc, err := newZstdEncoder(0)
	if err != nil {
		t.Fatal(err)
	}
	defer enc.Close()
	value := bytes.Repeat([]byte{2}, 1024)
	data := encodeEntry(Entry{Key: []byte{0}, Value: enc.EncodeAll(value, nil)})
	limit := 4 + 1 + len(value)
	for _, budget := range []int{limit - 1, limit, limit + 1} {
		entries, err := decodeCompressedEntriesLimit(data, 1, 1, budget)
		if budget < limit {
			if !errors.Is(err, ErrRecordTooLarge) {
				t.Fatalf("budget %d: want ErrRecordTooLarge, got %v", budget, err)
			}
			continue
		}
		if err != nil || len(entries) != 1 || !bytes.Equal(entries[0].Value, value) {
			t.Fatalf("budget %d: valid compressed entry rejected: %v", budget, err)
		}
	}
}
