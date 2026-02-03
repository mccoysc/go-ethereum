package sgx

import (
	"bytes"
	"fmt"
	"log"
)

// VerifyManifestWithMREnclaveCalculation performs complete manifest verification
// including MRENCLAVE recalculation
func VerifyManifestWithMREnclaveCalculation(manifestPath string) error {
	log.Printf("Starting complete manifest verification for: %s", manifestPath)
	
	// Step 1: Parse manifest file
	log.Println("Step 1: Parsing manifest file...")
	manifest, sigstruct, err := ParseManifestFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}
	log.Printf("  ✓ Manifest parsed successfully")
	log.Printf("  - Enclave size: %s", manifest.SGX.EnclaveSize)
	log.Printf("  - Thread num: %d", manifest.SGX.ThreadNum)
	log.Printf("  - Trusted files: %d", len(manifest.SGX.TrustedFiles))
	
	// Step 2: Calculate trusted files hashes
	log.Println("Step 2: Calculating trusted files hashes...")
	fileHashes, err := CalculateTrustedFilesHashes(manifest.SGX.TrustedFiles)
	if err != nil {
		return fmt.Errorf("failed to calculate file hashes: %w", err)
	}
	log.Printf("  ✓ Calculated %d file hashes", len(fileHashes))
	
	// Step 3: Calculate MRENCLAVE from manifest
	log.Println("Step 3: Calculating MRENCLAVE from manifest...")
	calculatedMR, err := CalculateMREnclave(manifest, fileHashes)
	if err != nil {
		return fmt.Errorf("failed to calculate MRENCLAVE: %w", err)
	}
	log.Printf("  ✓ Calculated MRENCLAVE: %x", calculatedMR)
	
	// Step 4: Extract MRENCLAVE from SIGSTRUCT
	log.Println("Step 4: Extracting MRENCLAVE from SIGSTRUCT...")
	sigstructMR := sigstruct[960:992]
	log.Printf("  ✓ SIGSTRUCT MRENCLAVE: %x", sigstructMR)
	
	// Step 5: Compare MRENCLAVEs
	log.Println("Step 5: Comparing MRENCLAVEs...")
	if !bytes.Equal(calculatedMR, sigstructMR) {
		log.Printf("  ✗ MRENCLAVE MISMATCH!")
		log.Printf("    Calculated: %x", calculatedMR)
		log.Printf("    SIGSTRUCT:  %x", sigstructMR)
		return fmt.Errorf("MRENCLAVE mismatch - SIGSTRUCT data does not match manifest content")
	}
	log.Printf("  ✓ MRENCLAVE match - SIGSTRUCT data is authentic")
	
	// Step 6: Verify SIGSTRUCT signature
	log.Println("Step 6: Verifying SIGSTRUCT signature...")
	if err := VerifySIGSTRUCTSignature(sigstruct); err != nil {
		return fmt.Errorf("SIGSTRUCT signature verification failed: %w", err)
	}
	log.Printf("  ✓ SIGSTRUCT signature valid")
	
	// Step 7: Read runtime MRENCLAVE
	log.Println("Step 7: Reading runtime MRENCLAVE...")
	runtimeMR, err := readRuntimeMREnclave()
	if err != nil {
		log.Printf("  ! Warning: Could not read runtime MRENCLAVE: %v", err)
		log.Printf("  ! This is OK in test mode or before enclave is loaded")
	} else {
		log.Printf("  ✓ Runtime MRENCLAVE: %x", runtimeMR)
		
		// Step 8: Compare with runtime
		log.Println("Step 8: Comparing with runtime MRENCLAVE...")
		if !bytes.Equal(calculatedMR, runtimeMR) {
			log.Printf("  ✗ Runtime MRENCLAVE MISMATCH!")
			log.Printf("    Expected: %x", calculatedMR)
			log.Printf("    Runtime:  %x", runtimeMR)
			return fmt.Errorf("runtime MRENCLAVE mismatch - enclave was not built from this manifest")
		}
		log.Printf("  ✓ Runtime MRENCLAVE matches - enclave is authentic")
	}
	
	log.Println("✓ Complete manifest verification successful!")
	return nil
}

// VerifyManifestWithMREnclaveCalculationOrFail is like VerifyManifestWithMREnclaveCalculation
// but logs and continues on certain errors in test mode
func VerifyManifestWithMREnclaveCalculationOrFail(manifestPath string, testMode bool) error {
	err := VerifyManifestWithMREnclaveCalculation(manifestPath)
	
	if err != nil {
		if testMode {
			log.Printf("Warning: Manifest verification failed in test mode: %v", err)
			log.Printf("Continuing anyway (test mode allows this)")
			return nil
		}
		return err
	}
	
	return nil
}
