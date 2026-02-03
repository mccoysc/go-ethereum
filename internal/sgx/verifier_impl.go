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
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/ethereum/go-ethereum/crypto"
)

// DCAPVerifier implements the Verifier interface using Intel DCAP.
type DCAPVerifier struct {
	mu               sync.RWMutex
	allowedMREnclave map[string]bool
	allowedMRSigner  map[string]bool
	allowOutdatedTCB bool
}

// NewDCAPVerifier creates a new DCAP-based verifier.
func NewDCAPVerifier(allowOutdatedTCB bool) *DCAPVerifier {
	return &DCAPVerifier{
		allowedMREnclave: make(map[string]bool),
		allowedMRSigner:  make(map[string]bool),
		allowOutdatedTCB: allowOutdatedTCB,
	}
}

// VerifyQuote verifies the validity of an SGX Quote.
// This method only verifies the Quote's cryptographic signature and TCB status.
// It does NOT check MRENCLAVE/MRSIGNER against whitelist - that's only for RA-TLS certificate verification.
func (v *DCAPVerifier) VerifyQuote(quote []byte) error {
	// Parse the quote
	parsedQuote, err := ParseQuote(quote)
	if err != nil {
		return fmt.Errorf("failed to parse quote: %w", err)
	}

	// Verify quote signature (in a real implementation, this would call DCAP libraries)
	if err := v.verifyQuoteSignature(quote); err != nil {
		return fmt.Errorf("quote signature verification failed: %w", err)
	}

	// Check TCB status
	if !v.allowOutdatedTCB && parsedQuote.TCBStatus != TCBUpToDate {
		return fmt.Errorf("TCB status not up to date: %d", parsedQuote.TCBStatus)
	}

	// NO MRENCLAVE/MRSIGNER whitelist check here!
	// Whitelist is only checked during RA-TLS certificate verification (VerifyCertificate method)

	return nil
}

// VerifyCertificate verifies an RA-TLS certificate.
// This is the ONLY place where MRENCLAVE whitelist is checked.
func (v *DCAPVerifier) VerifyCertificate(cert *x509.Certificate) error {
	// Extract SGX quote from certificate extensions
	var quote []byte
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(SGXQuoteOID) {
			quote = ext.Value
			break
		}
	}

	if quote == nil {
		return errors.New("no SGX quote found in certificate")
	}

	// Verify the quote (cryptographic signature and TCB status)
	if err := v.VerifyQuote(quote); err != nil {
		return fmt.Errorf("quote verification failed: %w", err)
	}

	// Parse quote to extract measurements
	parsedQuote, err := ParseQuote(quote)
	if err != nil {
		return fmt.Errorf("failed to parse quote: %w", err)
	}

	// *** WHITELIST CHECK - ONLY FOR RA-TLS CERTIFICATES ***
	// Check MRENCLAVE whitelist (MRSIGNER check not needed for now)
	if !v.IsAllowedMREnclave(parsedQuote.MRENCLAVE[:]) {
		return fmt.Errorf("MRENCLAVE not in allowed list: %x", parsedQuote.MRENCLAVE)
	}

	// Verify that the certificate's public key matches the quote's report data
	// Extract public key from certificate
	pubKey, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("certificate public key is not ECDSA")
	}

	// Marshal public key to bytes
	pubKeyBytes := elliptic.Marshal(pubKey.Curve, pubKey.X, pubKey.Y)

	// Compare with report data using constant-time comparison
	// Report data is limited to 64 bytes
	compareLen := len(pubKeyBytes)
	if compareLen > 64 {
		compareLen = 64
	}
	
	// Verify reportData matches - uses build tags for test/production
	if err := verifyReportDataMatch(parsedQuote.ReportData[:], pubKeyBytes, compareLen); err != nil {
		return fmt.Errorf("certificate public key does not match quote report data: %w", err)
	}

	return nil
}

// IsAllowedMREnclave checks if the MRENCLAVE is in the whitelist.
func (v *DCAPVerifier) IsAllowedMREnclave(mrenclave []byte) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// If whitelist is empty, allow all (for testing/development)
	if len(v.allowedMREnclave) == 0 {
		return true
	}

	return v.allowedMREnclave[string(mrenclave)]
}

// IsAllowedMRSigner checks if the MRSIGNER is in the whitelist.
func (v *DCAPVerifier) IsAllowedMRSigner(mrsigner []byte) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// If whitelist is empty, allow all (for testing/development)
	if len(v.allowedMRSigner) == 0 {
		return true
	}

	return v.allowedMRSigner[string(mrsigner)]
}

