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
	"fmt"
	"sort"

	"github.com/blinklabs-io/go-scls/cardano/cbor"
)

const (
	// EntitiesStakePoolsV0Name is the canonical stake-pool namespace name.
	EntitiesStakePoolsV0Name = "entities/stake_pools/v0"

	stakePoolKeySize    = 28
	hash28Size          = 28
	vrfHashSize         = 32
	leiosPublicKeySize  = 96
	leiosProofSize      = 48
	maxPoolMetadataURL  = 128
	maxRelayDNSNameSize = 128
)

var (
	// ErrStakePoolPayload indicates a payload that does not match the
	// entities/stake_pools/v0 schema.
	ErrStakePoolPayload = errors.New("invalid entities/stake_pools/v0 payload")

	// EntitiesStakePoolsV0 is the registered namespace codec.
	EntitiesStakePoolsV0 = Namespace{
		Metadata: NamespaceMetadata{
			Name:    EntitiesStakePoolsV0Name,
			KeySize: stakePoolKeySize,
		},
		Key:     StakePoolKeyCodec{},
		Payload: StakePoolPayloadCodec{},
	}
)

func init() {
	DefaultRegistry.MustRegister(EntitiesStakePoolsV0)
}

// StakePoolKey is a raw 28-byte stake-pool key hash.
type StakePoolKey [stakePoolKeySize]byte

// Hash28 is a dependency-neutral 28-byte Cardano hash.
type Hash28 [hash28Size]byte

// Hash32 is a dependency-neutral 32-byte Cardano hash.
type Hash32 [vrfHashSize]byte

// BLSPublicKey is a 96-byte BLS12-381 public key.
type BLSPublicKey [leiosPublicKeySize]byte

// BLSProofOfPossession is a 48-byte BLS12-381 proof of possession.
type BLSProofOfPossession [leiosProofSize]byte

// LeiosKey contains a BLS public key and its proof of possession.
type LeiosKey struct {
	PublicKey         BLSPublicKey
	ProofOfPossession BLSProofOfPossession
}

// CredentialKind identifies a key-hash or script-hash credential.
type CredentialKind uint8

const (
	// CredentialKeyHash identifies a key-hash credential.
	CredentialKeyHash CredentialKind = iota
	// CredentialScriptHash identifies a script-hash credential.
	CredentialScriptHash
)

// Credential is a dependency-neutral staking credential.
type Credential struct {
	Kind CredentialKind
	Hash Hash28
}

// UnitInterval is a non-negative rational number no greater than one.
type UnitInterval struct {
	Numerator   uint64
	Denominator uint64
}

// PoolMetadata is a stake-pool metadata reference.
type PoolMetadata struct {
	URL  string
	Hash []byte
}

// RelayKind identifies a stake-pool relay variant.
type RelayKind uint8

const (
	// RelaySingleHostAddress contains optional IP addresses.
	RelaySingleHostAddress RelayKind = iota
	// RelaySingleHostName contains an A or AAAA DNS name.
	RelaySingleHostName
	// RelayMultiHostName contains an SRV DNS name.
	RelayMultiHostName
)

// Relay is a dependency-neutral stake-pool relay.
type Relay struct {
	Kind    RelayKind
	Port    *uint16
	IPv4    *[4]byte
	IPv6    *[16]byte
	DNSName string
}

// StakePool is the entities/stake_pools/v0 payload.
type StakePool struct {
	StakePoolState        *StakePoolState
	RetiringEpochNo       *uint64
	FutureStakePoolParams *StakePoolParams
}

// StakePoolState is the current canonical stake-pool state.
type StakePoolState struct {
	VRF        Hash32
	Cost       uint64
	Margin     UnitInterval
	Owners     []Hash28
	Pledge     uint64
	Relays     []Relay
	Deposit    uint64
	Metadata   *PoolMetadata
	LeiosKey   *LeiosKey
	AccountID  Credential
	Delegators []Credential
}

// StakePoolParams is a future canonical stake-pool parameter set.
type StakePoolParams struct {
	ID             Hash28
	VRF            Hash32
	Cost           uint64
	Margin         UnitInterval
	Owners         []Hash28
	Pledge         uint64
	Relays         []Relay
	Metadata       *PoolMetadata
	LeiosKey       *LeiosKey
	AccountAddress []byte
}

