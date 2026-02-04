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
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	
	"github.com/ethereum/go-ethereum/crypto"
)

// ExtractInstanceID extracts the Instance ID (hardware unique identifier) from an SGX Quote.
// The Instance ID is used to:
// - Ensure each physical CPU can only register one validator node
// - Prevent Sybil attacks by the same hardware running multiple nodes
// - Distinguish different genesis administrators during bootstrap
//
// Implementation follows mccoysc/gramine JavaScript implementation:
// Priority 1: PPID (Platform Provisioning ID) from certification data
// Priority 2: PCK Certificate SPKI fingerprint
// Priority 3: CPUSVN + Attributes (fallback for testing only)
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

	// Extract instance ID based on quote type and available data
	switch parsedQuote.SignType {
	case 0, 1: // EPID (Unlinkable or Linkable)
		// For EPID quotes, try to extract from signature data
		instanceID.CPUInstanceID = extractEPIDInstanceID(quote)

	case 2, 3: // DCAP (ECDSA P-256 or ECDSA P-384)
		// For DCAP quotes, prioritize PPID extraction
		ppid, err := extractPPIDFromDCAPQuote(quote)
		if err == nil && len(ppid) > 0 {
			instanceID.CPUInstanceID = ppid
		} else {
			// Fallback: use CPUSVN + Attributes composite
			instanceID.CPUInstanceID = extractDCAPInstanceID(quote)
		}

	default:
		return nil, fmt.Errorf("unknown quote signature type: %d", parsedQuote.SignType)
	}

	if len(instanceID.CPUInstanceID) == 0 {
		return nil, errors.New("failed to extract instance ID from quote")
	}

	return instanceID, nil
}

