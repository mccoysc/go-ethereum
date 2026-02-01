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
	"testing"
)

func TestNewParameterValidator(t *testing.T) {
	validator := NewParameterValidator()
	if validator == nil {
		t.Fatal("Validator is nil")
	}
}

func TestValidateManifestParams(t *testing.T) {
	validator := NewParameterValidator()

	// Test with all required parameters
	manifestParams := map[string]string{
		"XCHAIN_ENCRYPTED_PATH":           "/data/encrypted",
		"XCHAIN_SECRET_PATH":              "/data/secrets",
		"XCHAIN_GOVERNANCE_CONTRACT":      "0x1234567890abcdef1234567890abcdef12345678",
		"XCHAIN_SECURITY_CONFIG_CONTRACT": "0xabcdef1234567890abcdef1234567890abcdef12",
	}

	err := validator.ValidateManifestParams(manifestParams)
	if err != nil {
		t.Fatalf("Failed to validate manifest params: %v", err)
	}
}

func TestValidateManifestParams_MissingRequired(t *testing.T) {
	validator := NewParameterValidator()

	// Test with missing required parameter
	manifestParams := map[string]string{
		"XCHAIN_ENCRYPTED_PATH": "/data/encrypted",
		// Missing other required parameters
	}

	err := validator.ValidateManifestParams(manifestParams)
	if err == nil {
		t.Fatal("Expected error for missing required parameters")
	}
}

func TestValidateChainParams(t *testing.T) {
	validator := NewParameterValidator()

	chainParams := map[string]interface{}{
		"allowed_mrenclaves":      []string{"abc123", "def456"},
		"key_migration_threshold": 3,
		"admission_strict":        true,
	}

	err := validator.ValidateChainParams(chainParams)
	if err != nil {
		t.Fatalf("Failed to validate chain params: %v", err)
	}
}

func TestMergeAndValidate_Priority(t *testing.T) {
	validator := NewParameterValidator()

	// Test priority: Manifest > Chain > CLI
	manifestParams := map[string]string{
		"XCHAIN_ENCRYPTED_PATH":           "/manifest/encrypted",
		"XCHAIN_SECRET_PATH":              "/manifest/secrets",
		"XCHAIN_GOVERNANCE_CONTRACT":      "0x1111111111111111111111111111111111111111",
		"XCHAIN_SECURITY_CONFIG_CONTRACT": "0x2222222222222222222222222222222222222222",
	}

	chainParams := map[string]interface{}{
		"XCHAIN_ENCRYPTED_PATH": "/chain/encrypted", // Should be overridden by manifest
		"chain_only_param":      "chain_value",
	}

	cmdLineParams := map[string]interface{}{
		"xchain.encrypted-path": "/cli/encrypted", // Should be overridden by manifest
		"cli_only_param":        "cli_value",
	}

	merged, err := validator.MergeAndValidate(manifestParams, chainParams, cmdLineParams)
	if err != nil {
		t.Fatalf("Failed to merge and validate: %v", err)
	}

	// Verify manifest params take priority
	if merged["XCHAIN_ENCRYPTED_PATH"] != "/manifest/encrypted" {
		t.Errorf("Expected manifest path, got %v", merged["XCHAIN_ENCRYPTED_PATH"])
	}

	// Verify chain-only params are included
	if merged["chain_only_param"] != "chain_value" {
		t.Error("Chain-only param should be included")
	}

	// Verify CLI-only params are included
	if merged["cli_only_param"] != "cli_value" {
		t.Error("CLI-only param should be included")
	}
}

