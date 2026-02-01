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

// ReputationManager is the reputation manager.
type ReputationManager struct {
	config      *ReputationConfig
	mu          sync.RWMutex
	reputations map[common.Address]*NodeReputation
}

// NodeReputation represents the node's reputation.
type NodeReputation struct {
	Address         common.Address
	Score           uint64
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

// NewReputationManager creates a new reputation manager.
func NewReputationManager(config *ReputationConfig) *ReputationManager {
	return &ReputationManager{
		config:      config,
		reputations: make(map[common.Address]*NodeReputation),
	}
}

// GetReputation retrieves the node's reputation.
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

// RecordBlockSuccess records a successful block production.
func (rm *ReputationManager) RecordBlockSuccess(addr common.Address) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rep := rm.getOrCreateReputation(addr)

	// Apply decay
	rm.applyDecay(rep)

	// Increase reputation
	newScore := rep.Score + rm.config.SuccessBonus
	if newScore > rm.config.MaxReputation {
		rep.Score = rm.config.MaxReputation
	} else {
		rep.Score = newScore
	}

	rep.TotalBlocks++
	rep.SuccessBlocks++
	rep.LastUpdateTime = time.Now()
}

// RecordBlockFailure records a block production failure.
func (rm *ReputationManager) RecordBlockFailure(addr common.Address) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rep := rm.getOrCreateReputation(addr)

	// Apply decay
	rm.applyDecay(rep)

	// Decrease reputation (protect against underflow)
	if rep.Score >= rm.config.FailurePenalty {
		rep.Score -= rm.config.FailurePenalty
	} else {
		rep.Score = rm.config.MinReputation
	}

	rep.TotalBlocks++
	rep.FailedBlocks++
	rep.LastUpdateTime = time.Now()
}

// RecordMaliciousBehavior records malicious behavior.
func (rm *ReputationManager) RecordMaliciousBehavior(addr common.Address) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rep := rm.getOrCreateReputation(addr)

	// Significantly decrease reputation (protect against underflow)
	if rep.Score >= rm.config.MaliciousPenalty {
		rep.Score -= rm.config.MaliciousPenalty
	} else {
		rep.Score = rm.config.MinReputation
	}

	rep.MaliciousCount++
	rep.PenaltyCount++
	rep.LastUpdateTime = time.Now()
}

// RecordOffline records node offline status.
//
// Based on the reputation decay mechanism in ARCHITECTURE.md:
// - 10 reputation points are deducted per hour offline
// - Nodes exceeding MaxPenaltyCount accumulated penalties will be excluded from the network
func (rm *ReputationManager) RecordOffline(addr common.Address, duration time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rep := rm.getOrCreateReputation(addr)

	// Calculate offline hours
	hours := uint64(duration.Hours())
	if hours == 0 && duration > 0 {
		hours = 1
	}

	// Apply offline penalty (protect against underflow)
	penalty := hours * rm.config.OfflinePenaltyPerHour
	if rep.Score >= penalty {
		rep.Score -= penalty
	} else {
		rep.Score = rm.config.MinReputation
	}

	rep.OfflineHours += hours
	rep.PenaltyCount++
	rep.LastUpdateTime = time.Now()
	rep.LastOnlineCheck = time.Now()
}

// RecordOnline records node online status.
//
// Based on the reputation recovery mechanism in ARCHITECTURE.md:
// - 50 reputation points are recovered per hour online (5x the offline penalty)
// - Helps nodes quickly recover reputation, encouraging long-term stable online presence
func (rm *ReputationManager) RecordOnline(addr common.Address, duration time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rep := rm.getOrCreateReputation(addr)

	// Calculate online hours
	hours := uint64(duration.Hours())
	if hours == 0 && duration > 0 {
		hours = 1
	}

	// Apply online recovery reward
	recovery := hours * rm.config.OnlineRecoveryPerHour
	newScore := rep.Score + recovery
	if newScore > rm.config.MaxReputation {
		rep.Score = rm.config.MaxReputation
	} else {
		rep.Score = newScore
	}

	rep.OnlineHours += hours
	rep.LastUpdateTime = time.Now()
	rep.LastOnlineCheck = time.Now()
}

// IsExcluded checks if the node is excluded due to excessive penalties.
func (rm *ReputationManager) IsExcluded(addr common.Address) bool {
	rep := rm.GetReputation(addr)
	return rep.PenaltyCount >= uint64(rm.config.MaxPenaltyCount)
}

// GetReputationScore retrieves the reputation score.
func (rm *ReputationManager) GetReputationScore(addr common.Address) uint64 {
	rep := rm.GetReputation(addr)
	return rep.Score
}

// IsReputationSufficient checks if the reputation is sufficient.
func (rm *ReputationManager) IsReputationSufficient(addr common.Address, threshold uint64) bool {
	return rm.GetReputationScore(addr) >= threshold
}

// getOrCreateReputation gets or creates a reputation record.
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

// applyDecay applies reputation decay.
func (rm *ReputationManager) applyDecay(rep *NodeReputation) {
	elapsed := time.Since(rep.LastDecayTime)
	periods := int(elapsed / (24 * time.Hour))

	if periods > 0 {
		for i := 0; i < periods; i++ {
			decay := (rep.Score * uint64(rm.config.DecayRate)) / 100
			if rep.Score >= decay {
				rep.Score -= decay
			} else {
				rep.Score = rm.config.MinReputation
				break
			}
		}
		rep.LastDecayTime = time.Now()
	}
}
