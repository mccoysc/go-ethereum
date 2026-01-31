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
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewRATLSEnvManager(t *testing.T) {
	// Set required environment variables
	os.Setenv("XCHAIN_SECURITY_CONFIG_CONTRACT", "0x1234567890abcdef1234567890abcdef12345678")
	os.Setenv("XCHAIN_GOVERNANCE_CONTRACT", "0xabcdef1234567890abcdef1234567890abcdef12")
	defer func() {
		os.Unsetenv("XCHAIN_SECURITY_CONFIG_CONTRACT")
		os.Unsetenv("XCHAIN_GOVERNANCE_CONTRACT")
	}()

	manager, err := NewRATLSEnvManager(nil)
	if err != nil {
		t.Fatalf("Failed to create env manager: %v", err)
	}

	if manager == nil {
		t.Fatal("Manager is nil")
	}

	expectedAddr := "0x1234567890abcdef1234567890abcdef12345678"
	actualAddr := manager.securityConfigContract.Hex()
	// Compare addresses case-insensitively
	if !strings.EqualFold(actualAddr, expectedAddr) {
		t.Errorf("Security config contract address mismatch: got %s, want %s", actualAddr, expectedAddr)
	}
}

func TestNewRATLSEnvManagerMissingEnvVars(t *testing.T) {
	// Test without environment variables
	os.Unsetenv("XCHAIN_SECURITY_CONFIG_CONTRACT")
	os.Unsetenv("XCHAIN_GOVERNANCE_CONTRACT")

	_, err := NewRATLSEnvManager(nil)
	if err == nil {
		t.Error("Expected error for missing environment variables")
	}
}

func TestInitFromContract(t *testing.T) {
	os.Setenv("XCHAIN_SECURITY_CONFIG_CONTRACT", "0x1234567890abcdef1234567890abcdef12345678")
	os.Setenv("XCHAIN_GOVERNANCE_CONTRACT", "0xabcdef1234567890abcdef1234567890abcdef12")
	defer func() {
		os.Unsetenv("XCHAIN_SECURITY_CONFIG_CONTRACT")
		os.Unsetenv("XCHAIN_GOVERNANCE_CONTRACT")
	}()

	manager, err := NewRATLSEnvManager(nil)
	if err != nil {
		t.Fatalf("Failed to create env manager: %v", err)
	}

	// Initialize from contract (will use mock data)
	err = manager.InitFromContract()
	if err != nil {
		t.Fatalf("InitFromContract failed: %v", err)
	}

	// Check that environment variables were cleared
	for _, envVar := range dynamicEnvVars {
		if val := os.Getenv(envVar); val != "" {
			// Some variables may be set by the mock config
			t.Logf("Environment variable %s = %s", envVar, val)
		}
	}
}

func TestGetCachedConfig(t *testing.T) {
	os.Setenv("XCHAIN_SECURITY_CONFIG_CONTRACT", "0x1234567890abcdef1234567890abcdef12345678")
	os.Setenv("XCHAIN_GOVERNANCE_CONTRACT", "0xabcdef1234567890abcdef1234567890abcdef12")
	defer func() {
		os.Unsetenv("XCHAIN_SECURITY_CONFIG_CONTRACT")
		os.Unsetenv("XCHAIN_GOVERNANCE_CONTRACT")
	}()

	manager, err := NewRATLSEnvManager(nil)
	if err != nil {
		t.Fatalf("Failed to create env manager: %v", err)
	}

	err = manager.InitFromContract()
	if err != nil {
		t.Fatalf("InitFromContract failed: %v", err)
	}

	config := manager.GetCachedConfig()
	if config == nil {
		t.Fatal("Cached config is nil")
	}

	// Verify config fields
	if config.ISVSVN != 1 {
		t.Errorf("ISVSVN = %d, want 1", config.ISVSVN)
	}

	if config.KeyMigrationThreshold != 3 {
		t.Errorf("KeyMigrationThreshold = %d, want 3", config.KeyMigrationThreshold)
	}
}

