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
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
)

// SecurityConfig represents the security configuration parameters read from chain.
type SecurityConfig struct {
	// MRENCLAVE whitelist
	AllowedMREnclave []string
	
	// MRSIGNER whitelist
	AllowedMRSigner []string
	
	// ISV Product ID and Security Version Number
	ISVProdID uint16
	ISVSVN    uint16
	
	// Certificate validity timestamps
	CertNotBefore string
	CertNotAfter  string
	
	// Key migration parameters
	KeyMigrationThreshold uint64
	
	// Admission policy
	AdmissionStrict bool
}

// RATLSEnvManager manages RA-TLS environment variables dynamically from on-chain contracts.
// It reads security parameters from the SecurityConfigContract and either:
// 1. Sets environment variables for Gramine RA-TLS to use, OR
// 2. Configures callbacks for multi-value parameters
type RATLSEnvManager struct {
	mu                     sync.RWMutex
	securityConfigContract common.Address
	governanceContract     common.Address
	client                 *ethclient.Client
	cachedConfig           *SecurityConfig
	lastUpdate             time.Time
}

// Dynamic environment variables that are read from on-chain contracts
var dynamicEnvVars = []string{
	"RA_TLS_MRENCLAVE",
	"RA_TLS_MRSIGNER",
	"RA_TLS_ISV_PROD_ID",
	"RA_TLS_ISV_SVN",
	"RA_TLS_CERT_TIMESTAMP_NOT_BEFORE",
	"RA_TLS_CERT_TIMESTAMP_NOT_AFTER",
}

// NewRATLSEnvManager creates a new RA-TLS environment variable manager.
// Contract addresses are read from Gramine manifest environment variables.
func NewRATLSEnvManager(client *ethclient.Client) (*RATLSEnvManager, error) {
	// Read contract addresses from manifest environment variables
	// These are fixed in the manifest and affect MRENCLAVE
	scAddr := os.Getenv("XCHAIN_SECURITY_CONFIG_CONTRACT")
	govAddr := os.Getenv("XCHAIN_GOVERNANCE_CONTRACT")
	
	if scAddr == "" {
		return nil, fmt.Errorf("XCHAIN_SECURITY_CONFIG_CONTRACT not set in environment")
	}
	if govAddr == "" {
		return nil, fmt.Errorf("XCHAIN_GOVERNANCE_CONTRACT not set in environment")
	}
	
	manager := &RATLSEnvManager{
		securityConfigContract: common.HexToAddress(scAddr),
		governanceContract:     common.HexToAddress(govAddr),
		client:                 client,
		cachedConfig:           &SecurityConfig{},
	}
	
	return manager, nil
}

// InitFromContract reads security parameters from on-chain contract and configures RA-TLS.
// This should be called once during geth startup before establishing P2P connections.
func (m *RATLSEnvManager) InitFromContract() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 1. Clear dynamic environment variables (prevent external injection)
	for _, envVar := range dynamicEnvVars {
		os.Unsetenv(envVar)
	}
	
	// 2. Fetch security configuration from on-chain contract
	config, err := m.fetchSecurityConfig()
	if err != nil {
		return fmt.Errorf("failed to fetch security config from contract: %w", err)
	}
	
	m.cachedConfig = config
	m.lastUpdate = time.Now()
	
	// 3. Configure environment variables or callbacks based on parameter types
	if err := m.applyConfiguration(config); err != nil {
		return fmt.Errorf("failed to apply configuration: %w", err)
	}
	
	return nil
}

