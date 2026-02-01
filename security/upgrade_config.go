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

package security

// UpgradeConfig holds upgrade configuration parameters
type UpgradeConfig struct {
	// NewMREnclave is the new version MRENCLAVE
	NewMREnclave [32]byte

	// UpgradeCompleteBlock is the block height when upgrade is considered complete
	UpgradeCompleteBlock uint64

	// UpgradeStartBlock is the block height when upgrade started
	UpgradeStartBlock uint64
}

// SecretDataSyncState tracks the secret data synchronization state
type SecretDataSyncState struct {
	// SyncedBlock is the block height that has been synced
	SyncedBlock uint64

	// SyncComplete indicates if synchronization is complete
	SyncComplete bool

	// LastSyncTime is the last synchronization timestamp
	LastSyncTime int64
}

// SecurityConfigContract is the interface for security configuration contract
type SecurityConfigContract interface {
	// GetUpgradeConfig returns the current upgrade configuration
	GetUpgradeConfig() *UpgradeConfig

	// SetUpgradeConfig sets the upgrade configuration (only callable by governance)
	SetUpgradeConfig(config *UpgradeConfig) error
}
