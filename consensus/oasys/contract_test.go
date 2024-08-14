package oasys

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/tests"
)

var (
	_environmentAddress  = common.HexToAddress("0x0000000000000000000000000000000000001000")
	_stakeManagerAddress = common.HexToAddress("0x0000000000000000000000000000000000001001")
)

func TestInitializeSystemContracts(t *testing.T) {
	wallets, accounts, err := makeWallets(1)
	if err != nil {
		t.Fatalf("failed to create test wallets: %v", err)
	}

	env, err := makeEnv(*wallets[0], *accounts[0])
	if err != nil {
		t.Fatalf("failed to create test env: %v", err)
	}

	header := &types.Header{
		Number:     big.NewInt(50),
		Coinbase:   accounts[0].Address,
		Difficulty: diffInTurn,
	}
	cx := env.chain
	txs := make([]*types.Transaction, 0)
	receipts := make([]*types.Receipt, 0)
	systemTxs := make([]*types.Transaction, 0)
	usedGas := uint64(0)
	mining := true

	err = env.engine.initializeSystemContracts(env.statedb, header, cx, &txs, &receipts, &systemTxs, &usedGas, mining)
	if err != nil {
		t.Fatalf("failed to call initializeSystemContracts method: %v", err)
	}
	if len(receipts) != 2 {
		t.Errorf("len(receipts), got %v, want 2", len(receipts))
	}
	if usedGas == 0 {
		t.Error("Block.GasUsed is zero")
	}

	envInitialized := env.statedb.GetState(_environmentAddress, common.HexToHash("0x00")).Big().Uint64()
	envStartBlock := env.statedb.GetState(_environmentAddress, common.HexToHash("0x02")).Big().Uint64()
	envStartEpoch := env.statedb.GetState(_environmentAddress, common.HexToHash("0x03")).Big().Uint64()
	envBlockPeriod := env.statedb.GetState(_environmentAddress, common.HexToHash("0x04")).Big().Uint64()
	envEpochPeriod := env.statedb.GetState(_environmentAddress, common.HexToHash("0x05")).Big().Uint64()
	envRewardRate := env.statedb.GetState(_environmentAddress, common.HexToHash("0x06")).Big().Uint64()
	envCommissionRate := env.statedb.GetState(_environmentAddress, common.HexToHash("0x07")).Big().Uint64()
	envValidatorThreshold := env.statedb.GetState(_environmentAddress, common.HexToHash("0x08"))
	envJailThreshold := env.statedb.GetState(_environmentAddress, common.HexToHash("0x09")).Big().Uint64()
	envJailPeriod := env.statedb.GetState(_environmentAddress, common.HexToHash("0x0a")).Big().Uint64()
	if envInitialized != 1 {
		t.Errorf("Environment.initialize: want=1 got=%d", envInitialized)
	}
	if envStartBlock != 0 {
		t.Errorf("Environment.startBlock: want=0 got=%d", envStartBlock)
	}
	if envStartEpoch != 1 {
		t.Errorf("Environment.startEpoch: want=1 got=%d", envStartEpoch)
	}
	if envBlockPeriod != 15 {
		t.Errorf("Environment.blockPeriod: want=15 got=%d", envBlockPeriod)
	}
	if envEpochPeriod != 5760 {
		t.Errorf("Environment.epochPeriod: want=5760 got=%d", envEpochPeriod)
	}
	if envRewardRate != 10 {
		t.Errorf("Environment.rewardRate: want=10 got=%d", envRewardRate)
	}
	if envCommissionRate != 10 {
		t.Errorf("Environment.commissionRate: want=10 got=%d", envCommissionRate)
	}
	if new(big.Int).Div(envValidatorThreshold.Big(), big.NewInt(params.Ether)).Uint64() != 10000000 {
		t.Errorf("Environment.validatorThreshold: want=10000000000000000000000000 got=%d", envValidatorThreshold)
	}
	if envJailThreshold != 500 {
		t.Errorf("Environment.jailThreshold: want=500 got=%d", envJailThreshold)
	}
	if envJailPeriod != 2 {
		t.Errorf("Environment.jailPeriod: want=2 got=%d", envJailPeriod)
	}

	stakeManagerInitialized := env.statedb.GetState(_stakeManagerAddress, common.HexToHash("0x00"))
	if stakeManagerInitialized.Big().Uint64() != 1 {
		t.Errorf("StakeManager.initialize not called")
	}

	for _, receipt := range []*types.Receipt{receipts[0], receipts[1]} {
		if receipt.TxHash == (common.Hash{0x00}) {
			t.Error("receipt.TxHash is empty")
		}
		if receipt.GasUsed == 0 {
			t.Error("receipt.GasUsed is zero")
		}
		if len(receipt.Logs) != 1 {
			t.Errorf("len(receipt.Logs), got %v, want 1", len(receipt.Logs))
		}
		if receipt.Bloom == (types.Bloom{}) {
			t.Error("receipt.Bloom is empty")
		}
		if receipt.BlockNumber.Uint64() != 50 {
			t.Errorf("receipt.BlockNumber, got %v, want 50", receipt.BlockNumber)
		}
	}
	if env.statedb.GetNonce(env.engine.signer) != 2 {
		t.Errorf("account nonce value, got %v, want 2", env.statedb.GetNonce(env.engine.signer))
	}
}

