package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	gethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func hashFromIndex(i uint64) common.Hash {
	return crypto.Keccak256Hash([]byte(fmt.Sprintf("tx%d", i)))
}

func makeElapsedItems(c *lrucache, window time.Duration) {
	for i := uint(0); i < c.len(); i++ {
		slot := c.physicalSlot(i)
		h := c.items[slot]
		meta := c.txMap[h]
		meta.createdAt -= int64(window/time.Second) + 1
		c.txMap[h] = meta // write back
	}
}

func checkLinearSearch(c *lrucache, threshold uint64, startTime int64, currentAmount uint64) (block bool, reason string, index uint) {
	// Check if the current tx exceeds the threshold
	if threshold < currentAmount {
		return true, fmt.Sprintf("over block amount threshold: %d (single tx)", threshold), 0
	}

	for i := uint(0); i < c.len(); i++ {
		meta, _ := c.get(i)
		sum := computeTotal(c, meta, currentAmount)
		// if bellow threadhold, exit
		if sum <= threshold {
			return false, "", i
		}
		// if within window, return true
		if meta.createdAt >= startTime {
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
			name:     "less then decimals",
			value:    rawFromString256("999999"),
			decimals: 6,
			want:     0,
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

func TestConfigCache(t *testing.T) {
	var countedTxs *lrucache
	whitelistA := common.HexToAddress("0x1000000000000000000000000000000000000001")
	whitelistB := common.HexToAddress("0x1000000000000000000000000000000000000002")
	targetA := common.HexToAddress("0x2000000000000000000000000000000000000001")
	configs := []PluginConfig{
		{
			Version:           1,
			Whitelists:        map[common.Address]bool{whitelistA: true},
			MeasurementWindow: 2 * time.Second,
			Threshold: ThresholdConfig{
				WarningCountThreshold:  2,
				BlockCountThreshold:    3,
				WarningAmountThreshold: 100,
				BlockAmountThreshold:   200,
			},
			NativeToken: NativeTokenConfig{
				ToYenRate: 0.123,
			},
			TargetERC20s: []TargetERC20Config{
				{
					Address:   targetA,
					Decimals:  6,
					ToYenRate: 155.61,
				},
			},
			Disabled: false,
		},
		{
			Version:           2,
			Whitelists:        map[common.Address]bool{whitelistB: true},
			MeasurementWindow: 3 * time.Second,
			Threshold: ThresholdConfig{
				WarningCountThreshold:  3,
				BlockCountThreshold:    5, // higher than v1 to verify expandCap
				WarningAmountThreshold: 150,
				BlockAmountThreshold:   300,
			},
			NativeToken: NativeTokenConfig{
				ToYenRate: 200,
			},
			TargetERC20s: []TargetERC20Config{
				{
					Address:   targetA,
					Decimals:  6,
					ToYenRate: 3,
				},
			},
			Disabled: false,
		},
	}

	reqCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		idx := reqCount
		if idx >= len(configs) {
			idx = len(configs) - 1
		}
		reqCount++
		if err := json.NewEncoder(w).Encode(configs[idx]); err != nil {
			t.Fatalf("failed to encode test config: %v", err)
		}
	}))
	defer srv.Close()

	configURL = srv.URL

	c := ConfigCache{
		ttl:    1 * time.Hour,
		client: http.Client{Timeout: 3 * time.Second},
	}

	if !c.isExpired() {
		t.Fatal("expected empty cache to be expired")
	}
	if !c.isEmptyConfig() {
		t.Fatal("expected empty config before update")
	}

	// 1st update: initialize config and countedTxs
	if err := c.update(&countedTxs); err != nil {
		t.Fatalf("update(v1) failed: %v", err)
	}

	if c.isExpired() {
		t.Fatal("expected cache to be fresh right after update")
	}
	if c.isEmptyConfig() {
		t.Fatal("expected config to be non-empty after update")
	}
	if countedTxs == nil {
		t.Fatal("expected countedTxs to be initialized")
	}
	if countedTxs.cap != configs[0].Threshold.BlockCountThreshold {
		t.Fatalf("countedTxs.cap = %d, want %d", countedTxs.cap, configs[0].Threshold.BlockCountThreshold)
	}
	if !c.isWhitelisted(whitelistA) {
		t.Fatal("expected whitelistA to be whitelisted in v1")
	}
	if c.isWhitelisted(whitelistB) {
		t.Fatal("expected whitelistB to not be whitelisted in v1")
	}
	if target, ok := c.isTargetERC20(targetA); !ok || target.Decimals != 6 || target.ToYenRate != 155.61 {
		t.Fatalf("expected targetA with decimals=6 and rate=2, got ok=%t target=%+v", ok, target)
	}

	// more than 1 yen
	nativeRaw := rawFromString256("10000000000000000000") // 10 * 10^18
	if got := c.toYen(nil, nativeRaw); got != 1 {
		t.Fatalf("toYen(native) = %d, want 1", got)
	}
	erc20Raw := rawFromString256("1000000") // 1 with decimals=6
	target, _ := c.isTargetERC20(targetA)
	if got := c.toYen(target, erc20Raw); got != 155 {
		t.Fatalf("toYen(erc20) = %d, want 155", got)
	}
	// less than 1 yen
	nativeRaw = rawFromString256("8000000000000000000") // 8 * 10^18
	if got := c.toYen(nil, nativeRaw); got != 0 {
		t.Fatalf("toYen(native) = %d, want 0", got)
	}
	erc20Raw = rawFromString256("999999") // 0.999999 with decimals=6
	if got := c.toYen(target, erc20Raw); got != 0 {
		t.Fatalf("toYen(erc20) = %d, want 0", got)
	}

	// 2nd update: version bump should expand countedTxs cap.
	if err := c.update(&countedTxs); err != nil {
		t.Fatalf("update(v2) failed: %v", err)
	}
	if countedTxs.cap != configs[1].Threshold.BlockCountThreshold {
		t.Fatalf("countedTxs.cap = %d, want %d after v2", countedTxs.cap, configs[1].Threshold.BlockCountThreshold)
	}
	if !c.isWhitelisted(whitelistB) {
		t.Fatal("expected whitelistB to be whitelisted in v2")
	}
}

