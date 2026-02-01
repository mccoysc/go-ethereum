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

package genesis

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestCalculateContractAddress(t *testing.T) {
	deployer := common.HexToAddress("0x1234567890123456789012345678901234567890")
	
	// Test nonce 0
	addr0 := CalculateContractAddress(deployer, 0)
	if addr0 == (common.Address{}) {
		t.Error("address should not be zero")
	}

	// Test nonce 1
	addr1 := CalculateContractAddress(deployer, 1)
	if addr1 == (common.Address{}) {
		t.Error("address should not be zero")
	}

	// Different nonces should produce different addresses
	if addr0 == addr1 {
		t.Error("different nonces should produce different addresses")
	}

	// Same inputs should produce same output (deterministic)
	addr0Again := CalculateContractAddress(deployer, 0)
	if addr0 != addr0Again {
		t.Error("same inputs should produce same output")
	}
}

func TestCalculateCreate2Address(t *testing.T) {
	deployer := common.HexToAddress("0x1234567890123456789012345678901234567890")
	salt := [32]byte{1, 2, 3}
	initCodeHash := [32]byte{4, 5, 6}

	addr := CalculateCreate2Address(deployer, salt, initCodeHash)
	if addr == (common.Address{}) {
		t.Error("address should not be zero")
	}

	// Same inputs should produce same output (deterministic)
	addrAgain := CalculateCreate2Address(deployer, salt, initCodeHash)
	if addr != addrAgain {
		t.Error("same inputs should produce same output")
	}

	// Different salt should produce different address
	differentSalt := [32]byte{7, 8, 9}
	addrDifferent := CalculateCreate2Address(deployer, differentSalt, initCodeHash)
	if addr == addrDifferent {
		t.Error("different salt should produce different address")
	}
}

func TestPredictGovernanceAddress(t *testing.T) {
	deployer := common.HexToAddress("0x1234567890123456789012345678901234567890")

	addr := PredictGovernanceAddress(deployer)
	if addr == (common.Address{}) {
		t.Error("address should not be zero")
	}

	// Should be same as nonce 0
	expected := CalculateContractAddress(deployer, 0)
	if addr != expected {
		t.Error("governance address should be at nonce 0")
	}
}

func TestPredictSecurityConfigAddress(t *testing.T) {
	deployer := common.HexToAddress("0x1234567890123456789012345678901234567890")

	addr := PredictSecurityConfigAddress(deployer)
	if addr == (common.Address{}) {
		t.Error("address should not be zero")
	}

	// Should be same as nonce 1
	expected := CalculateContractAddress(deployer, 1)
	if addr != expected {
		t.Error("security config address should be at nonce 1")
	}

	// Should be different from governance address
	govAddr := PredictGovernanceAddress(deployer)
	if addr == govAddr {
		t.Error("security config and governance should have different addresses")
	}
}

func TestDefaultBootstrapConfig(t *testing.T) {
	config := DefaultBootstrapConfig()

	if config == nil {
		t.Fatal("config should not be nil")
	}

	if config.MaxFounders != 5 {
		t.Errorf("expected max founders 5, got %d", config.MaxFounders)
	}

	if config.VotingThreshold != 67 {
		t.Errorf("expected voting threshold 67, got %d", config.VotingThreshold)
	}
}
