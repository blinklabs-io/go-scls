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

package cardano_test

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/blinklabs-io/go-scls/cardano"
	"github.com/blinklabs-io/go-scls/cardano/cbor"
)

func rewardAccount(start byte) []byte {
	return seq(start, 29)
}

func baseProposal(action cardano.GovAction) cardano.GovProposal {
	return cardano.GovProposal{
		Order:      7,
		ProposedIn: 3,
		DRepVotes: []cardano.CredentialVote{
			{Credential: cardano.Credential{Kind: cardano.CredentialScriptHash, Hash: hash28From(2)}, Vote: cardano.VoteYes},
			{Credential: cardano.Credential{Kind: cardano.CredentialKeyHash, Hash: hash28From(1)}, Vote: cardano.VoteAbstain},
		},
		ExpiresAfter: 9,
		CommitteeVotes: []cardano.CredentialVote{
			{Credential: cardano.Credential{Kind: cardano.CredentialKeyHash, Hash: hash28From(5)}, Vote: cardano.VoteNo},
		},
		StakePoolVotes: []cardano.PoolVote{
			{Pool: hash28From(9), Vote: cardano.VoteYes},
			{Pool: hash28From(4), Vote: cardano.VoteNo},
		},
		Procedure: cardano.ProposalProcedure{
			Deposit:       100_000_000_000,
			ReturnAddress: rewardAccount(0xe0),
			Action:        action,
			Anchor:        cardano.Anchor{URL: "https://example.invalid/p", Hash: seq(7, 32)},
		},
	}
}

func fullUpdate() cardano.ProtocolParametersUpdate {
	unit := cardano.UnitInterval{Numerator: 1, Denominator: 3}
	ratio := cardano.Rational{Numerator: 7, Denominator: 5}
	var pool [5]cardano.UnitInterval
	var drep [10]cardano.UnitInterval
	for i := range pool {
		pool[i] = unit
	}
	for i := range drep {
		drep[i] = unit
	}
	costs := map[uint8][]int64{1: {-5, 6}, 0: {}}
	return cardano.ProtocolParametersUpdate{
		MinFeeA: ptr(uint64(1)), MinFeeB: ptr(uint64(2)),
		MaxBlockSize: ptr(uint32(3)), MaxTxSize: ptr(uint32(4)), MaxBlockHeaderSize: ptr(uint16(5)),
		KeyDeposit: ptr(uint64(6)), PoolDeposit: ptr(uint64(7)), MaxEpoch: ptr(uint32(8)), NOpt: ptr(uint16(9)),
		A0: &ratio, Rho: &unit, Tau: &unit, MinPoolCost: ptr(uint64(10)), AdaPerUTxOByte: ptr(uint64(11)),
		CostModels:   &costs,
		ExUnitPrices: &cardano.ExUnitPrices{Memory: ratio, Step: ratio},
		MaxTxExUnits: &cardano.ExUnits{Memory: 1, Steps: 2}, MaxBlockExUnits: &cardano.ExUnits{Memory: 3, Steps: 4},
		MaxValueSize: ptr(uint32(12)), CollateralPercentage: ptr(uint16(13)), MaxCollateralInputs: ptr(uint16(14)),
		PoolVotingThresholds: &pool, DRepVotingThresholds: &drep,
		MinCommitteeSize: ptr(uint16(15)), CommitteeTermLimit: ptr(uint32(16)), GovActionValidity: ptr(uint32(17)),
		GovActionDeposit: ptr(uint64(18)), DRepDeposit: ptr(uint64(19)), DRepInactivity: ptr(uint32(20)),
		MinFeeRefScriptCostPerByte: &ratio,
	}
}