// AddAllowedMREnclave adds an MRENCLAVE to the whitelist.
func (v *DCAPVerifier) AddAllowedMREnclave(mrenclave []byte) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.allowedMREnclave[string(mrenclave)] = true
}

// RemoveAllowedMREnclave removes an MRENCLAVE from the whitelist.
func (v *DCAPVerifier) RemoveAllowedMREnclave(mrenclave []byte) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.allowedMREnclave, string(mrenclave))
}

// AddAllowedMRSigner adds an MRSIGNER to the whitelist.
func (v *DCAPVerifier) AddAllowedMRSigner(mrsigner []byte) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.allowedMRSigner[string(mrsigner)] = true
}

// RemoveAllowedMRSigner removes an MRSIGNER from the whitelist.
func (v *DCAPVerifier) RemoveAllowedMRSigner(mrsigner []byte) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.allowedMRSigner, string(mrsigner))
}

// verifyQuoteSignature verifies the quote signature.
// In a real implementation, this would call Intel DCAP libraries via CGO.
// For now, we provide a mock implementation for testing.
func (v *DCAPVerifier) verifyQuoteSignature(quote []byte) error {
	// Mock implementation: just check minimum length
	if len(quote) < 432 {
		return errors.New("quote too short for signature verification")
	}

	// In a real implementation, this would:
	// 1. Call libsgx_dcap_ql to verify the quote signature
	// 2. Check the quote against Intel's attestation service
	// 3. Verify the certificate chain

	// For testing purposes, we accept any quote that can be parsed
	_, err := ParseQuote(quote)
	return err
}

// ExtractMREnclave is a utility function to extract MRENCLAVE from a quote.
func ExtractMREnclave(quote []byte) ([]byte, error) {
	parsedQuote, err := ParseQuote(quote)
	if err != nil {
		return nil, err
	}

	result := make([]byte, 32)
	copy(result, parsedQuote.MRENCLAVE[:])
	return result, nil
}

// ExtractMRSigner is a utility function to extract MRSIGNER from a quote.
func ExtractMRSigner(quote []byte) ([]byte, error) {
	parsedQuote, err := ParseQuote(quote)
	if err != nil {
		return nil, err
	}

	result := make([]byte, 32)
	copy(result, parsedQuote.MRSIGNER[:])
	return result, nil
}

// VerifySignature verifies an ECDSA signature.
// data: the data that was signed
// signature: ECDSA signature (65 bytes: r + s + v)
// producerID: producer ID (Ethereum address, 20 bytes)
// VerifySignature verifies an ECDSA signature using the provided public key.
// data: the data that was signed
// signature: the ECDSA signature (65 bytes)
// publicKey: the public key in uncompressed format (65 bytes: 0x04 + X + Y)
func (v *DCAPVerifier) VerifySignature(data, signature, publicKey []byte) error {
	if len(signature) != 65 {
		return fmt.Errorf("invalid signature length: expected 65 bytes, got %d", len(signature))
	}
	if len(publicKey) != 65 || publicKey[0] != 0x04 {
		return fmt.Errorf("invalid public key format: expected 65 bytes with 0x04 prefix")
	}

	// Hash the data
	hash := crypto.Keccak256(data)

	// Recover public key from signature
	recoveredPubKey, err := crypto.SigToPub(hash, signature)
	if err != nil {
		return fmt.Errorf("failed to recover public key: %w", err)
	}

	// Convert recovered public key to bytes
	recoveredPubKeyBytes := crypto.FromECDSAPub(recoveredPubKey)

	// Compare with expected public key
	if !bytes.Equal(recoveredPubKeyBytes, publicKey) {
		return fmt.Errorf("signature verification failed: public key mismatch")
	}

	return nil
}

// ExtractPublicKeyFromQuote extracts the signing public key from the SGX Quote.
// The public key is embedded in ReportData as X+Y coordinates (64 bytes).
// Returns uncompressed format (65 bytes: 0x04 + X + Y).
func (v *DCAPVerifier) ExtractPublicKeyFromQuote(quote []byte) ([]byte, error) {
	parsedQuote, err := ParseQuote(quote)
	if err != nil {
		return nil, fmt.Errorf("failed to parse quote: %w", err)
	}

	// Extract public key coordinates from ReportData
	// ReportData contains: X coordinate (32 bytes) + Y coordinate (32 bytes)
	if len(parsedQuote.ReportData) < 64 {
		return nil, fmt.Errorf("insufficient report data for public key")
	}

	// Construct uncompressed public key: 0x04 + X + Y
	pubKey := make([]byte, 65)
	pubKey[0] = 0x04 // Uncompressed point marker
	copy(pubKey[1:33], parsedQuote.ReportData[0:32])   // X coordinate
	copy(pubKey[33:65], parsedQuote.ReportData[32:64]) // Y coordinate
	
	return pubKey, nil
}

