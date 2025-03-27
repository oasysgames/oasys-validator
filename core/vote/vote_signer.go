package vote

import (
	"context"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/prysmaticlabs/prysm/v5/crypto/bls"
	validatorpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1/validator-client"
	"github.com/prysmaticlabs/prysm/v5/validator/accounts/iface"
	"github.com/prysmaticlabs/prysm/v5/validator/accounts/wallet"
	"github.com/prysmaticlabs/prysm/v5/validator/keymanager"
<<<<<<< HEAD
	"github.com/prysmaticlabs/prysm/v5/validator/keymanager/local"
=======
>>>>>>> 294c7321ab439545b2ab1bb7eea74a44d83e94a1

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
)

const (
	voteSignerTimeout = time.Second * 5
)

var votesSigningErrorCounter = metrics.NewRegisteredCounter("votesSigner/error", nil)

type VoteSigner struct {
	km     *keymanager.IKeymanager
	PubKey [48]byte
}

<<<<<<< HEAD
func NewVoteSigner(blsPasswordPath, blsWalletPath, blsAccountName string) (*VoteSigner, error) {
=======
func NewVoteSigner(blsPasswordPath, blsWalletPath string) (*VoteSigner, error) {
>>>>>>> 294c7321ab439545b2ab1bb7eea74a44d83e94a1
	dirExists, err := wallet.Exists(blsWalletPath)
	if err != nil {
		log.Error("Check BLS wallet exists", "err", err)
		return nil, err
	}
	if !dirExists {
		log.Error("BLS wallet did not exists.")
		return nil, errors.New("BLS wallet did not exists")
	}

	walletPassword, err := os.ReadFile(blsPasswordPath)
	if err != nil {
		log.Error("Read BLS wallet password", "err", err)
		return nil, err
	}
	log.Info("Read BLS wallet password successfully")

	w, err := wallet.OpenWallet(context.Background(), &wallet.Config{
		WalletDir:      blsWalletPath,
		WalletPassword: string(walletPassword),
	})
	if err != nil {
		log.Error("Open BLS wallet failed", "err", err)
		return nil, err
	}
	log.Info("Open BLS wallet successfully")

	km, err := w.InitializeKeymanager(context.Background(), iface.InitKeymanagerConfig{ListenForChanges: false})
	if err != nil {
		log.Error("Initialize key manager failed", "err", err)
		return nil, err
	}
	log.Info("Initialized keymanager successfully")

	ctx, cancel := context.WithTimeout(context.Background(), voteSignerTimeout)
	defer cancel()

	pubKeys, err := km.FetchValidatingPublicKeys(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not fetch validating public keys")
	}
<<<<<<< HEAD
	if len(pubKeys) == 0 {
		return nil, errors.New("no public keys in the BLS wallet")
	}

	// The default uses the first found key.
	pubKey := pubKeys[0]

	// If a key name is specified, find for it, but use the first key if not found.
	if blsAccountName != "" {
		ikm, ok := km.(*local.Keymanager)
		if !ok {
			return nil, errors.New("could not assert BLS keymanager interface to concrete type")
		}
		accountNames, err := ikm.ValidatingAccountNames()
		if err != nil {
			return nil, errors.Wrap(err, "could not fetch BLS account names")
		}
		var found bool
		for i := 0; i < len(accountNames) && !found; i++ {
			found = accountNames[i] == blsAccountName
			if found {
				pubKey = pubKeys[i]
			}
		}
		if !found {
			log.Warn("Configured voting BLS public key was not found, so the default key will be used",
				"configured", blsAccountName, "default", accountNames[0])
		}
	}

	return &VoteSigner{
		km:     &km,
		PubKey: pubKey,
=======

	return &VoteSigner{
		km:     &km,
		PubKey: pubKeys[0],
>>>>>>> 294c7321ab439545b2ab1bb7eea74a44d83e94a1
	}, nil
}

func (signer *VoteSigner) SignVote(vote *types.VoteEnvelope) error {
	// Sign the vote, fetch the first pubKey as validator's bls public key.
	pubKey := signer.PubKey
	blsPubKey, err := bls.PublicKeyFromBytes(pubKey[:])
	if err != nil {
		return errors.Wrap(err, "convert public key from bytes to bls failed")
	}

	voteDataHash := vote.Data.Hash()

	ctx, cancel := context.WithTimeout(context.Background(), voteSignerTimeout)
	defer cancel()

	signature, err := (*signer.km).Sign(ctx, &validatorpb.SignRequest{
		PublicKey:   pubKey[:],
		SigningRoot: voteDataHash[:],
	})
	if err != nil {
		return err
	}

	copy(vote.VoteAddress[:], blsPubKey.Marshal()[:])
	copy(vote.Signature[:], signature.Marshal()[:])
	return nil
}