// StakePoolKeyCodec encodes typed stake-pool keys.
type StakePoolKeyCodec struct{}

// Size returns the fixed 28-byte key size.
func (StakePoolKeyCodec) Size() int {
	return stakePoolKeySize
}

// DecodeKey decodes a raw stake-pool key.
func (StakePoolKeyCodec) DecodeKey(data []byte) (any, error) {
	return DecodeStakePoolKey(data)
}

// EncodeKey encodes a typed stake-pool key.
func (StakePoolKeyCodec) EncodeKey(value any) ([]byte, error) {
	switch value := value.(type) {
	case StakePoolKey:
		return EncodeStakePoolKey(value), nil
	case *StakePoolKey:
		if value == nil {
			return nil, fmt.Errorf("%w: nil *StakePoolKey", ErrKeyType)
		}
		return EncodeStakePoolKey(*value), nil
	default:
		return nil, fmt.Errorf("%w: %T", ErrKeyType, value)
	}
}

// DecodeStakePoolKey validates and decodes a 28-byte pool key hash.
func DecodeStakePoolKey(data []byte) (StakePoolKey, error) {
	var key StakePoolKey
	if len(data) != len(key) {
		return key, fmt.Errorf("%w: got %d, want %d", ErrKeySize, len(data), len(key))
	}
	copy(key[:], data)
	return key, nil
}

// EncodeStakePoolKey returns a copy of the raw key bytes.
func EncodeStakePoolKey(key StakePoolKey) []byte {
	return bytes.Clone(key[:])
}

// StakePoolPayloadCodec encodes typed stake-pool payloads.
type StakePoolPayloadCodec struct{}

// DecodePayload decodes an entities/stake_pools/v0 payload.
func (StakePoolPayloadCodec) DecodePayload(data []byte) (any, error) {
	return DecodeStakePool(data)
}

// EncodePayload encodes an entities/stake_pools/v0 payload.
func (StakePoolPayloadCodec) EncodePayload(value any) ([]byte, error) {
	switch value := value.(type) {
	case StakePool:
		return EncodeStakePool(value)
	case *StakePool:
		if value == nil {
			return nil, fmt.Errorf("%w: nil *StakePool", ErrStakePoolPayload)
		}
		return EncodeStakePool(*value)
	default:
		return nil, fmt.Errorf("%w: Go type %T", ErrStakePoolPayload, value)
	}
}

// DecodeStakePool decodes and validates an entities/stake_pools/v0 payload.
func DecodeStakePool(data []byte) (StakePool, error) {
	value, err := cbor.Unmarshal(data)
	if err != nil {
		return StakePool{}, fmt.Errorf("%w: %w", ErrStakePoolPayload, err)
	}
	result, err := decodeStakePool(value)
	if err != nil {
		return StakePool{}, fmt.Errorf("%w: %w", ErrStakePoolPayload, err)
	}
	return result, nil
}

// EncodeStakePool validates and encodes an entities/stake_pools/v0 payload.
func EncodeStakePool(value StakePool) ([]byte, error) {
	encoded, err := encodeStakePool(value)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrStakePoolPayload, err)
	}
	result, err := cbor.Marshal(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrStakePoolPayload, err)
	}
	return result, nil
}

func decodeStakePool(value any) (StakePool, error) {
	fields, err := decodeFields(
		value,
		"stake pool",
		3,
		3,
		"stake_pool_state",
		"retiring_epoch_no",
		"future_stake_pool_params",
	)
	if err != nil {
		return StakePool{}, err
	}
	stateValue, err := requiredField(fields, "stake_pool_state")
	if err != nil {
		return StakePool{}, err
	}
	retiringValue, err := requiredField(fields, "retiring_epoch_no")
	if err != nil {
		return StakePool{}, err
	}
	futureValue, err := requiredField(fields, "future_stake_pool_params")
	if err != nil {
		return StakePool{}, err
	}

	var result StakePool
	if stateValue != nil {
		state, err := decodeStakePoolState(stateValue)
		if err != nil {
			return StakePool{}, fmt.Errorf("stake_pool_state: %w", err)
		}
		result.StakePoolState = &state
	}
	if retiringValue != nil {
		epoch, err := decodeUint(retiringValue)
		if err != nil {
			return StakePool{}, fmt.Errorf("retiring_epoch_no: %w", err)
		}
		result.RetiringEpochNo = &epoch
	}
	if futureValue != nil {
		params, err := decodeStakePoolParams(futureValue)
		if err != nil {
			return StakePool{}, fmt.Errorf("future_stake_pool_params: %w", err)
		}
		result.FutureStakePoolParams = &params
	}
	return result, nil
}