func TestUpdateEnvironmentValue(t *testing.T) {
	wallets, accounts, err := makeWallets(1)
	if err != nil {
		t.Fatalf("failed to create test wallets: %v", err)
	}

	env, err := makeEnv(*wallets[0], *accounts[0])
	if err != nil {
		t.Fatalf("failed to create test env: %v", err)
	}

	header := &types.Header{
		Number:     big.NewInt(50),
		Coinbase:   accounts[0].Address,
		Difficulty: diffInTurn,
	}
	cx := env.chain
	txs := make([]*types.Transaction, 0)
	receipts := make([]*types.Receipt, 0)
	systemTxs := make([]*types.Transaction, 0)
	usedGas := uint64(0)
	mining := true

	err = env.engine.updateEnvironmentValue(&environmentValue{
		StartBlock:         big.NewInt(10),
		StartEpoch:         big.NewInt(11),
		BlockPeriod:        big.NewInt(12),
		EpochPeriod:        big.NewInt(13),
		RewardRate:         big.NewInt(14),
		CommissionRate:     big.NewInt(15),
		ValidatorThreshold: big.NewInt(16),
		JailThreshold:      big.NewInt(17),
		JailPeriod:         big.NewInt(18),
	}, env.statedb, header, cx, &txs, &receipts, &systemTxs, &usedGas, mining)

	if err != nil {
		t.Fatalf("failed to call updateValue method: %v", err)
	}
	if len(receipts) != 1 {
		t.Errorf("len(receipts), got %v, want 1", len(receipts))
	}
	if usedGas == 0 {
		t.Error("Block.GasUsed is zero")
	}

	envUpdated := env.statedb.GetState(_environmentAddress, common.HexToHash("0x01")).Big().Uint64()
	envStartBlock := env.statedb.GetState(_environmentAddress, common.HexToHash("0x02")).Big().Uint64()
	envStartEpoch := env.statedb.GetState(_environmentAddress, common.HexToHash("0x03")).Big().Uint64()
	envBlockPeriod := env.statedb.GetState(_environmentAddress, common.HexToHash("0x04")).Big().Uint64()
	envEpochPeriod := env.statedb.GetState(_environmentAddress, common.HexToHash("0x05")).Big().Uint64()
	envRewardRate := env.statedb.GetState(_environmentAddress, common.HexToHash("0x06")).Big().Uint64()
	envCommissionRate := env.statedb.GetState(_environmentAddress, common.HexToHash("0x07")).Big().Uint64()
	envValidatorThreshold := env.statedb.GetState(_environmentAddress, common.HexToHash("0x08")).Big().Uint64()
	envJailThreshold := env.statedb.GetState(_environmentAddress, common.HexToHash("0x09")).Big().Uint64()
	envJailPeriod := env.statedb.GetState(_environmentAddress, common.HexToHash("0x0a")).Big().Uint64()
	if envUpdated != 1 {
		t.Errorf("Environment.updated: want=1 got=%d", envUpdated)
	}
	if envStartBlock != 10 {
		t.Errorf("Environment.startBlock: want=1 got=%d", envStartBlock)
	}
	if envStartEpoch != 11 {
		t.Errorf("Environment.startEpoch: want=2 got=%d", envStartEpoch)
	}
	if envBlockPeriod != 12 {
		t.Errorf("Environment.blockPeriod: want=3 got=%d", envBlockPeriod)
	}
	if envEpochPeriod != 13 {
		t.Errorf("Environment.epochPeriod: want=4 got=%d", envEpochPeriod)
	}
	if envRewardRate != 14 {
		t.Errorf("Environment.rewardRate: want=5 got=%d", envRewardRate)
	}
	if envCommissionRate != 15 {
		t.Errorf("Environment.commissionRate: want=6 got=%d", envCommissionRate)
	}
	if envValidatorThreshold != 16 {
		t.Errorf("Environment.validatorThreshold: want=7 got=%d", envValidatorThreshold)
	}
	if envJailThreshold != 17 {
		t.Errorf("Environment.jailThreshold: want=8 got=%d", envJailThreshold)
	}
	if envJailPeriod != 18 {
		t.Errorf("Environment.jailPeriod: want=9 got=%d", envJailPeriod)
	}

	receipt := receipts[0]
	if receipt.TxHash == (common.Hash{0x00}) {
		t.Error("receipt.TxHash is empty")
	}
	if receipt.GasUsed == 0 {
		t.Error("receipt.GasUsed is zero")
	}
	if len(receipt.Logs) != 1 {
		t.Errorf("len(receipt.Logs), got %v, want 1", len(receipt.Logs))
	}
	if receipt.Bloom == (types.Bloom{}) {
		t.Error("receipt.Bloom is empty")
	}
	if receipt.BlockNumber.Uint64() != 50 {
		t.Errorf("receipt.BlockNumber, got %v, want 50", receipt.BlockNumber)
	}
	if env.statedb.GetNonce(env.engine.signer) != 1 {
		t.Errorf("account nonce value, got %v, want 1", env.statedb.GetNonce(env.engine.signer))
	}
}

