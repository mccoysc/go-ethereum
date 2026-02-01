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

// MockSGXVerifier is a mock implementation of SGXVerifier for testing
type MockSGXVerifier struct {
	shouldFailVerify       bool
	shouldFailExtractMR    bool
	shouldFailExtractHW    bool
	mrenclaveToReturn      [32]byte
	hardwareIDToReturn     string
}

func (m *MockSGXVerifier) VerifyQuote(quote []byte) error {
	if m.shouldFailVerify {
		return ErrInvalidQuote
	}
	return nil
}

func (m *MockSGXVerifier) ExtractMREnclave(quote []byte) ([32]byte, error) {
	if m.shouldFailExtractMR {
		return [32]byte{}, ErrInvalidMREnclave
	}
	return m.mrenclaveToReturn, nil
}

func (m *MockSGXVerifier) ExtractHardwareID(quote []byte) (string, error) {
	if m.shouldFailExtractHW {
		return "", ErrInvalidQuote
	}
	return m.hardwareIDToReturn, nil
}

func TestBootstrapContract_RegisterFounder(t *testing.T) {
	allowedMR := [32]byte{1, 2, 3}
	maxFounders := uint64(3)

	tests := []struct {
		name           string
		setupVerifier  func() *MockSGXVerifier
		caller         common.Address
		mrenclave      [32]byte
		hardwareID     [32]byte
		quote          []byte
		expectError    error
		expectedCount  uint64
		shouldEnd      bool
	}{
		{
			name: "successful registration",
			setupVerifier: func() *MockSGXVerifier {
				return &MockSGXVerifier{
					mrenclaveToReturn:  allowedMR,
					hardwareIDToReturn: "hw1",
				}
			},
			caller:        common.HexToAddress("0x1"),
			mrenclave:     allowedMR,
			hardwareID:    [32]byte{1},
			quote:         []byte("valid-quote"),
			expectError:   nil,
			expectedCount: 1,
			shouldEnd:     false,
		},
		{
			name: "invalid MRENCLAVE",
			setupVerifier: func() *MockSGXVerifier {
				return &MockSGXVerifier{
					mrenclaveToReturn:  [32]byte{9, 9, 9},
					hardwareIDToReturn: "hw1",
				}
			},
			caller:        common.HexToAddress("0x1"),
			mrenclave:     [32]byte{9, 9, 9},
			hardwareID:    [32]byte{1},
			quote:         []byte("valid-quote"),
			expectError:   ErrInvalidMREnclave,
			expectedCount: 0,
			shouldEnd:     false,
		},
		{
			name: "quote verification failure",
			setupVerifier: func() *MockSGXVerifier {
				return &MockSGXVerifier{
					shouldFailVerify:   true,
					mrenclaveToReturn:  allowedMR,
					hardwareIDToReturn: "hw1",
				}
			},
			caller:        common.HexToAddress("0x1"),
			mrenclave:     allowedMR,
			hardwareID:    [32]byte{1},
			quote:         []byte("invalid-quote"),
			expectError:   ErrInvalidQuote,
			expectedCount: 0,
			shouldEnd:     false,
		},
		{
			name: "hardware already registered",
			setupVerifier: func() *MockSGXVerifier {
				return &MockSGXVerifier{
					mrenclaveToReturn:  allowedMR,
					hardwareIDToReturn: "hw1",
				}
			},
			caller:        common.HexToAddress("0x2"),
			mrenclave:     allowedMR,
			hardwareID:    [32]byte{1}, // Same hardware as first test
			quote:         []byte("valid-quote"),
			expectError:   ErrHardwareAlreadyRegistered,
			expectedCount: 1, // Should still have only 1 founder
			shouldEnd:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := tt.setupVerifier()
			bc := NewBootstrapContract(allowedMR, maxFounders, verifier)

			// Register first founder if needed
			if tt.name == "hardware already registered" {
				bc.RegisterFounder(
					common.HexToAddress("0x1"),
					allowedMR,
					[32]byte{1},
					[]byte("valid-quote"),
				)
			}

			err := bc.RegisterFounder(tt.caller, tt.mrenclave, tt.hardwareID, tt.quote)

			if tt.expectError != nil {
				if err != tt.expectError {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if bc.GetFounderCount() != tt.expectedCount {
				t.Errorf("expected count %d, got %d", tt.expectedCount, bc.GetFounderCount())
			}

			if bc.BootstrapEnded != tt.shouldEnd {
				t.Errorf("expected BootstrapEnded=%v, got %v", tt.shouldEnd, bc.BootstrapEnded)
			}
		})
	}
}

func TestBootstrapContract_MaxFounders(t *testing.T) {
	allowedMR := [32]byte{1, 2, 3}
	maxFounders := uint64(3)

	verifier := &MockSGXVerifier{
		mrenclaveToReturn:  allowedMR,
		hardwareIDToReturn: "hw",
	}

	bc := NewBootstrapContract(allowedMR, maxFounders, verifier)

	// Register 3 founders
	for i := 0; i < 3; i++ {
		addr := common.BigToAddress(common.Big1)
		addr[19] = byte(i)
		hwID := [32]byte{}
		hwID[0] = byte(i)

		err := bc.RegisterFounder(addr, allowedMR, hwID, []byte("quote"))
		if err != nil {
			t.Fatalf("failed to register founder %d: %v", i, err)
		}
	}

	// Should be at max
	if bc.GetFounderCount() != 3 {
		t.Errorf("expected 3 founders, got %d", bc.GetFounderCount())
	}

	if !bc.BootstrapEnded {
		t.Error("expected bootstrap to end after reaching max founders")
	}

	// Try to register one more
	addr := common.BigToAddress(common.Big1)
	addr[19] = byte(99)
	hwID := [32]byte{99}

	err := bc.RegisterFounder(addr, allowedMR, hwID, []byte("quote"))
	if err != ErrBootstrapEnded {
		t.Errorf("expected ErrBootstrapEnded, got %v", err)
	}
}

func TestBootstrapContract_IsFounder(t *testing.T) {
	allowedMR := [32]byte{1, 2, 3}
	verifier := &MockSGXVerifier{
		mrenclaveToReturn:  allowedMR,
		hardwareIDToReturn: "hw1",
	}

	bc := NewBootstrapContract(allowedMR, 5, verifier)

	addr := common.HexToAddress("0x123")
	hwID := [32]byte{1}

	// Not a founder yet
	if bc.IsFounder(addr) {
		t.Error("address should not be a founder yet")
	}

	// Register as founder
	err := bc.RegisterFounder(addr, allowedMR, hwID, []byte("quote"))
	if err != nil {
		t.Fatalf("failed to register founder: %v", err)
	}

	// Now should be a founder
	if !bc.IsFounder(addr) {
		t.Error("address should be a founder")
	}

	// Different address should not be a founder
	otherAddr := common.HexToAddress("0x456")
	if bc.IsFounder(otherAddr) {
		t.Error("other address should not be a founder")
	}
}

func TestBootstrapContract_GetAllFounders(t *testing.T) {
	allowedMR := [32]byte{1, 2, 3}
	verifier := &MockSGXVerifier{
		mrenclaveToReturn:  allowedMR,
		hardwareIDToReturn: "hw",
	}

	bc := NewBootstrapContract(allowedMR, 5, verifier)

	// Register 2 founders
	addr1 := common.HexToAddress("0x1")
	addr2 := common.HexToAddress("0x2")

	bc.RegisterFounder(addr1, allowedMR, [32]byte{1}, []byte("quote"))
	bc.RegisterFounder(addr2, allowedMR, [32]byte{2}, []byte("quote"))

	founders := bc.GetAllFounders()
	if len(founders) != 2 {
		t.Errorf("expected 2 founders, got %d", len(founders))
	}

	// Check both addresses are in the list
	foundAddr1, foundAddr2 := false, false
	for _, addr := range founders {
		if addr == addr1 {
			foundAddr1 = true
		}
		if addr == addr2 {
			foundAddr2 = true
		}
	}

	if !foundAddr1 || !foundAddr2 {
		t.Error("not all founders found in list")
	}
}
