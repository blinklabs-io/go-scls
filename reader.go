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
	"errors"
	"fmt"
	"io"
)

// Reader streams an SCLS file: it validates the HDR record on construction,
// then yields CHUNK records via Next, skipping reserved/unknown record types.
// After Next returns io.EOF, Manifest() returns the parsed MANIFEST.
//
// v0.1 limitation: files with DELTA records or multiple MANIFEST records
// (appended snapshots) are rejected.
type Reader struct {
	r        io.Reader
	header   *Header
	manifest *Manifest
	skipped  int
	done     bool
}

// NewReader reads and validates the HDR record.
func NewReader(r io.Reader) (*Reader, error) {
	recType, payload, err := readRecord(r)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, ErrMissingHeader
		}
		return nil, err
	}
	if recType != RecordTypeHdr {
		return nil, fmt.Errorf("%w: first record type 0x%02x", ErrMissingHeader, recType)
	}
	hdr, err := decodeHeader(payload)
	if err != nil {
		return nil, err
	}
	return &Reader{r: r, header: hdr}, nil
}

// Header returns the validated HDR record.
func (sr *Reader) Header() *Header { return sr.header }

// Skipped reports how many reserved/unknown records were skipped so far.
func (sr *Reader) Skipped() int { return sr.skipped }

// Manifest returns the MANIFEST; non-nil only after Next returned io.EOF.
func (sr *Reader) Manifest() *Manifest { return sr.manifest }

// Next returns the next CHUNK. It returns io.EOF after the MANIFEST has been
// read and the stream has cleanly ended. A stream ending without a MANIFEST
// returns ErrMissingManifest.
func (sr *Reader) Next() (*Chunk, error) {
	if sr.done {
		return nil, io.EOF
	}
	for {
		recType, payload, err := readRecord(sr.r)
		if errors.Is(err, io.EOF) {
			if sr.manifest == nil {
				return nil, ErrMissingManifest
			}
			sr.done = true
			return nil, io.EOF
		}
		if err != nil {
			return nil, err
		}
		switch recType {
		case RecordTypeChunk:
			if sr.manifest != nil {
				return nil, fmt.Errorf("%w: CHUNK after MANIFEST", ErrUnexpectedRecord)
			}
			return decodeChunk(payload)
		case RecordTypeManifest:
			if sr.manifest != nil {
				return nil, fmt.Errorf("%w: multiple MANIFEST records", ErrUnexpectedRecord)
			}
			m, err := decodeManifest(payload)
			if err != nil {
				return nil, err
			}
			sr.manifest = m
		case RecordTypeHdr:
			return nil, fmt.Errorf("%w: duplicate HDR", ErrMissingHeader)
		case RecordTypeDelta:
			// Skipping deltas would verify a live-set different from the
			// file's true content, so reject explicitly (RECONCILIATION E4).
			return nil, fmt.Errorf("%w: DELTA records not supported in v0.1", ErrUnexpectedRecord)
		default:
			// reserved/unknown record types are skipped (forward compatibility)
			sr.skipped++
		}
	}
}
