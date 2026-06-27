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
	"sort"
)

// chunkHeaderProbe bounds the prefix of a CHUNK record read at Open to extract
// its namespace and first key without decoding the whole chunk. Namespaces and
// keys are small, so this is generous; a chunk whose namespace+first key exceed
// it triggers an exact re-read.
var chunkHeaderProbe = 64 << 10

// chunkLoc indexes one CHUNK record by namespace, byte offset of its size
// prefix, and the key of its first entry (chunks within a namespace are in
// ascending-key order, so firstKeys are ascending and binary-searchable).
type chunkLoc struct {
	namespace string
	offset    int64
	firstKey  []byte
}

// Snapshot is an indexed, read-only view over a seekable SCLS file. It is
// immutable after Open; Get and Prove are safe for concurrent use.
//
// Open performs structural indexing only and is not a substitute for Verify.
// Verify untrusted files before trusting Get/Prove results.
type Snapshot struct {
	r        io.ReaderAt
	size     int64
	manifest *Manifest
	chunks   []chunkLoc        // file order: namespace-lex, then seq
	ranges   map[string][2]int // namespace -> [firstChunkIdx, lastChunkIdx+1)
}

// Open reads and validates the HDR, locates the trailing MANIFEST via its
// offset bookend, then walks the CHUNK records building an in-memory index.
func Open(r io.ReaderAt, size int64) (*Snapshot, error) {
	if size < 4+manifestMinRecordLen {
		return nil, fmt.Errorf("%w: file too small", ErrMissingManifest)
	}
	s := &Snapshot{r: r, size: size, ranges: map[string][2]int{}}

	// HDR at offset 0.
	hdrType, hdrPayload, hdrConsumed, err := s.readRecordAt(0)
	if err != nil {
		return nil, err
	}
	if hdrType != RecordTypeHdr {
		return nil, fmt.Errorf("%w: first record type 0x%02x", ErrMissingHeader, hdrType)
	}
	if _, err := decodeHeader(hdrPayload); err != nil {
		return nil, err
	}

	// Locate the trailing MANIFEST via the bookend (same math as ReadManifest).
	var tail [4]byte
	if _, err := r.ReadAt(tail[:], size-4); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMissingManifest, err)
	}
	moffset := int64(binary.BigEndian.Uint32(tail[:]))
	mstart := size - 4 - moffset
	if moffset < manifestMinRecordLen || mstart < hdrConsumed {
		return nil, fmt.Errorf("%w: implausible manifest offset %d", ErrMissingManifest, moffset)
	}
	mType, mPayload, _, err := s.readRecordAt(mstart)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMissingManifest, err)
	}
	if mType != RecordTypeManifest || int64(len(mPayload))+1 != moffset {
		return nil, fmt.Errorf("%w: bookend does not point at trailing manifest", ErrMissingManifest)
	}
	m, err := decodeManifest(mPayload)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMissingManifest, err)
	}
	s.manifest = m

	// Walk CHUNK records from end-of-HDR to start-of-MANIFEST.
	off := hdrConsumed
	for off < mstart {
		recType, body, consumed, err := s.readRecordHeaderAt(off, mstart)
		if err != nil {
			return nil, err
		}
		switch recType {
		case RecordTypeChunk:
			ns, firstKey, ok, err := parseChunkHeader(body)
			if err != nil {
				return nil, err
			}
			if !ok {
				// Probe too small (namespace + first key exceed chunkHeaderProbe):
				// re-read the full record. readRecordAt returns the payload
				// WITHOUT the leading type byte, but parseChunkHeader expects a
				// body whose first byte is the record type, so prepend it.
				rt, payload, _, ferr := s.readRecordAt(off)
				if ferr != nil {
					return nil, ferr
				}
				full := make([]byte, 1+len(payload))
				full[0] = rt
				copy(full[1:], payload)
				ns, firstKey, ok, err = parseChunkHeader(full)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, fmt.Errorf("%w: unparseable chunk at offset %d", ErrTruncatedRecord, off)
				}
			}
			s.chunks = append(s.chunks, chunkLoc{namespace: ns, offset: off, firstKey: firstKey})
		case RecordTypeDelta:
			return nil, fmt.Errorf("%w: DELTA records not supported", ErrUnexpectedRecord)
		case RecordTypeManifest:
			return nil, fmt.Errorf("%w: MANIFEST before file end", ErrUnexpectedRecord)
		default:
			// reserved/unknown record types are skipped (forward compatibility)
		}
		off += consumed
	}
	if off != mstart {
		return nil, fmt.Errorf("%w: chunk walk overran manifest start", ErrTruncatedRecord)
	}

	// Build namespace ranges over the contiguous chunk index.
	for i, c := range s.chunks {
		rng, seen := s.ranges[c.namespace]
		if !seen {
			s.ranges[c.namespace] = [2]int{i, i + 1}
			continue
		}
		if rng[1] != i {
			return nil, fmt.Errorf("%w: namespace %q chunks not contiguous", ErrNamespaceOrder, c.namespace)
		}
		rng[1] = i + 1
		s.ranges[c.namespace] = rng
	}
	return s, nil
}

