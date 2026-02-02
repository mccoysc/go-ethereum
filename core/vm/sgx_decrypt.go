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

// SGXDecrypt is the precompiled contract for symmetric decryption (0x8007)
type SGXDecrypt struct{}

// Name returns the name of the contract
func (c *SGXDecrypt) Name() string {
	return "SGXDecrypt"
}

// RequiredGas calculates the required gas
// Input format: keyID (32 bytes) + ciphertext (variable)
func (c *SGXDecrypt) RequiredGas(input []byte) uint64 {
	if len(input) < 32 {
		return 5000
	}
	
	ciphertextLen := uint64(len(input) - 32)
	return 5000 + (ciphertextLen * 10)
}

// Run executes the contract (requires context)
func (c *SGXDecrypt) Run(input []byte) ([]byte, error) {
	return nil, errors.New("context required")
}

// RunWithContext executes the contract with SGX context
// Input format: keyID (32 bytes) + ciphertext (variable: nonce + encrypted + tag)
// Output format: plaintext (variable) - ONLY in read-only mode
func (c *SGXDecrypt) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
	// 0. CRITICAL: DECRYPT returns secret data (plaintext)
	// In writable mode (transaction), return values go on-chain
	// Therefore, DECRYPT can ONLY be used in read-only mode (eth_call)
	if !ctx.ReadOnly {
		return nil, errors.New("DECRYPT can ONLY be called in read-only mode (eth_call). " +
			"Plaintext is secret data and cannot be returned in transactions (which go on-chain). " +
			"Use eth_call for off-chain decryption queries.")
	}
	
	// 1. Parse input
	if len(input) < 32 {
		return nil, errors.New("invalid input: missing key ID")
	}
	keyID := common.BytesToHash(input[:32])
	ciphertext := input[32:]
	
	// 2. Check decryption permission
	if !ctx.PermissionManager.CheckPermission(keyID, ctx.Caller, PermissionDecrypt, ctx.Timestamp) {
		// Check if caller has Admin permission
		if !ctx.PermissionManager.CheckPermission(keyID, ctx.Caller, PermissionAdmin, ctx.Timestamp) {
			return nil, errors.New("permission denied: caller does not have Decrypt or Admin permission")
		}
	}
	
	// 3. Check key metadata (ensure it's an AES key)
	metadata, err := ctx.KeyStore.GetMetadata(keyID)
	if err != nil {
		return nil, err
	}
	if metadata.KeyType != KeyTypeAES256 {
		return nil, errors.New("key type must be AES256 for decryption")
	}
	
	// 4. Execute decryption
	plaintext, err := ctx.KeyStore.Decrypt(keyID, ciphertext)
	if err != nil {
		return nil, err
	}
	
	// 5. Record permission usage (increment counter)
	// NOTE: In read-only mode, this does NOT persist (no state change)
	// This is acceptable for read operations
	_ = ctx.PermissionManager.UsePermission(keyID, ctx.Caller, PermissionDecrypt)
	
	// 6. Return plaintext (ONLY in read-only mode, never goes on-chain)
	return plaintext, nil
}
