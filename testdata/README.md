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

## Rust-generated fixtures

`empty.scls`, `multi-ns.scls`, `multi-chunk.scls`, and
`multi-ns-multi-chunk.scls` were produced by the Tweag Rust reference writer
([`tweag/cardano-scrawls`](https://github.com/tweag/cardano-scrawls),
Apache-2.0 — Copyright 2026 Tweag SARL), commit `fc197ff`, via the generator
below. They are committed verbatim; `created_at` in each MANIFEST is the
generation timestamp, so re-running reproduces the structure but not the exact
bytes (as with `minimal-raw.scls`). Their global roots and per-namespace
digests are pinned in `TestConformanceGolden`.

They use synthetic namespace names (`ns/alpha`, …) and opaque values on
purpose: the goal is to exercise the SCLS **container** (framing, ordering,
multi-namespace global tree, multi-chunk per-namespace trees, the empty-file
`H("")` root, and `value_len == 0` entries), not namespace payload schemas.

| Fixture | Exercises |
|---------|-----------|
| `empty.scls` | no entries → global root = `Blake2b-224("")` |
| `multi-ns.scls` | 3 namespaces, distinct key lengths, empty values |
| `multi-chunk.scls` | 1 namespace, 50 entries over 9 chunks (seqno 0..8) |
| `multi-ns-multi-chunk.scls` | 2 namespaces, each spanning multiple chunks |

Generator (`examples/gen_fixtures.rs` in a checkout of `cardano-scrawls`,
run with `cargo run --example gen_fixtures -- <out-dir>`):

```rust
use std::fs::File;
use std::path::Path;
use cardano_scrawls::writer::SclsWriter;

fn key_be(width: usize, n: u64) -> Vec<u8> { n.to_be_bytes()[8 - width..].to_vec() }

fn build<F>(path: &Path, max_chunk: Option<usize>, fill: F)
where F: FnOnce(&mut SclsWriter<File>) {
    let mut b = SclsWriter::builder().output(File::create(path).unwrap())
        .slot_no(42).tool("cardano-scrawls").comment("go-scls conformance fixture");
    if let Some(m) = max_chunk { b = b.max_chunk_size(m); }
    let mut w = b.build().unwrap();
    fill(&mut w);
    w.finalise().unwrap();
}

fn main() {
    let dir = std::env::args().nth(1).unwrap_or_else(|| ".".into());
    let p = |n: &str| Path::new(&dir).join(n);
    build(&p("empty.scls"), None, |_w| {});
    build(&p("multi-ns.scls"), None, |w| {
        for i in 0..3u64 { w.write_entry("ns/alpha", &key_be(2, i), &[0xa0, i as u8]).unwrap(); }
        for i in 0..2u64 { w.write_entry("ns/beta", &key_be(4, i), &[0xb0, i as u8, 0xff]).unwrap(); }
        for i in 0..4u64 { w.write_entry("ns/gamma", &key_be(8, i), &[]).unwrap(); }
    });
    build(&p("multi-chunk.scls"), Some(96), |w| {
        for i in 0..50u64 { w.write_entry("ns/alpha", &key_be(4, i), &[i as u8; 8]).unwrap(); }
    });
    build(&p("multi-ns-multi-chunk.scls"), Some(96), |w| {
        for i in 0..20u64 { w.write_entry("ns/alpha", &key_be(4, i), &[0xaa; 12]).unwrap(); }
        for i in 0..25u64 { w.write_entry("ns/beta", &key_be(6, i), &[0xbb; 10]).unwrap(); }
    });
}
```

Additional fixtures can be verified without copying them here by pointing
`SCLS_CONFORMANCE_FILE` at any externally generated `.scls` file.
