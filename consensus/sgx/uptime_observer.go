package sgx

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// UptimeObserver 多节点在线率观测器
type UptimeObserver struct {
	mu           sync.RWMutex
	observations map[common.Address]map[common.Address]time.Time // [observed][observer]lastSeen
	threshold    float64                                         // 共识阈值（例如 2/3）
}

// NewUptimeObserver 创建在线率观测器
func NewUptimeObserver(threshold float64) *UptimeObserver {
	return &UptimeObserver{
		observations: make(map[common.Address]map[common.Address]time.Time),
		threshold:    threshold,
	}
}

// RecordObservation 记录观测
func (uo *UptimeObserver) RecordObservation(observed, observer common.Address) {
	uo.mu.Lock()
	defer uo.mu.Unlock()

	if uo.observations[observed] == nil {
		uo.observations[observed] = make(map[common.Address]time.Time)
	}
	uo.observations[observed][observer] = time.Now()
}

// CalculateConsensusScore 计算共识评分
func (uo *UptimeObserver) CalculateConsensusScore(address common.Address, totalObservers int) uint64 {
	uo.mu.RLock()
	defer uo.mu.RUnlock()

	observations, exists := uo.observations[address]
	if !exists || totalObservers == 0 {
		return 0
	}

	// 计算最近看到该节点的观测者数量
	recentCount := 0
	now := time.Now()
	recentWindow := 5 * time.Minute

	for _, lastSeen := range observations {
		if now.Sub(lastSeen) < recentWindow {
			recentCount++
		}
	}

	// 计算共识比例
	consensusRatio := float64(recentCount) / float64(totalObservers)

	// 转换为评分（0-10000）
	score := uint64(consensusRatio * 10000)
	return score
}

// HasConsensus 检查是否达成共识
func (uo *UptimeObserver) HasConsensus(address common.Address, totalObservers int) bool {
	score := uo.CalculateConsensusScore(address, totalObservers)
	requiredScore := uint64(uo.threshold * 10000)
	return score >= requiredScore
}

// CleanOldObservations 清理旧观测记录
func (uo *UptimeObserver) CleanOldObservations(maxAge time.Duration) {
	uo.mu.Lock()
	defer uo.mu.Unlock()

	now := time.Now()
	for observed, observers := range uo.observations {
		for observer, lastSeen := range observers {
			if now.Sub(lastSeen) > maxAge {
				delete(observers, observer)
			}
		}
		if len(observers) == 0 {
			delete(uo.observations, observed)
		}
	}
}
