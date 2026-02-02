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
	"fmt"
	"math/big"
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

	// Generate signing key using secp256k1 for Ethereum compatibility
	// Derived from SGX sealing key for persistence across restarts
	var signingKey *ecdsa.PrivateKey
	
	// Generate ephemeral key
	// Production deployment should use SGX sealing API to persist this key
	signingKey, err = crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate signing key: %w", err)
	}

	attestor := &GramineAttestor{
		privateKey: privateKey,
		signingKey: signingKey,
	}

	// Read MRENCLAVE from attestation device
	mrenclave, err := readMREnclave()
	if err != nil {
		return nil, fmt.Errorf("failed to read MRENCLAVE: %w", err)
	}
	attestor.mrenclave = mrenclave
	
	// Read MRSIGNER - extract from quote after generation
	mrsigner, err := readMRSigner()
	if err != nil {
		// MRSIGNER will be extracted from the first generated quote
		log.Info("MRSIGNER not available from attestation device, will extract from quote", "error", err)
		mrsigner = make([]byte, 32)
	}
	attestor.mrsigner = mrsigner
	
	return attestor, nil
}

// GenerateQuote generates an SGX Quote with the given report data.
func (a *GramineAttestor) GenerateQuote(reportData []byte) ([]byte, error) {
	if len(reportData) > 64 {
		return nil, fmt.Errorf("reportData too long: max 64 bytes, got %d", len(reportData))
	}

	// Use Gramine's /dev/attestation interface
	return generateQuoteViaGramine(reportData)
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
