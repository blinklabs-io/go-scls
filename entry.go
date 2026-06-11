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
	"encoding/binary"
	"fmt"
)

// Entry is a single key-value pair within a namespace. Key holds the raw key
// bytes (fixed length per namespace, carried as len_key in the chunk header).
// Value holds the caller-supplied canonical CBOR encoding of the
// namespace-specific payload, written to the wire verbatim.
type Entry struct {
	Key   []byte
	Value []byte
}

// encodeEntry frames an entry as size(u32 BE) || key || value, where size
// covers key+value. VERIFY(#4): answered — raw key bytes, no CBOR framing.
func encodeEntry(e Entry) []byte {
	bodyLen := len(e.Key) + len(e.Value)
	out := make([]byte, 4, 4+bodyLen)
	binary.BigEndian.PutUint32(out[0:4], uint32(bodyLen)) //nolint:gosec // bounded by maxRecordSize at write sites
	out = append(out, e.Key...)
	out = append(out, e.Value...)
	return out
}

// decodeEntries parses exactly count concatenated wire entries from data,
// splitting each body at keyLen (the chunk's fixed namespace key length).
// Returned Key/Value slices alias data.
func decodeEntries(data []byte, count, keyLen uint32) ([]Entry, error) {
	entries := make([]Entry, 0, int(min(int64(count), 4096)))
	off := 0
	for i := uint32(0); i < count; i++ {
		if off+4 > len(data) {
			return nil, fmt.Errorf("%w: entry %d size prefix", ErrTruncatedChunk, i)
		}
		size := binary.BigEndian.Uint32(data[off : off+4])
		off += 4
		// Validate in int64 space before converting: int(size) and
		// int(keyLen) can wrap negative on 32-bit platforms.
		if int64(size) > int64(len(data)-off) {
			return nil, fmt.Errorf("%w: entry %d body", ErrTruncatedChunk, i)
		}
		if size < keyLen {
			return nil, fmt.Errorf("%w: entry %d body shorter than key length %d",
				ErrTruncatedChunk, i, keyLen)
		}
		body := data[off : off+int(size)]
		off += int(size)
		entries = append(entries, Entry{Key: body[:keyLen], Value: body[keyLen:]})
	}
	if off != len(data) {
		return nil, ErrTrailingChunkData
	}
	return entries, nil
}