func encodeStakePool(value StakePool) (cbor.Map, error) {
	var state any
	if value.StakePoolState != nil {
		encoded, err := encodeStakePoolState(*value.StakePoolState)
		if err != nil {
			return nil, fmt.Errorf("stake_pool_state: %w", err)
		}
		state = encoded
	}
	var future any
	if value.FutureStakePoolParams != nil {
		encoded, err := encodeStakePoolParams(*value.FutureStakePoolParams)
		if err != nil {
			return nil, fmt.Errorf("future_stake_pool_params: %w", err)
		}
		future = encoded
	}
	var retiring any
	if value.RetiringEpochNo != nil {
		retiring = *value.RetiringEpochNo
	}
	return cbor.Map{
		{Key: "stake_pool_state", Value: state},
		{Key: "retiring_epoch_no", Value: retiring},
		{Key: "future_stake_pool_params", Value: future},
	}, nil
}

func decodeStakePoolState(value any) (StakePoolState, error) {
	fields, err := decodeFields(
		value,
		"stake_pool_state",
		10,
		11,
		"vrf",
		"cost",
		"margin",
		"owners",
		"pledge",
		"relays",
		"deposit",
		"metadata",
		"leios_key",
		"account_id",
		"delegators",
	)
	if err != nil {
		return StakePoolState{}, err
	}
	var result StakePoolState
	if result.VRF, err = decodeHash32Field(fields, "vrf"); err != nil {
		return StakePoolState{}, err
	}
	if result.Cost, err = decodeUintField(fields, "cost"); err != nil {
		return StakePoolState{}, err
	}
	if result.Margin, err = decodeUnitIntervalField(fields, "margin"); err != nil {
		return StakePoolState{}, err
	}
	if result.Owners, err = decodeHash28SetField(fields, "owners"); err != nil {
		return StakePoolState{}, err
	}
	if result.Pledge, err = decodeUintField(fields, "pledge"); err != nil {
		return StakePoolState{}, err
	}
	if result.Relays, err = decodeRelaysField(fields, "relays"); err != nil {
		return StakePoolState{}, err
	}
	if result.Deposit, err = decodeUintField(fields, "deposit"); err != nil {
		return StakePoolState{}, err
	}
	if result.Metadata, err = decodeMetadataField(fields, "metadata"); err != nil {
		return StakePoolState{}, err
	}
	if leios, ok := fields["leios_key"]; ok {
		if leios == nil {
			return StakePoolState{}, errors.New("leios_key must not be null when present")
		}
		decoded, err := decodeLeiosKey(leios)
		if err != nil {
			return StakePoolState{}, fmt.Errorf("leios_key: %w", err)
		}
		result.LeiosKey = &decoded
	}
	accountID, err := requiredField(fields, "account_id")
	if err != nil {
		return StakePoolState{}, err
	}
	if result.AccountID, err = decodeCredential(accountID); err != nil {
		return StakePoolState{}, fmt.Errorf("account_id: %w", err)
	}
	if result.Delegators, err = decodeCredentialSetField(fields, "delegators"); err != nil {
		return StakePoolState{}, err
	}
	return result, nil
}

