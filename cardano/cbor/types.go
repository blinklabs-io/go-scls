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

package cbor

import (
	"errors"
	"fmt"
)

const maxDepth = 128

var (
	// ErrInvalid indicates malformed CBOR.
	ErrInvalid = errors.New("invalid CBOR")
	// ErrTrailingData indicates bytes after the first complete CBOR item.
	ErrTrailingData = errors.New("trailing CBOR data")
	// ErrIndefinite indicates an indefinite-length CBOR item.
	ErrIndefinite = errors.New("indefinite-length CBOR is not canonical")
	// ErrNonMinimal indicates a CBOR argument that uses more bytes than needed.
	ErrNonMinimal = errors.New("non-minimal CBOR encoding")
	// ErrDuplicateMapKey indicates repeated canonical map-key bytes.
	ErrDuplicateMapKey = errors.New("duplicate CBOR map key")
	// ErrMapOrder indicates map keys outside deterministic CBOR order.
	ErrMapOrder = errors.New("non-canonical CBOR map ordering")
	// ErrUnsupported indicates a CBOR value outside the supported subset.
	ErrUnsupported = errors.New("unsupported CBOR value")
	// ErrDepth indicates excessive CBOR nesting.
	ErrDepth = errors.New("CBOR nesting limit exceeded")
)

// SyntaxError reports the byte offset of a CBOR decoding error.
type SyntaxError struct {
	Offset int
	Err    error
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("CBOR offset %d: %v", e.Offset, e.Err)
}

// Unwrap returns the underlying error.
func (e *SyntaxError) Unwrap() error {
	return e.Err
}

// Negative represents the CBOR negative integer -1-n. It supports the entire
// CBOR major-type-1 range without overflowing an int64.
type Negative uint64

// Array is a CBOR array.
type Array []any

// MapEntry is one key-value pair in a CBOR map.
type MapEntry struct {
	Key   any
	Value any
}

// Map is a CBOR map. Unmarshal preserves the encoded deterministic order;
// Marshal sorts entries into deterministic order.
type Map []MapEntry

// Tag is a tagged CBOR value.
type Tag struct {
	Number uint64
	Value  any
}

// Simple represents an unassigned CBOR simple value. Values 20 through 22
// have dedicated Go representations (false, true, nil) and are rejected
// when supplied to Marshal; Simple(23) (undefined) has no dedicated Go type
// and is accepted.
type Simple uint8
