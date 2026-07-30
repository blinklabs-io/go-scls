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

//go:build ignore

// Run from the repository root with:
// go run ./testdata/cardano/generate_scls.go
package main

import (
	"fmt"
	"os"

	scls "github.com/blinklabs-io/go-scls"
	"github.com/blinklabs-io/go-scls/cardano"
)

type fixture struct {
	namespace string
	file      string
}

var fixtures = []fixture{
	{namespace: cardano.BlocksV0Name, file: "blocks_v0"},
	{namespace: cardano.EntitiesAccountsV0Name, file: "entities_accounts_v0"},
	{namespace: cardano.EntitiesCommitteeV0Name, file: "entities_committee_v0"},
	{namespace: cardano.EntitiesDRepsV0Name, file: "entities_dreps_v0"},
	{
		namespace: cardano.EntitiesStakePoolsV0Name,
		file:      "entities_stake_pools_v0_with_leios",
	},
	{
		namespace: cardano.EntitiesStakePoolsVRFKeyHashesV0Name,
		file:      "entities_stake_pools_vrf_key_hashes_v0",
	},
	{namespace: cardano.GovCommitteeV0Name, file: "gov_committee_v0"},
	{namespace: cardano.GovConstitutionV0Name, file: "gov_constitution_v0"},
	{namespace: cardano.GovPParamsV0Name, file: "gov_pparams_v0"},
	{namespace: cardano.GovProposalsRootsV0Name, file: "gov_proposals_roots_v0"},
	{namespace: cardano.GovProposalsV0Name, file: "gov_proposals_v0"},
	{namespace: cardano.NoncesV0Name, file: "nonces_v0"},
	{namespace: cardano.SnapshotsGoV0Name, file: "snapshots_go_v0"},
	{namespace: cardano.SnapshotsMarkV0Name, file: "snapshots_mark_v0"},
	{namespace: cardano.SnapshotsSetV0Name, file: "snapshots_set_v0"},
	{namespace: cardano.UTxOV0Name, file: "utxo_v0"},
}

func main() {
	const directory = "testdata/cardano"
	output, err := os.Create(directory + "/canonical-namespaces.scls")
	if err != nil {
		panic(err)
	}
	writer, err := scls.NewWriter(output)
	if err != nil {
		panic(err)
	}
	for _, fixture := range fixtures {
		namespace, ok := cardano.DefaultRegistry.Lookup(fixture.namespace)
		if !ok {
			panic(fmt.Errorf("namespace %q is not registered", fixture.namespace))
		}
		keyFile := fixture.file
		if fixture.namespace == cardano.EntitiesStakePoolsV0Name {
			keyFile = "entities_stake_pools_v0"
		}
		key, err := os.ReadFile(directory + "/" + keyFile + ".key")
		if err != nil {
			panic(err)
		}
		payload, err := os.ReadFile(directory + "/" + fixture.file + ".cbor")
		if err != nil {
			panic(err)
		}
		if _, err := namespace.Key.DecodeKey(key); err != nil {
			panic(err)
		}
		if _, err := namespace.Payload.DecodePayload(payload); err != nil {
			panic(err)
		}
		if err := writer.AddEntry(fixture.namespace, key, payload); err != nil {
			panic(err)
		}
	}
	if _, err := writer.Close(0); err != nil {
		panic(err)
	}
	if err := output.Close(); err != nil {
		panic(err)
	}
}
