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

// The typed and schema-invalid fixtures are written by
// testdata/cardano/generate.hs.
var typedFixtures = map[string]cardano.PayloadCodec{
	"blocks_v0_large":                            cardano.BlocksMadePayloadCodec{},
	"entities_accounts_v0_delegated":             cardano.AccountStatePayloadCodec{},
	"entities_accounts_v0_abstain":               cardano.AccountStatePayloadCodec{},
	"entities_dreps_v0_populated":                cardano.DRepStatePayloadCodec{},
	"entities_committee_v0_members":              cardano.CommitteeStatePayloadCodec{},
	"entities_stake_pools_vrf_key_hashes_v0_max": cardano.VRFKeyHashCountPayloadCodec{},
	"gov_committee_v0_members":                   cardano.GovCommitteePayloadCodec{},
	"gov_constitution_v0_guardrails":             cardano.ConstitutionPayloadCodec{},
	"gov_pparams_v0_populated":                   cardano.ProtocolParametersPayloadCodec{},
	"gov_proposals_roots_v0_max_index":           cardano.GovProposalRootPayloadCodec{},
	"nonces_v0_populated":                        cardano.NoncesPayloadCodec{},
	"snapshots_mark_v0_pool":                     cardano.SnapshotPayloadCodec{},
	"utxo_v0_shelley_assets":                     cardano.TxOutPayloadCodec{},
	"utxo_v0_babbage_datum_script":               cardano.TxOutPayloadCodec{},
	"utxo_v0_ledger_order":                       cardano.TxOutPayloadCodec{},
	"utxo_v0_indefinite_datum":                   cardano.TxOutPayloadCodec{},
	"gov_proposals_v0_parameter_change":          cardano.GovProposalPayloadCodec{},
	"gov_proposals_v0_hard_fork":                 cardano.GovProposalPayloadCodec{},
	"gov_proposals_v0_treasury_withdrawals":      cardano.GovProposalPayloadCodec{},
	"gov_proposals_v0_no_confidence":             cardano.GovProposalPayloadCodec{},
	"gov_proposals_v0_update_committee":          cardano.GovProposalPayloadCodec{},
	"gov_proposals_v0_new_constitution":          cardano.GovProposalPayloadCodec{},
	"gov_proposals_v0_info":                      cardano.GovProposalPayloadCodec{},
	"gov_proposals_v0_votes":                     cardano.GovProposalPayloadCodec{},
	"gov_proposals_v0_ledger_order":              cardano.GovProposalPayloadCodec{},
}

var schemaInvalidFixtures = map[string]cardano.PayloadCodec{
	"invalid_accounts_drep_kind":               cardano.AccountStatePayloadCodec{},
	"invalid_accounts_pool_hash_short":         cardano.AccountStatePayloadCodec{},
	"invalid_blocks_negative":                  cardano.BlocksMadePayloadCodec{},
	"invalid_dreps_anchor_url_long":            cardano.DRepStatePayloadCodec{},
	"invalid_committee_resignation_hash":       cardano.CommitteeStatePayloadCodec{},
	"invalid_vrf_zero":                         cardano.VRFKeyHashCountPayloadCodec{},
	"invalid_gov_committee_threshold":          cardano.GovCommitteePayloadCodec{},
	"invalid_constitution_script_hash":         cardano.ConstitutionPayloadCodec{},
	"invalid_pparams_missing_key":              cardano.ProtocolParametersPayloadCodec{},
	"invalid_pparams_rho_above_one":            cardano.ProtocolParametersPayloadCodec{},
	"invalid_pparams_max_tx_size_overflow":     cardano.ProtocolParametersPayloadCodec{},
	"invalid_pparams_cost_model_overflow":      cardano.ProtocolParametersPayloadCodec{},
	"invalid_roots_index_overflow":             cardano.GovProposalRootPayloadCodec{},
	"invalid_nonces_neutral_with_hash":         cardano.NoncesPayloadCodec{},
	"invalid_snapshot_kind":                    cardano.SnapshotPayloadCodec{},
	"invalid_snapshot_pool_as_coin":            cardano.SnapshotPayloadCodec{},
	"invalid_utxo_zero_quantity":               cardano.TxOutPayloadCodec{},
	"invalid_utxo_datum_kind":                  cardano.TxOutPayloadCodec{},
	"invalid_proposal_vote":                    cardano.GovProposalPayloadCodec{},
	"invalid_proposal_action_kind":             cardano.GovProposalPayloadCodec{},
	"invalid_proposal_update_protocol_version": cardano.GovProposalPayloadCodec{},
	"invalid_proposal_reward_account":          cardano.GovProposalPayloadCodec{},
}

func TestTypedHaskellFixturesRoundTrip(t *testing.T) {
	t.Parallel()
	for name, codec := range typedFixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			payload := readFixture(t, name+".cbor")
			decoded, err := codec.DecodePayload(payload)
			if err != nil {
				t.Fatalf("DecodePayload() error = %v", err)
			}
			encoded, err := codec.EncodePayload(decoded)
			if err != nil {
				t.Fatalf("EncodePayload(decoded) error = %v", err)
			}
			if !bytes.Equal(encoded, payload) {
				t.Fatalf("re-encode differs:\n got %x\nwant %x", encoded, payload)
			}
		})
	}
}