func encodeStakePoolState(value StakePoolState) (cbor.Map, error) {
	margin, err := encodeUnitInterval(value.Margin)
	if err != nil {
		return nil, fmt.Errorf("margin: %w", err)
	}
	owners, err := encodeHash28Set(value.Owners)
	if err != nil {
		return nil, fmt.Errorf("owners: %w", err)
	}
	relays, err := encodeRelays(value.Relays)
	if err != nil {
		return nil, fmt.Errorf("relays: %w", err)
	}
	metadata, err := encodeMetadata(value.Metadata)
	if err != nil {
		return nil, fmt.Errorf("metadata: %w", err)
	}
	accountID, err := encodeCredential(value.AccountID)
	if err != nil {
		return nil, fmt.Errorf("account_id: %w", err)
	}
	delegators, err := encodeCredentialSet(value.Delegators)
	if err != nil {
		return nil, fmt.Errorf("delegators: %w", err)
	}
	result := cbor.Map{
		{Key: "vrf", Value: bytes.Clone(value.VRF[:])},
		{Key: "cost", Value: value.Cost},
		{Key: "margin", Value: margin},
		{Key: "owners", Value: owners},
		{Key: "pledge", Value: value.Pledge},
		{Key: "relays", Value: relays},
		{Key: "deposit", Value: value.Deposit},
		{Key: "metadata", Value: metadata},
		{Key: "account_id", Value: accountID},
		{Key: "delegators", Value: delegators},
	}
	if value.LeiosKey != nil {
		result = append(result, cbor.MapEntry{
			Key:   "leios_key",
			Value: encodeLeiosKey(*value.LeiosKey),
		})
	}
	return result, nil
}

func decodeStakePoolParams(value any) (StakePoolParams, error) {
	fields, err := decodeFields(
		value,
		"stake_pool_params",
		9,
		10,
		"id",
		"vrf",
		"cost",
		"margin",
		"owners",
		"pledge",
		"relays",
		"metadata",
		"leios_key",
		"account_address",
	)
	if err != nil {
		return StakePoolParams{}, err
	}
	var result StakePoolParams
	if result.ID, err = decodeHash28Field(fields, "id"); err != nil {
		return StakePoolParams{}, err
	}
	if result.VRF, err = decodeHash32Field(fields, "vrf"); err != nil {
		return StakePoolParams{}, err
	}
	if result.Cost, err = decodeUintField(fields, "cost"); err != nil {
		return StakePoolParams{}, err
	}
	if result.Margin, err = decodeUnitIntervalField(fields, "margin"); err != nil {
		return StakePoolParams{}, err
	}
	if result.Owners, err = decodeHash28SetField(fields, "owners"); err != nil {
		return StakePoolParams{}, err
	}
	if result.Pledge, err = decodeUintField(fields, "pledge"); err != nil {
		return StakePoolParams{}, err
	}
	if result.Relays, err = decodeRelaysField(fields, "relays"); err != nil {
		return StakePoolParams{}, err
	}
	if result.Metadata, err = decodeMetadataField(fields, "metadata"); err != nil {
		return StakePoolParams{}, err
	}
	if leios, ok := fields["leios_key"]; ok {
		if leios == nil {
			return StakePoolParams{}, errors.New("leios_key must not be null when present")
		}
		decoded, err := decodeLeiosKey(leios)
		if err != nil {
			return StakePoolParams{}, fmt.Errorf("leios_key: %w", err)
		}
		result.LeiosKey = &decoded
	}
	address, err := requiredField(fields, "account_address")
	if err != nil {
		return StakePoolParams{}, err
	}
	result.AccountAddress, err = decodeBytes(address, -1)
	if err != nil {
		return StakePoolParams{}, fmt.Errorf("account_address: %w", err)
	}
	return result, nil
}

func encodeStakePoolParams(value StakePoolParams) (cbor.Map, error) {
	margin, err := encodeUnitInterval(value.Margin)
	if err != nil {
		return nil, fmt.Errorf("margin: %w", err)
	}
	owners, err := encodeHash28Set(value.Owners)
	if err != nil {
		return nil, fmt.Errorf("owners: %w", err)
	}
	relays, err := encodeRelays(value.Relays)
	if err != nil {
		return nil, fmt.Errorf("relays: %w", err)
	}
	metadata, err := encodeMetadata(value.Metadata)
	if err != nil {
		return nil, fmt.Errorf("metadata: %w", err)
	}
	result := cbor.Map{
		{Key: "id", Value: bytes.Clone(value.ID[:])},
		{Key: "vrf", Value: bytes.Clone(value.VRF[:])},
		{Key: "cost", Value: value.Cost},
		{Key: "margin", Value: margin},
		{Key: "owners", Value: owners},
		{Key: "pledge", Value: value.Pledge},
		{Key: "relays", Value: relays},
		{Key: "metadata", Value: metadata},
		{Key: "account_address", Value: bytes.Clone(value.AccountAddress)},
	}
	if value.LeiosKey != nil {
		result = append(result, cbor.MapEntry{
			Key:   "leios_key",
			Value: encodeLeiosKey(*value.LeiosKey),
		})
	}
	return result, nil
}

