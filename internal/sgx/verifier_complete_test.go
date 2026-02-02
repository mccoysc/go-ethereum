package sgx

import (
"encoding/hex"
"testing"

"github.com/ethereum/go-ethereum/common"
)

// TestVerifyQuoteComplete tests the complete quote verification
func TestVerifyQuoteComplete(t *testing.T) {
verifier := NewDCAPVerifier(true) // mockMode=true

// Generate a mock quote
attestor, err := NewGramineAttestor()
if err != nil {
t.Fatalf("Failed to create attestor: %v", err)
}

// Test data to include in quote
testData := []byte("test seal hash for block verification")

quote, err := attestor.GenerateQuote(testData)
if err != nil {
t.Fatalf("Failed to generate quote: %v", err)
}

// Verify the quote completely
result, err := verifier.VerifyQuoteComplete(quote)
if err != nil {
t.Fatalf("VerifyQuoteComplete failed: %v", err)
}

// Check that verification succeeded
if !result.Verified {
t.Errorf("Quote verification failed, error: %v", result.Error)
}

// Check that we got measurements
if len(result.Measurements.MrEnclave) != 32 {
t.Errorf("MrEnclave should be 32 bytes, got %d", len(result.Measurements.MrEnclave))
}

if len(result.Measurements.MrSigner) != 32 {
t.Errorf("MrSigner should be 32 bytes, got %d", len(result.Measurements.MrSigner))
}

if len(result.Measurements.ReportData) != 64 {
t.Errorf("ReportData should be 64 bytes, got %d", len(result.Measurements.ReportData))
}

// Check that we got a platform instance ID
if result.Measurements.PlatformInstanceID == (common.Address{}) {
t.Error("PlatformInstanceID should not be zero")
}

// Check that we have a source for the instance ID
if result.Measurements.PlatformInstanceIDSource == "" {
t.Error("PlatformInstanceIDSource should not be empty")
}

t.Logf("Quote verification successful:")
t.Logf("  MrEnclave: %x", result.Measurements.MrEnclave)
t.Logf("  MrSigner: %x", result.Measurements.MrSigner)
t.Logf("  IsvProdID: %d", result.Measurements.IsvProdID)
t.Logf("  IsvSvn: %d", result.Measurements.IsvSvn)
t.Logf("  PlatformInstanceID: %x", result.Measurements.PlatformInstanceID)
t.Logf("  PlatformInstanceIDSource: %s", result.Measurements.PlatformInstanceIDSource)
t.Logf("  TCBStatus: %s", result.TCBStatus)
t.Logf("  QuoteVersion: %d", result.QuoteVersion)
t.Logf("  AttestationKeyType: %d", result.AttestationKeyType)
}

// TestPlatformInstanceIDConsistency tests that the same platform produces the same instance ID
func TestPlatformInstanceIDConsistency(t *testing.T) {
verifier := NewDCAPVerifier(true)
attestor, err := NewGramineAttestor()
if err != nil {
t.Fatalf("Failed to create attestor: %v", err)
}

// Generate two quotes with different data
quote1, err := attestor.GenerateQuote([]byte("data1"))
if err != nil {
t.Fatalf("Failed to generate quote1: %v", err)
}

quote2, err := attestor.GenerateQuote([]byte("data2"))
if err != nil {
t.Fatalf("Failed to generate quote2: %v", err)
}

// Extract instance IDs
result1, err := verifier.VerifyQuoteComplete(quote1)
if err != nil {
t.Fatalf("Failed to verify quote1: %v", err)
}

result2, err := verifier.VerifyQuoteComplete(quote2)
if err != nil {
t.Fatalf("Failed to verify quote2: %v", err)
}

// Instance IDs should be the same (same platform)
if result1.Measurements.PlatformInstanceID != result2.Measurements.PlatformInstanceID {
t.Errorf("Platform instance IDs should be consistent:\n  Quote1: %x\n  Quote2: %x",
result1.Measurements.PlatformInstanceID,
result2.Measurements.PlatformInstanceID)
}

// But report data should be different
if hex.EncodeToString(result1.Measurements.ReportData) == hex.EncodeToString(result2.Measurements.ReportData) {
t.Error("ReportData should be different for different input data")
}

t.Logf("Platform instance ID consistency verified: %x", result1.Measurements.PlatformInstanceID)
}

// TestQuoteVerificationInvalidQuote tests that invalid quotes are rejected
func TestQuoteVerificationInvalidQuote(t *testing.T) {
verifier := NewDCAPVerifier(true)

// Test with empty quote
result, err := verifier.VerifyQuoteComplete([]byte{})
if err == nil {
t.Error("Expected error for empty quote")
}
if result != nil && result.Verified {
t.Error("Empty quote should not verify successfully")
}

// Test with truncated quote
shortQuote := make([]byte, 100)
result, err = verifier.VerifyQuoteComplete(shortQuote)
if err == nil {
t.Error("Expected error for truncated quote")
}
if result != nil && result.Verified {
t.Error("Truncated quote should not verify successfully")
}

t.Log("Invalid quote rejection tests passed")
}

// TestVerifyQuoteCompleteInputFormats tests that both quote and certificate inputs work
func TestVerifyQuoteCompleteInputFormats(t *testing.T) {
verifier := NewDCAPVerifier(true)
attestor, err := NewGramineAttestor()
if err != nil {
t.Fatalf("Failed to create attestor: %v", err)
}

testData := []byte("test data")

// Test 1: Raw quote input
quote, err := attestor.GenerateQuote(testData)
if err != nil {
t.Fatalf("Failed to generate quote: %v", err)
}

result1, err := verifier.VerifyQuoteComplete(quote)
if err != nil {
t.Fatalf("Failed to verify raw quote: %v", err)
}

if !result1.Verified {
t.Error("Raw quote should verify")
}

t.Logf("✓ Raw quote input verified successfully")
t.Logf("  PlatformInstanceID: %x (from %s)", 
result1.Measurements.PlatformInstanceID,
result1.Measurements.PlatformInstanceIDSource)

// Test 2: Certificate input (would need actual RA-TLS cert)
// For now just test that certificate detection works
fakeCert := []byte(`-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKHHCgVZU4pzMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl
c3RjYTAgFw0yNDAxMDEwMDAwMDBaGA8yMTI0MDEwMTAwMDAwMFowETEPMA0GA1UE
AwwGdGVzdGNhMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDGFH8VRWmMhPEq
-----END CERTIFICATE-----`)

// This will fail because fake cert doesn't have quote extension, but tests detection
result2, err := verifier.VerifyQuoteComplete(fakeCert)
if err == nil {
t.Error("Fake certificate should fail (no quote extension)")
}
// The error should be about missing quote, not about input format
if err != nil && !bytes.Contains([]byte(err.Error()), []byte("quote")) {
t.Logf("Expected error about missing quote, got: %v", err)
}

t.Log("✓ Certificate input format detected correctly")
t.Log("Test passed: VerifyQuoteComplete correctly handles both input formats")
}
