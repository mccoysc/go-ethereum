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

// SGXKeyDelete is the precompiled contract for deleting keys (0x8009)
type SGXKeyDelete struct{}

// Name returns the name of the contract
func (c *SGXKeyDelete) Name() string {
	return "SGXKeyDelete"
}

// RequiredGas calculates the required gas
// Input format: keyID (32 bytes)
func (c *SGXKeyDelete) RequiredGas(input []byte) uint64 {
	return 5000
}

// Run executes the contract (requires context)
func (c *SGXKeyDelete) Run(input []byte) ([]byte, error) {
	return nil, errors.New("context required")
}

// RunWithContext executes the contract with SGX context
// Input format: keyID (32 bytes)
// Output format: success (1 byte: 0x01 for success)
func (c *SGXKeyDelete) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
	// 1. Parse input
	if len(input) != 32 {
		return nil, errors.New("invalid input: expected keyID (32 bytes)")
	}
	keyID := common.BytesToHash(input[:32])
	
	// 2. Delete the key (ownership check is done inside DeleteKey)
	if err := ctx.KeyStore.DeleteKey(keyID, ctx.Caller); err != nil {
		return nil, err
	}
	
	// 3. Return success
	return []byte{0x01}, nil
}
