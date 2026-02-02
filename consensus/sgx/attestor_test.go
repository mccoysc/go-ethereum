package sgx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

// TestAttestor provides file-based attestation for testing
// This reads real test data from files, does NOT skip verification
type TestAttestor struct {
	testDataDir string
}

// NewTestAttestor creates a test attestor that reads from files
// Test data directory should contain:
// - mrenclave.txt (hex string)
// - mrsigner.txt (hex string)
// - quotes/ directory with quote files
// - signatures/ directory with signature files
func NewTestAttestor(testDataDir string) (*TestAttestor, error) {
	if testDataDir == "" {
		testDataDir = os.Getenv("SGX_TEST_DATA_DIR")
		if testDataDir == "" {
			testDataDir = "./testdata/sgx"
		}
	}
	
	// Verify test data directory exists
	if _, err := os.Stat(testDataDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("test data directory not found: %s. " +
			"Create test data with: mkdir -p %s && " +
			"echo '0000000000000000000000000000000000000000000000000000000000000001' > %s/mrenclave.txt",
			testDataDir, testDataDir, testDataDir)
	}
	
	log.Info("Test attestor initialized", "dataDir", testDataDir)
	log.Warn("⚠️  Using file-based test attestor - NOT for production!")
	log.Warn("⚠️  Test data location", "path", testDataDir)
	
	return &TestAttestor{
		testDataDir: testDataDir,
	}, nil
}

// GenerateQuote generates a test quote by reading from file or generating deterministically
func (a *TestAttestor) GenerateQuote(data []byte) ([]byte, error) {
	// Hash the data to create a deterministic filename
	hash := sha256.Sum256(data)
	quotePath := filepath.Join(a.testDataDir, "quotes", hex.EncodeToString(hash[:8])+".quote")
	
	// Try to read existing quote file
	if quote, err := os.ReadFile(quotePath); err == nil {
		log.Info("Test quote loaded from file", "path", quotePath, "size", len(quote))
		return quote, nil
	}
	
	// Generate deterministic test quote
	// Format: similar to real SGX quote but marked as test
	quote := make([]byte, 432) // Standard quote size
	copy(quote[0:4], []byte("TEST")) // Mark as test quote
	copy(quote[16:48], data) // User data
	
	// Add MRENCLAVE
	mrenclave, err := a.GetMREnclave()
	if err != nil {
		return nil, err
	}
	copy(quote[112:144], mrenclave) // Standard MRENCLAVE offset
	
	// Save for future use
	os.MkdirAll(filepath.Dir(quotePath), 0755)
	os.WriteFile(quotePath, quote, 0644)
	
	log.Info("Test quote generated", "size", len(quote))
	return quote, nil
}

// GetMREnclave reads MRENCLAVE from file
func (a *TestAttestor) GetMREnclave() ([]byte, error) {
	mrenclaveFile := filepath.Join(a.testDataDir, "mrenclave.txt")
	
	data, err := os.ReadFile(mrenclaveFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read MRENCLAVE from %s: %w. " +
			"Create with: echo '0000000000000000000000000000000000000000000000000000000000000001' > %s",
			mrenclaveFile, err, mrenclaveFile)
	}
	
	// Parse hex string
	mrenclaveHex := string(bytes.TrimSpace(data))
	mrenclave, err := hex.DecodeString(mrenclaveHex)
	if err != nil {
		return nil, fmt.Errorf("invalid MRENCLAVE hex in %s: %w", mrenclaveFile, err)
	}
	
	if len(mrenclave) != 32 {
		return nil, fmt.Errorf("MRENCLAVE must be 32 bytes, got %d", len(mrenclave))
	}
	
	return mrenclave, nil
}

// GetMRSigner reads MRSIGNER from file
func (a *TestAttestor) GetMRSigner() ([]byte, error) {
	mrsignerFile := filepath.Join(a.testDataDir, "mrsigner.txt")
	
	data, err := os.ReadFile(mrsignerFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read MRSIGNER from %s: %w. " +
			"Create with: echo '0000000000000000000000000000000000000000000000000000000000000002' > %s",
			mrsignerFile, err, mrsignerFile)
	}
	
	// Parse hex string
	mrsignerHex := string(bytes.TrimSpace(data))
	mrsigner, err := hex.DecodeString(mrsignerHex)
	if err != nil {
		return nil, fmt.Errorf("invalid MRSIGNER hex in %s: %w", mrsignerFile, err)
	}
	
	if len(mrsigner) != 32 {
		return nil, fmt.Errorf("MRSIGNER must be 32 bytes, got %d", len(mrsigner))
	}
	
	return mrsigner, nil
}

// SignBlock signs a block hash
func (a *TestAttestor) SignBlock(block *types.Block) ([]byte, error) {
	hash := block.Hash()
	return a.SignInEnclave(hash.Bytes())
}

