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
	// 1. Check if in read-only mode
	if ctx.IsReadOnly {
		return nil, errors.New("cannot perform ECDH in read-only mode")
	}
	
	// 2. Parse input
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
	
	// 3. Get key metadata and check ownership
	metadata, err := ctx.KeyStore.GetMetadata(keyID)
	if err != nil {
		return nil, err
	}
	
	// SECURITY: Only owner can perform ECDH
	if metadata.Owner != ctx.Caller {
		return nil, errors.New("permission denied: only key owner can perform ECDH")
	}
	
	// 4. Execute ECDH and get new key ID
	newKeyID, err := ctx.KeyStore.ECDH(keyID, peerPubKey, kdfParams)
	if err != nil {
		return nil, err
	}
	
	// 5. Return new key ID (caller is automatically the owner of derived key)
	return newKeyID.Bytes(), nil
}
