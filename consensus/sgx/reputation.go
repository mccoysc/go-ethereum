package sgx

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// ReputationSystem 信誉系统
type ReputationSystem struct {
	config           *ReputationConfig
	uptimeCalculator *UptimeCalculator
	penaltyManager   *PenaltyManagerImpl

	mu          sync.RWMutex
	reputations map[common.Address]*NodeReputation
}

// NewReputationSystem 创建信誉系统
func NewReputationSystem(config *ReputationConfig, uptimeCalculator *UptimeCalculator, penaltyManager *PenaltyManagerImpl) *ReputationSystem {
	return &ReputationSystem{
		config:           config,
		uptimeCalculator: uptimeCalculator,
		penaltyManager:   penaltyManager,
		reputations:      make(map[common.Address]*NodeReputation),
	}
}

// GetReputation 获取节点信誉
func (rs *ReputationSystem) GetReputation(address common.Address) (*NodeReputation, error) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	reputation, exists := rs.reputations[address]
	if !exists {
		return nil, nil
	}

	reputationCopy := *reputation
	return &reputationCopy, nil
}

// UpdateReputation 更新节点信誉
func (rs *ReputationSystem) UpdateReputation(address common.Address) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	reputation, exists := rs.reputations[address]
	if !exists {
		reputation = &NodeReputation{
			Address: address,
		}
		rs.reputations[address] = reputation
	}

	// 计算在线率评分
	// Use default network statistics
	// In production, these should come from a network statistics tracker
	const (
		defaultObservers = 10
		defaultTotalTxs  = uint64(10000)
		defaultTotalGas  = uint64(300000000)
	)
	
	uptimeData := rs.uptimeCalculator.CalculateUptimeScore(
		address,
		defaultObservers,
		defaultTotalTxs,
		defaultTotalGas,
	)
	reputation.UptimeScore = uptimeData.ComprehensiveScore

	// 获取惩罚次数
	penaltyCount, _ := rs.penaltyManager.GetPenaltyCount(address)
	reputation.PenaltyCount = penaltyCount

	// 计算综合信誉评分
	reputation.ReputationScore = rs.calculateReputationScore(reputation)
	reputation.LastUpdateTime = time.Now()

	return nil
}

// calculateReputationScore 计算信誉评分
func (rs *ReputationSystem) calculateReputationScore(reputation *NodeReputation) uint64 {
	// 在线率权重 60%
	uptimeComponent := float64(reputation.UptimeScore) * rs.config.UptimeWeight / 100.0

	// 成功率权重 30%
	successComponent := reputation.SuccessRate * 10000 * rs.config.SuccessRateWeight / 100.0

	// 惩罚权重 10%（惩罚降低评分）
	penaltyComponent := float64(reputation.PenaltyCount) * 1000
	if penaltyComponent > 10000*rs.config.PenaltyWeight/100.0 {
		penaltyComponent = 10000 * rs.config.PenaltyWeight / 100.0
	}

	score := uptimeComponent + successComponent - penaltyComponent
	if score < 0 {
		score = 0
	}
	if score > 10000 {
		score = 10000
	}

	return uint64(score)
}

// IsExcluded 检查节点是否被排除
func (rs *ReputationSystem) IsExcluded(address common.Address) bool {
	return rs.penaltyManager.IsExcluded(address)
}

// GetNodePriority 获取节点优先级
func (rs *ReputationSystem) GetNodePriority(address common.Address) (uint64, error) {
	reputation, err := rs.GetReputation(address)
	if err != nil {
		return 0, err
	}
	if reputation == nil {
		return 0, nil
	}
	return reputation.ReputationScore, nil
}
