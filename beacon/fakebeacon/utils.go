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
		// highEdge and lowEdge represent the current known range of block headers.
		highEdge = currentHeader // higher bound block header
		lowEdge  *types.Header   // lower bound block header
		// Start the search from the current header.
		cursor = currentHeader
		// isBackward toggles the search direction to help narrow the range.
		// - true: search past blocks from cursor
		// - false: search future blocks from cursor
		isBackward = true
		// The average block time is used to estimate the block number.
		// The initial value is from the chain configuration.
		averageBlockTime = getBlockPeriod(backend.ChainConfig())
	)

	for {
		estimated := estimateBlockNumber(averageBlockTime, int64(cursor.Time), int64(cursor.Number.Uint64()), ts)

		var err error
		if cursor, err = backend.HeaderByNumber(ctx, rpc.BlockNumber(estimated)); err != nil {
			return nil, fmt.Errorf("failed to fetch block by timestamp %d: %v", ts, err)
		}

		// Succeed! Found the block.
		if int64(cursor.Time) == ts {
			return cursor, nil
		}

		// Alternate updating the boundaries to narrow the search range.
		if isBackward {
			if ts > int64(cursor.Time) {
				lowEdge = cursor
				isBackward = !isBackward // Toggle the search direction.
			} else {
				highEdge = cursor
				if highEdge.Number.Uint64() == 0 {
					// higher bound reached genesis block.
					// Occurs when the target time is earlier than the genesis block
					return nil, fmt.Errorf("failed to fetch block by timestamp %d: earlier than genesis %d", ts, highEdge.Time)
				}
			}
		} else {
			if ts < int64(cursor.Time) {
				highEdge = cursor
				isBackward = !isBackward // Toggle the search direction.
			} else {
				lowEdge = cursor
			}
		}

		// Low edge yet to be reached.
		if lowEdge == nil {
			continue
		}

		// Sanity check
		if highEdge.Number.Cmp(lowEdge.Number) <= 0 {
			return nil, fmt.Errorf("failed to fetch block by timestamp %d: highEdge %d <= lowEdge %d", ts, highEdge.Number, lowEdge.Number)
		}

		// Update average block time.
		averageBlockTime = int64((highEdge.Time - lowEdge.Time) / (highEdge.Number.Uint64() - lowEdge.Number.Uint64()))
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
		// Target time is in the past from source time.
		estimated := sourceNumber - shift
		if estimated < 0 {
			// Smaller than genesis block.
			return 0
		}
		return estimated
	}

	// Target time is in the future from source time.
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
