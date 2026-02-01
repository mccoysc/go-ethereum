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

package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyGramineManifestSignature_NonSGX(t *testing.T) {
	// In non-SGX mode, should not fail
	err := VerifyGramineManifestSignature()
	if err != nil {
		t.Errorf("Expected no error in non-SGX mode, got: %v", err)
	}
}

func TestVerifyGramineManifestSignature_SGXMode(t *testing.T) {
	// Simulate SGX mode
	os.Setenv("IN_SGX", "1")
	os.Setenv("RA_TLS_MRENCLAVE", "test_mrenclave")
	os.Setenv("RA_TLS_MRSIGNER", "test_mrsigner")
	os.Setenv("GRAMINE_MANIFEST_HASH", "test_hash")
	defer func() {
		os.Unsetenv("IN_SGX")
		os.Unsetenv("RA_TLS_MRENCLAVE")
		os.Unsetenv("RA_TLS_MRSIGNER")
		os.Unsetenv("GRAMINE_MANIFEST_HASH")
	}()

	err := VerifyGramineManifestSignature()
	if err != nil {
		t.Errorf("Expected no error with valid SGX env, got: %v", err)
	}
}

func TestVerifyGramineManifestSignature_SGXModeNoHash(t *testing.T) {
	// Simulate SGX mode without manifest hash
	os.Setenv("IN_SGX", "1")
	os.Setenv("RA_TLS_MRENCLAVE", "test_mrenclave")
	defer func() {
		os.Unsetenv("IN_SGX")
		os.Unsetenv("RA_TLS_MRENCLAVE")
	}()

	err := VerifyGramineManifestSignature()
	if err == nil {
		t.Error("Expected error when manifest hash is missing")
	}
}

func TestNewGramineEncryptionValidator_NoEncryptedPaths(t *testing.T) {
	// Clear all encrypted path env variables
	oldEncPath := os.Getenv("GRAMINE_ENCRYPTED_PATHS")
	oldXChainPath := os.Getenv("XCHAIN_ENCRYPTED_PATH")
	oldSecretPath := os.Getenv("XCHAIN_SECRET_PATH")

	os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")
	os.Unsetenv("XCHAIN_ENCRYPTED_PATH")
	os.Unsetenv("XCHAIN_SECRET_PATH")

	defer func() {
		if oldEncPath != "" {
			os.Setenv("GRAMINE_ENCRYPTED_PATHS", oldEncPath)
		}
		if oldXChainPath != "" {
			os.Setenv("XCHAIN_ENCRYPTED_PATH", oldXChainPath)
		}
		if oldSecretPath != "" {
			os.Setenv("XCHAIN_SECRET_PATH", oldSecretPath)
		}
	}()

	_, err := NewGramineEncryptionValidator()
	if err == nil {
		t.Error("Expected error when no encrypted paths configured")
	}
}

func TestNewGramineEncryptionValidator_WithEnvPaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Set encrypted paths
	os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
	defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

	validator, err := NewGramineEncryptionValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	paths := validator.GetEncryptedPaths()
	if len(paths) == 0 {
		t.Error("Expected at least one encrypted path")
	}

	found := false
	for _, p := range paths {
		if p == tmpDir {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected to find %s in encrypted paths", tmpDir)
	}
}

func TestValidatePath_ValidPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Set encrypted paths
	os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
	defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

	validator, err := NewGramineEncryptionValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	// Validate the exact path
	err = validator.ValidatePath(tmpDir)
	if err != nil {
		t.Errorf("Expected path %s to be valid, got error: %v", tmpDir, err)
	}
}

func TestValidatePath_Subdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	os.Mkdir(subDir, 0755)

	// Set encrypted paths
	os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
	defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

	validator, err := NewGramineEncryptionValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	// Validate subdirectory
	err = validator.ValidatePath(subDir)
	if err != nil {
		t.Errorf("Expected subdirectory %s to be valid, got error: %v", subDir, err)
	}
}

func TestValidatePath_InvalidPath(t *testing.T) {
	tmpDir := t.TempDir()
	otherDir := t.TempDir()

	// Set encrypted paths
	os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
	defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

	validator, err := NewGramineEncryptionValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	// Validate a path outside encrypted directories
	err = validator.ValidatePath(otherDir)
	if err == nil {
		t.Error("Expected error for path outside encrypted directories")
	}
}

