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

// CheckSGXHardware checks if SGX hardware and necessary devices are available
func CheckSGXHardware() error {
	// Check enclave device
	if _, err := os.Stat("/dev/sgx_enclave"); os.IsNotExist(err) {
		return fmt.Errorf("SGX enclave device not found (/dev/sgx_enclave): %w", err)
	}

	// Check provision device
	if _, err := os.Stat("/dev/sgx_provision"); os.IsNotExist(err) {
		return fmt.Errorf("SGX provision device not found (/dev/sgx_provision): %w", err)
	}

	// Check attestation interface (Gramine provides this)
	if _, err := os.Stat("/dev/attestation"); os.IsNotExist(err) {
		// This is optional in some Gramine configurations
		// Just log a warning instead of returning error
	}

	return nil
}

// GetSGXInfo retrieves information about the current SGX environment
func GetSGXInfo() (*SGXInfo, error) {
	info := &SGXInfo{}

	// Get MRENCLAVE and MRSIGNER from attestor
	// Try to create a Gramine attestor
	attestor, err := NewGramineAttestor()
	if err != nil {
		// If we can't create an attestor (e.g., not in enclave), return mock data
		return GetMockSGXInfo(), nil
	}

	info.MRENCLAVE = attestor.GetMREnclave()
	info.MRSIGNER = attestor.GetMRSigner()

	// Check if running inside enclave
	info.IsInsideEnclave = isInsideEnclave()

	return info, nil
}

// SGXInfo contains information about the SGX environment
type SGXInfo struct {
	MRENCLAVE       []byte // Enclave measurement
	MRSIGNER        []byte // Signer measurement
	IsInsideEnclave bool   // Whether running inside an enclave
}

// isInsideEnclave checks if the process is running inside a Gramine enclave
func isInsideEnclave() bool {
	// Gramine sets specific environment variables when running in enclave
	_, exists := os.LookupEnv("SGX_AESM_ADDR")
	return exists
}

// IsSGXAvailable checks if SGX is available without returning an error
func IsSGXAvailable() bool {
	return CheckSGXHardware() == nil
}

// GetMockSGXInfo returns mock SGX info for testing without SGX hardware
func GetMockSGXInfo() *SGXInfo {
	return &SGXInfo{
		MRENCLAVE:       make([]byte, 32), // Zero-filled for mock
		MRSIGNER:        make([]byte, 32), // Zero-filled for mock
		IsInsideEnclave: false,
	}
}
