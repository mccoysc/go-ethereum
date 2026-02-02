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

package config

import (
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
)

// ValidateParameters validates parameter consistency across three layers:
// 1. Manifest fixed parameters (highest priority, affects MRENCLAVE)
// 2. On-chain security parameters (from SecurityConfigContract)
// 3. Command line parameters (lowest priority, runtime config)
func ValidateParameters(cliConfig *CLIConfig) error {
	// Load manifest config from environment variables
	manifestConfig := loadManifestConfig()

	// Validate path parameters consistency
	if cliConfig.EncryptedPath != "" &&
		cliConfig.EncryptedPath != manifestConfig.EncryptedPath {
		return fmt.Errorf(
			"encrypted path mismatch: CLI=%s, Manifest=%s. "+
				"Manifest parameters cannot be overridden",
			cliConfig.EncryptedPath,
			manifestConfig.EncryptedPath,
		)
	}

	// Validate contract address consistency
	if cliConfig.GovernanceContract != (common.Address{}) &&
		cliConfig.GovernanceContract != manifestConfig.GovernanceContract {
		return fmt.Errorf(
			"governance contract mismatch: CLI=%s, Manifest=%s. "+
				"Contract addresses are fixed in manifest",
			cliConfig.GovernanceContract.Hex(),
			manifestConfig.GovernanceContract.Hex(),
		)
	}

	if cliConfig.SecurityConfigContract != (common.Address{}) &&
		cliConfig.SecurityConfigContract != manifestConfig.SecurityConfigContract {
		return fmt.Errorf(
			"security config contract mismatch: CLI=%s, Manifest=%s. "+
				"Contract addresses are fixed in manifest",
			cliConfig.SecurityConfigContract.Hex(),
			manifestConfig.SecurityConfigContract.Hex(),
		)
	}

	// Use Manifest parameters to override CLI parameters (Manifest has highest priority)
	cliConfig.EncryptedPath = manifestConfig.EncryptedPath
	cliConfig.SecretPath = manifestConfig.SecretPath
	cliConfig.GovernanceContract = manifestConfig.GovernanceContract
	cliConfig.SecurityConfigContract = manifestConfig.SecurityConfigContract

	return nil
}

// ManifestConfig represents fixed parameters from the Gramine manifest
// These parameters affect MRENCLAVE and cannot be changed at runtime
type ManifestConfig struct {
	EncryptedPath          string         // Path to encrypted partition
	SecretPath             string         // Path to secrets storage
	GovernanceContract     common.Address // Governance contract address
	SecurityConfigContract common.Address // Security config contract address
}

// loadManifestConfig loads manifest configuration from environment variables
// These variables are set in the Gramine manifest and affect MRENCLAVE
func loadManifestConfig() *ManifestConfig {
	return &ManifestConfig{
		EncryptedPath: getEnvOrDefault("XCHAIN_ENCRYPTED_PATH", "/data/encrypted"),
		SecretPath:    getEnvOrDefault("XCHAIN_SECRET_PATH", "/data/secrets"),
		GovernanceContract: common.HexToAddress(
			getEnvOrDefault("XCHAIN_GOVERNANCE_CONTRACT", "0x0000000000000000000000000000000000001001"),
		),
		SecurityConfigContract: common.HexToAddress(
			getEnvOrDefault("XCHAIN_SECURITY_CONFIG_CONTRACT", "0x0000000000000000000000000000000000001002"),
		),
	}
}

// CLIConfig represents runtime configuration from command line
type CLIConfig struct {
	EncryptedPath          string         // Path to encrypted partition
	SecretPath             string         // Path to secrets storage
	GovernanceContract     common.Address // Governance contract address
	SecurityConfigContract common.Address // Security config contract address
	// Additional runtime parameters can be added here
}

// getEnvOrDefault retrieves an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// MergeConfigs merges configurations with proper priority:
// Manifest > Chain > CLI
func MergeConfigs(manifestConfig *ManifestConfig, chainConfig *ChainConfig, cliConfig *CLIConfig) *FinalConfig {
	return &FinalConfig{
		EncryptedPath:          manifestConfig.EncryptedPath,
		SecretPath:             manifestConfig.SecretPath,
		GovernanceContract:     manifestConfig.GovernanceContract,
		SecurityConfigContract: manifestConfig.SecurityConfigContract,
		// Chain config parameters would be merged here
		// CLI config parameters with lowest priority
	}
}

// ChainConfig represents on-chain security parameters
// These are read from the SecurityConfigContract
type ChainConfig struct {
	// Add on-chain configuration parameters here
	// For example: MaxGasLimit, BlockInterval, etc.
}

// FinalConfig is the merged configuration from all sources
type FinalConfig struct {
	EncryptedPath          string
	SecretPath             string
	GovernanceContract     common.Address
	SecurityConfigContract common.Address
	// Additional merged parameters
}
