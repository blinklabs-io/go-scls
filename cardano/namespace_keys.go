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
	"encoding/binary"
	"fmt"
)

// BlocksKey identifies a stake pool's block count in an epoch.
type BlocksKey struct {
	StakePoolID [28]byte
	Epoch       uint64
}

// BlocksKeyCodec encodes blocks/v0 keys.
type BlocksKeyCodec struct{}

// Size returns the 36-byte key size.
func (BlocksKeyCodec) Size() int {
	return 36
}

// DecodeKey decodes a blocks/v0 key.
func (BlocksKeyCodec) DecodeKey(data []byte) (any, error) {
	if err := requireKeySize(data, 36); err != nil {
		return nil, err
	}
	var result BlocksKey
	copy(result.StakePoolID[:], data[:28])
	result.Epoch = binary.BigEndian.Uint64(data[28:])
	return result, nil
}

// EncodeKey encodes a blocks/v0 key.
func (BlocksKeyCodec) EncodeKey(value any) ([]byte, error) {
	key, err := blocksKeyValue(value)
	if err != nil {
		return nil, err
	}
	result := make([]byte, 36)
	copy(result, key.StakePoolID[:])
	binary.BigEndian.PutUint64(result[28:], key.Epoch)
	return result, nil
}

func blocksKeyValue(value any) (BlocksKey, error) {
	switch value := value.(type) {
	case BlocksKey:
		return value, nil
	case *BlocksKey:
		if value != nil {
			return *value, nil
		}
	}
	return BlocksKey{}, fmt.Errorf("%w: %T", ErrKeyType, value)
}

// UTxOKey identifies a transaction output.
type UTxOKey struct {
	TransactionID [32]byte
	OutputIndex   uint16
}

// UTxOKeyCodec encodes utxo/v0 keys.
type UTxOKeyCodec struct{}

// Size returns the 34-byte key size.
func (UTxOKeyCodec) Size() int {
	return 34
}

// DecodeKey decodes a utxo/v0 key.
func (UTxOKeyCodec) DecodeKey(data []byte) (any, error) {
	if err := requireKeySize(data, 34); err != nil {
		return nil, err
	}
	var result UTxOKey
	copy(result.TransactionID[:], data[:32])
	result.OutputIndex = binary.BigEndian.Uint16(data[32:])
	return result, nil
}

// EncodeKey encodes a utxo/v0 key.
func (UTxOKeyCodec) EncodeKey(value any) ([]byte, error) {
	key, err := utxoKeyValue(value)
	if err != nil {
		return nil, err
	}
	result := make([]byte, 34)
	copy(result, key.TransactionID[:])
	binary.BigEndian.PutUint16(result[32:], key.OutputIndex)
	return result, nil
}

func utxoKeyValue(value any) (UTxOKey, error) {
	switch value := value.(type) {
	case UTxOKey:
		return value, nil
	case *UTxOKey:
		if value != nil {
			return *value, nil
		}
	}
	return UTxOKey{}, fmt.Errorf("%w: %T", ErrKeyType, value)
}

// CredentialKeyKind identifies the credential hash type in a raw key.
type CredentialKeyKind uint8

const (
	// CredentialKeyKindHash identifies a verification-key hash.
	CredentialKeyKindHash CredentialKeyKind = iota
	// CredentialKeyKindScript identifies a script hash.
	CredentialKeyKindScript
)

// CredentialKey is the 29-byte key used by account and DRep namespaces.
type CredentialKey struct {
	Kind CredentialKeyKind
	Hash [28]byte
}

// CredentialKeyCodec encodes credential keys.
type CredentialKeyCodec struct{}

// Size returns the 29-byte key size.
func (CredentialKeyCodec) Size() int {
	return 29
}

// DecodeKey decodes a credential key.
func (CredentialKeyCodec) DecodeKey(data []byte) (any, error) {
	if err := requireKeySize(data, 29); err != nil {
		return nil, err
	}
	kind := CredentialKeyKind(data[0])
	if kind != CredentialKeyKindHash && kind != CredentialKeyKindScript {
		return nil, fmt.Errorf("%w: credential tag %d", ErrKeyType, data[0])
	}
	result := CredentialKey{Kind: kind}
	copy(result.Hash[:], data[1:])
	return result, nil
}

// EncodeKey encodes a credential key.
func (CredentialKeyCodec) EncodeKey(value any) ([]byte, error) {
	key, err := credentialKeyValue(value)
	if err != nil {
		return nil, err
	}
	if key.Kind != CredentialKeyKindHash && key.Kind != CredentialKeyKindScript {
		return nil, fmt.Errorf("%w: credential tag %d", ErrKeyType, key.Kind)
	}
	result := make([]byte, 29)
	result[0] = byte(key.Kind)
	copy(result[1:], key.Hash[:])
	return result, nil
}

