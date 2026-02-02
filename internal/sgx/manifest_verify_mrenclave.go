// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package sgx

import (
"bytes"
"encoding/binary"
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
sigstructSize        = 1808
sigstructMREnclaveOffset = 128
sigstructMRSignerOffset  = 960
mrenclaveSize       = 32
mrsignerSize        = 32
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

// extractMRSignerFromSIGSTRUCT extracts MRSIGNER from a manifest.sgx file
func extractMRSignerFromSIGSTRUCT(manifestPath string) ([]byte, error) {
data, err := os.ReadFile(manifestPath)
if err != nil {
return nil, fmt.Errorf("failed to read manifest.sgx: %w", err)
}

if len(data) < sigstructSize {
return nil, fmt.Errorf("manifest.sgx file too small")
}

// Extract MRSIGNER from SIGSTRUCT
// MRSIGNER is at offset 960, size 32 bytes
mrsigner := make([]byte, mrsignerSize)
copy(mrsigner, data[sigstructMRSignerOffset:sigstructMRSignerOffset+mrsignerSize])

log.Debug("Extracted MRSIGNER from manifest.sgx",
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
// Extract MRENCLAVE from manifest.sgx (SIGSTRUCT)
manifestMR, err := extractMREnclaveFromSIGSTRUCT(manifestPath)
if err != nil {
return fmt.Errorf("failed to extract MRENCLAVE from manifest: %w", err)
}

// Read MRENCLAVE from runtime (/dev/attestation)
runtimeMR, err := readRuntimeMREnclave()
if err != nil {
// In test mode, /dev/attestation might not be available
// The build-tag-specific verifyManifestMREnclave will handle this
return fmt.Errorf("failed to read runtime MRENCLAVE: %w", err)
}

// Delegate to build-tag-specific verification
// This will be either production (strict) or testenv (lenient)
return verifyManifestMREnclaveImpl(manifestMR, runtimeMR)
}

// verifyManifestMREnclaveImpl is implemented in build-tag-specific files:
// - manifest_verify_production.go (build tag: !testenv)
// - manifest_verify_testenv.go (build tag: testenv)
func verifyManifestMREnclaveImpl(manifestMR, runtimeMR []byte) error {
// This will be provided by build-tag-specific files
// Default implementation for compatibility
if !bytes.Equal(manifestMR, runtimeMR) {
return fmt.Errorf("MRENCLAVE mismatch: manifest=%x runtime=%x",
manifestMR, runtimeMR)
}
return nil
}

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
