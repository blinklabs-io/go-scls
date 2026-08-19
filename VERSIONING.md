# Versioning policy

`go-scls` follows [Semantic Versioning 2.0.0](https://semver.org/) for the
Go module and publishes releases as repository tags in the form `vMAJOR.MINOR.PATCH`.
The first versioned release is `v0.1.0`.

## Go module API

While the module is below `v1.0.0`, minor releases may add public APIs and may
include breaking public API changes; consumers should review the release notes
before upgrading. Patch releases are reserved for backwards-compatible fixes,
and breaking changes are called out in the release notes. Once `v1.0.0` is
released, breaking API changes require a new major module version in accordance
with Go module versioning.

The root module uses `vMAJOR.MINOR.PATCH` tags. The nested `cmd/scls` module
uses separate `cmd/scls/vMAJOR.MINOR.PATCH` tags. The CLI embeds its tag and
commit in `scls version`; the library does not expose a runtime version
variable.

## SCLS container format

The Go module version and the SCLS container format version are independent.
The current container format is SCLS version 1, recorded in each file's HDR
record. A new Go release does not change the on-disk format version. A format
revision changes that version only when the wire format is intentionally
updated and the compatibility rules are documented in `spec/RECONCILIATION.md`.

## Cardano namespace APIs

Cardano namespace codecs are versioned by their namespace names, such as
`utxo/v0` and `blocks/v0`. A new namespace schema or an incompatible change
gets a new namespace version; existing namespace versions remain decodable.
Compatible additions to the Go codec API can be released under the module's
normal SemVer policy. Namespace schema changes must be documented in
`spec/namespaces/` and in the release notes.

## Release process

Releases are made by pushing an annotated root or `cmd/scls/` module tag. The
`publish` workflow creates the GitHub release, requests the tagged module from
`proxy.golang.org`, then verifies that the tagged version and generated
documentation are available on `pkg.go.dev`. It also builds and attaches the
CLI for supported OS/architecture pairs with build-provenance attestations.
