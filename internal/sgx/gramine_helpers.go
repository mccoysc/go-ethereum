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

package sgx

import (
	"crypto/sha256"
	"fmt"
	"os"
)

// readMREnclave reads the MRENCLAVE from Gramine's /dev/attestation interface.
// This function is used by both native and fallback implementations.
func readMREnclave() ([]byte, error) {
	// Check if we're in mock mode
	if os.Getenv("XCHAIN_SGX_MODE") == "mock" {
		// Return a deterministic mock MRENCLAVE for testing
		hash := sha256.Sum256([]byte("mock-mrenclave-for-testing"))
		mrenclave := make([]byte, 32)
		copy(mrenclave, hash[:])
		return mrenclave, nil
	}

	// Try to read from /dev/attestation/my_target_info
	targetInfo, err := os.ReadFile("/dev/attestation/my_target_info")
	if err != nil {
		// Not in SGX environment, return error
		return nil, fmt.Errorf("failed to read /dev/attestation/my_target_info: %w", err)
	}

	if len(targetInfo) < 32 {
		return nil, fmt.Errorf("target_info too short: got %d bytes, need at least 32", len(targetInfo))
	}

	// MRENCLAVE is the first 32 bytes of target_info
	mrenclave := make([]byte, 32)
	copy(mrenclave, targetInfo[:32])

	return mrenclave, nil
}

// readMRSigner reads the MRSIGNER from Gramine's /dev/attestation interface.
// This function is used by both native and fallback implementations.
func readMRSigner() ([]byte, error) {
	// Check if we're in mock mode
	if os.Getenv("XCHAIN_SGX_MODE") == "mock" {
		// Return a deterministic mock MRSIGNER for testing
		hash := sha256.Sum256([]byte("mock-mrsigner-for-testing"))
		mrsigner := make([]byte, 32)
		copy(mrsigner, hash[:])
		return mrsigner, nil
	}

	// In real SGX environment, MRSIGNER comes from the signing key
	// It's not directly available from /dev/attestation
	// For now, return an error indicating it needs to be extracted from Quote
	return nil, fmt.Errorf("MRSIGNER not available - extract from Quote in real SGX")
}

// ReadMREnclaveMock exports the mock MRENCLAVE value for testing.
// This is the same value used by the attestor in mock mode.
func ReadMREnclaveMock() ([]byte, error) {
	hash := sha256.Sum256([]byte("mock-mrenclave-for-testing"))
	mrenclave := make([]byte, 32)
	copy(mrenclave, hash[:])
	return mrenclave, nil
}

// ReadMRSignerMock exports the mock MRSIGNER value for testing.
// This is the same value used by the attestor in mock mode.
func ReadMRSignerMock() ([]byte, error) {
	hash := sha256.Sum256([]byte("mock-mrsigner-for-testing"))
	mrsigner := make([]byte, 32)
	copy(mrsigner, hash[:])
	return mrsigner, nil
}

// generateQuoteViaGramine generates an SGX Quote using Gramine's /dev/attestation interface.
// This function is used by both native and fallback implementations for Quote-only operations.
func generateQuoteViaGramine(reportData []byte) ([]byte, error) {
	if len(reportData) > 64 {
		return nil, fmt.Errorf("reportData too long: max 64 bytes, got %d", len(reportData))
	}

	// Pad report data to 64 bytes
	paddedData := make([]byte, 64)
	copy(paddedData, reportData)

	// Write report data to /dev/attestation/user_report_data
	err := os.WriteFile("/dev/attestation/user_report_data", paddedData, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to write user_report_data: %w", err)
	}

	// Read the generated quote from /dev/attestation/quote
	quote, err := os.ReadFile("/dev/attestation/quote")
	if err != nil {
		return nil, fmt.Errorf("failed to read quote: %w", err)
	}

	return quote, nil
}