func TestSchemaInvalidFixturesAreCanonicalButRejected(t *testing.T) {
	t.Parallel()
	for name, codec := range schemaInvalidFixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			payload := readFixture(t, name+".cbor")
			if err := cbor.Validate(payload); err != nil {
				t.Fatalf("fixture is not deterministic CBOR: %v", err)
			}
			if _, err := codec.DecodePayload(payload); !errors.Is(err, cardano.ErrPayload) {
				t.Fatalf("DecodePayload() error = %v, want ErrPayload", err)
			}
		})
	}
}

func TestTypedFixtureContents(t *testing.T) {
	t.Parallel()
	decodeProposal := func(name string) cardano.GovProposal {
		t.Helper()
		value, err := cardano.DecodeGovProposal(readFixture(t, name+".cbor"))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return value
	}
	kinds := map[string]cardano.GovActionKind{
		"parameter_change": cardano.GovParameterChange, "hard_fork": cardano.GovHardForkInitiation,
		"treasury_withdrawals": cardano.GovTreasuryWithdrawals, "no_confidence": cardano.GovNoConfidence,
		"update_committee": cardano.GovUpdateCommittee, "new_constitution": cardano.GovNewConstitution,
		"info": cardano.GovInfo,
	}
	for name, kind := range kinds {
		proposal := decodeProposal("gov_proposals_v0_" + name)
		if proposal.Procedure.Action.Kind != kind {
			t.Errorf("%s: kind = %d, want %d", name, proposal.Procedure.Action.Kind, kind)
		}
		if proposal.Procedure.Raw != nil {
			t.Errorf("%s: deterministic fixture produced Raw", name)
		}
	}
	update := decodeProposal("gov_proposals_v0_parameter_change").Procedure.Action.ParamUpdate
	if update.MinFeeA == nil || *update.MinFeeA != 50 || update.A0 == nil ||
		update.A0.Denominator != 2 || update.MaxTxSize != nil || update.CostModels == nil ||
		(*update.CostModels)[1][0] != -1 {
		t.Errorf("parameter update = %#v", update)
	}
	ledger := decodeProposal("gov_proposals_v0_ledger_order")
	if ledger.Procedure.Raw == nil || ledger.Procedure.Action.Removed[0].Kind != cardano.CredentialScriptHash {
		t.Errorf("ledger-ordered procedure: Raw=%v removed=%#v", ledger.Procedure.Raw != nil, ledger.Procedure.Action.Removed)
	}
	votes := decodeProposal("gov_proposals_v0_votes")
	if votes.Order != 12 || len(votes.DRepVotes) != 2 || votes.StakePoolVotes[0].Vote != cardano.VoteYes ||
		votes.CommitteeVotes[0].Vote != cardano.VoteNo {
		t.Errorf("votes proposal = %#v", votes)
	}

	params, err := cardano.DecodeProtocolParameters(readFixture(t, "gov_pparams_v0_populated.cbor"))
	if err != nil {
		t.Fatal(err)
	}
	if params.CostModels == nil {
		t.Fatal("cost models are nil")
	}
	if params.MinFeeA != 44 || params.ProtocolVersion.Major != 10 || params.MemoryPrice.Numerator != 577 ||
		params.DRepVotingThresholds[9].Numerator != 69 || len(params.CostModels[3]) != 0 {
		t.Errorf("parameters = %#v", params)
	}

	if want := []int64{1, -5, 4294967296}; !reflect.DeepEqual(params.CostModels[0], want) {
		t.Errorf("cost model 0 = %v, want %v", params.CostModels[0], want)
	}
	if want := []int64{-9223372036854775808}; !reflect.DeepEqual(params.CostModels[2], want) {
		t.Errorf("cost model 2 = %v, want %v", params.CostModels[2], want)
	}

	nonces, err := cardano.DecodeNonces(readFixture(t, "nonces_v0_populated.cbor"))
	if err != nil {
		t.Fatal(err)
	}
	if nonces.LabNonce.Hash == nil || nonces.CandidateNonce.Hash != nil || nonces.LastSlot == nil ||
		*nonces.LastSlot != 123456 || len(nonces.CertCounters) != 2 {
		t.Errorf("nonces = %#v", nonces)
	}

	ledgerOut, err := cardano.DecodeTxOut(readFixture(t, "utxo_v0_ledger_order.cbor"))
	if err != nil || ledgerOut.Raw == nil {
		t.Errorf("ledger-ordered tx_out: %#v (%v)", ledgerOut, err)
	}
	indefinite, err := cardano.DecodeTxOut(readFixture(t, "utxo_v0_indefinite_datum.cbor"))
	if err != nil || !bytes.Equal(indefinite.Datum.Inline, []byte{0x9f, 0x01, 0xff}) {
		t.Errorf("indefinite datum tx_out: %#v (%v)", indefinite, err)
	}
	babbage, err := cardano.DecodeTxOut(readFixture(t, "utxo_v0_babbage_datum_script.cbor"))
	if err != nil || babbage.Script == nil || babbage.Script.Language != 2 || babbage.Datum.Inline == nil ||
		babbage.Format != cardano.TxOutBabbage {
		t.Errorf("babbage tx_out: %#v (%v)", babbage, err)
	}
}
