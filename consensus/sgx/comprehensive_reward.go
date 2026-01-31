package sgx

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// ComprehensiveRewardCalculator 综合收益计算器
type ComprehensiveRewardCalculator struct {
	config                        *RewardConfig
	onlineRewardCalc              *OnlineRewardCalculator
	blockQualityScorer            *BlockQualityScorer
	serviceQualityScorer          *ServiceQualityScorer
	transactionVolumeTracker      *TransactionVolumeTracker
	historicalContributionTracker *HistoricalContributionTracker
}

// NewComprehensiveRewardCalculator 创建综合收益计算器
func NewComprehensiveRewardCalculator(config *RewardConfig) *ComprehensiveRewardCalculator {
	return &ComprehensiveRewardCalculator{
		config:                        config,
		onlineRewardCalc:              NewOnlineRewardCalculator(config),
		historicalContributionTracker: NewHistoricalContributionTracker(),
	}
}

// CalculateComprehensiveReward 计算综合奖励
func (crc *ComprehensiveRewardCalculator) CalculateComprehensiveReward(
	address common.Address,
	blockReward *big.Int,
	uptimeScore uint64,
	qualityScore uint64,
	serviceScore uint64,
) *ComprehensiveReward {
	// 1. 在线奖励
	onlineReward, _ := crc.onlineRewardCalc.CalculateOnlineReward(address, uptimeScore)

	// 2. 质量奖励
	qualityBonus := crc.calculateQualityBonus(blockReward, qualityScore)

	// 3. 服务奖励
	serviceBonus := crc.calculateServiceBonus(blockReward, serviceScore)

	// 4. 历史贡献奖励
	historicalBonus := crc.calculateHistoricalBonus(blockReward, address)

	// 5. 计算总奖励
	totalReward := new(big.Int).Set(blockReward)
	totalReward.Add(totalReward, onlineReward)
	totalReward.Add(totalReward, qualityBonus)
	totalReward.Add(totalReward, serviceBonus)
	totalReward.Add(totalReward, historicalBonus)

	return &ComprehensiveReward{
		Address:         address,
		BlockReward:     blockReward,
		OnlineReward:    onlineReward,
		QualityBonus:    qualityBonus,
		ServiceBonus:    serviceBonus,
		HistoricalBonus: historicalBonus,
		TotalReward:     totalReward,
	}
}

// calculateQualityBonus 计算质量奖励
func (crc *ComprehensiveRewardCalculator) calculateQualityBonus(blockReward *big.Int, qualityScore uint64) *big.Int {
	// 质量奖励 = 区块奖励 × 质量比率 × 质量奖励率
	qualityRatio := float64(qualityScore) / 10000.0
	bonusRate := crc.config.QualityBonusRate

	bonus := new(big.Int).Mul(blockReward, big.NewInt(int64(qualityRatio*bonusRate*1e18)))
	bonus.Div(bonus, big.NewInt(1e18))

	return bonus
}

// calculateServiceBonus 计算服务奖励
func (crc *ComprehensiveRewardCalculator) calculateServiceBonus(blockReward *big.Int, serviceScore uint64) *big.Int {
	// 服务奖励 = 区块奖励 × 服务比率 × 服务奖励率
	serviceRatio := float64(serviceScore) / 10000.0
	bonusRate := crc.config.ServiceBonusRate

	bonus := new(big.Int).Mul(blockReward, big.NewInt(int64(serviceRatio*bonusRate*1e18)))
	bonus.Div(bonus, big.NewInt(1e18))

	return bonus
}

// calculateHistoricalBonus 计算历史贡献奖励
func (crc *ComprehensiveRewardCalculator) calculateHistoricalBonus(blockReward *big.Int, address common.Address) *big.Int {
	// 获取历史贡献倍数
	multiplier := crc.historicalContributionTracker.GetMultiplier(address)
	bonusRate := crc.config.HistoricalBonusRate

	// 历史贡献奖励 = 区块奖励 × (倍数 - 1.0) × 历史奖励率
	bonusMultiplier := (multiplier - 1.0) * bonusRate

	bonus := new(big.Int).Mul(blockReward, big.NewInt(int64(bonusMultiplier*1e18)))
	bonus.Div(bonus, big.NewInt(1e18))

	return bonus
}

// SetBlockQualityScorer 设置区块质量评分器
func (crc *ComprehensiveRewardCalculator) SetBlockQualityScorer(scorer *BlockQualityScorer) {
	crc.blockQualityScorer = scorer
}

// SetServiceQualityScorer 设置服务质量评分器
func (crc *ComprehensiveRewardCalculator) SetServiceQualityScorer(scorer *ServiceQualityScorer) {
	crc.serviceQualityScorer = scorer
}

// SetTransactionVolumeTracker 设置交易量追踪器
func (crc *ComprehensiveRewardCalculator) SetTransactionVolumeTracker(tracker *TransactionVolumeTracker) {
	crc.transactionVolumeTracker = tracker
}

// GetHistoricalContributionTracker 获取历史贡献追踪器
func (crc *ComprehensiveRewardCalculator) GetHistoricalContributionTracker() *HistoricalContributionTracker {
	return crc.historicalContributionTracker
}
