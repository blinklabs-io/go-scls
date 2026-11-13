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
	"math"

	"github.com/blinklabs-io/go-scls/cardano/cbor"
)

const (
	maxAnchorURL     = 128
	maxAssetNameSize = 32
	rewardKeySize    = 29
)

func decodeAnchor(value any) (Anchor, error) {
	items, err := decodePair(value)
	if err != nil {
		return Anchor{}, err
	}
	url, ok := items[0].(string)
	if !ok || len(url) > maxAnchorURL {
		return Anchor{}, fmt.Errorf("anchor URL must be text of at most %d bytes", maxAnchorURL)
	}
	hash, err := decodeBytes(items[1], -1)
	if err != nil {
		return Anchor{}, fmt.Errorf("anchor hash: %w", err)
	}
	return Anchor{URL: url, Hash: hash}, nil
}

func encodeAnchor(value Anchor) (cbor.Array, error) {
	if len(value.URL) > maxAnchorURL {
		return nil, fmt.Errorf("anchor URL has %d bytes, maximum is %d", len(value.URL), maxAnchorURL)
	}
	return cbor.Array{value.URL, bytes.Clone(value.Hash)}, nil
}

func decodeOptionalAnchor(value any) (*Anchor, error) {
	if value == nil {
		return nil, nil
	}
	anchor, err := decodeAnchor(value)
	if err != nil {
		return nil, err
	}
	return &anchor, nil
}

func encodeOptionalAnchor(value *Anchor) (any, error) {
	if value == nil {
		return nil, nil
	}
	return encodeAnchor(*value)
}

func decodeOptionalHash28(value any) (*Hash28, error) {
	if value == nil {
		return nil, nil
	}
	hash, err := decodeHash28(value)
	if err != nil {
		return nil, err
	}
	return &hash, nil
}

func optionalHash28(value *Hash28) any {
	if value == nil {
		return nil
	}
	return bytes.Clone(value[:])
}

func decodeUintMax(value any, maximum uint64) (uint64, error) {
	result, err := decodeUint(value)
	if err != nil {
		return 0, err
	}
	if result > maximum {
		return 0, fmt.Errorf("%d exceeds maximum %d", result, maximum)
	}
	return result, nil
}

func decodeRational(value any) (Rational, error) {
	tag, ok := value.(cbor.Tag)
	if !ok || tag.Number != 30 {
		return Rational{}, errors.New("must be tag 30")
	}
	items, err := decodePair(tag.Value)
	if err != nil {
		return Rational{}, err
	}
	numerator, err := decodeUint(items[0])
	if err != nil {
		return Rational{}, fmt.Errorf("numerator: %w", err)
	}
	denominator, err := decodeUint(items[1])
	if err != nil || denominator == 0 {
		return Rational{}, errors.New("denominator must be a positive unsigned integer")
	}
	return Rational{Numerator: numerator, Denominator: denominator}, nil
}

func encodeRational(value Rational) (cbor.Tag, error) {
	if value.Denominator == 0 {
		return cbor.Tag{}, errors.New("denominator must be greater than zero")
	}
	return cbor.Tag{
		Number: 30,
		Value:  cbor.Array{value.Numerator, value.Denominator},
	}, nil
}

func decodeCredentialMap(value any, name string) ([]cbor.MapEntry, error) {
	entries, ok := value.(cbor.Map)
	if !ok {
		return nil, fmt.Errorf("%s must be a map", name)
	}
	return entries, nil
}

// DRep delegation: [0, keyhash] / [1, scripthash] / [2] / [3].

func decodeDRepDelegation(value any) (*DRepDelegation, error) {
	if value == nil {
		return nil, nil
	}
	items, ok := value.(cbor.Array)
	if !ok || len(items) == 0 {
		return nil, errors.New("drep delegation must be a non-empty array")
	}
	kind, err := decodeUint(items[0])
	if err != nil || kind > uint64(DRepAlwaysNoConfidence) {
		return nil, errors.New("drep delegation kind must be 0..3")
	}
	result := DRepDelegation{Kind: DRepKind(kind)} //nolint:gosec // kind <= 3 checked above
	switch kind {
	case uint64(DRepKeyHash), uint64(DRepScriptHash):
		if len(items) != 2 {
			return nil, errors.New("drep delegation credential must have 2 items")
		}
		if result.Hash, err = decodeHash28(items[1]); err != nil {
			return nil, err
		}
	default:
		if len(items) != 1 {
			return nil, errors.New("predefined drep must have 1 item")
		}
	}
	return &result, nil
}

