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
	"bytes"
	"errors"
	"fmt"
	"math"
	"unicode/utf8"
)

// Unmarshal decodes one deterministic CBOR item.
func Unmarshal(data []byte) (any, error) {
	d := decoder{data: data}
	value, err := d.value(0)
	if err != nil {
		return nil, err
	}
	if d.offset != len(data) {
		return nil, d.syntax(ErrTrailingData)
	}
	return value, nil
}

// UnmarshalLedger decodes one CBOR item serialized by the Cardano ledger
// (cardano-ledger-binary). Unlike Unmarshal it accepts map keys in any order,
// because the ledger orders maps and sets by its own key ordering rather than
// RFC 8949 section 4.2.1, and it accepts indefinite-length arrays and maps,
// because the ledger encodes every list, set and map with more than 23
// elements that way. It still rejects duplicate map keys, indefinite-length
// byte and text strings, and non-minimal arguments.
func UnmarshalLedger(data []byte) (any, error) {
	d := decoder{data: data, ledger: true}
	value, err := d.value(0)
	if err != nil {
		return nil, err
	}
	if d.offset != len(data) {
		return nil, d.syntax(ErrTrailingData)
	}
	return value, nil
}

// Validate verifies that data contains exactly one supported deterministic
// CBOR item.
func Validate(data []byte) error {
	_, err := Unmarshal(data)
	return err
}

type decoder struct {
	data   []byte
	offset int
	ledger bool
}

func (d *decoder) value(depth int) (any, error) {
	if depth > maxDepth {
		return nil, d.syntax(ErrDepth)
	}
	start := d.offset
	initial, err := d.byte()
	if err != nil {
		return nil, err
	}
	major := initial >> 5
	additional := initial & 0x1f

	switch major {
	case 0:
		return d.argument(additional, start)
	case 1:
		n, err := d.argument(additional, start)
		return Negative(n), err
	case 2:
		length, err := d.argument(additional, start)
		if err != nil {
			return nil, err
		}
		value, err := d.bytes(length)
		if err != nil {
			return nil, err
		}
		return bytes.Clone(value), nil
	case 3:
		length, err := d.argument(additional, start)
		if err != nil {
			return nil, err
		}
		value, err := d.bytes(length)
		if err != nil {
			return nil, err
		}
		if !utf8.Valid(value) {
			return nil, d.syntax(errors.Join(ErrInvalid, errors.New("invalid UTF-8 text string")))
		}
		return string(value), nil
	case 4:
		count, err := d.itemCount(additional, start, arrayItemMinSize)
		if err != nil {
			return nil, err
		}
		value := make(Array, 0, min(max(count, 0), maxPrealloc))
		for i := 0; count < 0 || i < count; i++ {
			if count < 0 && d.atBreak() {
				break
			}
			item, err := d.value(depth + 1)
			if err != nil {
				return nil, err
			}
			value = append(value, item)
		}
		return value, nil
	case 5:
		count, err := d.itemCount(additional, start, mapEntryMinSize)
		if err != nil {
			return nil, err
		}
		value := make(Map, 0, min(max(count, 0), maxPrealloc))
		var previous []byte
		var seen map[string]struct{}
		if d.ledger {
			seen = make(map[string]struct{}, min(max(count, 0), maxPrealloc))
		}
		for i := 0; count < 0 || i < count; i++ {
			if count < 0 && d.atBreak() {
				break
			}
			keyStart := d.offset
			key, err := d.value(depth + 1)
			if err != nil {
				return nil, err
			}
			keyBytes := d.data[keyStart:d.offset]
			if seen != nil {
				if _, dup := seen[string(keyBytes)]; dup {
					return nil, &SyntaxError{Offset: keyStart, Err: ErrDuplicateMapKey}
				}
				seen[string(keyBytes)] = struct{}{}
			} else if previous != nil {
				switch compareKeys(previous, keyBytes) {
				case 0:
					return nil, &SyntaxError{Offset: keyStart, Err: ErrDuplicateMapKey}
				case 1:
					return nil, &SyntaxError{Offset: keyStart, Err: ErrMapOrder}
				}
			}
			previous = keyBytes
			item, err := d.value(depth + 1)
			if err != nil {
				return nil, err
			}
			value = append(value, MapEntry{Key: key, Value: item})
		}
		return value, nil
	case 6:
		number, err := d.argument(additional, start)
		if err != nil {
			return nil, err
		}
		item, err := d.value(depth + 1)
		if err != nil {
			return nil, err
		}
		return Tag{Number: number, Value: item}, nil
	case 7:
		return d.simple(additional, start)
	default:
		panic("unreachable CBOR major type")
	}
}

func (d *decoder) simple(additional byte, start int) (any, error) {
	switch additional {
	case 20:
		return false, nil
	case 21:
		return true, nil
	case 22:
		return nil, nil
	case 23:
		return Simple(23), nil
	case 24:
		value, err := d.byte()
		if err != nil {
			return nil, err
		}
		if value < 32 {
			return nil, &SyntaxError{Offset: start, Err: ErrNonMinimal}
		}
		return Simple(value), nil
	case 25, 26, 27:
		return nil, &SyntaxError{Offset: start, Err: errors.Join(ErrUnsupported, errors.New("floating-point value"))}
	case 31:
		return nil, &SyntaxError{Offset: start, Err: ErrIndefinite}
	default:
		if additional < 20 {
			return Simple(additional), nil
		}
		return nil, &SyntaxError{Offset: start, Err: ErrInvalid}
	}
}

