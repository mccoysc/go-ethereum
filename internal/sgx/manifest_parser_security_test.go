package sgx

import (
"encoding/hex"
"os"
"testing"
)

// TestReadAndVerifyManifestFromDisk_MREnclaveMismatch tests security violation detection
func TestReadAndVerifyManifestFromDisk_MREnclaveMismatch(t *testing.T) {
// Use a real manifest file
manifestPath := "gramine/test.manifest.sgx"
if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
t.Skip("Test manifest file not found")
}

// Read the file to get its MRENCLAVE
data, err := os.ReadFile(manifestPath)
if err != nil {
t.Fatal(err)
}

fileMREnclave := data[960:992]

// Set runtime MRENCLAVE to a DIFFERENT value (simulate tampered file)
fakeMREnclave := make([]byte, 32)
copy(fakeMREnclave, fileMREnclave)
fakeMREnclave[0] ^= 0xFF // Flip first byte to make it different

os.Setenv("RA_TLS_MRENCLAVE", hex.EncodeToString(fakeMREnclave))
defer os.Unsetenv("RA_TLS_MRENCLAVE")

// Try to read and verify - should FAIL
_, err = ReadAndVerifyManifestFromDisk(manifestPath)
if err == nil {
t.Fatal("Expected security violation error, got nil")
}

if err.Error()[:18] != "SECURITY VIOLATION" {
t.Errorf("Expected SECURITY VIOLATION error, got: %v", err)
}

t.Logf("Correctly detected manifest tampering: %v", err)
}

// TestReadAndVerifyManifestFromDisk_MREnclaveMatch tests successful verification
func TestReadAndVerifyManifestFromDisk_MREnclaveMatch(t *testing.T) {
// Use a real manifest file
manifestPath := "gramine/test.manifest.sgx"
if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
t.Skip("Test manifest file not found")
}

// Read the file to get its MRENCLAVE
data, err := os.ReadFile(manifestPath)
if err != nil {
t.Fatal(err)
}

fileMREnclave := data[960:992]

// Set runtime MRENCLAVE to the SAME value (simulate correct file)
os.Setenv("RA_TLS_MRENCLAVE", hex.EncodeToString(fileMREnclave))
defer os.Unsetenv("RA_TLS_MRENCLAVE")

// Try to read and verify - should SUCCEED
config, err := ReadAndVerifyManifestFromDisk(manifestPath)
if err != nil {
t.Fatalf("Unexpected error: %v", err)
}

if config == nil {
t.Fatal("Config should not be nil")
}

t.Logf("Successfully verified manifest - MRENCLAVE matches")
t.Logf("Enclave size: %s", config.SGX.EnclaveSize)
}

// TestReadAndVerifyManifestFromDisk_NoMREnclave tests behavior when not in SGX
func TestReadAndVerifyManifestFromDisk_NoMREnclave(t *testing.T) {
// Use a real manifest file
manifestPath := "gramine/test.manifest.sgx"
if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
t.Skip("Test manifest file not found")
}

// Don't set RA_TLS_MRENCLAVE (simulate non-SGX mode)
os.Unsetenv("RA_TLS_MRENCLAVE")

// Try to read - should succeed with warning
config, err := ReadAndVerifyManifestFromDisk(manifestPath)
if err != nil {
t.Fatalf("Unexpected error in non-SGX mode: %v", err)
}

if config == nil {
t.Fatal("Config should not be nil")
}

t.Logf("Successfully read manifest in non-SGX mode (verification skipped)")
}
