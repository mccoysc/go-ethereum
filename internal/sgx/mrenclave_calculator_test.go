package sgx

import (
	"encoding/hex"
	"testing"
)

func TestParseMREnclaveSize(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		{"1G", 1024 * 1024 * 1024},
		{"2G", 2 * 1024 * 1024 * 1024},
		{"512M", 512 * 1024 * 1024},
		{"1024K", 1024 * 1024},
		{"", 1024 * 1024 * 1024}, // default
	}
	
	for _, tt := range tests {
		result, err := parseSize(tt.input)
		if err != nil {
			t.Errorf("parseSize(%q) error: %v", tt.input, err)
			continue
		}
		if result != tt.expected {
			t.Errorf("parseSize(%q) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestSGXMeasurement(t *testing.T) {
	m := NewSGXMeasurement()
	
	// Initial value should be all zeros
	initial := m.Value()
	allZeros := make([]byte, 32)
	for i := range allZeros {
		allZeros[i] = 0
	}
	
	if string(initial) != string(allZeros) {
		t.Errorf("Initial measurement not all zeros: %x", initial)
	}
	
	// Update with some data
	m.Update([]byte("test data"))
	
	// Value should have changed
	updated := m.Value()
	if string(updated) == string(allZeros) {
		t.Errorf("Measurement did not update after Update()")
	}
	
	// Update again
	m.Update([]byte("more data"))
	updated2 := m.Value()
	
	// Should be different from previous
	if string(updated2) == string(updated) {
		t.Errorf("Measurement did not change after second Update()")
	}
}

func TestCalculateMREnclaveBasic(t *testing.T) {
	// Create a simple manifest config
	manifest := &ManifestConfig{}
	manifest.SGX.EnclaveSize = "1G"
	manifest.SGX.ThreadNum = 32
	
	// Add a trusted file with known hash
	testHash, _ := hex.DecodeString("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
	manifest.SGX.TrustedFiles = []TrustedFileEntry{
		{
			URI:    "/test/file1",
			SHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}
	
	fileHashes := map[string][]byte{
		"/test/file1": testHash,
	}
	
	// Calculate MRENCLAVE
	mrenclave, err := CalculateMREnclave(manifest, fileHashes)
	if err != nil {
		t.Fatalf("CalculateMREnclave failed: %v", err)
	}
	
	// Should produce a 32-byte value
	if len(mrenclave) != 32 {
		t.Errorf("MRENCLAVE length = %d, want 32", len(mrenclave))
	}
	
	// Should not be all zeros
	allZeros := make([]byte, 32)
	if string(mrenclave) == string(allZeros) {
		t.Errorf("MRENCLAVE is all zeros")
	}
	
	t.Logf("Calculated MRENCLAVE: %x", mrenclave)
}

func TestCalculateMREnclaveDeterministic(t *testing.T) {
	// Same manifest should produce same MRENCLAVE
	manifest := &ManifestConfig{}
	manifest.SGX.EnclaveSize = "2G"
	manifest.SGX.ThreadNum = 16
	
	testHash, _ := hex.DecodeString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	manifest.SGX.TrustedFiles = []TrustedFileEntry{
		{URI: "/test/file", SHA256: hex.EncodeToString(testHash)},
	}
	
	fileHashes := map[string][]byte{
		"/test/file": testHash,
	}
	
	// Calculate twice
	mr1, err1 := CalculateMREnclave(manifest, fileHashes)
	mr2, err2 := CalculateMREnclave(manifest, fileHashes)
	
	if err1 != nil || err2 != nil {
		t.Fatalf("CalculateMREnclave failed: %v, %v", err1, err2)
	}
	
	// Should be identical
	if string(mr1) != string(mr2) {
		t.Errorf("MRENCLAVE not deterministic:\n  1st: %x\n  2nd: %x", mr1, mr2)
	}
}

func TestCalculateMREnclaveDifferentInputs(t *testing.T) {
	// Different manifests should produce different MRENCLAVEs
	
	// Manifest 1
	manifest1 := &ManifestConfig{}
	manifest1.SGX.EnclaveSize = "1G"
	manifest1.SGX.TrustedFiles = []TrustedFileEntry{
		{URI: "/test/file1", SHA256: "aa"},
	}
	
	// Manifest 2 - different size
	manifest2 := &ManifestConfig{}
	manifest2.SGX.EnclaveSize = "2G"
	manifest2.SGX.TrustedFiles = []TrustedFileEntry{
		{URI: "/test/file1", SHA256: "aa"},
	}
	
	hash1, _ := hex.DecodeString("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	fileHashes := map[string][]byte{
		"/test/file1": hash1,
	}
	
	mr1, _ := CalculateMREnclave(manifest1, fileHashes)
	mr2, _ := CalculateMREnclave(manifest2, fileHashes)
	
	if string(mr1) == string(mr2) {
		t.Errorf("Different manifests produced same MRENCLAVE: %x", mr1)
	}
	
	t.Logf("MRENCLAVE 1G: %x", mr1)
	t.Logf("MRENCLAVE 2G: %x", mr2)
}
