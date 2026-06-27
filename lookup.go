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
	"sort"
)

// Lookup scans an SCLS stream and returns the value for (ns, key), or
// ErrNotFound. It needs no seekable source. It stops early once the stream
// advances past ns (namespaces are emitted in ascending order).
func Lookup(r io.Reader, ns string, key []byte) (value []byte, err error) {
	sr, err := NewReader(r)
	if err != nil {
		return nil, err
	}
	for {
		c, err := sr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%w: key %x in %q", ErrNotFound, key, ns)
		}
		if err != nil {
			return nil, err
		}
		if c.Namespace < ns {
			continue
		}
		if c.Namespace > ns {
			return nil, fmt.Errorf("%w: key %x in %q", ErrNotFound, key, ns)
		}
		j := sort.Search(len(c.Entries), func(i int) bool {
			return bytes.Compare(c.Entries[i].Key, key) >= 0
		})
		if j < len(c.Entries) && bytes.Equal(c.Entries[j].Key, key) {
			return c.Entries[j].Value, nil
		}
		// key not in this chunk; if the chunk's last key already exceeds key,
		// it cannot appear later in the namespace.
		if len(c.Entries) > 0 && bytes.Compare(c.Entries[len(c.Entries)-1].Key, key) > 0 {
			return nil, fmt.Errorf("%w: key %x in %q", ErrNotFound, key, ns)
		}
	}
}

// LookupProof scans an SCLS stream and returns the value and an inclusion proof
// for (ns, key) bound to the file's global root, or ErrNotFound. It builds the
// per-namespace tree for ns and the global tree from per-namespace roots
// computed during the single pass.
func LookupProof(r io.Reader, ns string, key []byte) (value []byte, proof Proof, err error) {
	sr, err := NewReader(r)
	if err != nil {
		return nil, Proof{}, err
	}
	var (
		cur            string
		started        bool
		curTree        MerkleTree
		namespaceRoots []namespaceRoot // per-namespace roots in file (lexicographic) order
		tgtLeaves      []Hash
		tgtIdx         = -1
	)
	finish := func() {
		namespaceRoots = append(namespaceRoots, namespaceRoot{
			name: cur,
			root: curTree.Root(),
		})
	}
	for {
		c, e := sr.Next()
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			return nil, Proof{}, e
		}
		if !started {
			started, cur = true, c.Namespace
		} else if c.Namespace != cur {
			finish()
			cur, curTree = c.Namespace, MerkleTree{}
		}
		for _, en := range c.Entries {
			d := EntryDigest(c.Namespace, en.Key, en.Value)
			curTree.Add(d)
			if c.Namespace == ns {
				if tgtIdx < 0 && bytes.Equal(en.Key, key) {
					tgtIdx = len(tgtLeaves)
					value = en.Value
				}
				tgtLeaves = append(tgtLeaves, d)
			}
		}
	}
	if started {
		finish()
	}
	if tgtIdx < 0 {
		return nil, Proof{}, fmt.Errorf("%w: key %x in %q", ErrNotFound, key, ns)
	}
	nsPath := provePath(tgtLeaves, tgtIdx)

	// Global tree leaves in lexicographic namespace order.
	gLeaves := make([]Hash, len(namespaceRoots))
	gi := -1
	for i, nsRoot := range namespaceRoots {
		gLeaves[i] = NamespaceLeafDigest(nsRoot.root)
		if nsRoot.name == ns {
			gi = i
		}
	}
	if gi < 0 {
		return nil, Proof{}, fmt.Errorf("%w: namespace %q root absent", ErrInvalidManifest, ns)
	}
	globalPath := provePath(gLeaves, gi)
	return value, Proof{nsPath: nsPath, globalPath: globalPath}, nil
}

type namespaceRoot struct {
	name string
	root Hash
}
