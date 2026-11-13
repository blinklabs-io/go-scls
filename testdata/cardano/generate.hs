-- Copyright 2026 Blink Labs Software
--
-- Licensed under the Apache License, Version 2.0 (the "License");
-- you may not use this file except in compliance with the License.
-- You may obtain a copy of the License at
--
--     http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing, software
-- distributed under the License is distributed on an "AS IS" BASIS,
-- WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
-- See the License for the specific language governing permissions and
-- limitations under the License.

{-# LANGUAGE LambdaCase #-}

-- This dependency-free Haskell generator mirrors the canonical encoders at:
--
-- cardano-cls:
--   a7ddf62e6291b297889e78b4df95e2fb605c312f
-- cardano-ledger:
--   fb8d6f8a83b0efb86281e0b80e7ffba160dad8b1
--
-- Run from the repository root:
--   runghc testdata/cardano/generate.hs

import Control.Monad (forM_)
import Data.Bits ((.|.), shiftR)
import qualified Data.ByteString as BS
import qualified Data.ByteString.Builder as Builder
import qualified Data.ByteString.Lazy as LBS
import Data.List (sortOn)
import Data.Word (Word64, Word8)
import System.Directory (createDirectoryIfMissing)
import System.FilePath ((</>))

data Term
  = UInt Word64
  | Bytes BS.ByteString
  | Text String
  | Array [Term]
  | Map [(Term, Term)]
  | Tag Word64 Term
  | Null
  | NegInt Word64
  | MapInOrder [(Term, Term)]
  | Raw BS.ByteString

outputDirectory :: FilePath
outputDirectory = "testdata/cardano"

main :: IO ()
main = do
  createDirectoryIfMissing True outputDirectory
  forM_ keyFixtures $ \(name, value) ->
    BS.writeFile (outputDirectory </> name <> ".key") value
  forM_ payloadFixtures $ \(name, value) ->
    BS.writeFile (outputDirectory </> name <> ".cbor") (encode value)
  forM_ negativeFixtures $ \(name, value) ->
    BS.writeFile (outputDirectory </> name) value
  forM_ typedPayloadFixtures $ \(name, value) ->
    BS.writeFile (outputDirectory </> name <> ".cbor") (encode value)
  forM_ schemaInvalidFixtures $ \(name, value) ->
    BS.writeFile (outputDirectory </> name <> ".cbor") (encode value)

keyFixtures :: [(String, BS.ByteString)]
keyFixtures =
  [ ("blocks_v0", bytesFrom 0x10 28 <> word64BE 100)
  , ("utxo_v0", bytesFrom 0x20 32 <> BS.pack [0, 1])
  , ("entities_accounts_v0", BS.singleton 0 <> bytesFrom 0x30 28)
  , ("entities_committee_v0", BS.singleton 0)
  , ("entities_dreps_v0", BS.singleton 1 <> bytesFrom 0x40 28)
  , ("entities_stake_pools_v0", bytesFrom 0x50 28)
  , ("entities_stake_pools_vrf_key_hashes_v0", bytesFrom 0x60 32)
  , ("gov_committee_v0", BS.singleton 0)
  , ("gov_constitution_v0", BS.singleton 0)
  , ("gov_pparams_v0", BS.pack (map (fromIntegral . fromEnum) "curr"))
  , ("gov_proposals_v0", bytesFrom 0x70 32 <> BS.pack [0, 2])
  , ("gov_proposals_roots_v0", BS.singleton 0)
  , ("nonces_v0", BS.singleton 0)
  , ("snapshots_mark_v0", BS.replicate 31 0)
  , ("snapshots_set_v0", BS.replicate 31 0)
  , ("snapshots_go_v0", BS.replicate 31 0)
  ]

payloadFixtures :: [(String, Term)]
payloadFixtures =
  [ ("blocks_v0", UInt 0)
  , ("utxo_v0", Bytes (encode (Array [Bytes (bytesFrom 0xe0 29), UInt 0])))
  , ( "entities_accounts_v0"
    , Map
        [ field "balance" (UInt 0)
        , field "deposit" (UInt 0)
        , field "drep_delegation" Null
        , field "stake_pool_delegation" Null
        ]
    )
  , ("entities_committee_v0", Map [])
  , ( "entities_dreps_v0"
    , Map
        [ field "expiry" (UInt 0)
        , field "anchor" Null
        , field "deposit" (UInt 0)
        , field "delegations" (set [])
        ]
    )
  , ("entities_stake_pools_v0_without_leios", stakePool False)
  , ("entities_stake_pools_v0_with_leios", stakePool True)
  , ("entities_stake_pools_vrf_key_hashes_v0", UInt 1)
  , ("gov_committee_v0", Null)
  , ( "gov_constitution_v0"
    , Array [Array [Text "", Bytes (bytesFrom 0x80 32)], Null]
    )
  , ("gov_pparams_v0", protocolParameters)
  , ("gov_proposals_v0", governanceProposal)
  , ("gov_proposals_roots_v0", Array [Bytes (bytesFrom 0x90 32), UInt 0])
  , ("nonces_v0", nonces)
  , ("snapshots_mark_v0", Array [UInt 0, UInt 100])
  , ("snapshots_set_v0", Array [UInt 0, UInt 200])
  , ("snapshots_go_v0", Array [UInt 0, UInt 300])
  ]

negativeFixtures :: [(FilePath, BS.ByteString)]
negativeFixtures =
  [ ("invalid_key_short.key", BS.replicate 27 0)
  , ("invalid_key_long.key", BS.replicate 29 0)
  , ( "invalid_bls_public_key.cbor"
    , encode (stakePoolWithLeiosLengths 95 48)
    )
  , ( "invalid_bls_public_key_long.cbor"
    , encode (stakePoolWithLeiosLengths 97 48)
    )
  , ( "invalid_bls_proof.cbor"
    , encode (stakePoolWithLeiosLengths 96 47)
    )
  , ( "invalid_bls_proof_long.cbor"
    , encode (stakePoolWithLeiosLengths 96 49)
    )
  , ("invalid_field_order.cbor", invalidFieldOrder)
  , ("invalid_duplicate_field.cbor", invalidDuplicateField)
  , ("invalid_noncanonical.cbor", BS.pack [0x18, 0x00])
  ]

stakePool :: Bool -> Term
stakePool includeLeios =
  Map
    [ field "stake_pool_state" (stakePoolState includeLeios 96 48)
    , field "retiring_epoch_no" Null
    , field "future_stake_pool_params" (stakePoolParams includeLeios 96 48)
    ]

stakePoolWithLeiosLengths :: Int -> Int -> Term
stakePoolWithLeiosLengths publicKeyLength proofLength =
  Map
    [ field
        "stake_pool_state"
        (stakePoolState True publicKeyLength proofLength)
    , field "retiring_epoch_no" Null
    , field "future_stake_pool_params" Null
    ]

stakePoolState :: Bool -> Int -> Int -> Term
stakePoolState includeLeios publicKeyLength proofLength =
  Map $
    [ field "vrf" (Bytes (bytesFrom 0xa0 32))
    , field "cost" (UInt 340)
    , field "margin" unitInterval
    , field "owners" (set [Bytes (bytesFrom 0xb0 28)])
    , field "pledge" (UInt 1000000)
    , field "relays" (Array [])
    , field "deposit" (UInt 500)
    , field "metadata" Null
    ]
      <> leiosField includeLeios publicKeyLength proofLength
      <> [ field "account_id" credential
         , field "delegators" (set [])
         ]

stakePoolParams :: Bool -> Int -> Int -> Term
stakePoolParams includeLeios publicKeyLength proofLength =
  Map $
    [ field "id" (Bytes (bytesFrom 0xc0 28))
    , field "vrf" (Bytes (bytesFrom 0xa0 32))
    , field "cost" (UInt 350)
    , field "margin" unitInterval
    , field "owners" (set [Bytes (bytesFrom 0xb0 28)])
    , field "pledge" (UInt 2000000)
    , field "relays" (Array [])
    , field "metadata" Null
    ]
      <> leiosField includeLeios publicKeyLength proofLength
      <> [field "account_address" (Bytes (bytesFrom 0xd0 29))]

leiosField :: Bool -> Int -> Int -> [(Term, Term)]
leiosField includeLeios publicKeyLength proofLength
  | includeLeios =
      [ field
          "leios_key"
          ( Array
              [ Bytes (bytesFrom 0x01 publicKeyLength)
              , Bytes (bytesFrom 0x81 proofLength)
              ]
          )
      ]
  | otherwise = []

credential :: Term
credential = Array [UInt 0, Bytes (bytesFrom 0xe0 28)]

unitInterval :: Term
unitInterval = Tag 30 (Array [UInt 1, UInt 10])

set :: [Term] -> Term
set values = Tag 258 (Array (sortOn encode values))

protocolParameters :: Term
protocolParameters =
  Map
    [ numeric 0 (UInt 0)
    , numeric 1 (UInt 0)
    , numeric 2 (UInt 0)
    , numeric 3 (UInt 0)
    , numeric 4 (UInt 0)
    , numeric 5 (UInt 0)
    , numeric 6 (UInt 0)
    , numeric 7 (UInt 0)
    , numeric 8 (UInt 0)
    , numeric 9 nonnegativeInterval
    , numeric 10 unitInterval
    , numeric 11 unitInterval
    , numeric 14 (Array [UInt 0, UInt 0])
    , numeric 16 (UInt 0)
    , numeric 17 (UInt 0)
    , numeric 18 (Map [])
    , numeric 19 (Array [nonnegativeInterval, nonnegativeInterval])
    , numeric 20 (Array [UInt 0, UInt 0])
    , numeric 21 (Array [UInt 0, UInt 0])
    , numeric 22 (UInt 0)
    , numeric 23 (UInt 0)
    , numeric 24 (UInt 0)
    , numeric 25 (Array (replicate 5 unitInterval))
    , numeric 26 (Array (replicate 10 unitInterval))
    , numeric 27 (UInt 0)
    , numeric 28 (UInt 0)
    , numeric 29 (UInt 0)
    , numeric 30 (UInt 0)
    , numeric 31 (UInt 0)
    , numeric 32 (UInt 0)
    , numeric 33 nonnegativeInterval
    ]

nonnegativeInterval :: Term
nonnegativeInterval = Tag 30 (Array [UInt 0, UInt 1])

governanceProposal :: Term
governanceProposal =
  Array
    [ UInt 0
    , Map
        [ field "drep_votes" (Map [])
        , field "proposed_in" (UInt 0)
        , field "expires_after" (UInt 0)
        , field "committee_votes" (Map [])
        , field "stake_pool_votes" (Map [])
        , field "proposal_procedure" (Bytes (encode proposalProcedure))
        ]
    ]

proposalProcedure :: Term
proposalProcedure =
  Array
    [ UInt 0
    , Bytes (bytesFrom 0xf0 29)
    , Array [UInt 6]
    , Array [Text "", Bytes (bytesFrom 0x10 32)]
    ]

nonces :: Term
nonces =
  Map
    [ field "lab_nonce" neutralNonce
    , field "last_slot" (Array [UInt 0])
    , field "epoch_nonce" neutralNonce
    , field "cert_counters" (Map [])
    , field "evolving_nonce" neutralNonce
    , field "candidate_nonce" neutralNonce
    , field "last_epoch_block_nonce" neutralNonce
    ]

neutralNonce :: Term
neutralNonce = Array [UInt 0]

invalidFieldOrder :: BS.ByteString
invalidFieldOrder =
  encodeMapInOrder
    [ field "future_stake_pool_params" Null
    , field "retiring_epoch_no" Null
    , field "stake_pool_state" Null
    ]

invalidDuplicateField :: BS.ByteString
invalidDuplicateField =
  encodeMapInOrder
    [ field "stake_pool_state" Null
    , field "stake_pool_state" Null
    , field "retiring_epoch_no" Null
    , field "future_stake_pool_params" Null
    ]

field :: String -> Term -> (Term, Term)
field name value = (Text name, value)

numeric :: Word64 -> Term -> (Term, Term)
numeric key value = (UInt key, value)

encode :: Term -> BS.ByteString
encode = LBS.toStrict . Builder.toLazyByteString . encodeBuilder

encodeBuilder :: Term -> Builder.Builder
encodeBuilder = \case
  UInt value -> encodeHead 0 value
  Bytes value -> encodeHead 2 (fromIntegral (BS.length value)) <> Builder.byteString value
  Text value ->
    let encoded = BS.pack (map (fromIntegral . fromEnum) value)
     in encodeHead 3 (fromIntegral (BS.length encoded)) <> Builder.byteString encoded
  Array values ->
    encodeHead 4 (fromIntegral (length values)) <> foldMap encodeBuilder values
  Map values ->
    let encoded =
          sortOn
            fst
            [(encode key, (key, value)) | (key, value) <- values]
     in encodeHead 5 (fromIntegral (length values))
          <> foldMap
            (\(_, (key, value)) -> encodeBuilder key <> encodeBuilder value)
            encoded
  Tag number value -> encodeHead 6 number <> encodeBuilder value
  Null -> Builder.word8 0xf6
  NegInt value -> encodeHead 1 value
  MapInOrder values ->
    encodeHead 5 (fromIntegral (length values))
      <> foldMap (\(key, value) -> encodeBuilder key <> encodeBuilder value) values
  Raw value -> Builder.byteString value

encodeMapInOrder :: [(Term, Term)] -> BS.ByteString
encodeMapInOrder values =
  LBS.toStrict . Builder.toLazyByteString $
    encodeHead 5 (fromIntegral (length values))
      <> foldMap
        (\(key, value) -> encodeBuilder key <> encodeBuilder value)
        values

encodeHead :: Word8 -> Word64 -> Builder.Builder
encodeHead major value
  | value < 24 = Builder.word8 (major `shiftLeftFive` fromIntegral value)
  | value <= 0xff =
      Builder.word8 (major `shiftLeftFive` 24) <> Builder.word8 (fromIntegral value)
  | value <= 0xffff =
      Builder.word8 (major `shiftLeftFive` 25) <> Builder.word16BE (fromIntegral value)
  | value <= 0xffffffff =
      Builder.word8 (major `shiftLeftFive` 26) <> Builder.word32BE (fromIntegral value)
  | otherwise =
      Builder.word8 (major `shiftLeftFive` 27) <> Builder.word64BE value

shiftLeftFive :: Word8 -> Word8 -> Word8
shiftLeftFive major additional = (major * 32) .|. additional

bytesFrom :: Word8 -> Int -> BS.ByteString
bytesFrom start count =
  BS.pack [start + fromIntegral offset | offset <- [0 .. count - 1]]

word64BE :: Word64 -> BS.ByteString
word64BE value =
  BS.pack
    [ fromIntegral (value `shiftR` 56)
    , fromIntegral (value `shiftR` 48)
    , fromIntegral (value `shiftR` 40)
    , fromIntegral (value `shiftR` 32)
    , fromIntegral (value `shiftR` 24)
    , fromIntegral (value `shiftR` 16)
    , fromIntegral (value `shiftR` 8)
    , fromIntegral value
    ]

-- Typed fixtures. Each value exercises fields the minimal fixtures leave
-- empty. They are written to <name>.cbor beside the minimal fixtures.

typedPayloadFixtures :: [(String, Term)]
typedPayloadFixtures =
  [ ("blocks_v0_large", UInt 4294967296)
  , ( "entities_accounts_v0_delegated"
    , Map
        [ field "balance" (UInt 5000000)
        , field "deposit" (UInt 2000000)
        , field "drep_delegation" (Array [UInt 1, Bytes (bytesFrom 0x01 28)])
        , field "stake_pool_delegation" (Bytes (bytesFrom 0x50 28))
        ]
    )
  , ( "entities_accounts_v0_abstain"
    , Map
        [ field "balance" (UInt 1)
        , field "deposit" (UInt 0)
        , field "drep_delegation" (Array [UInt 2])
        , field "stake_pool_delegation" Null
        ]
    )
  , ( "entities_dreps_v0_populated"
    , Map
        [ field "anchor" (anchor "https://example.invalid/drep.json" 0x20)
        , field "expiry" (UInt 400)
        , field "deposit" (UInt 500000000)
        , field "delegations" (set [credential, scriptCredential])
        ]
    )
  , ( "entities_committee_v0_members"
    , Map
        [ (credential, Array [UInt 0, scriptCredential])
        , (scriptCredential, Array [UInt 1, anchor "https://example.invalid/resign" 0x30])
        , (Array [UInt 0, Bytes (bytesFrom 0x10 28)], Array [UInt 1, Null])
        ]
    )
  , ("entities_stake_pools_vrf_key_hashes_v0_max", UInt 18446744073709551615)
  , ( "gov_committee_v0_members"
    , Array
        [ Map [(credential, UInt 120), (scriptCredential, UInt 130)]
        , Tag 30 (Array [UInt 2, UInt 3])
        ]
    )
  , ( "gov_constitution_v0_guardrails"
    , Array [anchor "ipfs://constitution" 0x40, Bytes (bytesFrom 0x60 28)]
    )
  , ("gov_pparams_v0_populated", populatedParameters)
  , ("gov_proposals_roots_v0_max_index", Array [Bytes (bytesFrom 0x90 32), UInt 65535])
  , ( "nonces_v0_populated"
    , Map
        [ field "lab_nonce" (justNonce 0x01)
        , field "last_slot" (Array [UInt 1, UInt 123456])
        , field "epoch_nonce" (justNonce 0x21)
        , field "cert_counters" (Map [(Bytes (bytesFrom 0x50 28), UInt 3), (Bytes (bytesFrom 0x70 28), UInt 9)])
        , field "evolving_nonce" (justNonce 0x41)
        , field "candidate_nonce" neutralNonce
        , field "last_epoch_block_nonce" (justNonce 0x61)
        ]
    )
  , ("snapshots_mark_v0_pool", Array [UInt 1, Bytes (bytesFrom 0x50 28)])
  , ( "utxo_v0_shelley_assets"
    , Bytes
        ( encode
            ( Array
                [ Bytes (bytesFrom 0xe0 29)
                , Array [UInt 2000000, Map [(Bytes (bytesFrom 0x01 28), Map [(Bytes (BS.pack [1, 2]), UInt 5)])]]
                , Bytes (bytesFrom 0x90 32)
                ]
            )
        )
    )
  , ( "utxo_v0_babbage_datum_script"
    , Bytes
        ( encode
            ( Map
                [ (UInt 0, Bytes (bytesFrom 0x70 57))
                , (UInt 1, Array [UInt 3000000, Map [(Bytes (bytesFrom 0x01 28), Map [(Bytes BS.empty, UInt 1)])]])
                , (UInt 2, Array [UInt 1, Tag 24 (Bytes (encode (Tag 121 (Array []))))])
                , (UInt 3, Tag 24 (Bytes (encode (Array [UInt 2, Bytes (BS.pack [0x4e, 0x4d, 0x01, 0x00, 0x00, 0x00])]))))
                ]
            )
        )
    )
  , ( "utxo_v0_ledger_order"
    , Bytes
        ( encode
            ( Array
                [ Bytes (bytesFrom 0xe0 29)
                , Array
                    [ UInt 1
                    , Map
                        [ ( Bytes (bytesFrom 0x01 28)
                          , MapInOrder
                              [ (Bytes (BS.pack [0, 0]), UInt 1)
                              , (Bytes (BS.pack [255]), UInt 2)
                              ]
                          )
                        ]
                    ]
                ]
            )
        )
    )
  , ( "utxo_v0_indefinite_datum"
    , Bytes
        ( encode
            ( Map
                [ (UInt 0, Bytes (bytesFrom 0xe0 29))
                , (UInt 1, UInt 1)
                , (UInt 2, Array [UInt 1, Tag 24 (Bytes (BS.pack [0x9f, 0x01, 0xff]))])
                ]
            )
        )
    )
  ]
    <> [ ("gov_proposals_v0_" <> name, governanceProposalWith 0 (procedure action))
       | (name, action) <- governanceActions
       ]
    <> [ ( "gov_proposals_v0_votes"
         , Array
             [ UInt 12
             , Map
                 [ field "drep_votes" (Map [(credential, UInt 1), (scriptCredential, UInt 2)])
                 , field "proposed_in" (UInt 500)
                 , field "expires_after" (UInt 506)
                 , field "committee_votes" (Map [(credential, UInt 0)])
                 , field "stake_pool_votes" (Map [(Bytes (bytesFrom 0x50 28), UInt 1)])
                 , field "proposal_procedure" (Bytes (encode (procedure infoAction)))
                 ]
             ]
         )
       , ( "gov_proposals_v0_ledger_order"
         , governanceProposalWith 1 (ledgerOrderedProcedure)
         )
       ]

credentialPair :: Word64 -> Word8 -> Term
credentialPair kind start = Array [UInt kind, Bytes (bytesFrom start 28)]

scriptCredential :: Term
scriptCredential = credentialPair 1 0x01

anchor :: String -> Word8 -> Term
anchor url start = Array [Text url, Bytes (bytesFrom start 32)]

justNonce :: Word8 -> Term
justNonce start = Array [UInt 1, Bytes (bytesFrom start 32)]

populatedParameters :: Term
populatedParameters =
  Map
    [ numeric 0 (UInt 44)
    , numeric 1 (UInt 155381)
    , numeric 2 (UInt 90112)
    , numeric 3 (UInt 16384)
    , numeric 4 (UInt 1100)
    , numeric 5 (UInt 2000000)
    , numeric 6 (UInt 500000000)
    , numeric 7 (UInt 18)
    , numeric 8 (UInt 500)
    , numeric 9 (Tag 30 (Array [UInt 3, UInt 10]))
    , numeric 10 (Tag 30 (Array [UInt 3, UInt 1000]))
    , numeric 11 (Tag 30 (Array [UInt 1, UInt 5]))
    , numeric 14 (Array [UInt 10, UInt 2])
    , numeric 16 (UInt 170000000)
    , numeric 17 (UInt 4310)
    , numeric 18 (Map [(UInt 0, Array [UInt 1, NegInt 4, UInt 4294967296]), (UInt 2, Array [NegInt 9223372036854775807]), (UInt 3, Array [])])
    , numeric 19 (Array [Tag 30 (Array [UInt 577, UInt 10000]), Tag 30 (Array [UInt 721, UInt 10000000])])
    , numeric 20 (Array [UInt 14000000, UInt 10000000000])
    , numeric 21 (Array [UInt 62000000, UInt 20000000000])
    , numeric 22 (UInt 5000)
    , numeric 23 (UInt 150)
    , numeric 24 (UInt 3)
    , numeric 25 (Array [Tag 30 (Array [UInt (51 + i), UInt 100]) | i <- [0 .. 4]])
    , numeric 26 (Array [Tag 30 (Array [UInt (60 + i), UInt 100]) | i <- [0 .. 9]])
    , numeric 27 (UInt 7)
    , numeric 28 (UInt 146)
    , numeric 29 (UInt 6)
    , numeric 30 (UInt 100000000000)
    , numeric 31 (UInt 500000000)
    , numeric 32 (UInt 20)
    , numeric 33 (Tag 30 (Array [UInt 15, UInt 1]))
    ]

purposeId :: Term
purposeId = Array [Bytes (bytesFrom 0x30 32), UInt 4]

policyHash :: Term
policyHash = Bytes (bytesFrom 0x50 28)

infoAction :: Term
infoAction = Array [UInt 6]

governanceActions :: [(String, Term)]
governanceActions =
  [ ( "parameter_change"
    , Array
        [ UInt 0
        , purposeId
        , Map
            [ numeric 0 (UInt 50)
            , numeric 9 (Tag 30 (Array [UInt 1, UInt 2]))
            , numeric 18 (Map [(UInt 1, Array [NegInt 0, UInt 7])])
            , numeric 19 (Array [Tag 30 (Array [UInt 1, UInt 3]), Tag 30 (Array [UInt 2, UInt 3])])
            , numeric 25 (Array (replicate 5 (Tag 30 (Array [UInt 1, UInt 2]))))
            , numeric 33 (Tag 30 (Array [UInt 25, UInt 2]))
            ]
        , policyHash
        ]
    )
  , ("hard_fork", Array [UInt 1, Null, Array [UInt 11, UInt 0]])
  , ( "treasury_withdrawals"
    , Array
        [ UInt 2
        , Map [(Bytes (bytesFrom 0xe0 29), UInt 1000000), (Bytes (bytesFrom 0xf0 29), UInt 2000000)]
        , policyHash
        ]
    )
  , ("no_confidence", Array [UInt 3, purposeId])
  , ( "update_committee"
    , Array
        [ UInt 4
        , Null
        , set [credential, scriptCredential]
        , Map [(Array [UInt 0, Bytes (bytesFrom 0x10 28)], UInt 200)]
        , Tag 30 (Array [UInt 2, UInt 3])
        ]
    )
  , ("new_constitution", Array [UInt 5, purposeId, Array [anchor "ipfs://new" 0x40, policyHash]])
  , ("info", infoAction)
  ]

procedure :: Term -> Term
procedure action =
  Array [UInt 100000000000, Bytes (bytesFrom 0xe0 29), action, anchor "https://example.invalid/proposal" 0x10]

governanceProposalWith :: Word64 -> Term -> Term
governanceProposalWith order proposalProcedureTerm =
  Array
    [ UInt order
    , Map
        [ field "drep_votes" (Map [])
        , field "proposed_in" (UInt 10)
        , field "expires_after" (UInt 16)
        , field "committee_votes" (Map [])
        , field "stake_pool_votes" (Map [])
        , field "proposal_procedure" (Bytes (encode proposalProcedureTerm))
        ]
    ]

-- The ledger serializes sets and maps in its own key order, in which a
-- script credential precedes a key credential. Deterministic CBOR orders
-- them the other way.
ledgerOrderedProcedure :: Term
ledgerOrderedProcedure =
  procedure
    ( Array
        [ UInt 4
        , Null
        , Tag 258 (Array [scriptCredential, credential])
        , MapInOrder
            [ (scriptCredential, UInt 210)
            , (credential, UInt 200)
            ]
        , Tag 30 (Array [UInt 1, UInt 2])
        ]
    )

-- Payloads that are deterministic CBOR but violate their namespace schema.
schemaInvalidFixtures :: [(String, Term)]
schemaInvalidFixtures =
  [ ( "invalid_accounts_drep_kind"
    , Map
        [ field "balance" (UInt 0)
        , field "deposit" (UInt 0)
        , field "drep_delegation" (Array [UInt 4])
        , field "stake_pool_delegation" Null
        ]
    )
  , ( "invalid_accounts_pool_hash_short"
    , Map
        [ field "balance" (UInt 0)
        , field "deposit" (UInt 0)
        , field "drep_delegation" Null
        , field "stake_pool_delegation" (Bytes (bytesFrom 0x50 27))
        ]
    )
  , ("invalid_blocks_negative", NegInt 0)
  , ( "invalid_dreps_anchor_url_long"
    , Map
        [ field "anchor" (Array [Text (replicate 129 'a'), Bytes BS.empty])
        , field "expiry" (UInt 0)
        , field "deposit" (UInt 0)
        , field "delegations" (set [])
        ]
    )
  , ( "invalid_committee_resignation_hash"
    , Map [(credential, Array [UInt 1, Bytes (bytesFrom 0 28)])]
    )
  , ("invalid_vrf_zero", UInt 0)
  , ( "invalid_gov_committee_threshold"
    , Array [Map [], Tag 30 (Array [UInt 3, UInt 2])]
    )
  , ("invalid_constitution_script_hash", Array [anchor "" 0, Bytes (bytesFrom 0 27)])
  , ( "invalid_pparams_missing_key"
    , Map [numeric 0 (UInt 0)]
    )
  , ( "invalid_pparams_rho_above_one"
    , replaceKey 10 (Tag 30 (Array [UInt 2, UInt 1])) populatedParameters
    )
  , ( "invalid_pparams_max_tx_size_overflow"
    , replaceKey 3 (UInt 4294967296) populatedParameters
    )
  , ( "invalid_pparams_cost_model_overflow"
    , replaceKey 18 (Map [(UInt 0, Array [UInt 9223372036854775808])]) populatedParameters
    )
  , ("invalid_roots_index_overflow", Array [Bytes (bytesFrom 0x90 32), UInt 65536])
  , ( "invalid_nonces_neutral_with_hash"
    , Map
        [ field "lab_nonce" (Array [UInt 0, Bytes (bytesFrom 0 32)])
        , field "last_slot" (Array [UInt 0])
        , field "epoch_nonce" neutralNonce
        , field "cert_counters" (Map [])
        , field "evolving_nonce" neutralNonce
        , field "candidate_nonce" neutralNonce
        , field "last_epoch_block_nonce" neutralNonce
        ]
    )
  , ("invalid_snapshot_kind", Array [UInt 2, UInt 0])
  , ("invalid_snapshot_pool_as_coin", Array [UInt 1, UInt 5])
  , ( "invalid_utxo_zero_quantity"
    , Bytes (encode (Array [Bytes (bytesFrom 0xe0 29), Array [UInt 1, Map [(Bytes (bytesFrom 0 28), Map [(Bytes BS.empty, UInt 0)])]]]))
    )
  , ( "invalid_utxo_datum_kind"
    , Bytes (encode (Map [(UInt 0, Bytes (bytesFrom 0xe0 29)), (UInt 1, UInt 1), (UInt 2, Array [UInt 2, Bytes (bytesFrom 0 32)])]))
    )
  , ( "invalid_proposal_vote"
    , Array
        [ UInt 0
        , Map
            [ field "drep_votes" (Map [(credential, UInt 3)])
            , field "proposed_in" (UInt 0)
            , field "expires_after" (UInt 0)
            , field "committee_votes" (Map [])
            , field "stake_pool_votes" (Map [])
            , field "proposal_procedure" (Bytes (encode (procedure infoAction)))
            ]
        ]
    )
  , ("invalid_proposal_action_kind", governanceProposalWith 0 (procedure (Array [UInt 7])))
  , ( "invalid_proposal_update_protocol_version"
    , governanceProposalWith 0
        (procedure (Array [UInt 0, Null, Map [numeric 14 (Array [UInt 1, UInt 0])], Null]))
    )
  , ( "invalid_proposal_reward_account"
    , governanceProposalWith 0
        (Array [UInt 0, Bytes (bytesFrom 0xe0 28), infoAction, anchor "" 0])
    )
  ]

replaceKey :: Word64 -> Term -> Term -> Term
replaceKey key value = \case
  Map entries -> Map [(k, if k `isKey` key then value else v) | (k, v) <- entries]
  other -> other
  where
    isKey (UInt n) m = n == m
    isKey _ _ = False
