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

import "fmt"

// DecodeBlocksMade decodes a blocks/v0 payload.
func DecodeBlocksMade(data []byte) (BlocksMade, error) {
	return decodePayloadWith("blocks/v0", data, decodeBlocksMade)
}

// EncodeBlocksMade encodes a blocks/v0 payload as deterministic CBOR.
func EncodeBlocksMade(value BlocksMade) ([]byte, error) {
	return encodePayloadWith("blocks/v0", value, encodeBlocksMade)
}

// BlocksMadePayloadCodec is the typed payload codec for blocks/v0.
type BlocksMadePayloadCodec struct{}

// DecodePayload decodes a BlocksMade.
func (BlocksMadePayloadCodec) DecodePayload(data []byte) (any, error) {
	return DecodeBlocksMade(data)
}

// EncodePayload encodes a BlocksMade or *BlocksMade.
func (BlocksMadePayloadCodec) EncodePayload(value any) ([]byte, error) {
	typed, ok := payloadValue[BlocksMade](value)
	if !ok {
		return nil, fmt.Errorf("%w: blocks/v0: Go type %T", ErrPayload, value)
	}
	return EncodeBlocksMade(typed)
}

// DecodeAccountState decodes a entities/accounts/v0 payload.
func DecodeAccountState(data []byte) (AccountState, error) {
	return decodePayloadWith("entities/accounts/v0", data, decodeAccountState)
}

// EncodeAccountState encodes a entities/accounts/v0 payload as deterministic CBOR.
func EncodeAccountState(value AccountState) ([]byte, error) {
	return encodePayloadWith("entities/accounts/v0", value, encodeAccountState)
}

// AccountStatePayloadCodec is the typed payload codec for entities/accounts/v0.
type AccountStatePayloadCodec struct{}

// DecodePayload decodes a AccountState.
func (AccountStatePayloadCodec) DecodePayload(data []byte) (any, error) {
	return DecodeAccountState(data)
}

// EncodePayload encodes a AccountState or *AccountState.
func (AccountStatePayloadCodec) EncodePayload(value any) ([]byte, error) {
	typed, ok := payloadValue[AccountState](value)
	if !ok {
		return nil, fmt.Errorf("%w: entities/accounts/v0: Go type %T", ErrPayload, value)
	}
	return EncodeAccountState(typed)
}

// DecodeDRepState decodes a entities/dreps/v0 payload.
func DecodeDRepState(data []byte) (DRepState, error) {
	return decodePayloadWith("entities/dreps/v0", data, decodeDRepState)
}

// EncodeDRepState encodes a entities/dreps/v0 payload as deterministic CBOR.
func EncodeDRepState(value DRepState) ([]byte, error) {
	return encodePayloadWith("entities/dreps/v0", value, encodeDRepState)
}

// DRepStatePayloadCodec is the typed payload codec for entities/dreps/v0.
type DRepStatePayloadCodec struct{}

// DecodePayload decodes a DRepState.
func (DRepStatePayloadCodec) DecodePayload(data []byte) (any, error) {
	return DecodeDRepState(data)
}

// EncodePayload encodes a DRepState or *DRepState.
func (DRepStatePayloadCodec) EncodePayload(value any) ([]byte, error) {
	typed, ok := payloadValue[DRepState](value)
	if !ok {
		return nil, fmt.Errorf("%w: entities/dreps/v0: Go type %T", ErrPayload, value)
	}
	return EncodeDRepState(typed)
}

// DecodeCommitteeState decodes a entities/committee/v0 payload.
func DecodeCommitteeState(data []byte) (CommitteeState, error) {
	return decodePayloadWith("entities/committee/v0", data, decodeCommitteeState)
}

// EncodeCommitteeState encodes a entities/committee/v0 payload as deterministic CBOR.
func EncodeCommitteeState(value CommitteeState) ([]byte, error) {
	return encodePayloadWith("entities/committee/v0", value, encodeCommitteeState)
}

// CommitteeStatePayloadCodec is the typed payload codec for entities/committee/v0.
type CommitteeStatePayloadCodec struct{}

// DecodePayload decodes a CommitteeState.
func (CommitteeStatePayloadCodec) DecodePayload(data []byte) (any, error) {
	return DecodeCommitteeState(data)
}

// EncodePayload encodes a CommitteeState or *CommitteeState.
func (CommitteeStatePayloadCodec) EncodePayload(value any) ([]byte, error) {
	typed, ok := payloadValue[CommitteeState](value)
	if !ok {
		return nil, fmt.Errorf("%w: entities/committee/v0: Go type %T", ErrPayload, value)
	}
	return EncodeCommitteeState(typed)
}

// DecodeConstitution decodes a gov/constitution/v0 payload.
func DecodeConstitution(data []byte) (Constitution, error) {
	return decodePayloadWith("gov/constitution/v0", data, decodeConstitution)
}

