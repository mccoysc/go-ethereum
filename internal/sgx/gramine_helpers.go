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
	"fmt"
	"os"
)

// readMREnclave reads the MRENCLAVE from Gramine's /dev/attestation interface.
func readMREnclave() ([]byte, error) {
	// Read from /dev/attestation/my_target_info
	targetInfo, err := os.ReadFile("/dev/attestation/my_target_info")
	if err != nil {
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
func readMRSigner() ([]byte, error) {
	// MRSIGNER is derived from the enclave signing key
	// It is not directly available from /dev/attestation
	// Extract from Quote after generation
	return nil, fmt.Errorf("MRSIGNER not available - extract from Quote")
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