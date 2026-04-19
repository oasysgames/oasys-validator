package vm

import (
	"bytes"
	"errors"

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

	isBlockedAddressSlot = uint64(1)
	isBlockedAllSlot     = uint64(2)
	blockedAllKeyHash    = computeAddressMapStorageKey(common.Address(bytes.Repeat([]byte{0xFF}, common.AddressLength)), isBlockedAllSlot)
)

// Call `isBlockedAddress` function in the `TransactionBlocker` contract by directly accessing the storage
func IsBlockedAddress(state StateDB, address common.Address) bool {
	hash := computeAddressMapStorageKey(address, isBlockedAddressSlot)
	val := state.GetState(TransactionBlockerContract, hash)
	return val.Cmp(emptyHash) != 0
}

// Call `isBlockedAll` function in the `TransactionBlocker` contract by directly accessing the storage
// Keep in mind that system transactions must remain unblocked.
func IsBlockedAll(state StateDB) bool {
	val := state.GetState(TransactionBlockerContract, blockedAllKeyHash)
	return val.Cmp(emptyHash) != 0
}
