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

import (
	"bytes"
	"errors"
	"fmt"
	"math"

	"github.com/blinklabs-io/go-scls/cardano/cbor"
)

type exUnitPrices struct {
	memory *Rational
	step   *Rational
}

// pparamsFields lists the gov/pparams/v0 map keys in key order, each paired
// with a pointer to the field it populates. The key set is exact: keys 12,
// 13 and 15 are absent from the schema.
func pparamsFields(p *ProtocolParameters) []struct {
	key   uint64
	field any
} {
	return []struct {
		key   uint64
		field any
	}{
		{0, &p.MinFeeA},
		{1, &p.MinFeeB},
		{2, &p.MaxBlockSize},
		{3, &p.MaxTxSize},
		{4, &p.MaxBlockHeaderSize},
		{5, &p.KeyDeposit},
		{6, &p.PoolDeposit},
		{7, &p.MaxEpoch},
		{8, &p.NOpt},
		{9, &p.A0},
		{10, &p.Rho},
		{11, &p.Tau},
		{14, &p.ProtocolVersion},
		{16, &p.MinPoolCost},
		{17, &p.AdaPerUTxOByte},
		{18, &p.CostModels},
		{19, exUnitPrices{&p.MemoryPrice, &p.StepPrice}},
		{20, &p.MaxTxExUnits},
		{21, &p.MaxBlockExUnits},
		{22, &p.MaxValueSize},
		{23, &p.CollateralPercentage},
		{24, &p.MaxCollateralInputs},
		{25, &p.PoolVotingThresholds},
		{26, &p.DRepVotingThresholds},
		{27, &p.MinCommitteeSize},
		{28, &p.CommitteeTermLimit},
		{29, &p.GovActionValidity},
		{30, &p.GovActionDeposit},
		{31, &p.DRepDeposit},
		{32, &p.DRepInactivity},
		{33, &p.MinFeeRefScriptCostPerByte},
	}
}

func decodeProtocolParameters(value any) (ProtocolParameters, error) {
	entries, ok := value.(cbor.Map)
	if !ok {
		return ProtocolParameters{}, errors.New("protocol parameters must be a map")
	}
	var result ProtocolParameters
	table := pparamsFields(&result)
	if len(entries) != len(table) {
		return ProtocolParameters{}, fmt.Errorf(
			"protocol parameters map has %d keys, want %d", len(entries), len(table),
		)
	}
	byKey := make(map[uint64]any, len(entries))
	for _, entry := range entries {
		key, err := decodeUint(entry.Key)
		if err != nil {
			return ProtocolParameters{}, errors.New("protocol parameter key must be an unsigned integer")
		}
		byKey[key] = entry.Value
	}
	for _, item := range table {
		raw, ok := byKey[item.key]
		if !ok {
			return ProtocolParameters{}, fmt.Errorf("missing protocol parameter %d", item.key)
		}
		if err := decodeParam(item.field, raw); err != nil {
			return ProtocolParameters{}, fmt.Errorf("parameter %d: %w", item.key, err)
		}
	}
	return result, nil
}

