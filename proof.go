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
	"encoding/binary"
	"fmt"
)

// proofStep is one level of a Merkle path: combine the running hash with
// sibling. If left, the sibling is the left child (running hash is the right
// child); otherwise the sibling is the right child.
type proofStep struct {
	sibling Hash
	left    bool
}

// Proof is an in-process Merkle inclusion proof binding one entry to an SCLS
// global root. Promotion levels of the unbalanced tree carry the running hash
// unchanged and are therefore absent from the step lists. The step lists are
// unexported; the proof (de)serializes via MarshalBinary and
// UnmarshalProofBinary.
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
// given proof. It needs no file access. Returns nil on success,
// ErrInvalidProof for an excessive path depth, or ErrProofMismatch when the
// proof does not produce root.
func VerifyProof(root Hash, ns string, key, value []byte, proof Proof) error {
	if len(proof.nsPath) > maxProofSteps || len(proof.globalPath) > maxProofSteps {
		return fmt.Errorf("%w: proof path exceeds step limit", ErrInvalidProof)
	}
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

const proofBinaryVersion = 1

// maxProofSteps bounds a decoded proof so UnmarshalProofBinary stays
// allocation-safe on hostile input (CLAUDE.md rule 5). A real proof has at
// most ~log2(N) steps per path; this cap is far above any legitimate depth.
const maxProofSteps = 1024

// MarshalBinary encodes the proof as (RECONCILIATION E7):
// version u8 | nsLen u32 | nsPath | globalLen u32 | globalPath
// where each step is: left u8 (0 or 1) | sibling [HashSize]byte. All
// integers big-endian.
func (p Proof) MarshalBinary() ([]byte, error) {
	size := 1 + 4 + len(p.nsPath)*(1+HashSize) + 4 + len(p.globalPath)*(1+HashSize)
	out := make([]byte, 0, size)
	out = append(out, proofBinaryVersion)
	out = appendProofSteps(out, p.nsPath)
	out = appendProofSteps(out, p.globalPath)
	return out, nil
}

func appendProofSteps(out []byte, steps []proofStep) []byte {
	var lp [4]byte
	binary.BigEndian.PutUint32(lp[:], uint32(len(steps))) //nolint:gosec // bounded by tree depth
	out = append(out, lp[:]...)
	for _, s := range steps {
		if s.left {
			out = append(out, 1)
		} else {
			out = append(out, 0)
		}
		out = append(out, s.sibling[:]...)
	}
	return out
}

// UnmarshalProofBinary decodes a proof produced by Proof.MarshalBinary. It is
// panic-free on arbitrary input and rejects truncated input, trailing bytes,
// unknown versions, out-of-range step counts, and step flags other than 0/1.
// Every malformed-input error wraps ErrInvalidProof (truncation additionally
// wraps ErrTruncatedRecord), so callers can classify any failure with
// errors.Is(err, ErrInvalidProof).
func UnmarshalProofBinary(b []byte) (Proof, error) {
	d := &decoder{data: b}
	ver := d.u8()
	if d.err != nil {
		return Proof{}, fmt.Errorf("%w: %w", ErrInvalidProof, d.err)
	}
	if ver != proofBinaryVersion {
		return Proof{}, fmt.Errorf("%w: version %d", ErrInvalidProof, ver)
	}
	nsPath, err := readProofSteps(d)
	if err != nil {
		return Proof{}, err
	}
	globalPath, err := readProofSteps(d)
	if err != nil {
		return Proof{}, err
	}
	if d.off != len(b) {
		return Proof{}, fmt.Errorf("%w: %d trailing bytes", ErrInvalidProof, len(b)-d.off)
	}
	return Proof{nsPath: nsPath, globalPath: globalPath}, nil
}

func readProofSteps(d *decoder) ([]proofStep, error) {
	n := d.u32()
	if d.err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidProof, d.err)
	}
	if n > maxProofSteps {
		return nil, fmt.Errorf("%w: step count %d exceeds limit", ErrInvalidProof, n)
	}
	steps := make([]proofStep, 0, min(int(n), 1024))
	for i := uint32(0); i < n; i++ {
		flag := d.u8()
		var h Hash
		d.hash(&h)
		if d.err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidProof, d.err)
		}
		if flag > 1 {
			return nil, fmt.Errorf("%w: step flag %d", ErrInvalidProof, flag)
		}
		steps = append(steps, proofStep{sibling: h, left: flag == 1})
	}
	return steps, nil
}
