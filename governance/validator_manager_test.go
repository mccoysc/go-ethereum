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
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestValidatorManager_Stake(t *testing.T) {
	config := DefaultStakingConfig()
	vm := NewInMemoryValidatorManager(config)

	addr := common.HexToAddress("0x1")
	amount := new(big.Int).Mul(big.NewInt(15000), big.NewInt(1e18))

	// Stake
	err := vm.Stake(addr, amount)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check validator was created
	validator, err := vm.GetValidator(addr)
	if err != nil {
		t.Fatalf("failed to get validator: %v", err)
	}

	if validator.StakeAmount.Cmp(amount) != 0 {
		t.Errorf("expected stake %v, got %v", amount, validator.StakeAmount)
	}

	if validator.Status != ValidatorStatusActive {
		t.Error("validator should be active")
	}
}

func TestValidatorManager_Stake_InsufficientAmount(t *testing.T) {
	config := DefaultStakingConfig()
	vm := NewInMemoryValidatorManager(config)

	addr := common.HexToAddress("0x1")
	amount := new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)) // Less than minimum

	// Stake
	err := vm.Stake(addr, amount)
	if err != ErrInsufficientStake {
		t.Errorf("expected error %v, got %v", ErrInsufficientStake, err)
	}
}

func TestValidatorManager_Unstake(t *testing.T) {
	config := DefaultStakingConfig()
	vm := NewInMemoryValidatorManager(config)

	addr := common.HexToAddress("0x1")
	stakeAmount := new(big.Int).Mul(big.NewInt(20000), big.NewInt(1e18))
	unstakeAmount := new(big.Int).Mul(big.NewInt(5000), big.NewInt(1e18))

	// Stake first
	vm.Stake(addr, stakeAmount)

	// Unstake
	err := vm.Unstake(addr, unstakeAmount)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check remaining stake
	validator, _ := vm.GetValidator(addr)
	expectedStake := new(big.Int).Sub(stakeAmount, unstakeAmount)
	if validator.StakeAmount.Cmp(expectedStake) != 0 {
		t.Errorf("expected stake %v, got %v", expectedStake, validator.StakeAmount)
	}
}

func TestValidatorManager_Unstake_InsufficientBalance(t *testing.T) {
	config := DefaultStakingConfig()
	vm := NewInMemoryValidatorManager(config)

	addr := common.HexToAddress("0x1")
	stakeAmount := new(big.Int).Mul(big.NewInt(15000), big.NewInt(1e18))
	unstakeAmount := new(big.Int).Mul(big.NewInt(20000), big.NewInt(1e18))

	// Stake first
	vm.Stake(addr, stakeAmount)

	// Try to unstake more than staked
	err := vm.Unstake(addr, unstakeAmount)
	if err != ErrInsufficientBalance {
		t.Errorf("expected error %v, got %v", ErrInsufficientBalance, err)
	}
}

func TestValidatorManager_Slash(t *testing.T) {
	config := DefaultStakingConfig()
	vm := NewInMemoryValidatorManager(config)

	addr := common.HexToAddress("0x1")
	stakeAmount := new(big.Int).Mul(big.NewInt(20000), big.NewInt(1e18))

	// Stake first
	vm.Stake(addr, stakeAmount)

	// Slash
	err := vm.Slash(addr, "misbehavior")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check slashed amount (10% = 2000)
	validator, _ := vm.GetValidator(addr)
	slashAmount := new(big.Int).Mul(stakeAmount, big.NewInt(int64(config.SlashingRate)))
	slashAmount = slashAmount.Div(slashAmount, big.NewInt(100))
	expectedStake := new(big.Int).Sub(stakeAmount, slashAmount)
	
	if validator.StakeAmount.Cmp(expectedStake) != 0 {
		t.Errorf("expected stake %v, got %v after slashing", expectedStake, validator.StakeAmount)
	}
}

func TestValidatorManager_GetCoreValidators(t *testing.T) {
	config := DefaultStakingConfig()
	vm := NewInMemoryValidatorManager(config)

	// Add core validators
	core1 := &ValidatorInfo{
		Address:     common.HexToAddress("0x1"),
		Type:        VoterTypeCore,
		Status:      ValidatorStatusActive,
		StakeAmount: big.NewInt(1000),
		VotingPower: 1,
	}
	core2 := &ValidatorInfo{
		Address:     common.HexToAddress("0x2"),
		Type:        VoterTypeCore,
		Status:      ValidatorStatusActive,
		StakeAmount: big.NewInt(1000),
		VotingPower: 1,
	}

	// Add community validator
	community := &ValidatorInfo{
		Address:     common.HexToAddress("0x3"),
		Type:        VoterTypeCommunity,
		Status:      ValidatorStatusActive,
		StakeAmount: big.NewInt(1000),
		VotingPower: 1,
	}

	vm.AddValidator(core1)
	vm.AddValidator(core2)
	vm.AddValidator(community)

	// Get core validators
	coreValidators := vm.GetCoreValidators()
	if len(coreValidators) != 2 {
		t.Errorf("expected 2 core validators, got %d", len(coreValidators))
	}

	// Get community validators
	communityValidators := vm.GetCommunityValidators()
	if len(communityValidators) != 1 {
		t.Errorf("expected 1 community validator, got %d", len(communityValidators))
	}
}

func TestValidatorManager_IsValidator(t *testing.T) {
	config := DefaultStakingConfig()
	vm := NewInMemoryValidatorManager(config)

	addr := common.HexToAddress("0x1")

	// Not a validator initially
	if vm.IsValidator(addr) {
		t.Error("should not be a validator")
	}

	// Add validator
	validator := &ValidatorInfo{
		Address:     addr,
		Type:        VoterTypeCore,
		Status:      ValidatorStatusActive,
		StakeAmount: big.NewInt(1000),
		VotingPower: 1,
	}
	vm.AddValidator(validator)

	// Should be a validator now
	if !vm.IsValidator(addr) {
		t.Error("should be a validator")
	}

	// Mark as inactive
	vm.RemoveValidator(addr)
	validator.Status = ValidatorStatusExiting

	// Should not be active validator
	if vm.IsValidator(addr) {
		t.Error("should not be an active validator")
	}
}

func TestValidatorManager_UpdateMREnclave(t *testing.T) {
	config := DefaultStakingConfig()
	vm := NewInMemoryValidatorManager(config)

	addr := common.HexToAddress("0x1")
	oldMR := [32]byte{1, 2, 3}
	newMR := [32]byte{4, 5, 6}

	// Add validator
	validator := &ValidatorInfo{
		Address:     addr,
		Type:        VoterTypeCore,
		Status:      ValidatorStatusActive,
		MRENCLAVE:   oldMR,
		StakeAmount: big.NewInt(1000),
		VotingPower: 1,
	}
	vm.AddValidator(validator)

	// Update MRENCLAVE
	err := vm.UpdateMREnclave(addr, newMR)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify update
	updated, _ := vm.GetValidator(addr)
	if updated.MRENCLAVE != newMR {
		t.Error("MRENCLAVE not updated")
	}
}
