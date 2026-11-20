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
	"errors"
	"fmt"
	"testing"
)

func TestWriterEmptyFile(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	root, err := w.Close(100)
	if err != nil {
		t.Fatal(err)
	}
	if root != EmptyRoot() {
		t.Fatalf("empty file root %x, want EmptyRoot", root)
	}
	if buf.Len() == 0 {
		t.Fatal("no bytes written")
	}
}

func TestWriterRejectsUnsupportedChunkFormat(t *testing.T) {
	var buf bytes.Buffer
	if _, err := NewWriter(&buf, WithChunkFormat(0xff)); !errors.Is(err, ErrUnsupportedChunkFormat) {
		t.Fatalf("want ErrUnsupportedChunkFormat, got %v", err)
	}
	if buf.Len() != 0 {
		t.Fatal("invalid configuration wrote a header")
	}
}

func TestWriterOrderingEnforcement(t *testing.T) {
	newW := func() *Writer {
		var buf bytes.Buffer
		w, err := NewWriter(&buf)
		if err != nil {
			t.Fatal(err)
		}
		return w
	}

	// duplicate / descending key
	w := newW()
	if err := w.AddEntry("utxo/v0", []byte{0x02}, []byte{0x01}); err != nil {
		t.Fatal(err)
	}
	if err := w.AddEntry("utxo/v0", []byte{0x02}, []byte{0x01}); !errors.Is(err, ErrEntryOrder) {
		t.Fatalf("duplicate key: want ErrEntryOrder, got %v", err)
	}
	if err := w.AddEntry("utxo/v0", []byte{0x01}, []byte{0x01}); !errors.Is(err, ErrEntryOrder) {
		t.Fatalf("descending key: want ErrEntryOrder, got %v", err)
	}

	// descending namespace
	w = newW()
	if err := w.AddEntry("utxo/v0", []byte{0x01}, []byte{0x01}); err != nil {
		t.Fatal(err)
	}
	if err := w.AddEntry("pool_stake/v0", []byte{0x01}, []byte{0x01}); !errors.Is(err, ErrNamespaceOrder) {
		t.Fatalf("descending namespace: want ErrNamespaceOrder, got %v", err)
	}

	// inconsistent key length within a namespace
	w = newW()
	if err := w.AddEntry("utxo/v0", []byte{0x01, 0x00}, []byte{0x01}); err != nil {
		t.Fatal(err)
	}
	if err := w.AddEntry("utxo/v0", []byte{0x02, 0x00, 0x00}, []byte{0x01}); !errors.Is(err, ErrKeyLength) {
		t.Fatalf("key length: want ErrKeyLength, got %v", err)
	}

	// closed writer
	w = newW()
	if _, err := w.Close(1); err != nil {
		t.Fatal(err)
	}
	if err := w.AddEntry("utxo/v0", []byte{0x01}, []byte{0x01}); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed: want ErrClosed, got %v", err)
	}
	if _, err := w.Close(1); !errors.Is(err, ErrClosed) {
		t.Fatalf("double close: want ErrClosed, got %v", err)
	}
}