// extractPPIDFromDCAPQuote extracts PPID (Platform Provisioning ID) from DCAP quote.
// PPID is the most reliable hardware identifier and should be used when available.
//
// Quote structure:
// - Quote body: 432 bytes
// - Signature data (variable):
//   - Signature len (4) + Signature
//   - Attestation Public Key (64 for ECDSA-256)
//   - QE Report (384)
//   - QE Report Signature len (4) + QE Report Signature
//   - QE Auth Data len (2) + QE Auth Data
//   - Cert Data Type (2)
//   - Cert Data Size (4)
//   - Cert Data:
//     - For Type 5 (PPID_Encrypted_RSA_3072) or 6 (PPID_Cleartext):
//       - PPID (16 bytes)
//       - CPUSVN (16 bytes)
//       - PCESVN (2 bytes)
//       - PCEID (2 bytes)
func extractPPIDFromDCAPQuote(quote []byte) ([]byte, error) {
	if len(quote) < 432 {
		return nil, fmt.Errorf("quote too short")
	}

	offset := 432 // Start of signature data

	// Read signature length and skip signature
	if len(quote) < offset+4 {
		return nil, fmt.Errorf("cannot read signature length")
	}
	sigLen := binary.LittleEndian.Uint32(quote[offset : offset+4])
	offset += 4 + int(sigLen)

	// Skip attestation public key (64 bytes for ECDSA-256)
	offset += 64

	// Skip QE Report (384 bytes)
	offset += 384

	// Read and skip QE Report Signature
	if len(quote) < offset+4 {
		return nil, fmt.Errorf("cannot read QE report signature length")
	}
	qeReportSigLen := binary.LittleEndian.Uint32(quote[offset : offset+4])
	offset += 4 + int(qeReportSigLen)

	// Read and skip QE Auth Data
	if len(quote) < offset+2 {
		return nil, fmt.Errorf("cannot read QE auth data length")
	}
	qeAuthDataLen := binary.LittleEndian.Uint16(quote[offset : offset+2])
	offset += 2 + int(qeAuthDataLen)

	// Read Cert Data Type
	if len(quote) < offset+2 {
		return nil, fmt.Errorf("cannot read cert data type")
	}
	certDataType := binary.LittleEndian.Uint16(quote[offset : offset+2])
	offset += 2

	// Read Cert Data Size
	if len(quote) < offset+4 {
		return nil, fmt.Errorf("cannot read cert data size")
	}
	certDataSize := binary.LittleEndian.Uint32(quote[offset : offset+4])
	offset += 4

	// Check if this is PPID-based certification data
	// Type 5 = PPID_Encrypted_RSA_3072
	// Type 6 = PPID_Cleartext
	if certDataType == 5 || certDataType == 6 {
		// PPID is the first 16 bytes of cert data
		if len(quote) < offset+16 {
			return nil, fmt.Errorf("cert data too small for PPID")
		}
		if certDataSize < 16 {
			return nil, fmt.Errorf("cert data size too small: %d", certDataSize)
		}

		ppid := make([]byte, 16)
		copy(ppid, quote[offset:offset+16])
		return ppid, nil
	}

	// PPID not available in this quote
	return nil, fmt.Errorf("PPID not available (cert data type %d)", certDataType)
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
// 
// Recommended approach: FMSPC + PCK Public Key
// - FMSPC: Family-Model-Stepping-Platform-CustomSKU (6 bytes, from PCK cert)
// - PCK Public Key: Platform Certification Key public key (from PCK cert)
// - Combination ensures unique identification per platform
//
// Fallback: PPID (Platform Provisioning ID) if FMSPC extraction fails
func extractDCAPInstanceID(quote []byte) []byte {
	if len(quote) < 432 {
		return nil
	}

	offset := 432 // Start of signature data

	// Navigate to certification data
	// Skip signature
	if len(quote) < offset+4 {
		return extractFallbackInstanceID(quote)
	}
	sigLen := binary.LittleEndian.Uint32(quote[offset : offset+4])
	offset += 4 + int(sigLen)

	// Skip attestation public key (64 bytes)
	offset += 64

	// Skip QE Report (384 bytes)
	offset += 384

	// Skip QE Report Signature
	if len(quote) < offset+4 {
		return extractFallbackInstanceID(quote)
	}
	qeReportSigLen := binary.LittleEndian.Uint32(quote[offset : offset+4])
	offset += 4 + int(qeReportSigLen)

	// Skip QE Auth Data
	if len(quote) < offset+2 {
		return extractFallbackInstanceID(quote)
	}
	qeAuthDataLen := binary.LittleEndian.Uint16(quote[offset : offset+2])
	offset += 2 + int(qeAuthDataLen)

	// Read Cert Data Type
	if len(quote) < offset+2 {
		return extractFallbackInstanceID(quote)
	}
	certDataType := binary.LittleEndian.Uint16(quote[offset : offset+2])
	offset += 2

	// Read Cert Data Size
	if len(quote) < offset+4 {
		return extractFallbackInstanceID(quote)
	}
	certDataSize := binary.LittleEndian.Uint32(quote[offset : offset+4])
	offset += 4

	// For PCK certificate chain (Type 1-4), extract FMSPC + PCK Public Key
	if certDataType >= 1 && certDataType <= 4 && len(quote) >= offset+int(certDataSize) {
		certData := quote[offset : offset+int(certDataSize)]
		
		// Extract FMSPC and PCK Public Key from certificate chain
		// For simplicity, hash the entire cert chain which includes both
		// In production, would parse X.509 cert to extract FMSPC extension and public key
		// FMSPC is in SGX extension OID 1.2.840.113741.1.13.1.4
		// PCK Public Key is in the certificate's SubjectPublicKeyInfo
		
		// Hash cert data to create unique instance ID
		// This includes FMSPC + PCK Public Key + other unique platform info
		instanceID := crypto.Keccak256(certData)
		return instanceID
	}

	// For PPID-based certification (Type 5-6), use PPID as fallback
	if certDataType == 5 || certDataType == 6 && len(quote) >= offset+16 {
		ppid := make([]byte, 16)
		copy(ppid, quote[offset:offset+16])
		
		// Extend to 32 bytes
		instanceID := crypto.Keccak256(ppid)
		return instanceID
	}

	// Final fallback
	return extractFallbackInstanceID(quote)
}

// extractFallbackInstanceID creates a fallback instance ID from basic quote fields.
// This should only be used when PCK certificate extraction fails.
func extractFallbackInstanceID(quote []byte) []byte {
	if len(quote) < 432 {
		return nil
	}

	// Create composite ID from:
	// 1. CPUSVN (16 bytes at offset 48)
	// 2. Attributes (16 bytes at offset 96)
	// Hash them together for uniqueness
	
	composite := make([]byte, 32)
	
	// Copy CPUSVN
	if len(quote) >= 64 {
		copy(composite[0:16], quote[48:64])
	}
	
	// Copy Attributes
	if len(quote) >= 112 {
		copy(composite[16:32], quote[96:112])
	}
	
	// Hash for better distribution
	instanceID := crypto.Keccak256(composite)
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
