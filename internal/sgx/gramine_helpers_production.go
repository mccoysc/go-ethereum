//go:build !testenv
// +build !testenv

// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package sgx

import (
	"fmt"
	"os"
)

// generateQuoteViaGramine generates an SGX Quote using Gramine's /dev/attestation interface.
// Production version: uses real Gramine attestation device.
func generateQuoteViaGramine(reportData []byte) ([]byte, error) {
	if len(reportData) > 64 {
		return nil, fmt.Errorf("reportData too long: max 64 bytes, got %d", len(reportData))
	}

	// Pad report data to 64 bytes
	paddedData := make([]byte, 64)
	copy(paddedData, reportData)

	// Production mode: use real Gramine attestation device
	// Write report data to /dev/attestation/user_report_data
	err := os.WriteFile("/dev/attestation/user_report_data", paddedData, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to write user_report_data: %w", err)
	}

	// Read the generated quote from /dev/attestation/quote
	// In real Gramine, this file is automatically updated after writing user_report_data
	quote, err := os.ReadFile("/dev/attestation/quote")
	if err != nil {
		return nil, fmt.Errorf("failed to read quote: %w", err)
	}

	return quote, nil
}
