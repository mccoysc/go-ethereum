//go:build testenv
// +build testenv

// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package sgx

import (
	"encoding/binary"
	"fmt"
	
	"github.com/ethereum/go-ethereum/log"
)

// generateQuoteViaGramine generates an SGX Quote using Gramine's /dev/attestation interface.
// Test version: uses a real quote structure with dynamic reportData.
func generateQuoteViaGramine(reportData []byte) ([]byte, error) {
	if len(reportData) > 64 {
		return nil, fmt.Errorf("reportData too long: max 64 bytes, got %d", len(reportData))
	}

	// Pad report data to 64 bytes
	paddedData := make([]byte, 64)
	copy(paddedData, reportData)

	// Test mode: use real quote structure with dynamic reportData
	log.Debug("Test mode: generating real quote structure with reportData")
	return generateRealQuoteStructure(paddedData)
}

// readMREnclave reads the MRENCLAVE value.
// Test version: returns deterministic mock MRENCLAVE.
func readMREnclave() ([]byte, error) {
	// Return a deterministic mock MRENCLAVE for testing
	mrenclave := make([]byte, 32)
	for i := range mrenclave {
		mrenclave[i] = byte(i)
	}
	return mrenclave, nil
}

// generateRealQuoteStructure generates a real DCAP Quote v3 structure
// This matches the format of actual SGX quotes and can be parsed by ExtractInstanceID
func generateRealQuoteStructure(reportData []byte) ([]byte, error) {
	if len(reportData) != 64 {
		return nil, fmt.Errorf("reportData must be exactly 64 bytes, got %d", len(reportData))
	}

	// Read MRENCLAVE
	mrenclave, err := readMREnclave()
	if err != nil {
		return nil, fmt.Errorf("failed to read MRENCLAVE: %w", err)
	}

	quote := make([]byte, 0, 1024)

	// === Header (48 bytes) ===
	quote = append(quote, 0x03, 0x00) // Version 3
	quote = append(quote, 0x02, 0x00) // Attestation key type: ECDSA P-256
	quote = append(quote, 0x00, 0x00, 0x00, 0x00) // Reserved
	quote = append(quote, 0x01, 0x00) // QE SVN
	quote = append(quote, 0x01, 0x00) // PCE SVN
	// QE Vendor ID (16 bytes) - Intel
	quote = append(quote, 0x93, 0x9a, 0x72, 0x33, 0xf7, 0x9c, 0x4c, 0xa9,
		0x94, 0x0a, 0x0d, 0xb3, 0x95, 0x7f, 0x06, 0x07)
	// User data (20 bytes)
	quote = append(quote, make([]byte, 20)...)

	// === Report Body (384 bytes) ===
	// CPUSVN (16 bytes)
	quote = append(quote, make([]byte, 16)...)
	// MISCSELECT (4 bytes)
	quote = append(quote, 0x00, 0x00, 0x00, 0x00)
	// Reserved (28 bytes)
	quote = append(quote, make([]byte, 28)...)
	// ATTRIBUTES (16 bytes)
	quote = append(quote, make([]byte, 16)...)
	// MRENCLAVE (32 bytes)
	quote = append(quote, mrenclave...)
	// MRSIGNER (32 bytes)
	quote = append(quote, make([]byte, 32)...)
	// Reserved (96 bytes)
	quote = append(quote, make([]byte, 96)...)
	// ISVPRODID (2 bytes)
	quote = append(quote, 0x00, 0x00)
	// ISVSVN (2 bytes)
	quote = append(quote, 0x01, 0x00)
	// Reserved (60 bytes)
	quote = append(quote, make([]byte, 60)...)
	// REPORTDATA (64 bytes) - THE DYNAMIC PART
	quote = append(quote, reportData...)

	// === Signature Data ===
	// Signature length (4 bytes)
	sigLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(sigLen, 64)
	quote = append(quote, sigLen...)
	// Signature (64 bytes)
	quote = append(quote, make([]byte, 64)...)
	
	// Attestation public key (64 bytes)
	quote = append(quote, make([]byte, 64)...)
	
	// QE Report (384 bytes)
	quote = append(quote, make([]byte, 384)...)
	
	// QE Report Signature length (4 bytes)
	quote = append(quote, sigLen...)
	// QE Report Signature (64 bytes)
	quote = append(quote, make([]byte, 64)...)
	
	// QE Auth Data length (2 bytes)
	authLen := make([]byte, 2)
	binary.LittleEndian.PutUint16(authLen, 0)
	quote = append(quote, authLen...)
	
	// Cert Data Type (2 bytes) - Type 6: PPID_Cleartext
	certType := make([]byte, 2)
	binary.LittleEndian.PutUint16(certType, 6)
	quote = append(quote, certType...)
	
	// Cert Data Size (4 bytes) - 36 bytes total
	certSize := make([]byte, 4)
	binary.LittleEndian.PutUint32(certSize, 36)
	quote = append(quote, certSize...)
	
	// === Cert Data (36 bytes) ===
	// PPID (16 bytes) - all zeros for consistent instance ID
	ppid := make([]byte, 16)
	quote = append(quote, ppid...)
	// CPUSVN (16 bytes)
	quote = append(quote, make([]byte, 16)...)
	// PCESVN (2 bytes)
	quote = append(quote, 0x01, 0x00)
	// PCEID (2 bytes)
	quote = append(quote, 0x00, 0x00)

	log.Debug("Generated real quote structure",
		"quoteSize", len(quote),
		"reportData", fmt.Sprintf("%x", reportData[:32]),
		"ppid", fmt.Sprintf("%x", ppid))

	return quote, nil
}

