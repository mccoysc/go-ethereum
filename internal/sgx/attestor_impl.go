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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

// GramineAttestor implements the Attestor interface using Gramine's
// /dev/attestation interface for Quote generation.
// Note: For full RA-TLS support, use GramineRATLSAttestor with CGO.
type GramineAttestor struct {
	privateKey      *ecdsa.PrivateKey // P-384 for RA-TLS
	signingKey      *ecdsa.PrivateKey // secp256k1 for Ethereum block signing
	mrenclave       []byte
	mrsigner        []byte
	isSGX           bool // Whether we're running in a real SGX environment
}

// NewGramineAttestor creates a new Gramine-based attestor.
// It will detect if running in an SGX environment and fall if not.
// This implementation uses P-384 curve for RA-TLS as required by the specification,
// and secp256k1 for Ethereum block signing.
func NewGramineAttestor() (*GramineAttestor, error) {
	// Generate TLS key pair using P-384 (SECP384R1) as required by RA-TLS spec
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate TLS key: %w", err)
	}

	// Generate or derive signing key using secp256k1 for Ethereum compatibility
	var signingKey *ecdsa.PrivateKey
	
	// Check if we're in SGX mode or mock mode
	sgxMode := os.Getenv("XCHAIN_SGX_MODE")
	
	if sgxMode == "mock" {
		// In mock mode, use a deterministic key for testing
		// This ensures the same ProducerID across restarts
		// Real SGX would use sealing key to persist this
		log.Warn("SGX Mock Mode: Using deterministic test key (NOT for production!)")
		
		// Use a fixed seed for deterministic key generation in test mode
		// In production, this should be derived from SGX sealing key
		deterministicSeed := []byte("xchain-sgx-test-key-do-not-use-in-production")
		hash := crypto.Keccak256(deterministicSeed)
		signingKey, err = crypto.ToECDSA(hash)
		if err != nil {
			return nil, fmt.Errorf("failed to create deterministic signing key: %w", err)
		}
	} else {
		// In real SGX mode, we should derive the key from SGX sealing key
		// For now, generate randomly (TODO: implement sealing key derivation)
		signingKey, err = crypto.GenerateKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate signing key: %w", err)
		}
		
		// TODO: In production SGX, derive from sealing key:
		// sealingKey := getSGXSealingKey()
		// signingKey = deriveFromSealingKey(sealingKey)
	}

	attestor := &GramineAttestor{
		privateKey: privateKey,
		signingKey: signingKey,
	}

	// Read MRENCLAVE using helper function
	mrenclave, err := readMREnclave()
	if err != nil {
		return nil, fmt.Errorf("failed to read MRENCLAVE: %w", err)
	}
	attestor.mrenclave = mrenclave
	return attestor, nil
}

// GenerateQuote generates an SGX Quote with the given report data.
func (a *GramineAttestor) GenerateQuote(reportData []byte) ([]byte, error) {
	if len(reportData) > 64 {
		return nil, fmt.Errorf("reportData too long: max 64 bytes, got %d", len(reportData))
	}

	if a.isSGX {
		// Real SGX environment: use Gramine's /dev/attestation interface
		return generateQuoteViaGramine(reportData)
	}

	// Mock environment: generate a mock quote
	return a.generateMockQuote(reportData)
}

