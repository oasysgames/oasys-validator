package vm

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/oasys"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

var (
	// ErrUnauthorizedCreate is returned if an unauthorized address creates a contract
	ErrUnauthorizedCreate = errors.New("unauthorized create")

	// ErrUnauthorizedCall is returned if an unauthorized address calls the contract
	ErrUnauthorizedCall = errors.New("unauthorized call")

	evmAccessControl = common.HexToAddress(oasys.EVMAccessControl)
	emptyHash        common.Hash
)

// Check the `_createAllowedList`  mappings in the `EVMAccessControl` contract
func (evm *EVM) isAllowedToCreate(address common.Address) bool {
	if evm.chainConfig.ChainID.Cmp(params.OasysTestnetChainConfig.ChainID) == 0 {
		return true // Allow all contract creation on testnet
	}
	hash := computeAddressMapStorageKey(address, 1)
	val := evm.StateDB.GetState(evmAccessControl, hash)
	return val.Cmp(emptyHash) != 0
}

// Check the `_callAllowedList` mappings in the `EVMAccessControl` contract
func (evm *EVM) isDeniedToCall(address common.Address) bool {
	hash := computeAddressMapStorageKey(address, 2)
	val := evm.StateDB.GetState(evmAccessControl, hash)
	return val.Cmp(emptyHash) != 0
}

func computeAddressMapStorageKey(address common.Address, slot uint64) common.Hash {
	paddedAddress := common.LeftPadBytes(address.Bytes(), 32)
	paddedSlot := common.LeftPadBytes(big.NewInt(int64(slot)).Bytes(), 32)
	return crypto.Keccak256Hash(paddedAddress, paddedSlot)
}
