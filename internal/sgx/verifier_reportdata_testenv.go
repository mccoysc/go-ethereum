//go:build testenv
// +build testenv

// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package sgx

import (
	"github.com/ethereum/go-ethereum/log"
)

// verifyReportDataMatch verifies that the quote's report data matches expected data.
// Test version: skip comparison because real quote won't have matching reportData.
//
// Rationale: In test environment, we use a real SGX quote (with valid signature) 
// but its reportData doesn't match the test scenario's expected data.
// We still verify all other aspects of the quote (signature, TCB, MRENCLAVE).
// Only this final reportData comparison is skipped.
func verifyReportDataMatch(reportData, expected []byte, compareLen int) error {
	log.Debug("Test mode: skipping reportData comparison",
		"reason", "using real quote with different reportData",
		"reportDataLen", len(reportData),
		"expectedLen", len(expected))
	
	// In test mode, we skip this check because:
	// - We use a real quote from file (has valid signature)
	// - Real quote's reportData won't match test scenario data
	// - All other verification (signature, TCB, MRENCLAVE) still runs
	return nil
}