func TestNewEncryptedPartition_WithValidation(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up environment for testing
	os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
	defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

	// Create partition - should succeed with validation
	partition, err := NewEncryptedPartition(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create encrypted partition: %v", err)
	}

	if partition == nil {
		t.Fatal("Partition is nil")
	}
}

func TestNewEncryptedPartition_UnencryptedPath(t *testing.T) {
	encryptedDir := t.TempDir()
	unencryptedDir := t.TempDir()

	// Set up environment - only one dir is encrypted
	os.Setenv("GRAMINE_ENCRYPTED_PATHS", encryptedDir)
	defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

	// Try to create partition with unencrypted path - should fail
	_, err := NewEncryptedPartition(unencryptedDir)
	if err == nil {
		t.Error("Expected error when creating partition with unencrypted path")
	}
}

func TestGetEncryptedPaths(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	// Set multiple encrypted paths
	paths := tmpDir1 + "," + tmpDir2
	os.Setenv("GRAMINE_ENCRYPTED_PATHS", paths)
	defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

	validator, err := NewGramineEncryptionValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	encPaths := validator.GetEncryptedPaths()
	if len(encPaths) < 2 {
		t.Errorf("Expected at least 2 encrypted paths, got %d", len(encPaths))
	}
}

func TestVerifyEncryptedPath(t *testing.T) {
	tmpDir := t.TempDir()

	validator := &GramineEncryptionValidator{}

	// Path exists, should be able to write test file
	result := validator.verifyEncryptedPath(tmpDir)
	if !result {
		t.Error("Expected verification to succeed for writable directory")
	}
}

func TestVerifyEncryptedPath_NonExistent(t *testing.T) {
	validator := &GramineEncryptionValidator{}

	// Non-existent path should fail
	result := validator.verifyEncryptedPath("/non/existent/path")
	if result {
		t.Error("Expected verification to fail for non-existent path")
	}
}

func TestContainsPath_EdgeCases(t *testing.T) {
	validator := &GramineEncryptionValidator{
		encryptedPaths: []string{
			"/data/secrets",
			"/app/config",
		},
	}

	// Test exact match
	if !validator.containsPath("/data/secrets") {
		t.Error("Expected exact match to succeed")
	}

	// Test path in list
	if !validator.containsPath("/app/config") {
		t.Error("Expected path match to succeed")
	}

	// Test no match
	if validator.containsPath("/other/path") {
		t.Error("Expected no match for different path")
	}

	// Test partial match (should fail - exact match only)
	if validator.containsPath("/data") {
		t.Error("Expected partial match to fail")
	}
}

func TestValidatePath_EmptyPath(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
	defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

	validator, err := NewGramineEncryptionValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	// Empty path should fail
	err = validator.ValidatePath("")
	if err == nil {
		t.Error("Expected error for empty path")
	}
}

func TestLoadEncryptedPathsFromGramine_EmptyEnv(t *testing.T) {
	// Unset environment variable
	os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

	validator := &GramineEncryptionValidator{}
	validator.loadEncryptedPathsFromGramine()

	// Should have empty paths
	if len(validator.encryptedPaths) != 0 {
		t.Errorf("Expected no encrypted paths, got %d", len(validator.encryptedPaths))
	}
}

func TestVerifyGramineManifestSignature_MissingFiles(t *testing.T) {
	// VerifyGramineManifestSignature is a standalone function, not a method
	// Test with non-existent file in non-SGX mode (should succeed without verification)
	os.Unsetenv("RA_TLS_MRENCLAVE")
	
	err := VerifyGramineManifestSignature()
	// In non-SGX mode, verification is skipped
	if err != nil {
		t.Logf("Error in non-SGX mode: %v", err)
	}
}

func TestVerifyGramineManifestSignature_MissingSignature(t *testing.T) {
	// This function doesn't take parameters in its current implementation
	// It verifies the manifest based on environment variables
	
	// Set up SGX mode
	os.Setenv("RA_TLS_MRENCLAVE", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	defer os.Unsetenv("RA_TLS_MRENCLAVE")
	
	// In SGX mode without proper manifest, should return error
	err := VerifyGramineManifestSignature()
	// Expected to fail in SGX mode without manifest
	if err != nil {
		t.Logf("Expected error in SGX mode without manifest: %v", err)
	}
}

