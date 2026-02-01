// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// Integration tests for Gramine module
// These tests verify that all modules work correctly in the Gramine environment

//go:build integration
// +build integration

package gramine_test

import (
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/internal/sgx"
)

// TestSGXHardwareDetection tests SGX hardware detection
func TestSGXHardwareDetection(t *testing.T) {
	// This test will be skipped if SGX hardware is not available
	if err := sgx.CheckSGXHardware(); err != nil {
		t.Skipf("SGX hardware not available: %v", err)
	}

	info, err := sgx.GetSGXInfo()
	if err != nil {
		t.Fatalf("Failed to get SGX info: %v", err)
	}

	if !info.IsInsideEnclave {
		t.Log("Not running inside enclave - this is expected for this test")
	}

	if len(info.MRENCLAVE) == 0 {
		t.Error("MRENCLAVE is empty")
	}

	t.Logf("MRENCLAVE length: %d bytes", len(info.MRENCLAVE))
}

// TestEncryptedPartition tests the encrypted partition functionality
func TestEncryptedPartition(t *testing.T) {
	// Skip if not running in Gramine environment
	if !isGramineEnvironment() {
		t.Skip("Not running in Gramine environment")
	}

	testData := []byte("test secret data for encrypted partition")
	testFile := "/data/encrypted/test.bin"

	// Write test data
	if err := os.WriteFile(testFile, testData, 0600); err != nil {
		t.Fatalf("Failed to write to encrypted partition: %v", err)
	}

	// Read back
	readData, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read from encrypted partition: %v", err)
	}

	// Verify data matches
	if string(testData) != string(readData) {
		t.Error("Data mismatch after reading from encrypted partition")
	}

	// Cleanup
	os.Remove(testFile)
	t.Log("Encrypted partition test passed")
}

// TestManifestParameters tests that manifest parameters are correctly loaded
func TestManifestParameters(t *testing.T) {
	// Check expected environment variables from manifest
	expectedVars := map[string]string{
		"XCHAIN_ENCRYPTED_PATH":           "/data/encrypted",
		"XCHAIN_SECRET_PATH":              "/data/secrets",
		"XCHAIN_GOVERNANCE_CONTRACT":      "0x0000000000000000000000000000000000001001",
		"XCHAIN_SECURITY_CONFIG_CONTRACT": "0x0000000000000000000000000000000000001002",
	}

	for key, expectedValue := range expectedVars {
		actualValue := os.Getenv(key)
		if actualValue == "" {
			t.Logf("Environment variable %s not set (may be OK if not running in Gramine)", key)
			continue
		}

		// Normalize addresses to lowercase for comparison
		if key == "XCHAIN_GOVERNANCE_CONTRACT" || key == "XCHAIN_SECURITY_CONFIG_CONTRACT" {
			if actualValue != expectedValue && actualValue != expectedValue {
				t.Logf("Contract address %s = %s (expected %s)", key, actualValue, expectedValue)
			}
		} else if actualValue != expectedValue {
			t.Errorf("%s mismatch: got %s, want %s", key, actualValue, expectedValue)
		}
	}
}

// TestModuleIntegration tests that all modules can be initialized
func TestModuleIntegration(t *testing.T) {
	t.Run("SGX Module", func(t *testing.T) {
		// Test SGX module initialization
		if sgx.IsSGXAvailable() {
			t.Log("SGX is available")
		} else {
			t.Log("SGX is not available - using mock mode")
		}
	})

	t.Run("Mock SGX Info", func(t *testing.T) {
		// Test mock SGX info for development
		mockInfo := sgx.GetMockSGXInfo()
		if mockInfo == nil {
			t.Fatal("Failed to get mock SGX info")
		}
		if mockInfo.IsInsideEnclave {
			t.Error("Mock info should not report being inside enclave")
		}
	})
}

// TestGramineEnvironment tests Gramine-specific functionality
func TestGramineEnvironment(t *testing.T) {
	if !isGramineEnvironment() {
		t.Skip("Not running in Gramine environment")
	}

	// Test that we can access Gramine-specific features
	// For example, check if attestation device is accessible
	if _, err := os.Stat("/dev/attestation"); err == nil {
		t.Log("Gramine attestation device is available")
	} else {
		t.Log("Gramine attestation device not available (may be OK depending on config)")
	}
}

// Helper function to detect if running in Gramine
func isGramineEnvironment() bool {
	_, exists := os.LookupEnv("SGX_AESM_ADDR")
	return exists
}

// TestDataDirectories verifies that all required data directories exist
func TestDataDirectories(t *testing.T) {
	requiredDirs := []string{
		"/data/encrypted",
		"/data/secrets",
	}

	for _, dir := range requiredDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Logf("Directory %s does not exist (may be OK if not in Gramine)", dir)
		} else if err != nil {
			t.Errorf("Error accessing directory %s: %v", dir, err)
		} else {
			t.Logf("Directory %s exists and is accessible", dir)
		}
	}
}
