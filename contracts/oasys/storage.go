package oasys

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	storageSlotSize = 64
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

// Returns the copied instance.
func (s storage) copy() storage {
	cpy := storage{}
	for key, val := range s {
		switch t := val.(type) {
		case *big.Int:
			cpy[key] = common.BigToHash(t)
		case mapping:
			cpy[key] = t.copy()
		default:
			cpy[key] = t
		}
	}
	return cpy
}

// `mapping` type storage.
type mapping struct {
	keyType reflect.Type
	values  map[string]interface{}
}

// Add mapping values to storage.
func (m *mapping) add(storage map[common.Hash]common.Hash, rootSlot common.Hash) error {
	if m.keyType != reflect.TypeOf(common.Address{}) {
		return fmt.Errorf("unsupported key type: %s", m.keyType)
	}

	for mkey, mval := range m.values {
		k := bytes.Join([][]byte{common.HexToHash(mkey).Bytes(), rootSlot[:]}, nil)
		slot := common.BytesToHash(crypto.Keccak256(k))
		if err := setStorage(storage, slot, mval); err != nil {
			return err
		}
	}

	return nil
}

// Returns the copied instance.
func (m *mapping) copy() *mapping {
	cpy := &mapping{
		keyType: m.keyType,
		values:  map[string]interface{}{},
	}
	for k, v := range m.values {
		cpy.values[k] = v
	}
	return cpy
}

func setStorage(storage map[common.Hash]common.Hash, slot common.Hash, val interface{}) error {
	switch t := val.(type) {
	case common.Hash:
		storage[slot] = t
	case common.Address:
		storage[slot] = t.Hash()
	case *big.Int:
		storage[slot] = common.BigToHash(t)
	case mapping:
		if err := t.add(storage, slot); err != nil {
			return err
		}
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
