package vm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/oasys"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

func TestEVMAccessControl(t *testing.T) {
	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	oasys.Deploy(params.OasysTestChainConfig, statedb, big.NewInt(2), 0, 1)

	evm := NewEVM(
		BlockContext{
			CanTransfer: func(StateDB, common.Address, *uint256.Int) bool { return true },
			Transfer:    func(StateDB, common.Address, common.Address, *uint256.Int) {},
		},
		statedb,
		params.OasysTestChainConfig,
		Config{},
	)
	allowedDeployer := AccountRef(common.HexToAddress("0x1"))
	deniedDeployer := AccountRef(common.HexToAddress("0x2"))
	contract := crypto.CreateAddress(common.HexToAddress("0x3"), 0)
	input := []byte{}
	gas := uint64(1_000_000)
	value := uint256.NewInt(0)

	// Add to `_createAllowedList`
	evm.StateDB.SetState(
		common.HexToAddress("0x520000000000000000000000000000000000003F"),
		crypto.Keccak256Hash(
			common.HexToHash(allowedDeployer.Address().Hex()).Bytes(), // mapping key
			common.HexToHash("0x1").Bytes(),                           // mapping slot
		),
		common.HexToHash("0x1"), // mapping value
	)

	// Add to `_callDeniedList`
	evm.StateDB.SetState(
		common.HexToAddress("0x520000000000000000000000000000000000003F"),
		crypto.Keccak256Hash(
			common.HexToHash(contract.Hex()).Bytes(),
			common.HexToHash("0x2").Bytes(),
		),
		common.HexToHash("0x1"),
	)

	t.Run("ContractCreation", func(t *testing.T) {
		// From allowed deployer
		_, _, _, err := evm.Create(allowedDeployer, input, gas, value)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		// From denied deployer
		_, _, _, err = evm.Create(deniedDeployer, input, gas, value)
		if err != ErrUnauthorizedCreate {
			t.Errorf("expected ErrUnauthorizedCreate, got %v", err)
		}
		nonce := evm.StateDB.GetNonce(deniedDeployer.Address())
		if nonce != 1 {
			t.Errorf("expected caller nonce to be 1, got %d", nonce)
		}

		// From contract
		evm.depth = 1 // simulate internal call
		_, _, _, err = evm.Create(deniedDeployer, input, gas, value)
		if err != nil {
			t.Errorf("expected no error")
		}
	})

	t.Run("ContractCall", func(t *testing.T) {
		_, _, err := evm.Call(deniedDeployer, contract, input, gas, value)
		if err != ErrUnauthorizedCall {
			t.Errorf("expected ErrUnauthorizedCall, got %v", err)
		}

		// via eth_call
		evm.Config.NoBaseFee = true
		_, _, err = evm.Call(deniedDeployer, contract, input, gas, value)
		if err == ErrUnauthorizedCall {
			t.Errorf("expected no error")
		}
	})
}
