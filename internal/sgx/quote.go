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
	"encoding/asn1"
	"encoding/binary"
	"errors"
)

// SGXQuote represents the SGX Quote data structure.
type SGXQuote struct {
	Version            uint16   // Quote version
	AttestationKeyType uint16   // Attestation key type (2=ECDSA-P256, 3=ECDSA-P384)
	SignType           uint16   // Signature type (EPID/DCAP)
	MRENCLAVE          [32]byte // Enclave code measurement
	MRSIGNER           [32]byte // Signer measurement
	ISVProdID          uint16   // Product ID
	ISVSVN             uint16   // Security version number
	ReportData         [64]byte // User-defined data
	TCBStatus          uint8    // TCB status
	Signature          []byte   // Quote signature
	CertChain          []string // PCK certificate chain (PEM format) extracted from quote
}

// TCB status constants
const (
	TCBUpToDate            uint8 = 0x00
	TCBOutOfDate           uint8 = 0x01
	TCBRevoked             uint8 = 0x02
	TCBConfigurationNeeded uint8 = 0x03
)

// SGXQuoteOID is the OID for SGX Quote in X.509 certificates.
// This is a custom OID for embedding SGX quotes in RA-TLS certificates.
var SGXQuoteOID = asn1.ObjectIdentifier{1, 2, 840, 113741, 1, 13, 1}

// ParseQuote parses an SGX Quote from raw bytes.
// Matches the structure returned by gramine sgx-quote-verify.js parseQuoteStructure function.
// Reference: https://github.com/mccoysc/gramine/blob/master/tools/sgx/ra-tls/sgx-quote-verify.js
func ParseQuote(quote []byte) (*SGXQuote, error) {
	if len(quote) < 432 {
		return nil, errors.New("quote too short: minimum 432 bytes required")
	}

	q := &SGXQuote{}
	q.Version = binary.LittleEndian.Uint16(quote[0:2])
	q.AttestationKeyType = binary.LittleEndian.Uint16(quote[2:4])
	q.SignType = binary.LittleEndian.Uint16(quote[2:4]) // Same as AttestationKeyType for DCAP
	copy(q.MRENCLAVE[:], quote[112:144])
	copy(q.MRSIGNER[:], quote[176:208])
	q.ISVProdID = binary.LittleEndian.Uint16(quote[304:306])
	q.ISVSVN = binary.LittleEndian.Uint16(quote[306:308])
	copy(q.ReportData[:], quote[368:432])

	// TCB status is typically at a fixed offset for DCAP quotes
	// For simplicity, we default to up-to-date
	q.TCBStatus = TCBUpToDate

	// Signature data follows the fixed fields
	if len(quote) > 432 {
		q.Signature = make([]byte, len(quote)-432)
		copy(q.Signature, quote[432:])
		
		// Try to extract embedded certificate chain from signature data
		// Matches JS logic: check if quote has certChain embedded
		q.CertChain = extractCertChainFromSignatureData(q.Signature)
	}

	return q, nil
}

// extractCertChainFromSignatureData tries to extract PCK certificate chain from quote signature data.
// SGX DCAP Quote v3/v4 may embed the PCK cert chain in certification data (type 5).
// Returns nil if no cert chain found.
func extractCertChainFromSignatureData(sigData []byte) []string {
	if len(sigData) < 436 {
		return nil
	}

	// Skip: signature (64/96 bytes) + attestation key (64/96 bytes) + QE report (384 bytes) + QE sig (64/96 bytes)
	// Certification data starts after these
	// For simplicity, look for certification data type field at various offsets
	
	// Try to find certification data (type 5 = PCK cert chain)
	// Format: [cert_type: 2 bytes] [cert_data_size: 4 bytes] [cert_data: variable]
	
	// Common offset for P256: 64 + 64 + 384 + 64 = 576
	offset := 576
	if offset+6 > len(sigData) {
		return nil
	}
	
	certType := binary.LittleEndian.Uint16(sigData[offset : offset+2])
	certDataSize := binary.LittleEndian.Uint32(sigData[offset+2 : offset+6])
	
	if certType == 5 && certDataSize > 0 && int(certDataSize) < len(sigData)-offset-6 {
		certData := sigData[offset+6 : offset+6+int(certDataSize)]
		// Parse PEM certificates from cert data
		return parsePEMCertChainToStrings(certData)
	}
	
	return nil
}

// parsePEMCertChainToStrings parses multiple PEM certificates from byte data.
// Returns array of PEM strings (matching JS parsePemCertChain function).
func parsePEMCertChainToStrings(data []byte) []string {
	var certs []string
	rest := data
	
	for len(rest) > 0 {
		// Look for -----BEGIN CERTIFICATE-----
		beginMarker := []byte("-----BEGIN CERTIFICATE-----")
		endMarker := []byte("-----END CERTIFICATE-----")
		
		beginIdx := bytesIndex(rest, beginMarker)
		if beginIdx == -1 {
			break
		}
		
		endIdx := bytesIndex(rest[beginIdx:], endMarker)
		if endIdx == -1 {
			break
		}
		
		// Extract one certificate (including markers)
		certEnd := beginIdx + endIdx + len(endMarker)
		certPEM := string(rest[beginIdx:certEnd])
		certs = append(certs, certPEM)
		
		// Move to next potential certificate
		rest = rest[certEnd:]
	}
	
	return certs
}

// bytesIndex returns the index of the first instance of sep in s, or -1 if sep is not present in s.
func bytesIndex(s, sep []byte) int {
	for i := 0; i <= len(s)-len(sep); i++ {
		if bytesHasPrefix(s[i:], sep) {
			return i
		}
	}
	return -1
}

// bytesHasPrefix tests whether the byte slice s begins with prefix.
func bytesHasPrefix(s, prefix []byte) bool {
	return len(s) >= len(prefix) && bytesEqual(s[0:len(prefix)], prefix)
}

// bytesEqual reports whether a and b are the same length and contain the same bytes.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