// readRecordAt reads the full record at off and returns its type, payload, and
// the number of bytes the record occupies (4 + 1 + len(payload)).
func (s *Snapshot) readRecordAt(off int64) (recType byte, payload []byte, consumed int64, err error) {
	sr := io.NewSectionReader(s.r, off, s.size-off)
	recType, payload, err = readRecord(sr)
	if err != nil {
		return 0, nil, 0, err
	}
	return recType, payload, int64(len(payload)) + 5, nil
}

// readRecordHeaderAt reads only a bounded prefix (chunkHeaderProbe) of the
// record body at off, for cheap indexing. consumed is the full record size,
// computed from the on-disk size prefix. body is the record body prefix
// (type byte + payload prefix). limit bounds reads to before the manifest.
func (s *Snapshot) readRecordHeaderAt(off, limit int64) (recType byte, body []byte, consumed int64, err error) {
	var szBuf [4]byte
	if _, err := s.r.ReadAt(szBuf[:], off); err != nil {
		return 0, nil, 0, fmt.Errorf("%w: size prefix at %d: %w", ErrTruncatedRecord, off, err)
	}
	size := int64(binary.BigEndian.Uint32(szBuf[:]))
	if size == 0 || size > maxRecordSize {
		return 0, nil, 0, fmt.Errorf("%w: bad record size %d at %d", ErrTruncatedRecord, size, off)
	}
	consumed = 4 + size
	if off+consumed > limit {
		return 0, nil, 0, fmt.Errorf("%w: record at %d overruns manifest", ErrTruncatedRecord, off)
	}
	probe := size
	if probe > int64(chunkHeaderProbe) {
		probe = int64(chunkHeaderProbe)
	}
	body = make([]byte, probe)
	if _, err := s.r.ReadAt(body, off+4); err != nil {
		return 0, nil, 0, fmt.Errorf("%w: body at %d: %w", ErrTruncatedRecord, off, err)
	}
	return body[0], body, consumed, nil
}

// parseChunkHeader extracts the namespace and first entry key from a CHUNK
// record body (type byte + payload, possibly a prefix). ok is false if the
// body is too short to contain the namespace and first key.
// Layout: type(1) | seq(8) | format(1) | len_ns(4) | ns | len_key(4) |
//
//	entry0_size(4) | entry0_key(len_key) | ...
func parseChunkHeader(body []byte) (ns string, firstKey []byte, ok bool, err error) {
	if len(body) < 1 || body[0] != RecordTypeChunk {
		return "", nil, false, nil
	}
	p := body[1:]
	if len(p) < 8+1+4 {
		return "", nil, false, nil
	}
	if p[8] != ChunkFormatRaw {
		return "", nil, false, fmt.Errorf("%w: 0x%02x", ErrUnsupportedChunkFormat, p[8])
	}
	lenNS := int64(binary.BigEndian.Uint32(p[9:13]))
	off := int64(13)
	if off+lenNS+4 > int64(len(p)) {
		return "", nil, false, nil
	}
	nsBytes := p[off : off+lenNS]
	off += lenNS
	keyLen := int64(binary.BigEndian.Uint32(p[off : off+4]))
	off += 4
	if off+4+keyLen > int64(len(p)) {
		return "", nil, false, nil
	}
	off += 4 // skip entry0 size prefix
	firstKey = append([]byte(nil), p[off:off+keyLen]...)
	return string(nsBytes), firstKey, true, nil
}

// Manifest returns the file's parsed MANIFEST.
func (s *Snapshot) Manifest() *Manifest { return s.manifest }

// Namespaces returns the manifest namespace summaries.
func (s *Snapshot) Namespaces() []NamespaceInfo { return s.manifest.Namespaces }

// nsRange returns the [lo, hi) chunk-index range for a namespace.
func (s *Snapshot) nsRange(ns string) (lo, hi int, ok bool) {
	rng, ok := s.ranges[ns]
	return rng[0], rng[1], ok
}

// chunkAt decodes the chunk at index idx of the snapshot's chunk list.
func (s *Snapshot) chunkAt(idx int) (*Chunk, error) {
	_, payload, _, err := s.readRecordAt(s.chunks[idx].offset)
	if err != nil {
		return nil, err
	}
	return decodeChunk(payload)
}

