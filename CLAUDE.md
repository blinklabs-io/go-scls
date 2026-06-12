# go-scls — Claude Code Guide

Companion to `AGENTS.md`; read that first. This file: Claude-specific facts
and workflow rules.

## Status snapshot (2026-06-12)

| Metric | Value |
|--------|-------|
| Format version | SCLS v1 (CIP-0165), RAW chunks only |
| Test functions | 60+ (100% passing), 4 godoc examples, 3 benchmarks |
| Fuzz targets | 2 (`FuzzVerify`, `FuzzRoundTrip`) |
| Conformance fixtures | 1 (Haskell-generated `minimal-raw.scls`, passes VerifyFull) |
| Direct dependencies | `golang.org/x/crypto` only |
| Go floor | 1.25 (CI matrix 1.25.x/1.26.x) |

Update this table when counts change.

## Tool preferences

- Grep tool, not `grep`/`rg` via Bash.
- Glob tool, not `find` via Bash.
- Parallel tool calls for independent reads.

## Test commands

```bash
make test     # all unit tests with -race
make format   # gofmt -s
go test -run=NONE -fuzz=FuzzVerify -fuzztime=2m .   # before claiming verify-path changes safe
SCLS_CONFORMANCE_FILE=/path/to/ref.scls go test -run TestConformance -v .
```

## Rules Claude gets wrong without being told

1. The wire format has **no CBOR** despite the CIP prose mentioning
   "CBOR-encoded string key" — that prose is contradicted by the Kaitai
   definition and both reference implementations. Keys are raw fixed-size
   bytes. Read `spec/RECONCILIATION.md` before touching any codec.
2. The plan-time byte layouts in old docs/commits may differ from the spec;
   `spec/RECONCILIATION.md` is the single source of truth for layout
   questions, with the vendored `spec/format/format.ksy` behind it.
3. Hex test vectors in `*_test.go` are reproducible: hashing vectors via
   `hashlib.blake2b(digest_size=28)` (Python) or the documented preimages;
   Merkle golden vectors come from the Haskell reference (see
   `merkle_test.go`). Never hand-edit a vector to make a test pass.
4. `Writer.Close` is not idempotent (`ErrClosed` on second call) and does
   not close the underlying `io.Writer`.
5. Changes to `decodeChunk`/`decodeManifest`/`readRecord` must keep
   `FuzzVerify` panic-free — attacker-controlled lengths must be bounds-
   checked before allocation or slicing (see `maxRecordSize`, capped
   pre-allocations).
6. `testdata/minimal-raw.scls` is a vendored third-party fixture
   (Apache-2.0, attribution in `testdata/README.md`) — regenerating or
   "fixing" it is never correct.
7. GPG signing required; conventional commit messages.

## Related

`AGENTS.md`, `README.md`, `spec/RECONCILIATION.md`,
`docs/superpowers/plans/2026-06-11-scls-container-format.md` (implementation
plan with per-task TDD history).
