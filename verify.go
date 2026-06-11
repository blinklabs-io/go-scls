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
	"fmt"
	"io"
)

// VerifyLevel selects how much of an SCLS file Verify checks.
type VerifyLevel int

const (
	// VerifyStructure checks record sequence, namespace and key ordering,
	// chunk sequence numbers, and manifest counts.
	VerifyStructure VerifyLevel = iota
	// VerifyChunks additionally recomputes every chunk footer hash.
	VerifyChunks
	// VerifyFull additionally recomputes per-namespace Merkle roots and the
	// global root and compares them to the manifest.
	VerifyFull
)

// VerifyResult reports what was recomputed during verification.
type VerifyResult struct {
	TotalEntries uint64
	TotalChunks  uint64
	Namespaces   []NamespaceInfo // computed (digests zero unless VerifyFull)
	RootHash     Hash            // computed global root (zero unless VerifyFull)
}

// Verify streams an SCLS file and validates it at the requested level.
func Verify(r io.Reader, level VerifyLevel) (*VerifyResult, error) {
	sr, err := NewReader(r)
	if err != nil {
		return nil, err
	}
	res := &VerifyResult{}
	var (
		curNamespace string
		nsStarted    bool
		lastKey      []byte
		lastSeq      uint64
		seqSeen      bool
		nsTree       MerkleTree
		nsEntries    uint64
		nsChunks     uint64
	)
	finishNamespace := func() {
		info := NamespaceInfo{
			Name:         curNamespace,
			EntriesCount: nsEntries,
			ChunksCount:  nsChunks,
		}
		if level >= VerifyFull {
			info.Digest = nsTree.Root()
		}
		res.Namespaces = append(res.Namespaces, info)
	}
	for {
		c, err := sr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		// namespace ordering across chunks
		switch {
		case !nsStarted:
			nsStarted = true
			curNamespace = c.Namespace
		case c.Namespace == curNamespace:
			// continuation: keys must keep ascending across the chunk boundary
		case c.Namespace > curNamespace:
			finishNamespace()
			curNamespace = c.Namespace
			lastKey = nil
			nsTree = MerkleTree{}
			nsEntries, nsChunks = 0, 0
			seqSeen = false // RECONCILIATION #9: seqno resets per namespace
		default:
			return nil, fmt.Errorf("%w: %q after %q",
				ErrNamespaceOrder, c.Namespace, curNamespace)
		}
		// chunk sequence strictly increasing within the namespace; the start
		// value is not pinned by the spec (RECONCILIATION #9)
		if seqSeen && c.Seq <= lastSeq {
			return nil, fmt.Errorf("%w: chunk seq %d after %d in %q",
				ErrChunkSequence, c.Seq, lastSeq, c.Namespace)
		}
		lastSeq, seqSeen = c.Seq, true
		// entry ordering within namespace
		for _, e := range c.Entries {
			if lastKey != nil && bytes.Compare(e.Key, lastKey) <= 0 {
				return nil, fmt.Errorf("%w: %x after %x in %q",
					ErrEntryOrder, e.Key, lastKey, c.Namespace)
			}
			lastKey = e.Key
			if level >= VerifyFull {
				nsTree.Add(EntryDigest(c.Namespace, e.Key, e.Value))
			}
		}
		if level >= VerifyChunks {
			if c.ComputeHash() != c.DeclaredHash() {
				return nil, fmt.Errorf("%w: chunk %d footer", ErrHashMismatch, c.Seq)
			}
		}
		nsEntries += uint64(len(c.Entries))
		nsChunks++
		res.TotalEntries += uint64(len(c.Entries))
		res.TotalChunks++
	}
	if nsStarted {
		finishNamespace()
	}
	m := sr.Manifest()
	if m == nil {
		return nil, ErrMissingManifest
	}
	// counts vs manifest
	if m.TotalEntries != res.TotalEntries || m.TotalChunks != res.TotalChunks {
		return nil, fmt.Errorf("%w: manifest totals (%d,%d) vs computed (%d,%d)",
			ErrCountMismatch, m.TotalEntries, m.TotalChunks,
			res.TotalEntries, res.TotalChunks)
	}
	// manifest namespace order is not significant (RECONCILIATION E3):
	// compare as a set keyed by name, rejecting duplicates
	manifestNS := make(map[string]NamespaceInfo, len(m.Namespaces))
	for _, mns := range m.Namespaces {
		if _, dup := manifestNS[mns.Name]; dup {
			return nil, fmt.Errorf("%w: duplicate namespace %q", ErrInvalidManifest, mns.Name)
		}
		manifestNS[mns.Name] = mns
	}
	if len(manifestNS) != len(res.Namespaces) {
		return nil, fmt.Errorf("%w: manifest has %d namespaces, computed %d",
			ErrCountMismatch, len(manifestNS), len(res.Namespaces))
	}
	for _, ns := range res.Namespaces {
		mns, ok := manifestNS[ns.Name]
		if !ok {
			return nil, fmt.Errorf("%w: namespace %q missing from manifest",
				ErrCountMismatch, ns.Name)
		}
		if mns.EntriesCount != ns.EntriesCount || mns.ChunksCount != ns.ChunksCount {
			return nil, fmt.Errorf("%w: namespace %q summary", ErrCountMismatch, ns.Name)
		}
		if level >= VerifyFull && mns.Digest != ns.Digest {
			return nil, fmt.Errorf("%w: namespace %q merkle root", ErrHashMismatch, ns.Name)
		}
	}
	if level >= VerifyFull {
		// global tree over per-namespace roots in lexicographic namespace
		// order — res.Namespaces is already in that order (enforced above)
		var global MerkleTree
		for _, ns := range res.Namespaces {
			global.Add(NamespaceLeafDigest(ns.Digest))
		}
		res.RootHash = global.Root()
		if res.RootHash != m.RootHash {
			return nil, fmt.Errorf("%w: global merkle root", ErrHashMismatch)
		}
	}
	return res, nil
}
