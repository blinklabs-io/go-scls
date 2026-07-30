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

const (
	// BlocksV0Name is the canonical blocks namespace.
	BlocksV0Name = "blocks/v0"
	// UTxOV0Name is the canonical UTxO namespace.
	UTxOV0Name = "utxo/v0"
	// EntitiesAccountsV0Name is the canonical accounts namespace.
	EntitiesAccountsV0Name = "entities/accounts/v0"
	// EntitiesCommitteeV0Name is the canonical committee-entity namespace.
	EntitiesCommitteeV0Name = "entities/committee/v0"
	// EntitiesDRepsV0Name is the canonical DRep namespace.
	EntitiesDRepsV0Name = "entities/dreps/v0"
	// EntitiesStakePoolsVRFKeyHashesV0Name is the canonical stake-pool VRF
	// key-hash namespace.
	EntitiesStakePoolsVRFKeyHashesV0Name = "entities/stake_pools/vrf_key_hashes/v0"
	// GovCommitteeV0Name is the canonical governance committee namespace.
	GovCommitteeV0Name = "gov/committee/v0"
	// GovConstitutionV0Name is the canonical constitution namespace.
	GovConstitutionV0Name = "gov/constitution/v0"
	// GovPParamsV0Name is the canonical protocol-parameters namespace.
	GovPParamsV0Name = "gov/pparams/v0"
	// GovProposalsV0Name is the canonical governance proposals namespace.
	GovProposalsV0Name = "gov/proposals/v0"
	// GovProposalsRootsV0Name is the canonical governance proposal roots
	// namespace.
	GovProposalsRootsV0Name = "gov/proposals/roots/v0"
	// NoncesV0Name is the canonical nonces namespace.
	NoncesV0Name = "nonces/v0"
	// SnapshotsMarkV0Name is the canonical mark-snapshot namespace.
	SnapshotsMarkV0Name = "snapshots/mark/v0"
	// SnapshotsSetV0Name is the canonical set-snapshot namespace.
	SnapshotsSetV0Name = "snapshots/set/v0"
	// SnapshotsGoV0Name is the canonical go-snapshot namespace.
	SnapshotsGoV0Name = "snapshots/go/v0"
)

var (
	// BlocksV0 validates 36-byte keys and canonical CBOR payloads.
	BlocksV0 = newCanonicalNamespace(BlocksV0Name, BlocksKeyCodec{})
	// UTxOV0 validates 34-byte keys and canonical CBOR payloads.
	UTxOV0 = newCanonicalNamespace(UTxOV0Name, UTxOKeyCodec{})
	// EntitiesAccountsV0 validates 29-byte keys and canonical CBOR payloads.
	EntitiesAccountsV0 = newCanonicalNamespace(EntitiesAccountsV0Name, CredentialKeyCodec{})
	// EntitiesCommitteeV0 validates singleton keys and canonical CBOR payloads.
	EntitiesCommitteeV0 = newCanonicalNamespace(EntitiesCommitteeV0Name, SingletonKeyCodec{})
	// EntitiesDRepsV0 validates 29-byte keys and canonical CBOR payloads.
	EntitiesDRepsV0 = newCanonicalNamespace(EntitiesDRepsV0Name, CredentialKeyCodec{})
	// EntitiesStakePoolsVRFKeyHashesV0 validates 32-byte keys and canonical
	// CBOR payloads.
	EntitiesStakePoolsVRFKeyHashesV0 = newCanonicalNamespace(
		EntitiesStakePoolsVRFKeyHashesV0Name,
		VRFKeyHashKeyCodec{},
	)
	// GovCommitteeV0 validates singleton keys and canonical CBOR payloads.
	GovCommitteeV0 = newCanonicalNamespace(GovCommitteeV0Name, SingletonKeyCodec{})
	// GovConstitutionV0 validates singleton keys and canonical CBOR payloads.
	GovConstitutionV0 = newCanonicalNamespace(GovConstitutionV0Name, SingletonKeyCodec{})
	// GovPParamsV0 validates four-byte keys and canonical CBOR payloads.
	GovPParamsV0 = newCanonicalNamespace(GovPParamsV0Name, GovPParamsKeyCodec{})
	// GovProposalsV0 validates 34-byte keys and canonical CBOR payloads.
	GovProposalsV0 = newCanonicalNamespace(GovProposalsV0Name, GovProposalKeyCodec{})
	// GovProposalsRootsV0 validates one-byte keys and canonical CBOR payloads.
	GovProposalsRootsV0 = newCanonicalNamespace(
		GovProposalsRootsV0Name,
		GovProposalRootKeyCodec{},
	)
	// NoncesV0 validates singleton keys and canonical CBOR payloads.
	NoncesV0 = newCanonicalNamespace(NoncesV0Name, SingletonKeyCodec{})
	// SnapshotsMarkV0 validates 31-byte keys and canonical CBOR payloads.
	SnapshotsMarkV0 = newCanonicalNamespace(SnapshotsMarkV0Name, SnapshotKeyCodec{})
	// SnapshotsSetV0 validates 31-byte keys and canonical CBOR payloads.
	SnapshotsSetV0 = newCanonicalNamespace(SnapshotsSetV0Name, SnapshotKeyCodec{})
	// SnapshotsGoV0 validates 31-byte keys and canonical CBOR payloads.
	SnapshotsGoV0 = newCanonicalNamespace(SnapshotsGoV0Name, SnapshotKeyCodec{})

	// CanonicalNamespacesV0 contains the namespace codecs defined in this
	// file. The specialized entities/stake_pools/v0 codec is defined
	// separately.
	CanonicalNamespacesV0 = []Namespace{
		BlocksV0,
		UTxOV0,
		EntitiesAccountsV0,
		EntitiesCommitteeV0,
		EntitiesDRepsV0,
		EntitiesStakePoolsVRFKeyHashesV0,
		GovCommitteeV0,
		GovConstitutionV0,
		GovPParamsV0,
		GovProposalsV0,
		GovProposalsRootsV0,
		NoncesV0,
		SnapshotsMarkV0,
		SnapshotsSetV0,
		SnapshotsGoV0,
	}
)

func init() {
	for _, namespace := range CanonicalNamespacesV0 {
		DefaultRegistry.MustRegister(namespace)
	}
}

func newCanonicalNamespace(name string, key KeyCodec) Namespace {
	return Namespace{
		Metadata: NamespaceMetadata{Name: name, KeySize: key.Size()},
		Key:      key,
		Payload:  CanonicalPayloadCodec{},
	}
}
