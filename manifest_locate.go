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
	"io"
)

// manifestMinRecordLen is the smallest possible MANIFEST record length
// including the type byte (all strings empty, no namespaces):
// type(1) + slot/total_entries/total_chunks(24) + 3 empty tstr lengths(12) +
// list sentinel(4) + prev_manifest(8) + root_hash(28) + offset(4).
const manifestMinRecordLen = 1 + 24 + 12 + 4 + 8 + 28 + 4

// ReadManifest locates and decodes the MANIFEST of a non-amended SCLS file
// without scanning it, using the trailing u32 offset bookend: the last four
// bytes of the file equal the manifest record's size prefix (RECONCILIATION
// #7). The manifest must be the file's final record, which is always the
// case for files this library writes.
//
// Failures to locate or parse a trailing manifest are reported as
// ErrMissingManifest (wrapping the cause where there is one).
func ReadManifest(r io.ReadSeeker) (*Manifest, error) {
	end, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	if end < 4+manifestMinRecordLen {
		return nil, fmt.Errorf("%w: file too small for a trailing manifest", ErrMissingManifest)
	}
	if _, err := r.Seek(end-4, io.SeekStart); err != nil {
		return nil, err
	}
	var tail [4]byte
	if _, err := io.ReadFull(r, tail[:]); err != nil {
		return nil, err
	}
	offset := int64(binary.BigEndian.Uint32(tail[:]))
	start := end - 4 - offset
	if offset < manifestMinRecordLen || start < 0 {
		return nil, fmt.Errorf("%w: implausible manifest offset %d", ErrMissingManifest, offset)
	}
	if _, err := r.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	recType, payload, err := readRecord(r)
	if err != nil {
		return nil, fmt.Errorf("%w: record at offset %d: %w", ErrMissingManifest, start, err)
	}
	if recType != RecordTypeManifest {
		return nil, fmt.Errorf("%w: record at offset %d has type 0x%02x",
			ErrMissingManifest, start, recType)
	}
	// The record must end exactly at the file's bookend, i.e. its size
	// prefix (type byte + payload) equals the bookend value. Otherwise the
	// bookend points at an earlier, non-trailing manifest with other data
	// appended after it.
	if int64(len(payload))+1 != offset {
		return nil, fmt.Errorf("%w: record at offset %d is not the trailing record",
			ErrMissingManifest, start)
	}
	m, err := decodeManifest(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: record at offset %d: %w", ErrMissingManifest, start, err)
	}
	return m, nil
}
