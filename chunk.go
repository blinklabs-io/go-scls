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
	"io"
	"unicode/utf8"

	seekable "github.com/SaveTheRbtz/zstd-seekable-format-go/pkg"
	"github.com/klauspost/compress/zstd"
	"golang.org/x/crypto/blake2b"
)

// Chunk compression formats (CIP-0165).
const (
	ChunkFormatRaw         byte = 0x00 // raw CBOR entries
	ChunkFormatZstd        byte = 0x01 // seekable zstd over all entries
	ChunkFormatZstdEntries byte = 0x02 // per-value zstd
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

// zstdSeekFrameSize keeps random-access frames small while the uncompressed
// chunk-size limit bounds total decoder output.
const zstdSeekFrameSize = 1 << 20

// chunkHeaderFixedSize is seq(8) + format(1) + len_ns(4) + len_key(4),
// excluding the variable-length namespace bytes.
const chunkHeaderFixedSize = 8 + 1 + 4 + 4

// encodeChunkLevel encodes a CHUNK payload (RECONCILIATION #3/#6/#10):
// seq(u64 BE) || format(u8) || len_ns(u32 BE) || ns || len_key(u32 BE) ||
// entries || entries_count(u32 BE) || chunk_hash(28).
func encodeChunkLevel(c *Chunk, level int) ([]byte, error) {
	if c.Format > ChunkFormatZstdEntries {
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
	var entries bytes.Buffer
	var valueEncoder *zstd.Encoder
	if c.Format == ChunkFormatZstdEntries {
		var err error
		valueEncoder, err = newZstdEncoder(level)
		if err != nil {
			return nil, err
		}
		defer valueEncoder.Close()
	}
	decodedEntriesLen := 0
	for i, e := range c.Entries {
		if uint32(len(e.Key)) != c.KeyLen { //nolint:gosec // key length validated by Writer
			return nil, fmt.Errorf("%w: entry %d key length %d != %d",
				ErrKeyLength, i, len(e.Key), c.KeyLen)
		}
		if len(e.Key) > maxRecordSize-4 || len(e.Value) > maxRecordSize-4-len(e.Key) ||
			decodedEntriesLen > maxRecordSize-4-len(e.Key)-len(e.Value) {
			return nil, fmt.Errorf("%w: decoded chunk entries exceed %d bytes",
				ErrRecordTooLarge, maxRecordSize)
		}
		decodedEntriesLen += 4 + len(e.Key) + len(e.Value)
		encoded := encodeEntry(e)
		if c.Format == ChunkFormatZstdEntries {
			compressed := valueEncoder.EncodeAll(e.Value, nil)
			encoded = encodeEntry(Entry{Key: e.Key, Value: compressed})
		}
		entries.Write(encoded)
	}
	entryBytes := entries.Bytes()
	if c.Format == ChunkFormatZstd {
		var err error
		entryBytes, err = compressSeekable(entryBytes, level)
		if err != nil {
			return nil, err
		}
	}
	buf.Write(entryBytes)
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
	if c.Format > ChunkFormatZstdEntries {
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
	switch c.Format {
	case ChunkFormatRaw:
		c.Entries, err = decodeEntries(entriesData, count, c.KeyLen)
	case ChunkFormatZstd:
		var decoded []byte
		decoded, err = decompressSeekable(entriesData)
		if err == nil {
			c.Entries, err = decodeEntries(decoded, count, c.KeyLen)
		}
	case ChunkFormatZstdEntries:
		c.Entries, err = decodeCompressedEntries(entriesData, count, c.KeyLen)
	}
	if err != nil && c.Format != ChunkFormatRaw {
		return nil, fmt.Errorf("%w: %w", ErrCompressedChunk, err)
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func newZstdEncoder(level int) (*zstd.Encoder, error) {
	opts := []zstd.EOption{
		zstd.WithEncoderCRC(true),
		zstd.WithEncoderConcurrency(1),
	}
	if level != 0 {
		opts = append(opts, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)))
	}
	return zstd.NewWriter(nil, opts...)
}

func newZstdDecoder() (*zstd.Decoder, error) {
	return zstd.NewReader(
		nil,
		zstd.WithDecoderConcurrency(1),
		zstd.WithDecoderMaxMemory(maxRecordSize),
		zstd.WithDecoderMaxWindow(maxRecordSize),
	)
}

func compressSeekable(data []byte, level int) ([]byte, error) {
	enc, err := newZstdEncoder(level)
	if err != nil {
		return nil, err
	}
	defer enc.Close()
	var out bytes.Buffer
	w, err := seekable.NewWriter(&out, enc)
	if err != nil {
		return nil, err
	}
	for len(data) > 0 && err == nil {
		n := min(len(data), zstdSeekFrameSize)
		_, err = w.Write(data[:n])
		data = data[n:]
	}
	if err == nil {
		err = w.Close()
	}
	return out.Bytes(), err
}

func decompressSeekable(data []byte) ([]byte, error) {
	dec, err := newZstdDecoder()
	if err != nil {
		return nil, err
	}
	defer dec.Close()
	r, err := seekable.NewReader(bytes.NewReader(data), dec)
	if err != nil {
		return nil, err
	}
	if err := validateSeekableFrames(r, uint64(len(data))); err != nil {
		return nil, err
	}
	out, err := io.ReadAll(io.LimitReader(r, maxRecordSize+1))
	if err == nil && len(out) > maxRecordSize {
		err = ErrRecordTooLarge
	}
	return out, err
}

func validateSeekableFrames(r *seekable.Reader, compressedSize uint64) error {
	table, err := r.SeekTable()
	if err != nil {
		return err
	}
	if table.Size() > maxRecordSize {
		return ErrRecordTooLarge
	}
	for i := int64(0); i < table.NumFrames(); i++ {
		entry, ok := table.EntryByID(i)
		if !ok {
			return fmt.Errorf("%w: missing seek-table frame %d", ErrCompressedChunk, i)
		}
		if err := validateSeekableFrame(entry, compressedSize); err != nil {
			return err
		}
	}
	return nil
}

func validateSeekableFrame(entry seekable.FrameOffsetEntry, compressedSize uint64) error {
	if entry.CompressedOffset > compressedSize ||
		uint64(entry.CompressedSize) > compressedSize-entry.CompressedOffset {
		return fmt.Errorf("%w: seek-table frame %d exceeds compressed data",
			ErrCompressedChunk, entry.ID)
	}
	return nil
}

func decodeCompressedEntries(data []byte, count, keyLen uint32) ([]Entry, error) {
	return decodeCompressedEntriesLimit(data, count, keyLen, maxRecordSize)
}

func decodeCompressedEntriesLimit(data []byte, count, keyLen uint32, maxBytes int) ([]Entry, error) {
	stored, err := decodeEntries(data, count, keyLen)
	if err != nil {
		return nil, err
	}
	dec, err := newZstdDecoder()
	if err != nil {
		return nil, err
	}
	defer dec.Close()
	total := 0
	for i := range stored {
		overhead := 4 + len(stored[i].Key)
		remaining := maxBytes - total - overhead
		if remaining < 0 {
			return nil, ErrRecordTooLarge
		}
		// A frame's window can exceed its decoded value and remaining budget.
		if err = dec.ResetWithOptions(
			nil,
			zstd.WithDecoderMaxMemory(maxRecordSize),
		); err != nil {
			return nil, err
		}
		stored[i].Value, err = dec.DecodeAll(stored[i].Value, nil)
		if err != nil {
			return nil, err
		}
		total += overhead + len(stored[i].Value)
		if total > maxBytes {
			return nil, ErrRecordTooLarge
		}
	}
	return stored, nil
}
