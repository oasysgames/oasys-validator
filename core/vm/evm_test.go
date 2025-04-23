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

func TestEVMAccessControlForContractCreation(t *testing.T) {
	evm, caller := evmAccessControlTestEnv()
	codeAndHash := &codeAndHash{code: []byte{}}
	gas := uint64(1_000_000)
	value := uint256.NewInt(0)
	contractAddr := crypto.CreateAddress(caller.Address(), evm.StateDB.GetNonce(caller.Address()))

	_, _, _, err := evm.create(caller, codeAndHash, gas, value, contractAddr, CREATE)
	if err != ErrUnauthorizedCreate {
		t.Errorf("expected ErrUnauthorizedCreate, got %v", err)
	}
	nonce := evm.StateDB.GetNonce(caller.Address())
	if nonce != 1 {
		t.Errorf("expected caller nonce to be 1, got %d", nonce)
	}

	// Test with a contract creation from a contract
	evm.depth = 1
	_, _, _, err = evm.create(caller, codeAndHash, gas, value, contractAddr, CREATE)
	if err == ErrUnauthorizedCreate {
		t.Errorf("expected no error")
	}

	// Test with a contract creation from a contract(using CREATE2)
	evm.depth = 0
	_, _, _, err = evm.create(caller, codeAndHash, gas, value, contractAddr, CREATE2)
	if err == ErrUnauthorizedCreate {
		t.Errorf("expected no error")
	}
}

func TestEVMAccessControlForContractCall(t *testing.T) {
	evm, caller := evmAccessControlTestEnv()
	to := common.HexToAddress("0xdeaddeaddeaddeaddeaddeaddeaddeaddeaddead")
	input := []byte{}
	gas := uint64(1_000_000)
	value := uint256.NewInt(0)

	evm.StateDB.SetState(
		common.HexToAddress("0x520000000000000000000000000000000000003F"),
		crypto.Keccak256Hash(
			common.HexToHash(to.Hex()).Bytes(), // mapping key
			common.HexToHash("0x2").Bytes(),    // mapping slot
		),
		common.HexToHash("0x1"), // mapping value
	)

	_, _, err := evm.Call(caller, to, input, gas, value)
	if err != ErrUnauthorizedCall {
		t.Errorf("expected ErrUnauthorizedCall, got %v", err)
	}

	// Call by eth_call
	evm.Config.NoBaseFee = true
	_, _, err = evm.Call(caller, to, input, gas, value)
	if err == ErrUnauthorizedCall {
		t.Errorf("expected no error")
	}
}

func evmAccessControlTestEnv() (evm *EVM, caller AccountRef) {
	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	oasys.Deploy(params.OasysTestChainConfig, statedb, big.NewInt(2), 0, 1)

	evm = NewEVM(
		BlockContext{
			CanTransfer: func(StateDB, common.Address, *uint256.Int) bool { return true },
			Transfer:    func(StateDB, common.Address, common.Address, *uint256.Int) {},
		},
		statedb,
		params.OasysTestChainConfig,
		Config{},
	)
	caller = AccountRef(common.HexToAddress("0xbeef"))
	return evm, caller
}
