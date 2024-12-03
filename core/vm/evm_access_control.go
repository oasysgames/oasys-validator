package vm

import (
	"bytes"
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
	buildInPrefix1   = common.FromHex(oasys.BuiltInContractPrefix1)
	buildInPrefix2   = common.FromHex(oasys.BuiltInContractPrefix2)
)

// Check the `_createAllowedList` mappings in the `EVMAccessControl` contract
func IsAllowedToCreate(state StateDB, from common.Address) bool {
	// Always allow in testnet, as anybody freely create contracts
	return true
}

// Check the `_callAllowedList` mappings in the `EVMAccessControl` contract
func IsDeniedToCall(state StateDB, to common.Address) bool {
	// Don't deny calls to built-in contracts
	if bytes.HasPrefix(to.Bytes(), buildInPrefix1) || bytes.HasPrefix(to.Bytes(), buildInPrefix2) {
		return false
	}
	hash := computeAddressMapStorageKey(to, 2)
	val := state.GetState(evmAccessControl, hash)
	return val.Cmp(emptyHash) != 0
}

func computeAddressMapStorageKey(address common.Address, slot uint64) common.Hash {
	paddedAddress := common.LeftPadBytes(address.Bytes(), 32)
	paddedSlot := common.LeftPadBytes(big.NewInt(int64(slot)).Bytes(), 32)
	return crypto.Keccak256Hash(paddedAddress, paddedSlot)
}
