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
	"crypto/x509"
	"testing"
)

func TestNewGramineAttestor(t *testing.T) {
	attestor, err := NewGramineAttestor()
	if err != nil {
		t.Fatalf("Failed to create attestor: %v", err)
	}

	if attestor == nil {
		t.Fatal("Attestor is nil")
	}

	if attestor.privateKey == nil {
		t.Error("Private key is nil")
	}

	mrenclave := attestor.GetMREnclave()
	if len(mrenclave) != 32 {
		t.Errorf("MRENCLAVE length incorrect: expected 32, got %d", len(mrenclave))
	}
}

func TestGenerateQuote(t *testing.T) {
	attestor, err := NewGramineAttestor()
	if err != nil {
		t.Fatalf("Failed to create attestor: %v", err)
	}

	reportData := []byte("test report data")
	quote, err := attestor.GenerateQuote(reportData)
	if err != nil {
		t.Fatalf("GenerateQuote failed: %v", err)
	}

	if len(quote) < 432 {
		t.Errorf("Quote too short: expected at least 432 bytes, got %d", len(quote))
	}

	// Parse and verify quote structure
	parsedQuote, err := ParseQuote(quote)
	if err != nil {
		t.Fatalf("Failed to parse quote: %v", err)
	}

	// Verify report data was embedded correctly
	for i, b := range reportData {
		if parsedQuote.ReportData[i] != b {
			t.Errorf("Report data mismatch at byte %d: expected %x, got %x",
				i, b, parsedQuote.ReportData[i])
		}
	}

	// Verify MRENCLAVE
	mrenclave := attestor.GetMREnclave()
	for i, b := range mrenclave {
		if parsedQuote.MRENCLAVE[i] != b {
			t.Errorf("MRENCLAVE mismatch at byte %d: expected %x, got %x",
				i, b, parsedQuote.MRENCLAVE[i])
		}
	}
}

func TestGenerateQuoteTooLong(t *testing.T) {
	attestor, err := NewGramineAttestor()
	if err != nil {
		t.Fatalf("Failed to create attestor: %v", err)
	}

	// Try to generate quote with report data > 64 bytes
	reportData := make([]byte, 65)
	_, err = attestor.GenerateQuote(reportData)
	if err == nil {
		t.Error("Expected error for report data > 64 bytes, got nil")
	}
}

func TestGenerateCertificate(t *testing.T) {
	attestor, err := NewGramineAttestor()
	if err != nil {
		t.Fatalf("Failed to create attestor: %v", err)
	}

	cert, err := attestor.GenerateCertificate()
	if err != nil {
		t.Fatalf("GenerateCertificate failed: %v", err)
	}

	if len(cert.Certificate) == 0 {
		t.Fatal("No certificate generated")
	}

	// Parse the certificate
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("Failed to parse X.509 certificate: %v", err)
	}

	// Check that the certificate has an SGX quote extension
	foundQuote := false
	for _, ext := range x509Cert.Extensions {
		if ext.Id.Equal(SGXQuoteOID) {
			foundQuote = true
			if len(ext.Value) < 432 {
				t.Errorf("SGX quote in certificate too short: %d bytes", len(ext.Value))
			}
		}
	}

	if !foundQuote {
		t.Error("Certificate does not contain SGX quote extension")
	}
}

func TestGetMREnclave(t *testing.T) {
	attestor, err := NewGramineAttestor()
	if err != nil {
		t.Fatalf("Failed to create attestor: %v", err)
	}

	mrenclave := attestor.GetMREnclave()
	if len(mrenclave) != 32 {
		t.Errorf("MRENCLAVE length incorrect: expected 32, got %d", len(mrenclave))
	}

	// Verify it returns a copy, not the original
	mrenclave2 := attestor.GetMREnclave()
	if &mrenclave[0] == &mrenclave2[0] {
		t.Error("GetMREnclave returned a reference to internal data instead of a copy")
	}
}

func TestGetMRSigner(t *testing.T) {
	attestor, err := NewGramineAttestor()
	if err != nil {
		t.Fatalf("Failed to create attestor: %v", err)
	}

	mrsigner := attestor.GetMRSigner()
	if len(mrsigner) != 32 {
		t.Errorf("MRSIGNER length incorrect: expected 32, got %d", len(mrsigner))
	}

	// Verify it returns a copy
	mrsigner2 := attestor.GetMRSigner()
	if &mrsigner[0] == &mrsigner2[0] {
		t.Error("GetMRSigner returned a reference to internal data instead of a copy")
	}
}

func TestMockAttestor(t *testing.T) {
	attestor, err := NewGramineAttestor()
	if err != nil {
		t.Fatalf("Failed to create mock attestor: %v", err)
	}

	// Test quote generation
	reportData := []byte("mock test data")
	quote, err := attestor.GenerateQuote(reportData)
	if err != nil {
		t.Fatalf("Mock GenerateQuote failed: %v", err)
	}

	if len(quote) != 432 {
		t.Errorf("Mock quote length incorrect: expected 432, got %d", len(quote))
	}

	// Test certificate generation
	cert, err := attestor.GenerateCertificate()
	if err != nil {
		t.Fatalf("Mock GenerateCertificate failed: %v", err)
	}

	if len(cert.Certificate) == 0 {
		t.Error("Mock certificate not generated")
	}
}
