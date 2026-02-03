// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package sgx

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestParseManifestTOML(t *testing.T) {
	// Create a minimal manifest.sgx structure
	// First 1808 bytes: SIGSTRUCT (can be zeros for this test)
	sigstruct := make([]byte, 1808)
	
	// After SIGSTRUCT: TOML content
	tomlContent := `
[sgx]
trusted_files = [
  "file:/lib/test.so",
  { uri = "file:/bin/geth", sha256 = "abc123" },
]
`
	
	manifestSgx := append(sigstruct, []byte(tomlContent)...)
	
	// Parse
	config, err := parseManifestTOML(manifestSgx)
	if err != nil {
		t.Fatalf("Failed to parse manifest: %v", err)
	}
	
	if len(config.SGX.TrustedFiles) != 2 {
		t.Errorf("Expected 2 trusted files, got %d", len(config.SGX.TrustedFiles))
	}
}

func TestExtractTrustedFiles(t *testing.T) {
	config := &ManifestConfig{}
	config.SGX.TrustedFiles = []interface{}{
		"file:/lib/test1.so",
		map[string]interface{}{
			"uri":    "file:/bin/test2",
			"sha256": "deadbeef",
		},
	}
	
	files, err := extractTrustedFiles(config)
	if err != nil {
		t.Fatalf("Failed to extract trusted files: %v", err)
	}
	
	if len(files) != 2 {
		t.Fatalf("Expected 2 files, got %d", len(files))
	}
	
	// Check first file (string format)
	if files[0].URI != "file:/lib/test1.so" {
		t.Errorf("Unexpected URI: %s", files[0].URI)
	}
	if files[0].SHA256 != "" {
		t.Errorf("Expected empty hash for string format, got: %s", files[0].SHA256)
	}
	
	// Check second file (map format)
	if files[1].URI != "file:/bin/test2" {
		t.Errorf("Unexpected URI: %s", files[1].URI)
	}
	if files[1].SHA256 != "deadbeef" {
		t.Errorf("Unexpected hash: %s", files[1].SHA256)
	}
}

func TestResolveFilePath(t *testing.T) {
	tests := []struct {
		uri      string
		expected string
	}{
		{"file:/bin/test", "/bin/test"},
		{"/lib/test.so", "/lib/test.so"},
		{"file:/usr/lib/", ""}, // Directory, should return empty
	}
	
	for _, test := range tests {
		result := resolveFilePath(test.uri)
		if result != test.expected {
			t.Errorf("resolveFilePath(%s) = %s, want %s", 
				test.uri, result, test.expected)
		}
	}
}

func TestComputeFileHash(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("Hello, Gramine!")
	
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Compute hash
	hash, err := computeFileHash(testFile)
	if err != nil {
		t.Fatalf("Failed to compute hash: %v", err)
	}
	
	// Verify hash is correct
	expected := sha256.Sum256(testContent)
	expectedHash := hex.EncodeToString(expected[:])
	
	if hash != expectedHash {
		t.Errorf("Hash mismatch: got %s, want %s", hash, expectedHash)
	}
}

func TestVerifyTrustedFileHash(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("Test content for hash verification")
	
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Calculate expected hash
	hashBytes := sha256.Sum256(testContent)
	expectedHash := hex.EncodeToString(hashBytes[:])
	
	// Test 1: Correct hash
	file := TrustedFile{
		URI:    "file:" + testFile,
		SHA256: expectedHash,
	}
	
	if err := verifyTrustedFileHash(file); err != nil {
		t.Errorf("Verification should pass with correct hash: %v", err)
	}
	
	// Test 2: Incorrect hash
	file.SHA256 = "0000000000000000"
	if err := verifyTrustedFileHash(file); err == nil {
		t.Error("Verification should fail with incorrect hash")
	}
	
	// Test 3: File not found
	file.URI = "file:/nonexistent/file"
	if err := verifyTrustedFileHash(file); err == nil {
		t.Error("Verification should fail for nonexistent file")
	}
}

func TestVerifyTrustedFilesIntegration(t *testing.T) {
	// This test verifies the complete flow with a real temporary file
	
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "libtest.so")
	testContent := []byte("Mock library content")
	
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Calculate hash
	hashBytes := sha256.Sum256(testContent)
	fileHash := hex.EncodeToString(hashBytes[:])
	
	// Create manifest.sgx with this file
	sigstruct := make([]byte, 1808)
	tomlContent := `
[sgx]
trusted_files = [
  { uri = "file:` + testFile + `", sha256 = "` + fileHash + `" },
]
`
	manifestSgx := append(sigstruct, []byte(tomlContent)...)
	
	// Verify - should pass
	if err := VerifyTrustedFiles(manifestSgx); err != nil {
		t.Errorf("Verification failed unexpectedly: %v", err)
	}
	
	// Modify file content to cause hash mismatch
	modifiedContent := []byte("Modified content")
	if err := os.WriteFile(testFile, modifiedContent, 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}
	
	// Verify - should fail
	if err := VerifyTrustedFiles(manifestSgx); err == nil {
		t.Error("Verification should fail after file modification")
	}
}
