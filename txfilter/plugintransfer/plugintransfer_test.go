package main

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	gethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
)

func hashFromIndex(i uint64) common.Hash {
	return crypto.Keccak256Hash([]byte(fmt.Sprintf("tx%d", i)))
}

func makeElapsedItems(c *lrucache, window time.Duration) {
	for i := uint(0); i < c.len(); i++ {
		meta := c.get(i)
		meta.createdAt = meta.createdAt.Add(-window)
	}
}

func checkLinearSearch(c *lrucache, threshold uint64, window time.Duration, currentAmount uint64) (block bool, reason string, index uint) {
	windowStart := time.Now().Add(-window)

	// Check if the current tx exceeds the threshold
	if threshold < currentAmount {
		return true, fmt.Sprintf("over block amount threshold: %d (single tx)", threshold), 0
	}

	for i := uint(0); i < c.len(); i++ {
		meta := c.get(i)
		sum := computeTotal(c, meta, currentAmount)
		// if bellow threadhold, exit
		if sum <= threshold {
			return false, "", i
		}
		// if within window, return true
		if meta.createdAt.After(windowStart) {
			return true, fmt.Sprintf("over block amount threshold: %d, window sum: %d, current tx amount: %d", threshold, sum, currentAmount), i
		}
	}

	return
}

func rawFromString256(s string) [32]byte {
	v := gethmath.MustParseBig256(s)
	b := gethmath.PaddedBigBytes(v, 32)
	var raw [32]byte
	copy(raw[:], b)
	return raw
}

func TestAmountFromRaw(t *testing.T) {
	var maxRaw [32]byte
	for i := range maxRaw {
		maxRaw[i] = 0xff
	}

	tests := []struct {
		name     string
		value    [32]byte
		decimals uint8
		want     uint64
	}{
		{
			name:     "zero",
			value:    [32]byte{},
			decimals: 18,
			want:     0,
		},
		{
			name:     "no decimals",
			value:    rawFromString256("12345"),
			decimals: 0,
			want:     12345,
		},
		{
			name:     "exact ether unit",
			value:    rawFromString256("1000000000000000000"), // 1e18
			decimals: 18,
			want:     1,
		},
		{
			name:     "fractional truncation",
			value:    rawFromString256("1500000000000000000"), // 1.5e18
			decimals: 18,
			want:     1,
		},
		{
			name:     "total supply",
			value:    rawFromString256("10000000000000000000000000000"), // 10,000,000,000 x 10^18
			decimals: 18,
			want:     10_000_000_000,
		},
		{
			name:     "usdc style decimals",
			value:    rawFromString256("123456789"),
			decimals: 6,
			want:     123,
		},
		{
			name:     "overflow saturates",
			value:    maxRaw,
			decimals: 18,
			want:     math.MaxUint64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := amountFromRaw(tt.value, tt.decimals)
			if got != tt.want {
				t.Errorf("amountFromRaw(..., %d) = %d, want %d", tt.decimals, got, tt.want)
			}
		})
	}
}

