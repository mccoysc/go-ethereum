//go:build testenv
// +build testenv

package sgx

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestVerifyQuoteCompleteOnRealQuote(t *testing.T) {
	// Load the real quote
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	quotePath := filepath.Join(dir, "testdata", "gramine_ratls_quote.bin")

	quote, err := os.ReadFile(quotePath)
	if err != nil {
		t.Fatalf("Failed to load real quote: %v", err)
	}

	t.Logf("Loaded real quote: %d bytes", len(quote))

	// Create verifier with testMode=true (same as in consensus.go)
	verifier := NewDCAPVerifier(true)

	// Call VerifyQuoteComplete
	result, err := verifier.VerifyQuoteComplete(quote, nil)
	if err != nil {
		t.Fatalf("VerifyQuoteComplete failed: %v", err)
	}

	t.Logf("Verification result:")
	t.Logf("  Verified: %v", result.Verified)
	t.Logf("  MrEnclave: %x", result.Measurements.MrEnclave)
	t.Logf("  MrSigner: %x", result.Measurements.MrSigner)
	t.Logf("  PlatformInstanceID: %x", result.Measurements.PlatformInstanceID)
	t.Logf("  PlatformInstanceID Source: %s", result.Measurements.PlatformInstanceIDSource)
	t.Logf("  PlatformInstanceID length: %d", len(result.Measurements.PlatformInstanceID))

	// Check if instance ID is empty
	if len(result.Measurements.PlatformInstanceID) == 0 {
		t.Error("PlatformInstanceID is empty!")
	}

	if result.Measurements.PlatformInstanceIDSource == "" {
		t.Error("PlatformInstanceIDSource is empty!")
	}
}
