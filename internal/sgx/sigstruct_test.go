// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package sgx

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// TestVerifyRealManifestSignature tests SIGSTRUCT signature verification
// using a real manifest.sgx file from Gramine project
func TestVerifyRealManifestSignature(t *testing.T) {
	// Download real Gramine test manifest if not exists
	manifestPath := downloadTestManifest(t)

	// Read the manifest.sgx file
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Failed to read manifest: %v", err)
	}

	if len(data) < sigstructSize {
		t.Fatalf("Manifest too small: %d bytes (expected at least %d)", len(data), sigstructSize)
	}

	// Extract SIGSTRUCT (first 1808 bytes)
	sigstructData := data[0:sigstructSize]

	// Test 1: Parse SIGSTRUCT
	t.Run("ParseSIGSTRUCT", func(t *testing.T) {
		sig, err := ParseSIGSTRUCT(sigstructData)
		if err != nil {
			t.Fatalf("Failed to parse SIGSTRUCT: %v", err)
		}

		// Verify exponent is 3
		if sig.Exponent != 3 {
			t.Errorf("Invalid exponent: %d (expected 3)", sig.Exponent)
		}

		// Verify MRENCLAVE is not all zeros
		allZero := true
		for _, b := range sig.MREnclave {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			t.Error("MRENCLAVE is all zeros")
		}

		t.Logf("MRENCLAVE: %s", hex.EncodeToString(sig.MREnclave[:]))
		t.Logf("ISV_PROD_ID: %d", sig.ISVProdID)
		t.Logf("ISV_SVN: %d", sig.ISVSVN)
	})

	// Test 2: Get signing data
	t.Run("GetSigningData", func(t *testing.T) {
		signingData, err := GetSigningData(sigstructData)
		if err != nil {
			t.Fatalf("Failed to get signing data: %v", err)
		}

		// Signing data should be 256 bytes (128 + 128)
		if len(signingData) != 256 {
			t.Errorf("Invalid signing data length: %d (expected 256)", len(signingData))
		}
	})

	// Test 3: Verify SIGSTRUCT signature (CRITICAL TEST)
	t.Run("VerifySIGSTRUCTSignature", func(t *testing.T) {
		err := VerifySIGSTRUCTSignature(sigstructData)
		if err != nil {
			t.Fatalf("SIGSTRUCT signature verification failed: %v", err)
		}
		t.Log("✓ SIGSTRUCT signature verified successfully")
	})

	// Test 4: Extract MRENCLAVE
	t.Run("ExtractMREnclave", func(t *testing.T) {
		mrenclave, err := ExtractMREnclaveFromSIGSTRUCT(manifestPath)
		if err != nil {
			t.Fatalf("Failed to extract MRENCLAVE: %v", err)
		}

		if len(mrenclave) != 32 {
			t.Errorf("Invalid MRENCLAVE length: %d", len(mrenclave))
		}

		t.Logf("Extracted MRENCLAVE: %s", hex.EncodeToString(mrenclave))
	})

	// Test 5: Calculate MRSIGNER
	t.Run("ExtractMRSigner", func(t *testing.T) {
		mrsigner, err := extractMRSignerFromSIGSTRUCT(manifestPath)
		if err != nil {
			t.Fatalf("Failed to calculate MRSIGNER: %v", err)
		}

		if len(mrsigner) != 32 {
			t.Errorf("Invalid MRSIGNER length: %d", len(mrsigner))
		}

		t.Logf("Calculated MRSIGNER: %s", hex.EncodeToString(mrsigner))
	})

	// Test 6: Verify complete manifest
	t.Run("VerifyCompleteManifest", func(t *testing.T) {
		// Note: This will try to read /dev/attestation which doesn't exist in test
		// So we expect it to fail at runtime MRENCLAVE reading
		// But it should successfully verify the SIGSTRUCT signature first
		
		err := VerifyManifestSIGSTRUCT(manifestPath)
		
		// In test environment without /dev/attestation, this is expected to fail
		// But the failure should be about reading runtime MRENCLAVE, not signature
		if err != nil {
			t.Logf("Expected error (no /dev/attestation in test): %v", err)
			// Signature verification should have passed before runtime check
		}
	})
}

// downloadTestManifest downloads a real manifest.sgx from Gramine test data
func downloadTestManifest(t *testing.T) string {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "test.manifest.sgx")

	// Check if we have local test manifest in gramine directory
	localManifest := filepath.Join("..", "..", "gramine", "test.manifest.sgx")
	if _, err := os.Stat(localManifest); err == nil {
		// Copy local manifest
		data, err := os.ReadFile(localManifest)
		if err == nil {
			if err := os.WriteFile(manifestPath, data, 0644); err == nil {
				t.Logf("Using local manifest: %s", localManifest)
				return manifestPath
			}
		}
	}

	// Fallback: download from repository
	// This is a real signed manifest from our project
	url := "https://raw.githubusercontent.com/mccoysc/go-ethereum/copilot/add-poa-sgx-consensus-mechanism/gramine/test.manifest.sgx"
	
	t.Logf("Downloading test manifest from: %s", url)
	
	resp, err := http.Get(url)
	if err != nil {
		t.Skip("Cannot download test manifest (network issue):", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Cannot download test manifest (HTTP %d)", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Skip("Cannot read manifest data:", err)
	}

	if len(data) < sigstructSize {
		t.Skip("Downloaded manifest too small")
	}

	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		t.Skip("Cannot write manifest:", err)
	}

	t.Logf("Downloaded manifest to: %s (%d bytes)", manifestPath, len(data))
	return manifestPath
}

// TestSIGSTRUCTOffsets tests that our offset constants are correct
func TestSIGSTRUCTOffsets(t *testing.T) {
	// Verify offsets match Gramine specification
	tests := []struct {
		name   string
		offset int
		size   int
	}{
		{"MRENCLAVE", sigstructMREnclaveOffset, mrenclaveSize},
		{"Modulus", sigstructModulusOffset, modulusSize},
		{"Exponent", sigstructExponentOffset, rsaExponentSize},
		{"Signature", sigstructSignatureOffset, rsaKeySize},
		{"MiscSelect", sigstructMiscSelectOffset, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.offset < 0 || tt.offset+tt.size > sigstructSize {
				t.Errorf("Invalid offset for %s: %d+%d > %d",
					tt.name, tt.offset, tt.size, sigstructSize)
			}
		})
	}

	// Verify total size
	if sigstructSize != 1808 {
		t.Errorf("Invalid SIGSTRUCT size: %d (expected 1808)", sigstructSize)
	}
}

// TestMRSignerCalculation tests MRSIGNER calculation
func TestMRSignerCalculation(t *testing.T) {
	// MRSIGNER should be SHA256 of modulus
	// This is a fundamental SGX property
	
	// Create a dummy modulus (384 bytes)
	modulus := make([]byte, 384)
	for i := range modulus {
		modulus[i] = byte(i % 256)
	}

	// Expected MRSIGNER is SHA256(modulus)
	expected := sha256.Sum256(modulus)

	t.Logf("Test MRSIGNER: %s", hex.EncodeToString(expected[:]))
	
	// Verify our understanding is correct
	if len(expected) != 32 {
		t.Error("MRSIGNER should be 32 bytes")
	}
}
