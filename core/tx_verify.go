package core

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

var (
	// ErrUnauthorizedDeployment is returned if an unauthorized address deploys the contract
	ErrUnauthorizedDeployment = errors.New("unauthorized deployment")
)

var (
	deployers = map[common.Address]interface{}{}
)

func init() {
	_deployers := []string{
		"0x6DaB888D2Cb96591004251B408dFFE741b58d009", // AltLayer
	}
	for _, addr := range _deployers {
		deployers[common.HexToAddress(addr)] = nil
	}
}

// VerifyTx checks if it is ok to process the transaction.
func VerifyTx(tx *types.Transaction, from common.Address) error {
	if tx.To() == nil {
		if _, ok := deployers[from]; !ok {
			return ErrUnauthorizedDeployment
		}
	}
	return nil
}
