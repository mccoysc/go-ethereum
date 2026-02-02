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
	"crypto/ed25519"
	"errors"

	"github.com/ethereum/go-ethereum/crypto"
)

// SGXVerify is the precompiled contract for signature verification (0x8003)
type SGXVerify struct{}

// Name returns the name of the contract
func (c *SGXVerify) Name() string {
	return "SGXVerify"
}

// RequiredGas calculates the required gas
// Input format: hash (32 bytes) + signature (65 bytes) + publicKey (64 bytes)
func (c *SGXVerify) RequiredGas(input []byte) uint64 {
	return 5000
}

// Run executes the contract (no context needed, pure computation)
// Input format: hash (32 bytes) + signature (variable) + publicKey (variable)
// Output format: result (1 byte: 0x01 for valid, 0x00 for invalid)
func (c *SGXVerify) Run(input []byte) ([]byte, error) {
	// ECDSA verification: hash (32) + sig (65) + pubkey (64 or 65) = 161 or 162 bytes
	// Ed25519 verification: hash (32) + sig (64) + pubkey (32) = 128 bytes
	
	if len(input) == 161 || len(input) == 162 {
		// ECDSA verification
		hash := input[:32]
		signature := input[32:97]
		pubKey := input[97:]
		
		// Remove 0x04 prefix from pubKey if present
		if len(pubKey) == 65 && pubKey[0] == 0x04 {
			pubKey = pubKey[1:]
		}
		
		// Verify we have 64 bytes after processing
		if len(pubKey) != 64 {
			return []byte{0x00}, nil
		}
		
		// Recover public key from signature
		recoveredPubKey, err := crypto.SigToPub(hash, signature)
		if err != nil {
			return []byte{0x00}, nil
		}
		
		// Compare public keys
		recoveredPubKeyBytes := crypto.FromECDSAPub(recoveredPubKey)
		// Remove 0x04 prefix
		if len(recoveredPubKeyBytes) > 0 && recoveredPubKeyBytes[0] == 0x04 {
			recoveredPubKeyBytes = recoveredPubKeyBytes[1:]
		}
		
		// Compare
		if len(recoveredPubKeyBytes) != len(pubKey) {
			return []byte{0x00}, nil
		}
		for i := 0; i < len(pubKey); i++ {
			if recoveredPubKeyBytes[i] != pubKey[i] {
				return []byte{0x00}, nil
			}
		}
		
		return []byte{0x01}, nil
		
	} else if len(input) == 128 {
		// Ed25519 verification
		hash := input[:32]
		signature := input[32:96]
		pubKey := input[96:128]
		
		if ed25519.Verify(ed25519.PublicKey(pubKey), hash, signature) {
			return []byte{0x01}, nil
		}
		return []byte{0x00}, nil
		
	} else {
		return nil, errors.New("invalid input length: expected 161-162 bytes (ECDSA) or 128 bytes (Ed25519)")
	}
}
