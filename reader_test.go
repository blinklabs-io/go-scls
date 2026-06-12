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
	"io"
	"testing"
)

// writeTestFile builds a two-namespace file via Writer and returns its bytes.
func writeTestFile(t *testing.T) []byte {
	t.Helper()
	data, err := buildTestFile()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// buildTestFile is the testing.T-free variant used by the fuzz seed corpus.
func buildTestFile() ([]byte, error) {
	var buf bytes.Buffer
	w, err := NewWriter(&buf, WithMaxChunkEntries(2))
	if err != nil {
		return nil, err
	}
	if err := w.AddEntry("pool_stake/v0", []byte{0x0a}, []byte{0x01}); err != nil {
		return nil, err
	}
	for i := byte(0); i < 3; i++ {
		if err := w.AddEntry("utxo/v0", []byte{0x01, i}, []byte{0x05}); err != nil {
			return nil, err
		}
	}
	if _, err := w.Close(42); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestReaderStreamsChunks(t *testing.T) {
	r, err := NewReader(bytes.NewReader(writeTestFile(t)))
	if err != nil {
		t.Fatal(err)
	}
	if r.Header().Version != FormatVersion {
		t.Fatalf("header version %d", r.Header().Version)
	}
	var namespaces []string
	var entries int
	for {
		c, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		namespaces = append(namespaces, c.Namespace)
		entries += len(c.Entries)
	}
	// pool_stake: 1 chunk; utxo: 2 chunks (maxChunkEntries=2, 3 entries)
	if len(namespaces) != 3 || namespaces[0] != "pool_stake/v0" ||
		namespaces[1] != "utxo/v0" || namespaces[2] != "utxo/v0" {
		t.Fatalf("namespaces %v", namespaces)
	}
	if entries != 4 {
		t.Fatalf("entries %d, want 4", entries)
	}
	m := r.Manifest()
	if m == nil || m.SlotNo != 42 || m.TotalEntries != 4 || m.TotalChunks != 3 {
		t.Fatalf("manifest %+v", m)
	}
}

func TestReaderRejectsMissingHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := writeRecord(&buf, RecordTypeChunk, []byte{0x00}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewReader(&buf); !errors.Is(err, ErrMissingHeader) {
		t.Fatalf("want ErrMissingHeader, got %v", err)
	}
}

func TestReaderSkipsUnknownRecords(t *testing.T) {
	file := writeTestFile(t)
	// splice a META record between header and first chunk
	hdrLen := 4 + 9 // size prefix + (type byte + 8-byte header payload)
	var spliced bytes.Buffer
	spliced.Write(file[:hdrLen])
	if err := writeRecord(&spliced, RecordTypeMeta, []byte{0xa0}); err != nil {
		t.Fatal(err)
	}
	spliced.Write(file[hdrLen:])
	r, err := NewReader(&spliced)
	if err != nil {
		t.Fatal(err)
	}
	chunks := 0
	for {
		_, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		chunks++
	}
	if chunks != 3 {
		t.Fatalf("chunks %d, want 3", chunks)
	}
	if r.Skipped() != 1 {
		t.Fatalf("skipped %d, want 1", r.Skipped())
	}
}

func TestReaderMissingManifest(t *testing.T) {
	file := writeTestFile(t)
	// truncate the manifest record off the end: find its offset by scanning
	br := bytes.NewReader(file)
	var offset, manifestStart int
	for {
		recType, payload, err := readRecord(br)
		if err != nil {
			break
		}
		if recType == RecordTypeManifest {
			manifestStart = offset
		}
		offset += 5 + len(payload)
	}
	r, err := NewReader(bytes.NewReader(file[:manifestStart]))
	if err != nil {
		t.Fatal(err)
	}
	for {
		_, err = r.Next()
		if err != nil {
			break
		}
	}
	if !errors.Is(err, ErrMissingManifest) {
		t.Fatalf("want ErrMissingManifest, got %v", err)
	}
}
