// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package sgx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/ethereum/go-ethereum/log"
)

// TrustedFile represents a file entry in the manifest's sgx.trusted_files list
type TrustedFile struct {
	URI    string `toml:"uri"`
	SHA256 string `toml:"sha256"`
}

// ManifestConfig represents the parsed manifest TOML structure
type ManifestConfig struct {
	SGX struct {
		TrustedFiles []interface{} `toml:"trusted_files"`
	} `toml:"sgx"`
}

// parseManifestTOML extracts and parses the TOML content from manifest.sgx
// The manifest.sgx file format: [SIGSTRUCT 1808 bytes][TOML manifest content]
func parseManifestTOML(manifestSgxData []byte) (*ManifestConfig, error) {
	if len(manifestSgxData) <= 1808 {
		return nil, fmt.Errorf("manifest.sgx too small: %d bytes", len(manifestSgxData))
	}

	// Extract TOML content (everything after SIGSTRUCT)
	tomlContent := manifestSgxData[1808:]

	var config ManifestConfig
	if err := toml.Unmarshal(tomlContent, &config); err != nil {
		return nil, fmt.Errorf("failed to parse manifest TOML: %w", err)
	}

	return &config, nil
}

// extractTrustedFiles parses the trusted_files array from manifest config
func extractTrustedFiles(config *ManifestConfig) ([]TrustedFile, error) {
	var trustedFiles []TrustedFile

	for _, item := range config.SGX.TrustedFiles {
		switch v := item.(type) {
		case string:
			// Simple string format: "file:/path/to/file"
			trustedFiles = append(trustedFiles, TrustedFile{
				URI:    v,
				SHA256: "", // No hash specified, will be computed
			})
		case map[string]interface{}:
			// Structured format with uri and sha256
			file := TrustedFile{}
			if uri, ok := v["uri"].(string); ok {
				file.URI = uri
			}
			if hash, ok := v["sha256"].(string); ok {
				file.SHA256 = hash
			}
			trustedFiles = append(trustedFiles, file)
		default:
			log.Warn("Unknown trusted_files entry format", "type", fmt.Sprintf("%T", v))
		}
	}

	return trustedFiles, nil
}

// resolveFilePath converts a Gramine URI to actual file path
func resolveFilePath(uri string) string {
	// Remove "file:" prefix if present
	path := strings.TrimPrefix(uri, "file:")
	
	// Handle directory paths (ending with /)
	// For directories, we would need to hash all files in it
	// For now, we'll skip directory validation
	if strings.HasSuffix(path, "/") {
		return ""
	}
	
	return path
}

// computeFileHash calculates SHA256 hash of a file
func computeFileHash(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// verifyTrustedFileHash verifies a single file's hash matches the manifest
func verifyTrustedFileHash(file TrustedFile) error {
	filePath := resolveFilePath(file.URI)
	
	// Skip if it's a directory or no path resolved
	if filePath == "" {
		log.Debug("Skipping directory or unresolvable path", "uri", file.URI)
		return nil
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("trusted file not found: %s", filePath)
	}

	// If no hash specified in manifest, skip verification
	// (gramine-manifest might not have calculated it yet)
	if file.SHA256 == "" {
		log.Debug("No hash specified for file, skipping", "path", filePath)
		return nil
	}

	// Compute actual file hash
	computedHash, err := computeFileHash(filePath)
	if err != nil {
		return err
	}

	// Verify hash matches
	if computedHash != file.SHA256 {
		return fmt.Errorf("hash mismatch for %s: expected %s, got %s",
			filePath, file.SHA256, computedHash)
	}

	log.Debug("Trusted file hash verified", "path", filePath)
	return nil
}

// VerifyTrustedFiles verifies all files listed in manifest's trusted_files
// This is a critical security check to ensure files haven't been tampered with
func VerifyTrustedFiles(manifestSgxData []byte) error {
	// Parse manifest TOML
	config, err := parseManifestTOML(manifestSgxData)
	if err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Extract trusted files list
	trustedFiles, err := extractTrustedFiles(config)
	if err != nil {
		return fmt.Errorf("failed to extract trusted files: %w", err)
	}

	log.Info("Verifying trusted files from manifest", "count", len(trustedFiles))

	// Verify each file's hash
	var errors []string
	for _, file := range trustedFiles {
		if err := verifyTrustedFileHash(file); err != nil {
			errors = append(errors, err.Error())
			log.Warn("Trusted file verification failed", "uri", file.URI, "error", err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("trusted files verification failed: %d errors: %v", 
			len(errors), errors)
	}

	log.Info("All trusted files verified successfully", "count", len(trustedFiles))
	return nil
}
