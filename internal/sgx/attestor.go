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

// Package sgx implements Intel SGX attestation functionality for X Chain.
package sgx

import (
	"crypto/tls"
)

// Attestor is the SGX attestation interface.
// It provides methods to generate SGX quotes and RA-TLS certificates.
type Attestor interface {
	// GenerateQuote generates an SGX Quote with the given report data.
	// reportData: user-defined data (typically a public key hash), max 64 bytes
	// Returns: SGX Quote binary data
	GenerateQuote(reportData []byte) ([]byte, error)

	// GenerateCertificate generates an RA-TLS certificate.
	// The certificate embeds an SGX Quote for remote attestation during TLS handshake.
	GenerateCertificate() (*tls.Certificate, error)

	// GetMREnclave returns the MRENCLAVE of the local enclave.
	// MRENCLAVE is the SHA256 hash of the enclave code and initial data.
	GetMREnclave() []byte

	// GetMRSigner returns the MRSIGNER of the local enclave.
	// MRSIGNER is the hash of the signer's public key.
	GetMRSigner() []byte
}