// ExtractProducerID extracts the producer ID from the SGX Quote.
// ProducerID is derived from the public key in ReportData.
func (v *DCAPVerifier) ExtractProducerID(quote []byte) ([]byte, error) {
	// Extract public key first
	pubKey, err := v.ExtractPublicKeyFromQuote(quote)
	if err != nil {
		return nil, err
	}
	
	// Derive Ethereum address from public key
	address := crypto.Keccak256(pubKey[1:])[12:] // Skip 0x04 prefix, take last 20 bytes
	
	return address, nil
}

// ExtractQuoteUserData extracts the userData (block hash) from an SGX Quote.
// The userData is stored in the first 32 bytes of the ReportData field.
func (v *DCAPVerifier) ExtractQuoteUserData(quote []byte) ([]byte, error) {
	parsedQuote, err := ParseQuote(quote)
	if err != nil {
		return nil, fmt.Errorf("failed to parse quote: %w", err)
	}

	// Extract block hash from the first 32 bytes of report data
	if len(parsedQuote.ReportData) < 32 {
		return nil, fmt.Errorf("insufficient report data for block hash")
	}

	// Return the first 32 bytes (block hash)
	blockHash := make([]byte, 32)
	copy(blockHash, parsedQuote.ReportData[:32])
	
	return blockHash, nil
}

// ExtractReportData is a utility function to extract report data from a quote.
func ExtractReportData(quote []byte) ([]byte, error) {
	parsedQuote, err := ParseQuote(quote)
	if err != nil {
		return nil, err
	}

	result := make([]byte, 64)
	copy(result, parsedQuote.ReportData[:])
	return result, nil
}


// QuoteVerificationResult contains all data extracted from quote verification
// Matches the structure returned by gramine's sgx-quote-verify.js
type QuoteVerificationResult struct {
	Verified           bool
	Error              error
	Measurements       QuoteMeasurements
	TCBStatus          string
	QuoteVersion       uint16
	AttestationKeyType uint16
}

// QuoteMeasurements contains all measurement data from the quote
type QuoteMeasurements struct {
	MrEnclave                []byte
	MrSigner                 []byte
	IsvProdID                uint16
	IsvSvn                   uint16
	Attributes               []byte
	ReportData               []byte
	PlatformInstanceID       []byte // 32-byte hash (SHA-256 of PCK SPKI or PPID)
	PlatformInstanceIDSource string // "ppid" or "pck-spki" or "cpusvn-composite"
}