func TestLruCache_push(t *testing.T) {
	c := newCache(5)

	// Push 3 items
	c.push(hashFromIndex(0), 10)
	c.push(hashFromIndex(1), 20)
	c.push(hashFromIndex(2), 30)

	if c.len() != 3 {
		t.Errorf("len() = %d, want 3", c.len())
	}

	// Verify order: index 0=oldest (amount 10), index 1 (20), index 2=newest (30)
	for i := uint64(0); i < 3; i++ {
		meta := c.get(uint(i))
		if meta == nil {
			t.Fatalf("get(%d) returned nil", i)
		}
		if meta.amount != 10*(i+1) {
			t.Errorf("get(%d).amount = %d, want %d", i, meta.amount, 10*(i+1))
		}
		// accumulatedAmount: 0=>0, 1=>10, 2=>30
		expectedAccum := uint64(0)
		for j := uint64(0); j < i; j++ {
			expectedAccum += 10 * (j + 1)
		}
		if meta.accumulatedAmount != expectedAccum {
			t.Errorf("get(%d).accumulatedAmount = %d, want %d", i, meta.accumulatedAmount, expectedAccum)
		}
	}

	// Out of range
	if c.get(5) != nil {
		t.Error("get(5) should return nil")
	}

	// contains
	for i := uint64(0); i < 3; i++ {
		if !c.contains(hashFromIndex(i)) {
			t.Errorf("contains(hash %d) should be true", i)
		}
	}
	if c.contains(hashFromIndex(99)) {
		t.Error("contains(unknown hash) should be false")
	}

	// push more items, should evict the oldest
	c.push(hashFromIndex(3), 40)
	c.push(hashFromIndex(4), 50)
	c.push(hashFromIndex(5), 60)
	if c.len() != 5 {
		t.Errorf("len() = %d, want 5", c.len())
	}
	if c.contains(hashFromIndex(0)) {
		t.Error("hash 0 should have been evicted")
	}
	if !c.contains(hashFromIndex(1)) || !c.contains(hashFromIndex(2)) || !c.contains(hashFromIndex(3)) || !c.contains(hashFromIndex(4)) || !c.contains(hashFromIndex(5)) {
		t.Error("hashes 1,2,3,4,5 should be present")
	}
	if oldest := c.getOldest(); oldest == nil || oldest.amount != 20 {
		t.Errorf("oldest.amount = %d, want 20", oldest.amount)
	}
	if newest := c.getNewest(); newest == nil || newest.amount != 60 {
		t.Errorf("newest.amount = %d, want 60", newest.amount)
	}
	if newest := c.getNewest(); newest == nil || newest.accumulatedAmount != (10+20+30+40+50) {
		t.Errorf("newest.accumulatedAmount = %d, want 150", newest.accumulatedAmount)
	}

	// overflow check
	c.push(hashFromIndex(6), math.MaxUint64-100)
	c.push(hashFromIndex(7), 80)
	newest := c.getNewest()
	if newest.isOddOverflow != true {
		t.Errorf("newest.isOddOverflow = %t, want true", newest.isOddOverflow)
	}
	expectedAccumulatedAmount := (10 + 20 + 30 + 40 + 50 + 60) - 100 - 1
	if newest.accumulatedAmount != uint64(expectedAccumulatedAmount) {
		t.Errorf("newest.accumulatedAmount = %d, want %d", newest.accumulatedAmount, expectedAccumulatedAmount)
	}
}

func TestLruCache_expandCap(t *testing.T) {
	c := newCache(5)

	// push 6 items
	c.push(hashFromIndex(0), 10)
	c.push(hashFromIndex(1), 20)
	c.push(hashFromIndex(2), 30)
	c.push(hashFromIndex(3), 40)
	c.push(hashFromIndex(4), 50)
	c.push(hashFromIndex(5), 60)

	// expand cap to 6
	c.expandCap(6)

	// check cap and len
	if c.cap != 6 {
		t.Errorf("after expand: cap = %d, want 6", c.cap)
	}
	if c.len() != 5 {
		t.Errorf("after expand: len = %d, want 5", c.len())
	}

	// check order and data preserved
	accumulatedAmount := uint64(10)
	for i := uint(0); i < 5; i++ {
		meta := c.get(i)
		expectedAmount := uint64(10 * (i + 2))
		if meta.amount != expectedAmount {
			t.Errorf("after expand get(%d): got %v", i, meta)
		}
		if meta.accumulatedAmount != accumulatedAmount {
			t.Errorf("after expand get(%d): accumulatedAmount = %d, want %d", i, meta.accumulatedAmount, accumulatedAmount)
		}
		accumulatedAmount += expectedAmount
		if meta.isOddOverflow != false {
			t.Errorf("after expand get(%d): isOddOverflow = %t, want false", i, meta.isOddOverflow)
		}
	}

	// check out of range
	if meta := c.get(6); meta != nil {
		t.Errorf("after expand get(6): got %v, want nil", meta)
	}

	// push 2 more items, should evict the oldest
	c.push(hashFromIndex(6), 70)
	c.push(hashFromIndex(7), 80)
	if c.len() != 6 {
		t.Errorf("after more pushes: len = %d, want 6", c.len())
	}
	if c.contains(hashFromIndex(1)) {
		t.Error("hash 1 should have been evicted")
	}
	if !c.contains(hashFromIndex(2)) || !c.contains(hashFromIndex(3)) || !c.contains(hashFromIndex(4)) || !c.contains(hashFromIndex(5)) || !c.contains(hashFromIndex(6)) {
		t.Error("hashes 2,3,4,5,6 should be present")
	}
	if oldest := c.getOldest(); oldest == nil || oldest.amount != 30 {
		t.Errorf("oldest.amount = %d, want 30", oldest.amount)
	}
	if newest := c.getNewest(); newest == nil || newest.amount != 80 {
		t.Errorf("newest.amount = %d, want 80", newest.amount)
	}
}

