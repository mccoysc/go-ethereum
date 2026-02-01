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

// SGXKeyGetPublic is the precompiled contract for retrieving public keys (0x8001)
type SGXKeyGetPublic struct{}

// Name returns the name of the contract
func (c *SGXKeyGetPublic) Name() string {
	return "SGXKeyGetPublic"
}

// RequiredGas calculates the required gas
// Input format: keyID (32 bytes)
func (c *SGXKeyGetPublic) RequiredGas(input []byte) uint64 {
	return 3000
}

// Run executes the contract (requires context)
func (c *SGXKeyGetPublic) Run(input []byte) ([]byte, error) {
	return nil, errors.New("context required")
}

// RunWithContext executes the contract with SGX context
// Input format: keyID (32 bytes)
// Output format: publicKey (variable length)
func (c *SGXKeyGetPublic) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
	// 1. Parse input
	if len(input) < 32 {
		return nil, errors.New("invalid input: missing key ID")
	}
	keyID := common.BytesToHash(input[:32])
	
	// 2. Get public key (no permission check needed, public keys are public)
	pubKey, err := ctx.KeyStore.GetPublicKey(keyID)
	if err != nil {
		return nil, err
	}
	
	// 3. Return public key
	return pubKey, nil
}
