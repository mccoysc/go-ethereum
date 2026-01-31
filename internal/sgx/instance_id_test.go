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
	"testing"
)

func TestExtractInstanceID(t *testing.T) {
	// Create a mock attestor to generate a quote
	attestor, err := NewMockAttestor()
	if err != nil {
		t.Fatalf("Failed to create attestor: %v", err)
	}

	quote, err := attestor.GenerateQuote([]byte("test data"))
	if err != nil {
		t.Fatalf("Failed to generate quote: %v", err)
	}

	// Extract instance ID
	instanceID, err := ExtractInstanceID(quote)
	if err != nil {
		t.Fatalf("Failed to extract instance ID: %v", err)
	}

	if instanceID == nil {
		t.Fatal("Instance ID is nil")
	}

	if len(instanceID.CPUInstanceID) == 0 {
		t.Error("CPU Instance ID is empty")
	}

	// Verify instance ID is consistent for same quote
	instanceID2, err := ExtractInstanceID(quote)
	if err != nil {
		t.Fatalf("Failed to extract instance ID second time: %v", err)
	}

	if !instanceID.Equal(instanceID2) {
		t.Error("Instance IDs should be equal for same quote")
	}
}

func TestExtractInstanceIDTooShort(t *testing.T) {
	// Try with a quote that's too short
	shortQuote := make([]byte, 100)

	_, err := ExtractInstanceID(shortQuote)
	if err == nil {
		t.Error("Expected error for short quote, got nil")
	}
}

func TestInstanceIDString(t *testing.T) {
	instanceID := &InstanceID{
		CPUInstanceID: []byte{0x01, 0x02, 0x03, 0x04},
		QuoteType:     2,
	}

	str := instanceID.String()
	expected := "01020304"

	if str != expected {
		t.Errorf("String() = %s, want %s", str, expected)
	}
}

func TestInstanceIDEqual(t *testing.T) {
	id1 := &InstanceID{
		CPUInstanceID: []byte{0x01, 0x02, 0x03},
		QuoteType:     2,
	}

	id2 := &InstanceID{
		CPUInstanceID: []byte{0x01, 0x02, 0x03},
		QuoteType:     2,
	}

	id3 := &InstanceID{
		CPUInstanceID: []byte{0x04, 0x05, 0x06},
		QuoteType:     2,
	}

	if !id1.Equal(id2) {
		t.Error("Equal instance IDs should be equal")
	}

	if id1.Equal(id3) {
		t.Error("Different instance IDs should not be equal")
	}

	// Test nil cases
	var nilID *InstanceID
	if !nilID.Equal(nil) {
		t.Error("nil should equal nil")
	}

	if id1.Equal(nil) {
		t.Error("non-nil should not equal nil")
	}
}

func TestExtractDCAPInstanceID(t *testing.T) {
	// Create a DCAP quote (quote type 2)
	quote := make([]byte, 500)
	quote[2] = 2 // DCAP signature type

	// Fill in some platform-specific data
	for i := 64; i < 80; i++ {
		quote[i] = byte(i)
	}

	instanceID, err := ExtractInstanceID(quote)
	if err != nil {
		t.Fatalf("Failed to extract DCAP instance ID: %v", err)
	}

	if instanceID.QuoteType != 2 {
		t.Errorf("Quote type = %d, want 2", instanceID.QuoteType)
	}

	if len(instanceID.CPUInstanceID) != 32 {
		t.Errorf("CPU Instance ID length = %d, want 32", len(instanceID.CPUInstanceID))
	}
}

func TestExtractEPIDInstanceID(t *testing.T) {
	// Create an EPID quote (quote type 0 or 1)
	quote := make([]byte, 500)
	quote[2] = 0 // EPID unlinkable signature type

	// Fill in some signature data
	for i := 432; i < 464; i++ {
		quote[i] = byte(i - 432)
	}

	instanceID, err := ExtractInstanceID(quote)
	if err != nil {
		t.Fatalf("Failed to extract EPID instance ID: %v", err)
	}

	if instanceID.QuoteType != 0 {
		t.Errorf("Quote type = %d, want 0", instanceID.QuoteType)
	}

	if len(instanceID.CPUInstanceID) != 32 {
		t.Errorf("CPU Instance ID length = %d, want 32", len(instanceID.CPUInstanceID))
	}
}