// The writer must refuse strings its own decoders reject: decodeChunk
// validates namespace UTF-8 and decodeManifest validates summary strings.
func TestWriterRejectsInvalidUTF8(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.AddEntry("bad\xff/v0", []byte{0x01}, nil); !errors.Is(err, ErrNamespaceOrder) {
		t.Fatalf("namespace: want ErrNamespaceOrder, got %v", err)
	}

	buf.Reset()
	w, err = NewWriter(&buf, WithSummary(ManifestSummary{Tool: "bad\xff"}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Close(1); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("summary: want ErrInvalidManifest, got %v", err)
	}
}

// Root over two namespaces must equal the global tree built from
// per-namespace roots in lexicographic namespace order.
func TestWriterRootMatchesManualTree(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	// "pool_stake/v0" < "utxo/v0" bytewise
	if err := w.AddEntry("pool_stake/v0", []byte{0x0a}, []byte{0x01}); err != nil {
		t.Fatal(err)
	}
	if err := w.AddEntry("utxo/v0", []byte{0x01, 0x02}, []byte{0x03, 0x04}); err != nil {
		t.Fatal(err)
	}
	if err := w.AddEntry("utxo/v0", []byte{0x01, 0x03}, []byte{0x05}); err != nil {
		t.Fatal(err)
	}
	root, err := w.Close(42)
	if err != nil {
		t.Fatal(err)
	}

	psRoot := EntryDigest("pool_stake/v0", []byte{0x0a}, []byte{0x01}) // single leaf
	utxoRoot := nodeDigest(
		EntryDigest("utxo/v0", []byte{0x01, 0x02}, []byte{0x03, 0x04}),
		EntryDigest("utxo/v0", []byte{0x01, 0x03}, []byte{0x05}),
	)
	var global MerkleTree
	global.Add(NamespaceLeafDigest(psRoot))
	global.Add(NamespaceLeafDigest(utxoRoot))
	if want := global.Root(); root != want {
		t.Fatalf("writer root %x, want %x", root, want)
	}
}

// Chunk rotation: maxChunkEntries=2 with 5 entries -> 3 chunks.
func TestWriterChunkRotation(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewWriter(&buf, WithMaxChunkEntries(2))
	if err != nil {
		t.Fatal(err)
	}
	for i := byte(0); i < 5; i++ {
		if err := w.AddEntry("utxo/v0", []byte{i}, []byte{0x01}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.Close(7); err != nil {
		t.Fatal(err)
	}
	// count CHUNK records in the raw stream
	r := bytes.NewReader(buf.Bytes())
	chunks := 0
	for {
		recType, _, err := readRecord(r)
		if err != nil {
			break
		}
		if recType == RecordTypeChunk {
			chunks++
		}
	}
	if chunks != 3 {
		t.Fatalf("got %d chunks, want 3", chunks)
	}
}

// Size-based rotation: each entry is 6 wire bytes (4-byte size prefix +
// 1-byte key + 1-byte value); with a 16-byte budget the third entry crosses
// the threshold, so 5 entries split into chunks of 3 and 2.
func TestWriterChunkRotationBySize(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewWriter(&buf, WithMaxChunkBytes(16))
	if err != nil {
		t.Fatal(err)
	}
	for i := byte(0); i < 5; i++ {
		if err := w.AddEntry("utxo/v0", []byte{i}, []byte{0x01}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.Close(7); err != nil {
		t.Fatal(err)
	}
	var sizes []int
	r := bytes.NewReader(buf.Bytes())
	for {
		recType, payload, err := readRecord(r)
		if err != nil {
			break
		}
		if recType == RecordTypeChunk {
			c, err := decodeChunk(payload)
			if err != nil {
				t.Fatal(err)
			}
			sizes = append(sizes, len(c.Entries))
		}
	}
	if len(sizes) != 2 || sizes[0] != 3 || sizes[1] != 2 {
		t.Fatalf("chunk entry counts %v, want [3 2]", sizes)
	}
	if _, err := Verify(bytes.NewReader(buf.Bytes()), VerifyFull); err != nil {
		t.Fatalf("rotated file does not verify: %v", err)
	}
}

// chunk_seq is per-namespace, starting at 0 (RECONCILIATION #9).
func TestWriterSeqResetsPerNamespace(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewWriter(&buf, WithMaxChunkEntries(1))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []struct {
		ns  string
		key byte
	}{{"a", 1}, {"a", 2}, {"b", 1}} {
		if err := w.AddEntry(e.ns, []byte{e.key}, []byte{0x01}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.Close(7); err != nil {
		t.Fatal(err)
	}
	var seqs []uint64
	r := bytes.NewReader(buf.Bytes())
	for {
		recType, payload, err := readRecord(r)
		if err != nil {
			break
		}
		if recType == RecordTypeChunk {
			c, err := decodeChunk(payload)
			if err != nil {
				t.Fatal(err)
			}
			seqs = append(seqs, c.Seq)
		}
	}
	if len(seqs) != 3 || seqs[0] != 0 || seqs[1] != 1 || seqs[2] != 0 {
		t.Fatalf("seqs %v, want [0 1 0]", seqs)
	}
}

func TestWriterCompressedFormatsPreserveFileContract(t *testing.T) {
	t.Parallel()
	var raw bytes.Buffer
	rawWriter, err := NewWriter(&raw, WithMaxChunkEntries(2))
	if err != nil {
		t.Fatal(err)
	}
	for i := byte(1); i <= 4; i++ {
		if err := rawWriter.AddEntry("utxo/v0", []byte{i}, bytes.Repeat([]byte{i}, 128)); err != nil {
			t.Fatal(err)
		}
	}
	wantRoot, err := rawWriter.Close(7)
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []byte{ChunkFormatRaw, ChunkFormatZstd, ChunkFormatZstdEntries} {
		format := format
		t.Run(fmt.Sprintf("0x%02x", format), func(t *testing.T) {
			var buf bytes.Buffer
			w, err := NewWriter(&buf, WithChunkFormat(format), WithZstdLevel(3), WithMaxChunkEntries(2))
			if err != nil {
				t.Fatal(err)
			}
			for i := byte(1); i <= 4; i++ {
				if err := w.AddEntry("utxo/v0", []byte{i}, bytes.Repeat([]byte{i}, 128)); err != nil {
					t.Fatal(err)
				}
			}
			root, err := w.Close(7)
			if err != nil {
				t.Fatal(err)
			}
			if root != wantRoot {
				t.Fatal("compression changed Merkle root")
			}
			if _, err := Verify(bytes.NewReader(buf.Bytes()), VerifyFull); err != nil {
				t.Fatal(err)
			}
			r := bytes.NewReader(buf.Bytes())
			for {
				recordType, payload, readErr := readRecord(r)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if recordType != RecordTypeChunk {
					continue
				}
				chunk, decodeErr := decodeChunk(payload)
				if decodeErr != nil {
					t.Fatal(decodeErr)
				}
				if chunk.Format != format {
					t.Fatalf("chunk format 0x%02x, want 0x%02x", chunk.Format, format)
				}
				break
			}
			s, err := Open(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
			if err != nil {
				t.Fatal(err)
			}
			got, err := s.Get("utxo/v0", []byte{3})
			if err != nil || !bytes.Equal(got, bytes.Repeat([]byte{3}, 128)) {
				t.Fatalf("Get: %x, %v", got, err)
			}
			got, err = Lookup(bytes.NewReader(buf.Bytes()), "utxo/v0", []byte{3})
			if err != nil || !bytes.Equal(got, bytes.Repeat([]byte{3}, 128)) {
				t.Fatalf("Lookup: %x, %v", got, err)
			}
			for _, lookup := range []struct {
				name  string
				prove func() ([]byte, Proof, error)
			}{
				{"Prove", func() ([]byte, Proof, error) {
					return s.Prove("utxo/v0", []byte{3})
				}},
				{"LookupProof", func() ([]byte, Proof, error) {
					return LookupProof(bytes.NewReader(buf.Bytes()), "utxo/v0", []byte{3})
				}},
			} {
				value, proof, err := lookup.prove()
				if err != nil || !bytes.Equal(value, bytes.Repeat([]byte{3}, 128)) {
					t.Fatalf("%s: %x, %v", lookup.name, value, err)
				}
				if err := VerifyProof(wantRoot, "utxo/v0", []byte{3}, value, proof); err != nil {
					t.Fatalf("%s proof: %v", lookup.name, err)
				}
			}

		})
	}
}

func TestWriterZstdEntriesAtDecodedSizeLimit(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewWriter(&buf, WithChunkFormat(ChunkFormatZstdEntries), WithMaxChunkBytes(maxRecordSize))
	if err != nil {
		t.Fatal(err)
	}
	const entryOverhead = 4 + 1
	tail := bytes.Repeat([]byte{1}, 1024)
	if err := w.AddEntry("utxo/v0", []byte{0}, make([]byte, maxRecordSize-2*entryOverhead-len(tail))); err != nil {
		t.Fatal(err)
	}
	if err := w.AddEntry("utxo/v0", []byte{1}, tail); err != nil {
		t.Fatal(err)
	}
	root, err := w.Close(7)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Verify(bytes.NewReader(buf.Bytes()), VerifyFull)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalEntries != 2 || result.TotalChunks != 1 || result.RootHash != root {
		t.Fatalf("unexpected verified file: %+v", result)
	}
}
