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

package vm

import (
	"github.com/ethereum/go-ethereum/common"
)

// PermissionType represents the type of permission
type PermissionType uint8

const (
	PermissionSign    PermissionType = 0x01 // Signing permission
	PermissionDecrypt PermissionType = 0x02 // Decryption permission
	PermissionDerive  PermissionType = 0x04 // Key derivation permission
	PermissionAdmin   PermissionType = 0x80 // Administrative permission
)

// Permission defines access rights for a key
type Permission struct {
	Grantee   common.Address // Grantee address
	Type      PermissionType // Permission type
	ExpiresAt uint64         // Expiration timestamp (0 means never expires)
	MaxUses   uint64         // Maximum usage count (0 means unlimited)
	UsedCount uint64         // Current usage count
}

// PermissionManager is the interface for managing key permissions
type PermissionManager interface {
	// GrantPermission grants a permission to a grantee
	GrantPermission(keyID common.Hash, permission Permission) error
	
	// RevokePermission revokes a permission from a grantee
	RevokePermission(keyID common.Hash, grantee common.Address, permType PermissionType) error
	
	// CheckPermission checks if a caller has the specified permission
	CheckPermission(keyID common.Hash, caller common.Address, permType PermissionType, timestamp uint64) bool
	
	// GetPermissions retrieves all permissions for a key
	GetPermissions(keyID common.Hash) ([]Permission, error)
	
	// UsePermission records permission usage (increments counter)
	UsePermission(keyID common.Hash, caller common.Address, permType PermissionType) error
}
