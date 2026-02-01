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
	"errors"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// InMemoryPermissionManager 实现 PermissionManager 接口
type InMemoryPermissionManager struct {
	permissions map[common.Hash][]Permission // keyID -> permissions
	mutex       sync.RWMutex                  // 保护 permissions
}

// NewInMemoryPermissionManager 创建新的权限管理器
func NewInMemoryPermissionManager() *InMemoryPermissionManager {
	return &InMemoryPermissionManager{
		permissions: make(map[common.Hash][]Permission),
	}
}

// GrantPermission 授予权限
func (pm *InMemoryPermissionManager) GrantPermission(keyID common.Hash, permission Permission) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	
	// 检查是否已存在相同的权限
	perms := pm.permissions[keyID]
	for i, p := range perms {
		if p.Grantee == permission.Grantee && p.Type == permission.Type {
			// 更新现有权限
			perms[i] = permission
			pm.permissions[keyID] = perms
			return nil
		}
	}
	
	// 添加新权限
	pm.permissions[keyID] = append(perms, permission)
	return nil
}

// RevokePermission 撤销权限
func (pm *InMemoryPermissionManager) RevokePermission(keyID common.Hash, grantee common.Address, permType PermissionType) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	
	perms := pm.permissions[keyID]
	newPerms := make([]Permission, 0, len(perms))
	
	found := false
	for _, p := range perms {
		if p.Grantee == grantee && p.Type == permType {
			found = true
			continue
		}
		newPerms = append(newPerms, p)
	}
	
	if !found {
		return errors.New("permission not found")
	}
	
	pm.permissions[keyID] = newPerms
	return nil
}

// CheckPermission 检查权限
func (pm *InMemoryPermissionManager) CheckPermission(keyID common.Hash, caller common.Address, permType PermissionType, timestamp uint64) bool {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	perms := pm.permissions[keyID]
	for _, p := range perms {
		if p.Grantee != caller {
			continue
		}
		if p.Type&permType == 0 {
			continue
		}
		
		// 检查过期时间
		if p.ExpiresAt > 0 && timestamp > p.ExpiresAt {
			continue
		}
		
		// 检查使用次数
		if p.MaxUses > 0 && p.UsedCount >= p.MaxUses {
			continue
		}
		
		return true
	}
	
	return false
}

// GetPermissions 获取所有权限
func (pm *InMemoryPermissionManager) GetPermissions(keyID common.Hash) ([]Permission, error) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	perms := pm.permissions[keyID]
	result := make([]Permission, len(perms))
	copy(result, perms)
	
	return result, nil
}

// UsePermission 使用权限（增加计数）
func (pm *InMemoryPermissionManager) UsePermission(keyID common.Hash, caller common.Address, permType PermissionType) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	
	perms := pm.permissions[keyID]
	for i, p := range perms {
		if p.Grantee != caller {
			continue
		}
		if p.Type&permType == 0 {
			continue
		}
		
		// 增加使用计数
		perms[i].UsedCount++
		pm.permissions[keyID] = perms
		return nil
	}
	
	return fmt.Errorf("permission not found for caller %s and type %d", caller.Hex(), permType)
}
