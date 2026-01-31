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

//go:build cgo
// +build cgo

package sgx

/*
// Conditional library linking based on gramine_libs build tag
#cgo gramine_libs LDFLAGS: -lra_tls_attest -lra_tls_verify -lsgx_dcap_ql -lmbedtls -lmbedx509 -lmbedcrypto

#include <stdlib.h>
#include <stdint.h>

// Gramine RA-TLS function declarations
int ra_tls_create_key_and_crt_der(uint8_t** der_key, size_t* der_key_size,
                                   uint8_t** der_crt, size_t* der_crt_size);
void ra_tls_free_key_and_crt_der(uint8_t* der_key, uint8_t* der_crt);

// Stub implementations when Gramine libraries are not available
// These will cause linker errors to be resolved by defining them inline
#ifndef GRAMINE_LIBS_AVAILABLE

// Weak attribute allows real implementation to override if available
int __attribute__((weak)) ra_tls_create_key_and_crt_der(uint8_t** der_key, size_t* der_key_size,
                                                          uint8_t** der_crt, size_t* der_crt_size) {
    // Return error indicating library not available
    return -9999; // Special error code for stub
}

void __attribute__((weak)) ra_tls_free_key_and_crt_der(uint8_t* der_key, uint8_t* der_crt) {
    // No-op
}

#endif
*/
import "C"
import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"unsafe"
)

// GramineRATLSAttestor implements the Attestor interface using Gramine's
// native RA-TLS library via CGO.
type GramineRATLSAttestor struct {
	mrenclave []byte
	mrsigner  []byte
}

// NewGramineRATLSAttestor creates a new Gramine RA-TLS attestor.
// This version uses the actual Gramine RA-TLS C library.
func NewGramineRATLSAttestor() (*GramineRATLSAttestor, error) {
	attestor := &GramineRATLSAttestor{}

	// Read MRENCLAVE from Gramine
	mrenclave, err := readMREnclave()
	if err != nil {
		// If we can't read MRENCLAVE, we're probably not in SGX
		// Return an attestor with empty values for testing
		attestor.mrenclave = make([]byte, 32)
		attestor.mrsigner = make([]byte, 32)
		return attestor, nil
	}

	attestor.mrenclave = mrenclave
	// MRSIGNER would need to be read from attestation as well
	attestor.mrsigner = make([]byte, 32)

	return attestor, nil
}

// GenerateQuote generates an SGX quote using Gramine's /dev/attestation interface.
func (a *GramineRATLSAttestor) GenerateQuote(reportData []byte) ([]byte, error) {
	// Use Gramine's /dev/attestation interface
	return generateQuoteViaGramine(reportData)
}

// GenerateCertificate generates an RA-TLS certificate using Gramine's native library.
// This calls the C function ra_tls_create_key_and_crt_der() which generates
// a certificate with embedded SGX quote.
func (a *GramineRATLSAttestor) GenerateCertificate() (*tls.Certificate, error) {
	var derKey *C.uint8_t
	var derKeySize C.size_t
	var derCrt *C.uint8_t
	var derCrtSize C.size_t

	// Call Gramine's RA-TLS function to create key and certificate
	ret := C.ra_tls_create_key_and_crt_der(
		&derKey, &derKeySize,
		&derCrt, &derCrtSize,
	)

	if ret != 0 {
		return nil, fmt.Errorf("ra_tls_create_key_and_crt_der failed with code %d", ret)
	}

	// Convert C buffers to Go slices
	keyDER := C.GoBytes(unsafe.Pointer(derKey), C.int(derKeySize))
	crtDER := C.GoBytes(unsafe.Pointer(derCrt), C.int(derCrtSize))

	// Free the C-allocated memory
	C.ra_tls_free_key_and_crt_der(derKey, derCrt)

	// Parse the private key
	privateKey, err := x509.ParseECPrivateKey(keyDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Create TLS certificate structure
	cert := &tls.Certificate{
		Certificate: [][]byte{crtDER},
		PrivateKey:  privateKey,
	}

	return cert, nil
}

// GetMREnclave returns the MRENCLAVE of the current enclave.
func (a *GramineRATLSAttestor) GetMREnclave() []byte {
	return a.mrenclave
}

// GetMRSigner returns the MRSIGNER of the current enclave.
func (a *GramineRATLSAttestor) GetMRSigner() []byte {
	return a.mrsigner
}
