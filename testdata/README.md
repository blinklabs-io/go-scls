# Conformance fixtures

## minimal-raw.scls

Copied from the [`tweag/cardano-scrawls`](https://github.com/tweag/cardano-scrawls)
Rust reference implementation (`tests/fixtures/minimal-raw.scls`, Apache-2.0),
which in turn generated it with the Haskell reference implementation
([`tweag/cardano-cls`](https://github.com/tweag/cardano-cls)):

```sh
scls-util debug generate minimal-raw.scls --namespace blocks/v0:1
```

Contents: HDR (v1), one `blocks/v0` CHUNK with a single 36-byte-key entry,
and a MANIFEST. Note the chunk's sequence number is 1 — the Haskell tool
numbers chunks from 1 while cardano-scrawls and go-scls number from 0; the
spec only requires strict monotonic increase within a namespace (see
`spec/RECONCILIATION.md` row 9), making this fixture a useful check that
verification does not assume a 0 start.

Additional fixtures can be verified without copying them here by pointing
`SCLS_CONFORMANCE_FILE` at any externally generated `.scls` file.
