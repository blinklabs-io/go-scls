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

package cbor

import (
	"encoding/binary"
	"errors"
	"reflect"
	"runtime"
	"testing"
)

// collectionHeader encodes major type major with count, choosing the shortest
// argument form so the result survives the deterministic minimality check.
func collectionHeader(major byte, count uint64) []byte {
	switch {
	case count < 24:
		return []byte{major | byte(count)}
	case count <= 0xff:
		return []byte{major | 24, byte(count)}
	case count <= 0xffff:
		header := []byte{major | 25, 0, 0}
		binary.BigEndian.PutUint16(header[1:], uint16(count))
		return header
	default:
		header := []byte{major | 26, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(header[1:], uint32(count))
		return header
	}
}

// amplifierInput builds maxDepth nested collection headers, each declaring the
// largest element count collectionLength will still accept, followed by filler.
// Every level's reservation stays live while the next level is decoded, so a
// decoder that sizes its allocation from the declared count multiplies the
// input length by the slot size and again by the nesting depth.
func amplifierInput(total int, major byte, minItemSize uint64) []byte {
	out := make([]byte, 0, total)
	for range maxDepth {
		remaining := uint64(total-len(out)) / minItemSize
		count := uint64(0xffff)
		if remaining > 5 {
			count = min(count, remaining-5)
		}
		out = append(out, collectionHeader(major, count)...)
	}
	for len(out) < total {
		out = append(out, 0x00)
	}
	return out
}

// decodeBytesAllocated reports the heap bytes allocated while decoding data.
//
// TotalAlloc is used rather than testing.AllocsPerRun because the defect under
// test is bytes per allocation, not the number of allocations: one make of an
// arbitrary size is a single allocation either way. It is also independent of
// when the collector runs, unlike HeapAlloc. Nothing the decoder allocates is
// freed before Unmarshal returns, so the cumulative figure is an upper bound
// on the peak live heap, which is the conservative direction for a ceiling.
//
// MemStats is process-wide, so callers must not run in parallel.
func decodeBytesAllocated(data []byte) uint64 {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	value, err := Unmarshal(data)
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(value)
	runtime.KeepAlive(err)
	return after.TotalAlloc - before.TotalAlloc
}

func TestDecodeAllocationBounded(t *testing.T) {
	// No t.Parallel: decodeBytesAllocated reads process-wide statistics.
	tests := []struct {
		name        string
		major       byte
		minItemSize uint64
		slotSize    uint64
	}{
		{name: "array", major: 0x80, minItemSize: 1, slotSize: 16},
		{name: "map", major: 0xa0, minItemSize: 2, slotSize: 32},
	}
	for _, test := range tests {
		for _, size := range []int{4 << 10, 64 << 10} {
			data := amplifierInput(size, test.major, test.minItemSize)
			// The decoder may reserve maxPrealloc slots at each level, and
			// the outermost value sits at depth 0, so maxDepth+1 levels can
			// be live at once. Everything beyond that must track bytes
			// actually decoded; allow a generous slot per input byte for the
			// values the filler decodes to.
			limit := uint64(maxDepth+1)*maxPrealloc*test.slotSize +
				uint64(len(data))*128
			got := decodeBytesAllocated(data)
			if got > limit {
				t.Errorf("%s/%d: decoding %d bytes allocated %d, want <= %d (%.0fx amplification)",
					test.name, size, len(data), got, limit,
					float64(got)/float64(len(data)))
			}
		}
	}
}

func TestDecodeRejectsUnsatisfiableCollection(t *testing.T) {
	t.Parallel()
	// ascending holds 24 distinct single-byte CBOR unsigned integers, so a
	// map built from them has strictly ascending keys and cannot be rejected
	// by the duplicate-key or ordering rules first.
	ascending := make([]byte, 24)
	for i := range ascending {
		ascending[i] = byte(i)
	}
	tests := []struct {
		name string
		data []byte
	}{
		{
			// 24 map entries need at least 48 bytes; 24 follow. Checking the
			// count against the remaining bytes alone accepts this and
			// reserves capacity for 24 entries before decoding any.
			name: "map count exceeds half the remaining input",
			data: append([]byte{0xb8, 0x18}, ascending...),
		},
		{
			// 24 array items need at least 24 bytes; 23 follow.
			name: "array count exceeds the remaining input",
			data: append([]byte{0x98, 0x18}, ascending[:23]...),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Unmarshal(test.data)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("Unmarshal() error = %v, want ErrInvalid", err)
			}
			var syntaxErr *SyntaxError
			if !errors.As(err, &syntaxErr) {
				t.Fatalf("Unmarshal() error = %v, want *SyntaxError", err)
			}
			// Rejected at the header, before any element was decoded.
			if syntaxErr.Offset > 2 {
				t.Errorf("SyntaxError.Offset = %d, want <= 2", syntaxErr.Offset)
			}
		})
	}
}

func TestDecodeMinimumSizedCollections(t *testing.T) {
	t.Parallel()
	// 12 entries of a one-byte key and a one-byte value occupy exactly the
	// two bytes per entry that collectionLength now requires.
	data := []byte{0xac}
	want := make(Map, 0, 12)
	for i := range 12 {
		data = append(data, byte(2*i), byte(2*i+1))
		want = append(want, MapEntry{Key: uint64(2 * i), Value: uint64(2*i + 1)})
	}
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Unmarshal() = %v, want %v", got, want)
	}
}

func TestDecodeCollectionsLargerThanPrealloc(t *testing.T) {
	t.Parallel()
	sizes := []int{maxPrealloc - 1, maxPrealloc, maxPrealloc + 1, maxPrealloc * 4}
	for _, size := range sizes {
		array := make(Array, 0, size)
		entries := make(Map, 0, size)
		for i := range size {
			array = append(array, uint64(i))
			entries = append(entries, MapEntry{Key: uint64(i), Value: uint64(i)})
		}
		for name, want := range map[string]any{"array": array, "map": entries} {
			encoded, err := Marshal(want)
			if err != nil {
				t.Fatalf("Marshal(%s/%d) error = %v", name, size, err)
			}
			got, err := Unmarshal(encoded)
			if err != nil {
				t.Fatalf("Unmarshal(%s/%d) error = %v", name, size, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Unmarshal(%s/%d) did not round-trip", name, size)
			}
		}
	}
}
