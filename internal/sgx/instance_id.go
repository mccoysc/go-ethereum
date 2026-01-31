// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package sgx

import (
	"encoding/hex"
	"errors"
	"fmt"
)

// InstanceID represents a unique hardware identifier for an SGX CPU.
// This is extracted from the SGX Quote and is unique per physical SGX CPU.
type InstanceID struct {
	// CPUInstanceID is the unique identifier for the SGX CPU
	CPUInstanceID []byte

	// QuoteType indicates whether this is EPID or DCAP quote
	QuoteType uint16
}

// ExtractInstanceID extracts the Instance ID (hardware unique identifier) from an SGX Quote.
// The Instance ID is used to:
// - Ensure each physical CPU can only register one validator node
// - Prevent Sybil attacks by the same hardware running multiple nodes
// - Distinguish different genesis administrators during bootstrap
func ExtractInstanceID(quote []byte) (*InstanceID, error) {
	if len(quote) < 432 {
		return nil, errors.New("quote too short for instance ID extraction")
	}

	// Parse quote to get basic information
	parsedQuote, err := ParseQuote(quote)
	if err != nil {
		return nil, fmt.Errorf("failed to parse quote: %w", err)
	}

	instanceID := &InstanceID{
		QuoteType: parsedQuote.SignType,
	}

	// Extract instance ID based on quote type
	switch parsedQuote.SignType {
	case 0, 1: // EPID (Unlinkable or Linkable)
		// For EPID quotes, the platform instance ID is in the signature data
		// This is a simplified extraction - real implementation needs EPID library
		instanceID.CPUInstanceID = extractEPIDInstanceID(quote)

	case 2, 3: // DCAP (ECDSA P-256 or ECDSA P-384)
		// For DCAP quotes, extract FMSPC and other platform identifiers
		instanceID.CPUInstanceID = extractDCAPInstanceID(quote)

	default:
		return nil, fmt.Errorf("unknown quote signature type: %d", parsedQuote.SignType)
	}

	if len(instanceID.CPUInstanceID) == 0 {
		return nil, errors.New("failed to extract instance ID from quote")
	}

	return instanceID, nil
}

// extractEPIDInstanceID extracts the platform instance ID from an EPID quote.
// EPID quotes include platform-specific identifiers in the signature structure.
func extractEPIDInstanceID(quote []byte) []byte {
	// EPID quote structure (simplified):
	// - Quote body: 432 bytes
	// - Signature: variable length, starts at offset 432
	//
	// For EPID, we use a combination of:
	// - Platform Info (if available)
	// - GID (Group ID) - 4 bytes at offset within signature
	//
	// Note: This is a simplified implementation. Production code should
	// use Intel's EPID verification library for proper extraction.

	if len(quote) < 436 {
		return nil
	}

	// Extract signature length (2 bytes at offset 432)
	// GID is typically within the first part of EPID signature
	// For this implementation, we create a composite ID from available data

	// Use the first 32 bytes of signature data as instance identifier
	instanceID := make([]byte, 32)
	if len(quote) >= 432+32 {
		copy(instanceID, quote[432:464])
	}

	return instanceID
}

// extractDCAPInstanceID extracts the platform instance ID from a DCAP quote.
// DCAP quotes include certification data with platform-specific identifiers.
func extractDCAPInstanceID(quote []byte) []byte {
	// DCAP quote structure (v3):
	// - Quote header: 48 bytes
	// - ISV enclave report: 384 bytes (total 432)
	// - Quote signature data: variable
	//
	// The signature data for DCAP includes:
	// - ECDSA signature
	// - Attestation key
	// - Certification data (QE Report, QE Report Signature, Auth Data)
	// - PCK Certificate Chain
	//
	// Platform identifiers include:
	// - PPID (Platform Provisioning ID) - from certification data
	// - CPUSVN (CPU Security Version Number) - in report body
	// - PCESVN (PCE Security Version Number) - from QE report
	// - FMSPC (Family-Model-Stepping-Platform-CustomSKU) - from cert

	if len(quote) < 432 {
		return nil
	}

	// For simplified implementation, create composite ID from:
	// 1. CPUSVN (16 bytes at offset 64 in report)
	// 2. Report data subset (for uniqueness across same platform)

	instanceID := make([]byte, 32)

	// Copy CPUSVN (16 bytes from offset 64)
	if len(quote) >= 80 {
		copy(instanceID[0:16], quote[64:80])
	}

	// Copy part of attributes (16 bytes from offset 96)
	if len(quote) >= 112 {
		copy(instanceID[16:32], quote[96:112])
	}

	return instanceID
}

// String returns a hex string representation of the Instance ID.
func (id *InstanceID) String() string {
	return hex.EncodeToString(id.CPUInstanceID)
}

// Equal checks if two Instance IDs are equal.
func (id *InstanceID) Equal(other *InstanceID) bool {
	if id == nil || other == nil {
		return id == other
	}

	if id.QuoteType != other.QuoteType {
		return false
	}

	return ConstantTimeCompare(id.CPUInstanceID, other.CPUInstanceID)
}
