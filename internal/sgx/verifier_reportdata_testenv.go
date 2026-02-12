//go:build testenv
// +build testenv

// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package sgx

import (
	"crypto/subtle"
	"fmt"
	
	"github.com/ethereum/go-ethereum/log"
)

// verifyReportDataMatch verifies that the quote's report data matches expected data.
// Testenv version: Executes FULL verification (constant-time comparison), logs result,
// but allows failure (since we use real quote from file with different reportData).
//
// This ensures the verification logic itself is tested, while allowing testenv to run.
func verifyReportDataMatch(reportData, expected []byte, compareLen int) error {
	// Validate inputs (same as production)
	if len(reportData) < compareLen {
		err := fmt.Errorf("reportData too short: got %d bytes, need %d", len(reportData), compareLen)
		log.Warn("TESTENV: ReportData validation FAILED (would fail in production)",
			"error", err.Error(),
			"verificationResult", "FAILED",
			"productionBehavior", "WOULD_REJECT")
		// In testenv: allow despite validation failure
		return nil
	}
	
	if len(expected) < compareLen {
		err := fmt.Errorf("expected data too short: got %d bytes, need %d", len(expected), compareLen)
		log.Warn("TESTENV: ReportData validation FAILED (would fail in production)",
			"error", err.Error(),
			"verificationResult", "FAILED",
			"productionBehavior", "WOULD_REJECT")
		// In testenv: allow despite validation failure
		return nil
	}
	
	// Execute constant-time comparison (same as production)
	// This is critical for security - must use constant-time to prevent timing attacks
	reportDataToCompare := reportData[:compareLen]
	expectedToCompare := expected[:compareLen]
	
	// Use constant-time comparison to prevent timing attacks
	match := subtle.ConstantTimeCompare(reportDataToCompare, expectedToCompare) == 1
	
	if !match {
		// Verification FAILED - log detailed information
		displayLen := compareLen
		if displayLen > 32 {
			displayLen = 32
		}
		log.Warn("TESTENV: ReportData verification FAILED (allowed in testenv, would REJECT in production)",
			"expectedData", fmt.Sprintf("%x", expectedToCompare[:displayLen]),
			"gotReportData", fmt.Sprintf("%x", reportDataToCompare[:displayLen]),
			"compareLen", compareLen,
			"verificationResult", "FAILED",
			"productionBehavior", "WOULD_REJECT",
			"reason", "using real quote file with different reportData")
		
		// In testenv mode: allow despite verification failure
		// This lets us test with real SGX quote while not having matching reportData
		return nil
	}
	
	// Verification PASSED
	log.Debug("TESTENV: ReportData verification PASSED",
		"compareLen", compareLen,
		"verificationResult", "PASSED")
	
	return nil
}
