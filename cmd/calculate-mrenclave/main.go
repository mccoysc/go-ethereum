package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/internal/sgx"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <manifest.sgx>\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "\nThis tool verifies our MRENCLAVE calculation against Gramine's output.\n")
		os.Exit(1)
	}

	manifestPath := os.Args[1]

	// Parse manifest file
	fmt.Printf("Reading manifest: %s\n", manifestPath)
	manifest, sigstruct, err := sgx.ParseManifestFile(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing manifest: %v\n", err)
		os.Exit(1)
	}

	// Extract MRENCLAVE from SIGSTRUCT (what Gramine generated)
	if len(sigstruct) < 992 {
		fmt.Fprintf(os.Stderr, "SIGSTRUCT too short\n")
		os.Exit(1)
	}
	gramineEnclaveHash := sigstruct[960:992]
	
	fmt.Printf("\nGramine MRENCLAVE (from SIGSTRUCT): %s\n", hex.EncodeToString(gramineEnclaveHash))

	// Calculate trusted files hashes
	fmt.Println("\nCalculating trusted files hashes...")
	fileHashes, err := sgx.CalculateTrustedFilesHashes(manifest.SGX.TrustedFiles)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error calculating file hashes: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Calculated hashes for %d trusted files\n", len(fileHashes))

	// Calculate MRENCLAVE using our implementation
	fmt.Println("\nCalculating MRENCLAVE using our implementation...")
	calculatedMR, err := sgx.CalculateMREnclave(manifest, fileHashes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error calculating MRENCLAVE: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Our calculated MRENCLAVE:           %s\n", hex.EncodeToString(calculatedMR))

	// Compare
	fmt.Println("\n============================================================")
	if hex.EncodeToString(calculatedMR) == hex.EncodeToString(gramineEnclaveHash) {
		fmt.Println("✓ SUCCESS: MRENCLAVEs MATCH!")
		fmt.Println("Our implementation is CORRECT and matches Gramine exactly.")
		os.Exit(0)
	} else {
		fmt.Println("✗ FAILURE: MRENCLAVEs DO NOT MATCH!")
		fmt.Println("Our implementation needs correction.")
		fmt.Printf("\nExpected (Gramine): %s\n", hex.EncodeToString(gramineEnclaveHash))
		fmt.Printf("Got (Ours):         %s\n", hex.EncodeToString(calculatedMR))
		os.Exit(1)
	}
}
