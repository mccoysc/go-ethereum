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
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// BlockQualityScorer 区块质量评分器
type BlockQualityScorer struct {
	config *BlockQualityConfig
}

// NewBlockQualityScorer 创建评分器
func NewBlockQualityScorer(config *BlockQualityConfig) *BlockQualityScorer {
	return &BlockQualityScorer{config: config}
}

// ScoreBlock 评估区块质量
//
// 区块质量评分考虑多个维度：
// 1. 交易数量：更多交易意味着更高的网络效用
// 2. 区块大小：接近目标大小的区块得分更高
// 3. Gas 利用率：接近目标利用率（80%）得分最高
// 4. 交易多样性：不同类型的交易提高得分
//
// 返回值范围：0-100
func (s *BlockQualityScorer) ScoreBlock(block *types.Block, gasLimit uint64) uint64 {
	// 1. 交易数量得分
	txCountScore := s.scoreTxCount(len(block.Transactions()))

	// 2. 区块大小得分
	blockSizeScore := s.scoreBlockSize(block.Size())

	// 3. Gas 利用率得分
	gasUtilScore := s.scoreGasUtilization(block.GasUsed(), gasLimit)

	// 4. 交易多样性得分
	diversityScore := s.scoreTxDiversity(block.Transactions())

	// 综合得分（权重总和为 100）
	totalScore := (txCountScore*uint64(s.config.TxCountWeight) +
		blockSizeScore*uint64(s.config.BlockSizeWeight) +
		gasUtilScore*uint64(s.config.GasUtilizationWeight) +
		diversityScore*uint64(s.config.TxDiversityWeight)) / 100

	return totalScore
}

// CalculateQuality 计算区块质量详情（用于多生产者奖励）
func (s *BlockQualityScorer) CalculateQuality(block *types.Block) *BlockQuality {
	gasLimit := block.GasLimit()
	qualityScore := s.ScoreBlock(block, gasLimit)

	// 统计不同发送者
	txs := block.Transactions()
	senders := make(map[common.Address]bool)
	for _, tx := range txs {
		from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
		if err != nil {
			continue
		}
		senders[from] = true
	}

	// 质量倍数：质量分数映射到奖励倍数
	// 质量分数 100 -> 倍数 2.0
	// 质量分数 50  -> 倍数 1.0
	// 质量分数 0   -> 倍数 0.5
	multiplier := 0.5 + (float64(qualityScore) / 100.0 * 1.5)

	return &BlockQuality{
		TxCount:          uint64(len(txs)),
		BlockSize:        block.Size(),
		GasUsed:          block.GasUsed(),
		UniqueSenders:    uint64(len(senders)),
		RewardMultiplier: multiplier,
	}
}

// scoreTxCount 交易数量得分
//
// 评分策略：
// - 0 笔交易：0 分
// - 1-10 笔：线性增长（每笔 10 分）
// - 11-100 笔：继续增长但速度放缓
// - 100+ 笔：满分 100
func (s *BlockQualityScorer) scoreTxCount(txCount int) uint64 {
	if txCount == 0 {
		return 0
	}
	if txCount >= 100 {
		return 100
	}
	if txCount <= 10 {
		return uint64(txCount * 10)
	}
	// 10-100 笔之间，逐渐增长但速度放缓
	return uint64(10 + (txCount - 10))
}

// scoreGasUtilization Gas 利用率得分
//
// 评分策略（以目标利用率 80% 为最优）：
// - 利用率 = 目标（80%）：满分 100
// - 偏离目标：按偏离程度扣分
// - 过低或过高利用率都会降低得分
func (s *BlockQualityScorer) scoreGasUtilization(gasUsed, gasLimit uint64) uint64 {
	if gasLimit == 0 {
		return 0
	}

	utilization := gasUsed * 100 / gasLimit
	target := uint64(s.config.TargetGasUtilization * 100)

	// 计算偏离度
	var deviation uint64
	if utilization >= target {
		deviation = utilization - target
	} else {
		deviation = target - utilization
	}

	// 偏离越大，扣分越多
	// 偏离 20% 以内：扣分较少
	// 偏离超过 20%：大幅扣分
	if deviation <= 20 {
		score := uint64(100)
		penalty := deviation * 2
		if penalty > score {
			return 0
		}
		return score - penalty
	}
	score := uint64(100)
	penalty := 40 + (deviation-20)*3
	if penalty > score {
		return 0
	}
	return score - penalty
}

// scoreBlockSize 区块大小得分
//
// 评分策略（以目标区块大小为最优）：
// - 区块大小 = 目标（1MB）：满分 100
// - 偏离目标：按偏离程度扣分
// - 过小说明交易不足，过大可能影响传播
func (s *BlockQualityScorer) scoreBlockSize(blockSize uint64) uint64 {
	if blockSize == 0 {
		return 0
	}

	target := s.config.TargetBlockSize

	// 计算比例（百分比）
	var ratio uint64
	if blockSize >= target {
		ratio = blockSize * 100 / target
	} else {
		ratio = blockSize * 100 / target
	}

	// 最优范围：目标的 70%-130%
	if ratio >= 70 && ratio <= 130 {
		// 在最优范围内，得满分或接近满分
		if ratio >= 90 && ratio <= 110 {
			return 100
		}
		// 稍微偏离，轻微扣分
		var deviation uint64
		if ratio < 90 {
			deviation = 90 - ratio
		} else {
			deviation = ratio - 110
		}
		penalty := deviation / 2
		if penalty > 100 {
			return 0
		}
		return 100 - penalty
	}

	// 偏离较大
	if ratio < 70 {
		// 过小，按比例扣分
		return ratio * 100 / 70
	}

	// 过大，扣分更多
	deviation := ratio - 130
	score := uint64(100)
	if deviation > score {
		return 0
	}
	return score - deviation
}

// scoreTxDiversity 交易多样性得分
//
// 评分策略：
// - 单一类型交易：基础分
// - 多种类型交易：额外加分
// - 包含合约交互：额外加分
func (s *BlockQualityScorer) scoreTxDiversity(txs []*types.Transaction) uint64 {
	if len(txs) == 0 {
		return 0
	}

	hasTransfer := false
	hasContractCall := false
	hasContractCreation := false
	uniqueContracts := make(map[common.Address]bool)

	for _, tx := range txs {
		if tx.To() == nil {
			hasContractCreation = true
		} else if len(tx.Data()) > 0 {
			hasContractCall = true
			uniqueContracts[*tx.To()] = true
		} else {
			hasTransfer = true
		}
	}

	score := uint64(50)

	// 有多种交易类型
	typeCount := 0
	if hasTransfer {
		typeCount++
	}
	if hasContractCall {
		typeCount++
	}
	if hasContractCreation {
		typeCount++
	}

	score += uint64(typeCount * 15)

	// 合约多样性（最多加 20 分）
	contractDiversity := len(uniqueContracts)
	if contractDiversity > 10 {
		score += 20
	} else {
		score += uint64(contractDiversity * 2)
	}

	if score > 100 {
		score = 100
	}

	return score
}
