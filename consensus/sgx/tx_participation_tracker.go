package sgx

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// TxParticipationTracker 交易参与追踪器
type TxParticipationTracker struct {
	mu             sync.RWMutex
	participations map[common.Address]*TxParticipation
}

// NewTxParticipationTracker 创建交易参与追踪器
func NewTxParticipationTracker() *TxParticipationTracker {
	return &TxParticipationTracker{
		participations: make(map[common.Address]*TxParticipation),
	}
}

// RecordParticipation 记录交易参与
func (tpt *TxParticipationTracker) RecordParticipation(address common.Address, txCount, gasUsed uint64) {
	tpt.mu.Lock()
	defer tpt.mu.Unlock()

	participation, exists := tpt.participations[address]
	if !exists {
		participation = &TxParticipation{
			Address: address,
		}
		tpt.participations[address] = participation
	}

	participation.ProcessedTxs += txCount
	participation.ProcessedGas += gasUsed
	participation.TotalBlocks++
	participation.LastUpdateTime = time.Now()
}

// GetParticipation 获取参与数据
func (tpt *TxParticipationTracker) GetParticipation(address common.Address) *TxParticipation {
	tpt.mu.RLock()
	defer tpt.mu.RUnlock()

	participation, exists := tpt.participations[address]
	if !exists {
		return nil
	}

	participationCopy := *participation
	return &participationCopy
}

// CalculateParticipationScore 计算参与评分
func (tpt *TxParticipationTracker) CalculateParticipationScore(address common.Address, totalTxs, totalGas uint64) uint64 {
	tpt.mu.RLock()
	defer tpt.mu.RUnlock()

	participation, exists := tpt.participations[address]
	if !exists || totalTxs == 0 {
		return 0
	}

	// 计算交易份额
	txShare := float64(participation.ProcessedTxs) / float64(totalTxs)
	if txShare > 1.0 {
		txShare = 1.0
	}

	// 计算 Gas 份额
	gasShare := float64(participation.ProcessedGas) / float64(totalGas)
	if gasShare > 1.0 {
		gasShare = 1.0
	}

	// 综合评分（50% 交易数 + 50% Gas）
	score := (txShare*0.5 + gasShare*0.5) * 10000

	return uint64(score)
}

// Reset 重置统计
func (tpt *TxParticipationTracker) Reset(address common.Address) {
	tpt.mu.Lock()
	defer tpt.mu.Unlock()

	delete(tpt.participations, address)
}
