package oasys

import (
	"context"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

func getNextValidators(
	config *params.ChainConfig,
	ethAPI blockchainAPI,
	hash common.Hash,
	epoch uint64,
	block uint64,
	evm *vm.EVM,
) (*nextValidators, error) {
	if config.IsForkedOasysPublication(new(big.Int).SetUint64(block)) {
		return callGetHighStakes(ethAPI, hash, epoch, evm)
	}
	return callGetValidators(ethAPI, hash, epoch, evm)
}

func callGetHighStakes(ethAPI blockchainAPI, hash common.Hash, epoch uint64, evm *vm.EVM) (*nextValidators, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		method  = "getHighStakes"
		result  nextValidators
		bpoch   = new(big.Int).SetUint64(epoch)
		cursor  = big.NewInt(0)
		howMany = big.NewInt(100)
	)
	for {
		data, err := candidateManager.abi.Pack(method, bpoch, cursor, howMany)
		if err != nil {
			return nil, err
		}

		var (
			sender  = vm.AccountRef(common.Address{})
			hexData = (hexutil.Bytes)(data)
			rbytes  []byte
		)
		if evm != nil {
			rbytes, _, err = evm.Call(sender, candidateManager.address, data, uint64(math.MaxUint64/2), nil)
		} else {
			rbytes, err = ethAPI.Call(
				ctx,
				ethapi.TransactionArgs{
					To:   &candidateManager.address,
					Data: &hexData,
				},
				rpc.BlockNumberOrHashWithHash(hash, false),
				nil)
		}
		if err != nil {
			return nil, err
		}

		var recv struct {
			Owners     []common.Address
			Operators  []common.Address
			Stakes     []*big.Int
			Candidates []bool
			NewCursor  *big.Int

			// unused
			Actives, Jailed []bool
		}
		if err := candidateManager.abi.UnpackIntoInterface(&recv, method, rbytes); err != nil {
			return nil, err
		} else if len(recv.Owners) == 0 {
			break
		}

		cursor = recv.NewCursor
		for i := range recv.Owners {
			if recv.Candidates[i] {
				result.Owners = append(result.Owners, recv.Owners[i])
				result.Operators = append(result.Operators, recv.Operators[i])
				result.Stakes = append(result.Stakes, recv.Stakes[i])
			}
		}
	}

	return &result, nil
}

func getRewards(ethAPI blockchainAPI, hash common.Hash, evm *vm.EVM) (*big.Int, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	validators, err := getValidatorOwners(ethAPI, hash, evm)
	if err != nil {
		return nil, err
	}

	var (
		chunks     [][]common.Address
		size       = 200
		start, end = 0, size
	)
	for {
		if end > len(validators) {
			chunks = append(chunks, validators[start:])
			break
		} else {
			chunks = append(chunks, validators[start:end])
			start, end = end, end+size
		}
	}

	var (
		method = "getTotalRewards"
		result = new(big.Int)
	)
	for _, chunk := range chunks {
		data, err := stakeManager.abi.Pack(method, chunk, common.Big1)
		if err != nil {
			return nil, err
		}

		var (
			sender  = vm.AccountRef(common.Address{})
			hexData = (hexutil.Bytes)(data)
			rbytes  []byte
		)
		if evm != nil {
			rbytes, _, err = evm.Call(sender, stakeManager.address, data, uint64(math.MaxUint64/2), nil)
		} else {
			rbytes, err = ethAPI.Call(
				ctx,
				ethapi.TransactionArgs{
					To:   &stakeManager.address,
					Data: &hexData,
				},
				rpc.BlockNumberOrHashWithHash(hash, false),
				nil)
		}
		if err != nil {
			return nil, err
		}

		var recv *big.Int
		if err := stakeManager.abi.UnpackIntoInterface(&recv, method, rbytes); err != nil {
			return nil, err
		}

		result.Add(result, recv)
	}

	return result, nil
}

