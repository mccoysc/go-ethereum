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
// Input format: keyID (32 bytes) + ciphertext (variable: nonce + encrypted + tag) + recipientPubKey (optional 65 bytes for re-encryption)
// Output format: if recipientPubKey provided, returns re-encrypted data; otherwise returns plaintext (only in non-readonly mode)
func (c *SGXDecrypt) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
	// 1. Check if in read-only mode
	if ctx.IsReadOnly {
		return nil, errors.New("cannot decrypt in read-only mode: decryption requires state modification and re-encryption")
	}
	
	// 2. Parse input
	if len(input) < 32 {
		return nil, errors.New("invalid input: missing key ID")
	}
	keyID := common.BytesToHash(input[:32])
	remaining := input[32:]
	
	// Check if re-encryption public key is provided
	var ciphertext []byte
	var recipientPubKey []byte
	var reencrypt bool
	
	// If input has extra 65 bytes at the end, it's the recipient's public key for re-encryption
	if len(remaining) >= 65 {
		// Try to extract recipient public key from the end
		potentialPubKey := remaining[len(remaining)-65:]
		// Simple check: uncompressed public key should start with 0x04
		if potentialPubKey[0] == 0x04 {
			recipientPubKey = potentialPubKey
			ciphertext = remaining[:len(remaining)-65]
			reencrypt = true
		} else {
			ciphertext = remaining
			reencrypt = false
		}
	} else {
		ciphertext = remaining
		reencrypt = false
	}
	
	// 3. Get key metadata and check ownership
	metadata, err := ctx.KeyStore.GetMetadata(keyID)
	if err != nil {
		return nil, err
	}
	
	// SECURITY: Only owner can decrypt
	if metadata.Owner != ctx.Caller {
		return nil, errors.New("permission denied: only key owner can decrypt")
	}
	
	// 4. Check key metadata (ensure it's an AES key)
	if metadata.KeyType != KeyTypeAES256 {
		return nil, errors.New("key type must be AES256 for decryption")
	}
	
	// 5. Execute decryption
	plaintext, err := ctx.KeyStore.Decrypt(keyID, ciphertext)
	if err != nil {
		return nil, err
	}
	
	// 6. Re-encrypt for recipient if public key provided
	if reencrypt {
		// Create ephemeral ECDH key for re-encryption
		ephemeralKeyID, err := ctx.KeyStore.CreateKey(ctx.Caller, KeyTypeECDSA)
		if err != nil {
			return nil, errors.New("failed to create ephemeral key for re-encryption")
		}
		
		// Perform ECDH to derive shared secret
		sharedKeyID, err := ctx.KeyStore.ECDH(ephemeralKeyID, recipientPubKey, nil)
		if err != nil {
			return nil, errors.New("failed to perform ECDH for re-encryption")
		}
		
		// Re-encrypt plaintext with shared secret
		reencrypted, err := ctx.KeyStore.Encrypt(sharedKeyID, plaintext)
		if err != nil {
			return nil, errors.New("failed to re-encrypt data")
		}
		
		// Get ephemeral public key to return with encrypted data
		ephemeralPubKey, err := ctx.KeyStore.GetPublicKey(ephemeralKeyID)
		if err != nil {
			return nil, errors.New("failed to get ephemeral public key")
		}
		
		// Clean up ephemeral keys
		_ = ctx.KeyStore.DeleteKey(ephemeralKeyID, ctx.Caller)
		_ = ctx.KeyStore.DeleteKey(sharedKeyID, ctx.Caller)
		
		// Return: ephemeralPubKey + reencrypted data
		result := make([]byte, len(ephemeralPubKey)+len(reencrypted))
		copy(result, ephemeralPubKey)
		copy(result[len(ephemeralPubKey):], reencrypted)
		
		return result, nil
	}
	
	// 7. If no re-encryption, return plaintext directly
	// Note: This should only be used in trusted environments or with additional protection
	return plaintext, nil
}
