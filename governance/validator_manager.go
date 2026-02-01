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
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// InMemoryValidatorManager implements ValidatorManager with in-memory storage
type InMemoryValidatorManager struct {
	config     *StakingConfig
	mu         sync.RWMutex
	validators map[common.Address]*ValidatorInfo
}

// NewInMemoryValidatorManager creates a new in-memory validator manager
func NewInMemoryValidatorManager(config *StakingConfig) *InMemoryValidatorManager {
	return &InMemoryValidatorManager{
		config:     config,
		validators: make(map[common.Address]*ValidatorInfo),
	}
}

// GetValidator returns information about a validator
func (vm *InMemoryValidatorManager) GetValidator(addr common.Address) (*ValidatorInfo, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	validator, exists := vm.validators[addr]
	if !exists {
		return nil, ErrValidatorNotFound
	}

	validatorCopy := *validator
	validatorCopy.StakeAmount = new(big.Int).Set(validator.StakeAmount)
	return &validatorCopy, nil
}

// GetAllValidators returns all validators
func (vm *InMemoryValidatorManager) GetAllValidators() []*ValidatorInfo {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	validators := make([]*ValidatorInfo, 0, len(vm.validators))
	for _, v := range vm.validators {
		validatorCopy := *v
		validatorCopy.StakeAmount = new(big.Int).Set(v.StakeAmount)
		validators = append(validators, &validatorCopy)
	}

	return validators
}

// GetCoreValidators returns all core validators
func (vm *InMemoryValidatorManager) GetCoreValidators() []*ValidatorInfo {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	coreValidators := make([]*ValidatorInfo, 0)
	for _, v := range vm.validators {
		if v.Type == VoterTypeCore && v.Status == ValidatorStatusActive {
			validatorCopy := *v
			validatorCopy.StakeAmount = new(big.Int).Set(v.StakeAmount)
			coreValidators = append(coreValidators, &validatorCopy)
		}
	}

	return coreValidators
}

// GetCommunityValidators returns all community validators
func (vm *InMemoryValidatorManager) GetCommunityValidators() []*ValidatorInfo {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	communityValidators := make([]*ValidatorInfo, 0)
	for _, v := range vm.validators {
		if v.Type == VoterTypeCommunity && v.Status == ValidatorStatusActive {
			validatorCopy := *v
			validatorCopy.StakeAmount = new(big.Int).Set(v.StakeAmount)
			communityValidators = append(communityValidators, &validatorCopy)
		}
	}

	return communityValidators
}

// IsValidator checks if an address is a validator
func (vm *InMemoryValidatorManager) IsValidator(addr common.Address) bool {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	validator, exists := vm.validators[addr]
	if !exists {
		return false
	}

	return validator.Status == ValidatorStatusActive
}

// GetVoterType returns the voter type for an address
func (vm *InMemoryValidatorManager) GetVoterType(addr common.Address) VoterType {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	validator, exists := vm.validators[addr]
	if !exists {
		return VoterTypeCommunity // Default to community
	}

	return validator.Type
}

// Stake adds stake for a validator
func (vm *InMemoryValidatorManager) Stake(addr common.Address, amount *big.Int) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Check minimum stake
	if amount.Cmp(vm.config.MinStakeAmount) < 0 {
		return ErrInsufficientStake
	}

	validator, exists := vm.validators[addr]
	if !exists {
		// Create new validator
		validator = &ValidatorInfo{
			Address:      addr,
			Type:         VoterTypeCommunity,
			StakeAmount:  new(big.Int),
			VotingPower:  1,
			Status:       ValidatorStatusActive,
		}
		vm.validators[addr] = validator
	}

	// Add stake
	validator.StakeAmount = new(big.Int).Add(validator.StakeAmount, amount)

	return nil
}

// Unstake removes stake for a validator
func (vm *InMemoryValidatorManager) Unstake(addr common.Address, amount *big.Int) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	validator, exists := vm.validators[addr]
	if !exists {
		return ErrValidatorNotFound
	}

	// Check if validator has enough stake
	if validator.StakeAmount.Cmp(amount) < 0 {
		return ErrInsufficientBalance
	}

	// Remove stake
	validator.StakeAmount = new(big.Int).Sub(validator.StakeAmount, amount)

	// If stake falls below minimum, mark as inactive
	if validator.StakeAmount.Cmp(vm.config.MinStakeAmount) < 0 {
		validator.Status = ValidatorStatusInactive
	}

	return nil
}

// ClaimRewards claims staking rewards for a validator
func (vm *InMemoryValidatorManager) ClaimRewards(addr common.Address) (*big.Int, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	validator, exists := vm.validators[addr]
	if !exists {
		return nil, ErrValidatorNotFound
	}

	if validator.Status != ValidatorStatusActive {
		return nil, ErrValidatorNotActive
	}

	// Calculate rewards (simplified - in production, this would be based on blocks, time, etc.)
	// For now, return a placeholder
	rewards := big.NewInt(0)

	return rewards, nil
}

// Slash slashes a validator for misbehavior
func (vm *InMemoryValidatorManager) Slash(addr common.Address, reason string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	validator, exists := vm.validators[addr]
	if !exists {
		return ErrValidatorNotFound
	}

	// Calculate slashing amount
	slashAmount := new(big.Int).Mul(validator.StakeAmount, big.NewInt(int64(vm.config.SlashingRate)))
	slashAmount = slashAmount.Div(slashAmount, big.NewInt(100))

	// Apply slashing
	validator.StakeAmount = new(big.Int).Sub(validator.StakeAmount, slashAmount)

	// If stake falls below minimum, jail the validator
	if validator.StakeAmount.Cmp(vm.config.MinStakeAmount) < 0 {
		validator.Status = ValidatorStatusJailed
	}

	return nil
}

// UpdateMREnclave updates the MRENCLAVE for a validator
func (vm *InMemoryValidatorManager) UpdateMREnclave(addr common.Address, newMREnclave [32]byte) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	validator, exists := vm.validators[addr]
	if !exists {
		return ErrValidatorNotFound
	}

	validator.MRENCLAVE = newMREnclave

	return nil
}

// AddValidator adds a new validator (internal use)
func (vm *InMemoryValidatorManager) AddValidator(validator *ValidatorInfo) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	validatorCopy := *validator
	validatorCopy.StakeAmount = new(big.Int).Set(validator.StakeAmount)
	vm.validators[validator.Address] = &validatorCopy
}

// RemoveValidator removes a validator (internal use)
func (vm *InMemoryValidatorManager) RemoveValidator(addr common.Address) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if validator, exists := vm.validators[addr]; exists {
		validator.Status = ValidatorStatusExiting
	}
}
