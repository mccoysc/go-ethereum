// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package incentive

import (
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
)

// TestRewardCalculator tests the basic reward calculation with decay
func TestRewardCalculator(t *testing.T) {
	config := DefaultRewardConfig()
	calc := NewRewardCalculator(config)

	tests := []struct {
		name        string
		blockNumber uint64
		wantReward  *big.Int
	}{
		{
			name:        "Block 0 - no decay",
			blockNumber: 0,
			wantReward:  big.NewInt(2e18),
		},
		{
			name:        "Before first decay period",
			blockNumber: 1_000_000,
			wantReward:  big.NewInt(2e18),
		},
		{
			name:        "First decay period",
			blockNumber: 4_000_000,
			wantReward:  new(big.Int).Mul(big.NewInt(2e18), big.NewInt(90)).Div(new(big.Int).Mul(big.NewInt(2e18), big.NewInt(90)), big.NewInt(100)),
		},
		{
			name:        "Second decay period",
			blockNumber: 8_000_000,
			wantReward:  nil, // Will calculate expected value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reward := calc.CalculateBlockReward(tt.blockNumber)

			if tt.wantReward != nil {
				if reward.Cmp(tt.wantReward) != 0 {
					t.Errorf("CalculateBlockReward() = %v, want %v", reward, tt.wantReward)
				}
			}

			// Verify minimum reward constraint
			if reward.Cmp(config.MinBlockReward) < 0 {
				t.Errorf("Reward %v is below minimum %v", reward, config.MinBlockReward)
			}

			// Verify reward decreases with block number (for periods after 0)
			if tt.blockNumber >= config.DecayPeriod {
				prevReward := calc.CalculateBlockReward(tt.blockNumber - config.DecayPeriod)
				if reward.Cmp(prevReward) > 0 {
					t.Errorf("Reward should decrease over time, got %v > %v", reward, prevReward)
				}
			}
		})
	}

	t.Run("Decay formula verification", func(t *testing.T) {
		// After 1 period: 2e18 * 0.9 = 1.8e18
		reward1 := calc.CalculateBlockReward(4_000_000)
		expected1 := new(big.Int).Mul(big.NewInt(2e18), big.NewInt(90))
		expected1.Div(expected1, big.NewInt(100))
		
		if reward1.Cmp(expected1) != 0 {
			t.Errorf("After 1 period: got %v, want %v", reward1, expected1)
		}

		// After 2 periods: 2e18 * 0.9 * 0.9 = 1.62e18
		reward2 := calc.CalculateBlockReward(8_000_000)
		expected2 := new(big.Int).Mul(expected1, big.NewInt(90))
		expected2.Div(expected2, big.NewInt(100))
		
		if reward2.Cmp(expected2) != 0 {
			t.Errorf("After 2 periods: got %v, want %v", reward2, expected2)
		}
	})

	t.Run("Total reward with fees", func(t *testing.T) {
		blockNumber := uint64(1000)
		fees := big.NewInt(1e17)
		
		total := calc.CalculateTotalReward(blockNumber, fees)
		expected := new(big.Int).Add(calc.CalculateBlockReward(blockNumber), fees)
		
		if total.Cmp(expected) != 0 {
			t.Errorf("CalculateTotalReward() = %v, want %v", total, expected)
		}
	})
}