// EncodeConstitution encodes a gov/constitution/v0 payload as deterministic CBOR.
func EncodeConstitution(value Constitution) ([]byte, error) {
	return encodePayloadWith("gov/constitution/v0", value, encodeConstitution)
}

// ConstitutionPayloadCodec is the typed payload codec for gov/constitution/v0.
type ConstitutionPayloadCodec struct{}

// DecodePayload decodes a Constitution.
func (ConstitutionPayloadCodec) DecodePayload(data []byte) (any, error) {
	return DecodeConstitution(data)
}

// EncodePayload encodes a Constitution or *Constitution.
func (ConstitutionPayloadCodec) EncodePayload(value any) ([]byte, error) {
	typed, ok := payloadValue[Constitution](value)
	if !ok {
		return nil, fmt.Errorf("%w: gov/constitution/v0: Go type %T", ErrPayload, value)
	}
	return EncodeConstitution(typed)
}

// DecodeGovActionID decodes a gov/proposals/roots/v0 payload.
func DecodeGovActionID(data []byte) (GovActionID, error) {
	return decodePayloadWith("gov/proposals/roots/v0", data, decodeGovActionID)
}

// EncodeGovActionID encodes a gov/proposals/roots/v0 payload as deterministic CBOR.
func EncodeGovActionID(value GovActionID) ([]byte, error) {
	return encodePayloadWith("gov/proposals/roots/v0", value, encodeGovActionID)
}

// GovProposalRootPayloadCodec is the typed payload codec for gov/proposals/roots/v0.
type GovProposalRootPayloadCodec struct{}

// DecodePayload decodes a GovActionID.
func (GovProposalRootPayloadCodec) DecodePayload(data []byte) (any, error) {
	return DecodeGovActionID(data)
}

// EncodePayload encodes a GovActionID or *GovActionID.
func (GovProposalRootPayloadCodec) EncodePayload(value any) ([]byte, error) {
	typed, ok := payloadValue[GovActionID](value)
	if !ok {
		return nil, fmt.Errorf("%w: gov/proposals/roots/v0: Go type %T", ErrPayload, value)
	}
	return EncodeGovActionID(typed)
}

// DecodeProtocolParameters decodes a gov/pparams/v0 payload.
func DecodeProtocolParameters(data []byte) (ProtocolParameters, error) {
	return decodePayloadWith("gov/pparams/v0", data, decodeProtocolParameters)
}

// EncodeProtocolParameters encodes a gov/pparams/v0 payload as deterministic CBOR.
func EncodeProtocolParameters(value ProtocolParameters) ([]byte, error) {
	return encodePayloadWith("gov/pparams/v0", value, encodeProtocolParameters)
}

// ProtocolParametersPayloadCodec is the typed payload codec for gov/pparams/v0.
type ProtocolParametersPayloadCodec struct{}

// DecodePayload decodes a ProtocolParameters.
func (ProtocolParametersPayloadCodec) DecodePayload(data []byte) (any, error) {
	return DecodeProtocolParameters(data)
}

// EncodePayload encodes a ProtocolParameters or *ProtocolParameters.
func (ProtocolParametersPayloadCodec) EncodePayload(value any) ([]byte, error) {
	typed, ok := payloadValue[ProtocolParameters](value)
	if !ok {
		return nil, fmt.Errorf("%w: gov/pparams/v0: Go type %T", ErrPayload, value)
	}
	return EncodeProtocolParameters(typed)
}

// DecodeNonces decodes a nonces/v0 payload.
func DecodeNonces(data []byte) (Nonces, error) {
	return decodePayloadWith("nonces/v0", data, decodeNonces)
}

// EncodeNonces encodes a nonces/v0 payload as deterministic CBOR.
func EncodeNonces(value Nonces) ([]byte, error) {
	return encodePayloadWith("nonces/v0", value, encodeNonces)
}

// NoncesPayloadCodec is the typed payload codec for nonces/v0.
type NoncesPayloadCodec struct{}

// DecodePayload decodes a Nonces.
func (NoncesPayloadCodec) DecodePayload(data []byte) (any, error) {
	return DecodeNonces(data)
}

// EncodePayload encodes a Nonces or *Nonces.
func (NoncesPayloadCodec) EncodePayload(value any) ([]byte, error) {
	typed, ok := payloadValue[Nonces](value)
	if !ok {
		return nil, fmt.Errorf("%w: nonces/v0: Go type %T", ErrPayload, value)
	}
	return EncodeNonces(typed)
}

// DecodeSnapshotOut decodes a snapshots/{mark,set,go}/v0 payload.
func DecodeSnapshotOut(data []byte) (SnapshotOut, error) {
	return decodePayloadWith("snapshots/{mark,set,go}/v0", data, decodeSnapshotOut)
}

