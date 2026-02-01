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
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// OnlineRewardManager is the online reward manager.
type OnlineRewardManager struct {
	config     *OnlineRewardConfig
	mu         sync.RWMutex
	nodeStatus map[common.Address]*NodeOnlineStatus
}

// NodeOnlineStatus represents the node's online status.
type NodeOnlineStatus struct {
	Address           common.Address
	LastHeartbeat     time.Time
	OnlineStartTime   time.Time
	TotalOnlineTime   time.Duration
	TotalOfflineTime  time.Duration
	HeartbeatCount    uint64
	MissedHeartbeats  uint64
	AccumulatedReward *big.Int
	ClaimedReward     *big.Int
}

// NewOnlineRewardManager creates a new online reward manager.
func NewOnlineRewardManager(config *OnlineRewardConfig) *OnlineRewardManager {
	return &OnlineRewardManager{
		config:     config,
		nodeStatus: make(map[common.Address]*NodeOnlineStatus),
	}
}

// RecordHeartbeat records a heartbeat.
func (orm *OnlineRewardManager) RecordHeartbeat(addr common.Address) {
	orm.mu.Lock()
	defer orm.mu.Unlock()

	status := orm.getOrCreateStatus(addr)
	now := time.Now()

	// Check if recovering from offline
	if time.Since(status.LastHeartbeat) > orm.config.HeartbeatTimeout {
		// Record offline time
		if !status.LastHeartbeat.IsZero() {
			status.TotalOfflineTime += time.Since(status.LastHeartbeat)
		}
		status.OnlineStartTime = now
	} else {
		// Accumulate online time
		if !status.LastHeartbeat.IsZero() {
			status.TotalOnlineTime += time.Since(status.LastHeartbeat)
		}
	}

	status.LastHeartbeat = now
	status.HeartbeatCount++
}

// CalculateReward calculates the online reward.
func (orm *OnlineRewardManager) CalculateReward(addr common.Address) *big.Int {
	orm.mu.RLock()
	defer orm.mu.RUnlock()

	status, ok := orm.nodeStatus[addr]
	if !ok {
		return big.NewInt(0)
	}

	// Check minimum online duration
	if status.TotalOnlineTime < orm.config.MinOnlineDuration {
		return big.NewInt(0)
	}

	// Calculate uptime ratio
	totalTime := status.TotalOnlineTime + status.TotalOfflineTime
	if totalTime == 0 {
		return big.NewInt(0)
	}

	uptimeRatio := float64(status.TotalOnlineTime) / float64(totalTime)
	if uptimeRatio < orm.config.MinUptimeRatio {
		return big.NewInt(0)
	}

	// Calculate reward
	hours := int64(status.TotalOnlineTime / time.Hour)
	reward := new(big.Int).Mul(orm.config.HourlyReward, big.NewInt(hours))

	// Apply uptime ratio bonus
	bonus := new(big.Int).Mul(reward, big.NewInt(int64(uptimeRatio*100)))
	bonus.Div(bonus, big.NewInt(100))

	return bonus
}

// GetUptimeRatio retrieves the uptime ratio.
func (orm *OnlineRewardManager) GetUptimeRatio(addr common.Address) float64 {
	orm.mu.RLock()
	defer orm.mu.RUnlock()

	status, ok := orm.nodeStatus[addr]
	if !ok {
		return 0
	}

	totalTime := status.TotalOnlineTime + status.TotalOfflineTime
	if totalTime == 0 {
		return 0
	}

	return float64(status.TotalOnlineTime) / float64(totalTime)
}

// IsOnline checks if the node is online.
func (orm *OnlineRewardManager) IsOnline(addr common.Address) bool {
	orm.mu.RLock()
	defer orm.mu.RUnlock()

	status, ok := orm.nodeStatus[addr]
	if !ok {
		return false
	}

	return time.Since(status.LastHeartbeat) <= orm.config.HeartbeatTimeout
}

// GetOnlineTime retrieves the online duration.
func (orm *OnlineRewardManager) GetOnlineTime(addr common.Address) time.Duration {
	orm.mu.RLock()
	defer orm.mu.RUnlock()

	status, ok := orm.nodeStatus[addr]
	if !ok {
		return 0
	}

	return status.TotalOnlineTime
}

// GetOfflineTime retrieves the offline duration.
func (orm *OnlineRewardManager) GetOfflineTime(addr common.Address) time.Duration {
	orm.mu.RLock()
	defer orm.mu.RUnlock()

	status, ok := orm.nodeStatus[addr]
	if !ok {
		return 0
	}

	return status.TotalOfflineTime
}

// getOrCreateStatus gets or creates a status.
func (orm *OnlineRewardManager) getOrCreateStatus(addr common.Address) *NodeOnlineStatus {
	status, ok := orm.nodeStatus[addr]
	if !ok {
		status = &NodeOnlineStatus{
			Address:           addr,
			AccumulatedReward: big.NewInt(0),
			ClaimedReward:     big.NewInt(0),
		}
		orm.nodeStatus[addr] = status
	}
	return status
}