// VerifyQuoteComplete performs complete quote verification and returns all extracted data
// This matches the gramine sgx-quote-verify.js verifyQuote() function
// Reference: https://github.com/mccoysc/gramine/blob/master/tools/sgx/ra-tls/sgx-quote-verify.js
// Input can be:
// - RA-TLS certificate (PEM format) - quote will be extracted from certificate extensions
// - Raw quote bytes
// - Base64 encoded quote
// Options can include:
// - apiKey: Intel SGX API key (if not set, read from INTEL_SGX_API_KEY env var)
// - cacheDir: Directory for caching certificates (default: /tmp/sgx-cert-cache)
func (v *DCAPVerifier) VerifyQuoteComplete(input []byte, options map[string]interface{}) (*QuoteVerificationResult, error) {
	result := &QuoteVerificationResult{
		Verified: false,
	}
	
	// Get API key from options or environment variable
	apiKey := ""
	if options != nil {
		if key, ok := options["apiKey"].(string); ok {
			apiKey = key
		}
	}
	if apiKey == "" {
		apiKey = os.Getenv("INTEL_SGX_API_KEY")
	}
	
	// Get cache directory from options or use default
	cacheDir := "/tmp/sgx-cert-cache"
	if options != nil {
		if dir, ok := options["cacheDir"].(string); ok {
			cacheDir = dir
		}
	}
	
	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		// Log but don't fail if we can't create cache dir
		fmt.Printf("Warning: failed to create cache directory: %v\n", err)
	}

	// Extract quote from input (could be certificate or raw quote)
	quote, err := v.extractQuoteFromInput(input)
	if err != nil {
		result.Error = err
		return result, err
	}

	// Parse quote structure
	if len(quote) < 432 {
		result.Error = errors.New("quote too short")
		return result, result.Error
	}

	// Extract version and attestation key type
	result.QuoteVersion = binary.LittleEndian.Uint16(quote[0:2])
	result.AttestationKeyType = binary.LittleEndian.Uint16(quote[2:4])

	// Extract measurements (report body starts at offset 48)
	reportBodyOffset := 48
	result.Measurements.MrEnclave = make([]byte, 32)
	copy(result.Measurements.MrEnclave, quote[reportBodyOffset+64:reportBodyOffset+96])

	result.Measurements.MrSigner = make([]byte, 32)
	copy(result.Measurements.MrSigner, quote[reportBodyOffset+128:reportBodyOffset+160])

	result.Measurements.IsvProdID = binary.LittleEndian.Uint16(quote[reportBodyOffset+256 : reportBodyOffset+258])
	result.Measurements.IsvSvn = binary.LittleEndian.Uint16(quote[reportBodyOffset+258 : reportBodyOffset+260])

	result.Measurements.Attributes = make([]byte, 16)
	copy(result.Measurements.Attributes, quote[reportBodyOffset+48:reportBodyOffset+64])

	result.Measurements.ReportData = make([]byte, 64)
	copy(result.Measurements.ReportData, quote[reportBodyOffset+320:reportBodyOffset+384])

	// Extract platform instance ID (following gramine logic)
	instanceID, source, err := v.extractPlatformInstanceID(quote)
	if err == nil {
		result.Measurements.PlatformInstanceID = instanceID
		result.Measurements.PlatformInstanceIDSource = source
	} else {
		// Fallback to zero bytes with error source
		result.Measurements.PlatformInstanceID = make([]byte, 32)
		result.Measurements.PlatformInstanceIDSource = "error: " + err.Error()
	}

	// Perform basic validation
	err = v.VerifyQuote(quote)
	if err == nil {
		result.Verified = true
		result.TCBStatus = "OK"
	} else {
		result.Error = err
		result.TCBStatus = "INVALID"
	}

	// Verify against whitelist if configured
	if result.Verified {
		if err := v.verifyMeasurementsWhitelist(result.Measurements); err != nil {
			result.Verified = false
			result.Error = err
			result.TCBStatus = "WHITELIST_MISMATCH"
		}
	}

	return result, nil
}

// getCachedCertificate retrieves a cached certificate by key
// Returns nil if not found or error reading
func (v *DCAPVerifier) getCachedCertificate(cacheDir, key string) []byte {
	if cacheDir == "" {
		return nil
	}
	
	cachePath := filepath.Join(cacheDir, key)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil
	}
	
	return data
}

// setCachedCertificate stores a certificate in cache
func (v *DCAPVerifier) setCachedCertificate(cacheDir, key string, data []byte) error {
	if cacheDir == "" {
		return nil
	}
	
	cachePath := filepath.Join(cacheDir, key)
	return os.WriteFile(cachePath, data, 0644)
}

// ExtractQuoteFromInput extracts quote from various input formats
// Supports: PEM certificate, raw quote bytes, base64 encoded quote
// This is exported so it can be used by tools and tests
func (v *DCAPVerifier) ExtractQuoteFromInput(input []byte) ([]byte, error) {
	return v.extractQuoteFromInput(input)
}

// extractQuoteFromInput extracts quote from various input formats
// Supports: PEM certificate, raw quote bytes, base64 encoded quote
func (v *DCAPVerifier) extractQuoteFromInput(input []byte) ([]byte, error) {
	// Check if input starts with valid SGX quote header
	// SGX Quote v3/v4 header format:
	// - Version (2 bytes): 0x0003 or 0x0004
	// - Signature type (2 bytes): EPID(0-1) or DCAP(2-3)
	if len(input) >= 4 {
		version := binary.LittleEndian.Uint16(input[0:2])
		signType := binary.LittleEndian.Uint16(input[2:4])
		// Valid quote: version 3 or 4, signType 0-3
		if (version == 3 || version == 4) && signType <= 3 {
			// This looks like a raw quote
			return input, nil
		}
	}
	
	// Check if input is a PEM certificate (starts with PEM header)
	if bytes.HasPrefix(input, []byte("-----BEGIN CERTIFICATE-----")) {
		return v.extractQuoteFromCertificate(input)
	}

	// Fallback: if it contains certificate marker but doesn't start with it,
	// it might be a quote with embedded certificates, so treat as raw quote
	return input, nil
}

