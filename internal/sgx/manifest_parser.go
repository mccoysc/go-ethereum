package sgx

import (
	"fmt"
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
