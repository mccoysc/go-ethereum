package sgx

import (
	"fmt"
	"log"
	"os"
)

// AppConfig holds application configuration from manifest environment variables
type AppConfig struct {
	GovernanceContract     string
	SecurityConfigContract string
	NodeType               string
}

// GetAppConfigFromEnvironment reads application configuration from environment variables.
// Config is defined in manifest loader.env section, verified by Gramine at startup.
func GetAppConfigFromEnvironment() (*AppConfig, error) {
	// Verify we're in Gramine SGX environment
	mrenclave := os.Getenv("RA_TLS_MRENCLAVE")
	if mrenclave == "" {
		return nil, fmt.Errorf("not in SGX environment - RA_TLS_MRENCLAVE not set")
	}

	// Read application config from environment variables
	config := &AppConfig{
		GovernanceContract:     os.Getenv("GOVERNANCE_CONTRACT"),
		SecurityConfigContract: os.Getenv("SECURITY_CONFIG_CONTRACT"),
		NodeType:               os.Getenv("NODE_TYPE"),
	}

	// Validate required config
	if config.GovernanceContract == "" {
		return nil, fmt.Errorf("GOVERNANCE_CONTRACT not set in environment")
	}
	if config.SecurityConfigContract == "" {
		return nil, fmt.Errorf("SECURITY_CONFIG_CONTRACT not set in environment")
	}

	log.Printf("Loaded config from SGX environment (MRENCLAVE: %s...)", mrenclave[:16])
	return config, nil
}
