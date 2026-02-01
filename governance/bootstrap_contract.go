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

package governance

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/internal/sgx"
)

// BootstrapContract manages the bootstrap phase and founder registration
type BootstrapContract struct {
	mu sync.RWMutex

	// BootstrapEnded indicates if the bootstrap phase has ended
	BootstrapEnded bool

	// FounderCount is the current number of founders
	FounderCount uint64

	// MaxFounders is the maximum number of founders allowed
	MaxFounders uint64

	// AllowedMREnclave is the allowed initial MRENCLAVE
	AllowedMREnclave [32]byte

	// Founders maps founder addresses to their status
	Founders map[common.Address]bool

	// HardwareToFounder maps hardware IDs to founder addresses
	HardwareToFounder map[[32]byte]common.Address

	// verifier is the SGX verifier
	verifier SGXVerifier
}

// NewBootstrapContract creates a new bootstrap contract
func NewBootstrapContract(allowedMREnclave [32]byte, maxFounders uint64, verifier SGXVerifier) *BootstrapContract {
	return &BootstrapContract{
		AllowedMREnclave:  allowedMREnclave,
		MaxFounders:       maxFounders,
		Founders:          make(map[common.Address]bool),
		HardwareToFounder: make(map[[32]byte]common.Address),
		verifier:          verifier,
	}
}

// RegisterFounder registers a founder during the bootstrap phase
func (bc *BootstrapContract) RegisterFounder(
	caller common.Address,
	mrenclave [32]byte,
	hardwareID [32]byte,
	quote []byte,
) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// 1. Check if bootstrap phase has ended
	if bc.BootstrapEnded {
		return ErrBootstrapEnded
	}

	// 2. Verify MRENCLAVE matches
	if mrenclave != bc.AllowedMREnclave {
		return ErrInvalidMREnclave
	}

	// 3. Verify SGX Quote
	if err := bc.verifier.VerifyQuote(quote); err != nil {
		return ErrInvalidQuote
	}

	// 4. Extract and verify MRENCLAVE from quote
	quoteMREnclave, err := bc.verifier.ExtractMREnclave(quote)
	if err != nil || quoteMREnclave != mrenclave {
		return ErrInvalidMREnclave
	}

	// 5. Check if hardware ID is already registered
	if _, exists := bc.HardwareToFounder[hardwareID]; exists {
		return ErrHardwareAlreadyRegistered
	}

	// 6. Check if maximum founders reached
	if bc.FounderCount >= bc.MaxFounders {
		bc.BootstrapEnded = true
		return ErrMaxFoundersReached
	}

	// 7. Register founder
	bc.Founders[caller] = true
	bc.HardwareToFounder[hardwareID] = caller
	bc.FounderCount++

	// 8. Check if we've reached max founders and end bootstrap phase
	if bc.FounderCount >= bc.MaxFounders {
		bc.BootstrapEnded = true
	}

	return nil
}

// IsFounder checks if an address is a founder
func (bc *BootstrapContract) IsFounder(addr common.Address) bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.Founders[addr]
}

// IsBootstrapPhase checks if the bootstrap phase is still active
func (bc *BootstrapContract) IsBootstrapPhase() bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return !bc.BootstrapEnded
}

// GetFounderCount returns the current number of founders
func (bc *BootstrapContract) GetFounderCount() uint64 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.FounderCount
}

// GetAllFounders returns all founder addresses
func (bc *BootstrapContract) GetAllFounders() []common.Address {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	founders := make([]common.Address, 0, len(bc.Founders))
	for addr := range bc.Founders {
		founders = append(founders, addr)
	}
	return founders
}
