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
	"testing"

	"github.com/ethereum/go-ethereum/security"
)

func TestProgressivePermissionManager_GetPermissionLevel(t *testing.T) {
	config := DefaultProgressivePermissionConfig()
	pm := NewProgressivePermissionManager(config)

	mrenclave := [32]byte{1, 2, 3}

	// Default permission level
	level := pm.GetPermissionLevel(mrenclave, 1000)
	if level != PermissionBasic {
		t.Errorf("expected default permission %v, got %v", PermissionBasic, level)
	}

	// Activate node
	pm.ActivateNode(mrenclave, 100)

	// Should still be basic
	level = pm.GetPermissionLevel(mrenclave, 1000)
	if level != PermissionBasic {
		t.Errorf("expected permission %v, got %v", PermissionBasic, level)
	}
}

func TestProgressivePermissionManager_CheckUpgrade_ToStandard(t *testing.T) {
	config := DefaultProgressivePermissionConfig()
	pm := NewProgressivePermissionManager(config)

	mrenclave := [32]byte{1, 2, 3}
	pm.ActivateNode(mrenclave, 100)

	// Too early to upgrade (before duration)
	currentBlock := 100 + config.BasicDuration - 1
	upgraded, _ := pm.CheckUpgrade(mrenclave, currentBlock, 0.96)
	if upgraded {
		t.Error("should not upgrade before duration met")
	}

	// After duration with good uptime - should upgrade
	currentBlock = 100 + config.BasicDuration + 1
	upgraded, level := pm.CheckUpgrade(mrenclave, currentBlock, 0.96)
	if !upgraded {
		t.Error("should upgrade to standard")
	}
	if level != PermissionStandard {
		t.Errorf("expected level %v, got %v", PermissionStandard, level)
	}

	// Verify permission level persists
	finalLevel := pm.GetPermissionLevel(mrenclave, currentBlock+100)
	if finalLevel != PermissionStandard {
		t.Errorf("expected level %v to persist, got %v", PermissionStandard, finalLevel)
	}
}

func TestProgressivePermissionManager_CheckUpgrade_ToFull(t *testing.T) {
	config := DefaultProgressivePermissionConfig()
	pm := NewProgressivePermissionManager(config)

	mrenclave := [32]byte{1, 2, 3}
	pm.ActivateNode(mrenclave, 100)

	// Upgrade to standard first with high uptime (only one uptime reading of 0.99)
	currentBlock := 100 + config.BasicDuration + 1
	upgraded, level := pm.CheckUpgrade(mrenclave, currentBlock, 0.99)
	if !upgraded {
		t.Error("should upgrade to standard")
	}
	if level != PermissionStandard {
		t.Errorf("expected upgrade to standard level %v, got %v", PermissionStandard, level)
	}

	// After enough duration for full, add another high uptime reading
	// Average should be (0.99 + 0.99) / 2 = 0.99
	currentBlock = 100 + config.BasicDuration + config.StandardDuration + 1
	upgraded, level = pm.CheckUpgrade(mrenclave, currentBlock, 0.99)
	if !upgraded {
		t.Error("should upgrade to full")
	}
	if level != PermissionFull {
		t.Errorf("expected level %v, got %v", PermissionFull, level)
	}
}

func TestProgressivePermissionManager_Downgrade(t *testing.T) {
	config := DefaultProgressivePermissionConfig()
	pm := NewProgressivePermissionManager(config)

	mrenclave := [32]byte{1, 2, 3}
	pm.ActivateNode(mrenclave, 100)

	// Upgrade to standard
	currentBlock := 100 + config.BasicDuration + 1
	pm.CheckUpgrade(mrenclave, currentBlock, 0.96)

	// Verify standard permission
	perm, _ := pm.GetNodePermission(mrenclave)
	if perm.CurrentLevel != PermissionStandard {
		t.Error("should be at standard level")
	}

	// Downgrade
	pm.Downgrade(mrenclave, "misbehavior")

	// Verify downgraded to basic
	perm, _ = pm.GetNodePermission(mrenclave)
	if perm.CurrentLevel != PermissionBasic {
		t.Errorf("expected level %v after downgrade, got %v", PermissionBasic, perm.CurrentLevel)
	}

	// History should be reset
	if len(perm.UptimeHistory) != 0 {
		t.Error("uptime history should be reset after downgrade")
	}
}

