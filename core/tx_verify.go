package core

import (
	"errors"

	"github.com/ethereum/go-ethereum/core/types"
)

var (
	// ErrUnauthorizedDeployment is returned if an unauthorized address deploys the contract
	ErrUnauthorizedDeployment = errors.New("unauthorized deployment")
)

// VerifyTx checks if it is ok to process the transaction.
func VerifyTx(header *types.Header, tx *types.Transaction) error {
	if tx.To() == nil {
		return ErrUnauthorizedDeployment
	}
	return nil
}
