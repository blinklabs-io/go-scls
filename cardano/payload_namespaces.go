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

// TypedNamespacesV0 holds the version-0 namespaces from CanonicalNamespacesV0
// and entities/stake_pools/v0 with typed payload codecs in place of the
// generic deterministic-CBOR codec. They are not registered: DefaultRegistry
// keeps the generic codecs for callers that depend on generic values.
var TypedNamespacesV0 = []Namespace{
	typedNamespace(BlocksV0, BlocksMadePayloadCodec{}),
	typedNamespace(UTxOV0, TxOutPayloadCodec{}),
	typedNamespace(EntitiesAccountsV0, AccountStatePayloadCodec{}),
	typedNamespace(EntitiesCommitteeV0, CommitteeStatePayloadCodec{}),
	typedNamespace(EntitiesDRepsV0, DRepStatePayloadCodec{}),
	EntitiesStakePoolsV0,
	typedNamespace(EntitiesStakePoolsVRFKeyHashesV0, VRFKeyHashCountPayloadCodec{}),
	typedNamespace(GovCommitteeV0, GovCommitteePayloadCodec{}),
	typedNamespace(GovConstitutionV0, ConstitutionPayloadCodec{}),
	typedNamespace(GovPParamsV0, ProtocolParametersPayloadCodec{}),
	typedNamespace(GovProposalsV0, GovProposalPayloadCodec{}),
	typedNamespace(GovProposalsRootsV0, GovProposalRootPayloadCodec{}),
	typedNamespace(NoncesV0, NoncesPayloadCodec{}),
	typedNamespace(SnapshotsMarkV0, SnapshotPayloadCodec{}),
	typedNamespace(SnapshotsSetV0, SnapshotPayloadCodec{}),
	typedNamespace(SnapshotsGoV0, SnapshotPayloadCodec{}),
}

func typedNamespace(base Namespace, payload PayloadCodec) Namespace {
	base.Payload = payload
	return base
}
