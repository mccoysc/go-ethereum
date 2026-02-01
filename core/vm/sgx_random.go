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
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
)

// SGXRandom is the precompiled contract for secure random number generation (0x8005)
type SGXRandom struct{}

// Name returns the name of the contract
func (c *SGXRandom) Name() string {
	return "SGXRandom"
}

// RequiredGas calculates the required gas
// Input format: length (32 bytes)
func (c *SGXRandom) RequiredGas(input []byte) uint64 {
	if len(input) < 32 {
		return 1000
	}
	
	// Parse length
	length := binary.BigEndian.Uint64(input[24:32])
	
	// Base cost + per-byte cost
	return 1000 + (length * 100)
}

// Run executes the contract (no context needed)
// Input format: length (32 bytes)
// Output format: randomBytes (variable length)
func (c *SGXRandom) Run(input []byte) ([]byte, error) {
	// 1. Parse input
	if len(input) < 32 {
		return nil, errors.New("invalid input: missing length")
	}
	
	// Extract length (big-endian uint256)
	length := binary.BigEndian.Uint64(input[24:32])
	
	// 2. Validate length (limit to max 1KB)
	if length > 1024 {
		return nil, errors.New("requested length too large (max 1KB)")
	}
	if length == 0 {
		return nil, errors.New("requested length must be greater than 0")
	}
	
	// 3. Generate random bytes
	randomBytes := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, randomBytes); err != nil {
		return nil, err
	}
	
	// 4. Return random bytes
	return randomBytes, nil
}