func TestProgressivePermissionManager_ActivateNode(t *testing.T) {
	config := DefaultProgressivePermissionConfig()
	pm := NewProgressivePermissionManager(config)

	mrenclave := [32]byte{1, 2, 3}

	// Not activated yet
	_, exists := pm.GetNodePermission(mrenclave)
	if exists {
		t.Error("node should not be activated yet")
	}

	// Activate
	pm.ActivateNode(mrenclave, 1000)

	// Should exist now
	perm, exists := pm.GetNodePermission(mrenclave)
	if !exists {
		t.Error("node should be activated")
	}
	if perm.CurrentLevel != PermissionBasic {
		t.Error("should start at basic level")
	}
	if perm.ActivatedAt != 1000 {
		t.Error("activation block not set correctly")
	}
}

// MockSecurityConfigReader for testing
type MockSecurityConfigReader struct {
	whitelist []MREnclaveEntry
	upgrade   *security.UpgradeConfig
	syncState *security.SecretDataSyncState
}

func (m *MockSecurityConfigReader) GetMREnclaveWhitelist() []MREnclaveEntry {
	return m.whitelist
}

func (m *MockSecurityConfigReader) GetUpgradeConfig() *security.UpgradeConfig {
	return m.upgrade
}

func (m *MockSecurityConfigReader) GetSecretDataSyncState() *security.SecretDataSyncState {
	return m.syncState
}

func TestUpgradeModeChecker_IsUpgradeInProgress(t *testing.T) {
	localMR := [32]byte{1, 2, 3}

	// Single MRENCLAVE - no upgrade
	config := &MockSecurityConfigReader{
		whitelist: []MREnclaveEntry{
			{MRENCLAVE: localMR, Status: StatusActive},
		},
	}
	checker := NewUpgradeModeChecker(config, localMR)
	if checker.IsUpgradeInProgress() {
		t.Error("should not be in upgrade with single MRENCLAVE")
	}

	// Multiple MRENCLAVEs - upgrade in progress
	newMR := [32]byte{4, 5, 6}
	config.whitelist = []MREnclaveEntry{
		{MRENCLAVE: localMR, Status: StatusActive},
		{MRENCLAVE: newMR, Status: StatusActive},
	}
	if !checker.IsUpgradeInProgress() {
		t.Error("should be in upgrade with multiple MRENCLAVEs")
	}
}

func TestUpgradeModeChecker_IsUpgradeComplete(t *testing.T) {
	localMR := [32]byte{1, 2, 3}
	newMR := [32]byte{4, 5, 6}

	// Test 1: Only one MRENCLAVE - upgrade complete
	config := &MockSecurityConfigReader{
		whitelist: []MREnclaveEntry{
			{MRENCLAVE: localMR, Status: StatusActive},
		},
	}
	checker := NewUpgradeModeChecker(config, localMR)
	if !checker.IsUpgradeComplete() {
		t.Error("should be complete with single MRENCLAVE")
	}

	// Test 2: Multiple MRENCLAVEs but synced to upgrade complete block
	config.whitelist = []MREnclaveEntry{
		{MRENCLAVE: localMR, Status: StatusActive, AddedAt: 100},
		{MRENCLAVE: newMR, Status: StatusActive, AddedAt: 200},
	}
	config.upgrade = &security.UpgradeConfig{
		UpgradeCompleteBlock: 1000,
	}
	config.syncState = &security.SecretDataSyncState{
		SyncedBlock: 1000,
	}
	
	if !checker.IsUpgradeComplete() {
		t.Error("should be complete when synced to upgrade block")
	}

	// Test 3: Multiple MRENCLAVEs and not synced yet
	config.syncState.SyncedBlock = 500
	if checker.IsUpgradeComplete() {
		t.Error("should not be complete when not synced")
	}
}