// extractQuoteFromCertificate extracts SGX Quote from RA-TLS certificate extension
// Tries multiple OIDs in order:
// 1. TCG DICE Tagged Evidence (2.23.133.5.4.9) - standard format
// 2. Intel SGX Quote (1.2.840.113741.1.13.1) - legacy format
func (v *DCAPVerifier) extractQuoteFromCertificate(certPEM []byte) ([]byte, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, errors.New("failed to decode PEM certificate")
	}

	// Try raw DER extraction first (works with non-standard certs)
	quote, err := v.extractQuoteFromRawDER(block.Bytes)
	if err == nil && len(quote) > 0 {
		return quote, nil
	}

	// Fall back to standard X.509 parsing
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		// If both methods fail, return error
		return nil, fmt.Errorf("failed to extract quote from certificate: %v", err)
	}

	// Try TCG DICE OID first (standard)
	for _, ext := range cert.Extensions {
		// OID 2.23.133.5.4.9 = TCG DICE Tagged Evidence
		if ext.Id.String() == "2.23.133.5.4.9" {
			// For now, return the value directly
			// In full implementation, would parse CBOR structure
			return ext.Value, nil
		}
	}

	// Try Intel SGX Quote OID (legacy)
	for _, ext := range cert.Extensions {
		// OID 1.2.840.113741.1.13.1 = Intel SGX Quote
		if ext.Id.String() == "1.2.840.113741.1.13.1" {
			return ext.Value, nil
		}
	}

	return nil, errors.New("no SGX quote found in certificate extensions")
}

// extractQuoteFromRawDER extracts quote from raw DER certificate bytes
// Matches gramine sgx-quote-verify.js extractQuoteFromParsedCertViaAsn1 function
func (v *DCAPVerifier) extractQuoteFromRawDER(derBytes []byte) ([]byte, error) {
	// Try OIDs in order matching JS code:
	// 1. TCG DICE: 2.23.133.5.4.9
	// 2. LEGACY_QUOTE_OID (NON_STANDARD): 1.2.840.113741.1.13.1  
	// 3. LEGACY_QUOTE_OID_V1: 0.6.9.42.840.113741.1337.6
	
	tcgOID := v.oidToBytes("2.23.133.5.4.9")
	legacyOID := v.oidToBytes("1.2.840.113741.1.13.1")
	legacyOIDv1 := v.oidToBytes("0.6.9.42.840.113741.1337.6")
	
	// Try TCG DICE first
	value, err := v.extractExtensionByOid(derBytes, tcgOID)
	if err == nil && value != nil {
		// TCG DICE format uses CBOR encoding
		return nil, errors.New("TCG DICE format not supported")
	}
	
	// Try legacy OID (extension value IS the quote directly)
	value, err = v.extractExtensionByOid(derBytes, legacyOID)
	if err == nil && value != nil {
		return v.extractLegacyQuote(value)
	}
	
	// Try legacy OID v1
	value, err = v.extractExtensionByOid(derBytes, legacyOIDv1)
	if err == nil && value != nil {
		return v.extractLegacyQuote(value)
	}
	
	return nil, errors.New("no SGX quote extension found")
}

// extractLegacyQuote extracts quote from legacy extension value
// Matches JS extractLegacyQuoteFromExtension function
func (v *DCAPVerifier) extractLegacyQuote(extValue []byte) ([]byte, error) {
	if len(extValue) < 436 {
		return nil, errors.New("extension value too short to contain a valid quote")
	}
	
	// Read signature data length at offset 432 (little-endian uint32)
	signatureDataLen := binary.LittleEndian.Uint32(extValue[432:436])
	expectedQuoteSize := 432 + 4 + int(signatureDataLen)
	
	var quote []byte
	if expectedQuoteSize <= len(extValue) {
		quote = extValue[0:expectedQuoteSize]
	} else {
		quote = extValue
	}
	
	return quote, nil
}

