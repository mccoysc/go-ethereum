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

// SGXECDH ECDH 密钥交换预编译合约 (0x8004)
type SGXECDH struct{}

// Name returns the name of the contract
func (c *SGXECDH) Name() string {
	return "SGXECDH"
}

// RequiredGas 计算所需 Gas
// 输入格式: keyID (32 bytes) + peerPubKey (64 bytes)
func (c *SGXECDH) RequiredGas(input []byte) uint64 {
	return 20000
}

// Run 执行合约（需要上下文）
func (c *SGXECDH) Run(input []byte) ([]byte, error) {
	return nil, errors.New("context required")
}

// RunWithContext 带上下文执行
// 输入格式: keyID (32 bytes) + peerPubKey (64 bytes)
// 输出格式: sharedSecret (32 bytes)
func (c *SGXECDH) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
	// 1. 解析输入
	if len(input) < 96 {
		return nil, errors.New("invalid input: expected keyID (32 bytes) + peerPubKey (64 bytes)")
	}
	keyID := common.BytesToHash(input[:32])
	peerPubKey := input[32:96]
	
	// 2. 检查派生权限
	if !ctx.PermissionManager.CheckPermission(keyID, ctx.Caller, PermissionDerive, ctx.Timestamp) {
		// 检查是否有 Admin 权限
		if !ctx.PermissionManager.CheckPermission(keyID, ctx.Caller, PermissionAdmin, ctx.Timestamp) {
			return nil, errors.New("permission denied: caller does not have Derive or Admin permission")
		}
	}
	
	// 3. 执行 ECDH
	sharedSecret, err := ctx.KeyStore.ECDH(keyID, peerPubKey)
	if err != nil {
		return nil, err
	}
	
	// 4. 使用权限（增加计数）
	_ = ctx.PermissionManager.UsePermission(keyID, ctx.Caller, PermissionDerive)
	
	// 5. 返回共享密钥
	return sharedSecret, nil
}
