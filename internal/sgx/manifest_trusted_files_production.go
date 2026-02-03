//go:build !testenv

// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package sgx

import (
	"github.com/ethereum/go-ethereum/log"
)

// verifyTrustedFilesWithPolicy performs trusted files verification with production policy
// Production mode: STRICT - all files must exist and hashes must match
func verifyTrustedFilesWithPolicy(manifestSgxData []byte) error {
	log.Info("Running trusted files verification in PRODUCTION mode (strict)")
	
	// Call the full verification
	if err := VerifyTrustedFiles(manifestSgxData); err != nil {
		log.Crit("SECURITY CRITICAL: Trusted files verification failed in production mode", 
			"error", err)
		// In production, this is a critical security violation
		// The caller should exit the process
		return err
	}
	
	return nil
}
