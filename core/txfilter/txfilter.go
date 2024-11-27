package txfilter

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/oasys"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	txFilter  = common.HexToAddress(oasys.TransactionFilter)
	emptyHash common.Hash
)

type statedb interface {
	GetState(addr common.Address, hash common.Hash) common.Hash
}

// Check the `_createAllowedList` mappings in the `TransactionFilter` contract
func IsAllowedToCreate(state statedb, from common.Address) bool {
	hash := computeAddressMapStorageKey(from, 1)
	val := state.GetState(txFilter, hash)
	return val.Cmp(emptyHash) != 0
}

// Check the `_callAllowedList` mappings in the `TransactionFilter` contract
func IsDeniedToCall(state statedb, to common.Address) bool {
	hash := computeAddressMapStorageKey(to, 2)
	val := state.GetState(txFilter, hash)
	return val.Cmp(emptyHash) != 0
}

func computeAddressMapStorageKey(address common.Address, slot uint64) common.Hash {
	paddedAddress := common.LeftPadBytes(address.Bytes(), 32)
	paddedSlot := common.LeftPadBytes(big.NewInt(int64(slot)).Bytes(), 32)
	return crypto.Keccak256Hash(paddedAddress, paddedSlot)
}