func TestFilterTransaction(t *testing.T) {
	p := &Plugin
	defer p.Clear()

	from := common.HexToAddress("0x1000000000000000000000000000000000000001")
	to := common.HexToAddress("0x2000000000000000000000000000000000000001")
	erc20 := common.HexToAddress("0x2000000000000000000000000000000000000001")
	tx0 := hashFromIndex(1000)
	tx1 := hashFromIndex(1001)
	tx2 := hashFromIndex(1002)
	tx3 := hashFromIndex(1003)
	tx4 := hashFromIndex(1004)
	oneOAS := rawFromString256("1000000000000000000")     // 1 OAS
	belowOneOAS := rawFromString256("999999999999999999") // 0.99 OAS
	fiveOAS := rawFromString256("5000000000000000000")    // 5 OAS

	// Case 1: expired + empty config + update error => blocked
	configURL = "http://127.0.0.1:1/plugintransfer.json"
	p.configCache = ConfigCache{
		Config:    PluginConfig{},
		updatedAt: time.Time{},
		ttl:       1 * time.Hour,
		client:    http.Client{Timeout: 200 * time.Millisecond},
	}
	p.countedTxs = newCache(4)
	blocked, reason, err := p.FilterTransaction(tx0, from, to, oneOAS, nil)
	if err == nil {
		t.Fatalf("expected err, got nil")
	}
	if !strings.Contains(err.Error(), "failed to fetch config") {
		t.Fatalf("expected error to contain 'failed to fetch config', got: %s", err.Error())
	}
	if blocked || reason != "" {
		t.Fatalf("expected not blocked when config is empty and update fails, reason: %s", reason)
	}

	// Prepare stable in-memory config for the remaining scenario.

	p.configCache = ConfigCache{
		Config: PluginConfig{
			MeasurementWindow: 10 * time.Second,
			Threshold: ThresholdConfig{
				WarningCountThreshold:  1,
				BlockCountThreshold:    2,
				WarningAmountThreshold: 2,
				BlockAmountThreshold:   5,
			},
			NativeToken: NativeTokenConfig{
				ToYenRate: 1,
			},
			Whitelists: map[common.Address]bool{},
		},
		updatedAt: time.Now(),
		ttl:       1 * time.Hour,
	}
	p.countedTxs = newCache(4)

	// Case 2: disabled => allowed and not counted
	p.configCache.Config.Disabled = true
	blocked, reason, err = p.FilterTransaction(tx1, from, to, oneOAS, nil)
	if err != nil || blocked || reason != "" {
		t.Fatalf("disabled path expected allow, got blocked=%t reason=%q err=%v", blocked, reason, err)
	}
	if p.countedTxs.len() != 0 {
		t.Fatalf("expected no count in disabled path, got len=%d", p.countedTxs.len())
	}
	p.configCache.Config.Disabled = false

	// Case 3: whitelisted sender => allowed and not counted
	p.configCache.Config.Whitelists[from] = true
	blocked, reason, err = p.FilterTransaction(tx1, from, to, oneOAS, nil)
	if err != nil || blocked || reason != "" {
		t.Fatalf("whitelist path expected allow, got blocked=%t reason=%q err=%v", blocked, reason, err)
	}
	if p.countedTxs.len() != 0 {
		t.Fatalf("expected no count in whitelist path, got len=%d", p.countedTxs.len())
	}
	p.configCache.Config.Whitelists = map[common.Address]bool{}

	// Case 4: bellow one yen => allowed and not counted
	blocked, reason, err = p.FilterTransaction(tx1, from, to, belowOneOAS, nil)
	if err != nil || blocked || reason != "" {
		t.Fatalf("bellow one yen path expected allow, got blocked=%t reason=%q err=%v", blocked, reason, err)
	}
	if p.countedTxs.len() != 0 {
		t.Fatalf("expected no count in zero-amount path, got len=%d", p.countedTxs.len())
	}

	// Case 5: normal tx under thresholds => allowed and counted
	blocked, reason, err = p.FilterTransaction(tx1, from, to, oneOAS, nil)
	if err != nil || blocked || reason != "" {
		t.Fatalf("normal path expected allow, got blocked=%t reason=%q err=%v", blocked, reason, err)
	}
	if !p.countedTxs.contains(tx1) || p.countedTxs.len() != 1 {
		t.Fatalf("expected tx1 counted once, contains=%t len=%d", p.countedTxs.contains(tx1), p.countedTxs.len())
	}

	// Case 6: duplicate tx => allowed and count unchanged
	blocked, reason, err = p.FilterTransaction(tx1, from, to, oneOAS, nil)
	if err != nil || blocked || reason != "" {
		t.Fatalf("duplicate path expected allow, got blocked=%t reason=%q err=%v", blocked, reason, err)
	}
	if p.countedTxs.len() != 1 {
		t.Fatalf("expected count unchanged for duplicate tx, got len=%d", p.countedTxs.len())
	}

	// Case 7: amount block (single tx pushes amount over block threshold)
	blocked, reason, err = p.FilterTransaction(tx2, from, to, fiveOAS, nil)
	if err != nil {
		t.Fatalf("unexpected err in amount-block path: %v", err)
	}
	if !blocked {
		t.Fatalf("expected block by amount threshold, reason: %s", reason)
	}
	if !strings.Contains(reason, "over block amount threshold") {
		t.Fatalf("expected amount-threshold reason, got: %s", reason)
	}
	if p.countedTxs.contains(tx2) {
		t.Fatal("blocked tx should not be counted")
	}

	// Case 8: count block (third tx within window should block by count)
	// Keep amount thresholds high so only count threshold can trigger.
	// configCache.Config.Threshold.WarningAmountThreshold = math.MaxUint64
	// configCache.Config.Threshold.BlockAmountThreshold = math.MaxUint64
	p.countedTxs = newCache(4)
	blocked, reason, err = p.FilterTransaction(tx2, from, to, oneOAS, nil)
	if err != nil || blocked {
		t.Fatalf("first count-sequence tx expected allow, got blocked=%t reason=%q err=%v", blocked, reason, err)
	}
	blocked, reason, err = p.FilterTransaction(tx3, from, to, oneOAS, nil)
	if err != nil || blocked {
		t.Fatalf("second count-sequence tx expected allow, got blocked=%t reason=%q err=%v", blocked, reason, err)
	}
	blocked, reason, err = p.FilterTransaction(tx4, from, to, oneOAS, nil)
	if err != nil {
		t.Fatalf("unexpected err in count-block path: %v", err)
	}
	if !blocked {
		t.Fatalf("expected block by count threshold, reason: %s", reason)
	}
	if !strings.Contains(reason, "over count threshold: 2") {
		t.Fatalf("expected count-threshold reason, got: %s", reason)
	}

	// Case 9: erc20 amount block (third tx within window should block by amount)
	p.configCache.Config.Threshold.WarningCountThreshold = 100
	p.configCache.Config.Threshold.BlockCountThreshold = 100
	p.configCache.Config.Threshold.WarningAmountThreshold = 1
	p.configCache.Config.Threshold.BlockAmountThreshold = 2
	p.configCache.Config.TargetERC20s = []TargetERC20Config{
		{
			Address:   erc20,
			Decimals:  18,
			ToYenRate: 1,
		},
	}
	p.countedTxs = newCache(4)
	logs := []types.Log{
		{
			Address: erc20,
			Topics: []common.Hash{
				transferEventTopic,
				common.HexToHash("0x1"),
				common.HexToHash("0x2"),
			},
			Data: oneOAS[:],
		},
	}
	blocked, reason, err = p.FilterTransaction(tx2, from, to, [32]byte{}, logs)
	if err != nil || blocked {
		t.Fatalf("first erc20-sequence tx expected allow, got blocked=%t reason=%q err=%v", blocked, reason, err)
	}
	blocked, reason, err = p.FilterTransaction(tx3, from, to, oneOAS, nil)
	if err != nil || blocked {
		t.Fatalf("second erc20-sequence tx expected allow, got blocked=%t reason=%q err=%v", blocked, reason, err)
	}
	blocked, reason, err = p.FilterTransaction(tx4, from, to, [32]byte{}, logs)
	if err != nil {
		t.Fatalf("unexpected err in erc20 amount-block path: %v", err)
	}
	if !blocked {
		t.Fatalf("expected block by erc20 amount threshold, reason: %s", reason)
	}
	if !strings.Contains(reason, "over block amount threshold") {
		t.Fatalf("expected amount-threshold reason, got: %s", reason)
	}
	if p.countedTxs.contains(tx4) {
		t.Fatal("blocked tx should not be counted")
	}
}

