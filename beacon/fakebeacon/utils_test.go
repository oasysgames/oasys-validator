package fakebeacon

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

func TestFetchBlockNumberByTime(t *testing.T) {
	var (
		ctx       = context.Background()
		bt        = uint64(params.SHORT_BLOCK_TIME_SECONDS)
		genBtList = func(n int, b uint64) []uint64 {
			blockTimes := make([]uint64, n)
			for i := 0; i < n; i++ {
				blockTimes[i] = b
			}
			return blockTimes
		}
	)

	tests := []struct {
		name                string
		chainLength         int
		candidateBlockTimes []uint64
		targetBlockNumber   int
		expectedAttempts    int
	}{
		{
			name:                "stable block growth",
			chainLength:         100,
			candidateBlockTimes: []uint64{bt},
			targetBlockNumber:   10,
			expectedAttempts:    2,
		},
		{
			name:                "delayed block growth",
			chainLength:         14400 * (7 + 1), // 1 week + 1
			candidateBlockTimes: []uint64{bt, bt, bt, bt, bt, bt + 1, bt + 1, bt + 2},
			targetBlockNumber:   14400,
			expectedAttempts:    6,
		},
		{
			name:                "shorter block time",
			chainLength:         14400,
			candidateBlockTimes: []uint64{bt - 3},
			targetBlockNumber:   200,
			expectedAttempts:    16,
		},
		{
			name:                "longer block time",
			chainLength:         14400 * 2,
			candidateBlockTimes: []uint64{bt + 3},
			targetBlockNumber:   14400,
			expectedAttempts:    3,
		},
		{
			name:                "blocktime chainge",
			chainLength:         1500,
			candidateBlockTimes: append(genBtList(1000, bt*2), genBtList(500, bt)...),
			targetBlockNumber:   500,
			expectedAttempts:    4,
		},
		{
			name:                "big jumps",
			chainLength:         1000 * 4,
			candidateBlockTimes: append(genBtList(999, bt), 1000*bt),
			targetBlockNumber:   2000,
			expectedAttempts:    12,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := makeBackend(tt.chainLength, tt.candidateBlockTimes)
			targetBlock, _ := backend.HeaderByNumber(ctx, rpc.BlockNumber(tt.targetBlockNumber))

			actualBlock, err := fetchBlockNumberByTime(ctx, int64(targetBlock.Time), backend)
			if err != nil {
				t.Error(err)
			}
			if actualBlock.Number.Cmp(targetBlock.Number) != 0 {
				t.Errorf("test %d: expected block number %d, got %d", i, targetBlock.Number, actualBlock.Number)
			}
			if backend.(*MockBackendForFakeBeacon).searchAttempts != tt.expectedAttempts {
				t.Errorf("test %d: expected search attempts %d, got %d", i, tt.expectedAttempts, backend.(*MockBackendForFakeBeacon).searchAttempts)
			}
		})
	}
}

type MockBackendForFakeBeacon struct {
	ethapi.Backend
	headers        []*types.Header
	searchAttempts int // Count the number of search attempts
}

func (b *MockBackendForFakeBeacon) CurrentHeader() *types.Header {
	return b.headers[len(b.headers)-1]
}

func (b *MockBackendForFakeBeacon) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
	if number < 0 || number > rpc.BlockNumber(len(b.headers)) {
		return nil, fmt.Errorf("out of range. block number: %d", number)
	}
	b.searchAttempts++
	return b.headers[number], nil
}

func (b *MockBackendForFakeBeacon) ChainConfig() *params.ChainConfig {
	return params.OasysMainnetChainConfig
}

func makeBackend(length int, candidateBlockTimes []uint64) ethapi.Backend {
	var (
		backend    = MockBackendForFakeBeacon{headers: make([]*types.Header, length)}
		timeCursor = uint64(time.Now().Unix()) // Blocktime start from this time
	)
	for blockNumber := 0; blockNumber < length; blockNumber++ {
		index := blockNumber % len(candidateBlockTimes)
		timeCursor += candidateBlockTimes[index]
		backend.headers[blockNumber] = &types.Header{
			Number: big.NewInt(int64(blockNumber)),
			Time:   timeCursor,
		}
	}
	return &backend
}
