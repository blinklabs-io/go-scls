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

	"golang.org/x/crypto/blake2b"
)

// Chunk compression formats (CIP-0165).
const (
	ChunkFormatRaw         byte = 0x00 // raw CBOR entries
	ChunkFormatZstd        byte = 0x01 // seekable zstd over all entries (unsupported in v0.1)
	ChunkFormatZstdEntries byte = 0x02 // per-value zstd (unsupported in v0.1)
)

// Chunk is a CHUNK record: ordered entries for a single namespace.
// KeyLen is the fixed key length for all entries in the chunk's namespace
// (RECONCILIATION #10).
type Chunk struct {
	Seq       uint64
	Format    byte
	Namespace string
	KeyLen    uint32
	Entries   []Entry

	declaredHash Hash // footer hash as stored in the file (set by decodeChunk)
}

// DeclaredHash returns the chunk hash stored in the file footer.
func (c *Chunk) DeclaredHash() Hash { return c.declaredHash }

// ComputeHash recomputes the chunk hash over the (uncompressed) entries:
// Blake2b-224(concat(EntryDigest(namespace, key, value))).
// VERIFY(#6): answered — confirmed by CIP §CHUNK policy and cardano-scrawls.
func (c *Chunk) ComputeHash() Hash {
	h, err := blake2b.New(HashSize, nil)
	if err != nil {
		panic(err) // unreachable with nil key
	}
	for _, e := range c.Entries {
		d := EntryDigest(c.Namespace, e.Key, e.Value)
		h.Write(d[:])
	}
	var out Hash
	copy(out[:], h.Sum(nil))
	return out
}

const chunkFooterSize = 4 + HashSize // entries_count u32 || chunk_hash

// chunkHeaderFixedSize is seq(8) + format(1) + len_ns(4) + len_key(4),
// excluding the variable-length namespace bytes.
const chunkHeaderFixedSize = 8 + 1 + 4 + 4

// encodeChunk encodes a CHUNK payload (RECONCILIATION #3/#6/#10):
// seq(u64 BE) || format(u8) || len_ns(u32 BE) || ns || len_key(u32 BE) ||
// entries || entries_count(u32 BE) || chunk_hash(28).
func encodeChunk(c *Chunk) ([]byte, error) {
	if c.Format != ChunkFormatRaw {
		return nil, fmt.Errorf("%w: 0x%02x", ErrUnsupportedChunkFormat, c.Format)
	}
	var buf bytes.Buffer
	var u64buf [8]byte
	binary.BigEndian.PutUint64(u64buf[:], c.Seq)
	buf.Write(u64buf[:])
	buf.WriteByte(c.Format)
	var u32buf [4]byte
	binary.BigEndian.PutUint32(u32buf[:], uint32(len(c.Namespace))) //nolint:gosec // namespace length bounded by maxRecordSize
	buf.Write(u32buf[:])
	buf.WriteString(c.Namespace)
	binary.BigEndian.PutUint32(u32buf[:], c.KeyLen)
	buf.Write(u32buf[:])
	for i, e := range c.Entries {
		if uint32(len(e.Key)) != c.KeyLen { //nolint:gosec // key length validated by Writer
			return nil, fmt.Errorf("%w: entry %d key length %d != %d",
				ErrKeyLength, i, len(e.Key), c.KeyLen)
		}
		buf.Write(encodeEntry(e))
	}
	binary.BigEndian.PutUint32(u32buf[:], uint32(len(c.Entries))) //nolint:gosec // chunk sizes bounded by Writer
	buf.Write(u32buf[:])
	h := c.ComputeHash()
	buf.Write(h[:])
	return buf.Bytes(), nil
}

// decodeChunk parses a CHUNK payload. It does not verify the footer hash;
// callers use ComputeHash()/DeclaredHash() (see Verify).
func decodeChunk(payload []byte) (*Chunk, error) {
	if len(payload) < chunkHeaderFixedSize+chunkFooterSize {
		return nil, ErrTruncatedChunk
	}
	c := &Chunk{
		Seq:    binary.BigEndian.Uint64(payload[0:8]),
		Format: payload[8],
	}
	if c.Format != ChunkFormatRaw {
		return nil, fmt.Errorf("%w: 0x%02x", ErrUnsupportedChunkFormat, c.Format)
	}
	lenNS32 := binary.BigEndian.Uint32(payload[9:13])
	// Bounds-check in int64 space before converting: int(lenNS32) can
	// wrap negative on 32-bit platforms and panic the slice expressions
	// below (len(payload) >= header+footer was checked above).
	if int64(lenNS32) > int64(len(payload)-chunkHeaderFixedSize-chunkFooterSize) {
		return nil, ErrTruncatedChunk
	}
	lenNS := int(lenNS32)
	nsBytes := payload[13 : 13+lenNS]
	if !utf8.Valid(nsBytes) {
		return nil, fmt.Errorf("%w: namespace not valid UTF-8", ErrTruncatedChunk)
	}
	c.Namespace = string(nsBytes)
	c.KeyLen = binary.BigEndian.Uint32(payload[13+lenNS : 17+lenNS])
	rest := payload[17+lenNS:]
	entriesData := rest[:len(rest)-chunkFooterSize]
	footer := rest[len(rest)-chunkFooterSize:]
	count := binary.BigEndian.Uint32(footer[0:4])
	copy(c.declaredHash[:], footer[4:])
	var err error
	c.Entries, err = decodeEntries(entriesData, count, c.KeyLen)
	if err != nil {
		return nil, err
	}
	return c, nil
}