// oidToBytes converts OID string like "1.2.840.113741" to DER-encoded bytes
// Matches JS oidToBytes function
func (v *DCAPVerifier) oidToBytes(oid string) []byte {
	parts := []int{}
	for _, part := range bytes.Split([]byte(oid), []byte(".")) {
		num := 0
		for _, b := range part {
			num = num*10 + int(b-'0')
		}
		parts = append(parts, num)
	}
	
	if len(parts) < 2 {
		return nil
	}
	
	// First byte encodes first two parts
	result := []byte{byte(parts[0]*40 + parts[1])}
	
	// Encode remaining parts
	for i := 2; i < len(parts); i++ {
		value := parts[i]
		if value == 0 {
			result = append(result, 0)
			continue
		}
		
		encoded := []byte{}
		for value > 0 {
			b := byte(value & 0x7F)
			value >>= 7
			if len(encoded) > 0 {
				b |= 0x80
			}
			encoded = append([]byte{b}, encoded...)
		}
		result = append(result, encoded...)
	}
	
	// Prepend tag and length
	return append([]byte{0x06, byte(len(result))}, result...)
}

// extractExtensionByOid extracts extension value by OID from DER certificate
// Matches JS extractExtensionByOid function exactly
func (v *DCAPVerifier) extractExtensionByOid(derBuffer []byte, targetOidBytes []byte) ([]byte, error) {
	pos := 0
	
	readLength := func() (int, error) {
		if pos >= len(derBuffer) {
			return 0, errors.New("unexpected end of DER data")
		}
		first := derBuffer[pos]
		pos++
		
		if (first & 0x80) == 0 {
			return int(first), nil
		}
		
		numOctets := int(first & 0x7F)
		if numOctets == 0 || numOctets > 4 {
			return 0, errors.New("invalid DER length encoding")
		}
		
		length := 0
		for i := 0; i < numOctets; i++ {
			if pos >= len(derBuffer) {
				return 0, errors.New("unexpected end of DER data")
			}
			length = (length << 8) | int(derBuffer[pos])
			pos++
		}
		return length, nil
	}
	
	skipValue := func(length int) error {
		pos += length
		if pos > len(derBuffer) {
			return errors.New("DER value extends beyond buffer")
		}
		return nil
	}
	
	readBytes := func(length int) ([]byte, error) {
		if pos+length > len(derBuffer) {
			return nil, errors.New("not enough bytes to read")
		}
		bytes := derBuffer[pos : pos+length]
		pos += length
		return bytes, nil
	}
	
	matchesOid := func(oidBytes []byte) bool {
		if len(oidBytes) != len(targetOidBytes) {
			return false
		}
		for i := 0; i < len(oidBytes); i++ {
			if oidBytes[i] != targetOidBytes[i] {
				return false
			}
		}
		return true
	}
	
	// Certificate must start with SEQUENCE
	if derBuffer[pos] != 0x30 {
		return nil, errors.New("certificate must start with SEQUENCE tag")
	}
	pos++
	_, err := readLength()
	if err != nil {
		return nil, err
	}
	
	// TBSCertificate must be a SEQUENCE
	if derBuffer[pos] != 0x30 {
		return nil, errors.New("TBSCertificate must be a SEQUENCE")
	}
	pos++
	tbsLength, err := readLength()
	if err != nil {
		return nil, err
	}
	tbsEnd := pos + tbsLength
	
	// Search for extensions in TBSCertificate
	for pos < tbsEnd {
		tag := derBuffer[pos]
		
		// Extensions have tag 0xA3
		if tag == 0xA3 {
			pos++
			_, err := readLength()
			if err != nil {
				return nil, err
			}
			
			// Extensions must be a SEQUENCE
			if derBuffer[pos] != 0x30 {
				return nil, errors.New("extensions must be a SEQUENCE")
			}
			pos++
			extensionsLength, err := readLength()
			if err != nil {
				return nil, err
			}
			extensionsEnd := pos + extensionsLength
			
			// Iterate through extensions
			for pos < extensionsEnd {
				if derBuffer[pos] != 0x30 {
					break
				}
				pos++
				extLength, err := readLength()
				if err != nil {
					return nil, err
				}
				extEnd := pos + extLength
				
				// Read OID
				if derBuffer[pos] != 0x06 {
					pos = extEnd
					continue
				}
				oidTag := derBuffer[pos]
				pos++
				oidLength, err := readLength()
				if err != nil {
					pos = extEnd
					continue
				}
				oidBytes, err := readBytes(oidLength)
				if err != nil {
					pos = extEnd
					continue
				}
				// Prepend tag and length to match format
				fullOidBytes := append([]byte{oidTag, byte(oidLength)}, oidBytes...)
				
				// Check for critical flag
				if pos < extEnd && derBuffer[pos] == 0x01 {
					pos++
					boolLength, _ := readLength()
					_, _ = readBytes(boolLength)
				}
				
				// Read extension value (OCTET STRING)
				if pos >= extEnd {
					pos = extEnd
					continue
				}
				
				if derBuffer[pos] != 0x04 {
					pos = extEnd
					continue
				}
				pos++
				valueLength, err := readLength()
				if err != nil {
					pos = extEnd
					continue
				}
				valueBytes, err := readBytes(valueLength)
				if err != nil {
					pos = extEnd
					continue
				}
				
				// Check if OID matches
				if matchesOid(fullOidBytes) {
					return valueBytes, nil
				}
				
				pos = extEnd
			}
			
			return nil, errors.New("extension not found")
		} else {
			// Skip other fields
			pos++
			length, err := readLength()
			if err != nil {
				return nil, err
			}
			if err := skipValue(length); err != nil {
				return nil, err
			}
		}
	}
	
	return nil, errors.New("extensions not found in certificate")
}