func encodeDRepDelegation(value *DRepDelegation) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch value.Kind {
	case DRepKeyHash, DRepScriptHash:
		return cbor.Array{uint64(value.Kind), bytes.Clone(value.Hash[:])}, nil
	case DRepAlwaysAbstain, DRepAlwaysNoConfidence:
		if value.Hash != (Hash28{}) {
			return nil, errors.New("predefined drep must not carry a hash")
		}
		return cbor.Array{uint64(value.Kind)}, nil
	default:
		return nil, fmt.Errorf("drep kind %d is invalid", value.Kind)
	}
}

func decodeAccountState(value any) (AccountState, error) {
	fields, err := decodeFields(
		value, "account_state", 4, 4,
		"balance", "deposit", "drep_delegation", "stake_pool_delegation",
	)
	if err != nil {
		return AccountState{}, err
	}
	var result AccountState
	if result.Balance, err = decodeUintField(fields, "balance"); err != nil {
		return AccountState{}, err
	}
	if result.Deposit, err = decodeUintField(fields, "deposit"); err != nil {
		return AccountState{}, err
	}
	drep, err := requiredField(fields, "drep_delegation")
	if err != nil {
		return AccountState{}, err
	}
	if result.DRepDelegation, err = decodeDRepDelegation(drep); err != nil {
		return AccountState{}, fmt.Errorf("drep_delegation: %w", err)
	}
	pool, err := requiredField(fields, "stake_pool_delegation")
	if err != nil {
		return AccountState{}, err
	}
	if result.StakePoolDelegation, err = decodeOptionalHash28(pool); err != nil {
		return AccountState{}, fmt.Errorf("stake_pool_delegation: %w", err)
	}
	return result, nil
}

func encodeAccountState(value AccountState) (any, error) {
	drep, err := encodeDRepDelegation(value.DRepDelegation)
	if err != nil {
		return nil, fmt.Errorf("drep_delegation: %w", err)
	}
	return cbor.Map{
		{Key: "balance", Value: value.Balance},
		{Key: "deposit", Value: value.Deposit},
		{Key: "drep_delegation", Value: drep},
		{Key: "stake_pool_delegation", Value: optionalHash28(value.StakePoolDelegation)},
	}, nil
}

func decodeDRepState(value any) (DRepState, error) {
	fields, err := decodeFields(
		value, "drep_state", 4, 4, "anchor", "expiry", "deposit", "delegations",
	)
	if err != nil {
		return DRepState{}, err
	}
	var result DRepState
	anchor, err := requiredField(fields, "anchor")
	if err != nil {
		return DRepState{}, err
	}
	if result.Anchor, err = decodeOptionalAnchor(anchor); err != nil {
		return DRepState{}, fmt.Errorf("anchor: %w", err)
	}
	if result.Expiry, err = decodeUintField(fields, "expiry"); err != nil {
		return DRepState{}, err
	}
	if result.Deposit, err = decodeUintField(fields, "deposit"); err != nil {
		return DRepState{}, err
	}
	if result.Delegations, err = decodeCredentialSetField(fields, "delegations"); err != nil {
		return DRepState{}, err
	}
	return result, nil
}

func encodeDRepState(value DRepState) (any, error) {
	anchor, err := encodeOptionalAnchor(value.Anchor)
	if err != nil {
		return nil, fmt.Errorf("anchor: %w", err)
	}
	delegations, err := encodeCredentialSet(value.Delegations)
	if err != nil {
		return nil, fmt.Errorf("delegations: %w", err)
	}
	return cbor.Map{
		{Key: "anchor", Value: anchor},
		{Key: "expiry", Value: value.Expiry},
		{Key: "deposit", Value: value.Deposit},
		{Key: "delegations", Value: delegations},
	}, nil
}

func decodeCommitteeAuthorization(value any) (CommitteeAuthorization, error) {
	items, err := decodePair(value)
	if err != nil {
		return CommitteeAuthorization{}, err
	}
	kind, err := decodeUint(items[0])
	if err != nil {
		return CommitteeAuthorization{}, errors.New("authorization kind must be an unsigned integer")
	}
	switch kind {
	case uint64(CommitteeHotCredential):
		hot, err := decodeCredential(items[1])
		if err != nil {
			return CommitteeAuthorization{}, fmt.Errorf("hot credential: %w", err)
		}
		return CommitteeAuthorization{Kind: CommitteeHotCredential, HotCredential: hot}, nil
	case uint64(CommitteeResigned):
		anchor, err := decodeOptionalAnchor(items[1])
		if err != nil {
			return CommitteeAuthorization{}, fmt.Errorf("resignation anchor: %w", err)
		}
		return CommitteeAuthorization{Kind: CommitteeResigned, Anchor: anchor}, nil
	default:
		return CommitteeAuthorization{}, fmt.Errorf("authorization kind %d is invalid", kind)
	}
}

