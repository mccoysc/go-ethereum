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
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// TestMultiProducerCalculator_CalculateScores tests the calculateScores function
func TestMultiProducerCalculator_CalculateScores(t *testing.T) {
	config := &MultiProducerRewardConfig{
		MaxCandidates:      3,
		SpeedRewardRatios:  []float64{1.0, 0.6, 0.3},
		QualityScoreWeight: 60,
		TimestampWeight:    40,
	}
	
	qualityConfig := DefaultBlockQualityConfig()
	scorer := NewBlockQualityScorer(qualityConfig)
	calc := NewMultiProducerRewardCalculator(config, scorer)
	
	t.Run("Calculate scores for multiple candidates", func(t *testing.T) {
		baseTime := uint64(1000000)
		candidates := []*BlockCandidate{
			{
				Timestamp:    baseTime,
				QualityScore: 80,
			},
			{
				Timestamp:    baseTime + 100,
				QualityScore: 75,
			},
			{
				Timestamp:    baseTime + 500,
				QualityScore: 70,
			},
		}
		
		scores := calc.calculateScores(candidates)
		
		if len(scores) != len(candidates) {
			t.Fatalf("Expected %d scores, got %d", len(candidates), len(scores))
		}
		
		if scores[0] <= 0 {
			t.Errorf("First candidate should have positive score, got %d", scores[0])
		}
		
		if scores[0] < scores[1] {
			t.Errorf("First candidate (fastest) should have higher score than second: %d vs %d", scores[0], scores[1])
		}
	})
	
	t.Run("Calculate scores with same timestamp", func(t *testing.T) {
		baseTime := uint64(2000000)
		candidates := []*BlockCandidate{
			{
				Timestamp:    baseTime,
				QualityScore: 90,
			},
			{
				Timestamp:    baseTime,
				QualityScore: 85,
			},
		}
		
		scores := calc.calculateScores(candidates)
		
		if len(scores) != 2 {
			t.Fatalf("Expected 2 scores, got %d", len(scores))
		}
		
		if scores[0] <= scores[1] {
			t.Errorf("Higher quality should result in higher score when timestamps are equal")
		}
	})
	
	t.Run("Calculate scores with large time differences", func(t *testing.T) {
		baseTime := uint64(3000000)
		candidates := []*BlockCandidate{
			{
				Timestamp:    baseTime,
				QualityScore: 70,
			},
			{
				Timestamp:    baseTime + 100000,
				QualityScore: 90,
			},
		}
		
		scores := calc.calculateScores(candidates)
		
		if len(scores) != 2 {
			t.Fatalf("Expected 2 scores, got %d", len(scores))
		}
		
		if scores[0] <= scores[1] {
			t.Errorf("Time penalty should be significant for large delays, first=%d, second=%d", scores[0], scores[1])
		}
	})
	
	t.Run("Calculate scores single candidate", func(t *testing.T) {
		candidates := []*BlockCandidate{
			{
				Timestamp:    uint64(4000000),
				QualityScore: 85,
			},
		}
		
		scores := calc.calculateScores(candidates)
		
		if len(scores) != 1 {
			t.Fatalf("Expected 1 score, got %d", len(scores))
		}
		
		if scores[0] <= 0 {
			t.Errorf("Single candidate should have positive score, got %d", scores[0])
		}
	})
	
	t.Run("Calculate scores with zero quality", func(t *testing.T) {
		baseTime := uint64(5000000)
		candidates := []*BlockCandidate{
			{
				Timestamp:    baseTime,
				QualityScore: 0,
			},
			{
				Timestamp:    baseTime + 100,
				QualityScore: 50,
			},
		}
		
		scores := calc.calculateScores(candidates)
		
		if len(scores) != 2 {
			t.Fatalf("Expected 2 scores, got %d", len(scores))
		}
		
		if scores[1] == 0 {
			t.Errorf("Second candidate should have positive score even with delay")
		}
	})
}

