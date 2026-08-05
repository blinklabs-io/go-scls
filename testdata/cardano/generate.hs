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