func govActionVariants() map[string]cardano.GovAction {
	purpose := &cardano.GovActionID{TransactionID: hash32From(0x30), Index: 4}
	policy := hash28From(0x50)
	return map[string]cardano.GovAction{
		"parameter change": {
			Kind: cardano.GovParameterChange, Purpose: purpose,
			ParamUpdate: fullUpdate(), PolicyHash: &policy,
		},
		"parameter change minimal": {
			Kind: cardano.GovParameterChange,
			ParamUpdate: cardano.ProtocolParametersUpdate{
				MinFeeA: ptr(uint64(0)),
			},
		},
		"hard fork": {
			Kind: cardano.GovHardForkInitiation, Purpose: purpose,
			ProtocolVersion: cardano.ProtocolVersion{Major: 11, Minor: 1},
		},
		"treasury withdrawals": {
			Kind: cardano.GovTreasuryWithdrawals, PolicyHash: &policy,
			Withdrawals: []cardano.Withdrawal{
				{Account: rewardAccount(0xf0), Amount: 5},
				{Account: rewardAccount(0xe0), Amount: 6},
			},
		},
		"no confidence": {Kind: cardano.GovNoConfidence, Purpose: purpose},
		"update committee": {
			Kind: cardano.GovUpdateCommittee, Purpose: purpose,
			Removed: []cardano.Credential{
				{Kind: cardano.CredentialScriptHash, Hash: hash28From(1)},
				{Kind: cardano.CredentialKeyHash, Hash: hash28From(2)},
			},
			Added: []cardano.CommitteeMember{
				{Cold: cardano.Credential{Kind: cardano.CredentialKeyHash, Hash: hash28From(3)}, Expiry: 40},
			},
			Threshold: cardano.UnitInterval{Numerator: 2, Denominator: 3},
		},
		"new constitution": {
			Kind: cardano.GovNewConstitution, Purpose: purpose,
			Constitution: cardano.Constitution{
				Anchor:     cardano.Anchor{URL: "ipfs://c", Hash: seq(1, 32)},
				ScriptHash: &policy,
			},
		},
		"info": {Kind: cardano.GovInfo},
	}
}

func canonicalOrder(t *testing.T, value cardano.GovProposal) cardano.GovProposal {
	t.Helper()
	encoded, err := cardano.EncodeGovProposal(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := cardano.DecodeGovProposal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestGovProposalActionVariants(t *testing.T) {
	t.Parallel()
	for name, action := range govActionVariants() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			value := baseProposal(action)
			encoded, err := cardano.EncodeGovProposal(value)
			if err != nil {
				t.Fatalf("EncodeGovProposal() error = %v", err)
			}
			decoded, err := cardano.DecodeGovProposal(encoded)
			if err != nil {
				t.Fatalf("DecodeGovProposal() error = %v", err)
			}
			if decoded.Procedure.Raw != nil {
				t.Fatalf("canonical encoding produced Raw = %x", decoded.Procedure.Raw)
			}
			again, err := cardano.EncodeGovProposal(decoded)
			if err != nil || !bytes.Equal(again, encoded) {
				t.Fatalf("re-encode = %x (%v), want %x", again, err, encoded)
			}
			// Unordered collections come back in deterministic order, so a
			// second pass over the decoded value is a fixed point.
			if want := canonicalOrder(t, decoded); !reflect.DeepEqual(decoded, want) {
				t.Fatalf("decoded value is not a fixed point:\n%#v\n%#v", decoded, want)
			}
			if decoded.Procedure.Action.Kind != action.Kind {
				t.Fatalf("kind = %d, want %d", decoded.Procedure.Action.Kind, action.Kind)
			}
			if action.Kind == cardano.GovParameterChange &&
				!reflect.DeepEqual(decoded.Procedure.Action.ParamUpdate, action.ParamUpdate) {
				t.Fatalf("update = %#v, want %#v", decoded.Procedure.Action.ParamUpdate, action.ParamUpdate)
			}
		})
	}
}

func TestGovProposalInfoLayout(t *testing.T) {
	t.Parallel()
	value := cardano.GovProposal{Procedure: cardano.ProposalProcedure{
		ReturnAddress: rewardAccount(0xe0),
		Action:        cardano.GovAction{Kind: cardano.GovInfo},
		Anchor:        cardano.Anchor{Hash: []byte{}},
	}}
	encoded, err := cardano.EncodeGovProposal(value)
	if err != nil {
		t.Fatal(err)
	}
	procedure := marshalOrFatal(t, cbor.Array{
		uint64(0), rewardAccount(0xe0), cbor.Array{uint64(6)}, cbor.Array{"", []byte{}},
	})
	if !bytes.Contains(encoded, append([]byte{0x58, byte(len(procedure))}, procedure...)) {
		t.Fatalf("encoding %x does not embed procedure %x as a byte string", encoded, procedure)
	}
	decoded, err := cardano.DecodeGovProposal(encoded)
	if err != nil || decoded.Procedure.Action.Kind != cardano.GovInfo {
		t.Fatalf("decoded = %#v (%v)", decoded, err)
	}
}

