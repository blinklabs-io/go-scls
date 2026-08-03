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

func TestCanonicalNamespacesV0(t *testing.T) {
	t.Parallel()
	want := map[string]int{
		BlocksV0Name:                         36,
		UTxOV0Name:                           34,
		EntitiesAccountsV0Name:               29,
		EntitiesCommitteeV0Name:              1,
		EntitiesDRepsV0Name:                  29,
		EntitiesStakePoolsVRFKeyHashesV0Name: 32,
		GovCommitteeV0Name:                   1,
		GovConstitutionV0Name:                1,
		GovPParamsV0Name:                     4,
		GovProposalsV0Name:                   34,
		GovProposalsRootsV0Name:              1,
		NoncesV0Name:                         1,
		SnapshotsMarkV0Name:                  31,
		SnapshotsSetV0Name:                   31,
		SnapshotsGoV0Name:                    31,
	}
	if len(CanonicalNamespacesV0) != len(want) {
		t.Fatalf("CanonicalNamespacesV0 length = %d, want %d", len(CanonicalNamespacesV0), len(want))
	}

	for _, namespace := range CanonicalNamespacesV0 {
		namespace := namespace
		t.Run(namespace.Metadata.Name, func(t *testing.T) {
			t.Parallel()
			keySize, ok := want[namespace.Metadata.Name]
			if !ok {
				t.Fatalf("unexpected namespace %q", namespace.Metadata.Name)
			}
			if namespace.Metadata.KeySize != keySize {
				t.Fatalf("key size = %d, want %d", namespace.Metadata.KeySize, keySize)
			}
			if err := namespace.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			registered, ok := DefaultRegistry.Lookup(namespace.Metadata.Name)
			if !ok {
				t.Fatal("namespace is not registered")
			}
			if registered.Metadata != namespace.Metadata {
				t.Fatalf("registered metadata = %#v, want %#v", registered.Metadata, namespace.Metadata)
			}
		})
	}
}

func TestCanonicalNamespacesValidateKeys(t *testing.T) {
	t.Parallel()
	for _, namespace := range CanonicalNamespacesV0 {
		namespace := namespace
		t.Run(namespace.Metadata.Name, func(t *testing.T) {
			t.Parallel()
			key := make([]byte, namespace.Metadata.KeySize)
			if namespace.Metadata.Name == GovPParamsV0Name {
				copy(key, "prev")
			}
			for i := range key {
				if namespace.Metadata.Name != GovPParamsV0Name {
					key[i] = byte(i)
				}
			}
			decoded, err := namespace.Key.DecodeKey(key)
			if err != nil {
				t.Fatalf("DecodeKey() error = %v", err)
			}
			encoded, err := namespace.Key.EncodeKey(decoded)
			if err != nil {
				t.Fatalf("EncodeKey() error = %v", err)
			}
			if !bytes.Equal(encoded, key) {
				t.Fatalf("key round trip = %x, want %x", encoded, key)
			}
			if _, err := namespace.Key.DecodeKey(key[:len(key)-1]); !errors.Is(err, ErrKeySize) {
				t.Fatalf("DecodeKey(short) error = %v, want ErrKeySize", err)
			}
			if _, err := namespace.Key.DecodeKey(append(key, 0)); !errors.Is(err, ErrKeySize) {
				t.Fatalf("DecodeKey(long) error = %v, want ErrKeySize", err)
			}
		})
	}
}