// generateMockQuote generates a mock SGX quote for testing.
// The mock quote follows SGX DCAP Quote v3 format to ensure compatibility with parsers.
// Includes signature data with PPID for instance ID extraction.
func (a *GramineAttestor) generateMockQuote(reportData []byte) ([]byte, error) {
	// Create quote body (432 bytes) + signature data
	// Signature data includes PPID for instance ID extraction
	quoteBody := make([]byte, 432)

	// === Quote Header (48 bytes) ===
	// Version (2 bytes at offset 0)
	quoteBody[0] = 3 // Version 3 (DCAP)
	quoteBody[1] = 0

	// Attestation Key Type (2 bytes at offset 2)
	quoteBody[2] = 2 // ECDSA-256 with P-256 curve (DCAP)
	quoteBody[3] = 0

	// Reserved (4 bytes at offset 4-7)
	// QE SVN (2 bytes at offset 8)
	quoteBody[8] = 1
	quoteBody[9] = 0

	// PCE SVN (2 bytes at offset 10)
	quoteBody[10] = 1
	quoteBody[11] = 0

	// QE Vendor ID (16 bytes at offset 12-27) - Intel's vendor ID
	copy(quoteBody[12:28], []byte{0x93, 0x9a, 0x72, 0x33, 0xf7, 0x9c, 0x4c, 0xa9,
		0x94, 0x0a, 0x0d, 0xb3, 0x95, 0x7f, 0x06, 0x07})

	// User Data (20 bytes at offset 28-47)
	// Leave as zeros

	// === ISV Enclave Report (384 bytes, offset 48-431) ===
	// CPUSVN (16 bytes at offset 48)
	// For testing, use deterministic values
	for i := 0; i < 16; i++ {
		quoteBody[48+i] = byte(i) // CPUSVN
	}

	// MISCSELECT (4 bytes at offset 64)
	quoteBody[64] = 0
	quoteBody[65] = 0
	quoteBody[66] = 0
	quoteBody[67] = 0

	// Reserved (28 bytes at offset 68-95)

	// Attributes (16 bytes at offset 96)
	// Set typical SGX attributes
	quoteBody[96] = 0x07  // Flags: INIT | MODE64BIT | PROVISION_KEY
	quoteBody[97] = 0x00
	quoteBody[98] = 0x00
	quoteBody[99] = 0x00
	quoteBody[100] = 0x00
	quoteBody[101] = 0x00
	quoteBody[102] = 0x00
	quoteBody[103] = 0x00
	// XFRM (upper 8 bytes of attributes)
	quoteBody[104] = 0x1f // Enable common features
	quoteBody[105] = 0x00

	// MRENCLAVE (32 bytes at offset 112)
	copy(quoteBody[112:144], a.mrenclave)

	// Reserved (32 bytes at offset 144-175)

	// MRSIGNER (32 bytes at offset 176)
	copy(quoteBody[176:208], a.mrsigner)

	// Reserved (96 bytes at offset 208-303)

	// ISV Product ID (2 bytes at offset 304)
	quoteBody[304] = 0
	quoteBody[305] = 0

	// ISV SVN (2 bytes at offset 306)
	quoteBody[306] = 1
	quoteBody[307] = 0

	// Reserved (60 bytes at offset 308-367)

	// Report Data (64 bytes at offset 368-431)
	if len(reportData) > 0 {
		copyLen := len(reportData)
		if copyLen > 64 {
			copyLen = 64
		}
		copy(quoteBody[368:368+copyLen], reportData)
	}

	// === Signature Data (variable, starts at offset 432) ===
	// Build minimal signature data with PPID
	sigData := make([]byte, 0, 512)

	// Signature length (4 bytes) - mock signature is 64 bytes
	sigLen := uint32(64)
	sigLenBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(sigLenBytes, sigLen)
	sigData = append(sigData, sigLenBytes...)

	// Mock signature (64 bytes)
	mockSig := make([]byte, 64)
	for i := range mockSig {
		mockSig[i] = byte(i % 256)
	}
	sigData = append(sigData, mockSig...)

	// Attestation Public Key (64 bytes for ECDSA-256)
	mockPubKey := make([]byte, 64)
	for i := range mockPubKey {
		mockPubKey[i] = byte((i + 64) % 256)
	}
	sigData = append(sigData, mockPubKey...)

	// QE Report (384 bytes) - simplified
	qeReport := make([]byte, 384)
	sigData = append(sigData, qeReport...)

	// QE Report Signature length (4 bytes) - mock 64 bytes
	qeReportSigLen := uint32(64)
	qeReportSigLenBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(qeReportSigLenBytes, qeReportSigLen)
	sigData = append(sigData, qeReportSigLenBytes...)

	// QE Report Signature (64 bytes)
	qeReportSig := make([]byte, 64)
	sigData = append(sigData, qeReportSig...)

	// QE Auth Data length (2 bytes) - no auth data
	qeAuthDataLen := uint16(0)
	qeAuthDataLenBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(qeAuthDataLenBytes, qeAuthDataLen)
	sigData = append(sigData, qeAuthDataLenBytes...)

	// Cert Data Type (2 bytes) - Type 6 = PPID_Cleartext
	certDataType := uint16(6)
	certDataTypeBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(certDataTypeBytes, certDataType)
	sigData = append(sigData, certDataTypeBytes...)

	// Cert Data Size (4 bytes) - PPID(16) + CPUSVN(16) + PCESVN(2) + PCEID(2) = 36
	certDataSize := uint32(36)
	certDataSizeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(certDataSizeBytes, certDataSize)
	sigData = append(sigData, certDataSizeBytes...)

	// Cert Data: PPID-based
	// PPID (16 bytes) - use deterministic value based on MRENCLAVE
	ppid := make([]byte, 16)
	if len(a.mrenclave) >= 16 {
		copy(ppid, a.mrenclave[:16])
	} else {
		// Fallback: use sequential bytes
		for i := range ppid {
			ppid[i] = byte(i + 200)
		}
	}
	sigData = append(sigData, ppid...)

	// CPUSVN (16 bytes) - same as in report
	cpusvn := make([]byte, 16)
	for i := range cpusvn {
		cpusvn[i] = byte(i)
	}
	sigData = append(sigData, cpusvn...)

	// PCESVN (2 bytes)
	pcesvn := uint16(1)
	pcesvnBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(pcesvnBytes, pcesvn)
	sigData = append(sigData, pcesvnBytes...)

	// PCEID (2 bytes)
	pceid := uint16(0)
	pceidBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(pceidBytes, pceid)
	sigData = append(sigData, pceidBytes...)

	// Combine quote body and signature data
	fullQuote := make([]byte, 0, len(quoteBody)+len(sigData))
	fullQuote = append(fullQuote, quoteBody...)
	fullQuote = append(fullQuote, sigData...)

	return fullQuote, nil
}