func TestSlash(t *testing.T) {
	wallets, accounts, err := makeWallets(1)
	if err != nil {
		t.Fatalf("failed to create test wallets: %v", err)
	}

	env, err := makeEnv(*wallets[0], *accounts[0])
	if err != nil {
		t.Fatalf("failed to create test env: %v", err)
	}

	validator := accounts[0].Address
	schedule := []*common.Address{}
	header := &types.Header{
		Number:     big.NewInt(50),
		Coinbase:   accounts[0].Address,
		Difficulty: diffInTurn,
	}
	cx := env.chain
	txs := make([]*types.Transaction, 0)
	receipts := make([]*types.Receipt, 0)
	systemTxs := make([]*types.Transaction, 0)
	usedGas := uint64(0)
	mining := true

	err = env.engine.slash(validator, schedule, env.statedb, header, cx, &txs, &receipts, &systemTxs, &usedGas, mining)
	if err != nil {
		t.Fatalf("failed to call slash method: %v", err)
	}
	if len(receipts) != 1 {
		t.Errorf("len(receipts), got %v, want 1", len(receipts))
	}
	if usedGas == 0 {
		t.Error("Block.GasUsed is zero")
	}

	slashed := env.statedb.GetState(_stakeManagerAddress, common.HexToHash("0x01"))
	if slashed.Big().Uint64() != 2 {
		t.Errorf("StakeManager.slash not called")
	}

	receipt := receipts[0]
	if receipt.TxHash == (common.Hash{0x00}) {
		t.Error("receipt.TxHash is empty")
	}
	if receipt.GasUsed == 0 {
		t.Error("receipt.GasUsed is zero")
	}
	if receipt.Bloom == (types.Bloom{}) {
		t.Error("receipt.Bloom is empty")
	}
	if receipt.BlockNumber.Uint64() != 50 {
		t.Errorf("receipt.BlockNumber, got %v, want 50", receipt.BlockNumber)
	}
	if env.statedb.GetNonce(env.engine.signer) != 1 {
		t.Errorf("account nonce value, got %v, want 1", env.statedb.GetNonce(env.engine.signer))
	}
}

