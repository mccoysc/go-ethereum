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

package storage

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// MockSyncManager for testing AutoMigrationManager
type MockSyncManager struct {
	requestSyncCalled bool
}

func (m *MockSyncManager) RequestSync(peerID common.Hash, secretTypes []SecretDataType) (common.Hash, error) {
	m.requestSyncCalled = true
	return common.BytesToHash([]byte("request-id")), nil
}

func (m *MockSyncManager) HandleSyncRequest(request *SyncRequest) (*SyncResponse, error) {
	return &SyncResponse{}, nil
}

func (m *MockSyncManager) VerifyAndApplySync(response *SyncResponse) error {
	return nil
}

func (m *MockSyncManager) AddPeer(peerID common.Hash, mrenclave [32]byte, quote []byte) error {
	return nil
}

func (m *MockSyncManager) RemovePeer(peerID common.Hash) error {
	return nil
}

func (m *MockSyncManager) GetSyncStatus(peerID common.Hash) (SyncStatus, error) {
	return SyncStatusCompleted, nil
}

func (m *MockSyncManager) StartHeartbeat(ctx context.Context) error {
	return nil
}

func TestNewAutoMigrationManager(t *testing.T) {
	syncManager := &MockSyncManager{}
	securityConfigAddr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	manager, err := NewAutoMigrationManager(syncManager, nil, securityConfigAddr)
	if err != nil {
		t.Fatalf("Failed to create auto migration manager: %v", err)
	}

	if manager == nil {
		t.Fatal("Manager is nil")
	}
}

func TestVerifyPermissionLevel(t *testing.T) {
	syncManager := &MockSyncManager{}
	securityConfigAddr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	manager, err := NewAutoMigrationManager(syncManager, nil, securityConfigAddr)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Test MRENCLAVE not found
	mrenclave := [32]byte{1, 2, 3}
	_, err = manager.VerifyPermissionLevel(mrenclave)
	if err == nil {
		t.Fatal("Expected error for unknown MRENCLAVE")
	}

	// Add permission level
	manager.UpdatePermissionLevel(mrenclave, PermissionBasic)

	// Verify it can be retrieved
	level, err := manager.VerifyPermissionLevel(mrenclave)
	if err != nil {
		t.Fatalf("Failed to verify permission level: %v", err)
	}

	if level != PermissionBasic {
		t.Errorf("Expected %v, got %v", PermissionBasic, level)
	}
}

func TestGetMigrationStatus(t *testing.T) {
	syncManager := &MockSyncManager{}
	securityConfigAddr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	manager, err := NewAutoMigrationManager(syncManager, nil, securityConfigAddr)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	status, err := manager.GetMigrationStatus()
	if err != nil {
		t.Fatalf("Failed to get migration status: %v", err)
	}

	if status == nil {
		t.Fatal("Status is nil")
	}

	if status.InProgress {
		t.Error("Status should not be in progress initially")
	}
}

func TestEnforceMigrationLimit_Basic(t *testing.T) {
	syncManager := &MockSyncManager{}
	securityConfigAddr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	manager, err := NewAutoMigrationManager(syncManager, nil, securityConfigAddr)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Set Basic permission level
	mrenclave := [32]byte{1, 2, 3}
	manager.UpdatePermissionLevel(mrenclave, PermissionBasic)

	// Should be able to migrate initially
	err = manager.EnforceMigrationLimit()
	if err != nil {
		t.Fatalf("Should allow migration initially: %v", err)
	}

	// Simulate migrations up to the limit
	today := time.Now().Format("20060102")
	manager.migrationRecords[today] = &MigrationRecord{
		Timestamp: uint64(time.Now().Unix()),
		Count:     BasicDailyMigrationLimit, // 10
	}

	// Should fail now
	err = manager.EnforceMigrationLimit()
	if err == nil {
		t.Fatal("Expected error when limit exceeded")
	}
}

func TestEnforceMigrationLimit_Standard(t *testing.T) {
	syncManager := &MockSyncManager{}
	securityConfigAddr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	manager, err := NewAutoMigrationManager(syncManager, nil, securityConfigAddr)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Set Standard permission level
	mrenclave := [32]byte{1, 2, 3}
	manager.UpdatePermissionLevel(mrenclave, PermissionStandard)

	// Simulate migrations up to the limit
	today := time.Now().Format("20060102")
	manager.migrationRecords[today] = &MigrationRecord{
		Timestamp: uint64(time.Now().Unix()),
		Count:     StandardDailyMigrationLimit, // 100
	}

	// Should fail
	err = manager.EnforceMigrationLimit()
	if err == nil {
		t.Fatal("Expected error when limit exceeded")
	}
}

func TestEnforceMigrationLimit_Full(t *testing.T) {
	syncManager := &MockSyncManager{}
	securityConfigAddr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	manager, err := NewAutoMigrationManager(syncManager, nil, securityConfigAddr)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Set Full permission level (no limit)
	mrenclave := [32]byte{1, 2, 3}
	manager.UpdatePermissionLevel(mrenclave, PermissionFull)

	// Simulate many migrations
	today := time.Now().Format("20060102")
	manager.migrationRecords[today] = &MigrationRecord{
		Timestamp: uint64(time.Now().Unix()),
		Count:     1000, // Way over Basic and Standard limits
	}

	// Should still pass
	err = manager.EnforceMigrationLimit()
	if err != nil {
		t.Fatalf("Full permission should have no limit: %v", err)
	}
}

func TestCheckAndMigrate(t *testing.T) {
	syncManager := &MockSyncManager{}
	securityConfigAddr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	manager, err := NewAutoMigrationManager(syncManager, nil, securityConfigAddr)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Set upgrade complete block
	manager.SetUpgradeCompleteBlock(1000)

	// Set permission level
	mrenclave := [32]byte{1, 2, 3}
	manager.UpdatePermissionLevel(mrenclave, PermissionFull)

	// Check and migrate
	migrated, err := manager.CheckAndMigrate()
	if err != nil {
		t.Fatalf("Failed to check and migrate: %v", err)
	}

	if !migrated {
		t.Error("Expected migration to occur")
	}

	// Verify status was updated
	status, _ := manager.GetMigrationStatus()
	if status.MigrationCount != 1 {
		t.Errorf("Expected 1 migration, got %d", status.MigrationCount)
	}
}

func TestStartMonitoring(t *testing.T) {
	syncManager := &MockSyncManager{}
	securityConfigAddr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	manager, err := NewAutoMigrationManager(syncManager, nil, securityConfigAddr)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start monitoring
	err = manager.StartMonitoring(ctx)
	if err != nil {
		t.Fatalf("Failed to start monitoring: %v", err)
	}

	// Try to start again (should fail)
	err = manager.StartMonitoring(ctx)
	if err == nil {
		t.Fatal("Expected error when starting monitoring twice")
	}

	// Cancel context to stop monitoring
	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestSetUpgradeCompleteBlock(t *testing.T) {
	syncManager := &MockSyncManager{}
	securityConfigAddr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	manager, err := NewAutoMigrationManager(syncManager, nil, securityConfigAddr)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	blockNumber := uint64(12345)
	manager.SetUpgradeCompleteBlock(blockNumber)

	if manager.upgradeCompleteBlock != blockNumber {
		t.Errorf("Expected block %d, got %d", blockNumber, manager.upgradeCompleteBlock)
	}
}
