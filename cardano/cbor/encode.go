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
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
	"unicode/utf8"
)

// Marshal encodes value using deterministic CBOR.
func Marshal(value any) ([]byte, error) {
	var result []byte
	if err := appendValue(&result, value, 0); err != nil {
		return nil, err
	}
	return result, nil
}

func appendValue(dst *[]byte, value any, depth int) error {
	if depth > maxDepth {
		return ErrDepth
	}
	switch value := value.(type) {
	case nil:
		*dst = append(*dst, 0xf6)
	case bool:
		if value {
			*dst = append(*dst, 0xf5)
		} else {
			*dst = append(*dst, 0xf4)
		}
	case uint:
		appendArgument(dst, 0, uint64(value))
	case uint8:
		appendArgument(dst, 0, uint64(value))
	case uint16:
		appendArgument(dst, 0, uint64(value))
	case uint32:
		appendArgument(dst, 0, uint64(value))
	case uint64:
		appendArgument(dst, 0, value)
	case int:
		appendSigned(dst, int64(value))
	case int8:
		appendSigned(dst, int64(value))
	case int16:
		appendSigned(dst, int64(value))
	case int32:
		appendSigned(dst, int64(value))
	case int64:
		appendSigned(dst, value)
	case Negative:
		appendArgument(dst, 1, uint64(value))
	case []byte:
		appendArgument(dst, 2, uint64(len(value)))
		*dst = append(*dst, value...)
	case string:
		if !utf8.ValidString(value) {
			return errors.Join(ErrInvalid, errors.New("invalid UTF-8 text string"))
		}
		appendArgument(dst, 3, uint64(len(value)))
		*dst = append(*dst, value...)
	case Array:
		appendArgument(dst, 4, uint64(len(value)))
		for _, item := range value {
			if err := appendValue(dst, item, depth+1); err != nil {
				return err
			}
		}
	case Map:
		return appendMap(dst, value, depth)
	case Tag:
		appendArgument(dst, 6, value.Number)
		return appendValue(dst, value.Value, depth+1)
	case Simple:
		switch {
		case value < 20:
			*dst = append(*dst, 0xe0|byte(value))
		case value == 23:
			*dst = append(*dst, 0xf7)
		case value >= 32:
			*dst = append(*dst, 0xf8, byte(value))
		default:
			return fmt.Errorf("%w: simple value %d has a dedicated representation", ErrUnsupported, value)
		}
	default:
		return fmt.Errorf("%w: Go type %T", ErrUnsupported, value)
	}
	return nil
}

func appendSigned(dst *[]byte, value int64) {
	if value >= 0 {
		appendArgument(dst, 0, uint64(value))
		return
	}
	appendArgument(dst, 1, uint64(-(value + 1)))
}

func appendArgument(dst *[]byte, major byte, value uint64) {
	prefix := major << 5
	switch {
	case value < 24:
		*dst = append(*dst, prefix|byte(value))
	case value <= math.MaxUint8:
		*dst = append(*dst, prefix|24, byte(value))
	case value <= math.MaxUint16:
		*dst = append(*dst, prefix|25, 0, 0)
		binary.BigEndian.PutUint16((*dst)[len(*dst)-2:], uint16(value))
	case value <= math.MaxUint32:
		*dst = append(*dst, prefix|26, 0, 0, 0, 0)
		binary.BigEndian.PutUint32((*dst)[len(*dst)-4:], uint32(value))
	default:
		*dst = append(*dst, prefix|27, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64((*dst)[len(*dst)-8:], value)
	}
}

type encodedEntry struct {
	key   []byte
	value any
}

func appendMap(dst *[]byte, value Map, depth int) error {
	entries := make([]encodedEntry, 0, len(value))
	for _, entry := range value {
		var key []byte
		if err := appendValue(&key, entry.Key, depth+1); err != nil {
			return fmt.Errorf("map key: %w", err)
		}
		entries = append(entries, encodedEntry{key: key, value: entry.Value})
	}
	sort.Slice(entries, func(i, j int) bool {
		return compareKeys(entries[i].key, entries[j].key) < 0
	})
	for i := 1; i < len(entries); i++ {
		if bytes.Equal(entries[i-1].key, entries[i].key) {
			return ErrDuplicateMapKey
		}
	}
	appendArgument(dst, 5, uint64(len(entries)))
	for _, entry := range entries {
		*dst = append(*dst, entry.key...)
		if err := appendValue(dst, entry.value, depth+1); err != nil {
			return err
		}
	}
	return nil
}