func encodeCommitteeAuthorization(value CommitteeAuthorization) (any, error) {
	switch value.Kind {
	case CommitteeHotCredential:
		if value.Anchor != nil {
			return nil, errors.New("hot credential authorization must not carry an anchor")
		}
		hot, err := encodeCredential(value.HotCredential)
		if err != nil {
			return nil, err
		}
		return cbor.Array{uint64(value.Kind), hot}, nil
	case CommitteeResigned:
		if value.HotCredential != (Credential{}) {
			return nil, errors.New("resignation must not carry a hot credential")
		}
		anchor, err := encodeOptionalAnchor(value.Anchor)
		if err != nil {
			return nil, err
		}
		return cbor.Array{uint64(value.Kind), anchor}, nil
	default:
		return nil, fmt.Errorf("authorization kind %d is invalid", value.Kind)
	}
}

func decodeCommitteeState(value any) (CommitteeState, error) {
	entries, err := decodeCredentialMap(value, "committee_state")
	if err != nil {
		return nil, err
	}
	result := make(CommitteeState, 0, min(len(entries), 1024))
	for i, entry := range entries {
		cold, err := decodeCredential(entry.Key)
		if err != nil {
			return nil, fmt.Errorf("member %d cold credential: %w", i, err)
		}
		authorization, err := decodeCommitteeAuthorization(entry.Value)
		if err != nil {
			return nil, fmt.Errorf("member %d: %w", i, err)
		}
		result = append(result, CommitteeMemberState{Cold: cold, Authorization: authorization})
	}
	return result, nil
}

func encodeCommitteeState(value CommitteeState) (any, error) {
	result := make(cbor.Map, 0, len(value))
	for i, member := range value {
		cold, err := encodeCredential(member.Cold)
		if err != nil {
			return nil, fmt.Errorf("member %d cold credential: %w", i, err)
		}
		authorization, err := encodeCommitteeAuthorization(member.Authorization)
		if err != nil {
			return nil, fmt.Errorf("member %d: %w", i, err)
		}
		result = append(result, cbor.MapEntry{Key: cold, Value: authorization})
	}
	return result, nil
}

func decodeGovCommittee(value any) (*GovCommittee, error) {
	if value == nil {
		return nil, nil
	}
	items, err := decodePair(value)
	if err != nil {
		return nil, err
	}
	entries, err := decodeCredentialMap(items[0], "members")
	if err != nil {
		return nil, err
	}
	result := GovCommittee{Members: make([]CommitteeMember, 0, min(len(entries), 1024))}
	for i, entry := range entries {
		cold, err := decodeCredential(entry.Key)
		if err != nil {
			return nil, fmt.Errorf("member %d: %w", i, err)
		}
		expiry, err := decodeUint(entry.Value)
		if err != nil {
			return nil, fmt.Errorf("member %d expiry: %w", i, err)
		}
		result.Members = append(result.Members, CommitteeMember{Cold: cold, Expiry: expiry})
	}
	if result.Threshold, err = decodeUnitInterval(items[1]); err != nil {
		return nil, fmt.Errorf("threshold: %w", err)
	}
	return &result, nil
}

func encodeGovCommittee(value *GovCommittee) (any, error) {
	if value == nil {
		return nil, nil
	}
	members := make(cbor.Map, 0, len(value.Members))
	for i, member := range value.Members {
		cold, err := encodeCredential(member.Cold)
		if err != nil {
			return nil, fmt.Errorf("member %d: %w", i, err)
		}
		members = append(members, cbor.MapEntry{Key: cold, Value: member.Expiry})
	}
	threshold, err := encodeUnitInterval(value.Threshold)
	if err != nil {
		return nil, fmt.Errorf("threshold: %w", err)
	}
	return cbor.Array{members, threshold}, nil
}

