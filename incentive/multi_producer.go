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
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// MultiProducerRewardCalculator is the multi-producer reward calculator.
type MultiProducerRewardCalculator struct {
	config        *MultiProducerRewardConfig
	qualityScorer *BlockQualityScorer
}

// NewMultiProducerRewardCalculator creates a new calculator.
func NewMultiProducerRewardCalculator(config *MultiProducerRewardConfig, qualityScorer *BlockQualityScorer) *MultiProducerRewardCalculator {
	return &MultiProducerRewardCalculator{
		config:        config,
		qualityScorer: qualityScorer,
	}
}

// BlockCandidate represents a candidate block.
type BlockCandidate struct {
	Block        *types.Block
	Producer     common.Address
	BlockHash    common.Hash
	ReceivedAt   time.Time
	Timestamp    uint64
	TxCount      int
	GasUsed      uint64
	Quality      *BlockQuality
	QualityScore uint64
	Rank         int
}

// BlockQuality represents the block quality details.
type BlockQuality struct {
	TxCount          uint64
	BlockSize        uint64
	GasUsed          uint64
	NewTxCount       uint64
	UniqueSenders    uint64
	RewardMultiplier float64
}

// CandidateReward represents the reward for a candidate block.
type CandidateReward struct {
	Candidate       *BlockCandidate
	SpeedRatio      float64
	QualityMulti    float64
	FinalMultiplier float64
	Reward          *big.Int
}

// CalculateRewards calculates the multi-producer reward distribution.
//
// Top 3 reward distribution mechanism:
//
//	1st place: Speed base reward 100% × block quality multiplier
//	2nd place: Speed base reward  60% × block quality multiplier
//	3rd place: Speed base reward  30% × block quality multiplier
//
// Important improvement: Only candidate blocks containing new transactions can receive rewards.
func (c *MultiProducerRewardCalculator) CalculateRewards(
	candidates []*BlockCandidate,
	totalFees *big.Int,
) []*CandidateReward {
	if len(candidates) == 0 {
		return nil
	}

	// 1. Sort by received time (determine speed ranking)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ReceivedAt.Before(candidates[j].ReceivedAt)
	})

	// 2. Calculate quality score for each candidate and check for new transactions
	firstCandidateTxSet := make(map[common.Hash]bool)
	for _, tx := range candidates[0].Block.Transactions() {
		firstCandidateTxSet[tx.Hash()] = true
	}

	for i, candidate := range candidates {
		candidate.Rank = i + 1
		candidate.Quality = c.qualityScorer.CalculateQuality(candidate.Block)

		// Calculate the number of new transactions in this candidate block (transactions not in the first place)
		if i > 0 {
			newTxCount := 0
			for _, tx := range candidate.Block.Transactions() {
				if !firstCandidateTxSet[tx.Hash()] {
					newTxCount++
				}
			}
			candidate.Quality.NewTxCount = uint64(newTxCount)
		} else {
			// All transactions in the first place are considered "new"
			candidate.Quality.NewTxCount = candidate.Quality.TxCount
		}
	}

	// 3. Calculate rewards (only candidates with new transactions can receive rewards)
	rewards := make([]*CandidateReward, 0, len(candidates))
	totalMultiplier := 0.0

	for i, candidate := range candidates {
		if i >= c.config.MaxCandidates {
			break
		}

		// Key improvement: If a subsequent candidate has no new transactions, do not distribute rewards
		if i > 0 && candidate.Quality.NewTxCount == 0 {
			// All transactions in this candidate have already been included in the first place, no reward distribution
			continue
		}

		speedRatio := c.config.SpeedRewardRatios[i]
		qualityMulti := candidate.Quality.RewardMultiplier

		// For subsequent candidates, adjust rewards proportionally to new transactions
		if i > 0 && candidate.Quality.TxCount > 0 {
			newTxRatio := float64(candidate.Quality.NewTxCount) / float64(candidate.Quality.TxCount)
			qualityMulti *= newTxRatio
		}

		finalMulti := speedRatio * qualityMulti

		rewards = append(rewards, &CandidateReward{
			Candidate:       candidate,
			SpeedRatio:      speedRatio,
			QualityMulti:    qualityMulti,
			FinalMultiplier: finalMulti,
		})

		totalMultiplier += finalMulti
	}

	// 4. Distribute total transaction fees proportionally
	if totalMultiplier > 0 {
		for _, reward := range rewards {
			share := reward.FinalMultiplier / totalMultiplier
			reward.Reward = new(big.Int).Mul(
				totalFees,
				big.NewInt(int64(share*10000)),
			)
			reward.Reward.Div(reward.Reward, big.NewInt(10000))
		}
	}

	return rewards
}

// calculateScores calculates the comprehensive score.
func (c *MultiProducerRewardCalculator) calculateScores(candidates []*BlockCandidate) []uint64 {
	scores := make([]uint64, len(candidates))

	// Find the earliest timestamp
	minTimestamp := candidates[0].Timestamp
	for _, cand := range candidates {
		if cand.Timestamp < minTimestamp {
			minTimestamp = cand.Timestamp
		}
	}

	for i, cand := range candidates {
		// Quality score (0-100)
		qualityScore := cand.QualityScore

		// Time score (earlier is better)
		timeDiff := cand.Timestamp - minTimestamp
		timeScore := uint64(100)
		if timeDiff > 0 {
			// Deduct 1 point for every 100ms delay
			penalty := timeDiff / 100
			if penalty > 100 {
				penalty = 100
			}
			timeScore = 100 - penalty
		}

		// Comprehensive score
		scores[i] = (qualityScore*c.config.QualityScoreWeight +
			timeScore*c.config.TimestampWeight) / 100
	}

	return scores
}
