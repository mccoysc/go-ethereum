package sgx

import (
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// ProducerPenaltyTracker 出块者惩罚追踪器
type ProducerPenaltyTracker struct {
	config *PenaltyConfig

	mu        sync.RWMutex
	penalties map[common.Address]*ProducerPenalty
}

// NewProducerPenaltyTracker 创建出块者惩罚追踪器
func NewProducerPenaltyTracker(config *PenaltyConfig) *ProducerPenaltyTracker {
	return &ProducerPenaltyTracker{
		config:    config,
		penalties: make(map[common.Address]*ProducerPenalty),
	}
}

// RecordLowQualityBlock 记录低质量区块
func (ppt *ProducerPenaltyTracker) RecordLowQualityBlock(address common.Address, qualityScore uint64) {
	ppt.mu.Lock()
	defer ppt.mu.Unlock()

	penalty, exists := ppt.penalties[address]
	if !exists {
		penalty = &ProducerPenalty{
			Address:      address,
			TotalPenalty: big.NewInt(0),
		}
		ppt.penalties[address] = penalty
	}

	if qualityScore < ppt.config.LowQualityThreshold {
		penalty.LowQualityCount++

		// 累积惩罚
		penaltyAmount := new(big.Int).Set(ppt.config.PenaltyAmount)
		penalty.TotalPenalty.Add(penalty.TotalPenalty, penaltyAmount)
	}
}

// RecordEmptyBlock 记录空区块
func (ppt *ProducerPenaltyTracker) RecordEmptyBlock(address common.Address) {
	ppt.mu.Lock()
	defer ppt.mu.Unlock()

	penalty, exists := ppt.penalties[address]
	if !exists {
		penalty = &ProducerPenalty{
			Address:      address,
			TotalPenalty: big.NewInt(0),
		}
		ppt.penalties[address] = penalty
	}

	penalty.EmptyBlockCount++

	// 连续空区块达到阈值，施加惩罚
	if penalty.EmptyBlockCount >= ppt.config.EmptyBlockThreshold {
		penaltyAmount := new(big.Int).Set(ppt.config.PenaltyAmount)
		penalty.TotalPenalty.Add(penalty.TotalPenalty, penaltyAmount)
		penalty.ExcludedUntil = time.Now().Add(ppt.config.ExclusionPeriod)
	}
}

// GetPenalty 获取惩罚数据
func (ppt *ProducerPenaltyTracker) GetPenalty(address common.Address) *ProducerPenalty {
	ppt.mu.RLock()
	defer ppt.mu.RUnlock()

	penalty, exists := ppt.penalties[address]
	if !exists {
		return nil
	}

	penaltyCopy := *penalty
	penaltyCopy.TotalPenalty = new(big.Int).Set(penalty.TotalPenalty)
	return &penaltyCopy
}

// IsExcluded 检查是否被排除
func (ppt *ProducerPenaltyTracker) IsExcluded(address common.Address) bool {
	ppt.mu.RLock()
	defer ppt.mu.RUnlock()

	penalty, exists := ppt.penalties[address]
	if !exists {
		return false
	}

	return penalty.EmptyBlockCount >= ppt.config.EmptyBlockThreshold
}

// ResetPenalty 重置惩罚
func (ppt *ProducerPenaltyTracker) ResetPenalty(address common.Address) {
	ppt.mu.Lock()
	defer ppt.mu.Unlock()

	delete(ppt.penalties, address)
}
