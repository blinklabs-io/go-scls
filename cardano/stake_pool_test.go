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
	"reflect"
	"testing"

	"github.com/blinklabs-io/go-scls/cardano/cbor"
)

func TestStakePoolKeyCodec(t *testing.T) {
	t.Parallel()
	var want StakePoolKey
	fillBytes(want[:], 0x10)

	decoded, err := DecodeStakePoolKey(want[:])
	if err != nil {
		t.Fatalf("DecodeStakePoolKey() error = %v", err)
	}
	if decoded != want {
		t.Fatalf("DecodeStakePoolKey() = %x, want %x", decoded, want)
	}
	encoded := EncodeStakePoolKey(decoded)
	if !bytes.Equal(encoded, want[:]) {
		t.Fatalf("EncodeStakePoolKey() = %x, want %x", encoded, want)
	}

	codec := StakePoolKeyCodec{}
	value, err := codec.DecodeKey(want[:])
	if err != nil {
		t.Fatalf("DecodeKey() error = %v", err)
	}
	if value.(StakePoolKey) != want {
		t.Fatal("DecodeKey() returned a different key")
	}
	if _, err := codec.DecodeKey(make([]byte, 27)); !errors.Is(err, ErrKeySize) {
		t.Fatalf("DecodeKey(27 bytes) error = %v, want ErrKeySize", err)
	}
	if _, err := codec.DecodeKey(make([]byte, 29)); !errors.Is(err, ErrKeySize) {
		t.Fatalf("DecodeKey(29 bytes) error = %v, want ErrKeySize", err)
	}
}

func TestStakePoolRoundTripOptionalLeiosKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		stateLeios  bool
		paramsLeios bool
	}{
		{name: "neither", stateLeios: false, paramsLeios: false},
		{name: "state", stateLeios: true, paramsLeios: false},
		{name: "future params", stateLeios: false, paramsLeios: true},
		{name: "both", stateLeios: true, paramsLeios: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := sampleStakePool(test.stateLeios, test.paramsLeios)
			encoded, err := EncodeStakePool(value)
			if err != nil {
				t.Fatalf("EncodeStakePool() error = %v", err)
			}
			if err := cbor.Validate(encoded); err != nil {
				t.Fatalf("encoded payload is not canonical: %v", err)
			}
			decoded, err := DecodeStakePool(encoded)
			if err != nil {
				t.Fatalf("DecodeStakePool() error = %v", err)
			}
			if !reflect.DeepEqual(decoded, value) {
				t.Fatalf("DecodeStakePool() = %+v, want %+v", decoded, value)
			}
			if got := decoded.StakePoolState.LeiosKey != nil; got != test.stateLeios {
				t.Fatalf("decoded state Leios key presence = %v, want %v", got, test.stateLeios)
			}
			if got := decoded.FutureStakePoolParams.LeiosKey != nil; got != test.paramsLeios {
				t.Fatalf("decoded params Leios key presence = %v, want %v", got, test.paramsLeios)
			}
			reencoded, err := EncodeStakePool(decoded)
			if err != nil {
				t.Fatalf("EncodeStakePool(decoded) error = %v", err)
			}
			if !bytes.Equal(reencoded, encoded) {
				t.Fatalf("canonical re-encode differs:\n got %x\nwant %x", reencoded, encoded)
			}

			raw, err := cbor.Unmarshal(encoded)
			if err != nil {
				t.Fatalf("cbor.Unmarshal() error = %v", err)
			}
			top := raw.(cbor.Map)
			state := mapField(t, top, "stake_pool_state").(cbor.Map)
			wantStateFields := 10
			if test.stateLeios {
				wantStateFields++
			}
			if len(state) != wantStateFields {
				t.Fatalf("state map length = %d, want %d", len(state), wantStateFields)
			}
			params := mapField(t, top, "future_stake_pool_params").(cbor.Map)
			wantParamsFields := 9
			if test.paramsLeios {
				wantParamsFields++
			}
			if len(params) != wantParamsFields {
				t.Fatalf("params map length = %d, want %d", len(params), wantParamsFields)
			}
		})
	}
}

