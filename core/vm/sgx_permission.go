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

// PermissionType 权限类型
type PermissionType uint8

const (
	PermissionSign    PermissionType = 0x01 // 签名权限
	PermissionDecrypt PermissionType = 0x02 // 解密权限
	PermissionDerive  PermissionType = 0x04 // 派生权限
	PermissionAdmin   PermissionType = 0x80 // 管理权限
)

// Permission 权限定义
type Permission struct {
	Grantee   common.Address // 被授权者
	Type      PermissionType // 权限类型
	ExpiresAt uint64         // 过期时间（0 表示永不过期）
	MaxUses   uint64         // 最大使用次数（0 表示无限制）
	UsedCount uint64         // 已使用次数
}

// PermissionManager 权限管理器接口
type PermissionManager interface {
	// GrantPermission 授予权限
	GrantPermission(keyID common.Hash, permission Permission) error
	
	// RevokePermission 撤销权限
	RevokePermission(keyID common.Hash, grantee common.Address, permType PermissionType) error
	
	// CheckPermission 检查权限
	CheckPermission(keyID common.Hash, caller common.Address, permType PermissionType, timestamp uint64) bool
	
	// GetPermissions 获取所有权限
	GetPermissions(keyID common.Hash) ([]Permission, error)
	
	// UsePermission 使用权限（增加计数）
	UsePermission(keyID common.Hash, caller common.Address, permType PermissionType) error
}
