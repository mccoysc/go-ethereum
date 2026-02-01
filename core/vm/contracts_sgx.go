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

// SGXPrecompileWithContext 带上下文的预编译合约接口
type SGXPrecompileWithContext interface {
	PrecompiledContract
	
	// RunWithContext 带上下文执行
	RunWithContext(ctx *SGXContext, input []byte) ([]byte, error)
}

// SGXContext SGX 执行上下文
type SGXContext struct {
	// 调用者地址
	Caller common.Address
	
	// 交易发起者
	Origin common.Address
	
	// 区块号
	BlockNumber uint64
	
	// 时间戳
	Timestamp uint64
	
	// 密钥存储
	KeyStore KeyStore
	
	// 权限管理器
	PermissionManager PermissionManager
}

// Name returns the name of the contract
func (c *SGXContext) Name() string {
	return "SGXContext"
}
