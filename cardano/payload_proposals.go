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
	"reflect"

	"github.com/blinklabs-io/go-scls/cardano/cbor"
)

type updateField struct {
	key    uint64
	decode func(raw any) error
	encode func() (any, bool, error)
}

func optionalParam[T any](key uint64, target **T) updateField {
	return updateField{
		key: key,
		decode: func(raw any) error {
			var value T
			if err := decodeParam(&value, raw); err != nil {
				return err
			}
			*target = &value
			return nil
		},
		encode: func() (any, bool, error) {
			if *target == nil {
				return nil, false, nil
			}
			encoded, err := encodeParam(*target)
			return encoded, true, err
		},
	}
}

func updateFields(u *ProtocolParametersUpdate) []updateField {
	return []updateField{
		optionalParam(0, &u.MinFeeA),
		optionalParam(1, &u.MinFeeB),
		optionalParam(2, &u.MaxBlockSize),
		optionalParam(3, &u.MaxTxSize),
		optionalParam(4, &u.MaxBlockHeaderSize),
		optionalParam(5, &u.KeyDeposit),
		optionalParam(6, &u.PoolDeposit),
		optionalParam(7, &u.MaxEpoch),
		optionalParam(8, &u.NOpt),
		optionalParam(9, &u.A0),
		optionalParam(10, &u.Rho),
		optionalParam(11, &u.Tau),
		optionalParam(16, &u.MinPoolCost),
		optionalParam(17, &u.AdaPerUTxOByte),
		optionalParam(18, &u.CostModels),
		optionalParam(19, &u.ExUnitPrices),
		optionalParam(20, &u.MaxTxExUnits),
		optionalParam(21, &u.MaxBlockExUnits),
		optionalParam(22, &u.MaxValueSize),
		optionalParam(23, &u.CollateralPercentage),
		optionalParam(24, &u.MaxCollateralInputs),
		optionalParam(25, &u.PoolVotingThresholds),
		optionalParam(26, &u.DRepVotingThresholds),
		optionalParam(27, &u.MinCommitteeSize),
		optionalParam(28, &u.CommitteeTermLimit),
		optionalParam(29, &u.GovActionValidity),
		optionalParam(30, &u.GovActionDeposit),
		optionalParam(31, &u.DRepDeposit),
		optionalParam(32, &u.DRepInactivity),
		optionalParam(33, &u.MinFeeRefScriptCostPerByte),
	}
}

func decodeParamUpdate(value any) (ProtocolParametersUpdate, error) {
	entries, ok := value.(cbor.Map)
	if !ok {
		return ProtocolParametersUpdate{}, errors.New("parameter update must be a map")
	}
	var result ProtocolParametersUpdate
	byKey := make(map[uint64]updateField, 32)
	for _, field := range updateFields(&result) {
		byKey[field.key] = field
	}
	for _, entry := range entries {
		key, err := decodeUint(entry.Key)
		if err != nil {
			return ProtocolParametersUpdate{}, errors.New("parameter key must be an unsigned integer")
		}
		field, ok := byKey[key]
		if !ok {
			return ProtocolParametersUpdate{}, fmt.Errorf("parameter %d is not updatable", key)
		}
		if err := field.decode(entry.Value); err != nil {
			return ProtocolParametersUpdate{}, fmt.Errorf("parameter %d: %w", key, err)
		}
	}
	return result, nil
}

func encodeParamUpdate(value ProtocolParametersUpdate) (any, error) {
	result := cbor.Map{}
	for _, field := range updateFields(&value) {
		encoded, present, err := field.encode()
		if err != nil {
			return nil, fmt.Errorf("parameter %d: %w", field.key, err)
		}
		if present {
			result = append(result, cbor.MapEntry{Key: field.key, Value: encoded})
		}
	}
	return result, nil
}

func decodeVote(value any) (Vote, error) {
	vote, err := decodeUintMax(value, uint64(VoteAbstain))
	if err != nil {
		return 0, fmt.Errorf("vote: %w", err)
	}
	return Vote(vote), nil //nolint:gosec // bounded by decodeUintMax
}