func decodeFields(
	value any,
	name string,
	minLength,
	maxLength int,
	allowedNames ...string,
) (map[string]any, error) {
	valueMap, ok := value.(cbor.Map)
	if !ok {
		return nil, fmt.Errorf("%s must be a map", name)
	}
	if len(valueMap) < minLength || len(valueMap) > maxLength {
		return nil, fmt.Errorf("%s map has %d fields, want %d..%d", name, len(valueMap), minLength, maxLength)
	}
	allowed := make(map[string]struct{}, len(allowedNames))
	for _, allowedName := range allowedNames {
		allowed[allowedName] = struct{}{}
	}
	result := make(map[string]any, len(valueMap))
	for _, entry := range valueMap {
		key, ok := entry.Key.(string)
		if !ok {
			return nil, fmt.Errorf("%s field name must be text", name)
		}
		if _, ok := allowed[key]; !ok {
			return nil, fmt.Errorf("%s contains unknown field %q", name, key)
		}
		result[key] = entry.Value
	}
	return result, nil
}

func requiredField(fields map[string]any, name string) (any, error) {
	value, ok := fields[name]
	if !ok {
		return nil, fmt.Errorf("missing field %q", name)
	}
	return value, nil
}

func decodeUintField(fields map[string]any, name string) (uint64, error) {
	value, err := requiredField(fields, name)
	if err != nil {
		return 0, err
	}
	result, err := decodeUint(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return result, nil
}

func decodeUint(value any) (uint64, error) {
	result, ok := value.(uint64)
	if !ok {
		return 0, errors.New("must be an unsigned integer")
	}
	return result, nil
}

func decodeHash28Field(fields map[string]any, name string) (Hash28, error) {
	value, err := requiredField(fields, name)
	if err != nil {
		return Hash28{}, err
	}
	result, err := decodeHash28(value)
	if err != nil {
		return Hash28{}, fmt.Errorf("%s: %w", name, err)
	}
	return result, nil
}

func decodeHash28(value any) (Hash28, error) {
	var result Hash28
	data, err := decodeBytes(value, len(result))
	if err != nil {
		return result, err
	}
	copy(result[:], data)
	return result, nil
}

func decodeHash32Field(fields map[string]any, name string) (Hash32, error) {
	value, err := requiredField(fields, name)
	if err != nil {
		return Hash32{}, err
	}
	var result Hash32
	data, err := decodeBytes(value, len(result))
	if err != nil {
		return result, fmt.Errorf("%s: %w", name, err)
	}
	copy(result[:], data)
	return result, nil
}

func decodeBytes(value any, size int) ([]byte, error) {
	data, ok := value.([]byte)
	if !ok {
		return nil, errors.New("must be a byte string")
	}
	if size >= 0 && len(data) != size {
		return nil, fmt.Errorf("byte string has size %d, want %d", len(data), size)
	}
	return bytes.Clone(data), nil
}

func decodeUnitIntervalField(fields map[string]any, name string) (UnitInterval, error) {
	value, err := requiredField(fields, name)
	if err != nil {
		return UnitInterval{}, err
	}
	result, err := decodeUnitInterval(value)
	if err != nil {
		return UnitInterval{}, fmt.Errorf("%s: %w", name, err)
	}
	return result, nil
}

func decodeUnitInterval(value any) (UnitInterval, error) {
	tag, ok := value.(cbor.Tag)
	if !ok || tag.Number != 30 {
		return UnitInterval{}, errors.New("must be tag 30")
	}
	items, err := decodePair(tag.Value)
	if err != nil {
		return UnitInterval{}, err
	}
	numerator, err := decodeUint(items[0])
	if err != nil {
		return UnitInterval{}, fmt.Errorf("numerator: %w", err)
	}
	denominator, err := decodeUint(items[1])
	if err != nil {
		return UnitInterval{}, fmt.Errorf("denominator: %w", err)
	}
	result := UnitInterval{Numerator: numerator, Denominator: denominator}
	if _, err := encodeUnitInterval(result); err != nil {
		return UnitInterval{}, err
	}
	return result, nil
}

func encodeUnitInterval(value UnitInterval) (cbor.Tag, error) {
	if value.Denominator == 0 {
		return cbor.Tag{}, errors.New("denominator must be greater than zero")
	}
	if value.Numerator > value.Denominator {
		return cbor.Tag{}, errors.New("numerator must not exceed denominator")
	}
	return cbor.Tag{
		Number: 30,
		Value:  cbor.Array{value.Numerator, value.Denominator},
	}, nil
}

func decodeHash28SetField(fields map[string]any, name string) ([]Hash28, error) {
	value, err := requiredField(fields, name)
	if err != nil {
		return nil, err
	}
	items, err := decodeSet(value)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	result := make([]Hash28, 0, len(items))
	for i, item := range items {
		hash, err := decodeHash28(item)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", name, i, err)
		}
		result = append(result, hash)
	}
	return result, nil
}

