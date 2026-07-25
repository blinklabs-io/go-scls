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

// Validate verifies that data contains exactly one supported deterministic
// CBOR item.
func Validate(data []byte) error {
	_, err := Unmarshal(data)
	return err
}

type decoder struct {
	data   []byte
	offset int
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
		length, err := d.argument(additional, start)
		if err != nil {
			return nil, err
		}
		count, err := d.collectionLength(length)
		if err != nil {
			return nil, err
		}
		value := make(Array, 0, count)
		for range count {
			item, err := d.value(depth + 1)
			if err != nil {
				return nil, err
			}
			value = append(value, item)
		}
		return value, nil
	case 5:
		length, err := d.argument(additional, start)
		if err != nil {
			return nil, err
		}
		count, err := d.collectionLength(length)
		if err != nil {
			return nil, err
		}
		value := make(Map, 0, count)
		var previous []byte
		for range count {
			keyStart := d.offset
			key, err := d.value(depth + 1)
			if err != nil {
				return nil, err
			}
			keyBytes := d.data[keyStart:d.offset]
			if previous != nil {
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
	value, err := d.bytes(uint64(size))
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
	remaining := len(d.data) - d.offset
	if length > uint64(remaining) {
		return nil, d.syntax(errors.Join(ErrInvalid, errors.New("unexpected end of input")))
	}
	end := d.offset + int(length)
	value := d.data[d.offset:end]
	d.offset = end
	return value, nil
}

func (d *decoder) collectionLength(length uint64) (int, error) {
	if length > uint64(len(d.data)-d.offset) || length > uint64(maxInt()) {
		return 0, d.syntax(errors.Join(ErrInvalid, fmt.Errorf("collection length %d exceeds input", length)))
	}
	return int(length), nil
}

func (d *decoder) syntax(err error) error {
	return &SyntaxError{Offset: d.offset, Err: err}
}

func maxInt() int {
	return int(^uint(0) >> 1)
}

func compareKeys(left, right []byte) int {
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return bytes.Compare(left, right)
}
