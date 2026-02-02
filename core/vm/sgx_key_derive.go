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
	// 0. Check if this is a read-only call (eth_call)
	// KEY_DERIVE creates new derived keys, MUST be a transaction
	if ctx.ReadOnly {
		return nil, errors.New("KEY_DERIVE cannot be called in read-only mode (eth_call). " +
			"This operation creates derived keys and stores metadata on-chain. " +
			"Use eth_sendTransaction to ensure key derivation is recorded.")
	}
	
	// 1. Parse input
	if len(input) < 32 {
		return nil, errors.New("invalid input: missing parent key ID")
	}
	parentKeyID := common.BytesToHash(input[:32])
	derivationPath := input[32:]
	
	// 2. Check derivation permission
	if !ctx.PermissionManager.CheckPermission(parentKeyID, ctx.Caller, PermissionDerive, ctx.Timestamp) {
		// Check if caller has Admin permission
		if !ctx.PermissionManager.CheckPermission(parentKeyID, ctx.Caller, PermissionAdmin, ctx.Timestamp) {
			return nil, errors.New("permission denied: caller does not have Derive or Admin permission")
		}
	}
	
	// 3. Derive child key
	childKeyID, err := ctx.KeyStore.DeriveKey(parentKeyID, derivationPath)
	if err != nil {
		return nil, err
	}
	
	// 4. Record permission usage (increment counter)
	_ = ctx.PermissionManager.UsePermission(parentKeyID, ctx.Caller, PermissionDerive)
	
	// 5. Automatically grant Admin permission to the caller for the child key
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
	
	// 6. Return child key ID
	return childKeyID.Bytes(), nil
}
