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

// BlockQualityScorer is the block quality scorer.
type BlockQualityScorer struct {
	config *BlockQualityConfig
}

// NewBlockQualityScorer creates a new scorer.
func NewBlockQualityScorer(config *BlockQualityConfig) *BlockQualityScorer {
	return &BlockQualityScorer{config: config}
}

// ScoreBlock evaluates the block quality.
//
// Block quality scoring considers multiple dimensions:
// 1. Transaction count: More transactions mean higher network utility
// 2. Block size: Blocks closer to the target size get higher scores
// 3. Gas utilization: Blocks closer to the target utilization (80%) get the highest score
// 4. Transaction diversity: Different types of transactions improve the score
//
// Return value range: 0-100
func (s *BlockQualityScorer) ScoreBlock(block *types.Block, gasLimit uint64) uint64 {
	// 1. Transaction count score
	txCountScore := s.scoreTxCount(len(block.Transactions()))

	// 2. Block size score
	blockSizeScore := s.scoreBlockSize(block.Size())

	// 3. Gas utilization score
	gasUtilScore := s.scoreGasUtilization(block.GasUsed(), gasLimit)

	// 4. Transaction diversity score
	diversityScore := s.scoreTxDiversity(block.Transactions())

	// Comprehensive score (weights sum to 100)
	totalScore := (txCountScore*uint64(s.config.TxCountWeight) +
		blockSizeScore*uint64(s.config.BlockSizeWeight) +
		gasUtilScore*uint64(s.config.GasUtilizationWeight) +
		diversityScore*uint64(s.config.TxDiversityWeight)) / 100

	return totalScore
}

// CalculateQuality calculates detailed block quality (for multi-producer rewards).
func (s *BlockQualityScorer) CalculateQuality(block *types.Block) *BlockQuality {
	gasLimit := block.GasLimit()
	qualityScore := s.ScoreBlock(block, gasLimit)

	// Count unique senders
	txs := block.Transactions()
	senders := make(map[common.Address]bool)
	for _, tx := range txs {
		from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
		if err != nil {
			continue
		}
		senders[from] = true
	}

	// Quality multiplier: Map quality score to reward multiplier
	// Quality score 100 -> multiplier 2.0
	// Quality score 50  -> multiplier 1.0
	// Quality score 0   -> multiplier 0.5
	multiplier := 0.5 + (float64(qualityScore) / 100.0 * 1.5)

	return &BlockQuality{
		TxCount:          uint64(len(txs)),
		BlockSize:        block.Size(),
		GasUsed:          block.GasUsed(),
		UniqueSenders:    uint64(len(senders)),
		RewardMultiplier: multiplier,
	}
}

// scoreTxCount scores the transaction count.
//
// Scoring strategy:
// - 0 transactions: 0 points
// - 1-10 transactions: Linear growth (10 points per transaction)
// - 11-100 transactions: Continue to grow but at a slower pace
// - 100+ transactions: Full score of 100
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
	// Between 10-100 transactions, gradually increase but at a slower pace
	return uint64(10 + (txCount - 10))
}

// scoreGasUtilization scores the gas utilization.
//
// Scoring strategy (with target utilization at 80% being optimal):
// - Utilization = target (80%): Full score of 100
// - Deviation from target: Deduct points based on deviation
// - Both too low and too high utilization reduce the score
func (s *BlockQualityScorer) scoreGasUtilization(gasUsed, gasLimit uint64) uint64 {
	if gasLimit == 0 {
		return 0
	}

	utilization := gasUsed * 100 / gasLimit
	target := uint64(s.config.TargetGasUtilization * 100)

	// Calculate deviation
	var deviation uint64
	if utilization >= target {
		deviation = utilization - target
	} else {
		deviation = target - utilization
	}

	// The greater the deviation, the more points are deducted
	// Within 20% deviation: Fewer points deducted
	// Over 20% deviation: Significant point deduction
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

// scoreBlockSize scores the block size.
//
// Scoring strategy (with target block size being optimal):
// - Block size = target (1MB): Full score of 100
// - Deviation from target: Deduct points based on deviation
// - Too small indicates insufficient transactions, too large may affect propagation
func (s *BlockQualityScorer) scoreBlockSize(blockSize uint64) uint64 {
	if blockSize == 0 {
		return 0
	}

	target := s.config.TargetBlockSize

	// Calculate ratio (percentage)
	var ratio uint64
	if blockSize >= target {
		ratio = blockSize * 100 / target
	} else {
		ratio = blockSize * 100 / target
	}

	// Optimal range: 70%-130% of target
	if ratio >= 70 && ratio <= 130 {
		// Within optimal range, get full score or close to full score
		if ratio >= 90 && ratio <= 110 {
			return 100
		}
		// Slight deviation, minor point deduction
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

	// Larger deviation
	if ratio < 70 {
		// Too small, deduct points proportionally
		return ratio * 100 / 70
	}

	// Too large, deduct more points
	deviation := ratio - 130
	score := uint64(100)
	if deviation > score {
		return 0
	}
	return score - deviation
}

// scoreTxDiversity scores the transaction diversity.
//
// Scoring strategy:
// - Single type of transaction: Base score
// - Multiple types of transactions: Additional points
// - Contains contract interactions: Additional points
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

	// Multiple transaction types
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

	// Contract diversity (maximum 20 points)
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
