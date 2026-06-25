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

// proofStep is one level of a Merkle path: combine the running hash with
// sibling. If left, the sibling is the left child (running hash is the right
// child); otherwise the sibling is the right child.
type proofStep struct {
	sibling Hash
	left    bool
}

// Proof is an in-process Merkle inclusion proof binding one entry to an SCLS
// global root. Promotion levels of the unbalanced tree carry the running hash
// unchanged and are therefore absent from the step lists. Fields are
// unexported (serialization is out of scope for now); a MarshalBinary can be
// added later without breaking the API.
type Proof struct {
	nsPath     []proofStep // entry digest   -> per-namespace Merkle root
	globalPath []proofStep // namespace leaf -> global root
}

// Len reports the total number of sibling hashes in the proof.
func (p Proof) Len() int { return len(p.nsPath) + len(p.globalPath) }

// fold replays a Merkle path from a starting hash to the subtree root,
// matching the nodeDigest(left, right) order used by MerkleTree.
func fold(h Hash, steps []proofStep) Hash {
	for _, s := range steps {
		if s.left {
			h = nodeDigest(s.sibling, h)
		} else {
			h = nodeDigest(h, s.sibling)
		}
	}
	return h
}

// VerifyProof reports whether (ns, key, value) is committed under root by the
// given proof. It needs no file access. Returns nil on success, ErrProofMismatch
// on failure.
func VerifyProof(root Hash, ns string, key, value []byte, proof Proof) error {
	leaf := EntryDigest(ns, key, value)
	nsRoot := fold(leaf, proof.nsPath)
	nsLeaf := NamespaceLeafDigest(nsRoot)
	global := fold(nsLeaf, proof.globalPath)
	if global != root {
		return ErrProofMismatch
	}
	return nil
}

// provePath returns the sibling path from leaves[index] up to the Merkle root
// of leaves. It replays the exact promote-then-merge algorithm of
// MerkleTree.Add followed by Root, tagging which subtree contains the target
// leaf and recording the sibling (with side) at each merge the target subtree
// participates in. Promotions (a lone trailing subtree carried up) apply no
// hash and record nothing, so the verifier reproduces the root via fold.
func provePath(leaves []Hash, index int) []proofStep {
	type sub struct {
		depth    int
		hash     Hash
		contains bool
	}
	var (
		path  []proofStep
		stack []sub
	)
	// merge combines left subtree a with right subtree b (a precedes b),
	// matching nodeDigest(a.hash, b.hash) in MerkleTree, and records the
	// sibling step when the target is on one side.
	merge := func(a, b sub) sub {
		if a.contains {
			path = append(path, proofStep{sibling: b.hash, left: false})
		} else if b.contains {
			path = append(path, proofStep{sibling: a.hash, left: true})
		}
		return sub{depth: a.depth + 1, hash: nodeDigest(a.hash, b.hash), contains: a.contains || b.contains}
	}
	for i, leaf := range leaves {
		stack = append(stack, sub{depth: 0, hash: leaf, contains: i == index})
		for len(stack) >= 2 {
			a := stack[len(stack)-2]
			b := stack[len(stack)-1]
			if a.depth != b.depth {
				break
			}
			stack = stack[:len(stack)-2]
			stack = append(stack, merge(a, b))
		}
	}
	// Finalization mirrors MerkleTree.Root: merge the trailing (shallower)
	// subtree into the deeper one. Promotion is folded into the merge (the
	// hash is unchanged by promotion, so only the merge contributes a step).
	for len(stack) >= 2 {
		a := stack[len(stack)-2] // deeper, left
		b := stack[len(stack)-1] // shallower or equal, right
		stack[len(stack)-2] = merge(sub{depth: a.depth, hash: a.hash, contains: a.contains},
			sub{depth: a.depth, hash: b.hash, contains: b.contains})
		stack = stack[:len(stack)-1]
	}
	return path
}
