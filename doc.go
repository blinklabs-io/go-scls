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

// Package scls reads, writes, and verifies Cardano Standard Canonical
// Ledger State (SCLS) files as defined in CIP-0165.
//
// SCLS is a segmented binary container for streaming, verifiable snapshots
// of Cardano ledger state: key-value entries in strictly ascending key
// order, grouped into chunks by namespace (for example "utxo/v0" or
// "pool_stake/v0"), with integrity provided by Blake2b-224 chunk hashes,
// per-namespace Merkle trees, and a global Merkle root recorded in a
// trailing manifest.
//
// Entry values are opaque to this package: callers supply the canonical
// (deterministic) CBOR encoding of each namespace payload and it is written
// to and read from the wire verbatim. Namespace-specific schemas are out of
// scope here and live with the consumers of this library.
//
// # Writing
//
// A Writer streams a file in a single pass. Namespaces must be added in
// strictly ascending bytewise order, keys strictly ascending within each
// namespace, and all keys within a namespace must share one length:
//
//	w, err := scls.NewWriter(f)
//	if err != nil { ... }
//	err = w.AddEntry("utxo/v0", key, valueCbor)
//	root, err := w.Close(slotNo) // writes the MANIFEST, returns global root
//
// # Reading
//
// A Reader streams chunks and exposes the manifest once the stream ends:
//
//	r, err := scls.NewReader(f)
//	for {
//		chunk, err := r.Next()
//		if errors.Is(err, io.EOF) {
//			break
//		}
//		if err != nil { ... }
//		// chunk.Namespace, chunk.Entries
//	}
//	manifest := r.Manifest()
//
// When only the manifest is needed, ReadManifest seeks straight to it via
// the trailing offset bookend instead of scanning the file:
//
//	manifest, err := scls.ReadManifest(f) // f is an io.ReadSeeker
//
// # Verifying
//
// Verify checks a whole file at one of three levels: VerifyStructure
// (ordering, sequence numbers, manifest counts), VerifyChunks (plus chunk
// footer hashes), or VerifyFull (plus per-namespace Merkle roots and the
// global root against the manifest):
//
//	res, err := scls.Verify(f, scls.VerifyFull)
//
// All wire-format decisions are documented against the CIP-0165 spec and
// reference implementations in spec/RECONCILIATION.md within this
// repository.
package scls
