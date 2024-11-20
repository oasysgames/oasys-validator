package core

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"golang.org/x/crypto/sha3"
)

var (
	// ErrUnauthorizedCall is returned if an unauthorized address calls the contract
	ErrUnauthorizedCall = errors.New("unauthorized call")
	evmAccessControl    = common.HexToAddress("0x520000000000000000000000000000000000003F")
	trueHash            = common.HexToHash("0x1")
)

func computeMapStorageKey(address common.Address, slot uint64) common.Hash {
	// Convert address to left-padded 32 bytes
	var paddedAddress [32]byte
	copy(paddedAddress[12:], address.Bytes())

	// Convert slot to 32 bytes
	slotBytes := new(big.Int).SetUint64(slot).Bytes()
	var paddedSlot [32]byte
	copy(paddedSlot[32-len(slotBytes):], slotBytes)

	// Concatenate paddedAddress and paddedSlot
	data := append(paddedAddress[:], paddedSlot[:]...)

	// Compute the keccak256 hash
	hash := sha3.NewLegacyKeccak256()
	hash.Write(data)
	var result common.Hash
	copy(result[:], hash.Sum(nil))

	return result
}

func isAllowedToCreate(evm *vm.EVM, address common.Address) bool {
	hash := computeMapStorageKey(address, 1)
	val := evm.StateDB.GetState(evmAccessControl, hash)
	fmt.Println("isAllowedToCreate", address, val)
	return val.Cmp(trueHash) == 0
}

func isDeniedToCall(evm *vm.EVM, address common.Address) bool {
	hash := computeMapStorageKey(address, 2)
	val := evm.StateDB.GetState(evmAccessControl, hash)
	fmt.Println("isDeniedToCall", address, val)
	return val.Cmp(trueHash) == 0
}
