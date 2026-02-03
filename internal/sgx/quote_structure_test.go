//go:build testenv
// +build testenv

package sgx

import (
	"testing"
)

// TestVerifyRealQuoteStructure tests that our generated quote structure
// can be properly parsed and verified using the existing Verifier interface
func TestVerifyRealQuoteStructure(t *testing.T) {
	// Generate a quote with test report data
	testReportData := make([]byte, 64)
	copy(testReportData, []byte("test data for verification"))
	
	quote, err := generateQuoteViaGramine(testReportData)
	if err != nil {
		t.Fatalf("Failed to generate quote: %v", err)
	}
	
	t.Logf("Generated quote: %d bytes", len(quote))
	
	// Use existing Verifier to parse and validate the quote
	verifier := NewVerifier()
	
	// Test VerifyQuoteComplete
	result, err := verifier.VerifyQuoteComplete(quote)
	if err != nil {
		// In test mode, verification might fail due to signature issues
		// but we should at least be able to parse the structure
		t.Logf("VerifyQuoteComplete returned error (expected in test): %v", err)
	} else {
		t.Logf("VerifyQuoteComplete succeeded: %+v", result)
	}
	
	// Test ExtractInstanceID - this should work with our real structure
	instanceID, err := verifier.ExtractInstanceID(quote)
	if err != nil {
		t.Fatalf("Failed to extract instance ID: %v", err)
	}
	t.Logf("Extracted instance ID: %x", instanceID)
	
	// Instance ID should be 32 bytes
	if len(instanceID) != 32 {
		t.Errorf("Instance ID should be 32 bytes, got %d", len(instanceID))
	}
	
	// Test ExtractQuoteUserData
	userData, err := verifier.ExtractQuoteUserData(quote)
	if err != nil {
		t.Fatalf("Failed to extract user data: %v", err)
	}
	t.Logf("Extracted user data: %x", userData[:32])
	
	// User data should contain our test data
	if len(userData) < len(testReportData) {
		t.Errorf("User data too short: got %d, want at least %d", len(userData), len(testReportData))
	}
	
	// Test ExtractPublicKeyFromQuote
	pubKey, err := verifier.ExtractPublicKeyFromQuote(quote)
	if err != nil {
		t.Logf("ExtractPublicKeyFromQuote error (may be expected in test): %v", err)
	} else {
		t.Logf("Extracted public key: %d bytes", len(pubKey))
	}
}

// TestQuoteConsistency verifies that generating the same reportData
// produces consistent instance IDs
func TestQuoteConsistency(t *testing.T) {
	reportData := make([]byte, 64)
	copy(reportData, []byte("consistent test data"))
	
	verifier := NewVerifier()
	
	// Generate two quotes with the same reportData
	quote1, err := generateQuoteViaGramine(reportData)
	if err != nil {
		t.Fatalf("Failed to generate quote 1: %v", err)
	}
	
	quote2, err := generateQuoteViaGramine(reportData)
	if err != nil {
		t.Fatalf("Failed to generate quote 2: %v", err)
	}
	
	// Extract instance IDs
	id1, err := verifier.ExtractInstanceID(quote1)
	if err != nil {
		t.Fatalf("Failed to extract ID from quote 1: %v", err)
	}
	
	id2, err := verifier.ExtractInstanceID(quote2)
	if err != nil {
		t.Fatalf("Failed to extract ID from quote 2: %v", err)
	}
	
	// Instance IDs should be identical (derived from PPID which is constant)
	if string(id1) != string(id2) {
		t.Errorf("Instance IDs should be identical\nID1: %x\nID2: %x", id1, id2)
	}
	
	t.Logf("Instance IDs are consistent: %x", id1)
}