func marshalOrFatal(t *testing.T, value any) []byte {
	t.Helper()
	data, err := cbor.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// proposalWithProcedure wraps raw procedure bytes in an otherwise valid
// gov/proposals/v0 payload.
func proposalWithProcedure(t *testing.T, procedure []byte) []byte {
	t.Helper()
	return marshalOrFatal(t, cbor.Array{uint64(0), cbor.Map{
		{Key: "drep_votes", Value: cbor.Map{}},
		{Key: "proposed_in", Value: uint64(0)},
		{Key: "expires_after", Value: uint64(0)},
		{Key: "committee_votes", Value: cbor.Map{}},
		{Key: "stake_pool_votes", Value: cbor.Map{}},
		{Key: "proposal_procedure", Value: procedure},
	}})
}

func TestGovProposalPreservesLedgerOrderedProcedure(t *testing.T) {
	t.Parallel()
	anchor := cbor.Array{"u", []byte{1}}
	script := cbor.Array{uint64(1), seq(0, 28)}
	key := cbor.Array{uint64(0), seq(0, 28)}
	// Ledger order puts script credentials before key credentials, the
	// reverse of deterministic order; withdrawals likewise put the script
	// reward account first.
	ledgerOrdered := marshalOrFatalUnsorted(t,
		0x84,
		marshalOrFatal(t, uint64(5)),
		marshalOrFatal(t, rewardAccount(0xe0)),
		marshalOrFatal(t, cbor.Array{uint64(4), nil,
			cbor.Tag{Number: 258, Value: cbor.Array{script, key}},
			cbor.Map{}, cbor.Tag{Number: 30, Value: cbor.Array{uint64(1), uint64(2)}}}),
		marshalOrFatal(t, anchor),
	)
	data := proposalWithProcedure(t, ledgerOrdered)
	decoded, err := cardano.DecodeGovProposal(data)
	if err != nil {
		t.Fatalf("DecodeGovProposal(ledger order) error = %v", err)
	}
	if !bytes.Equal(decoded.Procedure.Raw, ledgerOrdered) {
		t.Fatalf("Raw = %x, want %x", decoded.Procedure.Raw, ledgerOrdered)
	}
	if got := decoded.Procedure.Action.Removed; len(got) != 2 || got[0].Kind != cardano.CredentialScriptHash {
		t.Fatalf("Removed = %#v, want ledger order preserved", got)
	}
	again, err := cardano.EncodeGovProposal(decoded)
	if err != nil || !bytes.Equal(again, data) {
		t.Fatalf("re-encode = %x (%v), want original %x", again, err, data)
	}

	tampered := decoded
	tampered.Procedure.Deposit++
	if _, err := cardano.EncodeGovProposal(tampered); !errors.Is(err, cardano.ErrPayload) {
		t.Fatalf("EncodeGovProposal(Raw disagrees with fields) error = %v, want ErrPayload", err)
	}
	cleared := decoded
	cleared.Procedure.Raw = nil
	canonical, err := cardano.EncodeGovProposal(cleared)
	if err != nil || bytes.Equal(canonical, data) {
		t.Fatalf("without Raw the encoding must be deterministic order: %x (%v)", canonical, err)
	}
}

// marshalOrFatalUnsorted builds an array of already-encoded items.
func marshalOrFatalUnsorted(t *testing.T, header byte, items ...[]byte) []byte {
	t.Helper()
	result := []byte{header}
	for _, item := range items {
		result = append(result, item...)
	}
	return result
}

func TestGovProposalRejectsInvalid(t *testing.T) {
	t.Parallel()
	anchor := cbor.Array{"u", []byte{1}}
	addr := rewardAccount(0xe0)
	procedure := func(action any) []byte {
		return marshalOrFatal(t, cbor.Array{uint64(0), addr, action, anchor})
	}
	field := func(name string, value any) []byte {
		valid := marshalOrFatal(t, cbor.Array{uint64(0), cbor.Map{
			{Key: "drep_votes", Value: cbor.Map{}},
			{Key: "proposed_in", Value: uint64(0)},
			{Key: "expires_after", Value: uint64(0)},
			{Key: "committee_votes", Value: cbor.Map{}},
			{Key: "stake_pool_votes", Value: cbor.Map{}},
			{Key: "proposal_procedure", Value: procedure(cbor.Array{uint64(6)})},
		}})
		generic, err := cbor.Unmarshal(valid)
		if err != nil {
			t.Fatal(err)
		}
		fields := generic.(cbor.Array)[1].(cbor.Map)
		for i := range fields {
			if fields[i].Key == name {
				fields[i].Value = value
			}
		}
		return marshalOrFatal(t, cbor.Array{uint64(0), fields})
	}
	badUpdate := func(entries cbor.Map) []byte {
		return procedure(cbor.Array{uint64(0), nil, entries, nil})
	}
	unit := cbor.Tag{Number: 30, Value: cbor.Array{uint64(1), uint64(2)}}
	voteCred := cbor.Array{uint64(0), seq(0, 28)}
	tests := map[string][]byte{
		"procedure not bytes":      field("proposal_procedure", uint64(1)),
		"procedure trailing":       proposalWithProcedure(t, append(procedure(cbor.Array{uint64(6)}), 0)),
		"procedure wrong arity":    proposalWithProcedure(t, marshalOrFatal(t, cbor.Array{uint64(0), addr, cbor.Array{uint64(6)}})),
		"reward account 28":        proposalWithProcedure(t, marshalOrFatal(t, cbor.Array{uint64(0), seq(0, 28), cbor.Array{uint64(6)}, anchor})),
		"action kind 7":            proposalWithProcedure(t, procedure(cbor.Array{uint64(7)})),
		"info with extra":          proposalWithProcedure(t, procedure(cbor.Array{uint64(6), nil})),
		"no confidence no purpose": proposalWithProcedure(t, procedure(cbor.Array{uint64(3)})),
		"purpose index 65536":      proposalWithProcedure(t, procedure(cbor.Array{uint64(3), cbor.Array{seq(0, 32), uint64(65536)}})),
		"hard fork no version":     proposalWithProcedure(t, procedure(cbor.Array{uint64(1), nil})),
		"withdrawal bad account":   proposalWithProcedure(t, procedure(cbor.Array{uint64(2), cbor.Map{{Key: seq(0, 28), Value: uint64(1)}}, nil})),
		"committee threshold 2/1":  proposalWithProcedure(t, procedure(cbor.Array{uint64(4), nil, cbor.Tag{Number: 258, Value: cbor.Array{}}, cbor.Map{}, cbor.Tag{Number: 30, Value: cbor.Array{uint64(2), uint64(1)}}})),
		"removed not a set":        proposalWithProcedure(t, procedure(cbor.Array{uint64(4), nil, cbor.Array{}, cbor.Map{}, unit})),
		"update protocol version":  proposalWithProcedure(t, badUpdate(cbor.Map{{Key: uint64(14), Value: cbor.Array{uint64(1), uint64(0)}}})),
		"update unknown key":       proposalWithProcedure(t, badUpdate(cbor.Map{{Key: uint64(12), Value: uint64(1)}})),
		"update u16 overflow":      proposalWithProcedure(t, badUpdate(cbor.Map{{Key: uint64(4), Value: uint64(65536)}})),
		"update zero denominator":  proposalWithProcedure(t, badUpdate(cbor.Map{{Key: uint64(9), Value: cbor.Tag{Number: 30, Value: cbor.Array{uint64(1), uint64(0)}}}})),
		"update duplicate key":     proposalWithProcedure(t, duplicateKeyProcedure(addr)),
		"vote 3":                   field("drep_votes", cbor.Map{{Key: voteCred, Value: uint64(3)}}),
		"drep vote bad credential": field("drep_votes", cbor.Map{{Key: cbor.Array{uint64(2), seq(0, 28)}, Value: uint64(0)}}),
		"pool vote key 29":         field("stake_pool_votes", cbor.Map{{Key: seq(0, 29), Value: uint64(0)}}),
		"committee vote not map":   field("committee_votes", cbor.Array{}),
		"proposed_in negative":     field("proposed_in", cbor.Negative(0)),
		"order not uint":           marshalOrFatal(t, cbor.Array{"x", cbor.Map{}}),
		"entry arity 3":            marshalOrFatal(t, cbor.Array{uint64(0), cbor.Map{}, uint64(0)}),
		"missing field":            marshalOrFatal(t, cbor.Array{uint64(0), cbor.Map{{Key: "proposed_in", Value: uint64(0)}}}),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := cardano.DecodeGovProposal(data); !errors.Is(err, cardano.ErrPayload) {
				t.Fatalf("DecodeGovProposal() error = %v, want ErrPayload", err)
			}
		})
	}
	control := proposalWithProcedure(t, procedure(cbor.Array{uint64(6)}))
	if _, err := cardano.DecodeGovProposal(control); err != nil {
		t.Fatalf("control payload error = %v", err)
	}
}

