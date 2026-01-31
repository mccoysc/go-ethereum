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
)

// MockAttestor is a mock implementation of the Attestor interface for testing
// in non-SGX environments.
type MockAttestor struct {
	privateKey *ecdsa.PrivateKey
	mrenclave  []byte
	mrsigner   []byte
}

// NewMockAttestor creates a new mock attestor for testing.
func NewMockAttestor() (*MockAttestor, error) {
	// Use P-384 to match specification requirements
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Generate deterministic mock values
	mrenclave := make([]byte, 32)
	mrsigner := make([]byte, 32)
	for i := range mrenclave {
		mrenclave[i] = byte(i)
	}
	for i := range mrsigner {
		mrsigner[i] = byte(i + 32)
	}

	return &MockAttestor{
		privateKey: privateKey,
		mrenclave:  mrenclave,
		mrsigner:   mrsigner,
	}, nil
}

// GenerateQuote generates a mock SGX quote.
func (m *MockAttestor) GenerateQuote(reportData []byte) ([]byte, error) {
	if len(reportData) > 64 {
		return nil, fmt.Errorf("reportData too long: max 64 bytes, got %d", len(reportData))
	}

	// Create a minimal mock quote (432 bytes)
	quote := make([]byte, 432)

	// Version 3 (DCAP)
	quote[0] = 3
	quote[1] = 0

	// Sign type
	quote[2] = 2 // DCAP
	quote[3] = 0

	// Copy MRENCLAVE at offset 112
	copy(quote[112:144], m.mrenclave)

	// Copy MRSIGNER at offset 176
	copy(quote[176:208], m.mrsigner)

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

// GenerateCertificate generates a mock RA-TLS certificate.
func (m *MockAttestor) GenerateCertificate() (*tls.Certificate, error) {
	// Generate public key bytes
	pubKeyBytes := elliptic.Marshal(m.privateKey.Curve, m.privateKey.X, m.privateKey.Y)

	// Limit to 64 bytes for report data
	reportData := pubKeyBytes
	if len(reportData) > 64 {
		reportData = reportData[:64]
	}

	// Generate mock quote
	quote, err := m.GenerateQuote(reportData)
	if err != nil {
		return nil, fmt.Errorf("failed to generate quote: %w", err)
	}

	// Create certificate template
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			CommonName: "X-Chain-Mock-Node",
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
		&m.privateKey.PublicKey, m.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	return &tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  m.privateKey,
	}, nil
}

// GetMREnclave returns the mock MRENCLAVE.
func (m *MockAttestor) GetMREnclave() []byte {
	result := make([]byte, len(m.mrenclave))
	copy(result, m.mrenclave)
	return result
}

// GetMRSigner returns the mock MRSIGNER.
func (m *MockAttestor) GetMRSigner() []byte {
	result := make([]byte, len(m.mrsigner))
	copy(result, m.mrsigner)
	return result
}