func decodeCredentialVotes(value any, name string) ([]CredentialVote, error) {
	entries, ok := value.(cbor.Map)
	if !ok {
		return nil, fmt.Errorf("%s must be a map", name)
	}
	result := make([]CredentialVote, 0, min(len(entries), 1024))
	for i, entry := range entries {
		credential, err := decodeCredential(entry.Key)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", name, i, err)
		}
		vote, err := decodeVote(entry.Value)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", name, i, err)
		}
		result = append(result, CredentialVote{Credential: credential, Vote: vote})
	}
	return result, nil
}

func encodeCredentialVotes(values []CredentialVote) (cbor.Map, error) {
	result := make(cbor.Map, 0, len(values))
	for i, value := range values {
		credential, err := encodeCredential(value.Credential)
		if err != nil {
			return nil, fmt.Errorf("vote %d: %w", i, err)
		}
		if value.Vote > VoteAbstain {
			return nil, fmt.Errorf("vote %d: vote %d is invalid", i, value.Vote)
		}
		result = append(result, cbor.MapEntry{Key: credential, Value: uint64(value.Vote)})
	}
	return result, nil
}

func decodeGovProposal(value any) (GovProposal, error) {
	items, err := decodePair(value)
	if err != nil {
		return GovProposal{}, err
	}
	var result GovProposal
	if result.Order, err = decodeUint(items[0]); err != nil {
		return GovProposal{}, fmt.Errorf("proposal order: %w", err)
	}
	fields, err := decodeFields(
		items[1], "proposal", 6, 6,
		"drep_votes", "proposed_in", "expires_after",
		"committee_votes", "stake_pool_votes", "proposal_procedure",
	)
	if err != nil {
		return GovProposal{}, err
	}
	drepVotes, err := requiredField(fields, "drep_votes")
	if err != nil {
		return GovProposal{}, err
	}
	if result.DRepVotes, err = decodeCredentialVotes(drepVotes, "drep_votes"); err != nil {
		return GovProposal{}, err
	}
	if result.ProposedIn, err = decodeUintField(fields, "proposed_in"); err != nil {
		return GovProposal{}, err
	}
	if result.ExpiresAfter, err = decodeUintField(fields, "expires_after"); err != nil {
		return GovProposal{}, err
	}
	committeeVotes, err := requiredField(fields, "committee_votes")
	if err != nil {
		return GovProposal{}, err
	}
	if result.CommitteeVotes, err = decodeCredentialVotes(committeeVotes, "committee_votes"); err != nil {
		return GovProposal{}, err
	}
	poolVotes, err := requiredField(fields, "stake_pool_votes")
	if err != nil {
		return GovProposal{}, err
	}
	pools, ok := poolVotes.(cbor.Map)
	if !ok {
		return GovProposal{}, errors.New("stake_pool_votes must be a map")
	}
	result.StakePoolVotes = make([]PoolVote, 0, min(len(pools), 1024))
	for i, entry := range pools {
		pool, err := decodeHash28(entry.Key)
		if err != nil {
			return GovProposal{}, fmt.Errorf("stake_pool_votes[%d]: %w", i, err)
		}
		vote, err := decodeVote(entry.Value)
		if err != nil {
			return GovProposal{}, fmt.Errorf("stake_pool_votes[%d]: %w", i, err)
		}
		result.StakePoolVotes = append(result.StakePoolVotes, PoolVote{Pool: pool, Vote: vote})
	}
	procedure, err := requiredField(fields, "proposal_procedure")
	if err != nil {
		return GovProposal{}, err
	}
	data, err := decodeBytes(procedure, -1)
	if err != nil {
		return GovProposal{}, fmt.Errorf("proposal_procedure: %w", err)
	}
	if result.Procedure, err = decodeProposalProcedure(data); err != nil {
		return GovProposal{}, fmt.Errorf("proposal_procedure: %w", err)
	}
	return result, nil
}

