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
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
)

// SGXRandom 安全随机数生成预编译合约 (0x8005)
type SGXRandom struct{}

// Name returns the name of the contract
func (c *SGXRandom) Name() string {
	return "SGXRandom"
}

// RequiredGas 计算所需 Gas
// 输入格式: length (32 bytes)
func (c *SGXRandom) RequiredGas(input []byte) uint64 {
	if len(input) < 32 {
		return 1000
	}
	
	// 解析长度
	length := binary.BigEndian.Uint64(input[24:32])
	
	// 基础成本 + 每字节成本
	return 1000 + (length * 100)
}

// Run 执行合约（不需要上下文）
// 输入格式: length (32 bytes)
// 输出格式: randomBytes (variable length)
func (c *SGXRandom) Run(input []byte) ([]byte, error) {
	// 1. 解析输入
	if len(input) < 32 {
		return nil, errors.New("invalid input: missing length")
	}
	
	// 提取长度（big-endian uint256）
	length := binary.BigEndian.Uint64(input[24:32])
	
	// 2. 验证长度（限制最大 1MB）
	if length > 1024*1024 {
		return nil, errors.New("requested length too large (max 1MB)")
	}
	if length == 0 {
		return nil, errors.New("requested length must be greater than 0")
	}
	
	// 3. 生成随机数
	randomBytes := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, randomBytes); err != nil {
		return nil, err
	}
	
	// 4. 返回随机数
	return randomBytes, nil
}
