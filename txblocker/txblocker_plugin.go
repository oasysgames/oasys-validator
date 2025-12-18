package main

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

var V int

func F() { fmt.Printf("Hello, number %d\n", V) }

func BlockTransaction(from, to common.Address, value [32]byte, logs []types.Log) (isBlocked bool, reason string, err error) {
	fmt.Println("BlockTransaction called", from, to, value, logs)
	return true, "blocked by plugin", errors.New("blocked by plugin")
}
