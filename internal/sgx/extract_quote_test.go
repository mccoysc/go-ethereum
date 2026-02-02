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

// Test function to extract Quote from certificate - run with: go test -tags extract_quote -run TestExtractQuote
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

// Parse Quote to get MRENCLAVE and MRSIGNER
parsedQuote, err := ParseQuote(quote)
if err != nil {
t.Fatalf("Failed to parse quote: %v", err)
}

fmt.Printf("✓ Quote parsed successfully\n")
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

fmt.Println("\n✓ Quote extraction complete!")
}
