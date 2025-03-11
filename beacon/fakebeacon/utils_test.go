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
	}{
		{
			name:                "stable block growth",
			chainLength:         100,
			candidateBlockTimes: []uint64{bt},
		},
		{
			name:                "delayed block growth",
			chainLength:         100,
			candidateBlockTimes: []uint64{bt, bt, bt, bt, bt, bt + 1, bt + 1, bt + 2},
		},
		{
			name:                "shorter block time",
			chainLength:         100,
			candidateBlockTimes: []uint64{bt - 3},
		},
		{
			name:                "longer block time",
			chainLength:         100,
			candidateBlockTimes: []uint64{bt + 3},
		},
		{
			name:                "blocktime chainge",
			chainLength:         100,
			candidateBlockTimes: append(genBtList(70, bt*2), genBtList(30, bt)...),
		},
		{
			name:                "big jumps",
			chainLength:         100 + 10,
			candidateBlockTimes: append(genBtList(99, bt), 1000*bt),
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := makeBackend(tt.chainLength, tt.candidateBlockTimes)
			for j := 0; j < tt.chainLength; j++ {
				targetBlock, _ := backend.HeaderByNumber(ctx, rpc.BlockNumber(j))
				actualBlock, err := fetchBlockNumberByTime(ctx, int64(targetBlock.Time), backend)
				if err != nil {
					t.Error(err)
				}
				if actualBlock.Number.Cmp(targetBlock.Number) != 0 {
					t.Errorf("test %d-%d: expected block number %d, got %d", i, j, targetBlock.Number, actualBlock.Number)
				}
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