func TestGovProposalRejectsInvalidEncode(t *testing.T) {
	t.Parallel()
	good := baseProposal(cardano.GovAction{Kind: cardano.GovInfo})
	mutations := map[string]func(*cardano.GovProposal){
		"reward account size": func(p *cardano.GovProposal) { p.Procedure.ReturnAddress = seq(0, 28) },
		"vote 3":              func(p *cardano.GovProposal) { p.DRepVotes[0].Vote = 3 },
		"action kind 9":       func(p *cardano.GovProposal) { p.Procedure.Action.Kind = 9 },
		"info with purpose": func(p *cardano.GovProposal) {
			p.Procedure.Action.Purpose = &cardano.GovActionID{}
		},
		"info with withdrawals": func(p *cardano.GovProposal) {
			p.Procedure.Action.Withdrawals = []cardano.Withdrawal{{Account: rewardAccount(0), Amount: 1}}
		},
		"duplicate pool vote": func(p *cardano.GovProposal) { p.StakePoolVotes[1].Pool = p.StakePoolVotes[0].Pool },
		"long anchor URL":     func(p *cardano.GovProposal) { p.Procedure.Anchor.URL = string(make([]byte, 129)) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			value := baseProposal(cardano.GovAction{Kind: cardano.GovInfo})
			mutate(&value)
			if _, err := cardano.EncodeGovProposal(value); !errors.Is(err, cardano.ErrPayload) {
				t.Fatalf("EncodeGovProposal() error = %v, want ErrPayload", err)
			}
		})
	}
	if _, err := cardano.EncodeGovProposal(good); err != nil {
		t.Fatalf("control error = %v", err)
	}
}

