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

func TestRecordRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte{0xde, 0xad, 0xbe, 0xef}
	if err := writeRecord(&buf, RecordTypeChunk, payload); err != nil {
		t.Fatal(err)
	}
	// size(u32 BE) || type(u8) || payload; size covers type+payload — VERIFY(#1): answered
	expected := []byte{0x00, 0x00, 0x00, 0x05, 0x10, 0xde, 0xad, 0xbe, 0xef}
	if !bytes.Equal(buf.Bytes(), expected) {
		t.Fatalf("wire bytes %x, want %x", buf.Bytes(), expected)
	}
	recType, got, err := readRecord(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if recType != RecordTypeChunk || !bytes.Equal(got, payload) {
		t.Fatalf("round trip mismatch: type=0x%02x payload=%x", recType, got)
	}
}

func TestReadRecordCleanEOF(t *testing.T) {
	_, _, err := readRecord(bytes.NewReader(nil))
	if !errors.Is(err, io.EOF) {
		t.Fatalf("want io.EOF, got %v", err)
	}
}

func TestReadRecordTruncated(t *testing.T) {
	// header promises 10 bytes (type+payload), only 3 present
	data := []byte{0x00, 0x00, 0x00, 0x0a, 0x10, 0x01, 0x02}
	_, _, err := readRecord(bytes.NewReader(data))
	if !errors.Is(err, ErrTruncatedRecord) {
		t.Fatalf("want ErrTruncatedRecord, got %v", err)
	}
}

// A zero-size record cannot contain the mandatory type byte (RECONCILIATION #1).
func TestReadRecordZeroSize(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00}
	_, _, err := readRecord(bytes.NewReader(data))
	if !errors.Is(err, ErrTruncatedRecord) {
		t.Fatalf("want ErrTruncatedRecord, got %v", err)
	}
}

// A size prefix followed by no body bytes at all is truncation, not clean
// EOF — the wrapped error must not satisfy errors.Is(err, io.EOF), or a
// dangling size prefix at the end of a file would be silently accepted.
func TestReadRecordDanglingSizePrefix(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x05}
	_, _, err := readRecord(bytes.NewReader(data))
	if !errors.Is(err, ErrTruncatedRecord) {
		t.Fatalf("want ErrTruncatedRecord, got %v", err)
	}
	if errors.Is(err, io.EOF) {
		t.Fatalf("truncation error must not match io.EOF: %v", err)
	}
}

func TestReadRecordSizeLimit(t *testing.T) {
	data := []byte{0xff, 0xff, 0xff, 0xff, 0x10}
	_, _, err := readRecord(bytes.NewReader(data))
	if !errors.Is(err, ErrRecordTooLarge) {
		t.Fatalf("want ErrRecordTooLarge, got %v", err)
	}
}

// writeRecord must refuse payloads whose size prefix would exceed the
// read-side maxRecordSize cap, so the writer cannot emit records its own
// reader rejects.
func TestWriteRecordSizeLimit(t *testing.T) {
	payload := make([]byte, maxRecordSize) // size prefix would be maxRecordSize+1
	if err := writeRecord(io.Discard, RecordTypeChunk, payload); !errors.Is(err, ErrRecordTooLarge) {
		t.Fatalf("want ErrRecordTooLarge, got %v", err)
	}
}