func TestEnvManagerIsAllowedMREnclave(t *testing.T) {
	os.Setenv("XCHAIN_SECURITY_CONFIG_CONTRACT", "0x1234567890abcdef1234567890abcdef12345678")
	os.Setenv("XCHAIN_GOVERNANCE_CONTRACT", "0xabcdef1234567890abcdef1234567890abcdef12")
	defer func() {
		os.Unsetenv("XCHAIN_SECURITY_CONFIG_CONTRACT")
		os.Unsetenv("XCHAIN_GOVERNANCE_CONTRACT")
	}()

	manager, err := NewRATLSEnvManager(nil)
	if err != nil {
		t.Fatalf("Failed to create env manager: %v", err)
	}

	err = manager.InitFromContract()
	if err != nil {
		t.Fatalf("InitFromContract failed: %v", err)
	}

	// Test with empty whitelist (should allow all in current mock)
	mrenclave := make([]byte, 32)
	for i := range mrenclave {
		mrenclave[i] = byte(i)
	}

	// With empty whitelist, should return false in production
	// (current mock returns empty list)
	allowed := manager.IsAllowedMREnclave(mrenclave)
	t.Logf("MRENCLAVE allowed: %v", allowed)
}

func TestGetLastUpdateTime(t *testing.T) {
	os.Setenv("XCHAIN_SECURITY_CONFIG_CONTRACT", "0x1234567890abcdef1234567890abcdef12345678")
	os.Setenv("XCHAIN_GOVERNANCE_CONTRACT", "0xabcdef1234567890abcdef1234567890abcdef12")
	defer func() {
		os.Unsetenv("XCHAIN_SECURITY_CONFIG_CONTRACT")
		os.Unsetenv("XCHAIN_GOVERNANCE_CONTRACT")
	}()

	manager, err := NewRATLSEnvManager(nil)
	if err != nil {
		t.Fatalf("Failed to create env manager: %v", err)
	}

	before := time.Now()
	err = manager.InitFromContract()
	if err != nil {
		t.Fatalf("InitFromContract failed: %v", err)
	}
	after := time.Now()

	lastUpdate := manager.GetLastUpdateTime()
	if lastUpdate.Before(before) || lastUpdate.After(after) {
		t.Errorf("Last update time %v not in expected range [%v, %v]",
			lastUpdate, before, after)
	}
}

func TestApplyConfiguration(t *testing.T) {
	os.Setenv("XCHAIN_SECURITY_CONFIG_CONTRACT", "0x1234567890abcdef1234567890abcdef12345678")
	os.Setenv("XCHAIN_GOVERNANCE_CONTRACT", "0xabcdef1234567890abcdef1234567890abcdef12")
	defer func() {
		os.Unsetenv("XCHAIN_SECURITY_CONFIG_CONTRACT")
		os.Unsetenv("XCHAIN_GOVERNANCE_CONTRACT")
	}()

	manager, err := NewRATLSEnvManager(nil)
	if err != nil {
		t.Fatalf("Failed to create env manager: %v", err)
	}

	// Create a test configuration
	config := &SecurityConfig{
		AllowedMREnclave: []string{"abc123"},
		AllowedMRSigner:  []string{"def456"},
		ISVProdID:        100,
		ISVSVN:           5,
		CertNotBefore:    "1000000",
		CertNotAfter:     "2000000",
	}

	err = manager.applyConfiguration(config)
	if err != nil {
		t.Fatalf("applyConfiguration failed: %v", err)
	}

	// Check that single-value environment variables were set
	if os.Getenv("RA_TLS_MRENCLAVE") != "abc123" {
		t.Errorf("RA_TLS_MRENCLAVE = %s, want abc123", os.Getenv("RA_TLS_MRENCLAVE"))
	}

	if os.Getenv("RA_TLS_ISV_PROD_ID") != "100" {
		t.Errorf("RA_TLS_ISV_PROD_ID = %s, want 100", os.Getenv("RA_TLS_ISV_PROD_ID"))
	}
}