func TestMergeAndValidate_SecurityParamPriority(t *testing.T) {
	validator := NewParameterValidator()

	// Manifest sets encrypted path
	manifestParams := map[string]string{
		"XCHAIN_ENCRYPTED_PATH":           "/manifest/encrypted",
		"XCHAIN_SECRET_PATH":              "/manifest/secrets",
		"XCHAIN_GOVERNANCE_CONTRACT":      "0x1111111111111111111111111111111111111111",
		"XCHAIN_SECURITY_CONFIG_CONTRACT": "0x2222222222222222222222222222222222222222",
	}

	chainParams := map[string]interface{}{}

	// CLI tries to override security param
	cmdLineParams := map[string]interface{}{
		"xchain.encrypted-path": "/cli/encrypted",
	}

	merged, err := validator.MergeAndValidate(manifestParams, chainParams, cmdLineParams)
	if err != nil {
		t.Fatalf("Failed to merge: %v", err)
	}

	// Manifest should win for security params
	if merged["XCHAIN_ENCRYPTED_PATH"] != "/manifest/encrypted" {
		t.Error("Security param should not be overridden by CLI")
	}
}

func TestMergeAndValidate_ChainOverridesCLI(t *testing.T) {
	validator := NewParameterValidator()

	manifestParams := map[string]string{
		"XCHAIN_ENCRYPTED_PATH":           "/manifest/encrypted",
		"XCHAIN_SECRET_PATH":              "/manifest/secrets",
		"XCHAIN_GOVERNANCE_CONTRACT":      "0x1111111111111111111111111111111111111111",
		"XCHAIN_SECURITY_CONFIG_CONTRACT": "0x2222222222222222222222222222222222222222",
	}

	// Chain sets a security param
	chainParams := map[string]interface{}{
		"secret_path": "/chain/secrets",
	}

	// CLI tries to set the same param
	cmdLineParams := map[string]interface{}{
		"xchain.secret-path": "/cli/secrets",
	}

	merged, err := validator.MergeAndValidate(manifestParams, chainParams, cmdLineParams)
	if err != nil {
		t.Fatalf("Failed to merge: %v", err)
	}

	// Manifest should still win
	if merged["XCHAIN_SECRET_PATH"] != "/manifest/secrets" {
		t.Error("Manifest should take priority over chain and CLI")
	}
}

func TestMergeAndValidate_MissingRequired(t *testing.T) {
	validator := NewParameterValidator()

	// Missing required parameters
	manifestParams := map[string]string{
		"XCHAIN_ENCRYPTED_PATH": "/manifest/encrypted",
		// Missing other required params
	}

	chainParams := map[string]interface{}{}
	cmdLineParams := map[string]interface{}{}

	_, err := validator.MergeAndValidate(manifestParams, chainParams, cmdLineParams)
	if err == nil {
		t.Fatal("Expected error for missing required parameters")
	}
}

func TestCheckSecurityParams(t *testing.T) {
	validator := NewParameterValidator()

	manifestParams := map[string]string{
		"XCHAIN_ENCRYPTED_PATH":           "/manifest/encrypted",
		"XCHAIN_SECRET_PATH":              "/manifest/secrets",
		"XCHAIN_GOVERNANCE_CONTRACT":      "0x1111111111111111111111111111111111111111",
		"XCHAIN_SECURITY_CONFIG_CONTRACT": "0x2222222222222222222222222222222222222222",
	}

	// Merge first
	_, err := validator.MergeAndValidate(manifestParams, map[string]interface{}{}, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to merge: %v", err)
	}

	// Check security params
	err = validator.CheckSecurityParams()
	if err != nil {
		t.Fatalf("Security params check failed: %v", err)
	}
}

func TestCheckSecurityParams_Missing(t *testing.T) {
	validator := NewParameterValidator()

	// Don't set any params
	manifestParams := map[string]string{}

	// Merge (will fail due to missing required params)
	_, err := validator.MergeAndValidate(manifestParams, map[string]interface{}{}, map[string]interface{}{})
	if err == nil {
		// If merge somehow passed, check should fail
		err = validator.CheckSecurityParams()
		if err == nil {
			t.Fatal("Expected error for missing security params")
		}
	}
}

func TestGetMergedParams(t *testing.T) {
	validator := NewParameterValidator()

	manifestParams := map[string]string{
		"XCHAIN_ENCRYPTED_PATH":           "/manifest/encrypted",
		"XCHAIN_SECRET_PATH":              "/manifest/secrets",
		"XCHAIN_GOVERNANCE_CONTRACT":      "0x1111111111111111111111111111111111111111",
		"XCHAIN_SECURITY_CONFIG_CONTRACT": "0x2222222222222222222222222222222222222222",
	}

	validator.MergeAndValidate(manifestParams, map[string]interface{}{}, map[string]interface{}{})

	merged := validator.GetMergedParams()
	if len(merged) == 0 {
		t.Error("Expected merged params to be non-empty")
	}
}

