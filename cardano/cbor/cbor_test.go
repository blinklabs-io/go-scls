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
	"math"
	"reflect"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value any
	}{
		{name: "uint", value: uint64(math.MaxUint64)},
		{name: "negative", value: Negative(math.MaxUint64)},
		{name: "bytes", value: []byte{0, 1, 2}},
		{name: "text", value: "ledger"},
		{name: "array", value: Array{uint64(1), "two", nil}},
		{
			name: "map",
			value: Map{
				{Key: "aa", Value: uint64(2)},
				{Key: "b", Value: uint64(1)},
			},
		},
		{name: "tag", value: Tag{Number: 258, Value: Array{uint64(1)}}},
		{name: "false", value: false},
		{name: "true", value: true},
		{name: "null", value: nil},
		{name: "simple", value: Simple(32)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			encoded, err := Marshal(test.value)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			decoded, err := Unmarshal(encoded)
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			reencoded, err := Marshal(decoded)
			if err != nil {
				t.Fatalf("Marshal(decoded) error = %v", err)
			}
			if !bytes.Equal(reencoded, encoded) {
				t.Fatalf("re-encode = %x, want %x", reencoded, encoded)
			}
		})
	}
}

func TestDecodeExpectedValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		encoded []byte
		want    any
	}{
		{encoded: []byte{0x00}, want: uint64(0)},
		{encoded: []byte{0x20}, want: Negative(0)},
		{encoded: []byte{0x40}, want: []byte{}},
		{encoded: []byte{0x60}, want: ""},
		{encoded: []byte{0x80}, want: Array{}},
		{encoded: []byte{0xa0}, want: Map{}},
		{encoded: []byte{0xc1, 0x00}, want: Tag{Number: 1, Value: uint64(0)}},
		{encoded: []byte{0xf4}, want: false},
		{encoded: []byte{0xf5}, want: true},
		{encoded: []byte{0xf6}, want: nil},
		{encoded: []byte{0xf7}, want: Simple(23)},
		{encoded: []byte{0xe0}, want: Simple(0)},
		{encoded: []byte{0xf8, 0x20}, want: Simple(32)},
	}
	for _, test := range tests {
		got, err := Unmarshal(test.encoded)
		if err != nil {
			t.Fatalf("Unmarshal(%x) error = %v", test.encoded, err)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("Unmarshal(%x) = %#v, want %#v", test.encoded, got, test.want)
		}
	}
}

func TestRejectsNonCanonicalEncoding(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		encoded []byte
		want    error
	}{
		{name: "uint", encoded: []byte{0x18, 0x17}, want: ErrNonMinimal},
		{name: "negative", encoded: []byte{0x38, 0x17}, want: ErrNonMinimal},
		{name: "bytes length", encoded: []byte{0x58, 0x00}, want: ErrNonMinimal},
		{name: "text length", encoded: []byte{0x78, 0x00}, want: ErrNonMinimal},
		{name: "array length", encoded: []byte{0x98, 0x00}, want: ErrNonMinimal},
		{name: "map length", encoded: []byte{0xb8, 0x00}, want: ErrNonMinimal},
		{name: "tag", encoded: []byte{0xd8, 0x17, 0x00}, want: ErrNonMinimal},
		{name: "simple", encoded: []byte{0xf8, 0x17}, want: ErrNonMinimal},
		{name: "indefinite bytes", encoded: []byte{0x5f, 0xff}, want: ErrIndefinite},
		{name: "indefinite array", encoded: []byte{0x9f, 0xff}, want: ErrIndefinite},
		{
			name:    "duplicate map key",
			encoded: []byte{0xa2, 0x01, 0x00, 0x01, 0x01},
			want:    ErrDuplicateMapKey,
		},
		{
			name:    "numeric map order",
			encoded: []byte{0xa2, 0x01, 0x00, 0x00, 0x00},
			want:    ErrMapOrder,
		},
		{
			name:    "length-first map order",
			encoded: []byte{0xa2, 0x62, 'a', 'a', 0x00, 0x61, 'b', 0x00},
			want:    ErrMapOrder,
		},
		{name: "trailing data", encoded: []byte{0x00, 0x00}, want: ErrTrailingData},
		{name: "float", encoded: []byte{0xf9, 0x00, 0x00}, want: ErrUnsupported},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(test.encoded)
			if !errors.Is(err, test.want) {
				t.Fatalf("Validate(%x) error = %v, want %v", test.encoded, err, test.want)
			}
		})
	}
}

func TestRejectsMalformedEncoding(t *testing.T) {
	t.Parallel()
	tests := [][]byte{
		nil,
		{0x1c},
		{0x18},
		{0x42, 0x00},
		{0x61, 0xff},
		{0x81},
		{0xa1, 0x00},
	}
	for _, encoded := range tests {
		if err := Validate(encoded); !errors.Is(err, ErrInvalid) {
			t.Errorf("Validate(%x) error = %v, want ErrInvalid", encoded, err)
		}
	}
}

func TestMarshalSortsMapAndRejectsDuplicates(t *testing.T) {
	t.Parallel()
	value := Map{
		{Key: "aa", Value: uint64(2)},
		{Key: "b", Value: uint64(1)},
	}
	encoded, err := Marshal(value)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	want := []byte{0xa2, 0x61, 'b', 0x01, 0x62, 'a', 'a', 0x02}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("Marshal() = %x, want %x", encoded, want)
	}

	_, err = Marshal(Map{
		{Key: uint64(1), Value: false},
		{Key: uint8(1), Value: true},
	})
	if !errors.Is(err, ErrDuplicateMapKey) {
		t.Fatalf("Marshal(duplicate) error = %v, want ErrDuplicateMapKey", err)
	}
}

func TestUnmarshalCopiesByteStrings(t *testing.T) {
	t.Parallel()
	encoded := []byte{0x42, 0xaa, 0xbb}
	value, err := Unmarshal(encoded)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	decoded := value.([]byte)
	encoded[1] = 0
	if decoded[0] != 0xaa {
		t.Fatal("decoded byte string aliases input")
	}
}

func TestDepthLimit(t *testing.T) {
	t.Parallel()
	value := any(uint64(0))
	for range maxDepth + 2 {
		value = Array{value}
	}
	if _, err := Marshal(value); !errors.Is(err, ErrDepth) {
		t.Fatalf("Marshal() error = %v, want ErrDepth", err)
	}
}

func FuzzValidate(f *testing.F) {
	f.Add([]byte{0xa1, 0x00, 0x01})
	f.Add([]byte{0x9f, 0xff})
	f.Add([]byte{0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	f.Fuzz(func(t *testing.T, data []byte) {
		value, err := Unmarshal(data)
		if err != nil {
			return
		}
		encoded, err := Marshal(value)
		if err != nil {
			t.Fatalf("Marshal(decoded) error = %v", err)
		}
		if !bytes.Equal(encoded, data) {
			t.Fatalf("canonical re-encode = %x, input = %x", encoded, data)
		}
	})
}
