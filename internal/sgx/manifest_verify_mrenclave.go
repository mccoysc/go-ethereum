// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package sgx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/log"
)

// SIGSTRUCT format based on Intel SGX specification
// https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-3d-part-4-manual.pdf
// Section 38.13: SIGSTRUCT Structure
//
// SIGSTRUCT is 1808 bytes total
// Offset 128-159 (32 bytes): MRENCLAVE
// Offset 160-191 (32 bytes): Reserved
// Offset 192-223 (32 bytes): ISVPRODID + ISVSVN
// Offset 960-991 (32 bytes): MRSIGNER

const (
	sigstructSize            = 1808
	sigstructMREnclaveOffset = 960 // Correct offset from Gramine sgx_arch.h (enclave_hash field)
	sigstructModulusOffset   = 128 // For calculating MRSIGNER = SHA256(modulus)
	mrenclaveSize            = 32
	mrsignerSize             = 32
	modulusSize              = 384 // RSA-3072 modulus size
)

// extractMREnclaveFromSIGSTRUCT extracts MRENCLAVE from a manifest.sgx file
// The manifest.sgx file contains the SIGSTRUCT structure which includes MRENCLAVE
func extractMREnclaveFromSIGSTRUCT(manifestPath string) ([]byte, error) {
// Read the manifest.sgx file
data, err := os.ReadFile(manifestPath)
if err != nil {
return nil, fmt.Errorf("failed to read manifest.sgx: %w", err)
}

// The manifest.sgx file format (Gramine):
// - First part: SIGSTRUCT (1808 bytes)
// - Rest: actual manifest content (TOML)

if len(data) < sigstructSize {
return nil, fmt.Errorf("manifest.sgx file too small: %d bytes (expected at least %d)", 
len(data), sigstructSize)
}

// Extract MRENCLAVE from SIGSTRUCT
// MRENCLAVE is at offset 128, size 32 bytes
mrenclave := make([]byte, mrenclaveSize)
copy(mrenclave, data[sigstructMREnclaveOffset:sigstructMREnclaveOffset+mrenclaveSize])

// Validate MRENCLAVE is not all zeros (invalid)
allZero := true
for _, b := range mrenclave {
if b != 0 {
allZero = false
break
}
}
if allZero {
return nil, fmt.Errorf("invalid MRENCLAVE: all zeros")
}

log.Debug("Extracted MRENCLAVE from manifest.sgx", 
"mrenclave", hex.EncodeToString(mrenclave))

return mrenclave, nil
}

// extractMRSignerFromSIGSTRUCT calculates MRSIGNER from a manifest.sgx file
// MRSIGNER = SHA256(modulus), as per Intel SGX specification
func extractMRSignerFromSIGSTRUCT(manifestPath string) ([]byte, error) {
data, err := os.ReadFile(manifestPath)
if err != nil {
return nil, fmt.Errorf("failed to read manifest.sgx: %w", err)
}

if len(data) < sigstructSize {
return nil, fmt.Errorf("manifest.sgx file too small")
}

// Extract modulus from SIGSTRUCT (offset 128, 384 bytes)
modulus := data[sigstructModulusOffset : sigstructModulusOffset+modulusSize]

// Calculate MRSIGNER = SHA256(modulus)
hash := sha256.Sum256(modulus)
mrsigner := hash[:]

log.Debug("Calculated MRSIGNER from SIGSTRUCT modulus",
"mrsigner", hex.EncodeToString(mrsigner))

return mrsigner, nil
}

// readRuntimeMREnclave reads MRENCLAVE from /dev/attestation/my_target_info
// This is the MRENCLAVE of the currently running enclave
func readRuntimeMREnclave() ([]byte, error) {
// Read from SGX attestation device
targetInfo, err := os.ReadFile("/dev/attestation/my_target_info")
if err != nil {
return nil, fmt.Errorf("failed to read /dev/attestation/my_target_info: %w", err)
}

// target_info structure contains MRENCLAVE in first 32 bytes
if len(targetInfo) < 32 {
return nil, fmt.Errorf("target_info too short: %d bytes", len(targetInfo))
}

mrenclave := make([]byte, 32)
copy(mrenclave, targetInfo[:32])

log.Debug("Read runtime MRENCLAVE from /dev/attestation",
"mrenclave", hex.EncodeToString(mrenclave))

return mrenclave, nil
}

// verifyMREnclaveConsistency compares manifest MRENCLAVE with runtime MRENCLAVE
// This is called by verifyManifestMREnclave (build tag specific versions)
func verifyMREnclaveConsistency(manifestPath string) error {
	// CRITICAL SECURITY STEP 1: Verify MRSIGNER (signing key) is trusted
	// This MUST be done BEFORE trusting any other data from the manifest
	// Read SIGSTRUCT to verify MRSIGNER
	sigstruct, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest for MRSIGNER verification: %w", err)
	}
	
	if len(sigstruct) < sigstructSize {
		return fmt.Errorf("manifest.sgx too small for SIGSTRUCT")
	}
	
	// Verify MRSIGNER is trusted (build-tag-specific: production validates, testenv may skip)
	if err := verifyMRSignerTrusted(sigstruct[:sigstructSize]); err != nil {
		return fmt.Errorf("MRSIGNER verification failed: %w", err)
	}
	
	// SECURITY STEP 2: Extract MRENCLAVE from manifest.sgx (SIGSTRUCT)
	// Now we can trust this data because we've verified the signing key
	manifestMR, err := extractMREnclaveFromSIGSTRUCT(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to extract MRENCLAVE from manifest: %w", err)
	}

	// SECURITY STEP 3: Read MRENCLAVE from runtime (/dev/attestation)
	runtimeMR, err := readRuntimeMREnclave()
	if err != nil {
		// In test mode, /dev/attestation might not be available
		// The build-tag-specific verifyManifestMREnclave will handle this
		return fmt.Errorf("failed to read runtime MRENCLAVE: %w", err)
	}

	// SECURITY STEP 4: Delegate to build-tag-specific verification
	// This will be either production (strict) or testenv (lenient)
	return verifyManifestMREnclaveImpl(manifestMR, runtimeMR)
}

// verifyManifestMREnclaveImpl is implemented in build-tag-specific files:
// - manifest_verify_production.go (build tag: !testenv) - strict validation
// - manifest_verify_testenv.go (build tag: testenv) - lenient validation

// ExtractMeasurementsFromManifest extracts both MRENCLAVE and MRSIGNER from manifest
func ExtractMeasurementsFromManifest(manifestPath string) (mrenclave, mrsigner []byte, err error) {
mrenclave, err = extractMREnclaveFromSIGSTRUCT(manifestPath)
if err != nil {
return nil, nil, fmt.Errorf("failed to extract MRENCLAVE: %w", err)
}

mrsigner, err = extractMRSignerFromSIGSTRUCT(manifestPath)
if err != nil {
return nil, nil, fmt.Errorf("failed to extract MRSIGNER: %w", err)
}

return mrenclave, mrsigner, nil
}