func credentialKeyValue(value any) (CredentialKey, error) {
	switch value := value.(type) {
	case CredentialKey:
		return value, nil
	case *CredentialKey:
		if value != nil {
			return *value, nil
		}
	}
	return CredentialKey{}, fmt.Errorf("%w: %T", ErrKeyType, value)
}

// SingletonKey is the sole zero key for singleton namespaces.
type SingletonKey struct{}

// SingletonKeyCodec encodes a one-byte zero key.
type SingletonKeyCodec struct{}

// Size returns the one-byte key size.
func (SingletonKeyCodec) Size() int {
	return 1
}

// DecodeKey decodes a singleton key.
func (SingletonKeyCodec) DecodeKey(data []byte) (any, error) {
	if err := requireKeySize(data, 1); err != nil {
		return nil, err
	}
	if data[0] != 0 {
		return nil, fmt.Errorf("%w: singleton key must be zero", ErrKeyType)
	}
	return SingletonKey{}, nil
}

// EncodeKey encodes a singleton key.
func (SingletonKeyCodec) EncodeKey(value any) ([]byte, error) {
	switch value := value.(type) {
	case SingletonKey:
		return []byte{0}, nil
	case *SingletonKey:
		if value != nil {
			return []byte{0}, nil
		}
	}
	return nil, fmt.Errorf("%w: %T", ErrKeyType, value)
}

// VRFKeyHashKey is a raw 32-byte stake-pool VRF key hash.
type VRFKeyHashKey [32]byte

// VRFKeyHashKeyCodec encodes stake-pool VRF key-hash keys.
type VRFKeyHashKeyCodec struct{}

// Size returns the 32-byte key size.
func (VRFKeyHashKeyCodec) Size() int {
	return 32
}

// DecodeKey decodes a VRF key hash.
func (VRFKeyHashKeyCodec) DecodeKey(data []byte) (any, error) {
	if err := requireKeySize(data, 32); err != nil {
		return nil, err
	}
	var result VRFKeyHashKey
	copy(result[:], data)
	return result, nil
}

// EncodeKey encodes a VRF key hash.
func (VRFKeyHashKeyCodec) EncodeKey(value any) ([]byte, error) {
	switch value := value.(type) {
	case VRFKeyHashKey:
		return bytes.Clone(value[:]), nil
	case *VRFKeyHashKey:
		if value != nil {
			return bytes.Clone(value[:]), nil
		}
	}
	return nil, fmt.Errorf("%w: %T", ErrKeyType, value)
}

// GovPParamsKey identifies a protocol-parameter generation.
type GovPParamsKey uint8

const (
	// GovPParamsPrevious selects previous protocol parameters.
	GovPParamsPrevious GovPParamsKey = iota
	// GovPParamsCurrent selects current protocol parameters.
	GovPParamsCurrent
	// GovPParamsPossibleFuture selects possible future protocol parameters.
	GovPParamsPossibleFuture
	// GovPParamsDefiniteFuture selects definite future protocol parameters.
	GovPParamsDefiniteFuture
)

var govPParamsKeyBytes = [...][4]byte{
	{'p', 'r', 'e', 'v'},
	{'c', 'u', 'r', 'r'},
	{'p', 'f', 'u', 't'},
	{'d', 'f', 'u', 't'},
}

// GovPParamsKeyCodec encodes gov/pparams/v0 selectors.
type GovPParamsKeyCodec struct{}

// Size returns the four-byte key size.
func (GovPParamsKeyCodec) Size() int {
	return 4
}

// DecodeKey decodes a protocol-parameter selector.
func (GovPParamsKeyCodec) DecodeKey(data []byte) (any, error) {
	if err := requireKeySize(data, 4); err != nil {
		return nil, err
	}
	switch string(data) {
	case "prev":
		return GovPParamsPrevious, nil
	case "curr":
		return GovPParamsCurrent, nil
	case "pfut":
		return GovPParamsPossibleFuture, nil
	case "dfut":
		return GovPParamsDefiniteFuture, nil
	default:
		return nil, fmt.Errorf("%w: unknown gov/pparams/v0 key %q", ErrKeyType, data)
	}
}

// EncodeKey encodes a protocol-parameter selector.
func (GovPParamsKeyCodec) EncodeKey(value any) ([]byte, error) {
	var key GovPParamsKey
	switch value := value.(type) {
	case GovPParamsKey:
		key = value
	case *GovPParamsKey:
		if value == nil {
			return nil, fmt.Errorf("%w: nil *GovPParamsKey", ErrKeyType)
		}
		key = *value
	default:
		return nil, fmt.Errorf("%w: %T", ErrKeyType, value)
	}
	if key > GovPParamsDefiniteFuture {
		return nil, fmt.Errorf("%w: gov/pparams/v0 key %d", ErrKeyType, key)
	}
	return bytes.Clone(govPParamsKeyBytes[key][:]), nil
}

