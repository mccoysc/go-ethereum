package sgx

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// HeartbeatTracker SGX 心跳追踪器
type HeartbeatTracker struct {
	mu         sync.RWMutex
	heartbeats map[common.Address]*HeartbeatRecord
}

// HeartbeatRecord 心跳记录
type HeartbeatRecord struct {
	Address        common.Address
	LastHeartbeat  time.Time
	HeartbeatCount uint64
	MissedCount    uint64
}

// NewHeartbeatTracker 创建心跳追踪器
func NewHeartbeatTracker() *HeartbeatTracker {
	return &HeartbeatTracker{
		heartbeats: make(map[common.Address]*HeartbeatRecord),
	}
}

// RecordHeartbeat 记录心跳
func (ht *HeartbeatTracker) RecordHeartbeat(msg *HeartbeatMessage) error {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	record, exists := ht.heartbeats[msg.NodeID]
	if !exists {
		record = &HeartbeatRecord{
			Address: msg.NodeID,
		}
		ht.heartbeats[msg.NodeID] = record
	}

	record.LastHeartbeat = time.Now()
	record.HeartbeatCount++

	return nil
}

// GetHeartbeatRecord 获取心跳记录
func (ht *HeartbeatTracker) GetHeartbeatRecord(address common.Address) *HeartbeatRecord {
	ht.mu.RLock()
	defer ht.mu.RUnlock()

	record, exists := ht.heartbeats[address]
	if !exists {
		return nil
	}

	// 返回副本
	recordCopy := *record
	return &recordCopy
}

// CalculateHeartbeatScore 计算心跳评分
func (ht *HeartbeatTracker) CalculateHeartbeatScore(address common.Address, interval time.Duration) uint64 {
	ht.mu.RLock()
	defer ht.mu.RUnlock()

	record, exists := ht.heartbeats[address]
	if !exists {
		return 0
	}

	// 计算预期心跳次数
	elapsed := time.Since(record.LastHeartbeat)
	expectedHeartbeats := uint64(elapsed / interval)
	if expectedHeartbeats == 0 {
		expectedHeartbeats = 1
	}

	// 计算实际心跳率
	actualRate := float64(record.HeartbeatCount) / float64(expectedHeartbeats)
	if actualRate > 1.0 {
		actualRate = 1.0
	}

	// 转换为评分（0-10000）
	score := uint64(actualRate * 10000)
	return score
}

// CheckMissedHeartbeats 检查缺失的心跳
func (ht *HeartbeatTracker) CheckMissedHeartbeats(interval time.Duration) {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	now := time.Now()
	for _, record := range ht.heartbeats {
		elapsed := now.Sub(record.LastHeartbeat)
		if elapsed > interval*2 {
			record.MissedCount++
		}
	}
}

// GetAllRecords 获取所有记录
func (ht *HeartbeatTracker) GetAllRecords() map[common.Address]*HeartbeatRecord {
	ht.mu.RLock()
	defer ht.mu.RUnlock()

	records := make(map[common.Address]*HeartbeatRecord)
	for addr, record := range ht.heartbeats {
		recordCopy := *record
		records[addr] = &recordCopy
	}

	return records
}

// ResetRecord 重置记录
func (ht *HeartbeatTracker) ResetRecord(address common.Address) {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	delete(ht.heartbeats, address)
}
