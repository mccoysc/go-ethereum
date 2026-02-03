//go:build testenv
// +build testenv

package sgx

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestVerifyRealQuoteFromCertificate(t *testing.T) {
	// Load the real quote
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	quotePath := filepath.Join(dir, "testdata", "gramine_ratls_quote.bin")

	quote, err := os.ReadFile(quotePath)
	if err != nil {
		t.Fatalf("Failed to load real quote: %v", err)
	}

	t.Logf("Loaded real quote: %d bytes", len(quote))

	// Parse the quote structure
	parsedQuote, err := ParseQuote(quote)
	if err != nil {
		t.Fatalf("ParseQuote failed: %v", err)
	}

	t.Logf("Quote parsed successfully:")
	t.Logf("  Version: %d", parsedQuote.Version)
	t.Logf("  MRENCLAVE: %x", parsedQuote.MRENCLAVE)
	t.Logf("  MRSIGNER: %x", parsedQuote.MRSIGNER)
	t.Logf("  ReportData: %x", parsedQuote.ReportData[:32])

	// Extract instance ID using the extraction function
	instanceID, err := ExtractInstanceID(quote)
	if err != nil {
		t.Fatalf("ExtractInstanceID failed: %v", err)
	}

	t.Logf("Extracted instance ID: %x", instanceID.CPUInstanceID)

	// Verify it's not all zeros
	allZeros := true
	for _, b := range instanceID.CPUInstanceID {
		if b != 0 {
			allZeros = false
			break
		}
	}
	
	if allZeros {
		t.Error("Instance ID is all zeros - this might be a problem")
	} else {
		t.Logf("Instance ID is valid (not all zeros)")
	}
}