func TestComputeTotal(t *testing.T) {
	// Non-overflow case
	c := newCache(5)
	c.push(hashFromIndex(0), 10)
	c.push(hashFromIndex(1), 20)
	c.push(hashFromIndex(2), 30)
	if got := computeTotal(c, c.get(0), 5); got != 10+20+30+5 {
		t.Errorf("computeTotal(c, 0, 5) = %d, want %d", got, 10+20+30+5)
	}
	if got := computeTotal(c, c.get(1), 5); got != 20+30+5 {
		t.Errorf("computeTotal(c, 1, 5) = %d, want %d", got, 20+30+5)
	}
	if got := computeTotal(c, c.get(2), 5); got != 30+5 {
		t.Errorf("computeTotal(c, 2, 5) = %d, want %d", got, 30+5)
	}

	// Overflow case
	c.push(hashFromIndex(3), math.MaxUint64-100)
	c.push(hashFromIndex(4), 50)
	if got := computeTotal(c, c.get(2), 5); got != 30+math.MaxUint64-100+50+5 {
		t.Errorf("computeTotal(c, 3, 5) = %d, want %d", got, uint64(math.MaxUint64-100+50+5))
	}
	if got := computeTotal(c, c.get(3), 5); got != math.MaxUint64-100+50+5 {
		t.Errorf("computeTotal(c, 3, 5) = %d, want %d", got, uint64(math.MaxUint64-100+50+5))
	}
	if got := computeTotal(c, c.get(4), 5); got != 50+5 {
		t.Errorf("computeTotal(c, 4, 5) = %d, want %d", got, 50+5)
	}
}

func TestCheckAmountThreshold(t *testing.T) {
	c := newCache(5)
	c.push(hashFromIndex(0), 10)
	c.push(hashFromIndex(1), 20)
	c.push(hashFromIndex(2), 30)
	window := 1 * time.Second

	// Single tx exceeds threshold
	threshold := uint64(100)
	block, reason, index := checkAmountThreshold(c, threshold, window, threshold+1)
	if !block {
		t.Errorf("expected block when single tx exceeds threshold, reason: %s", reason)
	}
	if !strings.Contains(reason, "single tx") {
		t.Errorf("reason %q should mention single tx", reason)
	}
	if index != 0 {
		t.Errorf("expected index 0, got %d", index)
	}

	// Sum of all below threshold
	block, reason, index = checkAmountThreshold(c, threshold, window, 40)
	if block {
		t.Errorf("expected no block when sum of all below threshold, reason: %s", reason)
	}
	if index != 0 {
		t.Errorf("expected index 0, got %d", index)
	}

	// Sum of all exceeds threshold
	block, reason, index = checkAmountThreshold(c, threshold, window, 41)
	if !block {
		t.Errorf("expected block when sum of all exceeds threshold, reason: %s", reason)
	}
	if index != 0 {
		t.Errorf("expected index 0, got %d", index)
	}

	makeElapsedItems(c, window)
	c.push(hashFromIndex(3), 40)
	c.push(hashFromIndex(4), 50)

	// not exceed thresholds
	threshold = uint64(30 + 40 + 50 + 1)
	block, reason, index = checkAmountThreshold(c, threshold, window, 1)
	if block {
		t.Errorf("expected no block, reason: %s", reason)
	}
	if index != 2 {
		t.Errorf("expected index 2, got %d", index)
	}
	block, reason, index = checkAmountThreshold(c, threshold, window, 30+1)
	if block {
		t.Errorf("expected no block, reason: %s", reason)
	}
	if index != 3 {
		t.Errorf("expected index 3, got %d", index)
	}

	// exceed thresholds
	block, _, index = checkAmountThreshold(c, threshold, window, 30+2)
	if !block {
		t.Error("expected block")
	}
	if index != 3 {
		t.Errorf("expected index 3, got %d", index)
	}
	block, _, index = checkAmountThreshold(c, threshold, window, 30+40+2)
	if !block {
		t.Error("expected block")
	}
	if index != 4 {
		t.Errorf("expected index 4, got %d", index)
	}

	c.push(hashFromIndex(5), 60) // evic the oldest item

	// not exceed thresholds
	threshold = uint64(40 + 50 + 60 + 1)
	block, reason, index = checkAmountThreshold(c, threshold, window, 1)
	if block {
		t.Errorf("expected no block, reason: %s", reason)
	}
	if index != 1 {
		t.Errorf("expected index 1, got %d", index)
	}

	// exceed thresholds
	block, _, index = checkAmountThreshold(c, threshold, window, 2)
	if !block {
		t.Error("expected block")
	}
	if index != 2 {
		t.Errorf("expected index 2, got %d", index)
	}

	makeElapsedItems(c, window)
	c.push(hashFromIndex(6), 70)

	// not exceed thresholds
	block, reason, index = checkAmountThreshold(c, uint64(60+70+1), window, 1)
	if block {
		t.Errorf("expected no block, reason: %s", reason)
	}
	if index != 3 {
		t.Errorf("expected index 3, got %d", index)
	}
	block, reason, index = checkAmountThreshold(c, uint64(40+50+60+70+1), window*2, 1)
	if block {
		t.Errorf("expected no block, reason: %s", reason)
	}
	if index != 1 {
		t.Errorf("expected index 1, got %d", index)
	}

	// exceed thresholds
	block, _, index = checkAmountThreshold(c, uint64(70+1), window, 2)
	if !block {
		t.Error("expected block")
	}
	if index != 4 {
		t.Errorf("expected index 4, got %d", index)
	}
	block, _, index = checkAmountThreshold(c, uint64(50+60+70+1), window*2, 2)
	if !block {
		t.Error("expected block")
	}
	if index != 2 {
		t.Errorf("expected index 2, got %d", index)
	}
	block, _, index = checkAmountThreshold(c, uint64(50+60+70+1), window*2, 1)
	if !block {
		t.Error("expected block")
	}
	if index != 1 {
		t.Errorf("expected index 1, got %d", index)
	}
}

