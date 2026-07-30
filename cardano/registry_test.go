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
	"errors"
	"strconv"
	"sync"
	"testing"
)

func testNamespace(name string, size int) Namespace {
	return Namespace{
		Metadata: NamespaceMetadata{Name: name, KeySize: size},
		Key:      MustFixedKeyCodec(size),
		Payload:  CanonicalPayloadCodec{},
	}
}

func TestRegistry(t *testing.T) {
	t.Parallel()
	registry, err := NewRegistry(
		testNamespace("utxo/v0", 34),
		testNamespace("blocks/v0", 36),
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	namespace, ok := registry.Lookup("utxo/v0")
	if !ok {
		t.Fatal("Lookup(utxo/v0) not found")
	}
	if namespace.Metadata.KeySize != 34 {
		t.Fatalf("Lookup(utxo/v0).KeySize = %d, want 34", namespace.Metadata.KeySize)
	}
	namespaces := registry.Namespaces()
	if len(namespaces) != 2 {
		t.Fatalf("Namespaces() length = %d, want 2", len(namespaces))
	}
	if namespaces[0].Metadata.Name != "blocks/v0" ||
		namespaces[1].Metadata.Name != "utxo/v0" {
		t.Fatalf("Namespaces() order = %q, %q", namespaces[0].Metadata.Name, namespaces[1].Metadata.Name)
	}

	err = registry.Register(testNamespace("utxo/v0", 34))
	if !errors.Is(err, ErrDuplicateNamespace) {
		t.Fatalf("Register(duplicate) error = %v, want ErrDuplicateNamespace", err)
	}
}

func TestNamespaceValidation(t *testing.T) {
	t.Parallel()
	tests := []Namespace{
		{},
		{
			Metadata: NamespaceMetadata{Name: "/utxo/v0", KeySize: 34},
			Key:      MustFixedKeyCodec(34),
			Payload:  CanonicalPayloadCodec{},
		},
		{
			Metadata: NamespaceMetadata{Name: "utxo/v0", KeySize: 33},
			Key:      MustFixedKeyCodec(34),
			Payload:  CanonicalPayloadCodec{},
		},
		{
			Metadata: NamespaceMetadata{Name: "utxo/v0", KeySize: 34},
			Key:      MustFixedKeyCodec(34),
		},
	}
	for i, namespace := range tests {
		if err := namespace.Validate(); !errors.Is(err, ErrInvalidNamespace) {
			t.Errorf("test %d: Validate() error = %v, want ErrInvalidNamespace", i, err)
		}
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	t.Parallel()
	registry := MustRegistry()
	const count = 32
	var wg sync.WaitGroup
	for i := range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := "namespace/" + strconv.Itoa(i)
			if err := registry.Register(testNamespace(name, 1)); err != nil {
				t.Errorf("Register(%q) error = %v", name, err)
			}
			registry.Lookup(name)
			registry.Namespaces()
		}()
	}
	wg.Wait()
	if got := len(registry.Namespaces()); got != count {
		t.Fatalf("Namespaces() length = %d, want %d", got, count)
	}
}
