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
	"fmt"
	"io"
	"unicode/utf8"
)

const (
	defaultMaxChunkEntries = 4096

	// defaultMaxChunkBytes follows the CIP-0165 chunk-size policy
	// (~8-16 MiB) and the Rust reference default of 16 MiB.
	defaultMaxChunkBytes = 16 << 20
)

// WriterOption configures a Writer.
type WriterOption func(*Writer)

// WithMaxChunkEntries sets how many entries are grouped per CHUNK record.
func WithMaxChunkEntries(n int) WriterOption {
	return func(w *Writer) {
		if n > 0 {
			w.maxChunkEntries = n
		}
	}
}

// WithMaxChunkBytes caps the serialized entry bytes per CHUNK record. A
// chunk is flushed once it reaches the cap, so a chunk may exceed it by at
// most one entry; a single oversized entry is never split.
func WithMaxChunkBytes(n int) WriterOption {
	return func(w *Writer) {
		if n > 0 {
			w.maxChunkBytes = n
		}
	}
}

// WithSummary sets the MANIFEST summary strings. CreatedAt should be an
// RFC 3339 timestamp; an empty Comment is encoded as absent.
func WithSummary(s ManifestSummary) WriterOption {
	return func(w *Writer) { w.summary = s }
}

// Writer streams an SCLS file: HDR, (CHUNK)*, MANIFEST. Entries must be
// added in strictly ascending namespace order (bytewise), and within a
// namespace in strictly ascending key order with a consistent key length.
type Writer struct {
	w               io.Writer
	maxChunkEntries int
	maxChunkBytes   int
	summary         ManifestSummary

	chunkSeq     uint64 // per-namespace, reset on namespace change (RECONCILIATION #9)
	totalEntries uint64
	totalChunks  uint64

	curNamespace string
	nsStarted    bool
	lastKey      []byte
	keyLen       int
	pending      []Entry
	pendingBytes int // serialized size of pending entries

	nsTree     MerkleTree
	nsEntries  uint64
	nsChunks   uint64
	namespaces []NamespaceInfo

	closed bool
}

// NewWriter writes the HDR record and returns a ready Writer.
func NewWriter(w io.Writer, opts ...WriterOption) (*Writer, error) {
	sw := &Writer{
		w:               w,
		maxChunkEntries: defaultMaxChunkEntries,
		maxChunkBytes:   defaultMaxChunkBytes,
		summary:         ManifestSummary{Tool: "go-scls"},
	}
	for _, opt := range opts {
		opt(sw)
	}
	hdr := &Header{Version: FormatVersion}
	if err := writeRecord(w, RecordTypeHdr, hdr.encode()); err != nil {
		return nil, fmt.Errorf("scls: write header: %w", err)
	}
	return sw, nil
}

// AddEntry appends one entry. Value must be the canonical CBOR encoding of
// the namespace-specific payload; it is written verbatim. Key and value are
// copied, so callers may reuse their buffers.
func (sw *Writer) AddEntry(namespace string, key, value []byte) error {
	if sw.closed {
		return ErrClosed
	}
	if namespace == "" {
		return fmt.Errorf("%w: empty namespace", ErrNamespaceOrder)
	}
	// decodeChunk rejects non-UTF-8 namespaces; refuse to emit them.
	if !utf8.ValidString(namespace) {
		return fmt.Errorf("%w: namespace not valid UTF-8", ErrNamespaceOrder)
	}
	switch {
	case !sw.nsStarted:
		sw.startNamespace(namespace, len(key))
	case namespace == sw.curNamespace:
		if bytes.Compare(key, sw.lastKey) <= 0 {
			return fmt.Errorf("%w: %x after %x in %q",
				ErrEntryOrder, key, sw.lastKey, namespace)
		}
		if len(key) != sw.keyLen {
			return fmt.Errorf("%w: %d != %d in %q",
				ErrKeyLength, len(key), sw.keyLen, namespace)
		}
	default:
		if namespace < sw.curNamespace {
			return fmt.Errorf("%w: %q after %q",
				ErrNamespaceOrder, namespace, sw.curNamespace)
		}
		if err := sw.finishNamespace(); err != nil {
			return err
		}
		sw.startNamespace(namespace, len(key))
	}
	e := Entry{
		Key:   bytes.Clone(key),
		Value: bytes.Clone(value),
	}
	sw.pending = append(sw.pending, e)
	sw.pendingBytes += 4 + len(e.Key) + len(e.Value)
	sw.lastKey = e.Key
	sw.nsTree.Add(EntryDigest(namespace, e.Key, e.Value))
	sw.nsEntries++
	sw.totalEntries++
	if len(sw.pending) >= sw.maxChunkEntries || sw.pendingBytes >= sw.maxChunkBytes {
		return sw.flushChunk()
	}
	return nil
}