func decodeParam(field any, raw any) error {
	switch target := field.(type) {
	case *uint64:
		return setUint(target, raw, math.MaxUint64)
	case *uint32:
		var wide uint64
		if err := setUint(&wide, raw, math.MaxUint32); err != nil {
			return err
		}
		*target = uint32(wide)
	case *uint16:
		var wide uint64
		if err := setUint(&wide, raw, math.MaxUint16); err != nil {
			return err
		}
		*target = uint16(wide)
	case *UnitInterval:
		value, err := decodeUnitInterval(raw)
		if err != nil {
			return err
		}
		*target = value
	case *Rational:
		value, err := decodeRational(raw)
		if err != nil {
			return err
		}
		*target = value
	case *ProtocolVersion:
		items, err := decodePair(raw)
		if err != nil {
			return err
		}
		major, err := decodeUint(items[0])
		if err != nil {
			return fmt.Errorf("major: %w", err)
		}
		minor, err := decodeUint(items[1])
		if err != nil {
			return fmt.Errorf("minor: %w", err)
		}
		*target = ProtocolVersion{Major: major, Minor: minor}
	case *map[uint8][]int64:
		value, err := decodeCostModels(raw)
		if err != nil {
			return err
		}
		*target = value
	case exUnitPrices:
		items, err := decodePair(raw)
		if err != nil {
			return err
		}
		if *target.memory, err = decodeRational(items[0]); err != nil {
			return fmt.Errorf("memory price: %w", err)
		}
		if *target.step, err = decodeRational(items[1]); err != nil {
			return fmt.Errorf("step price: %w", err)
		}
	case *ExUnitPrices:
		items, err := decodePair(raw)
		if err != nil {
			return err
		}
		if target.Memory, err = decodeRational(items[0]); err != nil {
			return fmt.Errorf("memory price: %w", err)
		}
		if target.Step, err = decodeRational(items[1]); err != nil {
			return fmt.Errorf("step price: %w", err)
		}
	case *ExUnits:
		items, err := decodePair(raw)
		if err != nil {
			return err
		}
		memory, err := decodeUint(items[0])
		if err != nil {
			return fmt.Errorf("memory: %w", err)
		}
		steps, err := decodeUint(items[1])
		if err != nil {
			return fmt.Errorf("steps: %w", err)
		}
		*target = ExUnits{Memory: memory, Steps: steps}
	case *[5]UnitInterval:
		return decodeThresholds(target[:], raw)
	case *[10]UnitInterval:
		return decodeThresholds(target[:], raw)
	default:
		return fmt.Errorf("unsupported parameter type %T", field)
	}
	return nil
}

func setUint(target *uint64, raw any, maximum uint64) error {
	value, err := decodeUintMax(raw, maximum)
	if err != nil {
		return err
	}
	*target = value
	return nil
}

func decodeThresholds(target []UnitInterval, raw any) error {
	items, ok := raw.(cbor.Array)
	if !ok || len(items) != len(target) {
		return fmt.Errorf("must be an array of %d unit intervals", len(target))
	}
	for i, item := range items {
		value, err := decodeUnitInterval(item)
		if err != nil {
			return fmt.Errorf("threshold %d: %w", i, err)
		}
		target[i] = value
	}
	return nil
}

func decodeCostModels(raw any) (map[uint8][]int64, error) {
	entries, ok := raw.(cbor.Map)
	if !ok {
		return nil, errors.New("cost models must be a map")
	}
	result := make(map[uint8][]int64, min(len(entries), 256))
	for _, entry := range entries {
		language, err := decodeUintMax(entry.Key, math.MaxUint8)
		if err != nil {
			return nil, fmt.Errorf("cost model language: %w", err)
		}
		items, ok := entry.Value.(cbor.Array)
		if !ok {
			return nil, fmt.Errorf("cost model %d must be an array", language)
		}
		model := make([]int64, 0, min(len(items), 1024))
		for i, item := range items {
			number, err := decodeInt64(item)
			if err != nil {
				return nil, fmt.Errorf("cost model %d[%d]: %w", language, i, err)
			}
			model = append(model, number)
		}
		result[uint8(language)] = model //nolint:gosec // bounded by decodeUintMax
	}
	return result, nil
}

func decodeInt64(value any) (int64, error) {
	switch value := value.(type) {
	case uint64:
		if value > math.MaxInt64 {
			return 0, errors.New("exceeds int64")
		}
		return int64(value), nil
	case cbor.Negative:
		if uint64(value) > math.MaxInt64 {
			return 0, errors.New("below int64")
		}
		return -1 - int64(value), nil //nolint:gosec // bounded above
	default:
		return 0, errors.New("must be an integer")
	}
}

func encodeProtocolParameters(value ProtocolParameters) (any, error) {
	table := pparamsFields(&value)
	result := make(cbor.Map, 0, len(table))
	for _, item := range table {
		encoded, err := encodeParam(item.field)
		if err != nil {
			return nil, fmt.Errorf("parameter %d: %w", item.key, err)
		}
		result = append(result, cbor.MapEntry{Key: item.key, Value: encoded})
	}
	return result, nil
}

