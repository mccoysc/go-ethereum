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
	"crypto/x509"
	"errors"
	"fmt"
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

	// Check MRENCLAVE whitelist
	if !v.IsAllowedMREnclave(parsedQuote.MRENCLAVE[:]) {
		return fmt.Errorf("MRENCLAVE not in allowed list: %x", parsedQuote.MRENCLAVE)
	}

	return nil
}

// VerifyCertificate verifies an RA-TLS certificate.
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

	// Verify the quote
	if err := v.VerifyQuote(quote); err != nil {
		return fmt.Errorf("quote verification failed: %w", err)
	}

	// Verify that the certificate's public key matches the quote's report data
	parsedQuote, err := ParseQuote(quote)
	if err != nil {
		return fmt.Errorf("failed to parse quote: %w", err)
	}

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
	reportDataToCompare := parsedQuote.ReportData[:compareLen]
	pubKeyToCompare := pubKeyBytes[:compareLen]
	if !ConstantTimeCompare(reportDataToCompare, pubKeyToCompare) {
		return errors.New("certificate public key does not match quote report data")
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

// ExtractInstanceID extracts the CPU instance ID from the SGX Quote.
// This uses the existing ExtractInstanceID function from the sgx package.
func (v *DCAPVerifier) ExtractInstanceID(quote []byte) ([]byte, error) {
instanceID, err := ExtractInstanceID(quote)
if err != nil {
return nil, err
}
return instanceID.CPUInstanceID, nil
}

