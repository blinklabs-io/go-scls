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
	"io"
	"os"
	"path/filepath"
	"testing"

	scls "github.com/blinklabs-io/go-scls"
	"github.com/blinklabs-io/go-scls/cardano"
	"github.com/blinklabs-io/go-scls/cardano/cbor"
)

const fixtureDirectory = "../testdata/cardano"

type payloadFixture struct {
	namespace string
	file      string
	keyFile   string
}

var payloadFixtures = []payloadFixture{
	{namespace: cardano.BlocksV0Name, file: "blocks_v0"},
	{namespace: cardano.UTxOV0Name, file: "utxo_v0"},
	{namespace: cardano.EntitiesAccountsV0Name, file: "entities_accounts_v0"},
	{namespace: cardano.EntitiesCommitteeV0Name, file: "entities_committee_v0"},
	{namespace: cardano.EntitiesDRepsV0Name, file: "entities_dreps_v0"},
	{
		namespace: cardano.EntitiesStakePoolsV0Name,
		file:      "entities_stake_pools_v0_without_leios",
		keyFile:   "entities_stake_pools_v0",
	},
	{
		namespace: cardano.EntitiesStakePoolsV0Name,
		file:      "entities_stake_pools_v0_with_leios",
		keyFile:   "entities_stake_pools_v0",
	},
	{
		namespace: cardano.EntitiesStakePoolsVRFKeyHashesV0Name,
		file:      "entities_stake_pools_vrf_key_hashes_v0",
	},
	{namespace: cardano.GovCommitteeV0Name, file: "gov_committee_v0"},
	{namespace: cardano.GovConstitutionV0Name, file: "gov_constitution_v0"},
	{namespace: cardano.GovPParamsV0Name, file: "gov_pparams_v0"},
	{namespace: cardano.GovProposalsV0Name, file: "gov_proposals_v0"},
	{namespace: cardano.GovProposalsRootsV0Name, file: "gov_proposals_roots_v0"},
	{namespace: cardano.NoncesV0Name, file: "nonces_v0"},
	{namespace: cardano.SnapshotsMarkV0Name, file: "snapshots_mark_v0"},
	{namespace: cardano.SnapshotsSetV0Name, file: "snapshots_set_v0"},
	{namespace: cardano.SnapshotsGoV0Name, file: "snapshots_go_v0"},
}

func TestHaskellPayloadFixtures(t *testing.T) {
	t.Parallel()
	covered := make(map[string]struct{})
	for _, fixture := range payloadFixtures {
		fixture := fixture
		t.Run(fixture.file, func(t *testing.T) {
			t.Parallel()
			namespace, ok := cardano.DefaultRegistry.Lookup(fixture.namespace)
			if !ok {
				t.Fatalf("namespace %q is not registered", fixture.namespace)
			}
			keyFile := fixture.keyFile
			if keyFile == "" {
				keyFile = fixture.file
			}
			key := readFixture(t, keyFile+".key")
			decodedKey, err := namespace.Key.DecodeKey(key)
			if err != nil {
				t.Fatalf("DecodeKey() error = %v", err)
			}
			reencodedKey, err := namespace.Key.EncodeKey(decodedKey)
			if err != nil {
				t.Fatalf("EncodeKey(decoded) error = %v", err)
			}
			if !bytes.Equal(reencodedKey, key) {
				t.Fatalf("key re-encode = %x, want %x", reencodedKey, key)
			}

			payload := readFixture(t, fixture.file+".cbor")
			decoded, err := namespace.Payload.DecodePayload(payload)
			if err != nil {
				t.Fatalf("DecodePayload() error = %v", err)
			}
			reencoded, err := namespace.Payload.EncodePayload(decoded)
			if err != nil {
				t.Fatalf("EncodePayload(decoded) error = %v", err)
			}
			if !bytes.Equal(reencoded, payload) {
				t.Fatalf("payload re-encode differs:\n got %x\nwant %x", reencoded, payload)
			}
		})
		covered[fixture.namespace] = struct{}{}
	}
	if got := len(covered); got != 16 {
		t.Fatalf("fixture namespace coverage = %d, want 16", got)
	}
}

func TestNegativeKeyFixtures(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"invalid_key_short.key", "invalid_key_long.key"} {
		key := readFixture(t, file)
		if _, err := cardano.EntitiesStakePoolsV0.Key.DecodeKey(key); !errors.Is(err, cardano.ErrKeySize) {
			t.Errorf("%s: DecodeKey() error = %v, want ErrKeySize", file, err)
		}
	}
}

func TestNegativeBLSFixtures(t *testing.T) {
	t.Parallel()
	files := []string{
		"invalid_bls_public_key.cbor",
		"invalid_bls_public_key_long.cbor",
		"invalid_bls_proof.cbor",
		"invalid_bls_proof_long.cbor",
	}
	for _, file := range files {
		payload := readFixture(t, file)
		_, err := cardano.EntitiesStakePoolsV0.Payload.DecodePayload(payload)
		if !errors.Is(err, cardano.ErrStakePoolPayload) {
			t.Errorf("%s: DecodePayload() error = %v, want ErrStakePoolPayload", file, err)
		}
	}
}

func TestNegativeCanonicalCBORFixtures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		file string
		want error
	}{
		{file: "invalid_field_order.cbor", want: cbor.ErrMapOrder},
		{file: "invalid_duplicate_field.cbor", want: cbor.ErrDuplicateMapKey},
		{file: "invalid_noncanonical.cbor", want: cbor.ErrNonMinimal},
	}
	for _, test := range tests {
		payload := readFixture(t, test.file)
		_, err := cardano.EntitiesStakePoolsV0.Payload.DecodePayload(payload)
		if !errors.Is(err, test.want) {
			t.Errorf("%s: DecodePayload() error = %v, want %v", test.file, err, test.want)
		}
	}
}