func TestUpgradeModeChecker_IsNewVersionNode(t *testing.T) {
	oldMR := [32]byte{1, 2, 3}
	newMR := [32]byte{4, 5, 6}

	config := &MockSecurityConfigReader{
		whitelist: []MREnclaveEntry{
			{MRENCLAVE: oldMR, Status: StatusActive, AddedAt: 100},
			{MRENCLAVE: newMR, Status: StatusActive, AddedAt: 200},
		},
	}

	// Old version node
	checker := NewUpgradeModeChecker(config, oldMR)
	if checker.IsNewVersionNode() {
		t.Error("should not be new version node")
	}

	// New version node
	checker = NewUpgradeModeChecker(config, newMR)
	if !checker.IsNewVersionNode() {
		t.Error("should be new version node")
	}
}

func TestUpgradeModeChecker_ShouldRejectWriteOperation(t *testing.T) {
	oldMR := [32]byte{1, 2, 3}
	newMR := [32]byte{4, 5, 6}

	// Upgrade in progress
	config := &MockSecurityConfigReader{
		whitelist: []MREnclaveEntry{
			{MRENCLAVE: oldMR, Status: StatusActive, AddedAt: 100},
			{MRENCLAVE: newMR, Status: StatusActive, AddedAt: 200},
		},
		upgrade: &security.UpgradeConfig{
			UpgradeCompleteBlock: 1000,
		},
		syncState: &security.SecretDataSyncState{
			SyncedBlock: 500, // Not yet synced
		},
	}

	// New version node should reject writes during upgrade
	checker := NewUpgradeModeChecker(config, newMR)
	if !checker.ShouldRejectWriteOperation() {
		t.Error("new version node should reject writes during upgrade")
	}

	// Old version node should not reject writes
	checker = NewUpgradeModeChecker(config, oldMR)
	if checker.ShouldRejectWriteOperation() {
		t.Error("old version node should not reject writes")
	}

	// After upgrade complete, new version should not reject
	config.syncState.SyncedBlock = 1000
	checker = NewUpgradeModeChecker(config, newMR)
	if checker.ShouldRejectWriteOperation() {
		t.Error("should not reject writes after upgrade complete")
	}
}

func TestUpgradeModeChecker_ShouldRejectOldVersionPeer(t *testing.T) {
	oldMR := [32]byte{1, 2, 3}
	newMR := [32]byte{4, 5, 6}

	// Upgrade complete
	config := &MockSecurityConfigReader{
		whitelist: []MREnclaveEntry{
			{MRENCLAVE: oldMR, Status: StatusActive, AddedAt: 100},
			{MRENCLAVE: newMR, Status: StatusActive, AddedAt: 200},
		},
		upgrade: &security.UpgradeConfig{
			UpgradeCompleteBlock: 1000,
		},
		syncState: &security.SecretDataSyncState{
			SyncedBlock: 1000,
		},
	}

	// New version node should reject old version peers after upgrade
	checker := NewUpgradeModeChecker(config, newMR)
	if !checker.ShouldRejectOldVersionPeer(oldMR) {
		t.Error("new version node should reject old version peers after upgrade")
	}

	// Should not reject peers with same MRENCLAVE
	if checker.ShouldRejectOldVersionPeer(newMR) {
		t.Error("should not reject peers with same MRENCLAVE")
	}

	// Old version node should not reject (it's being phased out anyway)
	checker = NewUpgradeModeChecker(config, oldMR)
	if checker.ShouldRejectOldVersionPeer(newMR) {
		t.Error("old version node is not considered new, so returns false")
	}
}