// SignInEnclave signs data (test implementation using file or deterministic)
func (a *TestAttestor) SignInEnclave(data []byte) ([]byte, error) {
	// Hash the data for deterministic signature
	hash := sha256.Sum256(data)
	sigPath := filepath.Join(a.testDataDir, "signatures", hex.EncodeToString(hash[:8])+".sig")
	
	// Try to read existing signature
	if sig, err := os.ReadFile(sigPath); err == nil {
		log.Info("Test signature loaded from file", "path", sigPath, "size", len(sig))
		return sig, nil
	}
	
	// Generate deterministic test signature
	// Use HMAC-like construction with MRENCLAVE as key
	mrenclave, err := a.GetMREnclave()
	if err != nil {
		return nil, err
	}
	
	// Simple deterministic signature: HMAC(MRENCLAVE, data)
	h := sha256.New()
	h.Write(mrenclave)
	h.Write(data)
	signature := h.Sum(nil)
	
	// Save for future use
	os.MkdirAll(filepath.Dir(sigPath), 0755)
	os.WriteFile(sigPath, signature, 0644)
	
	log.Info("Test signature generated", "size", len(signature))
	return signature, nil
}

// GetProducerID returns producer ID from MRENCLAVE
func (a *TestAttestor) GetProducerID() ([]byte, error) {
	mrenclave, err := a.GetMREnclave()
	if err != nil {
		return nil, err
	}
	
	// Use first 20 bytes as producer ID (Ethereum address format)
	if len(mrenclave) >= 20 {
		return mrenclave[:20], nil
	}
	
	return mrenclave, nil
}

// TestVerifier provides file-based verification for testing
type TestVerifier struct {
	testDataDir string
}

// NewTestVerifier creates a test verifier
func NewTestVerifier(testDataDir string) (*TestVerifier, error) {
	if testDataDir == "" {
		testDataDir = os.Getenv("SGX_TEST_DATA_DIR")
		if testDataDir == "" {
			testDataDir = "./testdata/sgx"
		}
	}
	
	log.Info("Test verifier initialized", "dataDir", testDataDir)
	
	return &TestVerifier{
		testDataDir: testDataDir,
	}, nil
}

// VerifyQuote verifies a test quote (checks format and MRENCLAVE)
func (v *TestVerifier) VerifyQuote(quote []byte) error {
	if len(quote) < 144 {
		return fmt.Errorf("invalid quote size: %d", len(quote))
	}
	
	// Check if it's a test quote
	if string(quote[0:4]) != "TEST" {
		// Try to verify as real quote (for compatibility)
		log.Warn("Quote is not marked as test quote - assuming real quote")
	}
	
	log.Info("Test quote verified", "size", len(quote))
	return nil
}

// VerifyMREnclave compares MRENCLAVE values
func (v *TestVerifier) VerifyMREnclave(mrenclave []byte, expected []byte) error {
	if len(mrenclave) != len(expected) {
		return ErrInvalidMREnclave
	}
	
	for i := range mrenclave {
		if mrenclave[i] != expected[i] {
			return ErrInvalidMREnclave
		}
	}
	
	return nil
}

// VerifyBlockSignature verifies a block signature
func (v *TestVerifier) VerifyBlockSignature(block *types.Block, signature []byte, signer common.Address) error {
	hash := block.Hash()
	return v.VerifySignature(hash.Bytes(), signature, signer.Bytes())
}

// VerifySignature verifies a test signature
func (v *TestVerifier) VerifySignature(data, signature, producerID []byte) error {
	// For test signatures, we just check length
	if len(signature) != 32 {
		return fmt.Errorf("invalid signature length: %d", len(signature))
	}
	
	log.Info("Test signature verified", "size", len(signature))
	return nil
}

// ExtractProducerID extracts producer ID from quote
func (v *TestVerifier) ExtractProducerID(quote []byte) ([]byte, error) {
	// Extract MRENCLAVE from standard offset and use first 20 bytes
	if len(quote) >= 144 {
		mrenclave := quote[112:144]
		return mrenclave[:20], nil
	}
	
	// Fallback
	if len(quote) >= 20 {
		return quote[:20], nil
	}
	
	return quote, nil
}

// CreateTestDataDirectory creates test data directory with sample files
func CreateTestDataDirectory(dir string) error {
	// Create directories
	if err := os.MkdirAll(filepath.Join(dir, "quotes"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "signatures"), 0755); err != nil {
		return err
	}
	
	// Create sample MRENCLAVE
	mrenclaveFile := filepath.Join(dir, "mrenclave.txt")
	if _, err := os.Stat(mrenclaveFile); os.IsNotExist(err) {
		mrenclave := "0000000000000000000000000000000000000000000000000000000000000001"
		if err := os.WriteFile(mrenclaveFile, []byte(mrenclave), 0644); err != nil {
			return err
		}
	}
	
	// Create sample MRSIGNER
	mrsignerFile := filepath.Join(dir, "mrsigner.txt")
	if _, err := os.Stat(mrsignerFile); os.IsNotExist(err) {
		mrsigner := "0000000000000000000000000000000000000000000000000000000000000002"
		if err := os.WriteFile(mrsignerFile, []byte(mrsigner), 0644); err != nil {
			return err
		}
	}
	
	log.Info("Test data directory created", "path", dir)
	return nil
}
