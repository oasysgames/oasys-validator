package fakebeacon

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

func fetchBlockNumberByTime(ctx context.Context, ts int64, backend ethapi.Backend) (*types.Header, error) {
	currentHeader := backend.CurrentHeader()
	if ts == int64(currentHeader.Time) {
		// Found the block.
		return currentHeader, nil
	} else if ts > int64(currentHeader.Time) {
		// Future time so return an error.
		return nil, fmt.Errorf("future time %d, current time %d", ts, currentHeader.Time)
	}

	// Performs a range search to locate the block whose timestamp matches ts.
	// Gradually narrows down the search range.
	var (
		blockPeriod = getBlockPeriod(backend.ChainConfig())
		// highEdge and lowEdge represent the current known range of block headers.
		highEdge = currentHeader // higher bound block header
		lowEdge  *types.Header   // lower bound block header
		// Start the search from the current header.
		cursor = currentHeader
		// isBackward toggles the search direction to help narrow the range.
		// - true: search past blocks
		// - false: search future blocks
		isBackward = true
	)

	for {
		estimated := estimateBlockNumber(blockPeriod, int64(cursor.Time), int64(cursor.Number.Uint64()), ts)

		// Make sure the estimate number is within the range.
		// If exceed the range, narrow the range by `1`.
		if int64(highEdge.Number.Uint64()) < estimated {
			estimated = int64(highEdge.Number.Uint64()) - 1
		} else if lowEdge != nil && int64(lowEdge.Number.Uint64()) > estimated {
			estimated = int64(lowEdge.Number.Uint64()) + 1
		}

		var err error
		if cursor, err = backend.HeaderByNumber(ctx, rpc.BlockNumber(estimated)); err != nil {
			return nil, fmt.Errorf("failed to fetch block by number %d: %v", estimated, err)
		}

		// Succeed! Found the block.
		if int64(cursor.Time) == ts {
			return cursor, nil
		}

		// in case, lowEdge have not yet been found
		// If lowEdge hasn't been set yet, adjust highEdge when target is earlier.
		if lowEdge == nil && ts < int64(cursor.Time) {
			highEdge = cursor
			continue
		}

		// Alternate updating the boundaries to narrow the search range.
		if isBackward {
			lowEdge = cursor
		} else {
			highEdge = cursor
		}

		// Toggle the search direction.
		isBackward = !isBackward
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

func estimateBlockNumber(blockPeriod, sourceTime, sourceNumber, targetTime int64) int64 {
	diff := targetTime - sourceTime

	// Determine how many blocks to shift.
	var shift int64
	if abs(diff) < blockPeriod {
		shift = 1
	} else {
		shift = abs(diff) / blockPeriod
	}

	if diff < 0 {
		// Target time is in the past.
		return sourceNumber - shift
	}
	// Target time is in the future.
	return sourceNumber + shift
}

// abs returns the absolute value of an int64.
// Define as the starndard library does not have an abs for int64.
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
