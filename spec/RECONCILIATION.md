# spec/RECONCILIATION.md — wire-format decisions vs CIP-0165

Sources consulted (2026-06-11):

- **KSY**: vendored `spec/format/format.ksy` (Kaitai Struct container definition from
  CIP-0165 `format/`; the CIP ships this instead of container-level CDDL — the
  `namespaces/*.cddl` files only describe entry *payloads*, which are out of scope
  for v0.1)
- **CIP**: vendored `spec/README.md` (CIP-0165 prose)
- **RUST**: `tweag/cardano-scrawls` @ `main` (commit cloned 2026-06-11), Rust reference
  (`src/records/*.rs`, `src/hash/*.rs`, `src/writer.rs`, `src/reader/*.rs`)
- **HS**: `tweag/cardano-cls` @ `main`, Haskell reference
  (`scls-format/src/Cardano/SCLS/Internal/Serializer/Dump.hs`)
- **FIXTURE**: `cardano-scrawls/tests/fixtures/minimal-raw.scls` (316 bytes, generated
  by the Haskell `scls-util debug generate`; byte layout decoded by hand)

Where the original implementation plan (derived from CIP prose) disagreed with these
sources, the sources win and the plan was corrected before implementation.

| # | Question | Answer | Source |
|---|----------|--------|--------|
| 1 | Does the u32 record size prefix cover the type byte or payload only? | Size prefix covers **type byte + payload** (`size = 1 + len(data)`). HDR record writes size 9 = type(1)+magic(4)+version(4). The KSY doc comment "including size" is wrong w.r.t. the size field itself: structurally `payload` has `size: len_payload` and contains the type byte first (`rec_payload_size = len_payload - 1`). Zero-size records are invalid (cannot contain the type byte). | KSY `scls_record`; RUST `header.rs:46`, `reader/mod.rs:181-186`; FIXTURE bytes 0–3 (`00000009`) |
| 2 | HDR payload fields beyond magic+version? | None. Payload = magic `"SCLS"` (4) ‖ version **u32 BE** (4) = 8 bytes. CIP prose says `u16` — outdated; KSY, Rust, and the fixture all use u32. Version starts at 1. Rust requires exactly 8 bytes; CIP extensibility prose says unknown HDR fields may be skipped — we tolerate trailing bytes on read, write exactly 8. | KSY `rec_header`; RUST `header.rs` (u32, len==8); FIXTURE bytes 4–12 |
| 3 | CHUNK namespace encoding? | Raw length-prefixed UTF-8: `len_ns` **u32 BE** ‖ ns bytes. **Not** CBOR. After the namespace comes `len_key` u32 BE (fixed key size for the namespace). Full CHUNK payload: `seqno u64 ‖ format u8 ‖ len_ns u32 ‖ ns ‖ len_key u32 ‖ entries ‖ entries_count u32 ‖ digest(28)`. | KSY `rec_chunk`; RUST `chunk.rs:parse`, `writer.rs:flush_chunk`; FIXTURE |
| 4 | DataEntry layout? | `len_body` u32 BE ‖ `key` (raw bytes, exactly `len_key` from the chunk header) ‖ `value` (raw bytes, `len_body - len_key`). `len_body` covers key+value only. The key is **not** CBOR-framed (the CIP "Entries" prose saying "CBOR-encoded string key" contradicts both the KSY and both reference impls; KSY/impls win). Value is caller-supplied canonical CBOR written verbatim — the container itself never CBOR-encodes anything, so this library needs **no CBOR dependency**. | KSY `entry`/`entry_body`; RUST `chunk.rs:for_each_entry`, `writer.rs:update_chunk` |
| 5 | Entry digest preimage? | `Blake2b-224(0x01 ‖ ns_utf8 ‖ key ‖ value)` over **raw** key/value bytes (no length prefixes, no CBOR framing). Matches the plan. | CIP §Verification; RUST `writer.rs:340-344`, `chunk.rs:verify_and` |
| 6 | Chunk footer layout and hash preimage? | Footer = `entries_count` u32 BE ‖ `chunk_hash` (28 bytes). `chunk_hash = Blake2b-224(concat(entry_digest(e) for e in entries))` — plain hash, no domain-separator prefix. Computed over uncompressed entries. Matches the plan. | KSY `rec_chunk`; CIP §CHUNK policy; RUST `chunk.rs:verify_and`, `writer.rs` |
| 7 | MANIFEST encoding? | Pure binary, **no CBOR**, field order differs from CIP prose (KSY/impls win): `slot_no u64 ‖ total_entries u64 ‖ total_chunks u64 ‖ summary ‖ namespace_info* ‖ sentinel u32(0) ‖ prev_manifest u64 ‖ root_hash(28) ‖ offset u32`. `summary` = 3 `tstr` (u32 BE length ‖ UTF-8): created_at, tool, comment (empty string = no comment). Each `namespace_info` = `len_ns u32 ‖ entries_count u64 ‖ chunks_count u64 ‖ name ‖ digest(28)`; the list is terminated by `len_ns == 0`. `offset` equals the record's u32 size prefix value (type+payload length) — the record is bookended by the same value so the manifest can be found from the last 4 bytes of the file. Decoders must reject an offset that does not match and reject trailing bytes. | KSY `rec_manifest`; RUST `manifest.rs` (esp. `offset()` and `try_from`); FIXTURE bytes 0x7d–0x13b |
| 8 | Empty-tree root? | `Blake2b-224("")` (hash of empty input), for both an empty namespace and an empty file (global root with no namespaces). | CIP §Verification ("H(\"\")"); RUST `merkle.rs:EMPTY` |
| 9 | chunk_seq semantics? | **Per-namespace**, reset on namespace change, strictly increasing within a namespace. Start value is NOT pinned by the spec: Rust writes from 0, the Haskell reference writes from 1 (`S.zip (S.enumFrom 1)`), and Rust `verify(full)` accepts the Haskell fixture. Therefore: we **write** starting at 0 (parity with the Rust crate) and **verify** only strict monotonic increase within each namespace, any start value. | RUST `writer.rs:INITIAL_CHUNK_SEQNO`, `verify.rs:check_chunk_ordering`; HS `Dump.hs:146`; FIXTURE (seqno=1, verifies OK) |
| 10 | Per-namespace key length? | Fixed per namespace; the chunk header carries `len_key` u32. Writers must reject inconsistent key lengths within a namespace; readers must reject `len_body < len_key`. Note: the vendored KSY declares the `len_key` *parameter* of `entries_record`/`entry`/`entry_body` as `u2` — an upstream KSY defect that narrows the 32-bit wire field; the wire field is u4 and this library treats it as u32 throughout. | KSY `rec_chunk.len_key` ("key size for this namespace"); RUST `writer.rs:update_chunk`, `chunk.rs:for_each_entry` |
| 11 | Merkle promotion rule? | Confirmed promote-then-merge: insert leaves left-to-right, merge equal-depth neighbours immediately; on finalization promote the shallower trailing subtree (hash unchanged) until depths match, then merge `H(0x00 ‖ l ‖ r)`. Equivalent to a recursive split at the largest power of two below the leaf count (left subtree full). Verified against the Haskell-generated golden vectors (leaf `H(0x01‖"Test")`, n∈{2,3,4,5,7}) which are reproduced in `merkle_test.go`. | CIP §Verification; RUST `merkle.rs:collapse` + golden test |
| 12 | Global tree leaf? | `Blake2b-224(0x01 ‖ ns_root)`; the namespace **name is not included**. Namespace roots taken in lexicographic (bytewise UTF-8) order. Matches the plan. | CIP §Global Merkle tree; RUST `writer.rs:finalise`, `verify.rs:check_manifest_integrity` |

