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

// StorageConfig defines configuration for the storage module
type StorageConfig struct {
	EncryptedPath string // 加密分区路径（Manifest）
	DataPath      string // 普通数据路径
	SecretPath    string // 秘密数据路径（Manifest）
	CacheSize     int    // 缓存大小（运行时）
	SyncInterval  int    // 同步间隔（运行时）
}

// SecretDataType defines the type of secret data
type SecretDataType uint8

const (
	SecretTypePrivateKey   SecretDataType = 0x01
	SecretTypeSealingKey   SecretDataType = 0x02
	SecretTypeNodeIdentity SecretDataType = 0x03
	SecretTypeSharedSecret SecretDataType = 0x04
)

// SecretData represents a secret data entry
type SecretData struct {
	Type      SecretDataType
	ID        []byte
	Data      []byte
	CreatedAt uint64
	ExpiresAt uint64
	Metadata  map[string]string
}

// PermissionLevel defines the permission level for MRENCLAVE
type PermissionLevel uint8

const (
	PermissionBasic    PermissionLevel = 0x01 // 基础权限（7天）
	PermissionStandard PermissionLevel = 0x02 // 标准权限（30天）
	PermissionFull     PermissionLevel = 0x03 // 完全权限（永久）
)

// 权限级别对应的迁移限制
const (
	BasicDailyMigrationLimit    = 10  // Basic 权限每日迁移限制
	StandardDailyMigrationLimit = 100 // Standard 权限每日迁移限制
)

// SyncStatus represents the status of a synchronization operation
type SyncStatus uint8

const (
	SyncStatusPending    SyncStatus = 0x01
	SyncStatusInProgress SyncStatus = 0x02
	SyncStatusCompleted  SyncStatus = 0x03
	SyncStatusFailed     SyncStatus = 0x04
)

// MigrationStatus represents the status of a migration operation
type MigrationStatus struct {
	InProgress        bool
	LastMigrationTime uint64
	MigrationCount    uint64
	TargetMREnclave   [32]byte
	SecretsSynced     uint64
}
