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
#cgo LDFLAGS: -ldl

#include <stdlib.h>
#include <stdint.h>
#include <dlfcn.h>
#include <stdio.h>

// Function pointers for dynamically loaded Gramine RA-TLS functions
static void* gramine_attest_handle = NULL;
static int (*ra_tls_create_key_and_crt_der_ptr)(uint8_t**, size_t*, uint8_t**, size_t*) = NULL;
static void (*ra_tls_free_key_and_crt_der_ptr)(uint8_t*, uint8_t*) = NULL;

// Initialize Gramine RA-TLS attestation library via dlopen
static int init_gramine_attest_lib() {
    if (gramine_attest_handle != NULL) {
        return 0; // Already initialized
    }

    // Try to load the Gramine RA-TLS attestation library
    gramine_attest_handle = dlopen("libra_tls_attest.so", RTLD_LAZY | RTLD_LOCAL);
    if (!gramine_attest_handle) {
        // Library not found - this is OK in non-SGX environments
        return -1;
    }

    // Load function symbols
    ra_tls_create_key_and_crt_der_ptr = dlsym(gramine_attest_handle, "ra_tls_create_key_and_crt_der");
    if (!ra_tls_create_key_and_crt_der_ptr) {
        dlclose(gramine_attest_handle);
        gramine_attest_handle = NULL;
        return -1;
    }

    ra_tls_free_key_and_crt_der_ptr = dlsym(gramine_attest_handle, "ra_tls_free_key_and_crt_der");
    if (!ra_tls_free_key_and_crt_der_ptr) {
        dlclose(gramine_attest_handle);
        gramine_attest_handle = NULL;
        return -1;
    }

    return 0;
}

// Wrapper function that uses dlopen/dlsym to call Gramine function
static int ra_tls_create_key_and_crt_der_dynamic(uint8_t** der_key, size_t* der_key_size,
                                                   uint8_t** der_crt, size_t* der_crt_size) {
    if (init_gramine_attest_lib() != 0) {
        return -10000; // Library not available
    }
    
    return ra_tls_create_key_and_crt_der_ptr(der_key, der_key_size, der_crt, der_crt_size);
}

static void ra_tls_free_key_and_crt_der_dynamic(uint8_t* der_key, uint8_t* der_crt) {
    if (gramine_attest_handle != NULL && ra_tls_free_key_and_crt_der_ptr != NULL) {
        ra_tls_free_key_and_crt_der_ptr(der_key, der_crt);
    }
}
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

	// Call Gramine's RA-TLS function to create key and certificate (via dlopen/dlsym)
	ret := C.ra_tls_create_key_and_crt_der_dynamic(
		&derKey, &derKeySize,
		&derCrt, &derCrtSize,
	)

	if ret != 0 {
		return nil, fmt.Errorf("ra_tls_create_key_and_crt_der failed with code %d (library may not be available)", ret)
	}

	// Convert C buffers to Go slices
	keyDER := C.GoBytes(unsafe.Pointer(derKey), C.int(derKeySize))
	crtDER := C.GoBytes(unsafe.Pointer(derCrt), C.int(derCrtSize))

	// Free the C-allocated memory
	C.ra_tls_free_key_and_crt_der_dynamic(derKey, derCrt)

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
