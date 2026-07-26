# Namespaces

This directory contains the supported Cardano namespace specifications.
Each namespace defines a non-intersecting slice of the ledger state.

The catalog and generated CDDL are synchronized with
[`cardano-cls` commit `a7ddf62e6291b297889e78b4df95e2fb605c312f`](https://github.com/koslambrou/cardano-cls/tree/a7ddf62e6291b297889e78b4df95e2fb605c312f/scls-cardano/cddl-src/Cardano/SCLS).

| Shortname                                      | Content                    | Key size | Key description                          |
| ---------------------------------------------- | -------------------------- | -------- | ---------------------------------------- |
| blocks/v0                                      | Blocks created             | 36       | stake pool key hash and epoch            |
| entities/accounts/v0                           | Accounts                   | 29       | credential                               |
| entities/committee/v0                          | Committee entity state     | 1        | singleton zero key                       |
| entities/dreps/v0                              | DReps                      | 29       | credential                               |
| entities/stake_pools/v0                        | Stake pools                | 28       | stake pool key hash                      |
| entities/stake_pools/vrf_key_hashes/v0         | Stake pool VRF key hashes  | 32       | stake pool VRF key hash                  |
| gov/committee/v0                               | Governance committee       | 1        | singleton zero key                       |
| gov/constitution/v0                            | Constitution               | 1        | singleton zero key                       |
| gov/pparams/v0                                 | Protocol parameters        | 4        | previous, current, or future selector    |
| gov/proposals/v0                               | Governance proposals       | 34       | transaction and governance action index  |
| gov/proposals/roots/v0                         | Governance proposal roots  | 1        | governance proposal purpose              |
| nonces/v0                                      | Nonces                     | 1        | singleton zero key                       |
| snapshots/mark/v0                              | Mark snapshot              | 31       | key type, key hash, and value type       |
| snapshots/set/v0                               | Set snapshot               | 31       | key type, key hash, and value type       |
| snapshots/go/v0                                | Go snapshot                | 31       | key type, key hash, and value type       |
| utxo/v0                                        | UTXOs                      | 34       | transaction ID and output index          |

Key layouts are described in each CDDL specification's comments.
