//go:build extract_quote
// +build extract_quote

package sgx

import (
"encoding/hex"
"fmt"
"io"
"net/http"
"os"
"testing"
)

// Test function to extract and verify Quote from certificate
// Run with: go test -tags extract_quote -run TestExtractQuote
func TestExtractQuote(t *testing.T) {
// Download certificate
fmt.Println("Downloading RA-TLS certificate...")
resp, err := http.Get("https://raw.githubusercontent.com/mccoysc/gramine/refs/heads/master/tools/sgx/ra-tls/test-ratls-cert.cert")
if err != nil {
t.Fatalf("Failed to download cert: %v", err)
}
defer resp.Body.Close()

certPEM, err := io.ReadAll(resp.Body)
if err != nil {
t.Fatalf("Failed to read cert: %v", err)
}
fmt.Printf("✓ Certificate downloaded (%d bytes)\n\n", len(certPEM))

// Extract Quote from certificate
fmt.Println("=== Extracting Quote from Certificate ===")
verifier := NewDCAPVerifier(true)

quote, err := verifier.ExtractQuoteFromInput(certPEM)
if err != nil {
t.Fatalf("Failed to extract quote: %v", err)
}

fmt.Printf("✓ Quote extracted successfully\n")
fmt.Printf("  Size: %d bytes\n\n", len(quote))

// Verify the extracted Quote to ensure extraction was correct
fmt.Println("=== Verifying Extracted Quote ===")
os.Setenv("INTEL_SGX_API_KEY", "a8ece8747e7b4d8d98d23faec065b0b8")

err = verifier.VerifyQuote(quote)
if err != nil {
t.Fatalf("Quote verification failed: %v", err)
}

fmt.Printf("✓ Quote cryptographic signature verified\n")
fmt.Printf("✓ TCB status checked\n\n")

// Parse Quote to get measurements
fmt.Println("=== Quote Measurements ===")
parsedQuote, err := ParseQuote(quote)
if err != nil {
t.Fatalf("Failed to parse quote: %v", err)
}

fmt.Printf("  Version: %d\n", parsedQuote.Version)
fmt.Printf("  MRENCLAVE: %x\n", parsedQuote.MRENCLAVE)
fmt.Printf("  MRSIGNER: %x\n", parsedQuote.MRSIGNER)
fmt.Printf("\n")

// Write quote to file
outputFile := "/tmp/real_quote.bin"
err = os.WriteFile(outputFile, quote, 0644)
if err != nil {
t.Fatalf("Failed to write quote: %v", err)
}
fmt.Printf("✓ Quote written to %s\n\n", outputFile)

// Output hex format for test_env.sh
fmt.Println("========================================")
fmt.Println("Shell Variables for test_env.sh:")
fmt.Println("========================================")
fmt.Printf("REAL_MRENCLAVE=\"%x\"\n", parsedQuote.MRENCLAVE)
fmt.Printf("REAL_MRSIGNER=\"%x\"\n", parsedQuote.MRSIGNER)
fmt.Printf("REAL_QUOTE_HEX=\"%s\"\n", hex.EncodeToString(quote))
fmt.Println("========================================")

fmt.Println("\n✓ Quote extraction and verification complete!")
fmt.Println("The Quote has been verified to ensure no extraction errors.")
}