// duplicateKeyProcedure is a parameter change whose update map repeats key 0.
func duplicateKeyProcedure(addr []byte) []byte {
	result := []byte{0x84, 0x00, 0x58, 0x1d}
	result = append(result, addr...)
	result = append(result, 0x84, 0x00, 0xf6, 0xa2, 0x00, 0x01, 0x00, 0x02, 0xf6)
	return append(result, 0x82, 0x60, 0x40)
}

func TestTxOutPreservesLedgerSerialization(t *testing.T) {
	t.Parallel()
	policy := seq(0, 28)
	// Ledger order sorts asset names lexicographically, so [0 0] precedes
	// [0xff]; deterministic order puts the shorter encoded key first.
	inner := marshalOrFatalUnsorted(t, 0x82,
		marshalOrFatal(t, seq(1, 29)),
		marshalOrFatalUnsorted(t, 0x82,
			[]byte{0x01},
			marshalOrFatalUnsorted(t, 0xa1,
				marshalOrFatal(t, policy),
				marshalOrFatalUnsorted(t, 0xa2,
					marshalOrFatal(t, []byte{0, 0}), []byte{0x01},
					marshalOrFatal(t, []byte{0xff}), []byte{0x02}),
			)),
	)
	data := marshalOrFatal(t, inner)
	decoded, err := cardano.DecodeTxOut(data)
	if err != nil {
		t.Fatalf("DecodeTxOut(ledger order) error = %v", err)
	}
	if !bytes.Equal(decoded.Raw, inner) {
		t.Fatalf("Raw = %x, want %x", decoded.Raw, inner)
	}
	if got := (*decoded.Value.Assets)[0].Assets[0].Name; !bytes.Equal(got, []byte{0, 0}) {
		t.Fatalf("first asset = %x, want ledger order preserved", got)
	}
	again, err := cardano.EncodeTxOut(decoded)
	if err != nil || !bytes.Equal(again, data) {
		t.Fatalf("re-encode = %x (%v), want %x", again, err, data)
	}
	tampered := decoded
	tampered.Value.Coin++
	if _, err := cardano.EncodeTxOut(tampered); !errors.Is(err, cardano.ErrPayload) {
		t.Fatalf("EncodeTxOut(Raw disagrees) error = %v, want ErrPayload", err)
	}
	cleared := decoded
	cleared.Raw = nil
	canonical, err := cardano.EncodeTxOut(cleared)
	if err != nil || bytes.Equal(canonical, data) {
		t.Fatalf("without Raw the encoding must be deterministic: %x (%v)", canonical, err)
	}
	duplicate := marshalOrFatalUnsorted(t, 0x82,
		marshalOrFatal(t, seq(1, 29)),
		marshalOrFatalUnsorted(t, 0x82, []byte{0x01},
			marshalOrFatalUnsorted(t, 0xa2,
				marshalOrFatal(t, policy), marshalOrFatalUnsorted(t, 0xa1, []byte{0x40}, []byte{0x01}),
				marshalOrFatal(t, policy), marshalOrFatalUnsorted(t, 0xa1, []byte{0x40}, []byte{0x01}))))
	if _, err := cardano.DecodeTxOut(marshalOrFatal(t, duplicate)); !errors.Is(err, cardano.ErrPayload) {
		t.Fatalf("DecodeTxOut(duplicate policy) error = %v, want ErrPayload", err)
	}
}

