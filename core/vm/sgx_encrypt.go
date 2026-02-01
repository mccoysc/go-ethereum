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

// SGXEncrypt 对称加密预编译合约 (0x8006)
type SGXEncrypt struct{}

// Name returns the name of the contract
func (c *SGXEncrypt) Name() string {
	return "SGXEncrypt"
}

// RequiredGas 计算所需 Gas
// 输入格式: keyID (32 bytes) + plaintext (variable)
func (c *SGXEncrypt) RequiredGas(input []byte) uint64 {
	if len(input) < 32 {
		return 5000
	}
	
	plaintextLen := uint64(len(input) - 32)
	return 5000 + (plaintextLen * 10)
}

// Run 执行合约（需要上下文）
func (c *SGXEncrypt) Run(input []byte) ([]byte, error) {
	return nil, errors.New("context required")
}

// RunWithContext 带上下文执行
// 输入格式: keyID (32 bytes) + plaintext (variable)
// 输出格式: nonce (12 bytes) + ciphertext + tag (16 bytes)
func (c *SGXEncrypt) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
	// 1. 解析输入
	if len(input) < 32 {
		return nil, errors.New("invalid input: missing key ID")
	}
	keyID := common.BytesToHash(input[:32])
	plaintext := input[32:]
	
	// 2. 检查密钥元数据（确保是 AES 密钥）
	metadata, err := ctx.KeyStore.GetMetadata(keyID)
	if err != nil {
		return nil, err
	}
	if metadata.KeyType != KeyTypeAES256 {
		return nil, errors.New("key type must be AES256 for encryption")
	}
	
	// 3. 执行加密（不需要权限检查，加密是公开操作）
	ciphertext, err := ctx.KeyStore.Encrypt(keyID, plaintext)
	if err != nil {
		return nil, err
	}
	
	// 4. 返回密文（包含 nonce + ciphertext + tag）
	return ciphertext, nil
}
