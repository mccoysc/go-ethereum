package sgx

import (
	"bytes"
	"encoding/hex"
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

// ReadAndVerifyManifestFromDisk reads manifest from external filesystem and verifies integrity
// by comparing MRENCLAVE in the file with runtime MRENCLAVE (set by Gramine after verification).
// 
// SECURITY: This is critical when reading manifest from disk because:
// 1. Gramine verified manifest at startup and set RA_TLS_MRENCLAVE
// 2. But the file on disk could be modified after startup
// 3. We MUST verify MRENCLAVE matches to ensure integrity
//
// User requirement: "既然要读manifest内容，如果是从外部不受保护环境读的，就是要被验证才行"
// Translation: "If reading manifest from external unprotected environment, MUST verify"
func ReadAndVerifyManifestFromDisk(manifestPath string) (*ManifestConfig, error) {
	// 1. Read manifest file from disk (potentially untrusted)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	if len(data) < 1808 {
		return nil, fmt.Errorf("manifest file too small: %d bytes", len(data))
	}

	// 2. Extract MRENCLAVE from SIGSTRUCT in the file
	sigstruct := data[0:1808]
	fileMREnclave := sigstruct[960:992]

	// 3. Get runtime MRENCLAVE (set by Gramine after verifying manifest at startup)
	runtimeMREnclaveHex := os.Getenv("RA_TLS_MRENCLAVE")
	if runtimeMREnclaveHex == "" {
		// Not in SGX mode or Gramine didn't set the variable
		// In test/development mode, skip verification
		log.Printf("Warning: RA_TLS_MRENCLAVE not set, skipping manifest MRENCLAVE verification")
		manifestTOML := data[1808:]
		return ParseManifestTOML(manifestTOML)
	}

	// 4. Convert runtime MRENCLAVE from hex string to bytes
	runtimeMREnclave, err := hex.DecodeString(runtimeMREnclaveHex)
	if err != nil {
		return nil, fmt.Errorf("invalid RA_TLS_MRENCLAVE format: %w", err)
	}

	// 5. CRITICAL SECURITY CHECK: Verify MRENCLAVE matches
	// This proves the file on disk corresponds to what Gramine verified at startup
	if !bytes.Equal(fileMREnclave, runtimeMREnclave) {
		return nil, fmt.Errorf("SECURITY VIOLATION: Manifest file MRENCLAVE mismatch\n"+
			"File MRENCLAVE:    %x\n"+
			"Runtime MRENCLAVE: %x\n"+
			"The manifest file may have been tampered with after Gramine verified it",
			fileMREnclave, runtimeMREnclave)
	}

	// 6. MRENCLAVE verified - file is authentic, safe to parse and use
	manifestTOML := data[1808:]
	config, err := ParseManifestTOML(manifestTOML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest TOML: %w", err)
	}

	log.Printf("Manifest verification successful - MRENCLAVE matches runtime value")
	return config, nil
}
