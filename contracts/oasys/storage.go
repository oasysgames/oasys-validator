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
)

const (
	storageSlotSize = 64
	hexPrefix       = "0x"
)

type storage map[string]interface{}

// Returns the contract storage slot map.
// see: https://docs.soliditylang.org/en/v0.8.11/internals/layout_in_storage.html
func (s storage) build() (map[common.Hash]common.Hash, error) {
	storage := make(map[common.Hash]common.Hash)
	for slot, val := range s {
		if err := setStorage(storage, common.HexToHash(slot), val); err != nil {
			return nil, err
		}
	}
	return storage, nil
}

// Different storage values for each genesis.
type genesismap map[common.Hash]interface{}

// Add values to storage.
func (g genesismap) add(storage map[common.Hash]common.Hash, rootSlot common.Hash) error {
	if val, ok := g[GenesisHash]; ok {
		return setStorage(storage, rootSlot, val)
	}
	if val, ok := g[defaultGenesisHash]; ok {
		return setStorage(storage, rootSlot, val)
	}
	return nil
}

// `array` type storage.
type array []interface{}

// Add array values to storage.
func (a *array) add(storage map[common.Hash]common.Hash, rootSlot common.Hash) error {
	storage[rootSlot] = common.BigToHash(big.NewInt(int64(len(*a))))

	slot := new(big.Int).SetBytes(crypto.Keccak256(rootSlot.Bytes()))
	for _, val := range *a {
		if err := setStorage(storage, common.BigToHash(slot), val); err != nil {
			return err
		}
		slot.Add(slot, common.Big1)
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
func (m *mapping) add(storage map[common.Hash]common.Hash, rootSlot common.Hash) error {
	for mkey, mval := range m.values {
		k := bytes.Join([][]byte{m.keyFn(mkey).Bytes(), rootSlot[:]}, nil)
		slot := common.BytesToHash(crypto.Keccak256(k))
		if err := setStorage(storage, slot, mval); err != nil {
			return err
		}
	}

	return nil
}

// `struct` type value.
type structValue []struct {
	pos   int64
	value interface{}
}

// Add members to storage.
func (s structValue) add(storage map[common.Hash]common.Hash, rootSlot common.Hash) error {
	for _, member := range s {
		memberSlot := new(big.Int).Add(rootSlot.Big(), big.NewInt(member.pos))
		if err := setStorage(storage, common.BigToHash(memberSlot), member.value); err != nil {
			return err
		}
	}

	return nil
}

func setStorage(storage map[common.Hash]common.Hash, slot common.Hash, val interface{}) error {
	switch t := val.(type) {
	case common.Hash:
		storage[slot] = t
	case common.Address:
		storage[slot] = t.Hash()
	case *big.Int:
		storage[slot] = common.BigToHash(t)
	case array:
		return t.add(storage, slot)
	case mapping:
		return t.add(storage, slot)
	case genesismap:
		return t.add(storage, slot)
	case structValue:
		return t.add(storage, slot)
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
