package main

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// version can be set at build time using -ldflags "-X main.version=1.0.0"
var version = "1.0.0"

func FilterTransaction(from, to common.Address, value [32]byte, logs []types.Log) (isBlocked bool, reason string, err error) {
	// fmt.Println("BlockTransaction called", from, to, value, logs)
	// return true, "blocked by plugin", errors.New("blocked by plugin")
	return false, "", nil
}

func Version() string {
	return version
}
