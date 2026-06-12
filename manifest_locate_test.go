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
	"os"
	"testing"
)

func TestReadManifestFromWriterOutput(t *testing.T) {
	file := writeTestFile(t)
	m, err := ReadManifest(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	if m.SlotNo != 42 || m.TotalEntries != 4 || m.TotalChunks != 3 {
		t.Fatalf("manifest %+v", m)
	}
	if len(m.Namespaces) != 2 || m.Namespaces[0].Name != "pool_stake/v0" ||
		m.Namespaces[1].Name != "utxo/v0" {
		t.Fatalf("namespaces %+v", m.Namespaces)
	}
}

// The reference fixture's manifest must be locatable via the offset bookend.
func TestReadManifestFromReferenceFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/minimal-raw.scls")
	if err != nil {
		t.Fatal(err)
	}
	m, err := ReadManifest(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if m.TotalEntries != 1 || m.TotalChunks != 1 {
		t.Fatalf("manifest %+v", m)
	}
	if m.Summary.Tool != "scls-tool:reference" {
		t.Fatalf("tool %q", m.Summary.Tool)
	}
}

func TestReadManifestEmptyWriterFile(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Close(9); err != nil {
		t.Fatal(err)
	}
	m, err := ReadManifest(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if m.SlotNo != 9 || len(m.Namespaces) != 0 {
		t.Fatalf("manifest %+v", m)
	}
}

func TestReadManifestRejectsGarbage(t *testing.T) {
	cases := map[string][]byte{
		"empty":            {},
		"too short":        {0x01, 0x02},
		"zero offset":      {'S', 'C', 'L', 'S', 0x00, 0x00, 0x00, 0x00},
		"offset too large": {'S', 'C', 'L', 'S', 0xff, 0xff, 0xff, 0xff},
		// offset points at bytes that are not a manifest record
		"not a manifest": append(bytes.Repeat([]byte{0xaa}, 40), 0x00, 0x00, 0x00, 0x10),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ReadManifest(bytes.NewReader(data)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

// A tampered offset that points at a valid-looking but wrong location must
// not be accepted: the record found there has to parse as a MANIFEST whose
// own offset field agrees.
func TestReadManifestRejectsTamperedOffset(t *testing.T) {
	file := writeTestFile(t)
	file[len(file)-1]++ // shift the bookend by one byte
	if _, err := ReadManifest(bytes.NewReader(file)); err == nil {
		t.Fatal("expected error")
	}
}

// A bookend that points back at a valid but non-trailing MANIFEST (data was
// appended after it) must be rejected: ReadManifest only locates the file
// trailer. (For a trailing manifest the payload's embedded offset field and
// the file bookend are the same four bytes, so they cannot disagree; what
// must be checked is that the located record ends exactly at EOF.)
func TestReadManifestRejectsNonTrailingManifest(t *testing.T) {
	file := writeTestFile(t)
	manifestStart := len(file) - 4 - int(binary.BigEndian.Uint32(file[len(file)-4:]))
	// Append junk, then a bookend pointing back at the original manifest.
	file = append(file, 0xaa, 0xbb, 0xcc, 0xdd)
	var bookend [4]byte
	binary.BigEndian.PutUint32(bookend[:], uint32(len(file)+4-4-manifestStart)) //nolint:gosec // small test sizes
	file = append(file, bookend[:]...)
	if _, err := ReadManifest(bytes.NewReader(file)); !errors.Is(err, ErrMissingManifest) {
		t.Fatalf("want ErrMissingManifest, got %v", err)
	}
}

func TestReadManifestMissingManifest(t *testing.T) {
	// header-only file: last 4 bytes are the header version, not an offset
	var buf bytes.Buffer
	if err := writeRecord(&buf, RecordTypeHdr, (&Header{Version: FormatVersion}).encode()); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadManifest(bytes.NewReader(buf.Bytes())); !errors.Is(err, ErrMissingManifest) {
		t.Fatalf("want ErrMissingManifest, got %v", err)
	}
}