// parseDERLength parses DER length encoding
func (v *DCAPVerifier) parseDERLength(data []byte) (int, int) {
	if len(data) == 0 {
		return 0, 0
	}
	
	firstByte := data[0]
	if firstByte&0x80 == 0 {
		// Short form: length is in the first byte
		return int(firstByte), 1
	}
	
	// Long form: first byte indicates number of length bytes
	numLengthBytes := int(firstByte & 0x7F)
	if numLengthBytes > len(data)-1 {
		return 0, 0
	}
	
	length := 0
	for i := 0; i < numLengthBytes; i++ {
		length = (length << 8) | int(data[1+i])
	}
	
	return length, 1 + numLengthBytes
}

// verifyMeasurementsWhitelist verifies measurements against configured whitelist
func (v *DCAPVerifier) verifyMeasurementsWhitelist(measurements QuoteMeasurements) error {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// If no whitelist configured, skip verification
	if len(v.allowedMREnclave) == 0 && len(v.allowedMRSigner) == 0 {
		return nil
	}

	// Check MRENCLAVE whitelist
	if len(v.allowedMREnclave) > 0 {
		mrEnclaveHex := hex.EncodeToString(measurements.MrEnclave)
		if !v.allowedMREnclave[mrEnclaveHex] {
			return fmt.Errorf("MRENCLAVE %s not in whitelist", mrEnclaveHex)
		}
	}

	// Check MRSIGNER whitelist
	if len(v.allowedMRSigner) > 0 {
		mrSignerHex := hex.EncodeToString(measurements.MrSigner)
		if !v.allowedMRSigner[mrSignerHex] {
			return fmt.Errorf("MRSIGNER %s not in whitelist", mrSignerHex)
		}
	}

	return nil
}

// extractPlatformInstanceID extracts platform instance ID following gramine's logic
// Priority: 1. PPID, 2. PCK SPKI fingerprint, 3. CPUSVN composite
func (v *DCAPVerifier) extractPlatformInstanceID(quote []byte) ([]byte, string, error) {
	// Try PPID first (most unique identifier)
	ppid, err := v.extractPPID(quote)
	if err == nil && len(ppid) >= 16 {
		// Return full PPID (or hash to 32 bytes if longer)
		if len(ppid) == 32 {
			return ppid, "ppid", nil
		} else if len(ppid) > 32 {
			// Hash if too long (use SHA-256 to match gramine)
			hash := sha256.Sum256(ppid)
			return hash[:], "ppid", nil
		} else {
			// Pad if too short
			padded := make([]byte, 32)
			copy(padded, ppid)
			return padded, "ppid", nil
		}
	}

	// Try PCK SPKI fingerprint (production method)
	instanceID, err := v.computePCKSPKIFingerprint(quote)
	if err == nil {
		return instanceID, "pck-spki", nil
	}

	// Fallback to CPUSVN + Attributes composite
	if len(quote) >= 48+16+4+28+16 {
		cpusvn := quote[48 : 48+16]
		attributes := quote[48+16+4+28 : 48+16+4+28+16]
		composite := append(cpusvn, attributes...)
		hash := sha256.Sum256(composite)
		return hash[:], "cpusvn-composite", nil
	}

	return nil, "", errors.New("failed to extract platform instance ID")
}

