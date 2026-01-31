package sgx

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// TransactionVolumeTracker 交易量追踪器
type TransactionVolumeTracker struct {
	mu         sync.RWMutex
	volumeData map[common.Address]*TransactionVolumeData
	totalTxCount uint64
	totalGasUsed uint64
}

// NewTransactionVolumeTracker 创建交易量追踪器
func NewTransactionVolumeTracker() *TransactionVolumeTracker {
	return &TransactionVolumeTracker{
		volumeData: make(map[common.Address]*TransactionVolumeData),
	}
}

// RecordVolume 记录交易量
func (tvt *TransactionVolumeTracker) RecordVolume(address common.Address, txCount uint64, gasUsed uint64) {
	tvt.mu.Lock()
	defer tvt.mu.Unlock()

	data, exists := tvt.volumeData[address]
	if !exists {
		data = &TransactionVolumeData{
			Address: address,
		}
		tvt.volumeData[address] = data
	}

	data.TxCount += txCount
	data.GasUsed += gasUsed

	// 更新总量
	tvt.totalTxCount += txCount
	tvt.totalGasUsed += gasUsed

	// 计算市场份额
	if tvt.totalTxCount > 0 {
		data.MarketShare = float64(data.TxCount) / float64(tvt.totalTxCount)
	}

	// 计算交易量评分
	data.VolumeScore = tvt.calculateVolumeScore(data)
}

// calculateVolumeScore 计算交易量评分
func (tvt *TransactionVolumeTracker) calculateVolumeScore(data *TransactionVolumeData) uint64 {
	// 基于市场份额计算评分
	// 市场份额 0-10% -> 评分 0-5000
	// 市场份额 10-50% -> 评分 5000-10000
	
	share := data.MarketShare
	if share <= 0.1 {
		return uint64(share / 0.1 * 5000)
	}
	
	return uint64(5000 + (share-0.1)/0.4*5000)
}

// GetVolumeData 获取交易量数据
func (tvt *TransactionVolumeTracker) GetVolumeData(address common.Address) *TransactionVolumeData {
	tvt.mu.RLock()
	defer tvt.mu.RUnlock()

	data, exists := tvt.volumeData[address]
	if !exists {
		return nil
	}

	dataCopy := *data
	return &dataCopy
}

// GetTotalVolume 获取总交易量
func (tvt *TransactionVolumeTracker) GetTotalVolume() (uint64, uint64) {
	tvt.mu.RLock()
	defer tvt.mu.RUnlock()

	return tvt.totalTxCount, tvt.totalGasUsed
}
