package vm

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/oasys"
)

var (
	// ErrAllTransactionBlocked is returned if all transactions are blocked
	ErrAllTransactionBlocked = errors.New("all transactions are blocked")

	// ErrAddressBlocked is returned if the address is blocked
	ErrAddressBlocked = errors.New("address is blocked")

	// Address of the transaction blocker contract
	TransactionBlockerContract = common.HexToAddress(oasys.TransactionBlocker)

	isBlockedAllSlot     = int64(1)
	isBlockedAddressSlot = uint64(2)
	isBlockedAllSlotHash = common.Hash(common.LeftPadBytes(big.NewInt(isBlockedAllSlot).Bytes(), 32))
)

// Check the `isBlockedAll` variable in the `TransactionBlocker` contract
func IsBlockedAll(state StateDB) bool {
	val := state.GetState(TransactionBlockerContract, isBlockedAllSlotHash)
	return val.Cmp(emptyHash) != 0
}

// Check the `_isBlockedAddress` mappings in the `TransactionBlocker` contract
func IsBlockedAddress(state StateDB, address common.Address) bool {
	hash := computeAddressMapStorageKey(address, isBlockedAddressSlot)
	val := state.GetState(TransactionBlockerContract, hash)
	return val.Cmp(emptyHash) != 0
}