func encodeParam(field any) (any, error) {
	switch source := field.(type) {
	case *uint64:
		return *source, nil
	case *uint32:
		return uint64(*source), nil
	case *uint16:
		return uint64(*source), nil
	case *UnitInterval:
		return encodeUnitInterval(*source)
	case *Rational:
		return encodeRational(*source)
	case *ProtocolVersion:
		return cbor.Array{source.Major, source.Minor}, nil
	case *map[uint8][]int64:
		result := make(cbor.Map, 0, len(*source))
		for language, model := range *source {
			numbers := make(cbor.Array, 0, len(model))
			for _, number := range model {
				numbers = append(numbers, number)
			}
			result = append(result, cbor.MapEntry{Key: uint64(language), Value: numbers})
		}
		return result, nil
	case exUnitPrices:
		memory, err := encodeRational(*source.memory)
		if err != nil {
			return nil, fmt.Errorf("memory price: %w", err)
		}
		step, err := encodeRational(*source.step)
		if err != nil {
			return nil, fmt.Errorf("step price: %w", err)
		}
		return cbor.Array{memory, step}, nil
	case *ExUnitPrices:
		memory, err := encodeRational(source.Memory)
		if err != nil {
			return nil, fmt.Errorf("memory price: %w", err)
		}
		step, err := encodeRational(source.Step)
		if err != nil {
			return nil, fmt.Errorf("step price: %w", err)
		}
		return cbor.Array{memory, step}, nil
	case *ExUnits:
		return cbor.Array{source.Memory, source.Steps}, nil
	case *[5]UnitInterval:
		return encodeThresholds(source[:])
	case *[10]UnitInterval:
		return encodeThresholds(source[:])
	default:
		return nil, fmt.Errorf("unsupported parameter type %T", field)
	}
}

func encodeThresholds(values []UnitInterval) (any, error) {
	result := make(cbor.Array, 0, len(values))
	for i, value := range values {
		encoded, err := encodeUnitInterval(value)
		if err != nil {
			return nil, fmt.Errorf("threshold %d: %w", i, err)
		}
		result = append(result, encoded)
	}
	return result, nil
}

// utxo/v0: the payload is a CBOR byte string holding a tx_out.

func decodeTxOut(value any) (TxOut, error) {
	inner, ok := value.([]byte)
	if !ok {
		return TxOut{}, errors.New("payload must be a byte string holding a tx_out")
	}
	decoded, err := cbor.UnmarshalLedger(inner)
	if err != nil {
		return TxOut{}, fmt.Errorf("embedded tx_out: %w", err)
	}
	var result TxOut
	switch decoded := decoded.(type) {
	case cbor.Array:
		result, err = decodeShelleyTxOut(decoded)
	case cbor.Map:
		result, err = decodeBabbageTxOut(decoded)
	default:
		err = errors.New("tx_out must be an array or a map")
	}
	if err != nil {
		return TxOut{}, err
	}
	canonical, err := encodeTxOutFields(result)
	if err != nil {
		return TxOut{}, err
	}
	if !bytes.Equal(canonical, inner) {
		result.Raw = bytes.Clone(inner)
	}
	return result, nil
}

func decodeShelleyTxOut(items cbor.Array) (TxOut, error) {
	if len(items) < 2 || len(items) > 3 {
		return TxOut{}, fmt.Errorf("shelley tx_out has %d items, want 2 or 3", len(items))
	}
	result := TxOut{Format: TxOutShelley}
	var err error
	if result.Address, err = decodeBytes(items[0], -1); err != nil {
		return TxOut{}, fmt.Errorf("address: %w", err)
	}
	if result.Value, err = decodeTxValue(items[1]); err != nil {
		return TxOut{}, fmt.Errorf("value: %w", err)
	}
	if len(items) == 3 {
		data, err := decodeBytes(items[2], 32)
		if err != nil {
			return TxOut{}, fmt.Errorf("datum hash: %w", err)
		}
		var hash Hash32
		copy(hash[:], data)
		result.Datum = &TxOutDatum{Hash: &hash}
	}
	return result, nil
}

