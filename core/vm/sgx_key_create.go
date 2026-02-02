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
)

// SGXKeyCreate is the precompiled contract for key creation (0x8000)
type SGXKeyCreate struct{}

// Name returns the name of the contract
func (c *SGXKeyCreate) Name() string {
	return "SGXKeyCreate"
}

// RequiredGas calculates the required gas
// Input format: keyType (1 byte)
func (c *SGXKeyCreate) RequiredGas(input []byte) uint64 {
	return 50000
}

// Run executes the contract (requires context)
func (c *SGXKeyCreate) Run(input []byte) ([]byte, error) {
	return nil, errors.New("context required")
}

// RunWithContext executes the contract with SGX context
// Input format: keyType (1 byte)
// Output format: keyID (32 bytes)
func (c *SGXKeyCreate) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
	// 0. Check if this is a read-only call (eth_call)
	// KEY_CREATE modifies state (creates keys, stores metadata) and MUST be a transaction
	if ctx.ReadOnly {
		return nil, errors.New("KEY_CREATE cannot be called in read-only mode (eth_call). " +
			"This operation creates and stores keys on-chain. " +
			"Use eth_sendTransaction to ensure the key creation is recorded on-chain.")
	}
	
	// 1. Parse input
	if len(input) < 1 {
		return nil, errors.New("invalid input: missing key type")
	}
	keyType := KeyType(input[0])
	
	// 2. Validate key type
	if keyType != KeyTypeECDSA && keyType != KeyTypeEd25519 && keyType != KeyTypeAES256 {
		return nil, fmt.Errorf("unsupported key type: %d", keyType)
	}
	
	// 3. Create key
	keyID, err := ctx.KeyStore.CreateKey(ctx.Caller, keyType)
	if err != nil {
		return nil, fmt.Errorf("failed to create key: %w", err)
	}
	
	// 4. Automatically grant Admin permission to the owner
	err = ctx.PermissionManager.GrantPermission(keyID, Permission{
		Grantee:   ctx.Caller,
		Type:      PermissionAdmin,
		ExpiresAt: 0,
		MaxUses:   0,
		UsedCount: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to grant admin permission: %w", err)
	}
	
	// 5. Return key ID
	return keyID.Bytes(), nil
}
