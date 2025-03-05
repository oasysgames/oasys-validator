package fakebeacon

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

func fetchBlockNumberByTime(ctx context.Context, ts int64, backend ethapi.Backend) (*types.Header, error) {
	// calc the block number of the ts.
	currentHeader := backend.CurrentHeader()
	blockTime := int64(currentHeader.Time)
	if ts > blockTime {
		return nil, errors.New("time too large")
	}
	blockPeriod := getBlockPeriod(backend.ChainConfig())
	blockNum := currentHeader.Number.Uint64()
	estimateEndNumber := int64(blockNum) - (blockTime-ts)/blockPeriod
	// find the end number
	for {
		header, err := backend.HeaderByNumber(ctx, rpc.BlockNumber(estimateEndNumber))
		if err != nil {
			time.Sleep(time.Duration(rand.Int()%180) * time.Millisecond)
			continue
		}
		if header == nil {
			estimateEndNumber -= 1
			time.Sleep(time.Duration(rand.Int()%180) * time.Millisecond)
			continue
		}
		headerTime := int64(header.Time)
		if headerTime == ts {
			return header, nil
		}

		// let the estimateEndNumber a little bigger than real value
		if headerTime > ts+(blockPeriod*3-1) {
			estimateEndNumber -= (headerTime - ts) / blockPeriod
		} else if headerTime < ts {
			estimateEndNumber += (ts-headerTime)/blockPeriod + 1
		} else {
			// search one by one
			for headerTime >= ts {
				header, err = backend.HeaderByNumber(ctx, rpc.BlockNumber(estimateEndNumber-1))
				if err != nil {
					time.Sleep(time.Duration(rand.Int()%180) * time.Millisecond)
					continue
				}
				headerTime = int64(header.Time)
				if headerTime == ts {
					return header, nil
				}
				estimateEndNumber -= 1
				if headerTime < ts { //found the real endNumber
					return nil, fmt.Errorf("block not found by time %d", ts)
				}
			}
		}
	}
}

func getBlockPeriod(cfg *params.ChainConfig) int64 {
	switch {
	case cfg.ChainID.Cmp(params.OasysMainnetChainConfig.ChainID) == 0:
		return params.SHORT_BLOCK_TIME_SECONDS
	case cfg.ChainID.Cmp(params.OasysTestnetChainConfig.ChainID) == 0:
		return params.SHORT_BLOCK_TIME_SECONDS
	case cfg.Oasys != nil:
		return int64(cfg.Oasys.Period) // for local chain
	default:
		return 1
	}
}
