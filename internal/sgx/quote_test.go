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
	"testing"
)

func TestParseQuote(t *testing.T) {
	// Create a valid quote
	quote := make([]byte, 432)

	// Set version
	binary.LittleEndian.PutUint16(quote[0:2], 3)

	// Set sign type
	binary.LittleEndian.PutUint16(quote[2:4], 2)

	// Set MRENCLAVE
	mrenclave := make([]byte, 32)
	for i := range mrenclave {
		mrenclave[i] = byte(i)
	}
	copy(quote[112:144], mrenclave)

	// Set MRSIGNER
	mrsigner := make([]byte, 32)
	for i := range mrsigner {
		mrsigner[i] = byte(i + 32)
	}
	copy(quote[176:208], mrsigner)

	// Set ISV Product ID
	binary.LittleEndian.PutUint16(quote[304:306], 100)

	// Set ISV SVN
	binary.LittleEndian.PutUint16(quote[306:308], 1)

	// Set report data
	reportData := []byte("test_report_data_for_quote_parsing")
	copy(quote[368:432], reportData)

	// Parse the quote
	parsed, err := ParseQuote(quote)
	if err != nil {
		t.Fatalf("Failed to parse quote: %v", err)
	}

	// Verify parsed fields
	if parsed.Version != 3 {
		t.Errorf("Version mismatch: expected 3, got %d", parsed.Version)
	}

	if parsed.SignType != 2 {
		t.Errorf("SignType mismatch: expected 2, got %d", parsed.SignType)
	}

	for i := range mrenclave {
		if parsed.MRENCLAVE[i] != mrenclave[i] {
			t.Errorf("MRENCLAVE mismatch at byte %d: expected %x, got %x",
				i, mrenclave[i], parsed.MRENCLAVE[i])
		}
	}

	for i := range mrsigner {
		if parsed.MRSIGNER[i] != mrsigner[i] {
			t.Errorf("MRSIGNER mismatch at byte %d: expected %x, got %x",
				i, mrsigner[i], parsed.MRSIGNER[i])
		}
	}

	if parsed.ISVProdID != 100 {
		t.Errorf("ISVProdID mismatch: expected 100, got %d", parsed.ISVProdID)
	}

	if parsed.ISVSVN != 1 {
		t.Errorf("ISVSVN mismatch: expected 1, got %d", parsed.ISVSVN)
	}

	for i, b := range reportData {
		if parsed.ReportData[i] != b {
			t.Errorf("ReportData mismatch at byte %d: expected %x, got %x",
				i, b, parsed.ReportData[i])
		}
	}

	if parsed.TCBStatus != TCBUpToDate {
		t.Errorf("TCBStatus mismatch: expected %d, got %d",
			TCBUpToDate, parsed.TCBStatus)
	}
}

func TestParseQuoteTooShort(t *testing.T) {
	// Create a quote that's too short
	quote := make([]byte, 100)

	_, err := ParseQuote(quote)
	if err == nil {
		t.Error("Expected error for quote too short, got nil")
	}
}

func TestParseQuoteWithSignature(t *testing.T) {
	// Create a quote with additional signature data
	quote := make([]byte, 500)

	// Set minimum required fields
	binary.LittleEndian.PutUint16(quote[0:2], 3)
	binary.LittleEndian.PutUint16(quote[2:4], 2)

	// Add some signature data
	signature := []byte("mock_signature_data_here")
	copy(quote[432:], signature)

	parsed, err := ParseQuote(quote)
	if err != nil {
		t.Fatalf("Failed to parse quote with signature: %v", err)
	}

	if len(parsed.Signature) != len(quote)-432 {
		t.Errorf("Signature length mismatch: expected %d (quote size - 432), got %d",
			len(quote)-432, len(parsed.Signature))
	}

	// Verify the signature contains our test data
	for i, b := range signature {
		if parsed.Signature[i] != b {
			t.Errorf("Signature mismatch at byte %d: expected %x, got %x",
				i, b, parsed.Signature[i])
		}
	}
}

func TestTCBStatusConstants(t *testing.T) {
	// Verify TCB status constants are defined correctly
	if TCBUpToDate != 0x00 {
		t.Errorf("TCBUpToDate should be 0x00, got 0x%02x", TCBUpToDate)
	}

	if TCBOutOfDate != 0x01 {
		t.Errorf("TCBOutOfDate should be 0x01, got 0x%02x", TCBOutOfDate)
	}

	if TCBRevoked != 0x02 {
		t.Errorf("TCBRevoked should be 0x02, got 0x%02x", TCBRevoked)
	}

	if TCBConfigurationNeeded != 0x03 {
		t.Errorf("TCBConfigurationNeeded should be 0x03, got 0x%02x",
			TCBConfigurationNeeded)
	}
}

func TestSGXQuoteOID(t *testing.T) {
	// Verify the OID is correct
	expectedOID := []int{1, 2, 840, 113741, 1, 13, 1}

	if len(SGXQuoteOID) != len(expectedOID) {
		t.Errorf("OID length mismatch: expected %d, got %d",
			len(expectedOID), len(SGXQuoteOID))
	}

	for i, val := range expectedOID {
		if SGXQuoteOID[i] != val {
			t.Errorf("OID mismatch at index %d: expected %d, got %d",
				i, val, SGXQuoteOID[i])
		}
	}
}

func BenchmarkParseQuote(b *testing.B) {
	quote := make([]byte, 432)
	binary.LittleEndian.PutUint16(quote[0:2], 3)
	binary.LittleEndian.PutUint16(quote[2:4], 2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseQuote(quote)
	}
}