func TestGetNextValidators(t *testing.T) {
	addressArrTy, _ := abi.NewType("address[]", "", nil)
	uint256ArrTy, _ := abi.NewType("uint256[]", "", nil)
	boolArrTy, _ := abi.NewType("bool[]", "", nil)
	uint256Ty, _ := abi.NewType("uint256", "", nil)

	// Return value of the `StakeManager.getValidators` method.
	returnTy1 := abi.Arguments{
		{Type: addressArrTy}, // owners
		{Type: addressArrTy}, // operators
		{Type: uint256ArrTy}, // stakes
		{Type: boolArrTy},    // candidates
		{Type: uint256Ty},    // newCursor
	}

	// Return value of the `CandidateValidatorManager.getHighStakes` method.
	returnTy2 := abi.Arguments{
		{Type: addressArrTy}, // owners
		{Type: addressArrTy}, // operators
		{Type: boolArrTy},    // actives
		{Type: boolArrTy},    // jailed
		{Type: uint256ArrTy}, // stakes
		{Type: boolArrTy},    // candidates
		{Type: uint256Ty},    // newCursor
	}

	var (
		wantOwners = []common.Address{
			common.HexToAddress("0x01"),
			common.HexToAddress("0x02"),
			common.HexToAddress("0x03"),
		}
		wantOperators = []common.Address{
			common.HexToAddress("0x04"),
			common.HexToAddress("0x05"),
			common.HexToAddress("0x06"),
		}
		wantStakes = []*big.Int{
			big.NewInt(0),
			big.NewInt(1),
			big.NewInt(2),
		}
	)
	var (
		// rbytes    = make([][]byte, 7)
		page      = 7
		howMany   = 100
		newCursor = howMany
		rbytes    = map[common.Address][][]byte{
			stakeManager.address:     make([][]byte, page),
			candidateManager.address: make([][]byte, page),
		}
	)
	for i := 0; i < page; i++ {
		var (
			owners     = make([]common.Address, howMany)
			operators  = make([]common.Address, howMany)
			actives    = make([]bool, howMany)
			jailed     = make([]bool, howMany)
			stakes     = make([]*big.Int, howMany)
			candidates = make([]bool, howMany)
		)
		for j := 0; j < howMany; j++ {
			if i%len(wantOwners) == 0 && j == howMany/2 {
				idx := i / len(wantOwners)
				owners[j] = wantOwners[idx]
				operators[j] = wantOperators[idx]
				actives[j] = true
				jailed[j] = false
				stakes[j] = wantStakes[idx]
				candidates[j] = true
			} else {
				owners[j] = common.Address{}
				operators[j] = common.Address{}
				actives[j] = true
				jailed[j] = false
				stakes[j] = big.NewInt(0)
				candidates[j] = false
			}
		}

		bnewCursor := big.NewInt(int64(newCursor))

		rbyte, _ := returnTy1.Pack(owners, operators, stakes, candidates, bnewCursor)
		rbytes[stakeManager.address][i] = rbyte

		rbyte, _ = returnTy2.Pack(owners, operators, actives, jailed, stakes, candidates, bnewCursor)
		rbytes[candidateManager.address][i] = rbyte

		if i == page-1 {
			rbyte, _ := returnTy1.Pack([]common.Address{}, []common.Address{}, []*big.Int{}, []bool{}, bnewCursor)
			rbytes[stakeManager.address] = append(rbytes[stakeManager.address], rbyte)

			rbyte, _ = returnTy2.Pack([]common.Address{}, []common.Address{}, []bool{}, []bool{}, []*big.Int{}, []bool{}, bnewCursor)
			rbytes[candidateManager.address] = append(rbytes[candidateManager.address], rbyte)

			break
		}

		newCursor += howMany
	}

	config := &params.ChainConfig{ChainID: big.NewInt(999999), Oasys: &params.OasysConfig{}}
	ethapi := &testBlockchainAPI{rbytes: rbytes}

	for _, block := range []uint64{1, 10} {
		got, _ := getNextValidators(config, ethapi, common.Hash{}, 1, block)

		if len(got.Owners) != len(wantOwners) {
			t.Errorf("invalid owners length, got: %d, want: %d", len(got.Owners), len(wantOwners))
		}
		if len(got.Operators) != len(wantOperators) {
			t.Errorf("invalid operators length, got: %d, want: %d", len(got.Operators), len(wantOperators))
		}
		if len(got.Stakes) != len(wantStakes) {
			t.Errorf("invalid stakes length, got: %d, want: %d", len(got.Stakes), len(wantStakes))
		}
		for i, want := range wantOwners {
			got := got.Owners[i]
			if got != want {
				t.Errorf("invalid owner, got %v, want: %v", got, want)
			}
		}
		for i, want := range wantOperators {
			got := got.Operators[i]
			if got != want {
				t.Errorf("invalid operator, got %v, want: %v", got, want)
			}
		}
		for i, want := range wantStakes {
			got := got.Stakes[i]
			if got.Cmp(want) != 0 {
				t.Errorf("invalid stake, got %v, want: %v", got, want)
			}
		}
	}
}

