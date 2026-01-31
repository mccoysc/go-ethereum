package sgx

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// HistoricalContributionTracker 历史贡献追踪器
type HistoricalContributionTracker struct {
	mu            sync.RWMutex
	contributions map[common.Address]*HistoricalContribution
}

// NewHistoricalContributionTracker 创建历史贡献追踪器
func NewHistoricalContributionTracker() *HistoricalContributionTracker {
	return &HistoricalContributionTracker{
		contributions: make(map[common.Address]*HistoricalContribution),
	}
}

// RecordContribution 记录贡献
func (hct *HistoricalContributionTracker) RecordContribution(address common.Address, blocks uint64, txs uint64) {
	hct.mu.Lock()
	defer hct.mu.Unlock()

	contribution, exists := hct.contributions[address]
	if !exists {
		contribution = &HistoricalContribution{
			Address:    address,
			ActiveDays: 0,
		}
		hct.contributions[address] = contribution
	}

	contribution.TotalBlocks += blocks
	contribution.TotalTxs += txs

	// 更新活跃天数（基于首次记录时间）
	if contribution.FirstContributionTime.IsZero() {
		contribution.FirstContributionTime = time.Now()
	}
	if contribution.LastUpdateTime.IsZero() {
		contribution.LastUpdateTime = time.Now()
	}
	daysSinceStart := uint64(time.Since(contribution.FirstContributionTime).Hours() / 24)
	if daysSinceStart > contribution.ActiveDays {
		contribution.ActiveDays = daysSinceStart
	}
	contribution.LastUpdateTime = time.Now()

	// 计算贡献倍数
	contribution.ContributionMultiplier = hct.calculateMultiplier(contribution)
}

// calculateMultiplier 计算贡献倍数
func (hct *HistoricalContributionTracker) calculateMultiplier(contribution *HistoricalContribution) float64 {
	// 基础倍数 1.0
	multiplier := 1.0

	// 根据总区块数增加倍数（最多 +0.5）
	blockBonus := float64(contribution.TotalBlocks) / 10000.0
	if blockBonus > 0.5 {
		blockBonus = 0.5
	}

	// 根据活跃天数增加倍数（最多 +0.5）
	dayBonus := float64(contribution.ActiveDays) / 365.0
	if dayBonus > 0.5 {
		dayBonus = 0.5
	}

	multiplier += blockBonus + dayBonus

	// 最大倍数 2.0
	if multiplier > 2.0 {
		multiplier = 2.0
	}

	return multiplier
}

// GetContribution 获取贡献数据
func (hct *HistoricalContributionTracker) GetContribution(address common.Address) *HistoricalContribution {
	hct.mu.RLock()
	defer hct.mu.RUnlock()

	contribution, exists := hct.contributions[address]
	if !exists {
		return nil
	}

	contributionCopy := *contribution
	return &contributionCopy
}

// GetMultiplier 获取倍数
func (hct *HistoricalContributionTracker) GetMultiplier(address common.Address) float64 {
	hct.mu.RLock()
	defer hct.mu.RUnlock()

	contribution, exists := hct.contributions[address]
	if !exists {
		return 1.0
	}

	return contribution.ContributionMultiplier
}
