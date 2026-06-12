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
	"encoding/hex"
	"testing"
)

func hashFromHex(t *testing.T, s string) Hash {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != HashSize {
		t.Fatalf("bad test vector %q", s)
	}
	var h Hash
	copy(h[:], b)
	return h
}

// Blake2b-224 of empty input
func TestEmptyRoot(t *testing.T) {
	expected := hashFromHex(t, "836cc68931c2e4e3e838602eca1902591d216837bafddfe6f0c8cb07")
	if got := EmptyRoot(); got != expected {
		t.Fatalf("got %x, want %x", got, expected)
	}
}

// Blake2b-224(0x01 || "utxo/v0" || 0x0102 || 0x0304)
func TestEntryDigest(t *testing.T) {
	expected := hashFromHex(t, "c9535b87a76815a7fef321f6e8c23fc215ba20433e802203607b871f")
	got := EntryDigest("utxo/v0", []byte{0x01, 0x02}, []byte{0x03, 0x04})
	if got != expected {
		t.Fatalf("got %x, want %x", got, expected)
	}
}

// Blake2b-224(0x00 || leafA || leafB)
func TestNodeDigest(t *testing.T) {
	leafA := EntryDigest("utxo/v0", []byte{0x01, 0x02}, []byte{0x03, 0x04})
	leafB := EntryDigest("utxo/v0", []byte{0x01, 0x03}, []byte{0x05})
	expected := hashFromHex(t, "7cb55d91ce9ab61848d854c13d3e796afeb379590a3f18ea6b3c6468")
	if got := nodeDigest(leafA, leafB); got != expected {
		t.Fatalf("got %x, want %x", got, expected)
	}
}

// Blake2b-224(0x01 || namespace_root); namespace name NOT included
func TestNamespaceLeafDigest(t *testing.T) {
	root := hashFromHex(t, "6bce4bbcc1ecfcb7a8cba0351a5cda149f404aa57b08a218718eb844")
	expected := hashFromHex(t, "835f179aa2e998c75f7fdcde927fe9c2b39cd229d807015d4c9f1ac6")
	if got := NamespaceLeafDigest(root); got != expected {
		t.Fatalf("got %x, want %x", got, expected)
	}
}