func TestGetRewards(t *testing.T) {
	want := big.NewInt(1902587519025875190)

	addressArrTy, _ := abi.NewType("address[]", "", nil)
	uint256Ty, _ := abi.NewType("uint256", "", nil)
	rbytes := make([][]byte, 2)

	// mocking to getValidatorOwners method
	rbyte, _ := abi.Arguments{{Type: addressArrTy}, {Type: uint256Ty}}.
		Pack([]common.Address{}, common.Big0)
	rbytes[0] = rbyte

	// mocking to getTotalRewards method
	rbyte, _ = abi.Arguments{{Type: uint256Ty}}.Pack(want)
	rbytes[1] = rbyte

	ethapi := &testBlockchainAPI{rbytes: map[common.Address][][]byte{stakeManager.address: rbytes}}
	got, _ := getRewards(ethapi, common.Hash{})
	if got.Cmp(want) != 0 {
		t.Errorf("got %v, want: %v", got, want)
	}
}

var _ blockchainAPI = (*testBlockchainAPI)(nil)

type testBlockchainAPI struct {
	rbytes map[common.Address][][]byte
	count  map[common.Address]int
}

func (p *testBlockchainAPI) Call(ctx context.Context, args ethapi.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, overrides *ethapi.StateOverride, blockOverrides *ethapi.BlockOverrides) (hexutil.Bytes, error) {
	if p.count == nil {
		p.count = map[common.Address]int{}
	}

	to := *args.To
	defer func() { p.count[to]++ }()
	return p.rbytes[to][p.count[to]], nil
}

type testEnv struct {
	engine  *Oasys
	chain   *core.BlockChain
	statedb *state.StateDB
}

func makeKeyStore() (*keystore.KeyStore, func(), error) {
	d, err := os.MkdirTemp("", "tmp-keystore")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		os.RemoveAll(d)
	}
	return keystore.NewPlaintextKeyStore(d), cleanup, nil
}

func makeWallets(count int) ([]*accounts.Wallet, []*accounts.Account, error) {
	keystore, cleanup, err := makeKeyStore()
	if err != nil {
		return nil, nil, err
	}
	defer cleanup()

	var (
		wallets  []*accounts.Wallet
		accounts []*accounts.Account
	)
	for i := 0; i < count; i++ {
		account, err := keystore.NewAccount("")
		if err != nil {
			return nil, nil, err
		}
		keystore.Unlock(account, "")
		wallets = append(wallets, &keystore.Wallets()[0])
		accounts = append(accounts, &account)
	}
	return wallets, accounts, err
}

