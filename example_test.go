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

package scls_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"

	scls "github.com/blinklabs-io/go-scls"
)

// buildExampleFile writes a small two-namespace snapshot.
func buildExampleFile() []byte {
	var buf bytes.Buffer
	w, err := scls.NewWriter(&buf)
	if err != nil {
		log.Fatal(err)
	}
	// Namespaces in ascending bytewise order, keys ascending within each
	// namespace, one key length per namespace. Values are caller-encoded
	// canonical CBOR, written verbatim.
	if err := w.AddEntry("pool_stake/v0", []byte{0x0a}, []byte{0x01}); err != nil {
		log.Fatal(err)
	}
	if err := w.AddEntry("utxo/v0", []byte{0x01, 0x02}, []byte{0x03}); err != nil {
		log.Fatal(err)
	}
	if err := w.AddEntry("utxo/v0", []byte{0x01, 0x03}, []byte{0x05}); err != nil {
		log.Fatal(err)
	}
	if _, err := w.Close(42); err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}

func ExampleWriter() {
	var buf bytes.Buffer
	w, err := scls.NewWriter(&buf)
	if err != nil {
		log.Fatal(err)
	}
	if err := w.AddEntry("utxo/v0", []byte{0x01, 0x02}, []byte{0x03}); err != nil {
		log.Fatal(err)
	}
	root, err := w.Close(42) // slot number recorded in the manifest
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("global root: %x\n", root)
	// Output:
	// global root: 1bd591ccbbbcf9ecd371b01ec435a10a85b0c186edae82815f0d44ae
}

func ExampleReader() {
	r, err := scls.NewReader(bytes.NewReader(buildExampleFile()))
	if err != nil {
		log.Fatal(err)
	}
	for {
		chunk, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s: %d entries\n", chunk.Namespace, len(chunk.Entries))
	}
	fmt.Printf("slot %d\n", r.Manifest().SlotNo)
	// Output:
	// pool_stake/v0: 1 entries
	// utxo/v0: 2 entries
	// slot 42
}

func ExampleVerify() {
	res, err := scls.Verify(bytes.NewReader(buildExampleFile()), scls.VerifyFull)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d entries in %d chunks across %d namespaces\n",
		res.TotalEntries, res.TotalChunks, len(res.Namespaces))
	// Output:
	// 3 entries in 2 chunks across 2 namespaces
}

func ExampleReadManifest() {
	// ReadManifest seeks to the manifest via the trailing offset bookend
	// instead of scanning the whole file.
	m, err := scls.ReadManifest(bytes.NewReader(buildExampleFile()))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("slot %d, %d entries\n", m.SlotNo, m.TotalEntries)
	// Output:
	// slot 42, 3 entries
}
