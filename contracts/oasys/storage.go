package oasys

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

const (
	storageSlotSize = 64
	hexPrefix       = "0x"
)

// Represents Solidity storage
type storage map[string]interface{}

// Returns the contract storage slot map.
// see: https://docs.soliditylang.org/en/v0.8.11/internals/layout_in_storage.html
func (s storage) build(cfg *params.ChainConfig) (map[common.Hash]common.Hash, error) {
	storage := make(map[common.Hash]common.Hash)
	for slot, val := range s {
		if err := setStorage(cfg, storage, common.HexToHash(slot), val); err != nil {
			return nil, err
		}
	}
	return storage, nil
}

// Non-primitive values in Solidity(such as array,mapping,struct)
type dynamicSlotValue interface {
	apply(cfg *params.ChainConfig, storage map[common.Hash]common.Hash, rootSlot common.Hash) error
}

// Different storage values for each genesis.
type genesismap map[common.Hash]interface{}

// Add values to storage.
func (g genesismap) apply(cfg *params.ChainConfig, storage map[common.Hash]common.Hash, rootSlot common.Hash) error {
	if val := g.value(); val != nil {
		return setStorage(cfg, storage, rootSlot, val)
	}
	return nil
}

// Return the mapped value.
func (g genesismap) value() interface{} {
	if val, ok := g[GenesisHash]; ok {
		return val
	}
	if val, ok := g[defaultGenesisHash]; ok {
		return val
	}
	return nil
}

// `array` type storage.
// If `length` is explicitly specified, it will be written to the root slot.
// Otherwise, the length of values will be written.
type array struct {
	length int64
	values map[int64]interface{}
}

// Add array values to storage.
func (a *array) apply(cfg *params.ChainConfig, storage map[common.Hash]common.Hash, rootSlot common.Hash) error {
	length := a.length
	if length == 0 {
		length = int64(len(a.values))
	}
	storage[rootSlot] = common.BigToHash(big.NewInt(length))

	startSlot := new(big.Int).SetBytes(crypto.Keccak256(rootSlot.Bytes()))
	for index, val := range a.values {
		offset := index

		// When the value is a struct, the starting slot will be
		// `index + number of slots used by struct`
		if size := structSize(cfg, val); size > 0 {
			offset *= size
		}

		slot := new(big.Int).Add(startSlot, big.NewInt(offset))
		if err := setStorage(cfg, storage, common.BigToHash(slot), val); err != nil {
			return err
		}
	}

	return nil
}

// `mapping` type storage.
type mapping struct {
	keyFn  func(key string) common.Hash
	values map[string]interface{}
}

var (
	addressKeyFn = func(key string) common.Hash { return common.HexToHash(key) }
)

// Add mapping values to storage.
func (m *mapping) apply(cfg *params.ChainConfig, storage map[common.Hash]common.Hash, rootSlot common.Hash) error {
	for mkey, mval := range m.values {
		k := bytes.Join([][]byte{m.keyFn(mkey).Bytes(), rootSlot[:]}, nil)
		slot := common.BytesToHash(crypto.Keccak256(k))
		if err := setStorage(cfg, storage, slot, mval); err != nil {
			return err
		}
	}

	return nil
}

// `struct` type value.
// Assumes that each contained value is exactly one slot in length (32 bytes).
type structvalue []interface{}

// Add members to storage.
func (s structvalue) apply(cfg *params.ChainConfig, storage map[common.Hash]common.Hash, rootSlot common.Hash) error {
	for pos, member := range s {
		memberSlot := new(big.Int).Add(rootSlot.Big(), big.NewInt(int64(pos)))
		if err := setStorage(cfg, storage, common.BigToHash(memberSlot), member); err != nil {
			return err
		}
	}

	return nil
}

func setStorage(cfg *params.ChainConfig, storage map[common.Hash]common.Hash, slot common.Hash, val interface{}) error {
	switch t := val.(type) {
	case common.Hash:
		storage[slot] = t
	case common.Address:
		storage[slot] = common.BytesToHash(t.Bytes())
	case *big.Int:
		storage[slot] = common.BigToHash(t)
	case string:
		isHex := strings.HasPrefix(t, hexPrefix)
		if isHex {
			t = strings.TrimPrefix(t, hexPrefix)
		} else {
			t = hex.EncodeToString([]byte(t))
		}

		if isHex && len(t) <= storageSlotSize {
			storage[slot] = common.HexToHash(t)
		} else if len(t) < storageSlotSize {
			ends := strconv.FormatInt(int64(len(t)), 16)
			storage[slot] = common.HexToHash(rightZeroPad(t, 62) + leftZeroPad(ends, 2))
		} else {
			storage[slot] = common.BigToHash(big.NewInt(int64(len(t) + 1)))
			chunkStartPos := crypto.Keccak256Hash(slot.Bytes()).Big()
			for i, chunk := range toChunks(t, storageSlotSize) {
				chunkSlot := common.BigToHash(new(big.Int).Add(chunkStartPos, big.NewInt(int64(i))))
				storage[chunkSlot] = common.HexToHash(chunk)
			}
		}
	case dynamicSlotValue:
		return t.apply(cfg, storage, slot)
	case func(cfg *params.ChainConfig) interface{}:
		return setStorage(cfg, storage, slot, t(cfg))
	default:
		return fmt.Errorf("unsupported type: %s, slot: %s", t, slot.String())
	}
	return nil
}

func toChunks(s string, l int) []string {
	slen := len(s)
	chunks := make([]string, 0)
	for i := 0; i < slen; i += l {
		end := i + l
		if end > slen {
			end = slen
		}
		slice := s[i:end]
		chunks = append(chunks, rightZeroPad(slice, l))
	}
	return chunks
}

func rightZeroPad(s string, l int) string {
	return s + strings.Repeat("0", l-len(s))
}

func leftZeroPad(s string, l int) string {
	return strings.Repeat("0", l-len(s)) + s
}

func structSize(cfg *params.ChainConfig, val interface{}) int64 {
	// Resolve the value until reaching either primitive or Solidity's data structure.
	var extract func(val interface{}) interface{}
	extract = func(val interface{}) interface{} {
		switch t := val.(type) {
		case genesismap:
			return extract(t.value())
		case func(cfg *params.ChainConfig) interface{}:
			return extract(t(cfg))
		}
		return val
	}

	if t, ok := extract(val).(structvalue); ok {
		return int64(len(t))
	}
	return 0
}