// Call the `StakeManager.getValidators` method.
func callGetValidators(ethAPI blockchainAPI, hash common.Hash, epoch uint64, evm *vm.EVM) (*nextValidators, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		method  = "getValidators"
		result  nextValidators
		bepoch  = new(big.Int).SetUint64(epoch)
		cursor  = big.NewInt(0)
		howMany = big.NewInt(100)
	)
	for {
		data, err := stakeManager.abi.Pack(method, bepoch, cursor, howMany)
		if err != nil {
			return nil, err
		}

		var (
			sender  = vm.AccountRef(common.Address{})
			hexData = (hexutil.Bytes)(data)
			rbytes  []byte
		)
		if evm != nil {
			rbytes, _, err = evm.Call(sender, stakeManager.address, data, uint64(math.MaxUint64/2), nil)
		} else {
			rbytes, err = ethAPI.Call(
				ctx,
				ethapi.TransactionArgs{
					To:   &stakeManager.address,
					Data: &hexData,
				},
				rpc.BlockNumberOrHashWithHash(hash, false),
				nil)
		}
		if err != nil {
			return nil, err
		}

		var recv struct {
			Owners     []common.Address
			Operators  []common.Address
			Stakes     []*big.Int
			Candidates []bool
			NewCursor  *big.Int
		}
		if err := stakeManager.abi.UnpackIntoInterface(&recv, method, rbytes); err != nil {
			return nil, err
		} else if len(recv.Owners) == 0 {
			break
		}

		cursor = recv.NewCursor
		for i := range recv.Owners {
			if recv.Candidates[i] {
				result.Owners = append(result.Owners, recv.Owners[i])
				result.Operators = append(result.Operators, recv.Operators[i])
				result.Stakes = append(result.Stakes, recv.Stakes[i])
			}
		}
	}

	return &result, nil
}

// Call the `StakeManager.getValidatorOwners` method.
func getValidatorOwners(ethAPI blockchainAPI, hash common.Hash, evm *vm.EVM) ([]common.Address, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		method  = "getValidatorOwners"
		result  []common.Address
		cursor  = big.NewInt(0)
		howMany = big.NewInt(100)
	)
	for {
		data, err := stakeManager.abi.Pack(method, cursor, howMany)
		if err != nil {
			return nil, err
		}

		var (
			sender  = vm.AccountRef(common.Address{})
			hexData = (hexutil.Bytes)(data)
			rbytes  []byte
		)
		if evm != nil {
			rbytes, _, err = evm.Call(sender, stakeManager.address, data, uint64(math.MaxUint64/2), nil)
		} else {
			rbytes, err = ethAPI.Call(
				ctx,
				ethapi.TransactionArgs{
					To:   &stakeManager.address,
					Data: &hexData,
				},
				rpc.BlockNumberOrHashWithHash(hash, false),
				nil)
		}
		if err != nil {
			return nil, err
		}

		var recv struct {
			Owners    []common.Address
			NewCursor *big.Int
		}
		if err := stakeManager.abi.UnpackIntoInterface(&recv, method, rbytes); err != nil {
			return nil, err
		} else if len(recv.Owners) == 0 {
			break
		}

		cursor = recv.NewCursor
		result = append(result, recv.Owners...)
	}

	return result, nil
}

func getNextEnvironmentValue(ethAPI blockchainAPI, hash common.Hash, evm *vm.EVM) (*environmentValue, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	method := "nextValue"
	data, err := environment.abi.Pack(method)
	if err != nil {
		return nil, err
	}

	var (
		sender  = vm.AccountRef(common.Address{})
		hexData = (hexutil.Bytes)(data)
		rbytes  []byte
	)
	if evm != nil {
		rbytes, _, err = evm.Call(sender, environment.address, data, uint64(math.MaxUint64/2), nil)
	} else {
		rbytes, err = ethAPI.Call(
			ctx,
			ethapi.TransactionArgs{
				To:   &environment.address,
				Data: &hexData,
			},
			rpc.BlockNumberOrHashWithHash(hash, false),
			nil)
	}
	if err != nil {
		return nil, err
	}

	var recv struct{ Result environmentValue }
	if err := environment.abi.UnpackIntoInterface(&recv, method, rbytes); err != nil {
		return nil, err
	}

	return &recv.Result, nil
}
