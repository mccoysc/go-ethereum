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
#include <string.h>
#include <dlfcn.h>
#include <stdio.h>

// Callback function type for custom measurement verification
typedef int (*verify_measurements_cb_t)(const char* mrenclave, const char* mrsigner,
                                         const char* isv_prod_id, const char* isv_svn);

// Function pointers for dynamically loaded Gramine RA-TLS functions
static void* gramine_verify_handle = NULL;
static int (*ra_tls_verify_callback_der_ptr)(uint8_t*, size_t) = NULL;
static void (*ra_tls_set_measurement_callback_ptr)(verify_measurements_cb_t) = NULL;

// Initialize Gramine RA-TLS verify library via dlopen
static int init_gramine_verify_lib() {
    if (gramine_verify_handle != NULL) {
        return 0; // Already initialized
    }

    // Try to load the Gramine RA-TLS verify library
    gramine_verify_handle = dlopen("libra_tls_verify.so", RTLD_LAZY | RTLD_LOCAL);
    if (!gramine_verify_handle) {
        // Library not found - this is OK in non-SGX environments
        return -1;
    }

    // Load function symbols
    ra_tls_verify_callback_der_ptr = dlsym(gramine_verify_handle, "ra_tls_verify_callback_der");
    if (!ra_tls_verify_callback_der_ptr) {
        dlclose(gramine_verify_handle);
        gramine_verify_handle = NULL;
        return -1;
    }

    ra_tls_set_measurement_callback_ptr = dlsym(gramine_verify_handle, "ra_tls_set_measurement_callback");
    if (!ra_tls_set_measurement_callback_ptr) {
        dlclose(gramine_verify_handle);
        gramine_verify_handle = NULL;
        return -1;
    }

    return 0;
}

// Wrapper functions that use dlopen/dlsym to call Gramine functions
static int ra_tls_verify_callback_der_dynamic(uint8_t* der_crt, size_t der_crt_size) {
    if (init_gramine_verify_lib() != 0) {
        return -10000; // Library not available
    }
    
    return ra_tls_verify_callback_der_ptr(der_crt, der_crt_size);
}

static void ra_tls_set_measurement_callback_dynamic(verify_measurements_cb_t f_cb) {
    if (init_gramine_verify_lib() == 0 && ra_tls_set_measurement_callback_ptr != NULL) {
        ra_tls_set_measurement_callback_ptr(f_cb);
    }
}

// Global storage for allowed measurements (accessed by callback)
static char** g_allowed_mrenclaves = NULL;
static int g_allowed_mrenclaves_count = 0;
static char** g_allowed_mrsigners = NULL;
static int g_allowed_mrsigners_count = 0;

// Custom verification callback implementation
int custom_verify_measurements(const char* mrenclave, const char* mrsigner,
                                 const char* isv_prod_id, const char* isv_svn) {
	// Check MRENCLAVE
	int mrenclave_valid = 0;
	for (int i = 0; i < g_allowed_mrenclaves_count; i++) {
		if (strcmp(mrenclave, g_allowed_mrenclaves[i]) == 0) {
			mrenclave_valid = 1;
			break;
		}
	}

	// Check MRSIGNER
	int mrsigner_valid = 0;
	if (g_allowed_mrsigners_count == 0) {
		// If no MRSIGNER whitelist, accept any
		mrsigner_valid = 1;
	} else {
		for (int i = 0; i < g_allowed_mrsigners_count; i++) {
			if (strcmp(mrsigner, g_allowed_mrsigners[i]) == 0) {
				mrsigner_valid = 1;
				break;
			}
		}
	}

	return (mrenclave_valid && mrsigner_valid) ? 0 : -1;
}

// Helper function to set allowed measurements
void set_allowed_measurements(char** mrenclaves, int mrenclave_count,
                                char** mrsigners, int mrsigner_count) {
	g_allowed_mrenclaves = mrenclaves;
	g_allowed_mrenclaves_count = mrenclave_count;
	g_allowed_mrsigners = mrsigners;
	g_allowed_mrsigners_count = mrsigner_count;
}
*/
import "C"
import (
	"crypto/x509"
	"fmt"
	"sync"
	"unsafe"
)

// GramineRATLSVerifier implements quote verification using Gramine's RA-TLS library.
type GramineRATLSVerifier struct {
	mu               sync.RWMutex
	allowedMREnclave map[string]bool
	allowedMRSigner  map[string]bool
	allowOutdatedTCB bool
}

