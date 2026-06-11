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

func TestEntryRoundTrip(t *testing.T) {
	in := []Entry{
		{Key: []byte{0x01, 0x02}, Value: []byte{0x43, 0x03, 0x04, 0x05}}, // value = CBOR bstr h'030405'
		{Key: []byte{0x01, 0x03}, Value: []byte{0x05}},                   // value = CBOR uint 5
	}
	var buf bytes.Buffer
	for _, e := range in {
		buf.Write(encodeEntry(e))
	}
	out, err := decodeEntries(buf.Bytes(), 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d entries", len(out))
	}
	for i := range in {
		if !bytes.Equal(out[i].Key, in[i].Key) || !bytes.Equal(out[i].Value, in[i].Value) {
			t.Fatalf("entry %d mismatch: %+v vs %+v", i, out[i], in[i])
		}
	}
}

// size(u32) covers key+value; key is raw bytes of the namespace's fixed
// length, value follows verbatim. VERIFY(#4): answered.
func TestEntryWireLayout(t *testing.T) {
	enc := encodeEntry(Entry{Key: []byte{0x01, 0x02}, Value: []byte{0x05}})
	// key h'0102' (2 bytes) + value 0x05 (1 byte) => size 3
	expected := []byte{0x00, 0x00, 0x00, 0x03, 0x01, 0x02, 0x05}
	if !bytes.Equal(enc, expected) {
		t.Fatalf("wire %x, want %x", enc, expected)
	}
}

func TestEntryTruncated(t *testing.T) {
	// promises a 9-byte body, only 1 present
	_, err := decodeEntries([]byte{0x00, 0x00, 0x00, 0x09, 0x42}, 1, 1)
	if !errors.Is(err, ErrTruncatedChunk) {
		t.Fatalf("want ErrTruncatedChunk, got %v", err)
	}
}

// body shorter than the namespace key length is malformed
func TestEntryBodyShorterThanKey(t *testing.T) {
	// len_body=2 but keyLen=4
	_, err := decodeEntries([]byte{0x00, 0x00, 0x00, 0x02, 0xff, 0xff}, 1, 4)
	if !errors.Is(err, ErrTruncatedChunk) {
		t.Fatalf("want ErrTruncatedChunk, got %v", err)
	}
}

func TestEntryTrailingData(t *testing.T) {
	enc := encodeEntry(Entry{Key: []byte{0x01}, Value: []byte{0x05}})
	_, err := decodeEntries(append(enc, 0xff), 1, 1)
	if !errors.Is(err, ErrTrailingChunkData) {
		t.Fatalf("want ErrTrailingChunkData, got %v", err)
	}
}

func TestEntryCountExceedsData(t *testing.T) {
	enc := encodeEntry(Entry{Key: []byte{0x01}, Value: []byte{0x05}})
	_, err := decodeEntries(enc, 2, 1)
	if !errors.Is(err, ErrTruncatedChunk) {
		t.Fatalf("want ErrTruncatedChunk, got %v", err)
	}
}