func TestStructuredKeyCodecs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		codec KeyCodec
		value any
		want  []byte
	}{
		{
			name:  "blocks",
			codec: BlocksKeyCodec{},
			value: BlocksKey{Epoch: 0x0102030405060708},
			want: append(
				make([]byte, 28),
				1, 2, 3, 4, 5, 6, 7, 8,
			),
		},
		{
			name:  "utxo",
			codec: UTxOKeyCodec{},
			value: UTxOKey{OutputIndex: 0x0102},
			want:  append(make([]byte, 32), 1, 2),
		},
		{
			name:  "credential",
			codec: CredentialKeyCodec{},
			value: CredentialKey{Kind: CredentialKeyKindScript},
			want:  append([]byte{1}, make([]byte, 28)...),
		},
		{
			name:  "singleton",
			codec: SingletonKeyCodec{},
			value: SingletonKey{},
			want:  []byte{0},
		},
		{
			name:  "vrf hash",
			codec: VRFKeyHashKeyCodec{},
			value: VRFKeyHashKey{},
			want:  make([]byte, 32),
		},
		{
			name:  "pparams",
			codec: GovPParamsKeyCodec{},
			value: GovPParamsPossibleFuture,
			want:  []byte("pfut"),
		},
		{
			name:  "proposal",
			codec: GovProposalKeyCodec{},
			value: GovProposalKey{ActionIndex: 0x0102},
			want:  append(make([]byte, 32), 1, 2),
		},
		{
			name:  "proposal root",
			codec: GovProposalRootKeyCodec{},
			value: GovProposalRootConstitution,
			want:  []byte{3},
		},
		{
			name:  "snapshot",
			codec: SnapshotKeyCodec{},
			value: SnapshotKey{},
			want:  make([]byte, 31),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			encoded, err := test.codec.EncodeKey(test.value)
			if err != nil {
				t.Fatalf("EncodeKey() error = %v", err)
			}
			if !bytes.Equal(encoded, test.want) {
				t.Fatalf("EncodeKey() = %x, want %x", encoded, test.want)
			}
			decoded, err := test.codec.DecodeKey(encoded)
			if err != nil {
				t.Fatalf("DecodeKey() error = %v", err)
			}
			reencoded, err := test.codec.EncodeKey(decoded)
			if err != nil {
				t.Fatalf("EncodeKey(decoded) error = %v", err)
			}
			if !bytes.Equal(reencoded, encoded) {
				t.Fatalf("key re-encode = %x, want %x", reencoded, encoded)
			}
		})
	}
}

func TestStructuredKeyCodecsRejectInvalidTags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		codec KeyCodec
		key   []byte
	}{
		{name: "credential", codec: CredentialKeyCodec{}, key: append([]byte{2}, make([]byte, 28)...)},
		{name: "singleton", codec: SingletonKeyCodec{}, key: []byte{1}},
		{name: "pparams", codec: GovPParamsKeyCodec{}, key: []byte("fut0")},
		{name: "proposal root", codec: GovProposalRootKeyCodec{}, key: []byte{4}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := test.codec.DecodeKey(test.key); !errors.Is(err, ErrKeyType) {
				t.Fatalf("DecodeKey() error = %v, want ErrKeyType", err)
			}
		})
	}
}

func TestCanonicalNamespacesValidatePayloads(t *testing.T) {
	t.Parallel()
	value := cbor.Map{
		{Key: uint64(0), Value: cbor.Array{uint64(1), []byte{2, 3}}},
	}
	for _, namespace := range CanonicalNamespacesV0 {
		namespace := namespace
		t.Run(namespace.Metadata.Name, func(t *testing.T) {
			t.Parallel()
			encoded, err := namespace.Payload.EncodePayload(value)
			if err != nil {
				t.Fatalf("EncodePayload() error = %v", err)
			}
			decoded, err := namespace.Payload.DecodePayload(encoded)
			if err != nil {
				t.Fatalf("DecodePayload() error = %v", err)
			}
			reencoded, err := namespace.Payload.EncodePayload(decoded)
			if err != nil {
				t.Fatalf("EncodePayload(decoded) error = %v", err)
			}
			if !bytes.Equal(reencoded, encoded) {
				t.Fatalf("canonical re-encode = %x, want %x", reencoded, encoded)
			}

			nonCanonical := []byte{0x18, 0x00}
			_, err = namespace.Payload.DecodePayload(nonCanonical)
			if !errors.Is(err, cbor.ErrNonMinimal) {
				t.Fatalf("DecodePayload(non-canonical) error = %v, want ErrNonMinimal", err)
			}
		})
	}
}
