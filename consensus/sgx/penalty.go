package sgx

import (
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// PenaltyManagerImpl 惩罚管理实现
type PenaltyManagerImpl struct {
	config *PenaltyConfig

	mu         sync.RWMutex
	penalties  map[common.Address][]*PenaltyRecord
	exclusions map[common.Address]time.Time
}

// NewPenaltyManager 创建惩罚管理器
func NewPenaltyManager(config *PenaltyConfig) *PenaltyManagerImpl {
	return &PenaltyManagerImpl{
		config:     config,
		penalties:  make(map[common.Address][]*PenaltyRecord),
		exclusions: make(map[common.Address]time.Time),
	}
}

// RecordPenalty 记录惩罚
func (pm *PenaltyManagerImpl) RecordPenalty(address common.Address, penaltyType string, amount *big.Int, reason string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	record := &PenaltyRecord{
		Address:       address,
		PenaltyType:   penaltyType,
		PenaltyAmount: amount,
		Timestamp:     time.Now(),
		Reason:        reason,
	}

	pm.penalties[address] = append(pm.penalties[address], record)

	// 检查是否需要排除
	if len(pm.penalties[address]) >= 3 {
		pm.exclusions[address] = time.Now().Add(pm.config.ExclusionPeriod)
	}

	return nil
}

// GetPenaltyCount 获取惩罚次数
func (pm *PenaltyManagerImpl) GetPenaltyCount(address common.Address) (uint64, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return uint64(len(pm.penalties[address])), nil
}

// IsExcluded 检查是否被排除
func (pm *PenaltyManagerImpl) IsExcluded(address common.Address) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	excludedUntil, exists := pm.exclusions[address]
	if !exists {
		return false
	}

	return time.Now().Before(excludedUntil)
}

// GetExclusionEndTime 获取排除结束时间
func (pm *PenaltyManagerImpl) GetExclusionEndTime(address common.Address) (time.Time, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	excludedUntil, exists := pm.exclusions[address]
	if !exists {
		return time.Time{}, nil
	}

	return excludedUntil, nil
}

// ClearPenalties 清除惩罚记录
func (pm *PenaltyManagerImpl) ClearPenalties(address common.Address) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	delete(pm.penalties, address)
	delete(pm.exclusions, address)
}