func encodeHash28Set(values []Hash28) (cbor.Tag, error) {
	items := make([]any, 0, len(values))
	for _, value := range values {
		items = append(items, bytes.Clone(value[:]))
	}
	return encodeSet(items)
}

func decodeCredentialSetField(fields map[string]any, name string) ([]Credential, error) {
	value, err := requiredField(fields, name)
	if err != nil {
		return nil, err
	}
	items, err := decodeSet(value)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	result := make([]Credential, 0, len(items))
	for i, item := range items {
		credential, err := decodeCredential(item)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", name, i, err)
		}
		result = append(result, credential)
	}
	return result, nil
}

func encodeCredentialSet(values []Credential) (cbor.Tag, error) {
	items := make([]any, 0, len(values))
	for i, value := range values {
		item, err := encodeCredential(value)
		if err != nil {
			return cbor.Tag{}, fmt.Errorf("credential %d: %w", i, err)
		}
		items = append(items, item)
	}
	return encodeSet(items)
}

func decodeSet(value any) (cbor.Array, error) {
	tag, ok := value.(cbor.Tag)
	if !ok || tag.Number != 258 {
		return nil, errors.New("must be tag 258")
	}
	items, ok := tag.Value.(cbor.Array)
	if !ok {
		return nil, errors.New("tag 258 value must be an array")
	}
	var previous []byte
	for i, item := range items {
		encoded, err := cbor.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("set item %d: %w", i, err)
		}
		if previous != nil {
			comparison := bytes.Compare(previous, encoded)
			if comparison == 0 {
				return nil, fmt.Errorf("set item %d duplicates its predecessor", i)
			}
			if comparison > 0 {
				return nil, fmt.Errorf("set item %d is outside canonical order", i)
			}
		}
		previous = encoded
	}
	return items, nil
}

func encodeSet(items []any) (cbor.Tag, error) {
	type setItem struct {
		value   any
		encoded []byte
	}
	sorted := make([]setItem, 0, len(items))
	for i, item := range items {
		encoded, err := cbor.Marshal(item)
		if err != nil {
			return cbor.Tag{}, fmt.Errorf("set item %d: %w", i, err)
		}
		sorted = append(sorted, setItem{value: item, encoded: encoded})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return bytes.Compare(sorted[i].encoded, sorted[j].encoded) < 0
	})
	result := make(cbor.Array, 0, len(sorted))
	for i, item := range sorted {
		if i > 0 && bytes.Equal(sorted[i-1].encoded, item.encoded) {
			return cbor.Tag{}, fmt.Errorf("set item %d is duplicated", i)
		}
		result = append(result, item.value)
	}
	return cbor.Tag{Number: 258, Value: result}, nil
}

