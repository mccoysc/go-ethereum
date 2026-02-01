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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/security"
)

// SecurityConfigReader is the interface for reading security configuration
type SecurityConfigReader interface {
	// GetMREnclaveWhitelist returns the MRENCLAVE whitelist
	GetMREnclaveWhitelist() []MREnclaveEntry

	// GetUpgradeConfig returns the upgrade configuration
	GetUpgradeConfig() *security.UpgradeConfig

	// GetSecretDataSyncState returns the secret data sync state
	GetSecretDataSyncState() *security.SecretDataSyncState
}

// UpgradeModeChecker checks upgrade mode and determines if operations should be rejected
type UpgradeModeChecker struct {
	securityConfig SecurityConfigReader
	localMREnclave [32]byte
}

// NewUpgradeModeChecker creates a new upgrade mode checker
func NewUpgradeModeChecker(config SecurityConfigReader, localMR [32]byte) *UpgradeModeChecker {
	return &UpgradeModeChecker{
		securityConfig: config,
		localMREnclave: localMR,
	}
}

// IsUpgradeInProgress checks if an upgrade is in progress
// An upgrade is in progress when the whitelist contains multiple MRENCLAVEs
func (c *UpgradeModeChecker) IsUpgradeInProgress() bool {
	whitelist := c.securityConfig.GetMREnclaveWhitelist()
	
	// Count active MRENCLAVEs
	activeCount := 0
	for _, entry := range whitelist {
		if entry.Status == StatusActive || entry.Status == StatusApproved {
			activeCount++
		}
	}
	
	return activeCount > 1
}

// IsUpgradeComplete checks if an upgrade has completed
// Upgrade is complete when:
// 1. Whitelist has only one MRENCLAVE, or
// 2. Secret data has synced to UpgradeCompleteBlock
func (c *UpgradeModeChecker) IsUpgradeComplete() bool {
	whitelist := c.securityConfig.GetMREnclaveWhitelist()

	// Count active MRENCLAVEs
	activeCount := 0
	for _, entry := range whitelist {
		if entry.Status == StatusActive || entry.Status == StatusApproved {
			activeCount++
		}
	}

	// Condition 1: Only one MRENCLAVE in whitelist
	if activeCount <= 1 {
		return true
	}

	// Condition 2: Secret data synced to upgrade complete block
	upgradeConfig := c.securityConfig.GetUpgradeConfig()
	syncState := c.securityConfig.GetSecretDataSyncState()
	if upgradeConfig != nil && upgradeConfig.UpgradeCompleteBlock > 0 && syncState != nil {
		if syncState.SyncedBlock >= upgradeConfig.UpgradeCompleteBlock {
			return true
		}
	}

	return false
}

// IsNewVersionNode checks if this node is running the new version
// A new version node has an MRENCLAVE that matches the newest entry in the whitelist
func (c *UpgradeModeChecker) IsNewVersionNode() bool {
	whitelist := c.securityConfig.GetMREnclaveWhitelist()
	if len(whitelist) <= 1 {
		return false
	}

	// Find the newest active MRENCLAVE (highest AddedAt block)
	var newestEntry *MREnclaveEntry
	for i := range whitelist {
		entry := &whitelist[i]
		if entry.Status == StatusActive || entry.Status == StatusApproved {
			if newestEntry == nil || entry.AddedAt > newestEntry.AddedAt {
				newestEntry = entry
			}
		}
	}

	if newestEntry == nil {
		return false
	}

	return c.localMREnclave == newestEntry.MRENCLAVE
}

// ShouldRejectWriteOperation checks if write operations should be rejected
// During upgrade (not complete), new version nodes reject all write operations
func (c *UpgradeModeChecker) ShouldRejectWriteOperation() bool {
	// If upgrade is complete, don't reject
	if c.IsUpgradeComplete() {
		return false
	}

	// If upgrade is in progress and this is a new version node, reject writes
	return c.IsUpgradeInProgress() && c.IsNewVersionNode()
}

// ShouldRejectOldVersionPeer checks if old version peer connections should be rejected
// After upgrade is complete, new nodes only accept peers with matching MRENCLAVE
func (c *UpgradeModeChecker) ShouldRejectOldVersionPeer(peerMREnclave [32]byte) bool {
	// If upgrade is complete and this is a new version node,
	// only accept peers with matching MRENCLAVE
	if c.IsUpgradeComplete() && c.IsNewVersionNode() {
		return peerMREnclave != c.localMREnclave
	}
	return false
}

// ValidateTransaction validates if a transaction can be processed
// During upgrade (not complete), new version nodes reject all signed transactions
func (c *UpgradeModeChecker) ValidateTransaction(tx *types.Transaction) error {
	if c.ShouldRejectWriteOperation() {
		return ErrUpgradeReadOnlyMode
	}
	return nil
}