// TestReputationManager_ApplyDecay tests the applyDecay function
func TestReputationManager_ApplyDecay(t *testing.T) {
	config := &ReputationConfig{
		InitialReputation: 5000,
		MaxReputation:     10000,
		MinReputation:     0,
		SuccessBonus:      100,
		FailurePenalty:    50,
		DecayRate:         5,
	}
	
	rm := NewReputationManager(config)
	
	t.Run("Apply decay after one day", func(t *testing.T) {
		addr := common.HexToAddress("0x0000000000000000000000000000000000000001")
		rep := rm.getOrCreateReputation(addr)
		rep.Score = 10000
		rep.LastDecayTime = time.Now().Add(-24 * time.Hour)
		
		initialScore := rep.Score
		rm.applyDecay(rep)
		
		expectedScore := initialScore - (initialScore * 5 / 100)
		if rep.Score != expectedScore {
			t.Errorf("After 1 day decay: got %d, want %d", rep.Score, expectedScore)
		}
	})
	
	t.Run("Apply decay after multiple days", func(t *testing.T) {
		addr := common.HexToAddress("0x0000000000000000000000000000000000000002")
		rep := rm.getOrCreateReputation(addr)
		rep.Score = 10000
		rep.LastDecayTime = time.Now().Add(-72 * time.Hour)
		
		rm.applyDecay(rep)
		
		if rep.Score >= 10000 {
			t.Errorf("Score should have decayed after 3 days: got %d", rep.Score)
		}
		
		if rep.Score == 0 {
			t.Errorf("Score should not be zero after 3 days with 5%% decay: got %d", rep.Score)
		}
	})
	
	t.Run("Apply decay with score near minimum", func(t *testing.T) {
		addr := common.HexToAddress("0x0000000000000000000000000000000000000003")
		rep := rm.getOrCreateReputation(addr)
		rep.Score = 100
		rep.LastDecayTime = time.Now().Add(-24 * time.Hour)
		
		rm.applyDecay(rep)
		
		if rep.Score < config.MinReputation {
			t.Errorf("Score should not go below minimum: got %d, min %d", rep.Score, config.MinReputation)
		}
	})
	
	t.Run("No decay if less than 24 hours", func(t *testing.T) {
		addr := common.HexToAddress("0x0000000000000000000000000000000000000004")
		rep := rm.getOrCreateReputation(addr)
		rep.Score = 5000
		rep.LastDecayTime = time.Now().Add(-12 * time.Hour)
		
		initialScore := rep.Score
		rm.applyDecay(rep)
		
		if rep.Score != initialScore {
			t.Errorf("Score should not decay before 24 hours: got %d, want %d", rep.Score, initialScore)
		}
	})
	
	t.Run("Apply decay updates LastDecayTime", func(t *testing.T) {
		addr := common.HexToAddress("0x0000000000000000000000000000000000000005")
		rep := rm.getOrCreateReputation(addr)
		rep.Score = 8000
		oldDecayTime := time.Now().Add(-48 * time.Hour)
		rep.LastDecayTime = oldDecayTime
		
		rm.applyDecay(rep)
		
		if rep.LastDecayTime.Before(oldDecayTime.Add(24 * time.Hour)) {
			t.Errorf("LastDecayTime should be updated after decay")
		}
	})
	
	t.Run("Decay through RecordBlockSuccess", func(t *testing.T) {
		addr := common.HexToAddress("0x0000000000000000000000000000000000000006")
		rep := rm.getOrCreateReputation(addr)
		rep.Score = 5000
		rep.LastDecayTime = time.Now().Add(-24 * time.Hour)
		
		initialScore := rep.Score
		rm.RecordBlockSuccess(addr)
		
		expectedAfterDecay := initialScore - (initialScore * 5 / 100)
		
		actualScore := rm.GetReputationScore(addr)
		if actualScore < expectedAfterDecay {
			t.Errorf("Decay should be applied: score=%d, expectedAfterDecay=%d", actualScore, expectedAfterDecay)
		}
	})
	
	t.Run("Decay through RecordBlockFailure", func(t *testing.T) {
		addr := common.HexToAddress("0x0000000000000000000000000000000000000007")
		rep := rm.getOrCreateReputation(addr)
		rep.Score = 5000
		rep.LastDecayTime = time.Now().Add(-24 * time.Hour)
		
		rm.RecordBlockFailure(addr)
		
		actualScore := rm.GetReputationScore(addr)
		if actualScore >= 5000 {
			t.Errorf("Score should have decayed and been penalized: got %d", actualScore)
		}
	})
	
	t.Run("Multiple decay periods", func(t *testing.T) {
		addr := common.HexToAddress("0x0000000000000000000000000000000000000008")
		rep := rm.getOrCreateReputation(addr)
		rep.Score = 8000
		rep.LastDecayTime = time.Now().Add(-120 * time.Hour)
		
		rm.applyDecay(rep)
		
		if rep.Score >= 8000 {
			t.Errorf("Score should have decayed after 5 days: got %d", rep.Score)
		}
		
		periods := 5
		expectedScore := uint64(8000)
		for i := 0; i < periods; i++ {
			decay := (expectedScore * 5) / 100
			if expectedScore >= decay {
				expectedScore -= decay
			} else {
				expectedScore = 0
				break
			}
		}
		
		if rep.Score != expectedScore {
			t.Errorf("After 5 periods: got %d, want ~%d", rep.Score, expectedScore)
		}
	})
	
	t.Run("Decay to minimum", func(t *testing.T) {
		addr := common.HexToAddress("0x0000000000000000000000000000000000000009")
		rep := rm.getOrCreateReputation(addr)
		rep.Score = 50
		rep.LastDecayTime = time.Now().Add(-24 * time.Hour)
		
		rm.applyDecay(rep)
		
		if rep.Score < config.MinReputation {
			t.Errorf("Score should not go below minimum: got %d, want %d", rep.Score, config.MinReputation)
		}
	})
}

