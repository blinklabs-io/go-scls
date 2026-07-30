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
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

var (
	// ErrInvalidNamespace indicates invalid namespace metadata or codecs.
	ErrInvalidNamespace = errors.New("invalid Cardano namespace")
	// ErrDuplicateNamespace indicates a repeated namespace name.
	ErrDuplicateNamespace = errors.New("duplicate Cardano namespace")
)

// NamespaceMetadata describes the stable wire-level identity of a namespace.
type NamespaceMetadata struct {
	Name    string
	KeySize int
}

// Namespace associates public metadata with key and payload codecs.
type Namespace struct {
	Metadata NamespaceMetadata
	Key      KeyCodec
	Payload  PayloadCodec
}

// Validate checks namespace metadata and codec consistency.
func (n Namespace) Validate() error {
	if !validNamespaceName(n.Metadata.Name) {
		return fmt.Errorf("%w: invalid name %q", ErrInvalidNamespace, n.Metadata.Name)
	}
	if n.Metadata.KeySize <= 0 {
		return fmt.Errorf("%w: invalid key size %d", ErrInvalidNamespace, n.Metadata.KeySize)
	}
	if n.Key == nil {
		return fmt.Errorf("%w: nil key codec", ErrInvalidNamespace)
	}
	if n.Key.Size() != n.Metadata.KeySize {
		return fmt.Errorf(
			"%w: metadata key size %d differs from codec size %d",
			ErrInvalidNamespace,
			n.Metadata.KeySize,
			n.Key.Size(),
		)
	}
	if n.Payload == nil {
		return fmt.Errorf("%w: nil payload codec", ErrInvalidNamespace)
	}
	return nil
}

func validNamespaceName(name string) bool {
	if name == "" || !utf8.ValidString(name) || strings.HasPrefix(name, "/") ||
		strings.HasSuffix(name, "/") || strings.Contains(name, "//") {
		return false
	}
	for _, r := range name {
		if r <= 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// Registry is a concurrency-safe collection keyed by canonical namespace
// name.
type Registry struct {
	mu         sync.RWMutex
	namespaces map[string]Namespace
}

// NewRegistry creates a registry containing namespaces.
func NewRegistry(namespaces ...Namespace) (*Registry, error) {
	registry := &Registry{namespaces: make(map[string]Namespace, len(namespaces))}
	for _, namespace := range namespaces {
		if err := registry.Register(namespace); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

// MustRegistry creates a registry and panics if any namespace is invalid.
func MustRegistry(namespaces ...Namespace) *Registry {
	registry, err := NewRegistry(namespaces...)
	if err != nil {
		panic(err)
	}
	return registry
}

// Register validates and adds a namespace.
func (r *Registry) Register(namespace Namespace) error {
	if r == nil {
		return fmt.Errorf("%w: nil registry", ErrInvalidNamespace)
	}
	if err := namespace.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.namespaces == nil {
		r.namespaces = make(map[string]Namespace)
	}
	if _, ok := r.namespaces[namespace.Metadata.Name]; ok {
		return fmt.Errorf("%w: %s", ErrDuplicateNamespace, namespace.Metadata.Name)
	}
	r.namespaces[namespace.Metadata.Name] = namespace
	return nil
}

// MustRegister adds a namespace and panics if registration fails.
func (r *Registry) MustRegister(namespace Namespace) {
	if err := r.Register(namespace); err != nil {
		panic(err)
	}
}

// Lookup returns a registered namespace by name.
func (r *Registry) Lookup(name string) (Namespace, bool) {
	if r == nil {
		return Namespace{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	namespace, ok := r.namespaces[name]
	return namespace, ok
}

// Namespaces returns a name-sorted snapshot of all registered namespaces.
func (r *Registry) Namespaces() []Namespace {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Namespace, 0, len(r.namespaces))
	for _, namespace := range r.namespaces {
		result = append(result, namespace)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Metadata.Name < result[j].Metadata.Name
	})
	return result
}

// DefaultRegistry is populated by the namespace codec implementations.
var DefaultRegistry = MustRegistry()