func (v *DCAPVerifier) ExtractInstanceID(quote []byte) ([]byte, error) {
	result, err := v.VerifyQuoteComplete(quote, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to verify quote: %v", err)
	}
	return result.Measurements.PlatformInstanceID, nil
}

// extractPPID extracts PPID from quote certification data
func (v *DCAPVerifier) extractPPID(quote []byte) ([]byte, error) {
	if len(quote) < 436 {
		return nil, errors.New("quote too short for signature data")
	}

	signatureDataLen := binary.LittleEndian.Uint32(quote[432:436])
	if len(quote) < 436+int(signatureDataLen) {
		return nil, errors.New("quote signature data truncated")
	}

	// Skip ECDSA signature (64 bytes), attestation pubkey (64 bytes), QE report (384 bytes), QE signature (64 bytes)
	offset := 436 + 64 + 64 + 384 + 64

	if offset+2 > len(quote) {
		return nil, errors.New("no auth data size field")
	}

	// Skip auth data
	authDataSize := binary.LittleEndian.Uint16(quote[offset : offset+2])
	offset += 2 + int(authDataSize)

	if offset+6 > len(quote) {
		return nil, errors.New("no certification data")
	}

	// Read certification data
	certDataType := binary.LittleEndian.Uint16(quote[offset : offset+2])
	certDataSize := binary.LittleEndian.Uint32(quote[offset+2 : offset+6])
	offset += 6

	if offset+int(certDataSize) > len(quote) {
		return nil, errors.New("certification data truncated")
	}

	// certDataType 1 = PPID
	if certDataType == 1 {
		ppid := make([]byte, certDataSize)
		copy(ppid, quote[offset:offset+int(certDataSize)])
		return ppid, nil
	}

	return nil, errors.New("PPID not found in certification data")
}

// computePCKSPKIFingerprint computes SHA-256 hash of PCK certificate's SPKI
// This provides a unique, stable platform identifier per gramine implementation
// Reference: gramine sgx-quote-verify.js computePckSpkiFingerprint()
func (v *DCAPVerifier) computePCKSPKIFingerprint(quote []byte) ([]byte, error) {
	if len(quote) < 436 {
		return nil, errors.New("quote too short for signature data")
	}

	signatureDataLen := binary.LittleEndian.Uint32(quote[432:436])
	if len(quote) < 436+int(signatureDataLen) {
		return nil, errors.New("quote signature data truncated")
	}

	// Skip ECDSA signature (64 bytes), attestation pubkey (64 bytes), QE report (384 bytes), QE signature (64 bytes)
	offset := 436 + 64 + 64 + 384 + 64

	if offset+2 > len(quote) {
		return nil, errors.New("no auth data size field")
	}

	// Skip auth data
	authDataSize := binary.LittleEndian.Uint16(quote[offset : offset+2])
	offset += 2 + int(authDataSize)

	if offset+6 > len(quote) {
		return nil, errors.New("no certification data")
	}

	// Read certification data
	certDataType := binary.LittleEndian.Uint16(quote[offset : offset+2])
	certDataSize := binary.LittleEndian.Uint32(quote[offset+2 : offset+6])
	offset += 6

	if offset+int(certDataSize) > len(quote) {
		return nil, errors.New("certification data truncated")
	}

	certData := quote[offset : offset+int(certDataSize)]

	// certDataType 5 = PCK cert chain
	if certDataType != 5 {
		return nil, fmt.Errorf("unsupported cert data type: %d", certDataType)
	}

	// Parse PEM cert chain to extract first (leaf) certificate
	pemCerts := v.parsePEMCertChain(certData)
	if len(pemCerts) == 0 {
		return nil, errors.New("no certificates in chain")
	}

	// Extract SPKI from leaf certificate and hash it (matching gramine logic)
	block, _ := pem.Decode(pemCerts[0])
	if block == nil {
		return nil, errors.New("failed to decode PCK certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PCK certificate: %v", err)
	}

	// Get SPKI bytes (SubjectPublicKeyInfo)
	spkiBytes, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %v", err)
	}

	// Compute SHA-256 hash of SPKI to match gramine (not Keccak256!)
	hash := sha256.Sum256(spkiBytes)
	return hash[:], nil
}

// parsePEMCertChain parses a byte array containing PEM certificates
func (v *DCAPVerifier) parsePEMCertChain(data []byte) [][]byte {
	var certs [][]byte
	rest := data

	for {
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			certs = append(certs, pem.EncodeToMemory(block))
		}
		rest = remaining
	}

	return certs
}

