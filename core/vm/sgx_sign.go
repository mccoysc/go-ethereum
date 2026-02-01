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

// SGXSign ECDSA 签名预编译合约 (0x8002)
type SGXSign struct{}

// Name returns the name of the contract
func (c *SGXSign) Name() string {
	return "SGXSign"
}

// RequiredGas 计算所需 Gas
// 输入格式: keyID (32 bytes) + hash (32 bytes)
func (c *SGXSign) RequiredGas(input []byte) uint64 {
	return 10000
}

// Run 执行合约（需要上下文）
func (c *SGXSign) Run(input []byte) ([]byte, error) {
	return nil, errors.New("context required")
}

// RunWithContext 带上下文执行
// 输入格式: keyID (32 bytes) + hash (32 bytes)
// 输出格式: signature (65 bytes for ECDSA, 64 bytes for Ed25519)
func (c *SGXSign) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
	// 1. 解析输入
	if len(input) < 64 {
		return nil, errors.New("invalid input: expected keyID (32 bytes) + hash (32 bytes)")
	}
	keyID := common.BytesToHash(input[:32])
	hash := input[32:64]
	
	// 2. 检查签名权限
	if !ctx.PermissionManager.CheckPermission(keyID, ctx.Caller, PermissionSign, ctx.Timestamp) {
		// 检查是否有 Admin 权限
		if !ctx.PermissionManager.CheckPermission(keyID, ctx.Caller, PermissionAdmin, ctx.Timestamp) {
			return nil, errors.New("permission denied: caller does not have Sign or Admin permission")
		}
	}
	
	// 3. 执行签名
	signature, err := ctx.KeyStore.Sign(keyID, hash)
	if err != nil {
		return nil, err
	}
	
	// 4. 使用权限（增加计数）
	_ = ctx.PermissionManager.UsePermission(keyID, ctx.Caller, PermissionSign)
	
	// 5. 返回签名
	return signature, nil
}