// NewGramineRATLSVerifier creates a new Gramine RA-TLS verifier.
func NewGramineRATLSVerifier(allowOutdatedTCB bool) *GramineRATLSVerifier {
	verifier := &GramineRATLSVerifier{
		allowedMREnclave: make(map[string]bool),
		allowedMRSigner:  make(map[string]bool),
		allowOutdatedTCB: allowOutdatedTCB,
	}

	// Register our custom verification callback (via dlopen/dlsym)
	C.ra_tls_set_measurement_callback_dynamic(C.verify_measurements_cb_t(C.custom_verify_measurements))

	return verifier
}

// VerifyQuote verifies an SGX quote using basic parsing.
// For full verification with signature checking, use VerifyCertificate.
// NO whitelist check - whitelist is only for RA-TLS certificate verification.
func (v *GramineRATLSVerifier) VerifyQuote(quote []byte) error {
	// Parse the quote to check basic structure
	_, err := ParseQuote(quote)
	if err != nil {
		return fmt.Errorf("failed to parse quote: %w", err)
	}

	// NO MRENCLAVE whitelist check here!
	// Whitelist is only checked during RA-TLS certificate verification (VerifyCertificate method)

	// For full signature verification, the quote needs to be embedded in a certificate
	// and verified via VerifyCertificate
	return nil
}

// VerifyCertificate verifies an RA-TLS certificate using Gramine's native library.
// This performs full cryptographic verification including SGX quote signature.
func (v *GramineRATLSVerifier) VerifyCertificate(cert *x509.Certificate) error {
	// Prepare whitelist arrays while holding the lock
	v.mu.RLock()
	
	// Convert whitelist to C arrays for the callback
	mrenclaveList := make([]*C.char, 0, len(v.allowedMREnclave))
	for mrenclave := range v.allowedMREnclave {
		mrenclaveList = append(mrenclaveList, C.CString(mrenclave))
	}

	mrsignerList := make([]*C.char, 0, len(v.allowedMRSigner))
	for mrsigner := range v.allowedMRSigner {
		mrsignerList = append(mrsignerList, C.CString(mrsigner))
	}
	
	// Release the lock before calling C functions
	v.mu.RUnlock()
	
	// Defer cleanup of C strings
	defer func() {
		for _, s := range mrenclaveList {
			C.free(unsafe.Pointer(s))
		}
		for _, s := range mrsignerList {
			C.free(unsafe.Pointer(s))
		}
	}()

	// Set the allowed measurements for the callback
	var mrenclavePtr **C.char
	var mrsignerPtr **C.char
	if len(mrenclaveList) > 0 {
		mrenclavePtr = &mrenclaveList[0]
	}
	if len(mrsignerList) > 0 {
		mrsignerPtr = &mrsignerList[0]
	}

	C.set_allowed_measurements(
		mrenclavePtr, C.int(len(mrenclaveList)),
		mrsignerPtr, C.int(len(mrsignerList)),
	)

	// Get the DER-encoded certificate
	certDER := cert.Raw

	// Call Gramine's verification function (via dlopen/dlsym)
	ret := C.ra_tls_verify_callback_der_dynamic(
		(*C.uint8_t)(unsafe.Pointer(&certDER[0])),
		C.size_t(len(certDER)),
	)

	if ret != 0 {
		return fmt.Errorf("RA-TLS certificate verification failed with code %d (library may not be available)", ret)
	}

	return nil
}

// IsAllowedMREnclave checks if the given MRENCLAVE is in the whitelist.
func (v *GramineRATLSVerifier) IsAllowedMREnclave(mrenclave []byte) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if len(v.allowedMREnclave) == 0 {
		// Empty whitelist means allow all
		return true
	}

	return v.allowedMREnclave[string(mrenclave)]
}

// AddAllowedMREnclave adds an MRENCLAVE to the whitelist.
func (v *GramineRATLSVerifier) AddAllowedMREnclave(mrenclave []byte) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.allowedMREnclave[string(mrenclave)] = true
}

// RemoveAllowedMREnclave removes an MRENCLAVE from the whitelist.
func (v *GramineRATLSVerifier) RemoveAllowedMREnclave(mrenclave []byte) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.allowedMREnclave, string(mrenclave))
}
