//go:build testenv
// +build testenv

// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package sgx

import (
	"fmt"
	
	"github.com/ethereum/go-ethereum/log"
)

// generateQuoteViaGramine generates an SGX Quote using Gramine's /dev/attestation interface.
// Test version: dynamically generates mock Quote with reportData.
func generateQuoteViaGramine(reportData []byte) ([]byte, error) {
	if len(reportData) > 64 {
		return nil, fmt.Errorf("reportData too long: max 64 bytes, got %d", len(reportData))
	}

	// Pad report data to 64 bytes
	paddedData := make([]byte, 64)
	copy(paddedData, reportData)

	// Test mode: dynamically generate Quote with reportData
	// This simulates Gramine's behavior of updating the quote file when user_report_data changes
	log.Debug("Test mode: generating dynamic mock Quote with reportData")
	return generateDynamicMockQuote(paddedData)
}

// generateDynamicMockQuote generates a mock Quote with the given reportData
// This is used in test mode to simulate Gramine's dynamic Quote generation
func generateDynamicMockQuote(reportData []byte) ([]byte, error) {
	if len(reportData) != 64 {
		return nil, fmt.Errorf("reportData must be exactly 64 bytes, got %d", len(reportData))
	}

	// Read MRENCLAVE from /dev/attestation/my_target_info
	mrenclave, err := readMREnclave()
	if err != nil {
		return nil, fmt.Errorf("failed to read MRENCLAVE: %w", err)
	}

	// Generate a minimal valid DCAP Quote v3 structure
	quote := make([]byte, 0, 512)

	// Header (48 bytes)
	quote = append(quote, 0x03, 0x00) // Version 3
	quote = append(quote, 0x02, 0x00) // Attestation key type: ECDSA P-256
	quote = append(quote, 0x00, 0x00, 0x00, 0x00) // Reserved
	quote = append(quote, 0x01, 0x00) // QE SVN
	quote = append(quote, 0x01, 0x00) // PCE SVN
	// QE Vendor ID (16 bytes) - Intel
	quote = append(quote, 0x93, 0x9a, 0x72, 0x33, 0xf7, 0x9c, 0x4c, 0xa9,
		0x94, 0x0a, 0x0d, 0xb3, 0x95, 0x7f, 0x06, 0x07)
	// User data (20 bytes) - zeros
	quote = append(quote, make([]byte, 20)...)

	// Report body (384 bytes)
	// CPUSVN (16 bytes)
	quote = append(quote, make([]byte, 16)...)
	// MISCSELECT (4 bytes)
	quote = append(quote, 0x00, 0x00, 0x00, 0x00)
	// Reserved (28 bytes)
	quote = append(quote, make([]byte, 28)...)
	// ATTRIBUTES (16 bytes)
	quote = append(quote, make([]byte, 16)...)
	// MRENCLAVE (32 bytes) - from /dev/attestation/my_target_info
	quote = append(quote, mrenclave...)
	// MRSIGNER (32 bytes) - all zeros for mock
	quote = append(quote, make([]byte, 32)...)
	// Reserved (96 bytes)
	quote = append(quote, make([]byte, 96)...)
	// ISVPRODID (2 bytes)
	quote = append(quote, 0x00, 0x00)
	// ISVSVN (2 bytes)
	quote = append(quote, 0x01, 0x00)
	// Reserved (60 bytes)
	quote = append(quote, make([]byte, 60)...)
	// REPORTDATA (64 bytes) - THE CRITICAL PART!
	quote = append(quote, reportData...)

	// Signature data (variable length, minimum structure)
	// For mock, add minimal signature data
	quote = append(quote, make([]byte, 64)...) // Mock signature

	log.Debug("Generated dynamic mock Quote",
		"quoteSize", len(quote),
		"reportData", fmt.Sprintf("%x", reportData[:32])) // Log first 32 bytes

	return quote, nil
}
