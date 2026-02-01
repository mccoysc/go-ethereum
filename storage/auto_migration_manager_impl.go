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
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// MigrationRecord tracks migration operations
type MigrationRecord struct {
	Timestamp uint64
	Count     uint64
}

// AutoMigrationManagerImpl implements AutoMigrationManager
type AutoMigrationManagerImpl struct {
	mu                    sync.RWMutex
	syncManager           SyncManager
	client                *ethclient.Client
	securityConfigAddress common.Address
	upgradeCompleteBlock  uint64
	permissionLevels      map[[32]byte]PermissionLevel
	migrationRecords      map[string]*MigrationRecord // key: YYYYMMDD
	status                *MigrationStatus
	monitoringRunning     bool
}

// NewAutoMigrationManager creates a new auto migration manager
func NewAutoMigrationManager(
	syncManager SyncManager,
	client *ethclient.Client,
	securityConfigAddress common.Address,
) (*AutoMigrationManagerImpl, error) {
	return &AutoMigrationManagerImpl{
		syncManager:           syncManager,
		client:                client,
		securityConfigAddress: securityConfigAddress,
		permissionLevels:      make(map[[32]byte]PermissionLevel),
		migrationRecords:      make(map[string]*MigrationRecord),
		status: &MigrationStatus{
			InProgress: false,
		},
	}, nil
}

// StartMonitoring starts monitoring for migration triggers
func (amm *AutoMigrationManagerImpl) StartMonitoring(ctx context.Context) error {
	amm.mu.Lock()
	if amm.monitoringRunning {
		amm.mu.Unlock()
		return fmt.Errorf("monitoring already running")
	}
	amm.monitoringRunning = true
	amm.mu.Unlock()

	go amm.monitoringLoop(ctx)
	return nil
}

// monitoringLoop runs the monitoring loop
func (amm *AutoMigrationManagerImpl) monitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			amm.mu.Lock()
			amm.monitoringRunning = false
			amm.mu.Unlock()
			return
		case <-ticker.C:
			// Check if migration is needed
			amm.CheckAndMigrate()
		}
	}
}

// CheckAndMigrate checks if migration is needed and performs it
func (amm *AutoMigrationManagerImpl) CheckAndMigrate() (bool, error) {
	amm.mu.Lock()
	defer amm.mu.Unlock()

	if amm.status.InProgress {
		return false, nil
	}

	// Check if we're before upgrade complete block
	if amm.upgradeCompleteBlock > 0 {
		// In production, would check current block height
		// For now, assume we need to migrate
		return amm.performMigration()
	}

	return false, nil
}

// performMigration performs the actual migration
func (amm *AutoMigrationManagerImpl) performMigration() (bool, error) {
	// Check migration limit
	if err := amm.enforceMigrationLimitInternal(); err != nil {
		return false, err
	}

	amm.status.InProgress = true
	amm.status.LastMigrationTime = uint64(time.Now().Unix())

	// Perform migration (simplified - would sync with target node)
	// This would call syncManager.RequestSync with appropriate peers

	amm.status.MigrationCount++
	amm.status.InProgress = false

	// Update daily record
	today := time.Now().Format("20060102")
	if record, exists := amm.migrationRecords[today]; exists {
		record.Count++
	} else {
		amm.migrationRecords[today] = &MigrationRecord{
			Timestamp: uint64(time.Now().Unix()),
			Count:     1,
		}
	}

	return true, nil
}

// GetMigrationStatus returns the current migration status
func (amm *AutoMigrationManagerImpl) GetMigrationStatus() (*MigrationStatus, error) {
	amm.mu.RLock()
	defer amm.mu.RUnlock()

	// Return a copy
	status := *amm.status
	return &status, nil
}

// VerifyPermissionLevel verifies the permission level for a given MRENCLAVE
func (amm *AutoMigrationManagerImpl) VerifyPermissionLevel(mrenclave [32]byte) (PermissionLevel, error) {
	amm.mu.RLock()
	defer amm.mu.RUnlock()

	level, exists := amm.permissionLevels[mrenclave]
	if !exists {
		return 0, fmt.Errorf("MRENCLAVE not found in permission levels")
	}

	return level, nil
}

// UpdatePermissionLevel updates the permission level for an MRENCLAVE
func (amm *AutoMigrationManagerImpl) UpdatePermissionLevel(mrenclave [32]byte, level PermissionLevel) {
	amm.mu.Lock()
	defer amm.mu.Unlock()

	amm.permissionLevels[mrenclave] = level
}

// EnforceMigrationLimit enforces the migration frequency limit
func (amm *AutoMigrationManagerImpl) EnforceMigrationLimit() error {
	amm.mu.Lock()
	defer amm.mu.Unlock()

	return amm.enforceMigrationLimitInternal()
}

// enforceMigrationLimitInternal is the internal version without locking
func (amm *AutoMigrationManagerImpl) enforceMigrationLimitInternal() error {
	// Get today's migration count
	today := time.Now().Format("20060102")
	record, exists := amm.migrationRecords[today]
	if !exists {
		return nil // No migrations today, ok to proceed
	}

	// Determine limit based on permission level
	// Use the lowest permission level to determine the limit (most restrictive)
	var lowestLevel PermissionLevel = PermissionFull

	for _, lvl := range amm.permissionLevels {
		if lowestLevel == PermissionFull || lvl < lowestLevel {
			lowestLevel = lvl
		}
	}

	maxLimit := amm.getDailyLimit(lowestLevel)

	if maxLimit > 0 && record.Count >= maxLimit {
		return fmt.Errorf("daily migration limit exceeded: %d/%d", record.Count, maxLimit)
	}

	return nil
}

// getDailyLimit returns the daily migration limit for a permission level
func (amm *AutoMigrationManagerImpl) getDailyLimit(level PermissionLevel) uint64 {
	switch level {
	case PermissionBasic:
		return BasicDailyMigrationLimit // 10
	case PermissionStandard:
		return StandardDailyMigrationLimit // 100
	case PermissionFull:
		return 0 // No limit
	default:
		return 0
	}
}

// SetUpgradeCompleteBlock sets the upgrade complete block
func (amm *AutoMigrationManagerImpl) SetUpgradeCompleteBlock(blockNumber uint64) {
	amm.mu.Lock()
	defer amm.mu.Unlock()

	amm.upgradeCompleteBlock = blockNumber
}