func TestTxOutKeepsInlineDatumBytesOpaque(t *testing.T) {
	t.Parallel()
	// An indefinite-length list is valid Plutus data as stored by the ledger
	// but is not deterministic CBOR.
	datum := []byte{0x9f, 0x01, 0xff}
	value := cardano.TxOut{
		Format: cardano.TxOutBabbage, Address: seq(1, 29), Value: cardano.TxValue{Coin: 1},
		Datum: &cardano.TxOutDatum{Inline: datum},
	}
	encoded, err := cardano.EncodeTxOut(value)
	if err != nil {
		t.Fatalf("EncodeTxOut() error = %v", err)
	}
	decoded, err := cardano.DecodeTxOut(encoded)
	if err != nil || !bytes.Equal(decoded.Datum.Inline, datum) {
		t.Fatalf("decoded = %#v (%v)", decoded, err)
	}
	if _, err := cardano.EncodeTxOut(cardano.TxOut{
		Format: cardano.TxOutBabbage, Address: seq(1, 29),
		Datum: &cardano.TxOutDatum{Inline: []byte{}},
	}); !errors.Is(err, cardano.ErrPayload) {
		t.Fatalf("EncodeTxOut(empty inline datum) error = %v, want ErrPayload", err)
	}
}

// indefinite builds an indefinite-length array (0x9f) or map (0xbf) from
// already-encoded items. The ledger serializer emits these for every list,
// set and map with more than 23 elements (lengthThreshold in
// cardano-ledger-binary's Encoder.hs), so OnChain payloads contain them.
func indefinite(header byte, items ...[]byte) []byte {
	result := []byte{header}
	for _, item := range items {
		result = append(result, item...)
	}
	return append(result, 0xff)
}