func decodeCredential(value any) (Credential, error) {
	items, err := decodePair(value)
	if err != nil {
		return Credential{}, err
	}
	kind, err := decodeUint(items[0])
	if err != nil || kind > uint64(CredentialScriptHash) {
		return Credential{}, errors.New("credential kind must be 0 or 1")
	}
	hash, err := decodeHash28(items[1])
	if err != nil {
		return Credential{}, fmt.Errorf("credential hash: %w", err)
	}
	return Credential{Kind: CredentialKind(kind), Hash: hash}, nil
}

func encodeCredential(value Credential) (cbor.Array, error) {
	if value.Kind != CredentialKeyHash && value.Kind != CredentialScriptHash {
		return nil, fmt.Errorf("credential kind %d is invalid", value.Kind)
	}
	return cbor.Array{uint64(value.Kind), bytes.Clone(value.Hash[:])}, nil
}

func decodeMetadataField(fields map[string]any, name string) (*PoolMetadata, error) {
	value, err := requiredField(fields, name)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	items, err := decodePair(value)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	url, ok := items[0].(string)
	if !ok || len(url) > maxPoolMetadataURL {
		return nil, fmt.Errorf("%s URL must be at most %d bytes", name, maxPoolMetadataURL)
	}
	hash, err := decodeBytes(items[1], -1)
	if err != nil {
		return nil, fmt.Errorf("%s hash: %w", name, err)
	}
	return &PoolMetadata{URL: url, Hash: hash}, nil
}

func encodeMetadata(value *PoolMetadata) (any, error) {
	if value == nil {
		return nil, nil
	}
	if len(value.URL) > maxPoolMetadataURL {
		return nil, fmt.Errorf("URL has %d bytes, maximum is %d", len(value.URL), maxPoolMetadataURL)
	}
	return cbor.Array{value.URL, bytes.Clone(value.Hash)}, nil
}

func decodeLeiosKey(value any) (LeiosKey, error) {
	items, err := decodePair(value)
	if err != nil {
		return LeiosKey{}, err
	}
	publicKey, err := decodeBytes(items[0], leiosPublicKeySize)
	if err != nil {
		return LeiosKey{}, fmt.Errorf("public key: %w", err)
	}
	proof, err := decodeBytes(items[1], leiosProofSize)
	if err != nil {
		return LeiosKey{}, fmt.Errorf("proof of possession: %w", err)
	}
	var result LeiosKey
	copy(result.PublicKey[:], publicKey)
	copy(result.ProofOfPossession[:], proof)
	return result, nil
}

func encodeLeiosKey(value LeiosKey) cbor.Array {
	return cbor.Array{
		bytes.Clone(value.PublicKey[:]),
		bytes.Clone(value.ProofOfPossession[:]),
	}
}

func decodeRelaysField(fields map[string]any, name string) ([]Relay, error) {
	value, err := requiredField(fields, name)
	if err != nil {
		return nil, err
	}
	items, ok := value.(cbor.Array)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", name)
	}
	result := make([]Relay, 0, len(items))
	for i, item := range items {
		relay, err := decodeRelay(item)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", name, i, err)
		}
		result = append(result, relay)
	}
	return result, nil
}

func encodeRelays(values []Relay) (cbor.Array, error) {
	result := make(cbor.Array, 0, len(values))
	for i, value := range values {
		relay, err := encodeRelay(value)
		if err != nil {
			return nil, fmt.Errorf("relay %d: %w", i, err)
		}
		result = append(result, relay)
	}
	return result, nil
}

