package versebuilder

import (
	"encoding/hex"
	"math/big"
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
	l1BuildEnvironment = contract{
		address: common.HexToAddress("0x4500000000000000000000000000000000000001"),
		code:    l1BuildEnvironmentCode,
		fixedStorage: map[string]interface{}{
			// address private _owner
			"0x00": common.HexToAddress("0x187caf6b1732ebf4f2f7a1190dde1b0e986490cf"),
			// uint256 public environment.maxTransactionGasLimit
			"0x01": big.NewInt(15000000),
			// uint256 public environment.l2GasDiscountDivisor
			"0x02": big.NewInt(32),
			// uint256 public environment.enqueueGasCost
			"0x03": big.NewInt(60000),
			// uint256 public environment.fraudProofWindow
			"0x04": common.Big0,
			// uint256 public environment.sequencerPublishWindow
			"0x05": common.Big0,
		},
		dynamicStorage: map[string]string{
			// bytes public l1StandardBridgeCode
			"0x06": l1StandardBridgeCode,
			// bytes public l1ERC721BridgeCode
			"0x07": l1ERC721BridgeCode,
		},
	}
	l1BuildProgress = contract{
		address: common.HexToAddress("0x4500000000000000000000000000000000000002"),
		code:    l1BuildProgressCode,
	}
	l1BuildStep1 = contract{
		address: common.HexToAddress("0x4500000000000000000000000000000000000003"),
		code:    l1BuildStep1Code,
		fixedStorage: map[string]interface{}{
			// L1BuildProgress public progress
			"0x00": l1BuildProgress.address,
			// address public buildStep4
			"0x01": l1BuildStep4.address,
		},
	}
	l1BuildStep2 = contract{
		address: common.HexToAddress("0x4500000000000000000000000000000000000004"),
		code:    l1BuildStep2Code,
		fixedStorage: map[string]interface{}{
			// L1BuildEnvironment public environment
			"0x00": l1BuildEnvironment.address,
			// L1BuildProgress public progress
			"0x01": l1BuildProgress.address,
		},
	}
	l1BuildStep3 = contract{
		address: common.HexToAddress("0x4500000000000000000000000000000000000005"),
		code:    l1BuildStep3Code,
		fixedStorage: map[string]interface{}{
			// L1BuildProgress public progress
			"0x00": l1BuildProgress.address,
			// address public buildStep4
			"0x01": l1BuildStep4.address,
		},
	}
	l1BuildStep4 = contract{
		address: common.HexToAddress("0x4500000000000000000000000000000000000006"),
		code:    l1BuildStep4Code,
		fixedStorage: map[string]interface{}{
			// L1BuildEnvironment public environment
			"0x00": l1BuildEnvironment.address,
			// L1BuildProgress public progress
			"0x01": l1BuildProgress.address,
		},
	}

	contracts = []contract{
		l1BuildEnvironment,
		l1BuildProgress,
		l1BuildStep1,
		l1BuildStep2,
		l1BuildStep3,
		l1BuildStep4,
	}
)

type contract struct {
	address        common.Address
	code           string
	fixedStorage   map[string]interface{}
	dynamicStorage map[string]string
}

func (c *contract) storage() map[common.Hash]common.Hash {
	storage := make(map[common.Hash]common.Hash)

	if c.fixedStorage != nil {
		for key, val := range c.fixedStorage {
			slot := common.HexToHash(key)
			switch t := val.(type) {
			case common.Hash:
				storage[slot] = t
			case common.Address:
				storage[slot] = t.Hash()
			case *big.Int:
				storage[slot] = common.BigToHash(t)
			default:
				panic("unknown type")
			}
		}
	}

	if c.dynamicStorage != nil {
		for key, val := range c.dynamicStorage {
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

	return storage
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
		chunks = append(chunks, slice+strings.Repeat("0", l-len(slice)))
	}
	return chunks
}

// Deploy verse builder contracts.
func Deploy(state *state.StateDB) error {
	for _, contract := range contracts {
		code, err := hex.DecodeString(strings.TrimPrefix(contract.code, hexPrefix))
		if err != nil {
			return err
		}
		state.SetCode(contract.address, code)

		for key, val := range contract.storage() {
			state.SetState(contract.address, key, val)
		}
	}
	return nil
}
