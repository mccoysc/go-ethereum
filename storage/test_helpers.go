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

package storage

import (
	"os"
	"testing"
)

// setupTestEnvironment sets up a test environment for storage tests
// This sets SGX_TEST_MODE to skip hardware-specific checks
func setupTestEnvironment(t *testing.T) {
	t.Helper()

	// Enable test mode for SGX to skip hardware checks
	os.Setenv("SGX_TEST_MODE", "true")

	// Set up mock MRENCLAVE/MRSIGNER for testing
	os.Setenv("RA_TLS_MRENCLAVE", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	os.Setenv("RA_TLS_MRSIGNER", "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210")
}

func cleanupTestEnvironment(t *testing.T) {
	t.Helper()

	os.Unsetenv("SGX_TEST_MODE")
	os.Unsetenv("RA_TLS_MRENCLAVE")
	os.Unsetenv("RA_TLS_MRSIGNER")
}
