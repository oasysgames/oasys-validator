package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// VerifyTx checks if it is ok to process the transaction.
func VerifyTx(tx *types.Transaction, from common.Address) error {
	return nil
}
