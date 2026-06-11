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

import "golang.org/x/crypto/blake2b"

// HashSize is the byte length of all SCLS digests (Blake2b-224).
const HashSize = 28

// Hash is a Blake2b-224 digest.
type Hash [HashSize]byte

const (
	domainSepNode byte = 0x00 // internal Merkle node
	domainSepLeaf byte = 0x01 // Merkle leaf
)

func hashParts(parts ...[]byte) Hash {
	h, err := blake2b.New(HashSize, nil)
	if err != nil {
		// blake2b.New only fails for invalid key length; nil key cannot fail
		panic(err)
	}
	for _, p := range parts {
		h.Write(p)
	}
	var out Hash
	h.Sum(out[:0]) // appends the digest into out's backing array; no allocation
	return out
}

// EntryDigest computes the Merkle leaf digest of a single entry:
// Blake2b-224(0x01 || namespace || key || value) over raw bytes.
// VERIFY(#5): answered — raw key/value bytes, no CBOR or length framing
// (spec/RECONCILIATION.md row 5).
func EntryDigest(namespace string, key, value []byte) Hash {
	return hashParts([]byte{domainSepLeaf}, []byte(namespace), key, value)
}

// NamespaceLeafDigest computes a global-tree leaf from a per-namespace root:
// Blake2b-224(0x01 || root). The namespace name is not included.
// VERIFY(#12): answered (spec/RECONCILIATION.md row 12).
func NamespaceLeafDigest(root Hash) Hash {
	return hashParts([]byte{domainSepLeaf}, root[:])
}

// nodeDigest computes an internal Merkle node: Blake2b-224(0x00 || left || right).
func nodeDigest(left, right Hash) Hash {
	return hashParts([]byte{domainSepNode}, left[:], right[:])
}

// EmptyRoot is the Merkle root of an empty tree (Blake2b-224 of empty input).
// VERIFY(#8): answered (spec/RECONCILIATION.md row 8).
func EmptyRoot() Hash {
	return hashParts()
}
