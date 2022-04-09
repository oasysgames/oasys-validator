package contracts

import (
	"encoding/hex"
	"errors"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	hexPrefix         = "0x"
	storageSlotLength = 64
)

var (
	// ErrContractAlreadyExists is returned if the contract address already exists.
	ErrContractAlreadyExists = errors.New("contract already exists")
)

var (
	// AddressPrefix is a reserved address prefix list for built-in contracts.
	AddressPrefix = map[string]common.Address{
		"genesis":      common.HexToAddress("0x0000000000000000000000000000000000000000"),
		"versebuilder": common.HexToAddress("0x0001000000000000000000000000000000000000"),
		"erc20":        common.HexToAddress("0x0002000000000000000000000000000000000000"),
	}
)

// Contract is a smart contract that writes directly to state.
type Contract struct {
	Address        common.Address
	Code           string
	FixedStorage   map[string]interface{}
	DynamicStorage map[string]string
}

func (c *Contract) Deploy(state *state.StateDB) error {
	if len(state.GetCode(c.Address)) != 0 {
		return ErrContractAlreadyExists
	}

	bytecode, err := c.ByteCodes()
	if err != nil {
		return err
	}
	storage, err := c.Storage()
	if err != nil {
		return err
	}

	state.SetCode(c.Address, bytecode)
	for key, val := range storage {
		state.SetState(c.Address, key, val)
	}
	return nil
}

// ByteCodes returns the contract byte codes.
func (c *Contract) ByteCodes() ([]byte, error) {
	bytecode, err := hex.DecodeString(strings.TrimPrefix(c.Code, hexPrefix))
	if err != nil {
		return nil, err
	}
	return bytecode, nil
}

// Storage returns the contract storage slot map.
func (c *Contract) Storage() (map[common.Hash]common.Hash, error) {
	storage := make(map[common.Hash]common.Hash)

	if c.FixedStorage != nil {
		for key, val := range c.FixedStorage {
			slot := common.HexToHash(key)
			switch t := val.(type) {
			case common.Hash:
				storage[slot] = t
			case common.Address:
				storage[slot] = t.Hash()
			case *big.Int:
				storage[slot] = common.BigToHash(t)
			case string:
				if !strings.HasPrefix(t, hexPrefix) {
					if len(t) > 31 {
						return nil, errors.New("strings longer than 32 bytes must be set to DynamicStorages")
					}
					t = toHex(t)
				}
				storage[slot] = common.HexToHash(t)
			default:
				return nil, errors.New("unsupported type")
			}
		}
	}

	if c.DynamicStorage != nil {
		for key, val := range c.DynamicStorage {
			val = strings.TrimPrefix(val, hexPrefix)

			rootSlot := common.HexToHash(key)
			storage[rootSlot] = common.BigToHash(big.NewInt(int64(len(val) + 1)))

			chunkStartPos := crypto.Keccak256Hash(rootSlot.Bytes()).Big()
			for i, chunk := range toChunks(val, storageSlotLength) {
				chunkSlot := common.BigToHash(new(big.Int).Add(chunkStartPos, big.NewInt(int64(i))))
				storage[chunkSlot] = common.HexToHash(chunk)
			}
		}
	}

	return storage, nil
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

func toHex(s string) string {
	hexs := hex.EncodeToString([]byte(s))
	hexlen := strconv.FormatInt(int64(len(s)*2), 16)
	return rightZeroPad(hexs, 62) + leftZeroPad(hexlen, 2)
}

func rightZeroPad(s string, l int) string {
	return s + strings.Repeat("0", l-len(s))
}

func leftZeroPad(s string, l int) string {
	return strings.Repeat("0", l-len(s)) + s
}
