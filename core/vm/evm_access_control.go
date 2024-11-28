package vm

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/oasys"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	// ErrUnauthorizedCreate is returned if an unauthorized address creates a contract
	ErrUnauthorizedCreate = errors.New("unauthorized create")

	// ErrUnauthorizedCall is returned if an unauthorized address calls the contract
	ErrUnauthorizedCall = errors.New("unauthorized call")

	evmAccessControl = common.HexToAddress(oasys.EVMAccessControl)
	emptyHash        common.Hash
)

// Check the `_createAllowedList` mappings in the `EVMAccessControl` contract
func IsAllowedToCreate(state StateDB, from common.Address) bool {
	hash := computeAddressMapStorageKey(from, 1)
	val := state.GetState(evmAccessControl, hash)
	return val.Cmp(emptyHash) != 0
}

// Check the `_callAllowedList` mappings in the `EVMAccessControl` contract
func IsDeniedToCall(state StateDB, to common.Address) bool {
	hash := computeAddressMapStorageKey(to, 2)
	val := state.GetState(evmAccessControl, hash)
	return val.Cmp(emptyHash) != 0
}

func computeAddressMapStorageKey(address common.Address, slot uint64) common.Hash {
	paddedAddress := common.LeftPadBytes(address.Bytes(), 32)
	paddedSlot := common.LeftPadBytes(big.NewInt(int64(slot)).Bytes(), 32)
	return crypto.Keccak256Hash(paddedAddress, paddedSlot)
}
