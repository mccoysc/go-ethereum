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
	"testing"
)

func TestValidateManifestIntegrity_TestMode(t *testing.T) {
	// Set test mode
	os.Setenv("SGX_TEST_MODE", "true")
	defer os.Unsetenv("SGX_TEST_MODE")

	err := ValidateManifestIntegrity()
	if err != nil {
		t.Errorf("Expected no error in test mode, got: %v", err)
	}
}

func TestValidateManifestIntegrity_NonSGXMode(t *testing.T) {
	// Ensure not in SGX mode
	os.Unsetenv("IN_SGX")
	os.Unsetenv("GRAMINE_SGX")
	os.Unsetenv("SGX_TEST_MODE")

	// Should not fail in non-SGX mode
	err := ValidateManifestIntegrity()
	if err != nil {
		t.Errorf("Expected no error in non-SGX mode, got: %v", err)
	}
}

func TestValidateManifestIntegrity_SGXModeWithMeasurements(t *testing.T) {
	// Simulate SGX mode with measurements
	os.Setenv("IN_SGX", "1")
	os.Setenv("RA_TLS_MRENCLAVE", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	os.Setenv("RA_TLS_MRSIGNER", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	defer func() {
		os.Unsetenv("IN_SGX")
		os.Unsetenv("RA_TLS_MRENCLAVE")
		os.Unsetenv("RA_TLS_MRSIGNER")
	}()

	err := ValidateManifestIntegrity()
	if err != nil {
		t.Errorf("Expected no error with valid SGX measurements, got: %v", err)
	}
}

func TestValidateManifestIntegrity_SGXModeNoMREnclave(t *testing.T) {
	// Simulate SGX mode without MRENCLAVE
	os.Setenv("IN_SGX", "1")
	os.Unsetenv("RA_TLS_MRENCLAVE")
	os.Unsetenv("SGX_MRENCLAVE")
	defer os.Unsetenv("IN_SGX")

	err := ValidateManifestIntegrity()
	if err == nil {
		t.Error("Expected error when MRENCLAVE is missing in SGX mode")
	}
}

func TestGetMRENCLAVE(t *testing.T) {
	expectedMREnclave := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	os.Setenv("RA_TLS_MRENCLAVE", expectedMREnclave)
	defer os.Unsetenv("RA_TLS_MRENCLAVE")

	mrenclave, err := GetMRENCLAVE()
	if err != nil {
		t.Fatalf("Failed to get MRENCLAVE: %v", err)
	}

	if mrenclave != expectedMREnclave {
		t.Errorf("Expected MRENCLAVE %s, got %s", expectedMREnclave, mrenclave)
	}
}

func TestGetMRENCLAVE_AlternativeEnvVar(t *testing.T) {
	expectedMREnclave := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	os.Unsetenv("RA_TLS_MRENCLAVE")
	os.Setenv("SGX_MRENCLAVE", expectedMREnclave)
	defer os.Unsetenv("SGX_MRENCLAVE")

	mrenclave, err := GetMRENCLAVE()
	if err != nil {
		t.Fatalf("Failed to get MRENCLAVE from alternative env var: %v", err)
	}

	if mrenclave != expectedMREnclave {
		t.Errorf("Expected MRENCLAVE %s, got %s", expectedMREnclave, mrenclave)
	}
}

func TestGetMRENCLAVE_NotFound(t *testing.T) {
	os.Unsetenv("RA_TLS_MRENCLAVE")
	os.Unsetenv("SGX_MRENCLAVE")

	_, err := GetMRENCLAVE()
	if err == nil {
		t.Error("Expected error when MRENCLAVE not found")
	}
}

func TestGetMRSIGNER(t *testing.T) {
	expectedMRSigner := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	os.Setenv("RA_TLS_MRSIGNER", expectedMRSigner)
	defer os.Unsetenv("RA_TLS_MRSIGNER")

	mrsigner, err := GetMRSIGNER()
	if err != nil {
		t.Fatalf("Failed to get MRSIGNER: %v", err)
	}

	if mrsigner != expectedMRSigner {
		t.Errorf("Expected MRSIGNER %s, got %s", expectedMRSigner, mrsigner)
	}
}

func TestGetMRSIGNER_NotFound(t *testing.T) {
	os.Unsetenv("RA_TLS_MRSIGNER")
	os.Unsetenv("SGX_MRSIGNER")

	_, err := GetMRSIGNER()
	if err == nil {
		t.Error("Expected error when MRSIGNER not found")
	}
}

func TestNewManifestSignatureVerifier_TestMode(t *testing.T) {
	os.Setenv("SGX_TEST_MODE", "true")
	defer os.Unsetenv("SGX_TEST_MODE")

	verifier, err := NewManifestSignatureVerifier()
	if err != nil {
		t.Fatalf("Failed to create verifier in test mode: %v", err)
	}

	if verifier == nil {
		t.Fatal("Verifier is nil")
	}

	if verifier.publicKey != nil {
		t.Error("Expected nil public key in test mode")
	}
}

func TestGetManifestPath_FromEnv(t *testing.T) {
	// Create a temporary manifest file
	tmpFile := t.TempDir() + "/geth.manifest.sgx"
	os.WriteFile(tmpFile, []byte("test manifest"), 0644)
	tmpSigFile := tmpFile + ".sig"
	os.WriteFile(tmpSigFile, []byte("test signature"), 0644)

	os.Setenv("GRAMINE_MANIFEST_PATH", tmpFile)
	defer os.Unsetenv("GRAMINE_MANIFEST_PATH")

	path, err := GetManifestPath()
	if err != nil {
		t.Fatalf("Failed to get manifest path: %v", err)
	}

	if path != tmpFile {
		t.Errorf("Expected path %s, got %s", tmpFile, path)
	}
}

func TestGetManifestPath_NotFound(t *testing.T) {
	os.Unsetenv("GRAMINE_MANIFEST_PATH")
	os.Unsetenv("GRAMINE_APP_NAME")

	_, err := GetManifestPath()
	if err == nil {
		t.Error("Expected error when manifest not found")
	}
}