// Get returns the value stored for key in namespace ns, or ErrNotFound.
//
// Preconditions: the file is well-formed (entries strictly ascending and unique
// per namespace). Run Verify on untrusted files first.
func (s *Snapshot) Get(ns string, key []byte) (value []byte, err error) {
	lo, hi, ok := s.nsRange(ns)
	if !ok {
		return nil, fmt.Errorf("%w: namespace %q", ErrNotFound, ns)
	}
	// Find the last chunk whose firstKey <= key. sort.Search returns the first
	// index in [lo,hi) whose firstKey > key; the candidate chunk is the one
	// before it.
	rel := sort.Search(hi-lo, func(i int) bool {
		return bytes.Compare(s.chunks[lo+i].firstKey, key) > 0
	})
	if rel == 0 {
		return nil, fmt.Errorf("%w: key %x in %q", ErrNotFound, key, ns)
	}
	c, err := s.chunkAt(lo + rel - 1)
	if err != nil {
		return nil, err
	}
	// Entries are sorted by key; binary-search within the chunk.
	j := sort.Search(len(c.Entries), func(i int) bool {
		return bytes.Compare(c.Entries[i].Key, key) >= 0
	})
	if j < len(c.Entries) && bytes.Equal(c.Entries[j].Key, key) {
		return c.Entries[j].Value, nil
	}
	return nil, fmt.Errorf("%w: key %x in %q", ErrNotFound, key, ns)
}

// globalLeaves returns the global-tree leaves in lexicographic namespace order
// (the order Verify uses) and a name->index map.
func (s *Snapshot) globalLeaves() ([]Hash, map[string]int) {
	names := make([]string, 0, len(s.manifest.Namespaces))
	digest := make(map[string]Hash, len(s.manifest.Namespaces))
	for _, ns := range s.manifest.Namespaces {
		names = append(names, ns.Name)
		digest[ns.Name] = ns.Digest
	}
	sort.Strings(names)
	leaves := make([]Hash, len(names))
	indexOf := make(map[string]int, len(names))
	for i, name := range names {
		leaves[i] = NamespaceLeafDigest(digest[name])
		indexOf[name] = i
	}
	return leaves, indexOf
}

// Prove returns the value for (ns, key) and an inclusion proof binding it to
// the manifest's global root, or ErrNotFound. See Get for preconditions.
func (s *Snapshot) Prove(ns string, key []byte) (value []byte, proof Proof, err error) {
	lo, hi, ok := s.nsRange(ns)
	if !ok {
		return nil, Proof{}, fmt.Errorf("%w: namespace %q", ErrNotFound, ns)
	}
	// Read every entry of the namespace in order to rebuild its Merkle tree.
	var (
		leaves  []Hash
		leafIdx = -1
	)
	for idx := lo; idx < hi; idx++ {
		c, cerr := s.chunkAt(idx)
		if cerr != nil {
			return nil, Proof{}, cerr
		}
		for _, e := range c.Entries {
			if leafIdx < 0 && bytes.Equal(e.Key, key) {
				leafIdx = len(leaves)
				value = e.Value
			}
			leaves = append(leaves, EntryDigest(ns, e.Key, e.Value))
		}
	}
	if leafIdx < 0 {
		return nil, Proof{}, fmt.Errorf("%w: key %x in %q", ErrNotFound, key, ns)
	}
	nsPath := provePath(leaves, leafIdx)

	// Sanity: the rebuilt namespace root must match the manifest digest, or the
	// file is internally inconsistent (run Verify).
	gLeaves, indexOf := s.globalLeaves()
	gi, ok := indexOf[ns]
	if !ok {
		return nil, Proof{}, fmt.Errorf("%w: namespace %q absent from manifest", ErrInvalidManifest, ns)
	}
	if fold(EntryDigest(ns, key, value), nsPath) != s.namespaceDigest(ns) {
		return nil, Proof{}, fmt.Errorf("%w: namespace %q root inconsistent with manifest", ErrInvalidManifest, ns)
	}
	globalPath := provePath(gLeaves, gi)
	if fold(NamespaceLeafDigest(s.namespaceDigest(ns)), globalPath) != s.manifest.RootHash {
		return nil, Proof{}, fmt.Errorf("%w: global root inconsistent with manifest", ErrInvalidManifest)
	}
	return value, Proof{nsPath: nsPath, globalPath: globalPath}, nil
}

// namespaceDigest returns the manifest's per-namespace Merkle root.
func (s *Snapshot) namespaceDigest(ns string) Hash {
	for _, n := range s.manifest.Namespaces {
		if n.Name == ns {
			return n.Digest
		}
	}
	return Hash{}
}
