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
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestAdmissionController_CheckAdmission(t *testing.T) {
	// Create whitelist
	whitelistCfg := DefaultWhitelistConfig()
	voting := NewMockVotingManager()
	whitelist := NewInMemoryWhitelistManager(whitelistCfg, voting)

	// Add MRENCLAVE to whitelist
	mrenclave := [32]byte{1, 2, 3}
	whitelist.AddEntry(&MREnclaveEntry{
		MRENCLAVE: mrenclave,
		Version:   "v1.0.0",
		Status:    StatusActive,
	})

	// Create verifier
	verifier := &MockSGXVerifier{
		mrenclaveToReturn:  mrenclave,
		hardwareIDToReturn: "hw1",
	}

	// Create admission controller
	ac := NewSGXAdmissionController(whitelist, verifier)

	nodeID := common.BytesToHash([]byte("node1"))
	quote := []byte("valid-quote")

	// Check admission
	allowed, err := ac.CheckAdmission(nodeID, mrenclave, quote)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("admission should be allowed")
	}

	// Verify status was recorded
	status, err := ac.GetAdmissionStatus(nodeID)
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if !status.Allowed {
		t.Error("status should show allowed")
	}
}

func TestAdmissionController_CheckAdmission_QuoteVerificationFailed(t *testing.T) {
	whitelistCfg := DefaultWhitelistConfig()
	voting := NewMockVotingManager()
	whitelist := NewInMemoryWhitelistManager(whitelistCfg, voting)

	mrenclave := [32]byte{1, 2, 3}
	whitelist.AddEntry(&MREnclaveEntry{
		MRENCLAVE: mrenclave,
		Version:   "v1.0.0",
		Status:    StatusActive,
	})

	// Verifier that fails verification
	verifier := &MockSGXVerifier{
		shouldFailVerify:   true,
		mrenclaveToReturn:  mrenclave,
		hardwareIDToReturn: "hw1",
	}

	ac := NewSGXAdmissionController(whitelist, verifier)

	nodeID := common.BytesToHash([]byte("node1"))
	quote := []byte("invalid-quote")

	// Check admission - should fail
	allowed, err := ac.CheckAdmission(nodeID, mrenclave, quote)
	if err != ErrQuoteVerificationFailed {
		t.Errorf("expected error %v, got %v", ErrQuoteVerificationFailed, err)
	}
	if allowed {
		t.Error("admission should not be allowed")
	}
}

func TestAdmissionController_CheckAdmission_MREnclaveMismatch(t *testing.T) {
	whitelistCfg := DefaultWhitelistConfig()
	voting := NewMockVotingManager()
	whitelist := NewInMemoryWhitelistManager(whitelistCfg, voting)

	mrenclave := [32]byte{1, 2, 3}
	whitelist.AddEntry(&MREnclaveEntry{
		MRENCLAVE: mrenclave,
		Version:   "v1.0.0",
		Status:    StatusActive,
	})

	// Verifier returns different MRENCLAVE
	differentMR := [32]byte{9, 9, 9}
	verifier := &MockSGXVerifier{
		mrenclaveToReturn:  differentMR,
		hardwareIDToReturn: "hw1",
	}

	ac := NewSGXAdmissionController(whitelist, verifier)

	nodeID := common.BytesToHash([]byte("node1"))
	quote := []byte("valid-quote")

	// Check admission - should fail
	allowed, err := ac.CheckAdmission(nodeID, mrenclave, quote)
	if err != ErrInvalidMREnclave {
		t.Errorf("expected error %v, got %v", ErrInvalidMREnclave, err)
	}
	if allowed {
		t.Error("admission should not be allowed")
	}
}