func makeEnv(wallet accounts.Wallet, account accounts.Account) (*testEnv, error) {
	var (
		db          = rawdb.NewMemoryDatabase()
		chainConfig = &params.ChainConfig{
			ChainID:             big.NewInt(248),
			HomesteadBlock:      common.Big0,
			EIP150Block:         common.Big0,
			EIP155Block:         common.Big0,
			EIP158Block:         common.Big0,
			ByzantiumBlock:      common.Big0,
			ConstantinopleBlock: common.Big0,
			PetersburgBlock:     common.Big0,
			IstanbulBlock:       common.Big0,
			MuirGlacierBlock:    common.Big0,
			BerlinBlock:         common.Big0,
			LondonBlock:         common.Big0,
			Oasys: &params.OasysConfig{
				Period: 15,
				Epoch:  5760,
			},
		}
		genspec = &core.Genesis{
			Config:    chainConfig,
			ExtraData: make([]byte, extraVanity+common.AddressLength+extraSeal),
			BaseFee:   big.NewInt(params.InitialBaseFee),
			Alloc: map[common.Address]core.GenesisAccount{
				account.Address: {
					Balance: big.NewInt(params.Ether),
				},
				_environmentAddress: {
					/*
						pragma solidity ^0.8.2;

						contract Environment {
							struct EnvironmentValue {
								uint256 startBlock;
								uint256 startEpoch;
								uint256 blockPeriod;
								uint256 epochPeriod;
								uint256 rewardRate;
								uint256 commissionRate;
								uint256 validatorThreshold;
								uint256 jailThreshold;
								uint256 jailPeriod;
							}

							event Initialized();
							event Updated();

							address public initialized;
							address public updated;
							address public startBlock;
							address public startEpoch;
							address public blockPeriod;
							address public epochPeriod;
							address public rewardRate;
							address public commissionRate;
							address public validatorThreshold;
							address public jailThreshold;
							address public jailPeriod;

							function initialize(EnvironmentValue memory initialValue) external {
								initialized = address(1);
								_set(initialValue);
								emit Initialized();
							}

							function updateValue(EnvironmentValue memory newValue) external {
								updated = address(1);
								_set(newValue);
								emit Updated();
							}

							function _set(EnvironmentValue memory val) internal {
								startBlock = _conv(val.startBlock);
								startEpoch = _conv(val.startEpoch);
								blockPeriod = _conv(val.blockPeriod);
								epochPeriod = _conv(val.epochPeriod);
								rewardRate = _conv(val.rewardRate);
								commissionRate = _conv(val.commissionRate);
								validatorThreshold = _conv(val.validatorThreshold);
								jailThreshold = _conv(val.jailThreshold);
								jailPeriod = _conv(val.jailPeriod);
							}

							function _conv(uint256 val) internal pure returns (address) {
								return address(uint160(val));
							}
						}
					*/
					Code:    common.FromHex("0x608060405234801561001057600080fd5b50600436106100cf5760003560e01c80637b0a47ee1161008c578063af0dfd3e11610066578063af0dfd3e1461019d578063b5b7a184146101b0578063bd4eff19146101c3578063de833b50146101d657600080fd5b80637b0a47ee146101645780637b2aab0314610177578063a2c8b1771461018a57600080fd5b806308a54356146100d4578063158ef93e146100e957806348cd4cb1146101185780634fd101d71461012b5780635ea1d6f81461013e578063660f5c1714610151575b600080fd5b6100e76100e2366004610372565b6101e9565b005b6000546100fc906001600160a01b031681565b6040516001600160a01b03909116815260200160405180910390f35b6002546100fc906001600160a01b031681565b6008546100fc906001600160a01b031681565b6007546100fc906001600160a01b031681565b600a546100fc906001600160a01b031681565b6006546100fc906001600160a01b031681565b6001546100fc906001600160a01b031681565b6003546100fc906001600160a01b031681565b6004546100fc906001600160a01b031681565b6005546100fc906001600160a01b031681565b6100e76101d1366004610372565b610231565b6009546100fc906001600160a01b031681565b600080546001600160a01b031916600117905561020581610278565b6040517f5daa87a0e9463431830481fd4b6e3403442dfb9a12b9c07597e9f61d50b633c890600090a150565b600180546001600160a01b0319168117905561024c81610278565b6040517ff2e795d4a33ae9a0d3282888375b8ae781ea4de1cbf101ac96150aa95ccff0b490600090a150565b8051600280546001600160a01b03199081166001600160a01b0393841617909155602083015160038054831691841691909117905560408301516004805483169184169190911790556060830151600580548316918416919091179055608083015160068054831691841691909117905560a083015160078054831691841691909117905560c083015160088054831691841691909117905560e083015160098054831691841691909117905561010090920151600a80549093169116179055565b604051610120810167ffffffffffffffff8111828210171561036c57634e487b7160e01b600052604160045260246000fd5b60405290565b6000610120828403121561038557600080fd5b61038d61033a565b823581526020830135602082015260408301356040820152606083013560608201526080830135608082015260a083013560a082015260c083013560c082015260e083013560e0820152610100808401358183015250809150509291505056fea2646970667358221220b3a402d60729e72e8f9f0637840b4c292983cb008e033e7489e294fa7d9ab98b64736f6c634300080c0033"),
					Balance: common.Big0,
				},
				_stakeManagerAddress: {
					/*
						pragma solidity ^0.8.2;

						contract StakeManager {
						    event Initialized();
						    event Slashed();

						    address public initialized;
						    address public slashed;

						    function initialize(address _environment, address _allowlist) external {
						        initialized = address(1);
						        emit Initialized();
						    }

						    function slash(address operator, uint256 blocks) external {
						        slashed = address(2);
						        emit Slashed();
						    }
						}
					*/
					Code:    common.FromHex("0x608060405234801561001057600080fd5b506004361061004c5760003560e01c806302fb4d85146100515780630a1ca63214610066578063158ef93e14610095578063485cc955146100a8575b600080fd5b61006461005f366004610155565b6100bb565b005b600154610079906001600160a01b031681565b6040516001600160a01b03909116815260200160405180910390f35b600054610079906001600160a01b031681565b6100646100b636600461017f565b6100fb565b600180546001600160a01b03191660021790556040517f47b2ee6ee7941903015ab048b24cfc914794d0faf7eefa9af7f7db11da1e5ec090600090a15050565b600080546001600160a01b03191660011781556040517f5daa87a0e9463431830481fd4b6e3403442dfb9a12b9c07597e9f61d50b633c89190a15050565b80356001600160a01b038116811461015057600080fd5b919050565b6000806040838503121561016857600080fd5b61017183610139565b946020939093013593505050565b6000806040838503121561019257600080fd5b61019b83610139565b91506101a960208401610139565b9050925092905056fea2646970667358221220e483794b8d74ebc03d106484a9089b470a42aabbba5e8d9481ba5c28eb76f2fb64736f6c634300080c0033"),
					Balance: common.Big0,
				},
			},
		}
	)

	// Generate consensus engine
	engine := New(chainConfig, chainConfig.Oasys, db, nil)
	engine.Authorize(account.Address, wallet.SignData, wallet.SignTx)

	// Generate a batch of blocks, each properly signed
	chain, err := core.NewBlockChain(db, nil, genspec, nil, engine, vm.Config{}, nil, nil)
	if err != nil {
		return nil, err
	}

	// Generate StateDB
	_, _, statedb := tests.MakePreState(db, genspec.Alloc, false, rawdb.HashScheme)

	// Replace artifact bytecode
	environment.artifact.DeployedBytecode = fmt.Sprintf("0x%s", hex.EncodeToString(genspec.Alloc[_environmentAddress].Code))
	stakeManager.artifact.DeployedBytecode = fmt.Sprintf("0x%s", hex.EncodeToString(genspec.Alloc[_stakeManagerAddress].Code))

	return &testEnv{engine, chain, statedb}, nil
}
