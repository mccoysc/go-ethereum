//go:build ignore
// +build ignore

// Tool to extract Quote from RA-TLS certificate and verify it
package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Import from parent package - we're in internal/sgx
// When running with `go run`, we're actually in package main but can access sgx types

func main() {
	var certPEM []byte
	var err error
	
	if len(os.Args) < 2 {
		fmt.Println("No certificate file provided, downloading default...")
		// Download from Gramine repository
		resp, err := http.Get("https://raw.githubusercontent.com/mccoysc/gramine/refs/heads/master/tools/sgx/ra-tls/test-ratls-cert.cert")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error downloading certificate: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		
		certPEM, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading certificate: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Certificate downloaded successfully")
	} else {
		certFile := os.Args[1]
		certPEM, err = os.ReadFile(certFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading certificate: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Certificate loaded from %s\n", certFile)
	}

	fmt.Println("\n=== Extracting Quote from Certificate ===")

	// Create verifier - must match the actual exported API
	// DCAPVerifier is not exported, so we need to use the factory or create it properly
	// Looking at verifier_impl.go, we need to create it directly
	verifier := &DCAPVerifier{
		allowNetworkAccess: true,
		allowedMREnclaves:  make(map[string]bool),
		allowedMRSigners:   make(map[string]bool),
	}

	// Extract quote using the exported method
	quote, err := verifier.ExtractQuoteFromInput(certPEM)
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
	fmt.Printf("  MRENCLAVE: %x\n", result.Measurements.MRENCLAVE)
	fmt.Printf("  MRSIGNER: %x\n", result.Measurements.MRSIGNER)
	fmt.Printf("  Platform Instance ID: %x\n", result.Measurements.PlatformInstanceID)
	fmt.Printf("\n")

	// Write quote to file
	outputFile := "/tmp/real_quote.bin"
	err = os.WriteFile(outputFile, quote, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing quote file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Quote written to %s\n\n", outputFile)

	// Output for test_env.sh
	fmt.Println("=== For test_env.sh ===")
	fmt.Printf("# Real verified Quote from Gramine RA-TLS certificate\n")
	fmt.Printf("# Size: %d bytes\n", len(quote))
	fmt.Printf("REAL_MRENCLAVE=\"%x\"\n", result.Measurements.MRENCLAVE)
	fmt.Printf("REAL_MRSIGNER=\"%x\"\n", result.Measurements.MRSIGNER)
	fmt.Printf("REAL_PLATFORM_INSTANCE_ID=\"%x\"\n", result.Measurements.PlatformInstanceID)
	fmt.Printf("REAL_QUOTE_HEX=\"%s\"\n", hex.EncodeToString(quote))
}
