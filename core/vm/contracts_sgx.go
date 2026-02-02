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

// SGXPrecompileWithContext is the precompiled contract interface with context support
type SGXPrecompileWithContext interface {
	PrecompiledContract
	
	// RunWithContext executes the contract with SGX context
	RunWithContext(ctx *SGXContext, input []byte) ([]byte, error)
}

// SGXContext represents the SGX execution context
type SGXContext struct {
	// Caller address
	Caller common.Address
	
	// Transaction originator
	Origin common.Address
	
	// Block number
	BlockNumber uint64
	
	// Timestamp
	Timestamp uint64
	
	// ReadOnly indicates if this is a read-only call (eth_call)
	// State-modifying operations must check this and fail if true
	ReadOnly bool
	
	// Key storage
	KeyStore KeyStore
	
	// Permission manager
	PermissionManager PermissionManager
}

// Name returns the name of the contract
func (c *SGXContext) Name() string {
	return "SGXContext"
}