func decodeBabbageTxOut(entries cbor.Map) (TxOut, error) {
	result := TxOut{Format: TxOutBabbage}
	fields := make(map[uint64]any, len(entries))
	for _, entry := range entries {
		key, err := decodeUint(entry.Key)
		if err != nil || key > 3 {
			return TxOut{}, errors.New("babbage tx_out keys must be 0..3")
		}
		fields[key] = entry.Value
	}
	address, ok := fields[0]
	if !ok {
		return TxOut{}, errors.New("missing address")
	}
	var err error
	if result.Address, err = decodeBytes(address, -1); err != nil {
		return TxOut{}, fmt.Errorf("address: %w", err)
	}
	value, ok := fields[1]
	if !ok {
		return TxOut{}, errors.New("missing value")
	}
	if result.Value, err = decodeTxValue(value); err != nil {
		return TxOut{}, fmt.Errorf("value: %w", err)
	}
	if datum, ok := fields[2]; ok {
		if result.Datum, err = decodeDatumOption(datum); err != nil {
			return TxOut{}, fmt.Errorf("datum: %w", err)
		}
	}
	if script, ok := fields[3]; ok {
		if result.Script, err = decodeScriptRef(script); err != nil {
			return TxOut{}, fmt.Errorf("script ref: %w", err)
		}
	}
	return result, nil
}

func decodeTxValue(value any) (TxValue, error) {
	if coin, ok := value.(uint64); ok {
		return TxValue{Coin: coin}, nil
	}
	items, err := decodePair(value)
	if err != nil {
		return TxValue{}, errors.New("must be a coin or [coin, multiasset]")
	}
	coin, err := decodeUint(items[0])
	if err != nil {
		return TxValue{}, fmt.Errorf("coin: %w", err)
	}
	policies, ok := items[1].(cbor.Map)
	if !ok {
		return TxValue{}, errors.New("multiasset must be a map")
	}
	assets := make(MultiAsset, 0, min(len(policies), 1024))
	for _, policyEntry := range policies {
		policy, err := decodeHash28(policyEntry.Key)
		if err != nil {
			return TxValue{}, fmt.Errorf("policy id: %w", err)
		}
		names, ok := policyEntry.Value.(cbor.Map)
		if !ok || len(names) == 0 {
			return TxValue{}, errors.New("policy assets must be a non-empty map")
		}
		group := PolicyAssets{Policy: policy, Assets: make([]Asset, 0, min(len(names), 1024))}
		for _, nameEntry := range names {
			name, err := decodeBytes(nameEntry.Key, -1)
			if err != nil || len(name) > maxAssetNameSize {
				return TxValue{}, fmt.Errorf("asset name must be at most %d bytes", maxAssetNameSize)
			}
			quantity, err := decodeUint(nameEntry.Value)
			if err != nil || quantity == 0 {
				return TxValue{}, errors.New("asset quantity must be a positive unsigned integer")
			}
			group.Assets = append(group.Assets, Asset{Name: name, Quantity: quantity})
		}
		assets = append(assets, group)
	}
	return TxValue{Coin: coin, Assets: &assets}, nil
}

func decodeDatumOption(value any) (*TxOutDatum, error) {
	items, err := decodePair(value)
	if err != nil {
		return nil, err
	}
	kind, err := decodeUint(items[0])
	if err != nil {
		return nil, errors.New("datum option kind must be an unsigned integer")
	}
	switch kind {
	case 0:
		data, err := decodeBytes(items[1], 32)
		if err != nil {
			return nil, fmt.Errorf("datum hash: %w", err)
		}
		var hash Hash32
		copy(hash[:], data)
		return &TxOutDatum{Hash: &hash}, nil
	case 1:
		inline, err := decodeEmbeddedCBOR(items[1])
		if err != nil {
			return nil, fmt.Errorf("inline datum: %w", err)
		}
		return &TxOutDatum{Inline: inline}, nil
	default:
		return nil, fmt.Errorf("datum option kind %d is invalid", kind)
	}
}