func decodeConstitution(value any) (Constitution, error) {
	items, err := decodePair(value)
	if err != nil {
		return Constitution{}, err
	}
	anchor, err := decodeAnchor(items[0])
	if err != nil {
		return Constitution{}, err
	}
	script, err := decodeOptionalHash28(items[1])
	if err != nil {
		return Constitution{}, fmt.Errorf("script hash: %w", err)
	}
	return Constitution{Anchor: anchor, ScriptHash: script}, nil
}

func encodeConstitution(value Constitution) (any, error) {
	anchor, err := encodeAnchor(value.Anchor)
	if err != nil {
		return nil, err
	}
	return cbor.Array{anchor, optionalHash28(value.ScriptHash)}, nil
}

func decodeGovActionID(value any) (GovActionID, error) {
	items, err := decodePair(value)
	if err != nil {
		return GovActionID{}, err
	}
	data, err := decodeBytes(items[0], 32)
	if err != nil {
		return GovActionID{}, fmt.Errorf("transaction id: %w", err)
	}
	index, err := decodeUintMax(items[1], math.MaxUint16)
	if err != nil {
		return GovActionID{}, fmt.Errorf("action index: %w", err)
	}
	result := GovActionID{Index: uint16(index)} //nolint:gosec // bounded by decodeUintMax
	copy(result.TransactionID[:], data)
	return result, nil
}

func encodeGovActionID(value GovActionID) (any, error) {
	return cbor.Array{bytes.Clone(value.TransactionID[:]), uint64(value.Index)}, nil
}

func decodeBlocksMade(value any) (BlocksMade, error) {
	result, err := decodeUint(value)
	return BlocksMade(result), err
}

func encodeBlocksMade(value BlocksMade) (any, error) {
	return uint64(value), nil
}

func decodeVRFKeyHashCount(value any) (VRFKeyHashCount, error) {
	result, err := decodeUint(value)
	if err != nil {
		return 0, err
	}
	if result == 0 {
		return 0, errors.New("must be positive")
	}
	return VRFKeyHashCount(result), nil
}

func encodeVRFKeyHashCount(value VRFKeyHashCount) (any, error) {
	if value == 0 {
		return nil, errors.New("must be positive")
	}
	return uint64(value), nil
}

func decodeSnapshotOut(value any) (SnapshotOut, error) {
	items, err := decodePair(value)
	if err != nil {
		return SnapshotOut{}, err
	}
	kind, err := decodeUint(items[0])
	if err != nil {
		return SnapshotOut{}, errors.New("snapshot kind must be an unsigned integer")
	}
	switch kind {
	case uint64(SnapshotCoin):
		coin, err := decodeUint(items[1])
		if err != nil {
			return SnapshotOut{}, fmt.Errorf("coin: %w", err)
		}
		return SnapshotOut{Kind: SnapshotCoin, Coin: coin}, nil
	case uint64(SnapshotPool):
		pool, err := decodeHash28(items[1])
		if err != nil {
			return SnapshotOut{}, fmt.Errorf("pool: %w", err)
		}
		return SnapshotOut{Kind: SnapshotPool, Pool: pool}, nil
	default:
		return SnapshotOut{}, fmt.Errorf("snapshot kind %d is invalid", kind)
	}
}

func encodeSnapshotOut(value SnapshotOut) (any, error) {
	switch value.Kind {
	case SnapshotCoin:
		if value.Pool != (Hash28{}) {
			return nil, errors.New("coin snapshot must not carry a pool")
		}
		return cbor.Array{uint64(value.Kind), value.Coin}, nil
	case SnapshotPool:
		if value.Coin != 0 {
			return nil, errors.New("pool snapshot must not carry a coin")
		}
		return cbor.Array{uint64(value.Kind), bytes.Clone(value.Pool[:])}, nil
	default:
		return nil, fmt.Errorf("snapshot kind %d is invalid", value.Kind)
	}
}

// Nonces.

func decodeNonce(value any) (Nonce, error) {
	items, ok := value.(cbor.Array)
	if !ok || len(items) == 0 {
		return Nonce{}, errors.New("nonce must be a non-empty array")
	}
	kind, err := decodeUint(items[0])
	if err != nil {
		return Nonce{}, errors.New("nonce kind must be an unsigned integer")
	}
	switch kind {
	case 0:
		if len(items) != 1 {
			return Nonce{}, errors.New("neutral nonce must have 1 item")
		}
		return Nonce{}, nil
	case 1:
		if len(items) != 2 {
			return Nonce{}, errors.New("nonce must have 2 items")
		}
		data, err := decodeBytes(items[1], 32)
		if err != nil {
			return Nonce{}, err
		}
		var hash Hash32
		copy(hash[:], data)
		return Nonce{Hash: &hash}, nil
	default:
		return Nonce{}, fmt.Errorf("nonce kind %d is invalid", kind)
	}
}