## Additional decisions discovered during reconciliation

| # | Decision | Rationale / Source |
|---|----------|--------------------|
| E1 | **Drop the `fxamacker/cbor` dependency.** The container framing uses no CBOR anywhere (#3, #4, #7); entry values are opaque bytes written/read verbatim and neither reference implementation validates them as CBOR. Only direct dependency: `golang.org/x/crypto`. | RUST `Cargo.toml` (no CBOR dep at all) |
| 2b | Header version ≠ 1 → reject with `ErrUnsupportedVersion` on decode. Rust parses any version and leaves the check to callers; since v1 is the only defined layout we reject eagerly. | RUST `header.rs` note |
| E3 | MANIFEST `namespace_info` order is **not significant** ("The order of the namespaces does not change the signatures"). Verify compares manifest namespaces to computed namespaces as a **set keyed by name** and rejects duplicate names. The global Merkle tree is always built from roots in lexicographic name order regardless of manifest order. | CIP §Supported Namespaces; RUST `verify.rs:build_ns_info` |
| E4 | Record sequence: `HDR CHUNK* MANIFEST` then EOF for v0.1. Unknown/reserved record types (BLOOM, INDEX, DIR, META, and truly unknown tags) are skipped. Rust skips DELTA as "unknown"; we reject DELTA explicitly with `ErrUnexpectedRecord` — silently skipping deltas would verify a live-set different from the file's true content. Records after the MANIFEST (other than skippable types) are rejected; multiple MANIFESTs are rejected. | RUST `seq_state.rs`; CIP file layout |
| E5 | META (0x31) has a defined layout in the KSY (URI-keyed entries + footer) but both v0.1 reference readers skip it; we skip it too. | RUST `reader/mod.rs:parse` |
| E6 | Conformance fixture: `testdata/minimal-raw.scls` vendored from `cardano-scrawls` `tests/fixtures/minimal-raw.scls` (Apache-2.0), originally generated by the Haskell `scls-util debug generate minimal-raw.scls --namespace blocks/v0:1`. | See `testdata/README.md` |
