//go:build testenv
// +build testenv

package sgx

import (
	"testing"
	
	internalsgx "github.com/ethereum/go-ethereum/internal/sgx"
)

func TestSealQuoteGeneration(t *testing.T) {
	// Create attestor
	attestor, err := internalsgx.NewGramineAttestor()
	if err != nil {
		t.Fatalf("NewGramineAttestor failed: %v", err)
	}

	// Generate quote (simulate what Seal does)
	reportData := make([]byte, 32)
	for i := range reportData {
		reportData[i] = byte(i)
	}

	quote, err := attestor.GenerateQuote(reportData)
	if err != nil {
		t.Fatalf("GenerateQuote failed: %v", err)
	}

	t.Logf("Generated quote: %d bytes", len(quote))

	// Verify quote (simulate what Seal does)
	verifier := internalsgx.NewDCAPVerifier(true)
	quoteResult, err := verifier.VerifyQuoteComplete(quote, nil)
	if err != nil {
		t.Fatalf("VerifyQuoteComplete failed: %v", err)
	}

	t.Logf("Verification result:")
	t.Logf("  Verified: %v", quoteResult.Verified)
	t.Logf("  PlatformInstanceID: %x", quoteResult.Measurements.PlatformInstanceID)
	t.Logf("  PlatformInstanceID Source: %s", quoteResult.Measurements.PlatformInstanceIDSource)
	t.Logf("  PlatformInstanceID length: %d", len(quoteResult.Measurements.PlatformInstanceID))

	// This is what Seal does
	producerID := quoteResult.Measurements.PlatformInstanceID[:]
	t.Logf("ProducerID (as slice): %x", producerID)
	t.Logf("ProducerID length: %d", len(producerID))

	// Check if it's all zeros
	allZeros := true
	for _, b := range producerID {
		if b != 0 {
			allZeros = false
			break
		}
	}

	if allZeros {
		t.Error("ProducerID is all zeros!")
	}
}
