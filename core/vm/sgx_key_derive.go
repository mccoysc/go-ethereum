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

// SGXKeyDerive is the precompiled contract for key derivation (0x8008)
type SGXKeyDerive struct{}

// Name returns the name of the contract
func (c *SGXKeyDerive) Name() string {
	return "SGXKeyDerive"
}

// RequiredGas calculates the required gas
// Input format: parentKeyID (32 bytes) + derivationPath (variable)
func (c *SGXKeyDerive) RequiredGas(input []byte) uint64 {
	return 10000
}

// Run executes the contract (requires context)
func (c *SGXKeyDerive) Run(input []byte) ([]byte, error) {
	return nil, errors.New("context required")
}

// RunWithContext executes the contract with SGX context
// Input format: parentKeyID (32 bytes) + derivationPath (variable)
// Output format: childKeyID (32 bytes)
func (c *SGXKeyDerive) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
	// 1. Check if in read-only mode
	if ctx.IsReadOnly {
		return nil, errors.New("cannot derive key in read-only mode")
	}
	
	// 2. Parse input
	if len(input) < 32 {
		return nil, errors.New("invalid input: missing parent key ID")
	}
	parentKeyID := common.BytesToHash(input[:32])
	derivationPath := input[32:]
	
	// 3. Get key metadata and check ownership/permission
	metadata, err := ctx.KeyStore.GetMetadata(parentKeyID)
	if err != nil {
		return nil, err
	}
	
	// SECURITY: Check if caller is owner or has derive permission
	if metadata.Owner != ctx.Caller {
		// Check if caller has derive permission
		hasPermission := ctx.PermissionManager.CheckPermission(parentKeyID, ctx.Caller, PermissionDerive, ctx.Timestamp)
		if !hasPermission {
			return nil, errors.New("permission denied: only key owner can derive child keys")
		}
	}
	
	// 4. Derive child key
	childKeyID, err := ctx.KeyStore.DeriveKey(parentKeyID, derivationPath)
	if err != nil {
		return nil, err
	}
	
	// 5. Return child key ID (caller is automatically the owner of derived key)
	return childKeyID.Bytes(), nil
}
