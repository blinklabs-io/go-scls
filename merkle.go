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

type merkleSubtree struct {
	depth int
	hash  Hash
}

// MerkleTree incrementally computes an SCLS Merkle root over leaf digests
// added in canonical order. The zero value is an empty tree ready for use.
//
// Whenever two subtrees of equal depth exist they are merged immediately
// into a node one depth up. On finalization (Root), trailing subtrees are
// processed shallowest-to-deepest: a shallower trailing subtree is promoted
// (depth incremented, hash unchanged) until depths match, then merged.
// VERIFY(#11): answered — matches the CIP finalization rule and the
// reference implementations (spec/RECONCILIATION.md row 11).
type MerkleTree struct {
	subtrees []merkleSubtree // stack, deepest first
}

// Add appends a leaf digest in canonical order.
func (t *MerkleTree) Add(leaf Hash) {
	t.subtrees = append(t.subtrees, merkleSubtree{depth: 0, hash: leaf})
	for len(t.subtrees) >= 2 {
		a := t.subtrees[len(t.subtrees)-2]
		b := t.subtrees[len(t.subtrees)-1]
		if a.depth != b.depth {
			break
		}
		t.subtrees = t.subtrees[:len(t.subtrees)-2]
		t.subtrees = append(t.subtrees, merkleSubtree{
			depth: a.depth + 1,
			hash:  nodeDigest(a.hash, b.hash),
		})
	}
}

// Root finalizes and returns the Merkle root without mutating the tree.
// An empty tree returns EmptyRoot().
func (t *MerkleTree) Root() Hash {
	if len(t.subtrees) == 0 {
		return EmptyRoot()
	}
	stack := make([]merkleSubtree, len(t.subtrees))
	copy(stack, t.subtrees)
	for len(stack) >= 2 {
		a := stack[len(stack)-2] // deeper
		b := stack[len(stack)-1] // shallower or equal: promote until depths match
		stack[len(stack)-2] = merkleSubtree{depth: a.depth + 1, hash: nodeDigest(a.hash, b.hash)}
		stack = stack[:len(stack)-1]
	}
	return stack[0].hash
}