func TestTxOutAcceptsLedgerIndefiniteLengths(t *testing.T) {
	t.Parallel()
	var assets [][]byte
	for i := range 24 {
		assets = append(assets, marshalOrFatal(t, []byte{byte(i)}), []byte{0x01})
	}
	inner := marshalOrFatalUnsorted(t, 0x82,
		marshalOrFatal(t, seq(1, 29)),
		marshalOrFatalUnsorted(t, 0x82, []byte{0x01},
			indefinite(0xbf, marshalOrFatal(t, seq(0, 28)), indefinite(0xbf, assets...))))
	data := marshalOrFatal(t, inner)
	decoded, err := cardano.DecodeTxOut(data)
	if err != nil {
		t.Fatalf("DecodeTxOut(24 assets, ledger encoding) error = %v", err)
	}
	if got := len((*decoded.Value.Assets)[0].Assets); got != 24 {
		t.Fatalf("assets = %d, want 24", got)
	}
	if !bytes.Equal(decoded.Raw, inner) {
		t.Fatalf("Raw = %x, want %x", decoded.Raw, inner)
	}
	again, err := cardano.EncodeTxOut(decoded)
	if err != nil || !bytes.Equal(again, data) {
		t.Fatalf("re-encode = %x (%v), want %x", again, err, data)
	}

	var scripts [][]byte
	for i := range 24 {
		scripts = append(scripts, marshalOrFatal(t, cbor.Array{uint64(0), seq(byte(i), 28)}))
	}
	native := marshalOrFatalUnsorted(t, 0x82, []byte{0x01}, indefinite(0x9f, scripts...))
	scriptRef := marshalOrFatalUnsorted(t, 0x82, []byte{0x00}, native)
	withScript := marshalOrFatal(t, cbor.Map{
		{Key: uint64(0), Value: seq(1, 29)},
		{Key: uint64(1), Value: uint64(1)},
		{Key: uint64(3), Value: cbor.Tag{Number: 24, Value: scriptRef}},
	})
	if _, err := cardano.DecodeTxOut(marshalOrFatal(t, withScript)); err != nil {
		t.Fatalf("DecodeTxOut(native script with indefinite list) error = %v", err)
	}
}

func TestGovProposalAcceptsLedgerIndefiniteLengths(t *testing.T) {
	t.Parallel()
	var costs [][]byte
	for i := range 24 {
		costs = append(costs, marshalOrFatal(t, uint64(i)))
	}
	update := marshalOrFatalUnsorted(t, 0xa1, []byte{0x12},
		marshalOrFatalUnsorted(t, 0xa1, []byte{0x02}, indefinite(0x9f, costs...)))
	action := marshalOrFatalUnsorted(t, 0x84, []byte{0x00}, []byte{0xf6}, update, []byte{0xf6})
	procedure := marshalOrFatalUnsorted(t, 0x84,
		marshalOrFatal(t, uint64(5)),
		marshalOrFatal(t, rewardAccount(0xe0)),
		action,
		marshalOrFatal(t, cbor.Array{"u", []byte{1}}),
	)
	data := proposalWithProcedure(t, procedure)
	decoded, err := cardano.DecodeGovProposal(data)
	if err != nil {
		t.Fatalf("DecodeGovProposal(24-entry cost model, ledger encoding) error = %v", err)
	}
	if got := len((*decoded.Procedure.Action.ParamUpdate.CostModels)[2]); got != 24 {
		t.Fatalf("cost model length = %d, want 24", got)
	}
	again, err := cardano.EncodeGovProposal(decoded)
	if err != nil || !bytes.Equal(again, data) {
		t.Fatalf("re-encode = %x (%v), want %x", again, err, data)
	}

	var withdrawals [][]byte
	for i := range 24 {
		withdrawals = append(withdrawals, marshalOrFatal(t, rewardAccount(byte(i))), []byte{0x01})
	}
	treasury := marshalOrFatalUnsorted(t, 0x83, []byte{0x02}, indefinite(0xbf, withdrawals...), []byte{0xf6})
	procedure = marshalOrFatalUnsorted(t, 0x84,
		marshalOrFatal(t, uint64(5)),
		marshalOrFatal(t, rewardAccount(0xe0)),
		treasury,
		marshalOrFatal(t, cbor.Array{"u", []byte{1}}),
	)
	decoded, err = cardano.DecodeGovProposal(proposalWithProcedure(t, procedure))
	if err != nil {
		t.Fatalf("DecodeGovProposal(24 withdrawals, ledger encoding) error = %v", err)
	}
	if got := len(decoded.Procedure.Action.Withdrawals); got != 24 {
		t.Fatalf("withdrawals = %d, want 24", got)
	}
}