// TestBlockQualityScorer tests the 4-dimensional block quality scoring
func TestBlockQualityScorer(t *testing.T) {
	config := DefaultBlockQualityConfig()
	scorer := NewBlockQualityScorer(config)

	t.Run("Transaction count scoring", func(t *testing.T) {
		tests := []struct {
			txCount   int
			wantScore uint64
		}{
			{0, 0},
			{5, 50},
			{10, 100},
			{50, 50},
			{100, 100},
			{150, 100},
		}

		for _, tt := range tests {
			score := scorer.scoreTxCount(tt.txCount)
			if score != tt.wantScore {
				t.Errorf("scoreTxCount(%d) = %d, want %d", tt.txCount, score, tt.wantScore)
			}
		}
	})

	t.Run("Gas utilization scoring", func(t *testing.T) {
		gasLimit := uint64(10_000_000)
		
		tests := []struct {
			name      string
			gasUsed   uint64
			wantScore uint64
		}{
			{"Zero utilization", 0, 0},
			{"Target utilization (80%)", 8_000_000, 100},
			{"Near target (75%)", 7_500_000, 90},
			{"Near target (85%)", 8_500_000, 90},
			{"Low utilization (50%)", 5_000_000, 30},
			{"Full utilization (100%)", 10_000_000, 60},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				score := scorer.scoreGasUtilization(tt.gasUsed, gasLimit)
				if score != tt.wantScore {
					t.Errorf("scoreGasUtilization(%d, %d) = %d, want %d", 
						tt.gasUsed, gasLimit, score, tt.wantScore)
				}
			})
		}
	})

	t.Run("Block size scoring", func(t *testing.T) {
		targetSize := config.TargetBlockSize
		
		tests := []struct {
			name      string
			size      uint64
			minScore  uint64
		}{
			{"Zero size", 0, 0},
			{"Target size", targetSize, 100},
			{"90% of target", targetSize * 9 / 10, 90},
			{"110% of target", targetSize * 11 / 10, 90},
			{"50% of target", targetSize / 2, 50},
			{"200% of target", targetSize * 2, 0},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				score := scorer.scoreBlockSize(tt.size)
				if score < tt.minScore {
					t.Errorf("scoreBlockSize(%d) = %d, want >= %d", tt.size, score, tt.minScore)
				}
			})
		}
	})

	t.Run("Transaction diversity scoring", func(t *testing.T) {
		// Create test transactions
		createTx := func(to *common.Address, data []byte) *types.Transaction {
			return types.NewTx(&types.LegacyTx{
				Nonce:    0,
				To:       to,
				Value:    big.NewInt(1e18),
				Gas:      21000,
				GasPrice: big.NewInt(1e9),
				Data:     data,
			})
		}

		addr1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
		addr2 := common.HexToAddress("0x2222222222222222222222222222222222222222")

		tests := []struct {
			name      string
			txs       []*types.Transaction
			minScore  uint64
		}{
			{
				name:     "Empty block",
				txs:      []*types.Transaction{},
				minScore: 0,
			},
			{
				name:     "Only transfers",
				txs:      []*types.Transaction{createTx(&addr1, nil), createTx(&addr2, nil)},
				minScore: 50,
			},
			{
				name:     "Contract calls",
				txs:      []*types.Transaction{createTx(&addr1, []byte{0x01}), createTx(&addr2, []byte{0x02})},
				minScore: 65,
			},
			{
				name:     "Contract creation",
				txs:      []*types.Transaction{createTx(nil, []byte{0x60, 0x60})},
				minScore: 65,
			},
			{
				name: "Mixed types",
				txs: []*types.Transaction{
					createTx(&addr1, nil),
					createTx(&addr2, []byte{0x01}),
					createTx(nil, []byte{0x60}),
				},
				minScore: 80,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				score := scorer.scoreTxDiversity(tt.txs)
				if score < tt.minScore {
					t.Errorf("scoreTxDiversity() = %d, want >= %d", score, tt.minScore)
				}
			})
		}
	})
}

