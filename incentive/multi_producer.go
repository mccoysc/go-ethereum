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

// MultiProducerRewardCalculator 多生产者奖励计算器
type MultiProducerRewardCalculator struct {
	config        *MultiProducerRewardConfig
	qualityScorer *BlockQualityScorer
}

// NewMultiProducerRewardCalculator 创建计算器
func NewMultiProducerRewardCalculator(config *MultiProducerRewardConfig, qualityScorer *BlockQualityScorer) *MultiProducerRewardCalculator {
	return &MultiProducerRewardCalculator{
		config:        config,
		qualityScorer: qualityScorer,
	}
}

// BlockCandidate 区块候选
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

// BlockQuality 区块质量详情
type BlockQuality struct {
	TxCount          uint64
	BlockSize        uint64
	GasUsed          uint64
	NewTxCount       uint64
	UniqueSenders    uint64
	RewardMultiplier float64
}

// CandidateReward 候选区块收益
type CandidateReward struct {
	Candidate       *BlockCandidate
	SpeedRatio      float64
	QualityMulti    float64
	FinalMultiplier float64
	Reward          *big.Int
}

// CalculateRewards 计算多生产者奖励分配
//
// 前三名收益分配机制：
//
//	第 1 名: 速度基础奖励 100% × 区块质量倍数
//	第 2 名: 速度基础奖励  60% × 区块质量倍数
//	第 3 名: 速度基础奖励  30% × 区块质量倍数
//
// 重要改进：只有包含新交易的候选区块才能获得收益
func (c *MultiProducerRewardCalculator) CalculateRewards(
	candidates []*BlockCandidate,
	totalFees *big.Int,
) []*CandidateReward {
	if len(candidates) == 0 {
		return nil
	}

	// 1. 按收到时间排序（确定速度排名）
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ReceivedAt.Before(candidates[j].ReceivedAt)
	})

	// 2. 计算每个候选的质量评分，并检查是否有新交易
	firstCandidateTxSet := make(map[common.Hash]bool)
	for _, tx := range candidates[0].Block.Transactions() {
		firstCandidateTxSet[tx.Hash()] = true
	}

	for i, candidate := range candidates {
		candidate.Rank = i + 1
		candidate.Quality = c.qualityScorer.CalculateQuality(candidate.Block)

		// 计算该候选区块包含的新交易数（第一名之外的交易）
		if i > 0 {
			newTxCount := 0
			for _, tx := range candidate.Block.Transactions() {
				if !firstCandidateTxSet[tx.Hash()] {
					newTxCount++
				}
			}
			candidate.Quality.NewTxCount = uint64(newTxCount)
		} else {
			// 第一名的所有交易都是"新"交易
			candidate.Quality.NewTxCount = candidate.Quality.TxCount
		}
	}

	// 3. 计算收益（只有包含新交易的候选才能获得收益）
	rewards := make([]*CandidateReward, 0, len(candidates))
	totalMultiplier := 0.0

	for i, candidate := range candidates {
		if i >= c.config.MaxCandidates {
			break
		}

		// 关键改进：如果后续候选没有新交易，不分配收益
		if i > 0 && candidate.Quality.NewTxCount == 0 {
			// 该候选的所有交易都已被第一名包含，不分配收益
			continue
		}

		speedRatio := c.config.SpeedRewardRatios[i]
		qualityMulti := candidate.Quality.RewardMultiplier

		// 对于后续候选，收益按新交易比例调整
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

	// 4. 按比例分配总交易费
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

// calculateScores 计算综合得分
func (c *MultiProducerRewardCalculator) calculateScores(candidates []*BlockCandidate) []uint64 {
	scores := make([]uint64, len(candidates))

	// 找出最早时间戳
	minTimestamp := candidates[0].Timestamp
	for _, cand := range candidates {
		if cand.Timestamp < minTimestamp {
			minTimestamp = cand.Timestamp
		}
	}

	for i, cand := range candidates {
		// 质量得分（0-100）
		qualityScore := cand.QualityScore

		// 时间得分（越早越高）
		timeDiff := cand.Timestamp - minTimestamp
		timeScore := uint64(100)
		if timeDiff > 0 {
			// 每延迟 100ms 扣 1 分
			penalty := timeDiff / 100
			if penalty > 100 {
				penalty = 100
			}
			timeScore = 100 - penalty
		}

		// 综合得分
		scores[i] = (qualityScore*c.config.QualityScoreWeight +
			timeScore*c.config.TimestampWeight) / 100
	}

	return scores
}