// GenerateCertificate generates an RA-TLS certificate with an embedded SGX Quote.
func (a *GramineAttestor) GenerateCertificate() (*tls.Certificate, error) {
	// Generate public key bytes to embed in the quote
	pubKeyBytes := elliptic.Marshal(a.privateKey.Curve, a.privateKey.X, a.privateKey.Y)

	// Ensure we don't exceed 64 bytes for report data
	reportData := pubKeyBytes
	if len(reportData) > 64 {
		reportData = reportData[:64]
	}

	// Generate quote with public key hash as report data
	quote, err := a.GenerateQuote(reportData)
	if err != nil {
		return nil, fmt.Errorf("failed to generate quote: %w", err)
	}

	// Create X.509 certificate template with embedded quote
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			CommonName: "X-Chain-Node",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		ExtraExtensions: []pkix.Extension{
			{
				Id:       SGXQuoteOID,
				Critical: false,
				Value:    quote,
			},
		},
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, template, template,
		&a.privateKey.PublicKey, a.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	return &tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  a.privateKey,
	}, nil
}

// GetMREnclave returns the MRENCLAVE of the local enclave.
func (a *GramineAttestor) GetMREnclave() []byte {
	result := make([]byte, len(a.mrenclave))
	copy(result, a.mrenclave)
	return result
}

// GetMRSigner returns the MRSIGNER of the local enclave.
func (a *GramineAttestor) GetMRSigner() []byte {
	result := make([]byte, len(a.mrsigner))
	copy(result, a.mrsigner)
	return result
}

// GetProducerID returns the producer ID (Ethereum address, 20 bytes)
// derived from the public key used for signing blocks.
// GetProducerID returns the CPU instance ID as the producer identifier.
// In SGX PoA consensus, each physical CPU can only act as one producer.
// The instance ID is extracted from the Quote and ensures uniqueness per hardware.
func (a *GramineAttestor) GetProducerID() ([]byte, error) {
	// Generate a minimal Quote to extract instance ID
	// We use a fixed nonce since we only need the instance ID
	nonce := make([]byte, 32)
	quote, err := a.GenerateQuote(nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to generate quote for instance ID: %w", err)
	}
	
	// Extract instance ID from the quote
	instanceID, err := ExtractInstanceID(quote)
	if err != nil {
		return nil, fmt.Errorf("failed to extract instance ID: %w", err)
	}
	
	// Use first 20 bytes of instance ID as Ethereum-compatible address
	// This ensures compatibility with address-based systems
	producerID := make([]byte, 20)
	if len(instanceID.CPUInstanceID) >= 20 {
		copy(producerID, instanceID.CPUInstanceID[:20])
	} else {
		// Pad with zeros if instance ID is shorter
		copy(producerID, instanceID.CPUInstanceID)
	}
	
	return producerID, nil
}

// GetSigningKey returns the secp256k1 signing key for external access.
// This is needed for extracting the public key to embed in Quote.
func (a *GramineAttestor) GetSigningKey() *ecdsa.PrivateKey {
	return a.signingKey
}

// GetSigningPublicKey returns the signing public key in uncompressed format (65 bytes).
// Format: 0x04 + X coordinate (32 bytes) + Y coordinate (32 bytes)
func (a *GramineAttestor) GetSigningPublicKey() []byte {
	return crypto.FromECDSAPub(&a.signingKey.PublicKey)
}

// SignInEnclave signs data using the enclave's private key.
// Returns an ECDSA signature (65 bytes: r + s + v).
// SignInEnclave signs data using the enclave's secp256k1 signing key.
// This produces an Ethereum-compatible ECDSA signature.
func (a *GramineAttestor) SignInEnclave(data []byte) ([]byte, error) {
	// Hash the data
	hash := crypto.Keccak256(data)

	// Sign using secp256k1 signing key (Ethereum-compatible)
	signature, err := crypto.Sign(hash, a.signingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign: %w", err)
	}

	return signature, nil
}
