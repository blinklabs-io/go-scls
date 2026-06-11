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
	"errors"
	"testing"
)

func testManifest(t *testing.T) *Manifest {
	t.Helper()
	return &Manifest{
		SlotNo:       1234567,
		TotalEntries: 3,
		TotalChunks:  2,
		RootHash:     hashFromHex(t, "61f4542182c577ab9471a9fb00d0a1dda75a0a06752bc1452fe5a190"),
		Namespaces: []NamespaceInfo{
			{
				Name:         "pool_stake/v0",
				EntriesCount: 1,
				ChunksCount:  1,
				Digest:       hashFromHex(t, "c9535b87a76815a7fef321f6e8c23fc215ba20433e802203607b871f"),
			},
			{
				Name:         "utxo/v0",
				EntriesCount: 2,
				ChunksCount:  1,
				Digest:       hashFromHex(t, "7cb55d91ce9ab61848d854c13d3e796afeb379590a3f18ea6b3c6468"),
			},
		},
		PrevManifest: 0,
		Summary: ManifestSummary{
			CreatedAt: "2026-06-11T00:00:00Z",
			Tool:      "go-scls/0.1.0",
		},
	}
}

func TestManifestRoundTrip(t *testing.T) {
	in := testManifest(t)
	payload, err := encodeManifest(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := decodeManifest(payload)
	if err != nil {
		t.Fatal(err)
	}
	if out.SlotNo != in.SlotNo || out.TotalEntries != in.TotalEntries ||
		out.TotalChunks != in.TotalChunks || out.RootHash != in.RootHash ||
		out.PrevManifest != in.PrevManifest {
		t.Fatalf("manifest fields mismatch: %+v", out)
	}
	if len(out.Namespaces) != 2 ||
		out.Namespaces[0] != in.Namespaces[0] ||
		out.Namespaces[1] != in.Namespaces[1] {
		t.Fatalf("namespaces mismatch: %+v", out.Namespaces)
	}
	if out.Summary != in.Summary {
		t.Fatalf("summary mismatch: %+v", out.Summary)
	}
}

// The manifest payload ends with offset == 1 + len(payload): the same value
// as the record's u32 size prefix (RECONCILIATION #7), so the manifest can
// be located by reading the last 4 bytes of a non-amended file.
func TestManifestOffsetBookend(t *testing.T) {
	payload, err := encodeManifest(testManifest(t))
	if err != nil {
		t.Fatal(err)
	}
	got := binary.BigEndian.Uint32(payload[len(payload)-4:])
	if got != uint32(len(payload))+1 {
		t.Fatalf("offset field %d, want %d", got, len(payload)+1)
	}
}

func TestManifestTruncated(t *testing.T) {
	payload, err := encodeManifest(testManifest(t))
	if err != nil {
		t.Fatal(err)
	}
	_, err = decodeManifest(payload[:20])
	if !errors.Is(err, ErrTruncatedRecord) {
		t.Fatalf("want ErrTruncatedRecord, got %v", err)
	}
}

func TestManifestRejectsBadOffset(t *testing.T) {
	payload, err := encodeManifest(testManifest(t))
	if err != nil {
		t.Fatal(err)
	}
	payload[len(payload)-1] ^= 0xff
	if _, err := decodeManifest(payload); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("want ErrInvalidManifest, got %v", err)
	}
}

func TestManifestRejectsTrailingBytes(t *testing.T) {
	payload, err := encodeManifest(testManifest(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeManifest(append(payload, 0x00)); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("want ErrInvalidManifest, got %v", err)
	}
}