// EncodeSnapshotOut encodes a snapshots/{mark,set,go}/v0 payload as deterministic CBOR.
func EncodeSnapshotOut(value SnapshotOut) ([]byte, error) {
	return encodePayloadWith("snapshots/{mark,set,go}/v0", value, encodeSnapshotOut)
}

// SnapshotPayloadCodec is the typed payload codec for snapshots/{mark,set,go}/v0.
type SnapshotPayloadCodec struct{}

// DecodePayload decodes a SnapshotOut.
func (SnapshotPayloadCodec) DecodePayload(data []byte) (any, error) {
	return DecodeSnapshotOut(data)
}

// EncodePayload encodes a SnapshotOut or *SnapshotOut.
func (SnapshotPayloadCodec) EncodePayload(value any) ([]byte, error) {
	typed, ok := payloadValue[SnapshotOut](value)
	if !ok {
		return nil, fmt.Errorf("%w: snapshots/{mark,set,go}/v0: Go type %T", ErrPayload, value)
	}
	return EncodeSnapshotOut(typed)
}

// DecodeVRFKeyHashCount decodes a entities/stake_pools/vrf_key_hashes/v0 payload.
func DecodeVRFKeyHashCount(data []byte) (VRFKeyHashCount, error) {
	return decodePayloadWith("entities/stake_pools/vrf_key_hashes/v0", data, decodeVRFKeyHashCount)
}

// EncodeVRFKeyHashCount encodes a entities/stake_pools/vrf_key_hashes/v0 payload as deterministic CBOR.
func EncodeVRFKeyHashCount(value VRFKeyHashCount) ([]byte, error) {
	return encodePayloadWith("entities/stake_pools/vrf_key_hashes/v0", value, encodeVRFKeyHashCount)
}

// VRFKeyHashCountPayloadCodec is the typed payload codec for entities/stake_pools/vrf_key_hashes/v0.
type VRFKeyHashCountPayloadCodec struct{}

// DecodePayload decodes a VRFKeyHashCount.
func (VRFKeyHashCountPayloadCodec) DecodePayload(data []byte) (any, error) {
	return DecodeVRFKeyHashCount(data)
}

// EncodePayload encodes a VRFKeyHashCount or *VRFKeyHashCount.
func (VRFKeyHashCountPayloadCodec) EncodePayload(value any) ([]byte, error) {
	typed, ok := payloadValue[VRFKeyHashCount](value)
	if !ok {
		return nil, fmt.Errorf("%w: entities/stake_pools/vrf_key_hashes/v0: Go type %T", ErrPayload, value)
	}
	return EncodeVRFKeyHashCount(typed)
}

// DecodeGovCommittee decodes a gov/committee/v0 payload. A nil result is
// the null (no committee) payload.
func DecodeGovCommittee(data []byte) (*GovCommittee, error) {
	return decodePayloadWith("gov/committee/v0", data, decodeGovCommittee)
}

// EncodeGovCommittee encodes a gov/committee/v0 payload. A nil value encodes
// as CBOR null.
func EncodeGovCommittee(value *GovCommittee) ([]byte, error) {
	return encodePayloadWith("gov/committee/v0", value, encodeGovCommittee)
}

// GovCommitteePayloadCodec is the typed payload codec for gov/committee/v0.
type GovCommitteePayloadCodec struct{}

// DecodePayload decodes a *GovCommittee, nil for the null payload.
func (GovCommitteePayloadCodec) DecodePayload(data []byte) (any, error) {
	return DecodeGovCommittee(data)
}

// EncodePayload encodes a *GovCommittee or GovCommittee.
func (GovCommitteePayloadCodec) EncodePayload(value any) ([]byte, error) {
	switch value := value.(type) {
	case *GovCommittee:
		return EncodeGovCommittee(value)
	case GovCommittee:
		return EncodeGovCommittee(&value)
	default:
		return nil, fmt.Errorf("%w: gov/committee/v0: Go type %T", ErrPayload, value)
	}
}

// DecodeTxOut decodes a utxo/v0 payload.
func DecodeTxOut(data []byte) (TxOut, error) {
	return decodePayloadWith("utxo/v0", data, decodeTxOut)
}

// EncodeTxOut encodes a utxo/v0 payload as deterministic CBOR.
func EncodeTxOut(value TxOut) ([]byte, error) {
	return encodePayloadWith("utxo/v0", value, encodeTxOut)
}

// TxOutPayloadCodec is the typed payload codec for utxo/v0.
type TxOutPayloadCodec struct{}

// DecodePayload decodes a TxOut.
func (TxOutPayloadCodec) DecodePayload(data []byte) (any, error) {
	return DecodeTxOut(data)
}

// EncodePayload encodes a TxOut or *TxOut.
func (TxOutPayloadCodec) EncodePayload(value any) ([]byte, error) {
	typed, ok := payloadValue[TxOut](value)
	if !ok {
		return nil, fmt.Errorf("%w: utxo/v0: Go type %T", ErrPayload, value)
	}
	return EncodeTxOut(typed)
}
