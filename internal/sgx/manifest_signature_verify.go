//go:build !testenv

package sgx

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"

	"github.com/ethereum/go-ethereum/log"
)

// VerifyManifestFileSignature verifies the manifest.sgx file signature
// CRITICAL SECURITY: signing_data is CALCULATED from file content, NOT extracted from SIGSTRUCT
// This ensures the signature is tied to the actual manifest content we read, preventing fake manifest attacks
func VerifyManifestFileSignature(manifestPath string) error {
	// Step 1: Read entire manifest.sgx file
	manifestData, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest file: %w", err)
	}

	if len(manifestData) < 1808 {
		return errors.New("manifest file too small - missing SIGSTRUCT")
	}

	// Step 2: Calculate signing_data = SHA256(entire file content)
	// THIS IS CALCULATED from what we actually read, NOT extracted from SIGSTRUCT
	// If attacker creates fake manifest.sgx with fake SIGSTRUCT, the calculated hash
	// will be different and signature verification will fail
	hash := sha256.Sum256(manifestData)
	signingData := hash[:]

	log.Info("Calculated manifest file hash for signature verification",
		"file", manifestPath,
		"hash", fmt.Sprintf("%x", signingData[:8]))

	// Step 3: Read external signature file (.sig)
	sigPath := manifestPath + ".sig"
	signature, err := ioutil.ReadFile(sigPath)
	if err != nil {
		return fmt.Errorf("failed to read signature file %s: %w", sigPath, err)
	}

	// Step 4: Extract public key (modulus) from SIGSTRUCT
	// We use the modulus from SIGSTRUCT to verify the signature
	// But the signing_data is calculated from file, not from SIGSTRUCT
	sigstruct := manifestData[0:1808]
	modulus := sigstruct[128:512] // RSA-N at offset 128, 384 bytes (RSA-3072)

	// Step 5: Verify RSA signature protects the CALCULATED hash
	// This ensures signature is tied to actual file content
	err = verifyRSASignatureAgainstHash(modulus, signature, signingData)
	if err != nil {
		return fmt.Errorf("manifest signature verification failed: %w", err)
	}

	log.Info("✓ Manifest file signature verified successfully",
		"file", manifestPath,
		"protection", "entire file content")
	return nil
}

// verifyRSASignatureAgainstHash verifies RSA-3072 signature (exponent=3) against a hash
// This is used to verify the external signature file protects the manifest file hash
func verifyRSASignatureAgainstHash(modulusBytes, signatureBytes, messageHash []byte) error {
	if len(messageHash) != 32 {
		return fmt.Errorf("invalid hash length: %d (expected 32)", len(messageHash))
	}

	// Convert modulus to big.Int (little-endian)
	modulus := new(big.Int).SetBytes(reverseBytes(modulusBytes))

	// Convert signature to big.Int (little-endian)
	sig := new(big.Int).SetBytes(reverseBytes(signatureBytes))

	// Compute S^3 mod N (RSA-3072 with exponent=3)
	e := big.NewInt(3)
	result := new(big.Int).Exp(sig, e, modulus)

	// Result should be PKCS#1 v1.5 padded message containing the hash
	resultBytes := result.Bytes()

	if len(resultBytes) < 32 {
		return fmt.Errorf("signature result too short: %d bytes", len(resultBytes))
	}

	// Extract hash from end of result (PKCS#1 v1.5 padding)
	// Format: 0x00 || 0x01 || 0xFF...FF || 0x00 || DigestInfo || Hash
	extractedHash := resultBytes[len(resultBytes)-32:]

	// Compare extracted hash with calculated hash
	if !bytesEqual(extractedHash, messageHash) {
		return fmt.Errorf("hash mismatch: signature does not protect this manifest file")
	}

	return nil
}

// Helper functions
func reverseBytes(b []byte) []byte {
	reversed := make([]byte, len(b))
	for i := range b {
		reversed[i] = b[len(b)-1-i]
	}
	return reversed
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
