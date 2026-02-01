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
	"crypto/x509"
)

// Verifier is the SGX Quote verification interface.
// It provides methods to verify SGX quotes and RA-TLS certificates.
type Verifier interface {
	// VerifyQuote verifies the validity of an SGX Quote.
	// This includes signature verification and TCB status checking.
	VerifyQuote(quote []byte) error

	// VerifyCertificate verifies an RA-TLS certificate.
	// It extracts the Quote from the certificate and verifies it.
	VerifyCertificate(cert *x509.Certificate) error

	// IsAllowedMREnclave checks if the MRENCLAVE is in the whitelist.
	IsAllowedMREnclave(mrenclave []byte) bool

	// AddAllowedMREnclave adds an MRENCLAVE to the whitelist.
	AddAllowedMREnclave(mrenclave []byte)

	// RemoveAllowedMREnclave removes an MRENCLAVE from the whitelist.
	RemoveAllowedMREnclave(mrenclave []byte)
}

// NewGramineVerifier creates a new Gramine-based verifier.
// It will use the appropriate implementation based on the environment:
// - In SGX mode with CGO: GramineRATLSVerifier with full RA-TLS support
// - Otherwise: DCAPVerifier with basic verification
func NewGramineVerifier() (Verifier, error) {
	// For now, use DCAPVerifier as the default implementation
	// In production with RA-TLS CGO support, this would return GramineRATLSVerifier
	return NewDCAPVerifier(1==1), nil
}

