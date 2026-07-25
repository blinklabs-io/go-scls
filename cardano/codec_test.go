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

package cardano

import (
	"bytes"
	"errors"
	"testing"

	"github.com/blinklabs-io/go-scls/cardano/cbor"
)

func TestFixedKeyCodec(t *testing.T) {
	t.Parallel()
	codec := MustFixedKeyCodec(2)
	input := []byte{0xaa, 0xbb}
	value, err := codec.DecodeKey(input)
	if err != nil {
		t.Fatalf("DecodeKey() error = %v", err)
	}
	input[0] = 0
	key := value.(RawKey)
	if key[0] != 0xaa {
		t.Fatal("DecodeKey() result aliases input")
	}

	encoded, err := codec.EncodeKey(key)
	if err != nil {
		t.Fatalf("EncodeKey() error = %v", err)
	}
	key[0] = 0
	if !bytes.Equal(encoded, []byte{0xaa, 0xbb}) {
		t.Fatal("EncodeKey() result aliases input")
	}

	if _, err := codec.DecodeKey([]byte{0}); !errors.Is(err, ErrKeySize) {
		t.Fatalf("DecodeKey(short) error = %v, want ErrKeySize", err)
	}
	if _, err := codec.EncodeKey("not bytes"); !errors.Is(err, ErrKeyType) {
		t.Fatalf("EncodeKey(type) error = %v, want ErrKeyType", err)
	}
	if _, err := NewFixedKeyCodec(0); !errors.Is(err, ErrKeySize) {
		t.Fatalf("NewFixedKeyCodec(0) error = %v, want ErrKeySize", err)
	}
}

func TestCanonicalPayloadCodec(t *testing.T) {
	t.Parallel()
	codec := CanonicalPayloadCodec{}
	value := cbor.Map{{Key: "balance", Value: uint64(10)}}
	encoded, err := codec.EncodePayload(value)
	if err != nil {
		t.Fatalf("EncodePayload() error = %v", err)
	}
	decoded, err := codec.DecodePayload(encoded)
	if err != nil {
		t.Fatalf("DecodePayload() error = %v", err)
	}
	reencoded, err := codec.EncodePayload(decoded)
	if err != nil {
		t.Fatalf("EncodePayload(decoded) error = %v", err)
	}
	if !bytes.Equal(reencoded, encoded) {
		t.Fatalf("re-encode = %x, want %x", reencoded, encoded)
	}

	if _, err := codec.DecodePayload([]byte{0x18, 0x00}); !errors.Is(err, cbor.ErrNonMinimal) {
		t.Fatalf("DecodePayload(non-minimal) error = %v, want ErrNonMinimal", err)
	}
}