func TestCanonicalNamespaceSCLSFixture(t *testing.T) {
	t.Parallel()
	expectedFixtures := map[string]payloadFixture{
		cardano.BlocksV0Name: {
			namespace: cardano.BlocksV0Name,
			file:      "blocks_v0",
		},
		cardano.EntitiesAccountsV0Name: {
			namespace: cardano.EntitiesAccountsV0Name,
			file:      "entities_accounts_v0",
		},
		cardano.EntitiesCommitteeV0Name: {
			namespace: cardano.EntitiesCommitteeV0Name,
			file:      "entities_committee_v0",
		},
		cardano.EntitiesDRepsV0Name: {
			namespace: cardano.EntitiesDRepsV0Name,
			file:      "entities_dreps_v0",
		},
		cardano.EntitiesStakePoolsV0Name: {
			namespace: cardano.EntitiesStakePoolsV0Name,
			file:      "entities_stake_pools_v0_with_leios",
			keyFile:   "entities_stake_pools_v0",
		},
		cardano.EntitiesStakePoolsVRFKeyHashesV0Name: {
			namespace: cardano.EntitiesStakePoolsVRFKeyHashesV0Name,
			file:      "entities_stake_pools_vrf_key_hashes_v0",
		},
		cardano.GovCommitteeV0Name: {
			namespace: cardano.GovCommitteeV0Name,
			file:      "gov_committee_v0",
		},
		cardano.GovConstitutionV0Name: {
			namespace: cardano.GovConstitutionV0Name,
			file:      "gov_constitution_v0",
		},
		cardano.GovPParamsV0Name: {
			namespace: cardano.GovPParamsV0Name,
			file:      "gov_pparams_v0",
		},
		cardano.GovProposalsRootsV0Name: {
			namespace: cardano.GovProposalsRootsV0Name,
			file:      "gov_proposals_roots_v0",
		},
		cardano.GovProposalsV0Name: {
			namespace: cardano.GovProposalsV0Name,
			file:      "gov_proposals_v0",
		},
		cardano.NoncesV0Name: {
			namespace: cardano.NoncesV0Name,
			file:      "nonces_v0",
		},
		cardano.SnapshotsGoV0Name: {
			namespace: cardano.SnapshotsGoV0Name,
			file:      "snapshots_go_v0",
		},
		cardano.SnapshotsMarkV0Name: {
			namespace: cardano.SnapshotsMarkV0Name,
			file:      "snapshots_mark_v0",
		},
		cardano.SnapshotsSetV0Name: {
			namespace: cardano.SnapshotsSetV0Name,
			file:      "snapshots_set_v0",
		},
		cardano.UTxOV0Name: {
			namespace: cardano.UTxOV0Name,
			file:      "utxo_v0",
		},
	}
	path := filepath.Join(fixtureDirectory, "canonical-namespaces.scls")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := scls.Verify(file, scls.VerifyFull)
	if err != nil {
		file.Close()
		t.Fatalf("Verify() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if result.TotalEntries != 16 || result.TotalChunks != 16 || len(result.Namespaces) != 16 {
		t.Fatalf(
			"Verify() counts = %d entries, %d chunks, %d namespaces; want 16 each",
			result.TotalEntries,
			result.TotalChunks,
			len(result.Namespaces),
		)
	}

	file, err = os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := scls.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	seenNamespaces := make(map[string]struct{}, len(expectedFixtures))
	for {
		chunk, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		namespace, ok := cardano.DefaultRegistry.Lookup(chunk.Namespace)
		if !ok {
			t.Fatalf("fixture namespace %q is not registered", chunk.Namespace)
		}
		expected, ok := expectedFixtures[chunk.Namespace]
		if !ok {
			t.Fatalf("fixture namespace %q is not selected by the generator", chunk.Namespace)
		}
		keyFile := expected.keyFile
		if keyFile == "" {
			keyFile = expected.file
		}
		expectedKey := readFixture(t, keyFile+".key")
		expectedPayload := readFixture(t, expected.file+".cbor")
		for _, entry := range chunk.Entries {
			if !bytes.Equal(entry.Key, expectedKey) {
				t.Fatalf("%s key differs from %s.key", chunk.Namespace, keyFile)
			}
			if !bytes.Equal(entry.Value, expectedPayload) {
				t.Fatalf("%s payload differs from %s.cbor", chunk.Namespace, expected.file)
			}
			if _, err := namespace.Key.DecodeKey(entry.Key); err != nil {
				t.Fatalf("%s key: %v", chunk.Namespace, err)
			}
			if _, err := namespace.Payload.DecodePayload(entry.Value); err != nil {
				t.Fatalf("%s payload: %v", chunk.Namespace, err)
			}
			seen++
		}
		seenNamespaces[chunk.Namespace] = struct{}{}
	}
	if seen != len(expectedFixtures) || len(seenNamespaces) != len(expectedFixtures) {
		t.Fatalf(
			"decoded fixture entries/namespaces = %d/%d, want %d/%d",
			seen,
			len(seenNamespaces),
			len(expectedFixtures),
			len(expectedFixtures),
		)
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	value, err := os.ReadFile(filepath.Join(fixtureDirectory, name))
	if err != nil {
		t.Fatal(err)
	}
	return value
}
