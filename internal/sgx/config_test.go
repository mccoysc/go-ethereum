package sgx

import (
	"os"
	"strings"
	"testing"
)

// SetupTestEnvironment sets up environment variables for testing outside Gramine
func SetupTestEnvironment(t *testing.T) func() {
	t.Helper()
	
	// Set test mode
	os.Setenv("SGX_TEST_MODE", "true")
	os.Setenv("GRAMINE_VERSION", "test")
	
	// Set required configuration (simulating loader.env from manifest)
	os.Setenv("GOVERNANCE_CONTRACT", "0xd9145CCE52D386f254917e481eB44e9943F39138")
	os.Setenv("SECURITY_CONFIG_CONTRACT", "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045")
	os.Setenv("NODE_TYPE", "validator")
	
	// Return cleanup function
	return func() {
		os.Unsetenv("SGX_TEST_MODE")
		os.Unsetenv("GRAMINE_VERSION")
		os.Unsetenv("GOVERNANCE_CONTRACT")
		os.Unsetenv("SECURITY_CONFIG_CONTRACT")
		os.Unsetenv("NODE_TYPE")
	}
}

func TestGetAppConfigFromEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		setupEnv    func()
		cleanupEnv  func()
		expectError bool
		errorMsg    string
		validate    func(*testing.T, *AppConfig)
	}{
		{
			name: "Success_with_complete_config_Gramine",
			setupEnv: func() {
				os.Setenv("RA_TLS_MRENCLAVE", "faa284c4d200890541c4515810ef8ad2065c18a4c979cfb1e16ee5576fe014ee")
				os.Setenv("GOVERNANCE_CONTRACT", "0xd9145CCE52D386f254917e481eB44e9943F39138")
				os.Setenv("SECURITY_CONFIG_CONTRACT", "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045")
				os.Setenv("NODE_TYPE", "validator")
			},
			cleanupEnv: func() {
				os.Unsetenv("RA_TLS_MRENCLAVE")
				os.Unsetenv("GOVERNANCE_CONTRACT")
				os.Unsetenv("SECURITY_CONFIG_CONTRACT")
				os.Unsetenv("NODE_TYPE")
			},
			expectError: false,
			validate: func(t *testing.T, config *AppConfig) {
				if config.GovernanceContract != "0xd9145CCE52D386f254917e481eB44e9943F39138" {
					t.Errorf("Wrong GovernanceContract: %s", config.GovernanceContract)
				}
				if config.SecurityConfigContract != "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045" {
					t.Errorf("Wrong SecurityConfigContract: %s", config.SecurityConfigContract)
				}
				if config.NodeType != "validator" {
					t.Errorf("Wrong NodeType: %s", config.NodeType)
				}
			},
		},
		{
			name: "Success_in_test_mode",
			setupEnv: func() {
				os.Setenv("SGX_TEST_MODE", "true")
				os.Setenv("GOVERNANCE_CONTRACT", "0xd9145CCE52D386f254917e481eB44e9943F39138")
				os.Setenv("SECURITY_CONFIG_CONTRACT", "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045")
			},
			cleanupEnv: func() {
				os.Unsetenv("SGX_TEST_MODE")
				os.Unsetenv("GOVERNANCE_CONTRACT")
				os.Unsetenv("SECURITY_CONFIG_CONTRACT")
			},
			expectError: false,
			validate: func(t *testing.T, config *AppConfig) {
				if config.GovernanceContract != "0xd9145CCE52D386f254917e481eB44e9943F39138" {
					t.Errorf("Wrong GovernanceContract: %s", config.GovernanceContract)
				}
			},
		},
		{
			name: "Fail_not_in_SGX_environment",
			setupEnv: func() {
				os.Unsetenv("RA_TLS_MRENCLAVE")
				os.Unsetenv("SGX_TEST_MODE")
				os.Unsetenv("GRAMINE_VERSION")
			},
			cleanupEnv:  func() {},
			expectError: true,
			errorMsg:    "not in SGX environment",
		},
		{
			name: "Fail_missing_governance_contract",
			setupEnv: func() {
				os.Setenv("SGX_TEST_MODE", "true")
				os.Unsetenv("GOVERNANCE_CONTRACT")
				os.Setenv("SECURITY_CONFIG_CONTRACT", "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045")
			},
			cleanupEnv: func() {
				os.Unsetenv("SGX_TEST_MODE")
				os.Unsetenv("SECURITY_CONFIG_CONTRACT")
			},
			expectError: true,
			errorMsg:    "GOVERNANCE_CONTRACT not set",
		},
		{
			name: "Fail_missing_security_config",
			setupEnv: func() {
				os.Setenv("SGX_TEST_MODE", "true")
				os.Setenv("GOVERNANCE_CONTRACT", "0xd9145CCE52D386f254917e481eB44e9943F39138")
				os.Unsetenv("SECURITY_CONFIG_CONTRACT")
			},
			cleanupEnv: func() {
				os.Unsetenv("SGX_TEST_MODE")
				os.Unsetenv("GOVERNANCE_CONTRACT")
			},
			expectError: true,
			errorMsg:    "SECURITY_CONFIG_CONTRACT not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()
			defer tt.cleanupEnv()

			config, err := GetAppConfigFromEnvironment()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if config == nil {
					t.Error("Expected non-nil config")
				}
				if tt.validate != nil {
					tt.validate(t, config)
				}
			}
		})
	}
}
