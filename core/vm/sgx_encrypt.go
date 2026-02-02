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

// SGXEncrypt is the precompiled contract for symmetric encryption (0x8006)
type SGXEncrypt struct{}

// Name returns the name of the contract
func (c *SGXEncrypt) Name() string {
	return "SGXEncrypt"
}

// RequiredGas calculates the required gas
// Input format: keyID (32 bytes) + plaintext (variable)
func (c *SGXEncrypt) RequiredGas(input []byte) uint64 {
	if len(input) < 32 {
		return 5000
	}
	
	plaintextLen := uint64(len(input) - 32)
	return 5000 + (plaintextLen * 10)
}

// Run executes the contract (requires context)
func (c *SGXEncrypt) Run(input []byte) ([]byte, error) {
	return nil, errors.New("context required")
}

// RunWithContext executes the contract with SGX context
// Input format: keyID (32 bytes) + plaintext (variable)
// Output format: nonce (12 bytes) + ciphertext + tag (16 bytes)
func (c *SGXEncrypt) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
	// 0. Check if this is a read-only call (eth_call)
	// ENCRYPT generates secret ciphertext (with random nonce), MUST be a transaction
	if ctx.ReadOnly {
		return nil, errors.New("ENCRYPT cannot be called in read-only mode (eth_call). " +
			"This operation generates ciphertext with random nonce. " +
			"Use eth_sendTransaction to ensure encryption is recorded on-chain.")
	}
	
	// 1. Parse input
	if len(input) < 32 {
		return nil, errors.New("invalid input: missing key ID")
	}
	keyID := common.BytesToHash(input[:32])
	plaintext := input[32:]
	
	// 2. Check key metadata (ensure it's an AES key)
	metadata, err := ctx.KeyStore.GetMetadata(keyID)
	if err != nil {
		return nil, err
	}
	if metadata.KeyType != KeyTypeAES256 {
		return nil, errors.New("key type must be AES256 for encryption")
	}
	
	// 3. Execute encryption (no permission check needed, encryption is a public operation)
	ciphertext, err := ctx.KeyStore.Encrypt(keyID, plaintext)
	if err != nil {
		return nil, err
	}
	
	// 4. Return ciphertext (includes nonce + ciphertext + tag)
	return ciphertext, nil
}
