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
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// TestConformanceFixtures verifies every .scls file in testdata/ plus an
// optional externally generated file pointed to by SCLS_CONFORMANCE_FILE
// (e.g. produced by cardano-cls or cardano-scrawls).
func TestConformanceFixtures(t *testing.T) {
	files, err := filepath.Glob("testdata/*.scls")
	if err != nil {
		t.Fatal(err)
	}
	if extra := os.Getenv("SCLS_CONFORMANCE_FILE"); extra != "" {
		files = append(files, extra)
	}
	if len(files) == 0 {
		t.Skip("no conformance fixtures available")
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			res, err := Verify(bytes.NewReader(data), VerifyFull)
			if err != nil {
				t.Fatalf("verify %s: %v", f, err)
			}
			t.Logf("%s: %d entries, %d chunks, root %x",
				f, res.TotalEntries, res.TotalChunks, res.RootHash)
		})
	}
}

// goldenNS is one namespace's expected manifest summary.
type goldenNS struct {
	entries, chunks uint64
	digest          string // hex Blake2b-224 per-namespace Merkle root
}

// goldenFile is the full expected VerifyFull result for a committed fixture.
type goldenFile struct {
	entries, chunks uint64
	root            string // hex global Merkle root
	ns              map[string]goldenNS
}

// TestConformanceGolden pins the exact entry/chunk counts, per-namespace
// Merkle roots, and global root for every committed fixture. The digests and
// roots below were produced by the Tweag reference implementations (the
// Haskell tool for minimal-raw.scls, the Rust writer for the rest — see
// testdata/README.md); go-scls must recompute them byte-for-byte. This makes
// the fixtures genuine cross-implementation golden vectors rather than a mere
// "does not error" check, and exercises all three verification levels against
// reference-produced bytes.
func TestConformanceGolden(t *testing.T) {
	golden := map[string]goldenFile{
		"empty.scls": {
			entries: 0, chunks: 0,
			// Blake2b-224 of the empty input: the empty-tree / empty-file root.
			root: "836cc68931c2e4e3e838602eca1902591d216837bafddfe6f0c8cb07",
			ns:   map[string]goldenNS{},
		},
		"minimal-raw.scls": {
			entries: 1, chunks: 1,
			root: "bb8a5d7411c14c15d11065976996ace745b6eaf1c6601bb79f702a7a",
			ns: map[string]goldenNS{
				"blocks/v0": {1, 1, "b86c26d831731eb4699c918abf1e2df880ae68196469a0f091a4ca79"},
			},
		},
		"multi-ns.scls": {
			entries: 9, chunks: 3,
			root: "b2a8707947de6867467d8081e4902ab272c65f53b0ec375499cd30e2",
			ns: map[string]goldenNS{
				"ns/alpha": {3, 1, "94020635fc7cb7491bfb35088c4a44aefad1484fac8193edc312f1f3"},
				"ns/beta":  {2, 1, "0b59e63dd81873f9fd76249237385f412333f89427a4b526aa9886d7"},
				"ns/gamma": {4, 1, "e8c82919cc20afb322d6c9fd4da7f0b827222cf64672b5d881a64a99"},
			},
		},
		"multi-chunk.scls": {
			entries: 50, chunks: 9,
			root: "db9010b218c4a09010a4d8f630ac0d5bc1f5aff9b7fc1c9c2b3f7208",
			ns: map[string]goldenNS{
				"ns/alpha": {50, 9, "e238b82948322a46f0e19860d2290ffc192aeaa057880933cbdc41bb"},
			},
		},
		"multi-ns-multi-chunk.scls": {
			entries: 45, chunks: 9,
			root: "3d8b80878849927490a5f9ed8ed9e10ac388918853bb36b9da052990",
			ns: map[string]goldenNS{
				"ns/alpha": {20, 4, "0fe221e272f417c2f92a4d07f3e0a68532f74047bb870a08c86665da"},
				"ns/beta":  {25, 5, "bb419e0ad226857ecb38f3283fa652af1d8560a62f3145cf3ad15c1d"},
			},
		},
	}

	for name, exp := range golden {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			// Every verification level must accept the reference-produced file.
			for _, lvl := range []VerifyLevel{VerifyStructure, VerifyChunks, VerifyFull} {
				if _, err := Verify(bytes.NewReader(data), lvl); err != nil {
					t.Fatalf("verify level %d: %v", lvl, err)
				}
			}
			res, err := Verify(bytes.NewReader(data), VerifyFull)
			if err != nil {
				t.Fatal(err)
			}
			if res.TotalEntries != exp.entries || res.TotalChunks != exp.chunks {
				t.Errorf("totals = (%d entries, %d chunks), want (%d, %d)",
					res.TotalEntries, res.TotalChunks, exp.entries, exp.chunks)
			}
			if got := hex.EncodeToString(res.RootHash[:]); got != exp.root {
				t.Errorf("global root = %s, want %s", got, exp.root)
			}
			if len(res.Namespaces) != len(exp.ns) {
				t.Fatalf("namespace count = %d, want %d", len(res.Namespaces), len(exp.ns))
			}
			for _, got := range res.Namespaces {
				want, ok := exp.ns[got.Name]
				if !ok {
					t.Errorf("unexpected namespace %q", got.Name)
					continue
				}
				if got.EntriesCount != want.entries || got.ChunksCount != want.chunks {
					t.Errorf("ns %q = (%d entries, %d chunks), want (%d, %d)",
						got.Name, got.EntriesCount, got.ChunksCount, want.entries, want.chunks)
				}
				if d := hex.EncodeToString(got.Digest[:]); d != want.digest {
					t.Errorf("ns %q digest = %s, want %s", got.Name, d, want.digest)
				}
			}
		})
	}
}
