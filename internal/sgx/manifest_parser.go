package sgx

import (
	"fmt"
	"log"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// ManifestConfig represents the parsed manifest structure
type ManifestConfig struct {
	LibOS struct {
		Entrypoint string `toml:"entrypoint"`
	} `toml:"libos"`
	
	Loader struct {
		Env map[string]string `toml:"env"`
	} `toml:"loader"`
	
	SGX struct {
		TrustedFiles  []TrustedFileEntry `toml:"trusted_files"`
		EnclaveSize   string            `toml:"enclave_size"`
		ThreadNum     int               `toml:"thread_num"`
		ISVProdID     int               `toml:"isvprodid"`
		ISVSVN        int               `toml:"isvsvn"`
		RemoteAttest  string            `toml:"remote_attestation"`
		MiscSelect    string            `toml:"misc_select"`
		Attributes    struct {
			Flags string `toml:"flags"`
			Xfrm  string `toml:"xfrm"`
		} `toml:"attributes"`
	} `toml:"sgx"`
}

// TrustedFileEntry represents a trusted file in the manifest
type TrustedFileEntry struct {
	URI    string `toml:"uri"`
	SHA256 string `toml:"sha256"`
}

// ParseManifestTOML parses the TOML content from manifest.sgx
func ParseManifestTOML(tomlData []byte) (*ManifestConfig, error) {
	var config ManifestConfig
	
	err := toml.Unmarshal(tomlData, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest TOML: %w", err)
	}
	
	return &config, nil
}

// ParseManifestFile reads and parses a manifest.sgx file
// SECURITY WARNING: This function does NOT verify the manifest integrity.
// Use ReadAndVerifyManifestFromDisk() when reading from external filesystem
// to ensure the file hasn't been tampered with.
func ParseManifestFile(manifestPath string) (*ManifestConfig, []byte, error) {
	// Read entire manifest.sgx file
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read manifest file: %w", err)
	}
	
	if len(data) < 1808 {
		return nil, nil, fmt.Errorf("manifest file too small: %d bytes", len(data))
	}
	
	// SIGSTRUCT is first 1808 bytes, TOML is the rest
	sigstruct := data[0:1808]
	tomlData := data[1808:]
	
	// Parse TOML
	config, err := ParseManifestTOML(tomlData)
	if err != nil {
		return nil, nil, err
	}
	
	return config, sigstruct, nil
}

// AppConfig holds application configuration from manifest environment variables
type AppConfig struct {
	GovernanceContract     string
	SecurityConfigContract string
	NodeType               string
	// Add other config fields as needed
}

// GetAppConfigFromEnvironment reads application configuration from environment variables.
// This is the CORRECT way to get configuration when running inside Gramine SGX.
//
// HOW IT WORKS:
// 1. Manifest defines config in loader.env section:
//    loader.env.GOVERNANCE_CONTRACT = "0x..."
//    loader.env.SECURITY_CONFIG_CONTRACT = "0x..."
// 2. Gramine verifies manifest at startup
// 3. Gramine sets these as environment variables
// 4. We read from environment variables
//
// SECURITY:
// - Gramine verified manifest (signature + MRENCLAVE)
// - Environment variables are in SGX-protected memory
// - No file reading needed
// - No additional verification needed
//
// User's question: "不从外部读取你从哪里取得manifest文件？"
// Answer: We DON'T get the manifest file. We get config from environment variables
//         that Gramine set from the verified manifest.
func GetAppConfigFromEnvironment() (*AppConfig, error) {
	// Verify we're in Gramine SGX environment
	mrenclave := os.Getenv("RA_TLS_MRENCLAVE")
	if mrenclave == "" {
		return nil, fmt.Errorf("not in SGX environment - RA_TLS_MRENCLAVE not set")
	}

	// Read application config from environment variables
	// These are set by Gramine from manifest loader.env section
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
