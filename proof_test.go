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
	"errors"
	"testing"
)

func TestVerifyProofSingleEntry(t *testing.T) {
	ns, key, val := "utxo/v0", []byte("k0"), []byte("v0")
	// One entry in one namespace: nsRoot = leaf, global root = NamespaceLeafDigest(nsRoot).
	root := NamespaceLeafDigest(EntryDigest(ns, key, val))

	if err := VerifyProof(root, ns, key, val, Proof{}); err != nil {
		t.Fatalf("valid proof rejected: %v", err)
	}
	if err := VerifyProof(root, ns, key, []byte("wrong"), Proof{}); !errors.Is(err, ErrProofMismatch) {
		t.Fatalf("tampered value: got %v, want ErrProofMismatch", err)
	}
	if err := VerifyProof(root, "other/v0", key, val, Proof{}); !errors.Is(err, ErrProofMismatch) {
		t.Fatalf("wrong namespace: got %v, want ErrProofMismatch", err)
	}
}

func TestFoldEmptyIsIdentity(t *testing.T) {
	h := EntryDigest("ns", []byte("k"), []byte("v"))
	if fold(h, nil) != h {
		t.Fatal("fold of empty path must be identity")
	}
}

func TestProvePathMatchesRoot(t *testing.T) {
	for n := 1; n <= 64; n++ {
		leaves := make([]Hash, n)
		var tree MerkleTree
		for i := range leaves {
			// distinct, deterministic leaf digests
			leaves[i] = EntryDigest("ns", []byte{byte(i), byte(i >> 8)}, []byte{0xAB})
			tree.Add(leaves[i])
		}
		root := tree.Root()
		for i := 0; i < n; i++ {
			got := fold(leaves[i], provePath(leaves, i))
			if got != root {
				t.Fatalf("n=%d i=%d: fold=%x want root=%x", n, i, got, root)
			}
		}
	}
}
