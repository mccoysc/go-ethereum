//go:build testenv

package sgx

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRealQuoteInstanceID(t *testing.T) {
	// Load the real quote from Gramine RA-TLS certificate
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	quotePath := filepath.Join(dir, "testdata", "gramine_ratls_quote.bin")

	quote, err := os.ReadFile(quotePath)
	if err != nil {
		t.Fatalf("Failed to load real quote: %v", err)
	}

	t.Logf("Real quote size: %d bytes", len(quote))

	// Extract instance ID using our function
	instanceID, err := ExtractInstanceID(quote)
	if err != nil {
		t.Fatalf("Failed to extract instance ID: %v", err)
	}

	t.Logf("Instance ID (hex): %s", hex.EncodeToString(instanceID.CPUInstanceID))
	t.Logf("Instance ID length: %d bytes", len(instanceID.CPUInstanceID))
	t.Logf("Quote type: %d", instanceID.QuoteType)

	if len(instanceID.CPUInstanceID) != 32 {
		t.Errorf("Expected instance ID to be 32 bytes, got %d", len(instanceID.CPUInstanceID))
	}
	
	// This is the instance ID that will be used as producer ID in blocks
	t.Logf("This instance ID will be the producer ID for blocks produced in testenv")
}