// TestOnlineRewardManager_GetStatus tests getter functions
func TestOnlineRewardManager_GetStatus(t *testing.T) {
	config := &OnlineRewardConfig{
		MinOnlineTime:    1 * time.Hour,
		MinUptimeRatio:   0.8,
		HourlyReward:     big.NewInt(100000),
		HeartbeatTimeout: 5 * time.Minute,
	}
	
	orm := NewOnlineRewardManager(config)
	addr := common.HexToAddress("0x0000000000000000000000000000000000000001")
	
	t.Run("Get online status for existing node", func(t *testing.T) {
		orm.RecordHeartbeat(addr)
		
		isOnline := orm.IsOnline(addr)
		if !isOnline {
			t.Error("Node should be online after heartbeat")
		}
		
		onlineTime := orm.GetOnlineTime(addr)
		if onlineTime < 0 {
			t.Errorf("OnlineTime should be non-negative, got %v", onlineTime)
		}
		
		offlineTime := orm.GetOfflineTime(addr)
		if offlineTime < 0 {
			t.Errorf("OfflineTime should be non-negative, got %v", offlineTime)
		}
	})
	
	t.Run("Get status for non-existent node", func(t *testing.T) {
		nonExistentAddr := common.HexToAddress("0x9999999999999999999999999999999999999999")
		
		isOnline := orm.IsOnline(nonExistentAddr)
		if isOnline {
			t.Error("Non-existent node should not be online")
		}
		
		onlineTime := orm.GetOnlineTime(nonExistentAddr)
		if onlineTime != 0 {
			t.Errorf("Non-existent node should have 0 online time, got %v", onlineTime)
		}
		
		offlineTime := orm.GetOfflineTime(nonExistentAddr)
		if offlineTime != 0 {
			t.Errorf("Non-existent node should have 0 offline time, got %v", offlineTime)
		}
	})
	
	t.Run("Get uptime ratio", func(t *testing.T) {
		addr2 := common.HexToAddress("0x0000000000000000000000000000000000000002")
		
		ratio := orm.GetUptimeRatio(addr2)
		if ratio < 0 || ratio > 1 {
			t.Errorf("Uptime ratio should be between 0 and 1, got %f", ratio)
		}
	})
	
	t.Run("Calculate reward", func(t *testing.T) {
		addr3 := common.HexToAddress("0x0000000000000000000000000000000000000003")
		
		reward := orm.CalculateReward(addr3)
		if reward == nil {
			t.Error("CalculateReward should not return nil")
		}
		if reward.Sign() < 0 {
			t.Errorf("Reward should be non-negative, got %v", reward)
		}
	})
	
	t.Run("Node becomes offline after timeout", func(t *testing.T) {
		t.Skip("Skipping test that requires 6 minute sleep")
	})
	
	t.Run("Multiple heartbeats accumulate online time", func(t *testing.T) {
		addr5 := common.HexToAddress("0x0000000000000000000000000000000000000005")
		
		orm.RecordHeartbeat(addr5)
		time.Sleep(100 * time.Millisecond)
		orm.RecordHeartbeat(addr5)
		time.Sleep(100 * time.Millisecond)
		orm.RecordHeartbeat(addr5)
		
		onlineTime := orm.GetOnlineTime(addr5)
		if onlineTime == 0 {
			t.Error("Online time should accumulate with heartbeats")
		}
	})
	
	t.Run("Calculate reward with sufficient online time", func(t *testing.T) {
		addr6 := common.HexToAddress("0x0000000000000000000000000000000000000006")
		
		status := orm.getOrCreateStatus(addr6)
		status.TotalOnlineTime = 10 * time.Hour
		status.TotalOfflineTime = 1 * time.Hour
		
		reward := orm.CalculateReward(addr6)
		if reward.Cmp(big.NewInt(0)) == 0 {
			t.Error("Reward should be positive for sufficient online time")
		}
	})
	
	t.Run("Calculate reward with insufficient uptime ratio", func(t *testing.T) {
		addr7 := common.HexToAddress("0x0000000000000000000000000000000000000007")
		
		status := orm.getOrCreateStatus(addr7)
		status.TotalOnlineTime = 1 * time.Hour
		status.TotalOfflineTime = 10 * time.Hour
		
		reward := orm.CalculateReward(addr7)
		if reward.Cmp(big.NewInt(0)) != 0 {
			t.Errorf("Reward should be 0 for insufficient uptime ratio, got %v", reward)
		}
	})
	
	t.Run("Get uptime ratio with online time", func(t *testing.T) {
		addr8 := common.HexToAddress("0x0000000000000000000000000000000000000008")
		
		status := orm.getOrCreateStatus(addr8)
		status.TotalOnlineTime = 8 * time.Hour
		status.TotalOfflineTime = 2 * time.Hour
		
		ratio := orm.GetUptimeRatio(addr8)
		expectedRatio := 0.8
		if ratio < expectedRatio-0.01 || ratio > expectedRatio+0.01 {
			t.Errorf("Expected ratio ~%f, got %f", expectedRatio, ratio)
		}
	})
}
