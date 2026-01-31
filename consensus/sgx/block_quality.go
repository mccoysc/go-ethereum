package sgx

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// BlockQualityScorer 区块质量评分器
type BlockQualityScorer struct {
	config *QualityConfig
}

// NewBlockQualityScorer 创建区块质量评分器
func NewBlockQualityScorer(config *QualityConfig) *BlockQualityScorer {
	if config == nil {
		config = DefaultConfig().QualityConfig
	}
	return &BlockQualityScorer{
		config: config,
	}
}

// CalculateQuality 计算区块质量
func (s *BlockQualityScorer) CalculateQuality(block *types.Block) *BlockQuality {
	txs := block.Transactions()

	quality := &BlockQuality{
		TxCount:   uint64(len(txs)),
		BlockSize: uint64(block.Size()),
		GasUsed:   block.GasUsed(),
	}

	// 统计不同发送者
	senders := make(map[common.Address]bool)
	for _, tx := range txs {
		from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
		if err == nil {
			senders[from] = true
		}
	}
	quality.DiversityScore = uint64(len(senders))

	// 1. 交易数量得分
	quality.TxCountScore = s.calculateTxCountScore(quality.TxCount)

	// 2. 区块大小得分
	quality.BlockSizeScore = s.calculateBlockSizeScore(quality.BlockSize)

	// 3. Gas 利用率得分
	quality.GasUtilScore = s.calculateGasUtilScore(quality.GasUsed, block.GasLimit())

	// 4. 交易多样性得分
	quality.DiversityScoreNorm = s.calculateDiversityScoreNorm(quality.TxCount, quality.DiversityScore)

	// 计算综合得分
	quality.TotalScore = uint64(
		(float64(quality.TxCountScore)*s.config.TxCountWeight +
			float64(quality.BlockSizeScore)*s.config.BlockSizeWeight +
			float64(quality.GasUtilScore)*s.config.GasUtilizationWeight +
			float64(quality.DiversityScoreNorm)*s.config.TxDiversityWeight) / 100.0,
	)

	// 计算收益倍数
	quality.RewardMultiplier = s.calculateRewardMultiplier(quality)

	return quality
}

// calculateTxCountScore 计算交易数量得分
func (s *BlockQualityScorer) calculateTxCountScore(txCount uint64) uint64 {
	if txCount == 0 {
		return 0
	}

	minThreshold := uint64(s.config.MinTxThreshold)

	// 低于最小阈值，得分很低
	if txCount < minThreshold {
		// 线性递减: 1 笔交易 = 20%, minThreshold-1 笔交易 = 80%
		return txCount * 2000 / minThreshold
	}

	// 达到阈值后，对数增长（避免无限追求大区块）
	// 5 笔 = 8000, 10 笔 = 8500, 50 笔 = 9500, 100+ 笔 = 10000
	baseScore := uint64(8000)
	bonus := uint64(2000 * min64(txCount-minThreshold, 95) / 95)

	return baseScore + bonus
}

// calculateBlockSizeScore 计算区块大小得分
func (s *BlockQualityScorer) calculateBlockSizeScore(blockSize uint64) uint64 {
	if blockSize == 0 {
		return 0
	}

	targetSize := s.config.TargetBlockSize

	// 目标大小附近得分最高
	ratio := float64(blockSize) / float64(targetSize)

	if ratio <= 1.0 {
		// 未达到目标大小，线性增长
		return uint64(ratio * 10000)
	}

	// 超过目标大小，轻微惩罚（避免过大区块）
	penalty := (ratio - 1.0) * 1000
	if penalty > 2000 {
		penalty = 2000
	}
	return uint64(10000 - penalty)
}

// calculateGasUtilScore 计算 Gas 利用率得分
func (s *BlockQualityScorer) calculateGasUtilScore(gasUsed, gasLimit uint64) uint64 {
	if gasLimit == 0 {
		return 0
	}

	utilization := float64(gasUsed) / float64(gasLimit)
	target := s.config.TargetGasUtilization

	if utilization <= target {
		// 未达到目标利用率，线性增长
		return uint64(utilization / target * 10000)
	}

	// 超过目标利用率，满分
	return 10000
}

// calculateDiversityScoreNorm 计算交易多样性得分
func (s *BlockQualityScorer) calculateDiversityScoreNorm(txCount, uniqueSenders uint64) uint64 {
	if txCount == 0 {
		return 0
	}

	// 多样性 = 不同发送者数量 / 交易数量
	diversity := float64(uniqueSenders) / float64(txCount)

	// 多样性越高越好（避免单一用户刷交易）
	return uint64(diversity * 10000)
}

// calculateRewardMultiplier 计算收益倍数
func (s *BlockQualityScorer) calculateRewardMultiplier(quality *BlockQuality) float64 {
	// 基于综合得分计算收益倍数
	// 得分 0-2000: 倍数 0.1-0.5 (惩罚低质量区块)
	// 得分 2000-5000: 倍数 0.5-1.0 (正常区块)
	// 得分 5000-8000: 倍数 1.0-1.5 (高质量区块)
	// 得分 8000-10000: 倍数 1.5-2.0 (优质区块)

	score := float64(quality.TotalScore)

	if score < 2000 {
		return 0.1 + (score/2000)*0.4
	} else if score < 5000 {
		return 0.5 + ((score-2000)/3000)*0.5
	} else if score < 8000 {
		return 1.0 + ((score-5000)/3000)*0.5
	} else {
		return 1.5 + ((score-8000)/2000)*0.5
	}
}

// GetQualityTier 获取质量等级
func (s *BlockQualityScorer) GetQualityTier(quality *BlockQuality) string {
	score := quality.TotalScore
	if score >= 8000 {
		return "Excellent"
	} else if score >= 5000 {
		return "High"
	} else if score >= 2000 {
		return "Normal"
	} else {
		return "Low"
	}
}

// min64 返回两个 uint64 的最小值
func min64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

// max64 返回两个 uint64 的最大值
func max64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
