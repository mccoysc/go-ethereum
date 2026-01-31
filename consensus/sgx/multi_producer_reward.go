package sgx

import (
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// MultiProducerRewardCalculator 多生产者收益计算器
type MultiProducerRewardCalculator struct {
	config        *Config
	qualityScorer *BlockQualityScorer
}

// NewMultiProducerRewardCalculator 创建多生产者收益计算器
func NewMultiProducerRewardCalculator(config *Config, qualityScorer *BlockQualityScorer) *MultiProducerRewardCalculator {
	return &MultiProducerRewardCalculator{
		config:        config,
		qualityScorer: qualityScorer,
	}
}

// CalculateRewards 计算所有候选区块的收益
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
			newTxCount := uint64(0)
			for _, tx := range candidate.Block.Transactions() {
				if !firstCandidateTxSet[tx.Hash()] {
					newTxCount++
				}
			}
			candidate.Quality.NewTxCount = newTxCount
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
			qualityMulti *= newTxRatio // 只有新交易部分才计入收益
		}

		finalMulti := speedRatio * qualityMulti

		rewards = append(rewards, &CandidateReward{
			Candidate:       candidate,
			SpeedRatio:      speedRatio,
			QualityMulti:    qualityMulti,
			FinalMultiplier: finalMulti,
			Reward:          new(big.Int),
		})

		totalMultiplier += finalMulti
	}

	// 4. 分配收益
	if totalMultiplier > 0 {
		for _, reward := range rewards {
			share := reward.FinalMultiplier / totalMultiplier
			rewardAmount := new(big.Int).Mul(totalFees, big.NewInt(int64(share*1e18)))
			rewardAmount.Div(rewardAmount, big.NewInt(1e18))
			reward.Reward = rewardAmount
		}
	}

	return rewards
}

// CalculateRewardsWithBaseReward 计算包含基础奖励的收益
func (c *MultiProducerRewardCalculator) CalculateRewardsWithBaseReward(
	candidates []*BlockCandidate,
	totalFees *big.Int,
	baseReward *big.Int,
) []*CandidateReward {
	// 基础奖励 + 交易费
	totalReward := new(big.Int).Add(totalFees, baseReward)

	// 计算收益分配
	rewards := c.CalculateRewards(candidates, totalReward)

	return rewards
}

// GetTopCandidate 获取排名第一的候选
func (c *MultiProducerRewardCalculator) GetTopCandidate(candidates []*BlockCandidate) *BlockCandidate {
	if len(candidates) == 0 {
		return nil
	}

	// 按收到时间排序
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ReceivedAt.Before(candidates[j].ReceivedAt)
	})

	return candidates[0]
}

// FilterQualifiedCandidates 过滤合格的候选区块
func (c *MultiProducerRewardCalculator) FilterQualifiedCandidates(
	candidates []*BlockCandidate,
	minQualityScore uint64,
) []*BlockCandidate {
	qualified := make([]*BlockCandidate, 0)

	for _, candidate := range candidates {
		if candidate.Quality == nil {
			candidate.Quality = c.qualityScorer.CalculateQuality(candidate.Block)
		}

		if candidate.Quality.TotalScore >= minQualityScore {
			qualified = append(qualified, candidate)
		}
	}

	return qualified
}

// EstimateReward 估算单个候选的收益
func (c *MultiProducerRewardCalculator) EstimateReward(
	candidate *BlockCandidate,
	totalFees *big.Int,
	rank int,
) *big.Int {
	if rank < 0 || rank >= len(c.config.SpeedRewardRatios) {
		return big.NewInt(0)
	}

	if candidate.Quality == nil {
		candidate.Quality = c.qualityScorer.CalculateQuality(candidate.Block)
	}

	speedRatio := c.config.SpeedRewardRatios[rank]
	qualityMulti := candidate.Quality.RewardMultiplier
	finalMulti := speedRatio * qualityMulti

	// 简化估算（假设只有这一个候选）
	rewardAmount := new(big.Int).Mul(totalFees, big.NewInt(int64(finalMulti*1e18)))
	rewardAmount.Div(rewardAmount, big.NewInt(1e18))

	return rewardAmount
}

// CollectCandidates 收集候选区块（模拟候选窗口）
func (c *MultiProducerRewardCalculator) CollectCandidates(
	firstBlock *types.Block,
	firstProducer common.Address,
	firstReceivedAt time.Time,
) []*BlockCandidate {
	candidates := make([]*BlockCandidate, 0)

	// 添加第一个候选
	candidates = append(candidates, &BlockCandidate{
		Block:      firstBlock,
		Producer:   firstProducer,
		ReceivedAt: firstReceivedAt,
		Rank:       1,
	})

	// 注意：实际实现中需要等待候选窗口结束后，收集所有候选区块
	// 这里仅提供接口示例

	return candidates
}

// ValidateRewardDistribution 验证收益分配是否正确
func (c *MultiProducerRewardCalculator) ValidateRewardDistribution(
	rewards []*CandidateReward,
	totalFees *big.Int,
) error {
	// 计算总分配收益
	totalDistributed := new(big.Int)
	for _, reward := range rewards {
		totalDistributed.Add(totalDistributed, reward.Reward)
	}

	// 验证总收益是否等于总交易费（允许小幅误差）
	diff := new(big.Int).Sub(totalFees, totalDistributed)
	diff.Abs(diff)

	// 允许 1% 的误差（由于浮点运算）
	maxDiff := new(big.Int).Div(totalFees, big.NewInt(100))
	if diff.Cmp(maxDiff) > 0 {
		return ErrInvalidReward
	}

	return nil
}
