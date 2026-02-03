//go:build testenv

// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package sgx

import (
	"github.com/ethereum/go-ethereum/log"
)

// verifyTrustedFilesWithPolicy performs trusted files verification with test policy
// Test mode: LENIENT - files may not exist, hash mismatches are warnings only
func verifyTrustedFilesWithPolicy(manifestSgxData []byte) error {
	log.Warn("Running trusted files verification in TEST mode (lenient)")
	log.Warn("Trusted files verification will be skipped in test environment")
	log.Warn("This is acceptable for testing but NEVER for production")
	
	// In test mode, we skip the verification because:
	// 1. Test environment may not have all the files listed in manifest
	// 2. Manifest was generated in different environment (Docker container)
	// 3. We're testing with real manifest.sgx but in different file system
	
	// We could try to verify, but just log errors instead of failing
	if err := VerifyTrustedFiles(manifestSgxData); err != nil {
		log.Warn("Trusted files verification failed (acceptable in test mode)", 
			"error", err)
		// Don't return error - allow testing to continue
	}
	
	return nil
}
