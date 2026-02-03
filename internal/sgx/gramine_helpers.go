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
)

// readMRSigner reads the MRSIGNER from Gramine's /dev/attestation interface.
func readMRSigner() ([]byte, error) {
	// MRSIGNER is derived from the enclave signing key
	// It is not directly available from /dev/attestation
	// Extract from Quote after generation
	return nil, fmt.Errorf("MRSIGNER not available - extract from Quote")
}

// generateQuoteViaGramine is implemented in:
// - gramine_helpers_production.go for production builds (default)
// - gramine_helpers_testenv.go for test builds (-tags testenv)

// readMREnclave is implemented in:
// - gramine_helpers_production.go for production builds (default)
// - gramine_helpers_testenv.go for test builds (-tags testenv)