func TestAdmissionController_CheckAdmission_NotInWhitelist(t *testing.T) {
	whitelistCfg := DefaultWhitelistConfig()
	voting := NewMockVotingManager()
	whitelist := NewInMemoryWhitelistManager(whitelistCfg, voting)

	// MRENCLAVE not in whitelist
	mrenclave := [32]byte{1, 2, 3}
	verifier := &MockSGXVerifier{
		mrenclaveToReturn:  mrenclave,
		hardwareIDToReturn: "hw1",
	}

	ac := NewSGXAdmissionController(whitelist, verifier)

	nodeID := common.BytesToHash([]byte("node1"))
	quote := []byte("valid-quote")

	// Check admission - should fail
	allowed, err := ac.CheckAdmission(nodeID, mrenclave, quote)
	if err != ErrMREnclaveNotAllowed {
		t.Errorf("expected error %v, got %v", ErrMREnclaveNotAllowed, err)
	}
	if allowed {
		t.Error("admission should not be allowed")
	}
}

func TestAdmissionController_RecordConnection(t *testing.T) {
	whitelistCfg := DefaultWhitelistConfig()
	voting := NewMockVotingManager()
	whitelist := NewInMemoryWhitelistManager(whitelistCfg, voting)
	verifier := &MockSGXVerifier{}
	ac := NewSGXAdmissionController(whitelist, verifier)

	nodeID := common.BytesToHash([]byte("node1"))
	mrenclave := [32]byte{1, 2, 3}

	// Record connection
	err := ac.RecordConnection(nodeID, mrenclave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check status
	status, err := ac.GetAdmissionStatus(nodeID)
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if status.ConnectedAt == 0 {
		t.Error("connection time should be set")
	}
}

func TestAdmissionController_RegisterValidatorHardware(t *testing.T) {
	whitelistCfg := DefaultWhitelistConfig()
	voting := NewMockVotingManager()
	whitelist := NewInMemoryWhitelistManager(whitelistCfg, voting)
	verifier := &MockSGXVerifier{}
	ac := NewSGXAdmissionController(whitelist, verifier)

	addr := common.HexToAddress("0x1")
	hardwareID := "hw123"

	// Register
	err := ac.RegisterValidatorHardware(addr, hardwareID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check binding
	retrievedHW, exists := ac.GetHardwareBinding(addr)
	if !exists {
		t.Fatal("hardware binding not found")
	}
	if retrievedHW != hardwareID {
		t.Errorf("expected hardware ID %s, got %s", hardwareID, retrievedHW)
	}

	// Check reverse lookup
	retrievedAddr, exists := ac.GetValidatorByHardware(hardwareID)
	if !exists {
		t.Fatal("validator not found by hardware")
	}
	if retrievedAddr != addr {
		t.Errorf("expected address %v, got %v", addr, retrievedAddr)
	}
}

func TestAdmissionController_RegisterValidatorHardware_AlreadyRegistered(t *testing.T) {
	whitelistCfg := DefaultWhitelistConfig()
	voting := NewMockVotingManager()
	whitelist := NewInMemoryWhitelistManager(whitelistCfg, voting)
	verifier := &MockSGXVerifier{}
	ac := NewSGXAdmissionController(whitelist, verifier)

	addr1 := common.HexToAddress("0x1")
	addr2 := common.HexToAddress("0x2")
	hardwareID := "hw123"

	// Register first validator
	ac.RegisterValidatorHardware(addr1, hardwareID)

	// Try to register second validator with same hardware
	err := ac.RegisterValidatorHardware(addr2, hardwareID)
	if err != ErrHardwareAlreadyRegistered {
		t.Errorf("expected error %v, got %v", ErrHardwareAlreadyRegistered, err)
	}
}

func TestAdmissionController_UnregisterValidator(t *testing.T) {
	whitelistCfg := DefaultWhitelistConfig()
	voting := NewMockVotingManager()
	whitelist := NewInMemoryWhitelistManager(whitelistCfg, voting)
	verifier := &MockSGXVerifier{}
	ac := NewSGXAdmissionController(whitelist, verifier)

	addr := common.HexToAddress("0x1")
	hardwareID := "hw123"

	// Register
	ac.RegisterValidatorHardware(addr, hardwareID)

	// Unregister
	err := ac.UnregisterValidator(addr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check binding removed
	_, exists := ac.GetHardwareBinding(addr)
	if exists {
		t.Error("hardware binding should be removed")
	}

	// Check reverse lookup removed
	_, exists = ac.GetValidatorByHardware(hardwareID)
	if exists {
		t.Error("reverse lookup should be removed")
	}
}
