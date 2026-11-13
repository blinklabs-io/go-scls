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

	"github.com/blinklabs-io/go-scls/cardano/cbor"
)

// ErrPayload indicates a namespace payload that does not match its
// namespace's schema. The wrapped error names the namespace and the field.
var ErrPayload = errors.New("invalid namespace payload")

// Anchor is a URL and the hash of the document it references.
type Anchor struct {
	URL  string
	Hash []byte
}

// Rational is a non-negative rational number with a positive denominator.
type Rational struct {
	Numerator   uint64
	Denominator uint64
}

// ExUnits is a pair of script execution budgets.
type ExUnits struct {
	Memory uint64
	Steps  uint64
}

// ProtocolVersion is a protocol major and minor version.
type ProtocolVersion struct {
	Major uint64
	Minor uint64
}

// DRepKind identifies a delegation target in entities/accounts/v0.
type DRepKind uint8

const (
	// DRepKeyHash delegates to a DRep key-hash credential.
	DRepKeyHash DRepKind = iota
	// DRepScriptHash delegates to a DRep script-hash credential.
	DRepScriptHash
	// DRepAlwaysAbstain is the predefined always-abstain DRep.
	DRepAlwaysAbstain
	// DRepAlwaysNoConfidence is the predefined always-no-confidence DRep.
	DRepAlwaysNoConfidence
)

// DRepDelegation is a vote delegation. Hash is set only for DRepKeyHash and
// DRepScriptHash.
type DRepDelegation struct {
	Kind DRepKind
	Hash Hash28
}

// AccountState is the entities/accounts/v0 payload.
type AccountState struct {
	Balance             uint64
	Deposit             uint64
	DRepDelegation      *DRepDelegation
	StakePoolDelegation *Hash28
}

// DRepState is the entities/dreps/v0 payload.
type DRepState struct {
	Anchor      *Anchor
	Expiry      uint64
	Deposit     uint64
	Delegations []Credential
}

// CommitteeAuthorizationKind identifies a committee member's hot-key state.
type CommitteeAuthorizationKind uint8

const (
	// CommitteeHotCredential is an authorized hot credential.
	CommitteeHotCredential CommitteeAuthorizationKind = iota
	// CommitteeResigned is a resignation with an optional anchor.
	CommitteeResigned
)

// CommitteeAuthorization is a committee member's hot-key authorization.
type CommitteeAuthorization struct {
	Kind          CommitteeAuthorizationKind
	HotCredential Credential
	Anchor        *Anchor
}

// CommitteeMemberState binds a cold credential to its authorization.
type CommitteeMemberState struct {
	Cold          Credential
	Authorization CommitteeAuthorization
}

// CommitteeState is the entities/committee/v0 payload. Members are encoded
// in deterministic order of their cold credentials.
type CommitteeState []CommitteeMemberState

// CommitteeMember is a committee member and its term expiry epoch.
type CommitteeMember struct {
	Cold   Credential
	Expiry uint64
}

// GovCommittee is the committee of the gov/committee/v0 payload. A nil
// *GovCommittee encodes as CBOR null.
type GovCommittee struct {
	Members   []CommitteeMember
	Threshold UnitInterval
}

// Constitution is the gov/constitution/v0 payload.
type Constitution struct {
	Anchor     Anchor
	ScriptHash *Hash28
}

// GovActionID identifies a governance action.
type GovActionID struct {
	TransactionID Hash32
	Index         uint16
}

// ProtocolParameters is the gov/pparams/v0 payload.
type ProtocolParameters struct {
	MinFeeA                    uint64
	MinFeeB                    uint64
	MaxBlockSize               uint32
	MaxTxSize                  uint32
	MaxBlockHeaderSize         uint16
	KeyDeposit                 uint64
	PoolDeposit                uint64
	MaxEpoch                   uint32
	NOpt                       uint16
	A0                         Rational
	Rho                        UnitInterval
	Tau                        UnitInterval
	ProtocolVersion            ProtocolVersion
	MinPoolCost                uint64
	AdaPerUTxOByte             uint64
	CostModels                 map[uint8][]int64
	MemoryPrice                Rational
	StepPrice                  Rational
	MaxTxExUnits               ExUnits
	MaxBlockExUnits            ExUnits
	MaxValueSize               uint32
	CollateralPercentage       uint16
	MaxCollateralInputs        uint16
	PoolVotingThresholds       [5]UnitInterval
	DRepVotingThresholds       [10]UnitInterval
	MinCommitteeSize           uint16
	CommitteeTermLimit         uint32
	GovActionValidity          uint32
	GovActionDeposit           uint64
	DRepDeposit                uint64
	DRepInactivity             uint32
	MinFeeRefScriptCostPerByte Rational
}