func TestStakePoolPayloadCodecAndRegistry(t *testing.T) {
	t.Parallel()
	value := sampleStakePool(true, true)
	codec := StakePoolPayloadCodec{}
	encoded, err := codec.EncodePayload(&value)
	if err != nil {
		t.Fatalf("EncodePayload() error = %v", err)
	}
	decoded, err := codec.DecodePayload(encoded)
	if err != nil {
		t.Fatalf("DecodePayload() error = %v", err)
	}
	if _, ok := decoded.(StakePool); !ok {
		t.Fatalf("DecodePayload() type = %T, want StakePool", decoded)
	}
	var nilStakePool *StakePool
	if _, err := codec.EncodePayload(nilStakePool); !errors.Is(err, ErrStakePoolPayload) {
		t.Fatalf("EncodePayload(nil *StakePool) error = %v, want ErrStakePoolPayload", err)
	}
	if _, err := codec.EncodePayload("invalid"); !errors.Is(err, ErrStakePoolPayload) {
		t.Fatalf("EncodePayload(string) error = %v, want ErrStakePoolPayload", err)
	}
	keyCodec := StakePoolKeyCodec{}
	var nilStakePoolKey *StakePoolKey
	if _, err := keyCodec.EncodeKey(nilStakePoolKey); !errors.Is(err, ErrKeyType) {
		t.Fatalf("EncodeKey(nil *StakePoolKey) error = %v, want ErrKeyType", err)
	}
	if _, err := keyCodec.EncodeKey("invalid"); !errors.Is(err, ErrKeyType) {
		t.Fatalf("EncodeKey(string) error = %v, want ErrKeyType", err)
	}
	namespace, ok := DefaultRegistry.Lookup(EntitiesStakePoolsV0Name)
	if !ok {
		t.Fatal("stake-pool namespace is not registered")
	}
	if namespace.Metadata.KeySize != stakePoolKeySize {
		t.Fatalf("registered key size = %d, want %d", namespace.Metadata.KeySize, stakePoolKeySize)
	}
}

func TestStakePoolRejectsInvalidLeiosKeySizes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		publicKey int
		proof     int
	}{
		{name: "short public key", publicKey: 95, proof: 48},
		{name: "long public key", publicKey: 97, proof: 48},
		{name: "short proof", publicKey: 96, proof: 47},
		{name: "long proof", publicKey: 96, proof: 49},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value, err := encodeStakePool(sampleStakePool(true, true))
			if err != nil {
				t.Fatalf("encodeStakePool() error = %v", err)
			}
			state := mapField(t, value, "stake_pool_state").(cbor.Map)
			setMapField(
				t,
				state,
				"leios_key",
				cbor.Array{make([]byte, test.publicKey), make([]byte, test.proof)},
			)
			encoded, err := cbor.Marshal(value)
			if err != nil {
				t.Fatalf("cbor.Marshal() error = %v", err)
			}
			if _, err := DecodeStakePool(encoded); !errors.Is(err, ErrStakePoolPayload) {
				t.Fatalf("DecodeStakePool() error = %v, want ErrStakePoolPayload", err)
			}
		})
	}
}

func TestStakePoolRejectsNonCanonicalSet(t *testing.T) {
	t.Parallel()
	value, err := encodeStakePool(sampleStakePool(false, false))
	if err != nil {
		t.Fatalf("encodeStakePool() error = %v", err)
	}
	state := mapField(t, value, "stake_pool_state").(cbor.Map)
	owners := mapField(t, state, "owners").(cbor.Tag)
	items := owners.Value.(cbor.Array)
	items[0], items[1] = items[1], items[0]
	encoded, err := cbor.Marshal(value)
	if err != nil {
		t.Fatalf("cbor.Marshal() error = %v", err)
	}
	if _, err := DecodeStakePool(encoded); !errors.Is(err, ErrStakePoolPayload) {
		t.Fatalf("DecodeStakePool() error = %v, want ErrStakePoolPayload", err)
	}
}

