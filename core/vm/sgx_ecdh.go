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
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// SGXECDH is the precompiled contract for ECDH key exchange (0x8004)
type SGXECDH struct{}

// Name returns the name of the contract
func (c *SGXECDH) Name() string {
	return "SGXECDH"
}

// RequiredGas calculates the required gas
// Input format: keyID (32 bytes) + peerPubKey (64 bytes) + optional kdfParams (variable)
func (c *SGXECDH) RequiredGas(input []byte) uint64 {
	return 20000
}

// Run executes the contract (requires context)
func (c *SGXECDH) Run(input []byte) ([]byte, error) {
	return nil, errors.New("context required")
}

// RunWithContext executes the contract with SGX context
// Input format: keyID (32 bytes) + peerPubKey (64 bytes) + optional kdfParams (variable)
// Output format: newKeyID (32 bytes)
func (c *SGXECDH) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
	// 1. Parse input
	if len(input) < 96 {
		return nil, errors.New("invalid input: expected keyID (32 bytes) + peerPubKey (64 bytes)")
	}
	keyID := common.BytesToHash(input[:32])
	peerPubKey := input[32:96]
	
	// Parse optional kdfParams
	var kdfParams []byte
	if len(input) > 96 {
		kdfParams = input[96:]
	}
	
	// 2. Check derivation permission
	if !ctx.PermissionManager.CheckPermission(keyID, ctx.Caller, PermissionDerive, ctx.Timestamp) {
		// Check if caller has Admin permission
		if !ctx.PermissionManager.CheckPermission(keyID, ctx.Caller, PermissionAdmin, ctx.Timestamp) {
			return nil, errors.New("permission denied: caller does not have Derive or Admin permission")
		}
	}
	
	// 3. Execute ECDH and get new key ID
	newKeyID, err := ctx.KeyStore.ECDH(keyID, peerPubKey, kdfParams)
	if err != nil {
		return nil, err
	}
	
	// 4. Record permission usage (increment counter)
	_ = ctx.PermissionManager.UsePermission(keyID, ctx.Caller, PermissionDerive)
	
	// 5. Grant caller Admin permission on the new shared secret key
	metadata, err := ctx.KeyStore.GetMetadata(keyID)
	if err != nil {
		return nil, err
	}
	
	// Grant admin permission if caller is not already the owner
	if metadata.Owner != ctx.Caller {
		adminPerm := Permission{
			Grantee:   ctx.Caller,
			Type:      PermissionAdmin,
			ExpiresAt: 0,
			MaxUses:   0,
			UsedCount: 0,
		}
		
		if err := ctx.PermissionManager.GrantPermission(newKeyID, adminPerm); err != nil {
			return nil, fmt.Errorf("failed to grant admin permission: %w", err)
		}
	}
	
	// 6. Return new key ID
	return newKeyID.Bytes(), nil
}
