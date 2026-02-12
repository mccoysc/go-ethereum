//go:build testenv
// +build testenv

// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package sgx

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	
	"github.com/ethereum/go-ethereum/log"
)

// generateQuoteViaGramine generates an SGX Quote using Gramine's /dev/attestation interface.
// Test version: loads real quote from Gramine RA-TLS certificate.
// NOTE: Does NOT modify the quote - returns it as-is from real SGX hardware.
// This allows full verification logic to run and detect mismatches.
func generateQuoteViaGramine(reportData []byte) ([]byte, error) {
	if len(reportData) > 64 {
		return nil, fmt.Errorf("reportData too long: max 64 bytes, got %d", len(reportData))
	}

	// Load the real quote extracted from Gramine RA-TLS certificate
	// This is a real, verifiable DCAP quote from actual SGX hardware
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	quotePath := filepath.Join(dir, "testdata", "gramine_ratls_quote.bin")

	quote, err := os.ReadFile(quotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load real quote: %w", err)
	}

	// DO NOT modify the quote - keep it as-is from real SGX
	// The verification will detect that reportData doesn't match (expected in testenv)
	// But testenv mode will log this and continue (allowing testing without real SGX)
	
	log.Debug("Test mode: loaded real Gramine quote (unmodified)",
		"quoteSize", len(quote),
		"requestedReportData", fmt.Sprintf("%x", reportData[:min(len(reportData), 32)]))

	return quote, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// readMREnclave reads the MRENCLAVE value.
// Test version: returns the MRENCLAVE from the real Gramine quote.
func readMREnclave() ([]byte, error) {
	// Extract MRENCLAVE from the real quote for consistency
	// MRENCLAVE is at offset 112 in the report body (offset 48 + 64)
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	quotePath := filepath.Join(dir, "testdata", "gramine_ratls_quote.bin")

	quote, err := os.ReadFile(quotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load real quote: %w", err)
	}

	if len(quote) < 144 {
		return nil, fmt.Errorf("quote too short to contain MRENCLAVE")
	}

	// Extract MRENCLAVE (32 bytes at offset 112)
	mrenclave := make([]byte, 32)
	copy(mrenclave, quote[112:144])
	
	return mrenclave, nil
}