func TestStakePoolRejectsUnknownAndMissingFields(t *testing.T) {
	t.Parallel()
	value, err := encodeStakePool(sampleStakePool(false, false))
	if err != nil {
		t.Fatalf("encodeStakePool() error = %v", err)
	}
	state := mapField(t, value, "stake_pool_state").(cbor.Map)
	for i := range state {
		if state[i].Key == "vrf" {
			state[i].Key = "bad"
			break
		}
	}
	encoded, err := cbor.Marshal(value)
	if err != nil {
		t.Fatalf("cbor.Marshal() error = %v", err)
	}
	if _, err := DecodeStakePool(encoded); !errors.Is(err, ErrStakePoolPayload) {
		t.Fatalf("DecodeStakePool() error = %v, want ErrStakePoolPayload", err)
	}
}

func TestStakePoolRejectsInvalidUnitInterval(t *testing.T) {
	t.Parallel()
	value := sampleStakePool(false, false)
	value.StakePoolState.Margin = UnitInterval{Numerator: 2, Denominator: 1}
	if _, err := EncodeStakePool(value); !errors.Is(err, ErrStakePoolPayload) {
		t.Fatalf("EncodeStakePool() error = %v, want ErrStakePoolPayload", err)
	}
}

func sampleStakePool(stateLeios, paramsLeios bool) StakePool {
	var vrf Hash32
	fillBytes(vrf[:], 0x20)
	var owner1, owner2 Hash28
	fillBytes(owner1[:], 0x30)
	fillBytes(owner2[:], 0x40)
	var accountHash Hash28
	fillBytes(accountHash[:], 0x50)
	var delegatorHash Hash28
	fillBytes(delegatorHash[:], 0x60)
	var id Hash28
	fillBytes(id[:], 0x70)
	ipv4 := [4]byte{127, 0, 0, 1}
	port := uint16(3001)
	epoch := uint64(500)

	state := &StakePoolState{
		VRF:      vrf,
		Cost:     340,
		Margin:   UnitInterval{Numerator: 1, Denominator: 20},
		Owners:   []Hash28{owner1, owner2},
		Pledge:   1_000_000,
		Relays:   []Relay{{Kind: RelaySingleHostAddress, Port: &port, IPv4: &ipv4}},
		Deposit:  500,
		Metadata: &PoolMetadata{URL: "https://pool.example", Hash: []byte{1, 2, 3}},
		AccountID: Credential{
			Kind: CredentialKeyHash,
			Hash: accountHash,
		},
		Delegators: []Credential{{
			Kind: CredentialScriptHash,
			Hash: delegatorHash,
		}},
	}
	params := &StakePoolParams{
		ID:             id,
		VRF:            vrf,
		Cost:           350,
		Margin:         UnitInterval{Numerator: 1, Denominator: 10},
		Owners:         []Hash28{owner1, owner2},
		Pledge:         2_000_000,
		Relays:         []Relay{{Kind: RelayMultiHostName, DNSName: "_cardano._tcp.pool.example"}},
		Metadata:       nil,
		AccountAddress: []byte{0xe1, 1, 2, 3},
	}
	if stateLeios {
		state.LeiosKey = sampleLeiosKey()
	}
	if paramsLeios {
		params.LeiosKey = sampleLeiosKey()
	}
	return StakePool{
		StakePoolState:        state,
		RetiringEpochNo:       &epoch,
		FutureStakePoolParams: params,
	}
}

func sampleLeiosKey() *LeiosKey {
	result := &LeiosKey{}
	fillBytes(result.PublicKey[:], 0x80)
	fillBytes(result.ProofOfPossession[:], 0xe0)
	return result
}

func fillBytes(value []byte, start byte) {
	for i := range value {
		value[i] = start + byte(i)
	}
}

func mapField(t *testing.T, value cbor.Map, name string) any {
	t.Helper()
	for _, entry := range value {
		if reflect.DeepEqual(entry.Key, name) {
			return entry.Value
		}
	}
	t.Fatalf("map field %q not found", name)
	return nil
}

func setMapField(t *testing.T, value cbor.Map, name string, fieldValue any) {
	t.Helper()
	for i := range value {
		if reflect.DeepEqual(value[i].Key, name) {
			value[i].Value = fieldValue
			return
		}
	}
	t.Fatalf("map field %q not found", name)
}
