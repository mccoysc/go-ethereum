//go:build !testenv

package sgx

import (
"bytes"
"crypto/sha256"
"encoding/hex"
"fmt"
"os"
"strings"

"github.com/ethereum/go-ethereum/log"
)

// Trusted MRSIGNER whitelist
// MRSIGNER = SHA256(modulus) identifies the signing key
var trustedMRSigners = make(map[string]bool)

func init() {
	// Load trusted MRSigners from configuration
	loadTrustedMRSigners()
}

// loadTrustedMRSigners initializes the trusted MRSIGNER whitelist
func loadTrustedMRSigners() {
	// Load from environment variable
	envMRSigners := os.Getenv("XCHAIN_TRUSTED_MRSIGNERS")
	if envMRSigners != "" {
		for _, mrsigner := range strings.Split(envMRSigners, ",") {
			mrsigner = strings.TrimSpace(mrsigner)
			if len(mrsigner) == 64 { // Valid hex string
				trustedMRSigners[mrsigner] = true
				log.Info("Added trusted MRSIGNER from environment", "mrsigner", mrsigner)
			}
		}
	}
	
	// If no trusted MRSigners configured, log critical warning
	if len(trustedMRSigners) == 0 {
		log.Warn("WARNING: No trusted MRSigners configured - all manifest signing keys will be rejected in production")
		log.Warn("Set XCHAIN_TRUSTED_MRSIGNERS environment variable with comma-separated hex values")
	}
}

// AddTrustedMRSigner adds a MRSIGNER to the trusted whitelist
func AddTrustedMRSigner(mrsigner []byte) {
	if len(mrsigner) != 32 {
		log.Error("Invalid MRSIGNER length", "length", len(mrsigner))
		return
	}
	mrSignerHex := hex.EncodeToString(mrsigner)
	trustedMRSigners[mrSignerHex] = true
	log.Info("Added trusted MRSIGNER", "mrsigner", mrSignerHex)
}

// IsMRSignerTrusted checks if a MRSIGNER is in the trusted whitelist
func IsMRSignerTrusted(mrsigner []byte) bool {
	if len(mrsigner) != 32 {
		return false
	}
	mrSignerHex := hex.EncodeToString(mrsigner)
	return trustedMRSigners[mrSignerHex]
}

// verifyMRSignerTrusted verifies the MRSIGNER (signing key) is trusted
// This is CRITICAL for security - without this, attacker could use their own key
func verifyMRSignerTrusted(sigstruct []byte) error {
	// Extract modulus (RSA public key) from SIGSTRUCT
	if len(sigstruct) < 512 {
		return fmt.Errorf("SIGSTRUCT too small: %d bytes", len(sigstruct))
	}
	
	modulus := sigstruct[128:512] // 384 bytes
	
	// Calculate MRSIGNER = SHA256(modulus)
	hash := sha256.Sum256(modulus)
	mrsigner := hash[:]
	mrSignerHex := hex.EncodeToString(mrsigner)
	
	// Verify MRSIGNER is in trusted whitelist
	if !IsMRSignerTrusted(mrsigner) {
		log.Error("SECURITY VIOLATION: Untrusted MRSIGNER",
			"mrsigner", mrSignerHex)
		log.Crit("CRITICAL: Manifest signed by unknown/untrusted key - possible attack")
		return fmt.Errorf("untrusted MRSIGNER: %s - signing key not in whitelist", mrSignerHex)
	}
	
	log.Info("MRSIGNER verification successful - signing key is trusted",
		"mrsigner", mrSignerHex)
	return nil
}

// verifyManifestMREnclaveImpl compares manifest MRENCLAVE with runtime MRENCLAVE
// Production version: strict validation, mismatch causes failure
func verifyManifestMREnclaveImpl(manifestMR, runtimeMR []byte) error {
	if len(manifestMR) != 32 {
		return fmt.Errorf("invalid manifest MRENCLAVE length: %d", len(manifestMR))
	}
	if len(runtimeMR) != 32 {
		return fmt.Errorf("invalid runtime MRENCLAVE length: %d", len(runtimeMR))
	}

	if !bytes.Equal(manifestMR, runtimeMR) {
		log.Error("MRENCLAVE mismatch - SECURITY VIOLATION",
			"manifest", hex.EncodeToString(manifestMR),
			"runtime", hex.EncodeToString(runtimeMR))
		log.Crit("CRITICAL: Manifest MRENCLAVE does not match runtime - possible tampering detected")
		return fmt.Errorf("MRENCLAVE mismatch: manifest=%x runtime=%x",
			manifestMR, runtimeMR)
	}

	log.Info("MRENCLAVE verification successful",
		"value", hex.EncodeToString(manifestMR))
	return nil
}