func (d *decoder) argument(additional byte, start int) (uint64, error) {
	switch {
	case additional < 24:
		return uint64(additional), nil
	case additional == 24:
		value, err := d.byte()
		if err != nil {
			return 0, err
		}
		if value < 24 {
			return 0, &SyntaxError{Offset: start, Err: ErrNonMinimal}
		}
		return uint64(value), nil
	case additional == 25:
		value, err := d.uint(2)
		if err != nil {
			return 0, err
		}
		if value <= math.MaxUint8 {
			return 0, &SyntaxError{Offset: start, Err: ErrNonMinimal}
		}
		return value, nil
	case additional == 26:
		value, err := d.uint(4)
		if err != nil {
			return 0, err
		}
		if value <= math.MaxUint16 {
			return 0, &SyntaxError{Offset: start, Err: ErrNonMinimal}
		}
		return value, nil
	case additional == 27:
		value, err := d.uint(8)
		if err != nil {
			return 0, err
		}
		if value <= math.MaxUint32 {
			return 0, &SyntaxError{Offset: start, Err: ErrNonMinimal}
		}
		return value, nil
	case additional == 31:
		return 0, &SyntaxError{Offset: start, Err: ErrIndefinite}
	default:
		return 0, &SyntaxError{Offset: start, Err: ErrInvalid}
	}
}

func (d *decoder) byte() (byte, error) {
	if d.offset >= len(d.data) {
		return 0, d.syntax(errors.Join(ErrInvalid, errors.New("unexpected end of input")))
	}
	value := d.data[d.offset]
	d.offset++
	return value, nil
}

func (d *decoder) uint(size int) (uint64, error) {
	value, err := d.bytesSize(size)
	if err != nil {
		return 0, err
	}
	var result uint64
	for _, b := range value {
		result = result<<8 | uint64(b)
	}
	return result, nil
}

func (d *decoder) bytes(length uint64) ([]byte, error) {
	if length > math.MaxInt {
		return nil, d.syntax(errors.Join(ErrInvalid, errors.New("item length exceeds platform limit")))
	}
	// The math.MaxInt check above makes the conversion safe.
	lengthInt := int(length) //nolint:gosec
	remaining := len(d.data) - d.offset
	if lengthInt > remaining {
		return nil, d.syntax(errors.Join(ErrInvalid, errors.New("unexpected end of input")))
	}
	end := d.offset + lengthInt
	value := d.data[d.offset:end]
	d.offset = end
	return value, nil
}

func (d *decoder) bytesSize(length int) ([]byte, error) {
	remaining := len(d.data) - d.offset
	if length < 0 || length > remaining {
		return nil, d.syntax(errors.Join(ErrInvalid, errors.New("unexpected end of input")))
	}
	end := d.offset + length
	value := d.data[d.offset:end]
	d.offset = end
	return value, nil
}

// Smallest encoded size of one collection element. Every CBOR item occupies
// at least one byte, and a map entry is a key item followed by a value item.
const (
	arrayItemMinSize = 1
	mapEntryMinSize  = 2
)

// collectionLength converts a declared element count to an int, rejecting any
// count the remaining input cannot possibly satisfy.
//
// The returned count is still attacker-controlled up to the remaining input
// length, so callers must not use it to size an allocation; see maxPrealloc.
// itemCount reads an array or map header. It returns -1 for an
// indefinite-length header, which only the ledger decoder accepts; the
// caller then reads items until atBreak. Every item consumes at least one
// byte, so that loop is bounded by the input.
func (d *decoder) itemCount(additional byte, start int, minItemSize int) (int, error) {
	if additional == 31 && d.ledger {
		return -1, nil
	}
	length, err := d.argument(additional, start)
	if err != nil {
		return 0, err
	}
	return d.collectionLength(length, minItemSize)
}

// atBreak consumes the break byte that ends an indefinite-length item.
func (d *decoder) atBreak() bool {
	if d.offset < len(d.data) && d.data[d.offset] == 0xff {
		d.offset++
		return true
	}
	return false
}

func (d *decoder) collectionLength(length uint64, minItemSize int) (int, error) {
	if length > math.MaxInt {
		return 0, d.syntax(errors.Join(ErrInvalid, fmt.Errorf("collection length %d exceeds input", length)))
	}
	// The math.MaxInt check above makes the conversion safe.
	count := int(length) //nolint:gosec
	if count > (len(d.data)-d.offset)/minItemSize {
		return 0, d.syntax(errors.Join(ErrInvalid, fmt.Errorf("collection length %d exceeds input", length)))
	}
	return count, nil
}

func (d *decoder) syntax(err error) error {
	return &SyntaxError{Offset: d.offset, Err: err}
}

func compareKeys(left, right []byte) int {
	return bytes.Compare(left, right)
}
