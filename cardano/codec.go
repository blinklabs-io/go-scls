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
	"fmt"

	"github.com/blinklabs-io/go-scls/cardano/cbor"
)

var (
	// ErrKeySize indicates a namespace key with the wrong byte length.
	ErrKeySize = errors.New("invalid namespace key size")
	// ErrKeyType indicates a value unsupported by a key encoder.
	ErrKeyType = errors.New("invalid namespace key type")
)

// KeyCodec converts between a namespace's typed key and its fixed-size raw
// bytes in an SCLS entry.
type KeyCodec interface {
	Size() int
	DecodeKey([]byte) (any, error)
	EncodeKey(any) ([]byte, error)
}

// PayloadCodec converts between a namespace's dependency-neutral value and
// its canonical CBOR bytes.
type PayloadCodec interface {
	DecodePayload([]byte) (any, error)
	EncodePayload(any) ([]byte, error)
}

// RawKey is a dependency-neutral fixed-size namespace key.
type RawKey []byte

// FixedKeyCodec validates raw fixed-size keys.
type FixedKeyCodec struct {
	size int
}

// NewFixedKeyCodec creates a raw key codec for size bytes.
func NewFixedKeyCodec(size int) (FixedKeyCodec, error) {
	if size <= 0 {
		return FixedKeyCodec{}, fmt.Errorf("%w: %d", ErrKeySize, size)
	}
	return FixedKeyCodec{size: size}, nil
}

// MustFixedKeyCodec creates a raw key codec and panics if size is invalid.
func MustFixedKeyCodec(size int) FixedKeyCodec {
	codec, err := NewFixedKeyCodec(size)
	if err != nil {
		panic(err)
	}
	return codec
}

// Size returns the required raw key length.
func (c FixedKeyCodec) Size() int {
	return c.size
}

// DecodeKey validates and copies a raw key.
func (c FixedKeyCodec) DecodeKey(data []byte) (any, error) {
	if len(data) != c.size {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrKeySize, len(data), c.size)
	}
	return RawKey(bytes.Clone(data)), nil
}

// EncodeKey validates and copies a RawKey or []byte.
func (c FixedKeyCodec) EncodeKey(value any) ([]byte, error) {
	var data []byte
	switch value := value.(type) {
	case RawKey:
		data = value
	case []byte:
		data = value
	default:
		return nil, fmt.Errorf("%w: %T", ErrKeyType, value)
	}
	if len(data) != c.size {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrKeySize, len(data), c.size)
	}
	return bytes.Clone(data), nil
}

// CanonicalPayloadCodec validates and converts the generic values supported by
// package cardano/cbor.
type CanonicalPayloadCodec struct{}

// DecodePayload decodes one deterministic CBOR item.
func (CanonicalPayloadCodec) DecodePayload(data []byte) (any, error) {
	return cbor.Unmarshal(data)
}

// EncodePayload encodes a generic value as deterministic CBOR.
func (CanonicalPayloadCodec) EncodePayload(value any) ([]byte, error) {
	return cbor.Marshal(value)
}
