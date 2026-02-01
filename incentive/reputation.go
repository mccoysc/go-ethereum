// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package incentive

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// ReputationManager 声誉管理器
type ReputationManager struct {
	config      *ReputationConfig
	mu          sync.RWMutex
	reputations map[common.Address]*NodeReputation
}

// NodeReputation 节点声誉
type NodeReputation struct {
	Address         common.Address
	Score           int64
	TotalBlocks     uint64
	SuccessBlocks   uint64
	FailedBlocks    uint64
	MaliciousCount  uint64
	PenaltyCount    uint64
	OfflineHours    uint64
	OnlineHours     uint64
	LastUpdateTime  time.Time
	LastDecayTime   time.Time
	LastOnlineCheck time.Time
}

// NewReputationManager 创建声誉管理器
func NewReputationManager(config *ReputationConfig) *ReputationManager {
	return &ReputationManager{
		config:      config,
		reputations: make(map[common.Address]*NodeReputation),
	}
}

// GetReputation 获取节点声誉
func (rm *ReputationManager) GetReputation(addr common.Address) *NodeReputation {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	rep, ok := rm.reputations[addr]
	if !ok {
		return &NodeReputation{
			Address: addr,
			Score:   rm.config.InitialReputation,
		}
	}

	return rep
}

// RecordBlockSuccess 记录成功出块
func (rm *ReputationManager) RecordBlockSuccess(addr common.Address) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rep := rm.getOrCreateReputation(addr)

	// 应用衰减
	rm.applyDecay(rep)

	// 增加声誉
	rep.Score += rm.config.SuccessBonus
	if rep.Score > rm.config.MaxReputation {
		rep.Score = rm.config.MaxReputation
	}

	rep.TotalBlocks++
	rep.SuccessBlocks++
	rep.LastUpdateTime = time.Now()
}

// RecordBlockFailure 记录出块失败
func (rm *ReputationManager) RecordBlockFailure(addr common.Address) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rep := rm.getOrCreateReputation(addr)

	// 应用衰减
	rm.applyDecay(rep)

	// 减少声誉
	rep.Score -= rm.config.FailurePenalty
	if rep.Score < rm.config.MinReputation {
		rep.Score = rm.config.MinReputation
	}

	rep.TotalBlocks++
	rep.FailedBlocks++
	rep.LastUpdateTime = time.Now()
}

// RecordMaliciousBehavior 记录恶意行为
func (rm *ReputationManager) RecordMaliciousBehavior(addr common.Address) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rep := rm.getOrCreateReputation(addr)

	// 大幅减少声誉
	rep.Score -= rm.config.MaliciousPenalty
	if rep.Score < rm.config.MinReputation {
		rep.Score = rm.config.MinReputation
	}

	rep.MaliciousCount++
	rep.PenaltyCount++
	rep.LastUpdateTime = time.Now()
}

// RecordOffline 记录节点离线
//
// 基于 ARCHITECTURE.md 的声誉衰减机制：
// - 每小时离线扣除 10 分声誉
// - 累积惩罚次数超过 MaxPenaltyCount 将被排除出网络
func (rm *ReputationManager) RecordOffline(addr common.Address, duration time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rep := rm.getOrCreateReputation(addr)

	// 计算离线小时数
	hours := uint64(duration.Hours())
	if hours == 0 && duration > 0 {
		hours = 1
	}

	// 应用离线惩罚
	penalty := int64(hours * uint64(rm.config.OfflinePenaltyPerHour))
	rep.Score -= penalty
	if rep.Score < rm.config.MinReputation {
		rep.Score = rm.config.MinReputation
	}

	rep.OfflineHours += hours
	rep.PenaltyCount++
	rep.LastUpdateTime = time.Now()
	rep.LastOnlineCheck = time.Now()
}

// RecordOnline 记录节点在线
//
// 基于 ARCHITECTURE.md 的声誉恢复机制：
// - 每小时在线恢复 50 分声誉（是离线惩罚的 5 倍）
// - 帮助节点快速恢复声誉，鼓励长期稳定在线
func (rm *ReputationManager) RecordOnline(addr common.Address, duration time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rep := rm.getOrCreateReputation(addr)

	// 计算在线小时数
	hours := uint64(duration.Hours())
	if hours == 0 && duration > 0 {
		hours = 1
	}

	// 应用在线恢复奖励
	recovery := int64(hours * uint64(rm.config.RecoveryPerHour))
	rep.Score += recovery
	if rep.Score > rm.config.MaxReputation {
		rep.Score = rm.config.MaxReputation
	}

	rep.OnlineHours += hours
	rep.LastUpdateTime = time.Now()
	rep.LastOnlineCheck = time.Now()
}

// IsExcluded 检查节点是否因惩罚过多而被排除
func (rm *ReputationManager) IsExcluded(addr common.Address) bool {
	rep := rm.GetReputation(addr)
	return rep.PenaltyCount >= uint64(rm.config.MaxPenaltyCount)
}

// GetReputationScore 获取声誉分数
func (rm *ReputationManager) GetReputationScore(addr common.Address) int64 {
	rep := rm.GetReputation(addr)
	return rep.Score
}

// IsReputationSufficient 检查声誉是否足够
func (rm *ReputationManager) IsReputationSufficient(addr common.Address, threshold int64) bool {
	return rm.GetReputationScore(addr) >= threshold
}

// getOrCreateReputation 获取或创建声誉记录
func (rm *ReputationManager) getOrCreateReputation(addr common.Address) *NodeReputation {
	rep, ok := rm.reputations[addr]
	if !ok {
		now := time.Now()
		rep = &NodeReputation{
			Address:         addr,
			Score:           rm.config.InitialReputation,
			LastUpdateTime:  now,
			LastDecayTime:   now,
			LastOnlineCheck: now,
		}
		rm.reputations[addr] = rep
	}
	return rep
}

// applyDecay 应用声誉衰减
func (rm *ReputationManager) applyDecay(rep *NodeReputation) {
	elapsed := time.Since(rep.LastDecayTime)
	periods := int(elapsed / (24 * time.Hour))

	if periods > 0 {
		for i := 0; i < periods; i++ {
			decay := int64(float64(rep.Score) * rm.config.DecayRate)
			rep.Score -= decay
			if rep.Score < rm.config.MinReputation {
				rep.Score = rm.config.MinReputation
				break
			}
		}
		rep.LastDecayTime = time.Now()
	}
}
