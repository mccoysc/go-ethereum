// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package sgx_test

import (
	"crypto/x509"
	"fmt"
	"log"
	"os"

	"github.com/ethereum/go-ethereum/internal/sgx"
)

// Example_basicQuoteGeneration demonstrates basic SGX Quote generation.
func Example_basicQuoteGeneration() {
	// Set mock mode for examples
	os.Setenv("XCHAIN_SGX_MODE", "mock")
	defer os.Unsetenv("XCHAIN_SGX_MODE")

	// Create attestor (auto-detects SGX environment)
	attestor, err := sgx.NewGramineAttestor()
	if err != nil {
		log.Fatal(err)
	}

	// Prepare report data (e.g., hash of public key)
	reportData := []byte("example_public_key_hash")

	// Generate SGX Quote
	quote, err := attestor.GenerateQuote(reportData)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Generated SGX Quote of %d bytes\n", len(quote))
	// Output: Generated SGX Quote of 1060 bytes
}

// Example_certificateGenerationAndVerification demonstrates RA-TLS certificate
// generation and verification.
func Example_certificateGenerationAndVerification() {
	// Set mock mode for examples
	os.Setenv("XCHAIN_SGX_MODE", "mock")
	defer os.Unsetenv("XCHAIN_SGX_MODE")

	// Create attestor
	attestor, err := sgx.NewGramineAttestor()
	if err != nil {
		log.Fatal(err)
	}

	// Generate RA-TLS certificate
	cert, err := attestor.GenerateCertificate()
	if err != nil {
		log.Fatal(err)
	}

	// Parse the certificate
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		log.Fatal(err)
	}

	// Create verifier
	verifier := sgx.NewDCAPVerifier(true) // allow outdated TCB for testing

	// Add MRENCLAVE to whitelist
	mrenclave := attestor.GetMREnclave()
	verifier.AddAllowedMREnclave(mrenclave)

	// Verify certificate
	err = verifier.VerifyCertificate(x509Cert)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Certificate verified successfully")
	// Output: Certificate verified successfully
}

// Example_whitelistManagement demonstrates MRENCLAVE whitelist management.
func Example_whitelistManagement() {
	verifier := sgx.NewDCAPVerifier(false)

	// Create two different MRENCLAVEs
	mrenclave1 := make([]byte, 32)
	mrenclave2 := make([]byte, 32)
	for i := range mrenclave1 {
		mrenclave1[i] = byte(i)
		mrenclave2[i] = byte(i + 100)
	}

	// Add to whitelist
	verifier.AddAllowedMREnclave(mrenclave1)
	verifier.AddAllowedMREnclave(mrenclave2)

	// Check whitelist
	if verifier.IsAllowedMREnclave(mrenclave1) {
		fmt.Println("MRENCLAVE1 is allowed")
	}

	// Remove from whitelist
	verifier.RemoveAllowedMREnclave(mrenclave1)

	if !verifier.IsAllowedMREnclave(mrenclave1) {
		fmt.Println("MRENCLAVE1 removed from whitelist")
	}

	// Output:
	// MRENCLAVE1 is allowed
	// MRENCLAVE1 removed from whitelist
}

// Example_quoteExtraction demonstrates extracting information from SGX Quotes.
func Example_quoteExtraction() {
	// Set mock mode for examples
	os.Setenv("XCHAIN_SGX_MODE", "mock")
	defer os.Unsetenv("XCHAIN_SGX_MODE")

	// Create attestor
	attestor, err := sgx.NewGramineAttestor()
	if err != nil {
		log.Fatal(err)
	}

	// Generate quote
	quote, err := attestor.GenerateQuote([]byte("test_data"))
	if err != nil {
		log.Fatal(err)
	}

	// Extract MRENCLAVE
	mrenclave, err := sgx.ExtractMREnclave(quote)
	if err != nil {
		log.Fatal(err)
	}

	// Extract MRSIGNER
	mrsigner, err := sgx.ExtractMRSigner(quote)
	if err != nil {
		log.Fatal(err)
	}

	// Extract report data
	reportData, err := sgx.ExtractReportData(quote)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("MRENCLAVE: %d bytes\n", len(mrenclave))
	fmt.Printf("MRSIGNER: %d bytes\n", len(mrsigner))
	fmt.Printf("Report Data: %d bytes\n", len(reportData))

	// Output:
	// MRENCLAVE: 32 bytes
	// MRSIGNER: 32 bytes
	// Report Data: 64 bytes
}

// Example_constantTimeOperations demonstrates side-channel safe operations.
func Example_constantTimeOperations() {
	secret := []byte("secret_password")
	input := []byte("secret_password")

	// Use constant-time comparison (safe from timing attacks)
	if sgx.ConstantTimeCompare(secret, input) {
		fmt.Println("Passwords match")
	}

	// Constant-time selection
	option1 := []byte("option_a")
	option2 := []byte("option_b")
	condition := true

	selected := sgx.ConstantTimeSelect(condition, option1, option2)
	if selected != nil {
		fmt.Printf("Selected: %s\n", string(selected[:8]))
	}

	// Output:
	// Passwords match
	// Selected: option_a
}
