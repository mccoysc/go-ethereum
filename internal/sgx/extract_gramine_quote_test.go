package sgx

import (
	"io/ioutil"
	"os"
	"testing"
)

// TestExtractQuoteFromGramineCert extracts the real quote from Gramine's test certificate
func TestExtractQuoteFromGramineCert(t *testing.T) {
	// Read the certificate from the URL provided by user
	// First download it if not already present
	certPath := "/tmp/ratls.cert"
	certData, err := ioutil.ReadFile(certPath)
	if err != nil {
		t.Skipf("Certificate not found at %s, skipping test", certPath)
		return
	}
	
	t.Logf("Read certificate: %d bytes", len(certData))
	
	// Create verifier (verifyMREnclave=false for testing)
	verifier := NewDCAPVerifier(false)
	
	// Extract quote using existing tool
	quote, err := verifier.ExtractQuoteFromInput(certData)
	if err != nil {
		t.Fatalf("Error extracting quote: %v", err)
	}
	
	t.Logf("Extracted quote: %d bytes", len(quote))
	
	// Save to testdata
	testdataDir := "testdata"
	os.MkdirAll(testdataDir, 0755)
	quotePath := testdataDir + "/gramine_ratls_quote.bin"
	err = ioutil.WriteFile(quotePath, quote, 0644)
	if err != nil {
		t.Fatalf("Error saving quote: %v", err)
	}
	
	t.Logf("Saved to %s", quotePath)
	
	// Verify it can be parsed
	t.Logf("First 16 bytes: %x", quote[:16])
	
	// Extract MRENCLAVE
	mrenclave, err := ExtractMREnclave(quote)
	if err != nil {
		t.Logf("MRENCLAVE extraction: %v", err)
	} else {
		t.Logf("MRENCLAVE: %x", mrenclave)
	}
	
	// Extract instance ID  
	instanceID, err := verifier.ExtractInstanceID(quote)
	if err != nil {
		t.Logf("Instance ID extraction: %v", err)
	} else {
		t.Logf("Instance ID: %x", instanceID)
	}
	
	// Extract user data
	userData, err := verifier.ExtractQuoteUserData(quote)
	if err != nil {
		t.Logf("User data extraction: %v", err)
	} else {
		t.Logf("User data (first 32 bytes): %x", userData[:32])
	}
	
	// Verify quote can be used for verification
	result, err := verifier.VerifyQuoteComplete(quote, nil)
	if err != nil {
		t.Logf("Verification error (may be expected if certs not available): %v", err)
	} else {
		t.Logf("Verification result: %+v", result)
	}
}
