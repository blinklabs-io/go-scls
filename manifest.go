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
	"fmt"
	"unicode/utf8"
)

// NamespaceInfo summarizes one namespace in the MANIFEST.
type NamespaceInfo struct {
	Name         string
	EntriesCount uint64
	ChunksCount  uint64
	Digest       Hash // per-namespace Merkle root
}

// ManifestSummary holds the three length-prefixed summary strings in the
// MANIFEST. An empty Comment is encoded as a zero-length string (= absent).
type ManifestSummary struct {
	CreatedAt string // ISO 8601 / RFC 3339 timestamp
	Tool      string
	Comment   string
}

// Manifest is the MANIFEST record payload (RECONCILIATION #7):
// slot_no u64 || total_entries u64 || total_chunks u64 ||
// summary (3 x tstr) || namespace_info* || sentinel u32(0) ||
// prev_manifest u64 || root_hash(28) || offset u32.
// All integers big-endian; tstr = u32 length || UTF-8 bytes. The offset
// field equals the record's size prefix (type byte + payload length).
type Manifest struct {
	SlotNo       uint64
	TotalEntries uint64
	TotalChunks  uint64
	RootHash     Hash
	Namespaces   []NamespaceInfo
	PrevManifest uint64
	Summary      ManifestSummary
}

func encodeManifest(m *Manifest) ([]byte, error) {
	// decodeManifest rejects non-UTF-8 strings; refuse to emit them.
	for _, s := range []string{m.Summary.CreatedAt, m.Summary.Tool, m.Summary.Comment} {
		if !utf8.ValidString(s) {
			return nil, fmt.Errorf("%w: summary string not valid UTF-8", ErrInvalidManifest)
		}
	}
	var buf bytes.Buffer
	writeU64(&buf, m.SlotNo)
	writeU64(&buf, m.TotalEntries)
	writeU64(&buf, m.TotalChunks)
	writeTstr(&buf, m.Summary.CreatedAt)
	writeTstr(&buf, m.Summary.Tool)
	writeTstr(&buf, m.Summary.Comment)
	for i, ns := range m.Namespaces {
		if ns.Name == "" {
			return nil, fmt.Errorf("%w: empty name for namespace %d (collides with list sentinel)",
				ErrInvalidManifest, i)
		}
		if !utf8.ValidString(ns.Name) {
			return nil, fmt.Errorf("%w: name for namespace %d not valid UTF-8",
				ErrInvalidManifest, i)
		}
		writeU32(&buf, uint32(len(ns.Name))) //nolint:gosec // name length bounded by maxRecordSize
		writeU64(&buf, ns.EntriesCount)
		writeU64(&buf, ns.ChunksCount)
		buf.WriteString(ns.Name)
		buf.Write(ns.Digest[:])
	}
	writeU32(&buf, 0) // namespace_info list sentinel
	writeU64(&buf, m.PrevManifest)
	buf.Write(m.RootHash[:])
	// offset bookend: type(1) + payload-so-far + offset field itself(4)
	writeU32(&buf, uint32(buf.Len())+5) //nolint:gosec // bounded by maxRecordSize
	return buf.Bytes(), nil
}

func decodeManifest(payload []byte) (*Manifest, error) {
	d := &decoder{data: payload}
	m := &Manifest{}
	m.SlotNo = d.u64()
	m.TotalEntries = d.u64()
	m.TotalChunks = d.u64()
	m.Summary.CreatedAt = d.tstr()
	m.Summary.Tool = d.tstr()
	m.Summary.Comment = d.tstr()
	if d.err != nil {
		return nil, d.err
	}
	for {
		lenNS := d.u32()
		if d.err != nil {
			return nil, d.err
		}
		if lenNS == 0 {
			break // list sentinel
		}
		var ns NamespaceInfo
		ns.EntriesCount = d.u64()
		ns.ChunksCount = d.u64()
		ns.Name = d.str(int(lenNS))
		d.hash(&ns.Digest)
		if d.err != nil {
			return nil, fmt.Errorf("scls: manifest namespace %d: %w", len(m.Namespaces), d.err)
		}
		m.Namespaces = append(m.Namespaces, ns)
	}
	m.PrevManifest = d.u64()
	d.hash(&m.RootHash)
	offset := d.u32()
	if d.err != nil {
		return nil, d.err
	}
	if d.off != len(payload) {
		return nil, fmt.Errorf("%w: %d trailing bytes", ErrInvalidManifest, len(payload)-d.off)
	}
	if int64(offset) != int64(len(payload))+1 {
		return nil, fmt.Errorf("%w: offset field %d != record length %d",
			ErrInvalidManifest, offset, len(payload)+1)
	}
	return m, nil
}

func writeU32(buf *bytes.Buffer, v uint32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	buf.Write(b[:])
}

func writeU64(buf *bytes.Buffer, v uint64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], v)
	buf.Write(b[:])
}

// writeTstr writes a u32 BE length-prefixed UTF-8 string.
func writeTstr(buf *bytes.Buffer, s string) {
	writeU32(buf, uint32(len(s))) //nolint:gosec // string length bounded by maxRecordSize
	buf.WriteString(s)
}

// decoder is a cursor over a record payload; first error sticks.
type decoder struct {
	data []byte
	off  int
	err  error
}

func (d *decoder) take(n int) []byte {
	if d.err != nil {
		return nil
	}
	if n < 0 || d.off+n > len(d.data) {
		d.err = ErrTruncatedRecord
		return nil
	}
	b := d.data[d.off : d.off+n]
	d.off += n
	return b
}

func (d *decoder) u32() uint32 {
	b := d.take(4)
	if b == nil {
		return 0
	}
	return binary.BigEndian.Uint32(b)
}

func (d *decoder) u64() uint64 {
	b := d.take(8)
	if b == nil {
		return 0
	}
	return binary.BigEndian.Uint64(b)
}

func (d *decoder) hash(h *Hash) {
	b := d.take(HashSize)
	if b != nil {
		copy(h[:], b)
	}
}

// str reads n raw bytes as a UTF-8 string.
func (d *decoder) str(n int) string {
	b := d.take(n)
	if b == nil {
		return ""
	}
	if !utf8.Valid(b) {
		d.err = fmt.Errorf("%w: invalid UTF-8 string", ErrInvalidManifest)
		return ""
	}
	return string(b)
}

// tstr reads a u32 BE length-prefixed UTF-8 string.
func (d *decoder) tstr() string {
	n := d.u32()
	return d.str(int(n))
}
