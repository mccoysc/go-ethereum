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
	"os"
	"time"
)

// GramineAttestor implements the Attestor interface using Gramine's
// /dev/attestation interface.
type GramineAttestor struct {
	privateKey *ecdsa.PrivateKey
	mrenclave  []byte
	mrsigner   []byte
	isSGX      bool // Whether we're running in a real SGX environment
}

// NewGramineAttestor creates a new Gramine-based attestor.
// It will detect if running in an SGX environment and fall back to mock mode if not.
func NewGramineAttestor() (*GramineAttestor, error) {
	// Generate TLS key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	attestor := &GramineAttestor{
		privateKey: privateKey,
	}

	// Try to detect SGX environment by checking for /dev/attestation
	if _, err := os.Stat("/dev/attestation/my_target_info"); err == nil {
		attestor.isSGX = true
		// Read MRENCLAVE from /dev/attestation/my_target_info
		targetInfo, err := os.ReadFile("/dev/attestation/my_target_info")
		if err != nil {
			return nil, fmt.Errorf("failed to read MRENCLAVE: %w", err)
		}
		if len(targetInfo) >= 32 {
			attestor.mrenclave = make([]byte, 32)
			copy(attestor.mrenclave, targetInfo[:32])
		}
	} else {
		// Not in SGX environment, use mock values
		attestor.isSGX = false
		attestor.mrenclave = make([]byte, 32)
		attestor.mrsigner = make([]byte, 32)
		// Fill with deterministic test values
		for i := range attestor.mrenclave {
			attestor.mrenclave[i] = byte(i)
		}
		for i := range attestor.mrsigner {
			attestor.mrsigner[i] = byte(i + 32)
		}
	}

	return attestor, nil
}

// GenerateQuote generates an SGX Quote with the given report data.
func (a *GramineAttestor) GenerateQuote(reportData []byte) ([]byte, error) {
	if len(reportData) > 64 {
		return nil, fmt.Errorf("reportData too long: max 64 bytes, got %d", len(reportData))
	}

	if a.isSGX {
		// Real SGX environment: use Gramine's /dev/attestation interface
		return a.generateRealQuote(reportData)
	}

	// Mock environment: generate a mock quote
	return a.generateMockQuote(reportData)
}

// generateRealQuote generates a real SGX quote using Gramine.
func (a *GramineAttestor) generateRealQuote(reportData []byte) ([]byte, error) {
	// Pad report data to 64 bytes
	paddedData := make([]byte, 64)
	copy(paddedData, reportData)

	// Write report data to /dev/attestation/user_report_data
	err := os.WriteFile("/dev/attestation/user_report_data", paddedData, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to write user_report_data: %w", err)
	}

	// Read the generated quote from /dev/attestation/quote
	quote, err := os.ReadFile("/dev/attestation/quote")
	if err != nil {
		return nil, fmt.Errorf("failed to read quote: %w", err)
	}

	return quote, nil
}

// generateMockQuote generates a mock SGX quote for testing.
func (a *GramineAttestor) generateMockQuote(reportData []byte) ([]byte, error) {
	// Create a minimal mock quote (432 bytes minimum)
	quote := make([]byte, 432)

	// Version and sign type
	quote[0] = 3 // Version 3 (DCAP)
	quote[1] = 0

	// Copy MRENCLAVE at offset 112
	copy(quote[112:144], a.mrenclave)

	// Copy MRSIGNER at offset 176
	copy(quote[176:208], a.mrsigner)

	// ISV Product ID at offset 304
	quote[304] = 0
	quote[305] = 0

	// ISV SVN at offset 306
	quote[306] = 1
	quote[307] = 0

	// Copy report data at offset 368
	copy(quote[368:432], reportData)

	return quote, nil
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