func (sw *Writer) startNamespace(namespace string, keyLen int) {
	sw.curNamespace = namespace
	sw.nsStarted = true
	sw.keyLen = keyLen
	sw.lastKey = nil
	sw.nsTree = MerkleTree{}
	sw.nsEntries = 0
	sw.nsChunks = 0
	// VERIFY(#9): answered — seqno is per-namespace; the spec does not pin
	// the start value (Rust writes from 0, Haskell from 1). We write from 0.
	sw.chunkSeq = 0
}

func (sw *Writer) flushChunk() error {
	if len(sw.pending) == 0 {
		return nil
	}
	c := &Chunk{
		Seq:       sw.chunkSeq,
		Format:    ChunkFormatRaw,
		Namespace: sw.curNamespace,
		KeyLen:    uint32(sw.keyLen), //nolint:gosec // key length bounded by maxRecordSize
		Entries:   sw.pending,
	}
	payload, err := encodeChunk(c)
	if err != nil {
		return err
	}
	if err := writeRecord(sw.w, RecordTypeChunk, payload); err != nil {
		return fmt.Errorf("scls: write chunk %d: %w", sw.chunkSeq, err)
	}
	sw.chunkSeq++
	sw.totalChunks++
	sw.nsChunks++
	sw.pending = nil
	sw.pendingBytes = 0
	return nil
}

func (sw *Writer) finishNamespace() error {
	if err := sw.flushChunk(); err != nil {
		return err
	}
	sw.namespaces = append(sw.namespaces, NamespaceInfo{
		Name:         sw.curNamespace,
		EntriesCount: sw.nsEntries,
		ChunksCount:  sw.nsChunks,
		Digest:       sw.nsTree.Root(),
	})
	return nil
}

// Close flushes pending entries, writes the MANIFEST for the given slot,
// and returns the global Merkle root. The Writer is unusable afterwards.
// Close does not close the underlying io.Writer.
func (sw *Writer) Close(slotNo uint64) (Hash, error) {
	if sw.closed {
		return Hash{}, ErrClosed
	}
	sw.closed = true
	if sw.nsStarted {
		if err := sw.finishNamespace(); err != nil {
			return Hash{}, err
		}
	}
	var global MerkleTree
	for _, ns := range sw.namespaces {
		global.Add(NamespaceLeafDigest(ns.Digest))
	}
	root := global.Root()
	m := &Manifest{
		SlotNo:       slotNo,
		TotalEntries: sw.totalEntries,
		TotalChunks:  sw.totalChunks,
		RootHash:     root,
		Namespaces:   sw.namespaces,
		PrevManifest: 0,
		Summary:      sw.summary,
	}
	payload, err := encodeManifest(m)
	if err != nil {
		return Hash{}, err
	}
	if err := writeRecord(sw.w, RecordTypeManifest, payload); err != nil {
		return Hash{}, fmt.Errorf("scls: write manifest: %w", err)
	}
	return root, nil
}
