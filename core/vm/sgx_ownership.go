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

// SGXTransferOwnership is the precompiled contract for key ownership transfer (0x8009)
type SGXTransferOwnership struct{}

// Name returns the name of the contract
func (c *SGXTransferOwnership) Name() string {
	return "SGXTransferOwnership"
}

// RequiredGas calculates the required gas
// Input format: keyID (32 bytes) + newOwner (20 bytes)
func (c *SGXTransferOwnership) RequiredGas(input []byte) uint64 {
	return 3000
}

// Run executes the contract (requires context)
func (c *SGXTransferOwnership) Run(input []byte) ([]byte, error) {
	return nil, errors.New("context required")
}

// RunWithContext executes the contract with SGX context
// Input format: keyID (32 bytes) + newOwner (20 bytes)
// Output: success (1 byte)
func (c *SGXTransferOwnership) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
	// 1. Check if in read-only mode
	if ctx.IsReadOnly {
		return nil, errors.New("cannot transfer ownership in read-only mode")
	}
	
	// 2. Parse input
	if len(input) != 52 { // 32 + 20
		return nil, errors.New("invalid input: expected 52 bytes (keyID + newOwner)")
	}
	
	keyID := common.BytesToHash(input[:32])
	newOwner := common.BytesToAddress(input[32:52])
	
	// 3. Get current key metadata
	metadata, err := ctx.KeyStore.GetMetadata(keyID)
	if err != nil {
		return nil, err
	}
	
	// 4. Check if caller is the current owner
	if metadata.Owner != ctx.Caller {
		return nil, errors.New("permission denied: only current owner can transfer ownership")
	}
	
	// 5. Transfer ownership (this needs to be added to KeyStore interface)
	if transferable, ok := ctx.KeyStore.(interface {
		TransferOwnership(keyID common.Hash, newOwner common.Address) error
	}); ok {
		if err := transferable.TransferOwnership(keyID, newOwner); err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("keystore does not support ownership transfer")
	}
	
	// 6. Return success
	return []byte{0x01}, nil
}
