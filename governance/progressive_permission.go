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

package governance

import (
	"sync"
)

// ProgressivePermissionManager manages progressive permission levels for nodes
type ProgressivePermissionManager struct {
	config    *ProgressivePermissionConfig
	mu        sync.RWMutex
	nodePerms map[[32]byte]*NodePermission
}

// NewProgressivePermissionManager creates a new progressive permission manager
func NewProgressivePermissionManager(config *ProgressivePermissionConfig) *ProgressivePermissionManager {
	return &ProgressivePermissionManager{
		config:    config,
		nodePerms: make(map[[32]byte]*NodePermission),
	}
}

// GetPermissionLevel returns the permission level for an MRENCLAVE
func (pm *ProgressivePermissionManager) GetPermissionLevel(mrenclave [32]byte, currentBlock uint64) PermissionLevel {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	perm, exists := pm.nodePerms[mrenclave]
	if !exists {
		return PermissionBasic
	}

	return perm.CurrentLevel
}

// CheckUpgrade checks if a node should be upgraded to a higher permission level
func (pm *ProgressivePermissionManager) CheckUpgrade(mrenclave [32]byte, currentBlock uint64, uptime float64) (bool, PermissionLevel) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	perm, exists := pm.nodePerms[mrenclave]
	if !exists {
		// Initialize with basic permission
		perm = &NodePermission{
			MRENCLAVE:     mrenclave,
			CurrentLevel:  PermissionBasic,
			ActivatedAt:   currentBlock,
			LastUpgradeAt: currentBlock,
			UptimeHistory: make([]float64, 0),
		}
		pm.nodePerms[mrenclave] = perm
		return false, PermissionBasic
	}

	// Add current uptime to history
	perm.UptimeHistory = append(perm.UptimeHistory, uptime)

	// Calculate average uptime
	avgUptime := pm.calculateAverageUptime(perm.UptimeHistory)

	// Check for upgrade based on current level
	switch perm.CurrentLevel {
	case PermissionBasic:
		// Check if node can upgrade to Standard
		blocksSinceActivation := currentBlock - perm.ActivatedAt
		if blocksSinceActivation >= pm.config.BasicDuration && avgUptime >= pm.config.StandardUptimeThreshold {
			perm.CurrentLevel = PermissionStandard
			perm.LastUpgradeAt = currentBlock
			return true, PermissionStandard
		}

	case PermissionStandard:
		// Check if node can upgrade to Full
		blocksSinceActivation := currentBlock - perm.ActivatedAt
		if blocksSinceActivation >= pm.config.BasicDuration+pm.config.StandardDuration && avgUptime >= pm.config.FullUptimeThreshold {
			perm.CurrentLevel = PermissionFull
			perm.LastUpgradeAt = currentBlock
			return true, PermissionFull
		}
	}

	return false, perm.CurrentLevel
}

// calculateAverageUptime calculates the average uptime from history
func (pm *ProgressivePermissionManager) calculateAverageUptime(history []float64) float64 {
	if len(history) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, uptime := range history {
		sum += uptime
	}

	return sum / float64(len(history))
}

// Downgrade downgrades a node's permission level for misbehavior
func (pm *ProgressivePermissionManager) Downgrade(mrenclave [32]byte, reason string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	perm, exists := pm.nodePerms[mrenclave]
	if !exists {
		return
	}

	// Downgrade to basic permission
	perm.CurrentLevel = PermissionBasic
	perm.UptimeHistory = make([]float64, 0) // Reset history
}

// GetNodePermission returns the permission details for a node
func (pm *ProgressivePermissionManager) GetNodePermission(mrenclave [32]byte) (*NodePermission, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	perm, exists := pm.nodePerms[mrenclave]
	if !exists {
		return nil, false
	}

	permCopy := *perm
	permCopy.UptimeHistory = make([]float64, len(perm.UptimeHistory))
	copy(permCopy.UptimeHistory, perm.UptimeHistory)

	return &permCopy, true
}

// ActivateNode activates a new node with basic permission
func (pm *ProgressivePermissionManager) ActivateNode(mrenclave [32]byte, currentBlock uint64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.nodePerms[mrenclave]; exists {
		return // Already activated
	}

	perm := &NodePermission{
		MRENCLAVE:     mrenclave,
		CurrentLevel:  PermissionBasic,
		ActivatedAt:   currentBlock,
		LastUpgradeAt: currentBlock,
		UptimeHistory: make([]float64, 0),
	}
	pm.nodePerms[mrenclave] = perm
}
