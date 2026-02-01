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

// SGXDecrypt 对称解密预编译合约 (0x8007)
type SGXDecrypt struct{}

// Name returns the name of the contract
func (c *SGXDecrypt) Name() string {
	return "SGXDecrypt"
}

// RequiredGas 计算所需 Gas
// 输入格式: keyID (32 bytes) + ciphertext (variable)
func (c *SGXDecrypt) RequiredGas(input []byte) uint64 {
	if len(input) < 32 {
		return 5000
	}
	
	ciphertextLen := uint64(len(input) - 32)
	return 5000 + (ciphertextLen * 10)
}

// Run 执行合约（需要上下文）
func (c *SGXDecrypt) Run(input []byte) ([]byte, error) {
	return nil, errors.New("context required")
}

// RunWithContext 带上下文执行
// 输入格式: keyID (32 bytes) + ciphertext (variable: nonce + encrypted + tag)
// 输出格式: plaintext (variable)
func (c *SGXDecrypt) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
	// 1. 解析输入
	if len(input) < 32 {
		return nil, errors.New("invalid input: missing key ID")
	}
	keyID := common.BytesToHash(input[:32])
	ciphertext := input[32:]
	
	// 2. 检查解密权限
	if !ctx.PermissionManager.CheckPermission(keyID, ctx.Caller, PermissionDecrypt, ctx.Timestamp) {
		// 检查是否有 Admin 权限
		if !ctx.PermissionManager.CheckPermission(keyID, ctx.Caller, PermissionAdmin, ctx.Timestamp) {
			return nil, errors.New("permission denied: caller does not have Decrypt or Admin permission")
		}
	}
	
	// 3. 检查密钥元数据（确保是 AES 密钥）
	metadata, err := ctx.KeyStore.GetMetadata(keyID)
	if err != nil {
		return nil, err
	}
	if metadata.KeyType != KeyTypeAES256 {
		return nil, errors.New("key type must be AES256 for decryption")
	}
	
	// 4. 执行解密
	plaintext, err := ctx.KeyStore.Decrypt(keyID, ciphertext)
	if err != nil {
		return nil, err
	}
	
	// 5. 使用权限（增加计数）
	_ = ctx.PermissionManager.UsePermission(keyID, ctx.Caller, PermissionDecrypt)
	
	// 6. 返回明文
	return plaintext, nil
}
