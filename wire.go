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
	"errors"
	"fmt"
	"io"
)

// SCLS record type tags (CIP-0165).
const (
	RecordTypeHdr      byte = 0x00
	RecordTypeManifest byte = 0x01
	RecordTypeChunk    byte = 0x10
	RecordTypeDelta    byte = 0x11
	RecordTypeBloom    byte = 0x20
	RecordTypeIndex    byte = 0x21
	RecordTypeDir      byte = 0x30
	RecordTypeMeta     byte = 0x31
)

// maxRecordSize bounds a single record (type byte + payload) to prevent
// unbounded allocations from a corrupt size prefix.
const maxRecordSize = 64 << 20 // 64 MiB

// writeRecord frames a record as size(u32 BE) || type(u8) || payload, where
// size covers the type byte and payload (VERIFY(#1): answered, size = 1+len).
func writeRecord(w io.Writer, recType byte, payload []byte) error {
	if len(payload) >= maxRecordSize { // size = 1 + len(payload) must fit the read-side cap
		return fmt.Errorf("%w: %d bytes", ErrRecordTooLarge, len(payload)+1)
	}
	var hdr [5]byte
	binary.BigEndian.PutUint32(hdr[0:4], uint32(len(payload))+1) //nolint:gosec // bounded by the maxRecordSize check above
	hdr[4] = recType
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

// readRecord reads the next framed record. Returns io.EOF (unwrapped) at a
// clean end of stream; a partial size prefix or body returns ErrTruncatedRecord.
func readRecord(r io.Reader) (byte, []byte, error) {
	var szBuf [4]byte
	if _, err := io.ReadFull(r, szBuf[:]); err != nil {
		if err == io.ErrUnexpectedEOF {
			return 0, nil, fmt.Errorf("%w: partial record size", ErrTruncatedRecord)
		}
		return 0, nil, err // io.EOF: clean end of stream
	}
	size := binary.BigEndian.Uint32(szBuf[:])
	if size == 0 {
		return 0, nil, fmt.Errorf("%w: zero-size record", ErrTruncatedRecord)
	}
	if size > maxRecordSize {
		return 0, nil, fmt.Errorf("%w: %d bytes", ErrRecordTooLarge, size)
	}
	body := make([]byte, size) // type byte + payload
	if _, err := io.ReadFull(r, body); err != nil {
		if errors.Is(err, io.EOF) {
			// zero body bytes after a size prefix is still truncation; do
			// not let the wrapped error satisfy errors.Is(err, io.EOF)
			err = io.ErrUnexpectedEOF
		}
		return 0, nil, fmt.Errorf("%w: partial record body: %w", ErrTruncatedRecord, err)
	}
	return body[0], body[1:], nil
}