func TestCheckCountThreshold(t *testing.T) {
	c := newCache(5)
	window := 1 * time.Second

	// Empty cache
	block, reason := checkCountThreshold(c, 1, window, "warning")
	if block {
		t.Errorf("expected no block for empty cache, reason: %s", reason)
	}
	if reason != "" {
		t.Errorf("expected empty reason, got %q", reason)
	}

	c.push(hashFromIndex(0), 10)

	// Threshold not reached
	block, reason = checkCountThreshold(c, 2, window, "warning")
	if block {
		t.Errorf("expected no block when count is below threshold, reason: %s", reason)
	}
	if reason != "" {
		t.Errorf("expected empty reason, got %q", reason)
	}

	c.push(hashFromIndex(1), 20)

	// Warning threshold reached within window
	block, reason = checkCountThreshold(c, 2, window, "warning")
	if !block {
		t.Errorf("expected block=true when warning threshold is reached, reason: %s", reason)
	}
	if reason != "over warning count threshold: 2" {
		t.Errorf("reason = %q, want %q", reason, "over warning count threshold: 2")
	}

	// Block threshold reached within window
	block, reason = checkCountThreshold(c, 2, window, "block")
	if !block {
		t.Errorf("expected block=true when block threshold is reached, reason: %s", reason)
	}
	if reason != "over block count threshold: 2" {
		t.Errorf("reason = %q, want %q", reason, "over block count threshold: 2")
	}

	// Elapsed records should not match threshold
	makeElapsedItems(c, window)
	block, reason = checkCountThreshold(c, 2, window, "warning")
	if block {
		t.Errorf("expected no block for elapsed records, reason: %s", reason)
	}
	if reason != "" {
		t.Errorf("expected empty reason, got %q", reason)
	}

	// Mix elapsed and fresh records
	c.push(hashFromIndex(2), 30)
	block, reason = checkCountThreshold(c, 2, window, "warning")
	if block {
		t.Errorf("expected no block with only one fresh record in window, reason: %s", reason)
	}
	c.push(hashFromIndex(3), 40)
	block, reason = checkCountThreshold(c, 2, window, "warning")
	if !block {
		t.Errorf("expected block=true with two fresh records in window, reason: %s", reason)
	}
	if reason != "over warning count threshold: 2" {
		t.Errorf("reason = %q, want %q", reason, "over warning count threshold: 2")
	}
}

func TestCheckAmountThreshold_StressTest(t *testing.T) {
	var (
		cap        = uint(255)
		iterations = cap * 1000
		window     = 1 * time.Second
		threadhold = uint64(cap)
		c          = newCache(cap)
	)

	for i := uint(0); i < iterations; i++ {
		// amout between 1 and threadhold / 10
		amount := uint64(rand.Intn(int(threadhold/10)) + 1)
		c.push(hashFromIndex(uint64(i)), amount)

		// Randomly make some items elapsed
		if rand.Intn(int(cap/2)) == 0 {
			makeElapsedItems(c, window)
		}

		// Check with both algorithms
		block, reason, index := checkAmountThreshold(c, threadhold, window, amount)
		expectedBlock, expectedReason, expectedIndex := checkLinearSearch(c, threadhold, window, amount)
		if block != expectedBlock {
			t.Errorf("expected block %t, got %t, expected reason %s, got reason %s, expected index %d, got index %d", expectedBlock, block, expectedReason, reason, expectedIndex, index)
		}
	}
}