// Nonce is a protocol nonce. A nil Hash is the neutral nonce.
type Nonce struct {
	Hash *Hash32
}

// Nonces is the nonces/v0 payload. A nil LastSlot is the origin.
type Nonces struct {
	LabNonce            Nonce
	LastSlot            *uint64
	EpochNonce          Nonce
	CertCounters        map[Hash28]uint64
	EvolvingNonce       Nonce
	CandidateNonce      Nonce
	LastEpochBlockNonce Nonce
}

// SnapshotValueKind identifies the variant of a snapshot entry.
type SnapshotValueKind uint8

const (
	// SnapshotCoin is a coin amount.
	SnapshotCoin SnapshotValueKind = iota
	// SnapshotPool is a pool key hash.
	SnapshotPool
)

// SnapshotOut is the payload of the snapshots/{mark,set,go}/v0 namespaces.
// Coin is set for SnapshotCoin and Pool for SnapshotPool.
type SnapshotOut struct {
	Kind SnapshotValueKind
	Coin uint64
	Pool Hash28
}

// BlocksMade is the blocks/v0 payload: the number of blocks a pool made.
type BlocksMade uint64

// VRFKeyHashCount is the entities/stake_pools/vrf_key_hashes/v0 payload. It
// is positive.
type VRFKeyHashCount uint64

// GovActionKind identifies a governance action variant.
type GovActionKind uint8

const (
	// GovParameterChange updates protocol parameters.
	GovParameterChange GovActionKind = iota
	// GovHardForkInitiation proposes a protocol version.
	GovHardForkInitiation
	// GovTreasuryWithdrawals withdraws from the treasury.
	GovTreasuryWithdrawals
	// GovNoConfidence is a motion of no confidence.
	GovNoConfidence
	// GovUpdateCommittee changes committee membership and threshold.
	GovUpdateCommittee
	// GovNewConstitution replaces the constitution.
	GovNewConstitution
	// GovInfo is an information-only action.
	GovInfo
)

// Vote is a governance vote: 0 no, 1 yes, 2 abstain.
type Vote uint8

const (
	// VoteNo is a no vote.
	VoteNo Vote = iota
	// VoteYes is a yes vote.
	VoteYes
	// VoteAbstain is an abstention.
	VoteAbstain
)

// CredentialVote is a vote cast by a DRep or committee credential.
type CredentialVote struct {
	Credential Credential
	Vote       Vote
}

// PoolVote is a vote cast by a stake pool.
type PoolVote struct {
	Pool Hash28
	Vote Vote
}

// Withdrawal is a treasury withdrawal to a 29-byte reward account.
type Withdrawal struct {
	Account []byte
	Amount  uint64
}

// ExUnitPrices holds the prices of memory and steps.
type ExUnitPrices struct {
	Memory Rational
	Step   Rational
}

// ProtocolParametersUpdate is the partial parameter update of a parameter
// change action. A nil field is absent from the update. The protocol version
// is never part of an update.
type ProtocolParametersUpdate struct {
	MinFeeA                    *uint64
	MinFeeB                    *uint64
	MaxBlockSize               *uint32
	MaxTxSize                  *uint32
	MaxBlockHeaderSize         *uint16
	KeyDeposit                 *uint64
	PoolDeposit                *uint64
	MaxEpoch                   *uint32
	NOpt                       *uint16
	A0                         *Rational
	Rho                        *UnitInterval
	Tau                        *UnitInterval
	MinPoolCost                *uint64
	AdaPerUTxOByte             *uint64
	CostModels                 *map[uint8][]int64
	ExUnitPrices               *ExUnitPrices
	MaxTxExUnits               *ExUnits
	MaxBlockExUnits            *ExUnits
	MaxValueSize               *uint32
	CollateralPercentage       *uint16
	MaxCollateralInputs        *uint16
	PoolVotingThresholds       *[5]UnitInterval
	DRepVotingThresholds       *[10]UnitInterval
	MinCommitteeSize           *uint16
	CommitteeTermLimit         *uint32
	GovActionValidity          *uint32
	GovActionDeposit           *uint64
	DRepDeposit                *uint64
	DRepInactivity             *uint32
	MinFeeRefScriptCostPerByte *Rational
}

