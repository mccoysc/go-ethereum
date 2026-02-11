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

// SGXSign is the precompiled contract for ECDSA signing (0x8002)
type SGXSign struct{}

// Name returns the name of the contract
func (c *SGXSign) Name() string {
	return "SGXSign"
}

// RequiredGas calculates the required gas
// Input format: keyID (32 bytes) + hash (32 bytes)
func (c *SGXSign) RequiredGas(input []byte) uint64 {
	return 10000
}

// Run executes the contract (requires context)
func (c *SGXSign) Run(input []byte) ([]byte, error) {
	return nil, errors.New("context required")
}

// RunWithContext executes the contract with SGX context
// Input format: keyID (32 bytes) + hash (32 bytes)
// Output format: signature (65 bytes for ECDSA, 64 bytes for Ed25519)
func (c *SGXSign) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
	// 1. Check if in read-only mode
	if ctx.IsReadOnly {
		return nil, errors.New("cannot sign in read-only mode")
	}
	
	// 2. Parse input
	if len(input) < 64 {
		return nil, errors.New("invalid input: expected keyID (32 bytes) + hash (32 bytes)")
	}
	keyID := common.BytesToHash(input[:32])
	hash := input[32:64]
	
	// 3. Get key metadata and check ownership
	metadata, err := ctx.KeyStore.GetMetadata(keyID)
	if err != nil {
		return nil, err
	}
	
	// SECURITY: Only owner can sign
	if metadata.Owner != ctx.Caller {
		return nil, errors.New("permission denied: only key owner can sign")
	}
	
	// 4. Check key type
	if metadata.KeyType != KeyTypeECDSA && metadata.KeyType != KeyTypeEd25519 {
		return nil, errors.New("key type must be ECDSA or Ed25519 for signing")
	}
	
	// 5. Execute signing
	signature, err := ctx.KeyStore.Sign(keyID, hash)
	if err != nil {
		return nil, err
	}
	
	// 6. Return signature
	return signature, nil
}
