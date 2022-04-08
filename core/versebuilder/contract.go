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
	l1BuildParamAddress   = common.HexToAddress("0x0001000000000000000000000000000000001000")
	l1BuildDepositAddress = common.HexToAddress("0x0001000000000000000000000000000000002000")
	l1BuildAgentAddress   = common.HexToAddress("0x0001000000000000000000000000000000003000")
	l1BuildStep1Address   = common.HexToAddress("0x0001000000000000000000000000000000004000")
	l1BuildStep2Address   = common.HexToAddress("0x0001000000000000000000000000000000005000")
	l1BuildStep3Address   = common.HexToAddress("0x0001000000000000000000000000000000006000")
	l1BuildStep4Address   = common.HexToAddress("0x0001000000000000000000000000000000007000")

	l1BuildParam = &contract{
		address: l1BuildParamAddress,
		code:    l1BuildParamCode,
		fixedStorage: map[string]interface{}{
			// uint256 public maxTransactionGasLimit
			"0x00": big.NewInt(15_000_000),
			// uint256 public l2GasDiscountDivisor
			"0x01": big.NewInt(32),
			// uint256 public enqueueGasCost
			"0x02": big.NewInt(60_000),
			// uint256 public fraudProofWindow
			"0x03": common.Big0,
			// uint256 public sequencerPublishWindow
			"0x04": common.Big0,
		},
		dynamicStorage: map[string]string{
			// bytes public l1StandardBridgeCode
			"0x05": l1StandardBridgeCode,
			// bytes public l1ERC721BridgeCode
			"0x06": l1ERC721BridgeCode,
		},
	}
	l1BuildDeposit = &contract{
		address: l1BuildDepositAddress,
		code:    l1BuildDepositCode,
		fixedStorage: map[string]interface{}{
			// uint256 public requiredAmount
			"0x00": common.Big0,
			// uint256 public lockedBlock
			"0x01": common.Big0,
			// address public agentAddress
			"0x02": l1BuildAgentAddress,
		},
	}
	l1BuildAgent = &contract{
		address: l1BuildAgentAddress,
		code:    l1BuildAgentCode,
		fixedStorage: map[string]interface{}{
			// address public depositAddress
			"0x00": l1BuildDepositAddress,
			// address public step1Address
			"0x01": l1BuildStep1Address,
			// address public step2Address
			"0x02": l1BuildStep2Address,
			// address public step3Address
			"0x03": l1BuildStep3Address,
			// address public step4Address
			"0x04": l1BuildStep4Address,
		},
	}
	l1BuildStep1 = &contract{
		address: l1BuildStep1Address,
		code:    l1BuildStep1Code,
		fixedStorage: map[string]interface{}{
			// address public agentAddress
			"0x00": l1BuildAgentAddress,
		},
	}
	l1BuildStep2 = &contract{
		address: l1BuildStep2Address,
		code:    l1BuildStep2Code,
		fixedStorage: map[string]interface{}{
			// address public agentAddress
			"0x00": l1BuildAgentAddress,
			// address public paramAddress
			"0x01": l1BuildParamAddress,
		},
	}
	l1BuildStep3 = &contract{
		address: l1BuildStep3Address,
		code:    l1BuildStep3Code,
		fixedStorage: map[string]interface{}{
			// address public agentAddress
			"0x00": l1BuildAgentAddress,
		},
	}
	l1BuildStep4 = &contract{
		address: l1BuildStep4Address,
		code:    l1BuildStep4Code,
		fixedStorage: map[string]interface{}{
			// address public agentAddress
			"0x00": l1BuildAgentAddress,
			// address public paramAddress
			"0x01": l1BuildParamAddress,
		},
	}

	contracts = []*contract{
		l1BuildParam,
		l1BuildDeposit,
		l1BuildAgent,
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