func TestParamCategory(t *testing.T) {
	// Test that security params are properly categorized
	securityParamCount := 0
	for _, param := range SecurityParams {
		if param.Category == ParamCategorySecurity {
			securityParamCount++
		}
	}

	if securityParamCount != len(SecurityParams) {
		t.Errorf("Expected all SecurityParams to be security category, got %d/%d", securityParamCount, len(SecurityParams))
	}
}

func TestConcurrentAccess(t *testing.T) {
	validator := NewParameterValidator()

	manifestParams := map[string]string{
		"XCHAIN_ENCRYPTED_PATH":           "/manifest/encrypted",
		"XCHAIN_SECRET_PATH":              "/manifest/secrets",
		"XCHAIN_GOVERNANCE_CONTRACT":      "0x1111111111111111111111111111111111111111",
		"XCHAIN_SECURITY_CONFIG_CONTRACT": "0x2222222222222222222222222222222222222222",
	}

	// Initialize
	validator.MergeAndValidate(manifestParams, map[string]interface{}{}, map[string]interface{}{})

	// Concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = validator.GetMergedParams()
			_ = validator.CheckSecurityParams()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestMergeAndValidate_AllSources(t *testing.T) {
	validator := NewParameterValidator()

	// Test with all three sources (include all required params)
	manifestParams := map[string]string{
		"manifest_param":       "manifest_value",
		"shared_param":         "from_manifest",
		"encrypted_path":       "/data/secrets",
		"secret_path":          "/secrets",
		"governance_contract":  "0x1234567890abcdef1234567890abcdef12345678",
		"security_config_contract": "0xfedcba9876543210fedcba9876543210fedcba98",
	}
	chainParams := map[string]interface{}{
		"chain_param":  "chain_value",
		"shared_param": "from_chain",
	}
	cliParams := map[string]interface{}{
		"cli_param":    "cli_value",
		"shared_param": "from_cli",
	}

	merged, err := validator.MergeAndValidate(manifestParams, chainParams, cliParams)
	if err != nil {
		t.Fatalf("Failed to merge params: %v", err)
	}

	// Manifest param should win
	if merged["shared_param"] != "from_manifest" {
		t.Errorf("Expected manifest priority, got %v", merged["shared_param"])
	}

	// All unique params should be present
	if merged["manifest_param"] != "manifest_value" {
		t.Error("Missing manifest param")
	}
	if merged["chain_param"] != "chain_value" {
		t.Error("Missing chain param")
	}
	if merged["cli_param"] != "cli_value" {
		t.Error("Missing CLI param")
	}
}

func TestCheckSecurityParams_WithoutMerge(t *testing.T) {
	validator := NewParameterValidator()

	// Should return error when called without merging first
	err := validator.CheckSecurityParams()
	if err == nil {
		t.Error("Expected error when checking security params without merge")
	}
}

func TestMergeAndValidate_EmptyParams(t *testing.T) {
	validator := NewParameterValidator()

	// Test with all empty params - should fail due to missing required params
	_, err := validator.MergeAndValidate(nil, nil, nil)
	if err == nil {
		t.Error("Expected error for missing required parameters")
	}
	
	// Provide all required parameters
	manifestParams := map[string]string{
		"encrypted_path":       "/data/secrets",
		"secret_path":          "/secrets",
		"governance_contract":  "0x1234567890abcdef1234567890abcdef12345678",
		"security_config_contract": "0xfedcba9876543210fedcba9876543210fedcba98",
	}
	merged, err := validator.MergeAndValidate(manifestParams, nil, nil)
	if err != nil {
		t.Fatalf("Failed to merge with required params: %v", err)
	}

	if merged["encrypted_path"] != "/data/secrets" {
		t.Errorf("Expected encrypted_path, got %v", merged)
	}
}