func encodeGovProposal(value GovProposal) (any, error) {
	drepVotes, err := encodeCredentialVotes(value.DRepVotes)
	if err != nil {
		return nil, fmt.Errorf("drep_votes: %w", err)
	}
	committeeVotes, err := encodeCredentialVotes(value.CommitteeVotes)
	if err != nil {
		return nil, fmt.Errorf("committee_votes: %w", err)
	}
	pools := make(cbor.Map, 0, len(value.StakePoolVotes))
	for i, vote := range value.StakePoolVotes {
		if vote.Vote > VoteAbstain {
			return nil, fmt.Errorf("stake_pool_votes[%d]: vote %d is invalid", i, vote.Vote)
		}
		pools = append(pools, cbor.MapEntry{Key: bytes.Clone(vote.Pool[:]), Value: uint64(vote.Vote)})
	}
	procedure, err := encodeProposalProcedureBytes(value.Procedure)
	if err != nil {
		return nil, fmt.Errorf("proposal_procedure: %w", err)
	}
	return cbor.Array{value.Order, cbor.Map{
		{Key: "drep_votes", Value: drepVotes},
		{Key: "proposed_in", Value: value.ProposedIn},
		{Key: "expires_after", Value: value.ExpiresAfter},
		{Key: "committee_votes", Value: committeeVotes},
		{Key: "stake_pool_votes", Value: pools},
		{Key: "proposal_procedure", Value: procedure},
	}}, nil
}

func decodeProposalProcedure(data []byte) (ProposalProcedure, error) {
	value, err := cbor.UnmarshalLedger(data)
	if err != nil {
		return ProposalProcedure{}, err
	}
	items, ok := value.(cbor.Array)
	if !ok || len(items) != 4 {
		return ProposalProcedure{}, errors.New("proposal procedure must be an array of 4 items")
	}
	var result ProposalProcedure
	if result.Deposit, err = decodeUint(items[0]); err != nil {
		return ProposalProcedure{}, fmt.Errorf("deposit: %w", err)
	}
	if result.ReturnAddress, err = decodeBytes(items[1], rewardKeySize); err != nil {
		return ProposalProcedure{}, fmt.Errorf("return address: %w", err)
	}
	if result.Action, err = decodeGovAction(items[2]); err != nil {
		return ProposalProcedure{}, fmt.Errorf("gov action: %w", err)
	}
	if result.Anchor, err = decodeAnchor(items[3]); err != nil {
		return ProposalProcedure{}, err
	}
	canonical, err := encodeProposalProcedureFields(result)
	if err != nil {
		return ProposalProcedure{}, err
	}
	if !bytes.Equal(canonical, data) {
		result.Raw = bytes.Clone(data)
	}
	return result, nil
}

func encodeProposalProcedureFields(value ProposalProcedure) ([]byte, error) {
	if len(value.ReturnAddress) != rewardKeySize {
		return nil, fmt.Errorf("return address has %d bytes, want %d", len(value.ReturnAddress), rewardKeySize)
	}
	action, err := encodeGovAction(value.Action)
	if err != nil {
		return nil, fmt.Errorf("gov action: %w", err)
	}
	anchor, err := encodeAnchor(value.Anchor)
	if err != nil {
		return nil, err
	}
	return cbor.Marshal(cbor.Array{value.Deposit, bytes.Clone(value.ReturnAddress), action, anchor})
}

func encodeProposalProcedureBytes(value ProposalProcedure) ([]byte, error) {
	canonical, err := encodeProposalProcedureFields(value)
	if err != nil {
		return nil, err
	}
	if value.Raw == nil {
		return canonical, nil
	}
	decoded, err := decodeProposalProcedure(value.Raw)
	if err != nil {
		return nil, fmt.Errorf("raw procedure: %w", err)
	}
	decoded.Raw = nil
	again, err := encodeProposalProcedureFields(decoded)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(again, canonical) {
		return nil, errors.New("raw procedure does not match the procedure fields")
	}
	return bytes.Clone(value.Raw), nil
}

func decodeOptionalActionID(value any) (*GovActionID, error) {
	if value == nil {
		return nil, nil
	}
	id, err := decodeGovActionID(value)
	if err != nil {
		return nil, fmt.Errorf("purpose: %w", err)
	}
	return &id, nil
}

func optionalActionID(value *GovActionID) any {
	if value == nil {
		return nil
	}
	return cbor.Array{bytes.Clone(value.TransactionID[:]), uint64(value.Index)}
}