// decodeEmbeddedCBOR reads tag 24 around a byte string and returns that byte
// string unvalidated: Plutus data and scripts keep the exact bytes the ledger
// stored, which may use indefinite lengths.
func decodeEmbeddedCBOR(value any) ([]byte, error) {
	tag, ok := value.(cbor.Tag)
	if !ok || tag.Number != 24 {
		return nil, errors.New("must be tag 24")
	}
	data, err := decodeBytes(tag.Value, -1)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func decodeScriptRef(value any) (*TxOutScript, error) {
	data, err := decodeEmbeddedCBOR(value)
	if err != nil {
		return nil, err
	}
	script, err := cbor.UnmarshalLedger(data)
	if err != nil {
		return nil, err
	}
	items, err := decodePair(script)
	if err != nil {
		return nil, err
	}
	language, err := decodeUint(items[0])
	if err != nil || language > 3 {
		return nil, errors.New("script language must be 0..3")
	}
	if language == 0 {
		if err := validateNativeScript(items[1], 0); err != nil {
			return nil, fmt.Errorf("native script: %w", err)
		}
		// The ledger hashes a native script over its original bytes, so keep
		// them. The pair header and the language 0 are one byte each (minimal
		// arguments are enforced), and an indefinite pair ends with a break.
		end := len(data)
		if data[0] == 0x9f {
			end--
		}
		return &TxOutScript{Language: 0, Script: bytes.Clone(data[2:end])}, nil
	}
	body, err := decodeBytes(items[1], -1)
	if err != nil {
		return nil, fmt.Errorf("plutus script: %w", err)
	}
	return &TxOutScript{Language: uint8(language), Script: body}, nil
}

func validateNativeScript(value any, depth int) error {
	if depth > 64 {
		return errors.New("nesting too deep")
	}
	items, ok := value.(cbor.Array)
	if !ok || len(items) == 0 {
		return errors.New("must be a non-empty array")
	}
	kind, err := decodeUint(items[0])
	if err != nil {
		return errors.New("kind must be an unsigned integer")
	}
	switch kind {
	case 0:
		if len(items) != 2 {
			return errors.New("pubkey script must have 2 items")
		}
		_, err := decodeHash28(items[1])
		return err
	case 1, 2:
		if len(items) != 2 {
			return errors.New("all/any script must have 2 items")
		}
		return validateNativeScripts(items[1], depth)
	case 3:
		if len(items) != 3 {
			return errors.New("n-of-k script must have 3 items")
		}
		if _, err := decodeInt64(items[1]); err != nil {
			return fmt.Errorf("n: %w", err)
		}
		return validateNativeScripts(items[2], depth)
	case 4, 5:
		if len(items) != 2 {
			return errors.New("timelock script must have 2 items")
		}
		_, err := decodeUint(items[1])
		return err
	default:
		return fmt.Errorf("native script kind %d is invalid", kind)
	}
}

func validateNativeScripts(value any, depth int) error {
	items, ok := value.(cbor.Array)
	if !ok {
		return errors.New("must be an array of scripts")
	}
	for i, item := range items {
		if err := validateNativeScript(item, depth+1); err != nil {
			return fmt.Errorf("script %d: %w", i, err)
		}
	}
	return nil
}

func encodeTxOut(value TxOut) (any, error) {
	canonical, err := encodeTxOutFields(value)
	if err != nil {
		return nil, err
	}
	if value.Raw == nil {
		return canonical, nil
	}
	decoded, err := decodeTxOut(value.Raw)
	if err != nil {
		return nil, fmt.Errorf("raw tx_out: %w", err)
	}
	decoded.Raw = nil
	again, err := encodeTxOutFields(decoded)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(again, canonical) {
		return nil, errors.New("raw tx_out does not match the tx_out fields")
	}
	return bytes.Clone(value.Raw), nil
}

func encodeTxOutFields(value TxOut) ([]byte, error) {
	txValue, err := encodeTxValue(value.Value)
	if err != nil {
		return nil, fmt.Errorf("value: %w", err)
	}
	var inner any
	switch value.Format {
	case TxOutShelley:
		if value.Script != nil {
			return nil, errors.New("shelley tx_out cannot carry a script reference")
		}
		items := cbor.Array{bytes.Clone(value.Address), txValue}
		if value.Datum != nil {
			if value.Datum.Hash == nil || value.Datum.Inline != nil {
				return nil, errors.New("shelley tx_out datum must be a datum hash")
			}
			items = append(items, bytes.Clone(value.Datum.Hash[:]))
		}
		inner = items
	case TxOutBabbage:
		entries := cbor.Map{
			{Key: uint64(0), Value: bytes.Clone(value.Address)},
			{Key: uint64(1), Value: txValue},
		}
		if value.Datum != nil {
			datum, err := encodeDatumOption(*value.Datum)
			if err != nil {
				return nil, fmt.Errorf("datum: %w", err)
			}
			entries = append(entries, cbor.MapEntry{Key: uint64(2), Value: datum})
		}
		if value.Script != nil {
			script, err := encodeScriptRef(*value.Script)
			if err != nil {
				return nil, fmt.Errorf("script ref: %w", err)
			}
			entries = append(entries, cbor.MapEntry{Key: uint64(3), Value: script})
		}
		inner = entries
	default:
		return nil, fmt.Errorf("tx_out format %d is invalid", value.Format)
	}
	return cbor.Marshal(inner)
}

func encodeTxValue(value TxValue) (any, error) {
	if value.Assets == nil {
		return value.Coin, nil
	}
	policies := make(cbor.Map, 0, len(*value.Assets))
	for _, group := range *value.Assets {
		if len(group.Assets) == 0 {
			return nil, errors.New("policy must hold at least one asset")
		}
		names := make(cbor.Map, 0, len(group.Assets))
		for _, asset := range group.Assets {
			if len(asset.Name) > maxAssetNameSize {
				return nil, fmt.Errorf("asset name has %d bytes, maximum is %d", len(asset.Name), maxAssetNameSize)
			}
			if asset.Quantity == 0 {
				return nil, errors.New("asset quantity must be positive")
			}
			names = append(names, cbor.MapEntry{Key: bytes.Clone(asset.Name), Value: asset.Quantity})
		}
		policies = append(policies, cbor.MapEntry{Key: bytes.Clone(group.Policy[:]), Value: names})
	}
	return cbor.Array{value.Coin, policies}, nil
}

func encodeDatumOption(value TxOutDatum) (any, error) {
	switch {
	case value.Hash != nil && value.Inline == nil:
		return cbor.Array{uint64(0), bytes.Clone(value.Hash[:])}, nil
	case value.Hash == nil && value.Inline != nil:
		if len(value.Inline) == 0 {
			return nil, errors.New("inline datum must not be empty")
		}
		return cbor.Array{uint64(1), cbor.Tag{Number: 24, Value: bytes.Clone(value.Inline)}}, nil
	default:
		return nil, errors.New("datum must be exactly one of hash or inline")
	}
}

func encodeScriptRef(value TxOutScript) (any, error) {
	var body any
	switch value.Language {
	case 0:
		native, err := cbor.UnmarshalLedger(value.Script)
		if err != nil {
			return nil, fmt.Errorf("native script: %w", err)
		}
		if err := validateNativeScript(native, 0); err != nil {
			return nil, fmt.Errorf("native script: %w", err)
		}
		encoded := append([]byte{0x82, 0x00}, value.Script...)
		return cbor.Tag{Number: 24, Value: encoded}, nil
	case 1, 2, 3:
		body = bytes.Clone(value.Script)
	default:
		return nil, fmt.Errorf("script language %d is invalid", value.Language)
	}
	encoded, err := cbor.Marshal(cbor.Array{uint64(value.Language), body})
	if err != nil {
		return nil, err
	}
	return cbor.Tag{Number: 24, Value: encoded}, nil
}
