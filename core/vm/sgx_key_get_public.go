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

// SGXKeyGetPublic 获取公钥预编译合约 (0x8001)
type SGXKeyGetPublic struct{}

// Name returns the name of the contract
func (c *SGXKeyGetPublic) Name() string {
	return "SGXKeyGetPublic"
}

// RequiredGas 计算所需 Gas
// 输入格式: keyID (32 bytes)
func (c *SGXKeyGetPublic) RequiredGas(input []byte) uint64 {
	return 3000
}

// Run 执行合约（需要上下文）
func (c *SGXKeyGetPublic) Run(input []byte) ([]byte, error) {
	return nil, errors.New("context required")
}

// RunWithContext 带上下文执行
// 输入格式: keyID (32 bytes)
// 输出格式: publicKey (variable length)
func (c *SGXKeyGetPublic) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
	// 1. 解析输入
	if len(input) < 32 {
		return nil, errors.New("invalid input: missing key ID")
	}
	keyID := common.BytesToHash(input[:32])
	
	// 2. 获取公钥（不需要权限检查，公钥是公开的）
	pubKey, err := ctx.KeyStore.GetPublicKey(keyID)
	if err != nil {
		return nil, err
	}
	
	// 3. 返回公钥
	return pubKey, nil
}