// GovAction is a governance action. Which fields are used depends on Kind:
// Purpose (all but treasury withdrawals and info), ParamUpdate and
// PolicyHash (parameter change), PolicyHash and Withdrawals (treasury),
// ProtocolVersion (hard fork), Removed, Added and Threshold (committee
// update), Constitution (new constitution). Unused fields must be zero.
type GovAction struct {
	Kind            GovActionKind
	Purpose         *GovActionID
	ParamUpdate     ProtocolParametersUpdate
	PolicyHash      *Hash28
	ProtocolVersion ProtocolVersion
	Withdrawals     []Withdrawal
	Removed         []Credential
	Added           []CommitteeMember
	Threshold       UnitInterval
	Constitution    Constitution
}

// ProposalProcedure is the proposal embedded in a gov/proposals/v0 payload.
// The embedded bytes are ledger-serialized CBOR, whose set and map order may
// differ from deterministic order. Raw holds the original bytes when they
// differ from this package's encoding of the fields; Encode emits Raw when it
// is non-nil and still decodes to the same fields.
type ProposalProcedure struct {
	Deposit       uint64
	ReturnAddress []byte
	Action        GovAction
	Anchor        Anchor
	Raw           []byte
}

// GovProposal is the gov/proposals/v0 payload.
type GovProposal struct {
	Order          uint64
	DRepVotes      []CredentialVote
	ProposedIn     uint64
	ExpiresAfter   uint64
	CommitteeVotes []CredentialVote
	StakePoolVotes []PoolVote
	Procedure      ProposalProcedure
}

// TxOutFormat selects the transaction-output encoding.
type TxOutFormat uint8

const (
	// TxOutShelley is the array form [address, value, ?datum_hash].
	TxOutShelley TxOutFormat = iota
	// TxOutBabbage is the map form {0: address, 1: value, ...}.
	TxOutBabbage
)

// Asset is one named asset quantity. Quantity is positive.
type Asset struct {
	Name     []byte
	Quantity uint64
}

// PolicyAssets lists the assets of one policy in canonical name order.
type PolicyAssets struct {
	Policy Hash28
	Assets []Asset
}

// MultiAsset lists policies in canonical policy order. Each policy holds at
// least one asset.
type MultiAsset []PolicyAssets

// TxValue is an output value. A nil Assets encodes as a bare coin; a non-nil
// Assets, even an empty one, encodes as [coin, multiasset].
type TxValue struct {
	Coin   uint64
	Assets *MultiAsset
}

// TxOutDatum is an output datum option.
type TxOutDatum struct {
	// Hash is set for a datum hash.
	Hash *Hash32
	// Inline is the deterministic CBOR of an inline datum (Plutus data).
	Inline []byte
}

// TxOutScript is a reference script. Script holds the deterministic CBOR of
// a native script for Language 0 and the script bytes for Languages 1-3.
type TxOutScript struct {
	Language uint8
	Script   []byte
}

// TxOut is the utxo/v0 payload.
type TxOut struct {
	Format TxOutFormat
	// Address is the raw address bytes.
	Address []byte
	Value   TxValue
	// Datum is nil when absent. The Shelley form allows only a datum hash.
	Datum *TxOutDatum
	// Script is nil when absent and is valid only in the Babbage form.
	Script *TxOutScript
	// Raw holds the embedded ledger-serialized tx_out when it differs from
	// this package's encoding of the fields (ledger map order need not be
	// deterministic order). Encode emits Raw when it is non-nil and still
	// decodes to the same fields.
	Raw []byte
}

func decodePayloadWith[T any](
	name string,
	data []byte,
	decode func(any) (T, error),
) (T, error) {
	var zero T
	value, err := cbor.Unmarshal(data)
	if err != nil {
		return zero, fmt.Errorf("%w: %s: %w", ErrPayload, name, err)
	}
	result, err := decode(value)
	if err != nil {
		return zero, fmt.Errorf("%w: %s: %w", ErrPayload, name, err)
	}
	return result, nil
}

func encodePayloadWith[T any](
	name string,
	value T,
	encode func(T) (any, error),
) ([]byte, error) {
	encoded, err := encode(value)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrPayload, name, err)
	}
	result, err := cbor.Marshal(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrPayload, name, err)
	}
	return result, nil
}

// payloadValue accepts T or *T. A nil pointer is reported as ok == false.
func payloadValue[T any](value any) (T, bool) {
	switch value := value.(type) {
	case T:
		return value, true
	case *T:
		if value != nil {
			return *value, true
		}
	}
	var zero T
	return zero, false
}
