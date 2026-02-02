//go:build ignore
// +build ignore

// Tool to extract Quote from RA-TLS certificate and verify it
package main

import (
"encoding/hex"
"fmt"
"io/ioutil"
"os"
)

func main() {
if len(os.Args) < 2 {
fmt.Println("Usage: go run cmd_extract_and_verify_quote.go <cert-file>")
fmt.Println("Example: go run cmd_extract_and_verify_quote.go /tmp/test-ratls-cert.cert")
os.Exit(1)
}

certFile := os.Args[1]

// Read certificate
certPEM, err := ioutil.ReadFile(certFile)
if err != nil {
fmt.Fprintf(os.Stderr, "Error reading certificate: %v\n", err)
os.Exit(1)
}

fmt.Println("=== Extracting Quote from Certificate ===")

// Create verifier (we need to import from current package)
// Since this is in the sgx package, we can use the types directly
verifier := NewDCAPVerifier(true) // mock mode for testing

// Extract quote
quote, err := verifier.extractQuoteFromInput(certPEM)
if err != nil {
fmt.Fprintf(os.Stderr, "Error extracting quote: %v\n", err)
os.Exit(1)
}

fmt.Printf("✓ Quote extracted successfully\n")
fmt.Printf("  Size: %d bytes\n\n", len(quote))

// Verify the extracted quote
fmt.Println("=== Verifying Extracted Quote ===")

// Set API key for verification
os.Setenv("INTEL_SGX_API_KEY", "a8ece8747e7b4d8d98d23faec065b0b8")

result, err := verifier.VerifyQuoteComplete(quote, nil)
if err != nil {
fmt.Fprintf(os.Stderr, "Error verifying quote: %v\n", err)
os.Exit(1)
}

if !result.Verified {
fmt.Fprintf(os.Stderr, "Quote verification FAILED\n")
os.Exit(1)
}

fmt.Printf("✓ Quote verified successfully\n")
fmt.Printf("  TCB Status: %s\n", result.TCBStatus)
fmt.Printf("  MRENCLAVE: %x\n", result.Measurements.MrEnclave)
fmt.Printf("  MRSIGNER: %x\n", result.Measurements.MrSigner)
fmt.Printf("  Platform Instance ID: %x\n", result.Measurements.PlatformInstanceID)
fmt.Printf("\n")

// Write quote to file
outputFile := "/tmp/real_quote.bin"
err = ioutil.WriteFile(outputFile, quote, 0644)
if err != nil {
fmt.Fprintf(os.Stderr, "Error writing quote file: %v\n", err)
os.Exit(1)
}
fmt.Printf("✓ Quote written to %s\n\n", outputFile)

// Output for test_env.sh
fmt.Println("=== For test_env.sh ===")
fmt.Printf("# Real verified Quote from Gramine RA-TLS certificate\n")
fmt.Printf("# Size: %d bytes\n", len(quote))
fmt.Printf("# MRENCLAVE: %x\n", result.Measurements.MrEnclave)
fmt.Printf("# MRSIGNER: %x\n", result.Measurements.MrSigner)
fmt.Printf("REAL_QUOTE_HEX=\"%s\"\n", hex.EncodeToString(quote))
}
