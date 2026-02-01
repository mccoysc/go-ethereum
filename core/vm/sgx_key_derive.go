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

	"github.com/ethereum/go-ethereum/common"
)

// SGXKeyDerive 密钥派生预编译合约 (0x8008)
type SGXKeyDerive struct{}

// Name returns the name of the contract
func (c *SGXKeyDerive) Name() string {
	return "SGXKeyDerive"
}

// RequiredGas 计算所需 Gas
// 输入格式: parentKeyID (32 bytes) + derivationPath (variable)
func (c *SGXKeyDerive) RequiredGas(input []byte) uint64 {
	return 10000
}

// Run 执行合约（需要上下文）
func (c *SGXKeyDerive) Run(input []byte) ([]byte, error) {
	return nil, errors.New("context required")
}

// RunWithContext 带上下文执行
// 输入格式: parentKeyID (32 bytes) + derivationPath (variable)
// 输出格式: childKeyID (32 bytes)
func (c *SGXKeyDerive) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
	// 1. 解析输入
	if len(input) < 32 {
		return nil, errors.New("invalid input: missing parent key ID")
	}
	parentKeyID := common.BytesToHash(input[:32])
	derivationPath := input[32:]
	
	// 2. 检查派生权限
	if !ctx.PermissionManager.CheckPermission(parentKeyID, ctx.Caller, PermissionDerive, ctx.Timestamp) {
		// 检查是否有 Admin 权限
		if !ctx.PermissionManager.CheckPermission(parentKeyID, ctx.Caller, PermissionAdmin, ctx.Timestamp) {
			return nil, errors.New("permission denied: caller does not have Derive or Admin permission")
		}
	}
	
	// 3. 派生子密钥
	childKeyID, err := ctx.KeyStore.DeriveKey(parentKeyID, derivationPath)
	if err != nil {
		return nil, err
	}
	
	// 4. 使用权限（增加计数）
	_ = ctx.PermissionManager.UsePermission(parentKeyID, ctx.Caller, PermissionDerive)
	
	// 5. 自动授予调用者对子密钥的 Admin 权限
	err = ctx.PermissionManager.GrantPermission(childKeyID, Permission{
		Grantee:   ctx.Caller,
		Type:      PermissionAdmin,
		ExpiresAt: 0,
		MaxUses:   0,
		UsedCount: 0,
	})
	if err != nil {
		return nil, err
	}
	
	// 6. 返回子密钥 ID
	return childKeyID.Bytes(), nil
}