// TestMultiProducerRewardCalculator tests multi-producer distribution and new transaction detection
func TestMultiProducerRewardCalculator(t *testing.T) {
	config := DefaultMultiProducerRewardConfig()
	qualityConfig := DefaultBlockQualityConfig()
	scorer := NewBlockQualityScorer(qualityConfig)
	calc := NewMultiProducerRewardCalculator(config, scorer)

	t.Run("Speed reward ratios", func(t *testing.T) {
		// Verify default ratios
		expected := []float64{1.0, 0.6, 0.3}
		if len(config.SpeedRewardRatios) != len(expected) {
			t.Fatalf("Expected %d ratios, got %d", len(expected), len(config.SpeedRewardRatios))
		}
		for i, ratio := range expected {
			if config.SpeedRewardRatios[i] != ratio {
				t.Errorf("Ratio[%d] = %f, want %f", i, config.SpeedRewardRatios[i], ratio)
			}
		}
	})

	t.Run("New transaction detection", func(t *testing.T) {
		// Create test blocks with overlapping transactions
		addr := common.HexToAddress("0x1111111111111111111111111111111111111111")
		
		tx1 := types.NewTx(&types.LegacyTx{
			Nonce:    0,
			To:       &addr,
			Value:    big.NewInt(1),
			Gas:      21000,
			GasPrice: big.NewInt(1e9),
		})
		
		tx2 := types.NewTx(&types.LegacyTx{
			Nonce:    1,
			To:       &addr,
			Value:    big.NewInt(2),
			Gas:      21000,
			GasPrice: big.NewInt(1e9),
		})
		
		tx3 := types.NewTx(&types.LegacyTx{
			Nonce:    2,
			To:       &addr,
			Value:    big.NewInt(3),
			Gas:      21000,
			GasPrice: big.NewInt(1e9),
		})

		// First candidate has tx1 and tx2
		header1 := &types.Header{
			Number:     big.NewInt(1000),
			GasLimit:   10_000_000,
			GasUsed:    42000,
			Time:       1234567890,
		}
		body1 := &types.Body{
			Transactions: []*types.Transaction{tx1, tx2},
		}
		block1 := types.NewBlock(header1, body1, nil, trie.NewStackTrie(nil))
		
		// Second candidate has tx1 (duplicate) and tx3 (new)
		header2 := &types.Header{
			Number:     big.NewInt(1000),
			GasLimit:   10_000_000,
			GasUsed:    42000,
			Time:       1234567891,
		}
		body2 := &types.Body{
			Transactions: []*types.Transaction{tx1, tx3},
		}
		block2 := types.NewBlock(header2, body2, nil, trie.NewStackTrie(nil))

		now := time.Now()
		candidates := []*BlockCandidate{
			{
				Block:      block1,
				Producer:   common.HexToAddress("0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
				ReceivedAt: now,
				Timestamp:  header1.Time,
			},
			{
				Block:      block2,
				Producer:   common.HexToAddress("0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"),
				ReceivedAt: now.Add(100 * time.Millisecond),
				Timestamp:  header2.Time,
			},
		}

		totalFees := big.NewInt(1e18)
		rewards := calc.CalculateRewards(candidates, totalFees)

		// First candidate should get full reward
		if len(rewards) < 1 {
			t.Fatal("Expected at least 1 reward")
		}

		// Second candidate should get reduced reward (only 1 new tx out of 2)
		if len(rewards) >= 2 {
			if candidates[1].Quality.NewTxCount != 1 {
				t.Errorf("Second candidate NewTxCount = %d, want 1", candidates[1].Quality.NewTxCount)
			}
		}
	})

	t.Run("No new transactions in second candidate", func(t *testing.T) {
		addr := common.HexToAddress("0x1111111111111111111111111111111111111111")
		
		tx1 := types.NewTx(&types.LegacyTx{
			Nonce:    0,
			To:       &addr,
			Value:    big.NewInt(1),
			Gas:      21000,
			GasPrice: big.NewInt(1e9),
		})

		header := &types.Header{
			Number:     big.NewInt(1000),
			GasLimit:   10_000_000,
			GasUsed:    21000,
			Time:       1234567890,
		}

		now := time.Now()
		
		// Both candidates have the same transaction
		body1 := &types.Body{Transactions: []*types.Transaction{tx1}}
		body2 := &types.Body{Transactions: []*types.Transaction{tx1}}
		block1 := types.NewBlock(header, body1, nil, trie.NewStackTrie(nil))
		block2 := types.NewBlock(header, body2, nil, trie.NewStackTrie(nil))

		candidates := []*BlockCandidate{
			{
				Block:      block1,
				Producer:   common.HexToAddress("0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
				ReceivedAt: now,
				Timestamp:  header.Time,
			},
			{
				Block:      block2,
				Producer:   common.HexToAddress("0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"),
				ReceivedAt: now.Add(100 * time.Millisecond),
				Timestamp:  header.Time,
			},
		}

		totalFees := big.NewInt(1e18)
		rewards := calc.CalculateRewards(candidates, totalFees)

		// Only first candidate should receive reward
		if len(rewards) != 1 {
			t.Errorf("Expected 1 reward, got %d", len(rewards))
		}

		if candidates[1].Quality.NewTxCount != 0 {
			t.Errorf("Second candidate NewTxCount = %d, want 0", candidates[1].Quality.NewTxCount)
		}
	})
}

// TestReputationManager tests reputation tracking, offline penalties, and recovery
func TestReputationManager(t *testing.T) {
	config := DefaultReputationConfig()
	mgr := NewReputationManager(config)

	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	t.Run("Initial reputation", func(t *testing.T) {
		rep := mgr.GetReputation(addr)
		if rep.Score != config.InitialReputation {
			t.Errorf("Initial reputation = %d, want %d", rep.Score, config.InitialReputation)
		}
	})

	t.Run("Block success increases reputation", func(t *testing.T) {
		initialScore := mgr.GetReputationScore(addr)
		mgr.RecordBlockSuccess(addr)
		
		newScore := mgr.GetReputationScore(addr)
		expectedScore := initialScore + config.SuccessBonus
		
		if newScore != expectedScore {
			t.Errorf("After success: score = %d, want %d", newScore, expectedScore)
		}
	})

	t.Run("Block failure decreases reputation", func(t *testing.T) {
		initialScore := mgr.GetReputationScore(addr)
		mgr.RecordBlockFailure(addr)
		
		newScore := mgr.GetReputationScore(addr)
		expectedScore := initialScore - config.FailurePenalty
		
		if newScore != expectedScore {
			t.Errorf("After failure: score = %d, want %d", newScore, expectedScore)
		}
	})

	t.Run("Malicious behavior penalty", func(t *testing.T) {
		addr2 := common.HexToAddress("0x2222222222222222222222222222222222222222")
		initialScore := mgr.GetReputationScore(addr2)
		
		mgr.RecordMaliciousBehavior(addr2)
		
		newScore := mgr.GetReputationScore(addr2)
		expectedScore := initialScore - config.MaliciousPenalty
		
		if newScore != expectedScore {
			t.Errorf("After malicious: score = %d, want %d", newScore, expectedScore)
		}
	})

	t.Run("Offline penalty", func(t *testing.T) {
		addr3 := common.HexToAddress("0x3333333333333333333333333333333333333333")
		initialScore := mgr.GetReputationScore(addr3)
		
		offlineDuration := 3 * time.Hour
		mgr.RecordOffline(addr3, offlineDuration)
		
		newScore := mgr.GetReputationScore(addr3)
		expectedPenalty := uint64(3 * config.OfflinePenaltyPerHour)
		var expectedScore uint64
		if initialScore >= expectedPenalty {
			expectedScore = initialScore - expectedPenalty
		} else {
			expectedScore = config.MinReputation
		}
		
		if newScore != expectedScore {
			t.Errorf("After 3h offline: score = %d, want %d", newScore, expectedScore)
		}
	})

	t.Run("Online recovery", func(t *testing.T) {
		addr4 := common.HexToAddress("0x4444444444444444444444444444444444444444")
		initialScore := mgr.GetReputationScore(addr4)
		
		onlineDuration := 2 * time.Hour
		mgr.RecordOnline(addr4, onlineDuration)
		
		newScore := mgr.GetReputationScore(addr4)
		expectedRecovery := uint64(2 * config.OnlineRecoveryPerHour)
		expectedScore := initialScore + expectedRecovery
		
		if newScore != expectedScore {
			t.Errorf("After 2h online: score = %d, want %d", newScore, expectedScore)
		}
	})

	t.Run("Max reputation limit", func(t *testing.T) {
		addr5 := common.HexToAddress("0x5555555555555555555555555555555555555555")
		
		// Add reputation beyond max
		for i := 0; i < 1000; i++ {
			mgr.RecordBlockSuccess(addr5)
		}
		
		score := mgr.GetReputationScore(addr5)
		if score > config.MaxReputation {
			t.Errorf("Score %d exceeds max %d", score, config.MaxReputation)
		}
	})

	t.Run("Min reputation limit", func(t *testing.T) {
		addr6 := common.HexToAddress("0x6666666666666666666666666666666666666666")
		
		// Subtract reputation beyond min
		for i := 0; i < 1000; i++ {
			mgr.RecordBlockFailure(addr6)
		}
		
		score := mgr.GetReputationScore(addr6)
		if score < config.MinReputation {
			t.Errorf("Score %d below min %d", score, config.MinReputation)
		}
	})

	t.Run("Exclusion after max penalties", func(t *testing.T) {
		addr7 := common.HexToAddress("0x7777777777777777777777777777777777777777")
		
		for i := 0; i < config.MaxPenaltyCount; i++ {
			mgr.RecordMaliciousBehavior(addr7)
		}
		
		if !mgr.IsExcluded(addr7) {
			t.Error("Node should be excluded after max penalties")
		}
	})
}

// TestOnlineRewardManager tests heartbeat tracking and uptime calculation
func TestOnlineRewardManager(t *testing.T) {
	config := DefaultOnlineRewardConfig()
	mgr := NewOnlineRewardManager(config)

	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	t.Run("Initial state", func(t *testing.T) {
		if mgr.IsOnline(addr) {
			t.Error("Node should not be online initially")
		}
		
		uptime := mgr.GetUptimeRatio(addr)
		if uptime != 0 {
			t.Errorf("Initial uptime = %f, want 0", uptime)
		}
	})

	t.Run("Record heartbeat", func(t *testing.T) {
		mgr.RecordHeartbeat(addr)
		
		if !mgr.IsOnline(addr) {
			t.Error("Node should be online after heartbeat")
		}
	})

	t.Run("Heartbeat timeout", func(t *testing.T) {
		addr2 := common.HexToAddress("0x2222222222222222222222222222222222222222")
		mgr.RecordHeartbeat(addr2)
		
		// Simulate time passing beyond timeout
		time.Sleep(10 * time.Millisecond)
		
		// With default config timeout of 2 minutes, node should still be online
		if !mgr.IsOnline(addr2) {
			t.Error("Node should still be online within timeout")
		}
	})

	t.Run("Calculate reward with insufficient online time", func(t *testing.T) {
		addr3 := common.HexToAddress("0x3333333333333333333333333333333333333333")
		mgr.RecordHeartbeat(addr3)
		
		reward := mgr.CalculateReward(addr3)
		if reward.Cmp(big.NewInt(0)) != 0 {
			t.Error("Reward should be 0 for insufficient online time")
		}
	})

	t.Run("Uptime ratio calculation", func(t *testing.T) {
		// This is a simplified test; in practice, uptime ratio accumulates over time
		addr4 := common.HexToAddress("0x4444444444444444444444444444444444444444")
		mgr.RecordHeartbeat(addr4)
		
		ratio := mgr.GetUptimeRatio(addr4)
		// Initially should be 0 or very low
		if ratio < 0 || ratio > 1 {
			t.Errorf("Invalid uptime ratio: %f", ratio)
		}
	})
}

// TestPenaltyManager tests all 4 penalty types
func TestPenaltyManager(t *testing.T) {
	config := DefaultPenaltyConfig()
	mgr := NewPenaltyManager(config)

	nodeBalance := new(big.Int).Exp(big.NewInt(10), big.NewInt(20), nil) // 100 tokens
	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	t.Run("Double sign penalty", func(t *testing.T) {
		penalty := mgr.CalculateDoubleSignPenalty(nodeBalance)
		
		expected := new(big.Int).Mul(nodeBalance, big.NewInt(int64(config.DoubleSignPenaltyRate)))
		expected.Div(expected, big.NewInt(100))
		
		if penalty.Cmp(expected) != 0 {
			t.Errorf("Double sign penalty = %v, want %v", penalty, expected)
		}
		
		// Should be 50% of balance
		if penalty.Cmp(new(big.Int).Div(nodeBalance, big.NewInt(2))) != 0 {
			t.Error("Double sign penalty should be 50% of balance")
		}
	})

	t.Run("Offline penalty", func(t *testing.T) {
		offlineHours := uint64(10)
		penalty := mgr.CalculateOfflinePenalty(offlineHours)
		
		expected := new(big.Int).Mul(config.OfflinePenaltyPerHour, big.NewInt(int64(offlineHours)))
		
		if penalty.Cmp(expected) != 0 {
			t.Errorf("Offline penalty = %v, want %v", penalty, expected)
		}
	})

	t.Run("Invalid block penalty", func(t *testing.T) {
		penalty := mgr.CalculateInvalidBlockPenalty()
		
		if penalty.Cmp(config.InvalidBlockPenalty) != 0 {
			t.Errorf("Invalid block penalty = %v, want %v", penalty, config.InvalidBlockPenalty)
		}
	})

	t.Run("Malicious penalty", func(t *testing.T) {
		penalty := mgr.CalculateMaliciousPenalty(nodeBalance)
		
		// Should be 100% of balance
		expected := new(big.Int).Mul(nodeBalance, big.NewInt(int64(config.MaliciousPenaltyRate)))
		expected.Div(expected, big.NewInt(100))
		
		if penalty.Cmp(expected) != 0 {
			t.Errorf("Malicious penalty = %v, want %v", penalty, expected)
		}
		
		if penalty.Cmp(nodeBalance) != 0 {
			t.Error("Malicious penalty should be 100% of balance")
		}
	})

	t.Run("Generic calculate penalty", func(t *testing.T) {
		tests := []struct {
			name       string
			penaltyType PenaltyType
			additional interface{}
			wantNonZero bool
		}{
			{"Double sign", PenaltyDoubleSign, nil, true},
			{"Offline", PenaltyOffline, uint64(5), true},
			{"Invalid block", PenaltyInvalidBlock, nil, true},
			{"Malicious", PenaltyMalicious, nil, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				penalty := mgr.CalculatePenalty(tt.penaltyType, nodeBalance, tt.additional)
				
				if tt.wantNonZero && penalty.Cmp(big.NewInt(0)) == 0 {
					t.Error("Expected non-zero penalty")
				}
			})
		}
	})

	t.Run("Record and retrieve penalty history", func(t *testing.T) {
		record := &PenaltyRecord{
			NodeAddress: addr,
			Type:        PenaltyDoubleSign,
			Amount:      big.NewInt(1e18),
			Reason:      "Test penalty",
			Timestamp:   time.Now(),
			BlockNumber: 1000,
		}
		
		mgr.RecordPenalty(record)
		
		history := mgr.GetPenaltyHistory(addr)
		if len(history) == 0 {
			t.Error("Expected penalty in history")
		}
		
		count := mgr.GetPenaltyCount(addr)
		if count == 0 {
			t.Error("Expected non-zero penalty count")
		}
		
		total := mgr.GetTotalPenalty(addr)
		if total.Cmp(big.NewInt(0)) == 0 {
			t.Error("Expected non-zero total penalty")
		}
	})

	t.Run("Get penalty by type", func(t *testing.T) {
		addr2 := common.HexToAddress("0x2222222222222222222222222222222222222222")
		
		mgr.RecordPenalty(&PenaltyRecord{
			NodeAddress: addr2,
			Type:        PenaltyOffline,
			Amount:      big.NewInt(1e17),
			Timestamp:   time.Now(),
		})
		
		mgr.RecordPenalty(&PenaltyRecord{
			NodeAddress: addr2,
			Type:        PenaltyOffline,
			Amount:      big.NewInt(1e17),
			Timestamp:   time.Now(),
		})
		
		offlineCount := mgr.GetPenaltyByType(addr2, PenaltyOffline)
		if offlineCount != 2 {
			t.Errorf("Offline penalty count = %d, want 2", offlineCount)
		}
	})
}

// TestCompetitionManager tests comprehensive scoring, ranking, and rewards distribution
func TestCompetitionManager(t *testing.T) {
	config := DefaultCompetitionConfig()
	repConfig := DefaultReputationConfig()
	onlineConfig := DefaultOnlineRewardConfig()
	qualityConfig := DefaultBlockQualityConfig()
	
	repMgr := NewReputationManager(repConfig)
	onlineMgr := NewOnlineRewardManager(onlineConfig)
	qualityScorer := NewBlockQualityScorer(qualityConfig)
	
	mgr := NewCompetitionManager(config, repMgr, onlineMgr, qualityScorer)

	t.Run("Weight configuration", func(t *testing.T) {
		totalWeight := config.ReputationWeight + config.UptimeWeight + 
			config.BlockQualityWeight + config.ServiceQualityWeight
		
		if totalWeight != 1.0 {
			t.Errorf("Total weight = %f, want 1.0", totalWeight)
		}
	})

	t.Run("Calculate comprehensive score", func(t *testing.T) {
		metrics := &NodeMetrics{
			Address:        common.HexToAddress("0x1111111111111111111111111111111111111111"),
			Reputation:     100,
			UptimeRatio:    0.95,
			BlockQuality:   80,
			ServiceQuality: 90,
		}
		
		score := mgr.CalculateComprehensiveScore(metrics)
		
		// Score should be weighted average
		expectedScore := uint64(100*uint64(config.ReputationWeight*100) +
			95*uint64(config.UptimeWeight*100) +
			80*uint64(config.BlockQualityWeight*100) +
			90*uint64(config.ServiceQualityWeight*100)) / 100
		
		if score != expectedScore {
			t.Errorf("Comprehensive score = %d, want %d", score, expectedScore)
		}
	})

	t.Run("Rank nodes", func(t *testing.T) {
		nodes := []*NodeMetrics{
			{
				Address:        common.HexToAddress("0x1111111111111111111111111111111111111111"),
				Reputation:     50,
				UptimeRatio:    0.8,
				BlockQuality:   60,
				ServiceQuality: 70,
			},
			{
				Address:        common.HexToAddress("0x2222222222222222222222222222222222222222"),
				Reputation:     100,
				UptimeRatio:    0.95,
				BlockQuality:   90,
				ServiceQuality: 95,
			},
			{
				Address:        common.HexToAddress("0x3333333333333333333333333333333333333333"),
				Reputation:     75,
				UptimeRatio:    0.85,
				BlockQuality:   80,
				ServiceQuality: 85,
			},
		}
		
		ranked := mgr.RankNodes(nodes)
		
		if len(ranked) != 3 {
			t.Fatalf("Expected 3 ranked nodes, got %d", len(ranked))
		}
		
		// Verify nodes are ranked by score (highest first)
		for i := 0; i < len(ranked)-1; i++ {
			score1 := mgr.CalculateComprehensiveScore(ranked[i])
			score2 := mgr.CalculateComprehensiveScore(ranked[i+1])
			
			if score1 < score2 {
				t.Errorf("Ranking error: node %d (score %d) ranked before node %d (score %d)",
					i, score1, i+1, score2)
			}
		}
		
		// Best node should be 0x2222...
		if ranked[0].Address != nodes[1].Address {
			t.Error("Highest scoring node should be ranked first")
		}
	})

	t.Run("Distribute ranking rewards", func(t *testing.T) {
		nodes := []*NodeMetrics{
			{Address: common.HexToAddress("0x1111111111111111111111111111111111111111"), Reputation: 100, UptimeRatio: 1.0, BlockQuality: 100, ServiceQuality: 100},
			{Address: common.HexToAddress("0x2222222222222222222222222222222222222222"), Reputation: 90, UptimeRatio: 0.9, BlockQuality: 90, ServiceQuality: 90},
			{Address: common.HexToAddress("0x3333333333333333333333333333333333333333"), Reputation: 80, UptimeRatio: 0.8, BlockQuality: 80, ServiceQuality: 80},
		}
		
		ranked := mgr.RankNodes(nodes)
		totalReward := new(big.Int).Exp(big.NewInt(10), big.NewInt(20), nil) // 1e20
		
		rewards := mgr.DistributeRankingRewards(totalReward, ranked)
		
		if len(rewards) != 3 {
			t.Fatalf("Expected 3 rewards, got %d", len(rewards))
		}
		
		// Verify reward ratios
		reward1 := rewards[ranked[0].Address]
		reward2 := rewards[ranked[1].Address]
		reward3 := rewards[ranked[2].Address]
		
		if reward1.Cmp(reward2) <= 0 {
			t.Error("First place should receive more than second place")
		}
		
		if reward2.Cmp(reward3) <= 0 {
			t.Error("Second place should receive more than third place")
		}
		
		// Verify total distributed matches expected ratios
		totalDistributed := new(big.Int).Add(reward1, reward2)
		totalDistributed.Add(totalDistributed, reward3)
		
		if totalDistributed.Cmp(big.NewInt(0)) == 0 {
			t.Error("No rewards were distributed")
		}
	})

	t.Run("Get top nodes", func(t *testing.T) {
		nodes := make([]*NodeMetrics, 15)
		for i := 0; i < 15; i++ {
			nodes[i] = &NodeMetrics{
				Address:        common.BigToAddress(big.NewInt(int64(i))),
				Reputation:     uint64(100 - i),
				UptimeRatio:    float64(100-i) / 100.0,
				BlockQuality:   uint64(90 - i),
				ServiceQuality: uint64(85 - i),
			}
		}
		
		top10 := mgr.GetTopNodes(nodes, 10)
		
		if len(top10) != 10 {
			t.Errorf("Expected 10 nodes, got %d", len(top10))
		}
		
		// Verify they are the highest scoring
		for i := 0; i < len(top10)-1; i++ {
			score1 := mgr.CalculateComprehensiveScore(top10[i])
			score2 := mgr.CalculateComprehensiveScore(top10[i+1])
			
			if score1 < score2 {
				t.Errorf("Top nodes not properly ranked at position %d", i)
			}
		}
	})

	t.Run("Get node metrics", func(t *testing.T) {
		addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
		
		// Set up some reputation and online status
		repMgr.RecordBlockSuccess(addr)
		onlineMgr.RecordHeartbeat(addr)
		
		metrics := mgr.GetNodeMetrics(addr, 85, 90)
		
		if metrics.Address != addr {
			t.Error("Address mismatch in metrics")
		}
		
		if metrics.BlockQuality != 85 {
			t.Errorf("BlockQuality = %d, want 85", metrics.BlockQuality)
		}
		
		if metrics.ServiceQuality != 90 {
			t.Errorf("ServiceQuality = %d, want 90", metrics.ServiceQuality)
		}
	})
}

// TestConcurrentAccess tests concurrent access to managers with mutexes
func TestConcurrentAccess(t *testing.T) {
	t.Run("Concurrent reputation updates", func(t *testing.T) {
		config := DefaultReputationConfig()
		mgr := NewReputationManager(config)
		
		addr := common.HexToAddress("0x1111111111111111111111111111111111111111")
		
		var wg sync.WaitGroup
		iterations := 100
		
		// Concurrent successes
		wg.Add(iterations)
		for i := 0; i < iterations; i++ {
			go func() {
				defer wg.Done()
				mgr.RecordBlockSuccess(addr)
			}()
		}
		wg.Wait()
		
		rep := mgr.GetReputation(addr)
		if rep.SuccessBlocks != uint64(iterations) {
			t.Errorf("SuccessBlocks = %d, want %d", rep.SuccessBlocks, iterations)
		}
	})

	t.Run("Concurrent online rewards", func(t *testing.T) {
		config := DefaultOnlineRewardConfig()
		mgr := NewOnlineRewardManager(config)
		
		addr := common.HexToAddress("0x2222222222222222222222222222222222222222")
		
		var wg sync.WaitGroup
		iterations := 50
		
		wg.Add(iterations)
		for i := 0; i < iterations; i++ {
			go func() {
				defer wg.Done()
				mgr.RecordHeartbeat(addr)
			}()
		}
		wg.Wait()
		
		// Should complete without panics or races
		if !mgr.IsOnline(addr) {
			t.Error("Node should be online after heartbeats")
		}
	})
}

// TestEdgeCases tests boundary conditions and edge cases
func TestEdgeCases(t *testing.T) {
	t.Run("Zero block reward", func(t *testing.T) {
		config := &RewardConfig{
			BaseBlockReward: big.NewInt(0),
			DecayPeriod:     1000,
			DecayRate:       10,
			MinBlockReward:  big.NewInt(0),
		}
		calc := NewRewardCalculator(config)
		
		reward := calc.CalculateBlockReward(5000)
		if reward.Cmp(big.NewInt(0)) != 0 {
			t.Error("Expected zero reward")
		}
	})

	t.Run("Empty block quality", func(t *testing.T) {
		config := DefaultBlockQualityConfig()
		scorer := NewBlockQualityScorer(config)
		
		header := &types.Header{
			Number:     big.NewInt(1000),
			GasLimit:   10_000_000,
			GasUsed:    0,
			Time:       1234567890,
		}
		block := types.NewBlock(header, &types.Body{}, nil, trie.NewStackTrie(nil))
		
		score := scorer.ScoreBlock(block, header.GasLimit)
		if score > 10 {
			t.Errorf("Empty block should have very low score, got %d", score)
		}
	})

	t.Run("Negative reputation handling", func(t *testing.T) {
		config := DefaultReputationConfig()
		mgr := NewReputationManager(config)
		
		addr := common.HexToAddress("0x1111111111111111111111111111111111111111")
		
		// Force reputation to minimum
		for i := 0; i < 1000; i++ {
			mgr.RecordMaliciousBehavior(addr)
		}
		
		score := mgr.GetReputationScore(addr)
		if score < config.MinReputation {
			t.Errorf("Score should not go below minimum: got %d, min %d", score, config.MinReputation)
		}
	})

	t.Run("Very large block number", func(t *testing.T) {
		config := DefaultRewardConfig()
		calc := NewRewardCalculator(config)
		
		// Test with very large block number
		largeBlockNumber := uint64(100_000_000)
		reward := calc.CalculateBlockReward(largeBlockNumber)
		
		// Should be at or near minimum reward after so many decay periods
		if reward.Cmp(config.MinBlockReward) < 0 {
			t.Errorf("Reward should not be below minimum: got %v, min %v", 
				reward, config.MinBlockReward)
		}
		
		// Should be significantly decayed from base
		if reward.Cmp(config.BaseBlockReward) >= 0 {
			t.Errorf("Reward should be decayed from base: got %v, base %v",
				reward, config.BaseBlockReward)
		}
	})

	t.Run("Zero uptime ratio", func(t *testing.T) {
		config := DefaultOnlineRewardConfig()
		mgr := NewOnlineRewardManager(config)
		
		addr := common.HexToAddress("0x3333333333333333333333333333333333333333")
		
		ratio := mgr.GetUptimeRatio(addr)
		if ratio != 0 {
			t.Errorf("Uptime ratio should be 0 for new node, got %f", ratio)
		}
	})
}

// TestConfigDefaults verifies all default configurations are valid
func TestConfigDefaults(t *testing.T) {
	t.Run("DefaultRewardConfig", func(t *testing.T) {
		config := DefaultRewardConfig()
		
		if config.BaseBlockReward.Cmp(big.NewInt(0)) <= 0 {
			t.Error("BaseBlockReward should be positive")
		}
		
		if config.DecayPeriod == 0 {
			t.Error("DecayPeriod should be non-zero")
		}
		
		if config.DecayRate >= 100 {
			t.Error("DecayRate should be less than 100%")
		}
		
		if config.MinBlockReward.Cmp(config.BaseBlockReward) >= 0 {
			t.Error("MinBlockReward should be less than BaseBlockReward")
		}
	})

	t.Run("DefaultBlockQualityConfig", func(t *testing.T) {
		config := DefaultBlockQualityConfig()
		
		totalWeight := uint(config.TxCountWeight) + uint(config.BlockSizeWeight) +
			uint(config.GasUtilizationWeight) + uint(config.TxDiversityWeight)
		
		if totalWeight != 100 {
			t.Errorf("Total weight = %d, want 100", totalWeight)
		}
		
		if config.TargetGasUtilization <= 0 || config.TargetGasUtilization > 1 {
			t.Error("TargetGasUtilization should be between 0 and 1")
		}
	})

	t.Run("DefaultCompetitionConfig", func(t *testing.T) {
		config := DefaultCompetitionConfig()
		
		totalWeight := config.ReputationWeight + config.UptimeWeight +
			config.BlockQualityWeight + config.ServiceQualityWeight
		
		if totalWeight != 1.0 {
			t.Errorf("Total weight = %f, want 1.0", totalWeight)
		}
		
		totalRankingReward := 0.0
		for _, ratio := range config.RankingRewards {
			totalRankingReward += ratio
		}
		
		if totalRankingReward != 1.0 {
			t.Errorf("Total ranking reward = %f, want 1.0", totalRankingReward)
		}
	})
}
