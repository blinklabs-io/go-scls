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
	"fmt"
	"testing"
)

// BenchmarkUnmarshalCollection covers element counts on both sides of
// maxPrealloc: at or below it the decoder reserves the whole collection up
// front, above it append grows the slice, which is the throughput a count
// taken from the input would have bought.
func BenchmarkUnmarshalCollection(b *testing.B) {
	sizes := []int{4, 64, maxPrealloc, maxPrealloc * 8}
	for _, size := range sizes {
		array := make(Array, 0, size)
		entries := make(Map, 0, size)
		for i := range size {
			array = append(array, uint64(i))
			entries = append(entries, MapEntry{Key: uint64(i), Value: uint64(i)})
		}
		for _, shape := range []struct {
			name  string
			value any
		}{{"array", array}, {"map", entries}} {
			data, err := Marshal(shape.value)
			if err != nil {
				b.Fatal(err)
			}
			b.Run(fmt.Sprintf("%s/%d", shape.name, size), func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(data)))
				for b.Loop() {
					if _, err := Unmarshal(data); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