func TestLruCache_push(t *testing.T) {
	countedTxs := newCache(5)
	c := countedTxs

	// Push 3 items
	c.push(hashFromIndex(0), 10, time.Now().Unix())
	c.push(hashFromIndex(1), 20, time.Now().Unix())
	c.push(hashFromIndex(2), 30, time.Now().Unix())

	if c.len() != 3 {
		t.Errorf("len() = %d, want 3", c.len())
	}

	// Verify order: index 0=oldest (amount 10), index 1 (20), index 2=newest (30)
	for i := uint64(0); i < 3; i++ {
		meta, ok := c.get(uint(i))
		if !ok {
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
	if _, ok := c.get(5); ok {
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
	c.push(hashFromIndex(3), 40, time.Now().Unix())
	c.push(hashFromIndex(4), 50, time.Now().Unix())
	c.push(hashFromIndex(5), 60, time.Now().Unix())
	if c.len() != 5 {
		t.Errorf("len() = %d, want 5", c.len())
	}
	if c.contains(hashFromIndex(0)) {
		t.Error("hash 0 should have been evicted")
	}
	if !c.contains(hashFromIndex(1)) || !c.contains(hashFromIndex(2)) || !c.contains(hashFromIndex(3)) || !c.contains(hashFromIndex(4)) || !c.contains(hashFromIndex(5)) {
		t.Error("hashes 1,2,3,4,5 should be present")
	}
	if oldest, ok := c.getOldest(); !ok || oldest.amount != 20 {
		t.Errorf("oldest.amount = %d, want 20", oldest.amount)
	}
	if newest, ok := c.getNewest(); !ok || newest.amount != 60 {
		t.Errorf("newest.amount = %d, want 60", newest.amount)
	}
	if newest, ok := c.getNewest(); !ok || newest.accumulatedAmount != (10+20+30+40+50) {
		t.Errorf("newest.accumulatedAmount = %d, want 150", newest.accumulatedAmount)
	}

	// overflow check
	c.push(hashFromIndex(6), math.MaxUint64-100, time.Now().Unix())
	c.push(hashFromIndex(7), 80, time.Now().Unix())
	newest, _ := c.getNewest()
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
	c.push(hashFromIndex(0), 10, time.Now().Unix())
	c.push(hashFromIndex(1), 20, time.Now().Unix())
	c.push(hashFromIndex(2), 30, time.Now().Unix())
	c.push(hashFromIndex(3), 40, time.Now().Unix())
	c.push(hashFromIndex(4), 50, time.Now().Unix())
	c.push(hashFromIndex(5), 60, time.Now().Unix())

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
		meta, _ := c.get(i)
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
	if meta, ok := c.get(6); ok {
		t.Errorf("after expand get(6): got %v, want nil", meta)
	}

	// push 2 more items, should evict the oldest
	c.push(hashFromIndex(6), 70, time.Now().Unix())
	c.push(hashFromIndex(7), 80, time.Now().Unix())
	if c.len() != 6 {
		t.Errorf("after more pushes: len = %d, want 6", c.len())
	}
	if c.contains(hashFromIndex(1)) {
		t.Error("hash 1 should have been evicted")
	}
	if !c.contains(hashFromIndex(2)) || !c.contains(hashFromIndex(3)) || !c.contains(hashFromIndex(4)) || !c.contains(hashFromIndex(5)) || !c.contains(hashFromIndex(6)) {
		t.Error("hashes 2,3,4,5,6 should be present")
	}
	if oldest, ok := c.getOldest(); !ok || oldest.amount != 30 {
		t.Errorf("oldest.amount = %d, want 30", oldest.amount)
	}
	if newest, ok := c.getNewest(); !ok || newest.amount != 80 {
		t.Errorf("newest.amount = %d, want 80", newest.amount)
	}
}

func TestComputeTotal(t *testing.T) {
	c := newCache(5)

	// Non-overflow case
	c.push(hashFromIndex(0), 10, time.Now().Unix())
	c.push(hashFromIndex(1), 20, time.Now().Unix())
	c.push(hashFromIndex(2), 30, time.Now().Unix())
	meta0, _ := c.get(0)
	if got := computeTotal(c, meta0, 5); got != 10+20+30+5 {
		t.Errorf("computeTotal(c, 0, 5) = %d, want %d", got, 10+20+30+5)
	}
	meta1, _ := c.get(1)
	if got := computeTotal(c, meta1, 5); got != 20+30+5 {
		t.Errorf("computeTotal(c, 1, 5) = %d, want %d", got, 20+30+5)
	}
	meta2, _ := c.get(2)
	if got := computeTotal(c, meta2, 5); got != 30+5 {
		t.Errorf("computeTotal(c, 2, 5) = %d, want %d", got, 30+5)
	}

	// Overflow case
	c.push(hashFromIndex(3), math.MaxUint64-100, time.Now().Unix())
	c.push(hashFromIndex(4), 50, time.Now().Unix())
	meta2, _ = c.get(2)
	if got := computeTotal(c, meta2, 5); got != 30+math.MaxUint64-100+50+5 {
		t.Errorf("computeTotal(c, 3, 5) = %d, want %d", got, uint64(math.MaxUint64-100+50+5))
	}
	meta3, _ := c.get(3)
	if got := computeTotal(c, meta3, 5); got != math.MaxUint64-100+50+5 {
		t.Errorf("computeTotal(c, 3, 5) = %d, want %d", got, uint64(math.MaxUint64-100+50+5))
	}
	meta4, _ := c.get(4)
	if got := computeTotal(c, meta4, 5); got != 50+5 {
		t.Errorf("computeTotal(c, 4, 5) = %d, want %d", got, 50+5)
	}
}

func TestCheckAmountThreshold(t *testing.T) {
	c := newCache(5)

	c.push(hashFromIndex(0), 10, time.Now().Unix())
	c.push(hashFromIndex(1), 20, time.Now().Unix())
	c.push(hashFromIndex(2), 30, time.Now().Unix())
	window := 1 * time.Second
	startTime := time.Now().Unix() - int64(window/time.Second)

	// Single tx exceeds threshold
	threshold := uint64(100)
	block, reason, index := checkAmountThreshold(c, threshold, startTime, threshold+1)
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
	block, reason, index = checkAmountThreshold(c, threshold, startTime, 40)
	if block {
		t.Errorf("expected no block when sum of all below threshold, reason: %s", reason)
	}
	if index != 0 {
		t.Errorf("expected index 0, got %d", index)
	}

	// Sum of all exceeds threshold
	block, reason, index = checkAmountThreshold(c, threshold, startTime, 41)
	if !block {
		t.Errorf("expected block when sum of all exceeds threshold, reason: %s", reason)
	}
	if index != 0 {
		t.Errorf("expected index 0, got %d", index)
	}

	makeElapsedItems(c, window)
	startTime = time.Now().Unix() - int64(window/time.Second)
	c.push(hashFromIndex(3), 40, time.Now().Unix())
	c.push(hashFromIndex(4), 50, time.Now().Unix())

	// not exceed thresholds
	threshold = uint64(30 + 40 + 50 + 1)
	block, reason, index = checkAmountThreshold(c, threshold, startTime, 1)
	if block {
		t.Errorf("expected no block, reason: %s", reason)
	}
	if index != 2 {
		t.Errorf("expected index 2, got %d", index)
	}
	block, reason, index = checkAmountThreshold(c, threshold, startTime, 30+1)
	if block {
		t.Errorf("expected no block, reason: %s", reason)
	}
	if index != 3 {
		t.Errorf("expected index 3, got %d", index)
	}

	// exceed thresholds
	block, _, index = checkAmountThreshold(c, threshold, startTime, 30+2)
	if !block {
		t.Error("expected block")
	}
	if index != 3 {
		t.Errorf("expected index 3, got %d", index)
	}
	block, _, index = checkAmountThreshold(c, threshold, startTime, 30+40+2)
	if !block {
		t.Error("expected block")
	}
	if index != 4 {
		t.Errorf("expected index 4, got %d", index)
	}

	c.push(hashFromIndex(5), 60, time.Now().Unix()) // evic the oldest item

	// not exceed thresholds
	threshold = uint64(40 + 50 + 60 + 1)
	block, reason, index = checkAmountThreshold(c, threshold, startTime, 1)
	if block {
		t.Errorf("expected no block, reason: %s", reason)
	}
	if index != 1 {
		t.Errorf("expected index 1, got %d", index)
	}

	// exceed thresholds
	block, _, index = checkAmountThreshold(c, threshold, startTime, 2)
	if !block {
		t.Error("expected block")
	}
	if index != 2 {
		t.Errorf("expected index 2, got %d", index)
	}

	makeElapsedItems(c, window)
	startTime = time.Now().Unix() - int64(window/time.Second)
	startTimeDoubleWindow := time.Now().Unix() - int64(window*2/time.Second)
	c.push(hashFromIndex(6), 70, time.Now().Unix())

	// not exceed thresholds
	block, reason, index = checkAmountThreshold(c, threshold, startTime, 1)
	if block {
		t.Errorf("expected no block, reason: %s", reason)
	}
	if index != 3 {
		t.Errorf("expected index 3, got %d", index)
	}
	block, reason, index = checkAmountThreshold(c, uint64(40+50+60+70+1), startTimeDoubleWindow, 1)
	if block {
		t.Errorf("expected no block, reason: %s", reason)
	}
	if index != 1 {
		t.Errorf("expected index 1, got %d", index)
	}

	// exceed thresholds
	block, _, index = checkAmountThreshold(c, uint64(70+1), startTime, 2)
	if !block {
		t.Error("expected block")
	}
	if index != 4 {
		t.Errorf("expected index 4, got %d", index)
	}
	block, _, index = checkAmountThreshold(c, uint64(50+60+70+1), startTimeDoubleWindow, 2)
	if !block {
		t.Error("expected block")
	}
	if index != 2 {
		t.Errorf("expected index 2, got %d", index)
	}
	block, _, index = checkAmountThreshold(c, uint64(50+60+70+1), startTimeDoubleWindow, 1)
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
	startTime := time.Now().Unix() - int64(window/time.Second)

	// Empty cache
	block, reason := checkCountThreshold(c, 1, startTime)
	if block {
		t.Errorf("expected no block for empty cache, reason: %s", reason)
	}
	if reason != "" {
		t.Errorf("expected empty reason, got %q", reason)
	}

	c.push(hashFromIndex(0), 10, time.Now().Unix())

	// Threshold not reached
	block, reason = checkCountThreshold(c, 2, startTime)
	if block {
		t.Errorf("expected no block when count is below threshold, reason: %s", reason)
	}
	if reason != "" {
		t.Errorf("expected empty reason, got %q", reason)
	}

	c.push(hashFromIndex(1), 20, time.Now().Unix())

	// Threshold reached within window
	block, reason = checkCountThreshold(c, 2, startTime)
	if !block {
		t.Errorf("expected block=true when threshold is reached, reason: %s", reason)
	}
	if reason != "over count threshold: 2" {
		t.Errorf("reason = %q, want %q", reason, "over count threshold: 2")
	}

	// Elapsed records should not match threshold
	makeElapsedItems(c, window)
	startTime = time.Now().Unix() - int64(window/time.Second)
	block, reason = checkCountThreshold(c, 2, startTime)
	if block {
		t.Errorf("expected no block for elapsed records, reason: %s", reason)
	}
	if reason != "" {
		t.Errorf("expected empty reason, got %q", reason)
	}

	// Mix elapsed and fresh records
	c.push(hashFromIndex(2), 30, time.Now().Unix())
	block, reason = checkCountThreshold(c, 2, startTime)
	if block {
		t.Errorf("expected no block with only one fresh record in window, reason: %s", reason)
	}
	c.push(hashFromIndex(3), 40, time.Now().Unix())
	block, reason = checkCountThreshold(c, 2, startTime)
	if !block {
		t.Errorf("expected block=true with two fresh records in window, reason: %s", reason)
	}
	if reason != "over count threshold: 2" {
		t.Errorf("reason = %q, want %q", reason, "over count threshold: 2")
	}
}

func TestCheckAmountThreshold_StressTest(t *testing.T) {
	var (
		cap        = uint(255)
		iterations = cap * 1000
		window     = 1 * time.Hour
		threadhold = uint64(cap)
	)
	c := newCache(cap)

	for i := uint(0); i < iterations; i++ {
		// amout between 1 and threadhold / 10
		amount := uint64(rand.Intn(int(threadhold/10)) + 1)
		c.push(hashFromIndex(uint64(i)), amount, time.Now().Unix())

		// Randomly make some items elapsed
		if rand.Intn(int(cap/2)) == 0 {
			makeElapsedItems(c, window)
		}

		// Check with both algorithms
		startTime := time.Now().Unix() - int64(window/time.Second)
		block, reason, index := checkAmountThreshold(c, threadhold, startTime, amount)
		expectedBlock, expectedReason, expectedIndex := checkLinearSearch(c, threadhold, startTime, amount)
		if block != expectedBlock {
			t.Errorf("expected block %t, got %t, expected reason %s, got reason %s, expected index %d, got index %d", expectedBlock, block, expectedReason, reason, expectedIndex, index)
		}
	}
}