func encodeNonce(value Nonce) cbor.Array {
	if value.Hash == nil {
		return cbor.Array{uint64(0)}
	}
	return cbor.Array{uint64(1), bytes.Clone(value.Hash[:])}
}

func decodeNonces(value any) (Nonces, error) {
	fields, err := decodeFields(
		value, "nonces", 7, 7,
		"lab_nonce", "last_slot", "epoch_nonce", "cert_counters",
		"evolving_nonce", "candidate_nonce", "last_epoch_block_nonce",
	)
	if err != nil {
		return Nonces{}, err
	}
	var result Nonces
	nonceTargets := []struct {
		name   string
		target *Nonce
	}{
		{"lab_nonce", &result.LabNonce},
		{"epoch_nonce", &result.EpochNonce},
		{"evolving_nonce", &result.EvolvingNonce},
		{"candidate_nonce", &result.CandidateNonce},
		{"last_epoch_block_nonce", &result.LastEpochBlockNonce},
	}
	for _, item := range nonceTargets {
		raw, err := requiredField(fields, item.name)
		if err != nil {
			return Nonces{}, err
		}
		if *item.target, err = decodeNonce(raw); err != nil {
			return Nonces{}, fmt.Errorf("%s: %w", item.name, err)
		}
	}
	slot, err := requiredField(fields, "last_slot")
	if err != nil {
		return Nonces{}, err
	}
	if result.LastSlot, err = decodeWithOriginSlot(slot); err != nil {
		return Nonces{}, fmt.Errorf("last_slot: %w", err)
	}
	counters, err := requiredField(fields, "cert_counters")
	if err != nil {
		return Nonces{}, err
	}
	entries, ok := counters.(cbor.Map)
	if !ok {
		return Nonces{}, errors.New("cert_counters must be a map")
	}
	result.CertCounters = make(map[Hash28]uint64, min(len(entries), 1024))
	for i, entry := range entries {
		pool, err := decodeHash28(entry.Key)
		if err != nil {
			return Nonces{}, fmt.Errorf("cert_counters[%d] key: %w", i, err)
		}
		count, err := decodeUint(entry.Value)
		if err != nil {
			return Nonces{}, fmt.Errorf("cert_counters[%d]: %w", i, err)
		}
		result.CertCounters[pool] = count
	}
	return result, nil
}

func decodeWithOriginSlot(value any) (*uint64, error) {
	items, ok := value.(cbor.Array)
	if !ok || len(items) == 0 {
		return nil, errors.New("with-origin must be a non-empty array")
	}
	kind, err := decodeUint(items[0])
	if err != nil {
		return nil, errors.New("with-origin kind must be an unsigned integer")
	}
	switch kind {
	case 0:
		if len(items) != 1 {
			return nil, errors.New("origin must have 1 item")
		}
		return nil, nil
	case 1:
		if len(items) != 2 {
			return nil, errors.New("at-slot must have 2 items")
		}
		slot, err := decodeUint(items[1])
		if err != nil {
			return nil, err
		}
		return &slot, nil
	default:
		return nil, fmt.Errorf("with-origin kind %d is invalid", kind)
	}
}

func encodeNonces(value Nonces) (any, error) {
	slot := cbor.Array{uint64(0)}
	if value.LastSlot != nil {
		slot = cbor.Array{uint64(1), *value.LastSlot}
	}
	counters := make(cbor.Map, 0, len(value.CertCounters))
	for pool, count := range value.CertCounters {
		counters = append(counters, cbor.MapEntry{Key: bytes.Clone(pool[:]), Value: count})
	}
	return cbor.Map{
		{Key: "lab_nonce", Value: encodeNonce(value.LabNonce)},
		{Key: "last_slot", Value: slot},
		{Key: "epoch_nonce", Value: encodeNonce(value.EpochNonce)},
		{Key: "cert_counters", Value: counters},
		{Key: "evolving_nonce", Value: encodeNonce(value.EvolvingNonce)},
		{Key: "candidate_nonce", Value: encodeNonce(value.CandidateNonce)},
		{Key: "last_epoch_block_nonce", Value: encodeNonce(value.LastEpochBlockNonce)},
	}, nil
}