func decodeRelay(value any) (Relay, error) {
	items, ok := value.(cbor.Array)
	if !ok || len(items) < 1 {
		return Relay{}, errors.New("relay must be a non-empty array")
	}
	kind, err := decodeUint(items[0])
	if err != nil {
		return Relay{}, errors.New("relay kind must be an unsigned integer")
	}
	switch kind {
	case uint64(RelaySingleHostAddress):
		if len(items) != 4 {
			return Relay{}, errors.New("single-host-address relay must have 4 items")
		}
		port, err := decodePort(items[1])
		if err != nil {
			return Relay{}, fmt.Errorf("port: %w", err)
		}
		ipv4, err := decodeOptionalIPv4(items[2])
		if err != nil {
			return Relay{}, fmt.Errorf("IPv4: %w", err)
		}
		ipv6, err := decodeOptionalIPv6(items[3])
		if err != nil {
			return Relay{}, fmt.Errorf("IPv6: %w", err)
		}
		return Relay{Kind: RelaySingleHostAddress, Port: port, IPv4: ipv4, IPv6: ipv6}, nil
	case uint64(RelaySingleHostName):
		if len(items) != 3 {
			return Relay{}, errors.New("single-host-name relay must have 3 items")
		}
		port, err := decodePort(items[1])
		if err != nil {
			return Relay{}, fmt.Errorf("port: %w", err)
		}
		name, err := decodeDNSName(items[2])
		if err != nil {
			return Relay{}, err
		}
		return Relay{Kind: RelaySingleHostName, Port: port, DNSName: name}, nil
	case uint64(RelayMultiHostName):
		if len(items) != 2 {
			return Relay{}, errors.New("multi-host-name relay must have 2 items")
		}
		name, err := decodeDNSName(items[1])
		if err != nil {
			return Relay{}, err
		}
		return Relay{Kind: RelayMultiHostName, DNSName: name}, nil
	default:
		return Relay{}, fmt.Errorf("relay kind %d is invalid", kind)
	}
}

func encodeRelay(value Relay) (cbor.Array, error) {
	port := any(nil)
	if value.Port != nil {
		port = uint64(*value.Port)
	}
	switch value.Kind {
	case RelaySingleHostAddress:
		if value.DNSName != "" {
			return nil, errors.New("single-host-address relay must not contain a DNS name")
		}
		var ipv4 any
		if value.IPv4 != nil {
			ipv4 = bytes.Clone(value.IPv4[:])
		}
		var ipv6 any
		if value.IPv6 != nil {
			ipv6 = bytes.Clone(value.IPv6[:])
		}
		return cbor.Array{uint64(value.Kind), port, ipv4, ipv6}, nil
	case RelaySingleHostName:
		if value.IPv4 != nil || value.IPv6 != nil {
			return nil, errors.New("single-host-name relay must not contain IP addresses")
		}
		if err := validateDNSName(value.DNSName); err != nil {
			return nil, err
		}
		return cbor.Array{uint64(value.Kind), port, value.DNSName}, nil
	case RelayMultiHostName:
		if value.Port != nil || value.IPv4 != nil || value.IPv6 != nil {
			return nil, errors.New("multi-host-name relay must contain only a DNS name")
		}
		if err := validateDNSName(value.DNSName); err != nil {
			return nil, err
		}
		return cbor.Array{uint64(value.Kind), value.DNSName}, nil
	default:
		return nil, fmt.Errorf("relay kind %d is invalid", value.Kind)
	}
}

func decodePort(value any) (*uint16, error) {
	if value == nil {
		return nil, nil
	}
	port, err := decodeUint(value)
	if err != nil || port > 65535 {
		return nil, errors.New("must be null or an unsigned 16-bit integer")
	}
	result := uint16(port)
	return &result, nil
}

func decodeOptionalIPv4(value any) (*[4]byte, error) {
	if value == nil {
		return nil, nil
	}
	var result [4]byte
	data, err := decodeBytes(value, len(result))
	if err != nil {
		return nil, err
	}
	copy(result[:], data)
	return &result, nil
}

func decodeOptionalIPv6(value any) (*[16]byte, error) {
	if value == nil {
		return nil, nil
	}
	var result [16]byte
	data, err := decodeBytes(value, len(result))
	if err != nil {
		return nil, err
	}
	copy(result[:], data)
	return &result, nil
}

func decodeDNSName(value any) (string, error) {
	name, ok := value.(string)
	if !ok {
		return "", errors.New("DNS name must be text")
	}
	if err := validateDNSName(name); err != nil {
		return "", err
	}
	return name, nil
}

func validateDNSName(value string) error {
	if len(value) > maxRelayDNSNameSize {
		return fmt.Errorf("DNS name has %d bytes, maximum is %d", len(value), maxRelayDNSNameSize)
	}
	return nil
}

func decodePair(value any) (cbor.Array, error) {
	items, ok := value.(cbor.Array)
	if !ok {
		return nil, errors.New("must be an array")
	}
	if len(items) != 2 {
		return nil, fmt.Errorf("array has %d items, want 2", len(items))
	}
	return items, nil
}