func decodeGovAction(value any) (GovAction, error) {
	items, ok := value.(cbor.Array)
	if !ok || len(items) == 0 {
		return GovAction{}, errors.New("gov action must be a non-empty array")
	}
	kind, err := decodeUintMax(items[0], uint64(GovInfo))
	if err != nil {
		return GovAction{}, errors.New("gov action kind must be 0..6")
	}
	result := GovAction{Kind: GovActionKind(kind)} //nolint:gosec // bounded by decodeUintMax
	arity := map[GovActionKind]int{
		GovParameterChange: 4, GovHardForkInitiation: 3, GovTreasuryWithdrawals: 3,
		GovNoConfidence: 2, GovUpdateCommittee: 5, GovNewConstitution: 3, GovInfo: 1,
	}[result.Kind]
	if len(items) != arity {
		return GovAction{}, fmt.Errorf("gov action %d has %d items, want %d", kind, len(items), arity)
	}
	if result.Kind != GovTreasuryWithdrawals && result.Kind != GovInfo {
		if result.Purpose, err = decodeOptionalActionID(items[1]); err != nil {
			return GovAction{}, err
		}
	}
	switch result.Kind {
	case GovParameterChange:
		if result.ParamUpdate, err = decodeParamUpdate(items[2]); err != nil {
			return GovAction{}, err
		}
		if result.PolicyHash, err = decodeOptionalHash28(items[3]); err != nil {
			return GovAction{}, fmt.Errorf("policy hash: %w", err)
		}
	case GovHardForkInitiation:
		if err := decodeParam(&result.ProtocolVersion, items[2]); err != nil {
			return GovAction{}, fmt.Errorf("protocol version: %w", err)
		}
	case GovTreasuryWithdrawals:
		entries, ok := items[1].(cbor.Map)
		if !ok {
			return GovAction{}, errors.New("withdrawals must be a map")
		}
		result.Withdrawals = make([]Withdrawal, 0, min(len(entries), 1024))
		for i, entry := range entries {
			account, err := decodeBytes(entry.Key, rewardKeySize)
			if err != nil {
				return GovAction{}, fmt.Errorf("withdrawal %d account: %w", i, err)
			}
			amount, err := decodeUint(entry.Value)
			if err != nil {
				return GovAction{}, fmt.Errorf("withdrawal %d amount: %w", i, err)
			}
			result.Withdrawals = append(result.Withdrawals, Withdrawal{Account: account, Amount: amount})
		}
		if result.PolicyHash, err = decodeOptionalHash28(items[2]); err != nil {
			return GovAction{}, fmt.Errorf("policy hash: %w", err)
		}
	case GovUpdateCommittee:
		set, err := decodeLooseSet(items[2])
		if err != nil {
			return GovAction{}, fmt.Errorf("removed: %w", err)
		}
		result.Removed = make([]Credential, 0, len(set))
		for i, item := range set {
			credential, err := decodeCredential(item)
			if err != nil {
				return GovAction{}, fmt.Errorf("removed[%d]: %w", i, err)
			}
			result.Removed = append(result.Removed, credential)
		}
		added, ok := items[3].(cbor.Map)
		if !ok {
			return GovAction{}, errors.New("added must be a map")
		}
		result.Added = make([]CommitteeMember, 0, min(len(added), 1024))
		for i, entry := range added {
			cold, err := decodeCredential(entry.Key)
			if err != nil {
				return GovAction{}, fmt.Errorf("added[%d]: %w", i, err)
			}
			expiry, err := decodeUint(entry.Value)
			if err != nil {
				return GovAction{}, fmt.Errorf("added[%d] expiry: %w", i, err)
			}
			result.Added = append(result.Added, CommitteeMember{Cold: cold, Expiry: expiry})
		}
		if result.Threshold, err = decodeUnitInterval(items[4]); err != nil {
			return GovAction{}, fmt.Errorf("threshold: %w", err)
		}
	case GovNewConstitution:
		if result.Constitution, err = decodeConstitution(items[2]); err != nil {
			return GovAction{}, fmt.Errorf("constitution: %w", err)
		}
	case GovNoConfidence, GovInfo:
	}
	return result, nil
}