// applyConfiguration applies the security configuration by setting environment variables
// or configuring callbacks for multi-value parameters.
func (m *RATLSEnvManager) applyConfiguration(config *SecurityConfig) error {
	// Handle MRENCLAVE whitelist
	if len(config.AllowedMREnclave) == 1 {
		// Single value: set environment variable
		os.Setenv("RA_TLS_MRENCLAVE", config.AllowedMREnclave[0])
	} else if len(config.AllowedMREnclave) > 1 {
		// Multiple values: use callback function (handled by verifier)
		// Don't set environment variable
		log.Info("MRENCLAVE whitelist has multiple values, using callback verification",
			"count", len(config.AllowedMREnclave))
	}
	
	// Handle MRSIGNER whitelist
	if len(config.AllowedMRSigner) == 1 {
		os.Setenv("RA_TLS_MRSIGNER", config.AllowedMRSigner[0])
	} else if len(config.AllowedMRSigner) > 1 {
		log.Info("MRSIGNER whitelist has multiple values, using callback verification",
			"count", len(config.AllowedMRSigner))
	}
	
	// Set single-value parameters
	os.Setenv("RA_TLS_ISV_PROD_ID", fmt.Sprintf("%d", config.ISVProdID))
	os.Setenv("RA_TLS_ISV_SVN", fmt.Sprintf("%d", config.ISVSVN))
	os.Setenv("RA_TLS_CERT_TIMESTAMP_NOT_BEFORE", config.CertNotBefore)
	os.Setenv("RA_TLS_CERT_TIMESTAMP_NOT_AFTER", config.CertNotAfter)
	
	return nil
}

// fetchSecurityConfig fetches the security configuration from the on-chain contract.
// This is a placeholder - actual implementation would call contract methods.
func (m *RATLSEnvManager) fetchSecurityConfig() (*SecurityConfig, error) {
	// TODO: Implement actual contract calls
	// For now, return a default configuration
	
	// In production, this would:
	// 1. Call SecurityConfigContract.getAllowedMREnclave()
	// 2. Call SecurityConfigContract.getAllowedMRSigner()
	// 3. Call SecurityConfigContract.getISVProdID()
	// 4. Call SecurityConfigContract.getISVSVN()
	// 5. Call SecurityConfigContract.getCertValidityPeriod()
	// 6. Call GovernanceContract.getKeyMigrationThreshold()
	// 7. Call SecurityConfigContract.getAdmissionPolicy()
	
	config := &SecurityConfig{
		AllowedMREnclave:      []string{},
		AllowedMRSigner:       []string{},
		ISVProdID:             0,
		ISVSVN:                1,
		CertNotBefore:         "0",
		CertNotAfter:          fmt.Sprintf("%d", time.Now().Add(365*24*time.Hour).Unix()),
		KeyMigrationThreshold: 3,
		AdmissionStrict:       false,
	}
	
	return config, nil
}

// StartPeriodicRefresh starts a background goroutine that periodically refreshes
// security configuration from on-chain contracts.
func (m *RATLSEnvManager) StartPeriodicRefresh(refreshInterval time.Duration) {
	go func() {
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()
		
		for range ticker.C {
			if err := m.InitFromContract(); err != nil {
				log.Error("Failed to refresh security config from contract", "error", err)
			} else {
				log.Info("Security config refreshed from contract")
			}
		}
	}()
}

// GetCachedConfig returns the cached security configuration.
// This is safe to call concurrently.
func (m *RATLSEnvManager) GetCachedConfig() *SecurityConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Return a copy to prevent external modification
	config := *m.cachedConfig
	config.AllowedMREnclave = append([]string{}, m.cachedConfig.AllowedMREnclave...)
	config.AllowedMRSigner = append([]string{}, m.cachedConfig.AllowedMRSigner...)
	
	return &config
}

// IsAllowedMREnclave checks if the given MRENCLAVE is in the whitelist.
// This uses the cached configuration.
func (m *RATLSEnvManager) IsAllowedMREnclave(mrenclave []byte) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	mrenclaveHex := fmt.Sprintf("%x", mrenclave)
	
	for _, allowed := range m.cachedConfig.AllowedMREnclave {
		if mrenclaveHex == allowed {
			return true
		}
	}
	
	return false
}

// GetLastUpdateTime returns the timestamp of the last configuration update.
func (m *RATLSEnvManager) GetLastUpdateTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastUpdate
}
