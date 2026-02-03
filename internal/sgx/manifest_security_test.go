package sgx

import (
"bytes"
"encoding/hex"
"os"
"path/filepath"
"testing"
)

// TestManifestSecurityRequirement tests the user's security requirement:
// "既然要读manifest内容，如果是从外部不受保护环境读的，就是要被验证才行"
// Translation: "If reading manifest from external unprotected environment, MUST verify"
func TestManifestSecurityRequirement(t *testing.T) {
// Create a temporary test manifest file
tmpDir := t.TempDir()
manifestPath := filepath.Join(tmpDir, "test.manifest.sgx")

// Create fake SIGSTRUCT with known MRENCLAVE
sigstruct := make([]byte, 1808)
// Put a known MRENCLAVE at offset 960
knownMREnclave := []byte{
0xaa, 0xbb, 0xcc, 0xdd, 0x11, 0x22, 0x33, 0x44,
0x55, 0x66, 0x77, 0x88, 0x99, 0x00, 0xff, 0xee,
0xaa, 0xbb, 0xcc, 0xdd, 0x11, 0x22, 0x33, 0x44,
0x55, 0x66, 0x77, 0x88, 0x99, 0x00, 0xff, 0xee,
}
copy(sigstruct[960:992], knownMREnclave)

// Create minimal TOML manifest
manifestTOML := `[sgx]
enclave_size = "1G"
thread_num = 16
`

// Write complete manifest file
manifestData := append(sigstruct, []byte(manifestTOML)...)
if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
t.Fatal(err)
}

t.Run("Security_Violation_When_MRENCLAVE_Mismatch", func(t *testing.T) {
// Set runtime MRENCLAVE to DIFFERENT value (simulate tampered file)
fakeMREnclave := make([]byte, 32)
copy(fakeMREnclave, knownMREnclave)
fakeMREnclave[0] = 0x00 // Change first byte

os.Setenv("RA_TLS_MRENCLAVE", hex.EncodeToString(fakeMREnclave))
defer os.Unsetenv("RA_TLS_MRENCLAVE")

// Try to read - should FAIL with security error
_, err := ReadAndVerifyManifestFromDisk(manifestPath)

if err == nil {
t.Fatal("Expected security violation error, got nil")
}

errMsg := err.Error()
if len(errMsg) < 18 || errMsg[:18] != "SECURITY VIOLATION" {
t.Errorf("Expected SECURITY VIOLATION error, got: %v", err)
}

t.Logf("✓ Correctly detected manifest tampering: %v", err)
})

t.Run("Successful_Verification_When_MRENCLAVE_Match", func(t *testing.T) {
// Set runtime MRENCLAVE to SAME value (correct file)
os.Setenv("RA_TLS_MRENCLAVE", hex.EncodeToString(knownMREnclave))
defer os.Unsetenv("RA_TLS_MRENCLAVE")

// Try to read - should SUCCEED
config, err := ReadAndVerifyManifestFromDisk(manifestPath)

if err != nil {
t.Fatalf("Unexpected error: %v", err)
}

if config == nil {
t.Fatal("Config should not be nil")
}

if config.SGX.EnclaveSize != "1G" {
t.Errorf("Expected enclave size 1G, got %s", config.SGX.EnclaveSize)
}

t.Logf("✓ Successfully verified manifest - MRENCLAVE matches")
})

t.Run("Skip_Verification_When_Not_In_SGX", func(t *testing.T) {
// Don't set RA_TLS_MRENCLAVE (non-SGX mode)
os.Unsetenv("RA_TLS_MRENCLAVE")

// Try to read - should succeed with warning
config, err := ReadAndVerifyManifestFromDisk(manifestPath)

if err != nil {
t.Fatalf("Unexpected error in non-SGX mode: %v", err)
}

if config == nil {
t.Fatal("Config should not be nil")
}

t.Logf("✓ Successfully read manifest in non-SGX mode (verification skipped)")
})
}

// TestMREnclaveExtraction verifies we correctly extract MRENCLAVE from SIGSTRUCT
func TestMREnclaveExtraction(t *testing.T) {
// Create fake SIGSTRUCT
sigstruct := make([]byte, 1808)

// Put known value at MRENCLAVE offset (960)
expectedMREnclave := []byte{
0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
}
copy(sigstruct[960:992], expectedMREnclave)

// Extract MRENCLAVE
extractedMREnclave := sigstruct[960:992]

if !bytes.Equal(extractedMREnclave, expectedMREnclave) {
t.Errorf("MRENCLAVE extraction failed\nExpected: %x\nGot:      %x",
expectedMREnclave, extractedMREnclave)
}

t.Logf("✓ Correctly extracted MRENCLAVE from offset 960")
}