// decodeLooseSet reads a tag 258 array, rejecting duplicates but accepting
// any order: the ledger orders sets by its own key ordering.
func decodeLooseSet(value any) (cbor.Array, error) {
	tag, ok := value.(cbor.Tag)
	if !ok || tag.Number != 258 {
		return nil, errors.New("must be tag 258")
	}
	items, ok := tag.Value.(cbor.Array)
	if !ok {
		return nil, errors.New("tag 258 value must be an array")
	}
	seen := make(map[string]struct{}, min(len(items), 1024))
	for i, item := range items {
		encoded, err := cbor.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("set item %d: %w", i, err)
		}
		if _, dup := seen[string(encoded)]; dup {
			return nil, fmt.Errorf("set item %d is duplicated", i)
		}
		seen[string(encoded)] = struct{}{}
	}
	return items, nil
}

func encodeGovAction(value GovAction) (any, error) {
	if err := requireUnusedActionFieldsZero(value); err != nil {
		return nil, err
	}
	kind := uint64(value.Kind)
	switch value.Kind {
	case GovParameterChange:
		update, err := encodeParamUpdate(value.ParamUpdate)
		if err != nil {
			return nil, err
		}
		return cbor.Array{kind, optionalActionID(value.Purpose), update, optionalHash28(value.PolicyHash)}, nil
	case GovHardForkInitiation:
		return cbor.Array{kind, optionalActionID(value.Purpose), cbor.Array{value.ProtocolVersion.Major, value.ProtocolVersion.Minor}}, nil
	case GovTreasuryWithdrawals:
		withdrawals := make(cbor.Map, 0, len(value.Withdrawals))
		for i, withdrawal := range value.Withdrawals {
			if len(withdrawal.Account) != rewardKeySize {
				return nil, fmt.Errorf("withdrawal %d account has %d bytes, want %d", i, len(withdrawal.Account), rewardKeySize)
			}
			withdrawals = append(withdrawals, cbor.MapEntry{Key: bytes.Clone(withdrawal.Account), Value: withdrawal.Amount})
		}
		return cbor.Array{kind, withdrawals, optionalHash28(value.PolicyHash)}, nil
	case GovNoConfidence:
		return cbor.Array{kind, optionalActionID(value.Purpose)}, nil
	case GovUpdateCommittee:
		removed, err := encodeCredentialSet(value.Removed)
		if err != nil {
			return nil, fmt.Errorf("removed: %w", err)
		}
		added := make(cbor.Map, 0, len(value.Added))
		for i, member := range value.Added {
			cold, err := encodeCredential(member.Cold)
			if err != nil {
				return nil, fmt.Errorf("added[%d]: %w", i, err)
			}
			added = append(added, cbor.MapEntry{Key: cold, Value: member.Expiry})
		}
		threshold, err := encodeUnitInterval(value.Threshold)
		if err != nil {
			return nil, fmt.Errorf("threshold: %w", err)
		}
		return cbor.Array{kind, optionalActionID(value.Purpose), removed, added, threshold}, nil
	case GovNewConstitution:
		constitution, err := encodeConstitution(value.Constitution)
		if err != nil {
			return nil, fmt.Errorf("constitution: %w", err)
		}
		return cbor.Array{kind, optionalActionID(value.Purpose), constitution}, nil
	case GovInfo:
		return cbor.Array{kind}, nil
	default:
		return nil, fmt.Errorf("gov action kind %d is invalid", value.Kind)
	}
}

func requireUnusedActionFieldsZero(value GovAction) error {
	rest := value
	rest.Kind = 0
	switch value.Kind {
	case GovParameterChange:
		rest.Purpose, rest.PolicyHash, rest.ParamUpdate = nil, nil, ProtocolParametersUpdate{}
	case GovHardForkInitiation:
		rest.Purpose, rest.ProtocolVersion = nil, ProtocolVersion{}
	case GovTreasuryWithdrawals:
		rest.PolicyHash, rest.Withdrawals = nil, nil
	case GovNoConfidence:
		rest.Purpose = nil
	case GovUpdateCommittee:
		rest.Purpose, rest.Removed, rest.Added, rest.Threshold = nil, nil, nil, UnitInterval{}
	case GovNewConstitution:
		rest.Purpose, rest.Constitution = nil, Constitution{}
	case GovInfo:
	default:
		return fmt.Errorf("gov action kind %d is invalid", value.Kind)
	}
	if !reflect.DeepEqual(rest, GovAction{}) {
		return fmt.Errorf("gov action kind %d carries fields it does not use", value.Kind)
	}
	return nil
}