// GovProposalKey identifies a governance action in a transaction.
type GovProposalKey struct {
	TransactionID [32]byte
	ActionIndex   uint16
}

// GovProposalKeyCodec encodes gov/proposals/v0 keys.
type GovProposalKeyCodec struct{}

// Size returns the 34-byte key size.
func (GovProposalKeyCodec) Size() int {
	return 34
}

// DecodeKey decodes a governance proposal key.
func (GovProposalKeyCodec) DecodeKey(data []byte) (any, error) {
	if err := requireKeySize(data, 34); err != nil {
		return nil, err
	}
	var result GovProposalKey
	copy(result.TransactionID[:], data[:32])
	result.ActionIndex = binary.BigEndian.Uint16(data[32:])
	return result, nil
}

// EncodeKey encodes a governance proposal key.
func (GovProposalKeyCodec) EncodeKey(value any) ([]byte, error) {
	key, err := govProposalKeyValue(value)
	if err != nil {
		return nil, err
	}
	result := make([]byte, 34)
	copy(result, key.TransactionID[:])
	binary.BigEndian.PutUint16(result[32:], key.ActionIndex)
	return result, nil
}

func govProposalKeyValue(value any) (GovProposalKey, error) {
	switch value := value.(type) {
	case GovProposalKey:
		return value, nil
	case *GovProposalKey:
		if value != nil {
			return *value, nil
		}
	}
	return GovProposalKey{}, fmt.Errorf("%w: %T", ErrKeyType, value)
}

// GovProposalRootKey identifies a governance-action purpose root.
type GovProposalRootKey uint8

const (
	// GovProposalRootPParams identifies protocol-parameter updates.
	GovProposalRootPParams GovProposalRootKey = iota
	// GovProposalRootHardFork identifies hard-fork actions.
	GovProposalRootHardFork
	// GovProposalRootCommittee identifies committee actions.
	GovProposalRootCommittee
	// GovProposalRootConstitution identifies constitution actions.
	GovProposalRootConstitution
)

// GovProposalRootKeyCodec encodes gov/proposals/roots/v0 keys.
type GovProposalRootKeyCodec struct{}

// Size returns the one-byte key size.
func (GovProposalRootKeyCodec) Size() int {
	return 1
}

// DecodeKey decodes a governance proposal root key.
func (GovProposalRootKeyCodec) DecodeKey(data []byte) (any, error) {
	if err := requireKeySize(data, 1); err != nil {
		return nil, err
	}
	key := GovProposalRootKey(data[0])
	if key > GovProposalRootConstitution {
		return nil, fmt.Errorf("%w: governance proposal root tag %d", ErrKeyType, key)
	}
	return key, nil
}

// EncodeKey encodes a governance proposal root key.
func (GovProposalRootKeyCodec) EncodeKey(value any) ([]byte, error) {
	var key GovProposalRootKey
	switch value := value.(type) {
	case GovProposalRootKey:
		key = value
	case *GovProposalRootKey:
		if value == nil {
			return nil, fmt.Errorf("%w: nil *GovProposalRootKey", ErrKeyType)
		}
		key = *value
	default:
		return nil, fmt.Errorf("%w: %T", ErrKeyType, value)
	}
	if key > GovProposalRootConstitution {
		return nil, fmt.Errorf("%w: governance proposal root tag %d", ErrKeyType, key)
	}
	return []byte{byte(key)}, nil
}

// SnapshotKey is the fixed 31-byte key shared by snapshot namespaces.
type SnapshotKey [31]byte

// SnapshotKeyCodec encodes snapshot namespace keys.
type SnapshotKeyCodec struct{}

// Size returns the 31-byte key size.
func (SnapshotKeyCodec) Size() int {
	return 31
}

// DecodeKey decodes a snapshot key.
func (SnapshotKeyCodec) DecodeKey(data []byte) (any, error) {
	if err := requireKeySize(data, 31); err != nil {
		return nil, err
	}
	var result SnapshotKey
	copy(result[:], data)
	return result, nil
}

// EncodeKey encodes a snapshot key.
func (SnapshotKeyCodec) EncodeKey(value any) ([]byte, error) {
	switch value := value.(type) {
	case SnapshotKey:
		return bytes.Clone(value[:]), nil
	case *SnapshotKey:
		if value != nil {
			return bytes.Clone(value[:]), nil
		}
	}
	return nil, fmt.Errorf("%w: %T", ErrKeyType, value)
}

func requireKeySize(data []byte, size int) error {
	if len(data) != size {
		return fmt.Errorf("%w: got %d, want %d", ErrKeySize, len(data), size)
	}
	return nil
}
