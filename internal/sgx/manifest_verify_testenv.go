//go:build testenv

package sgx

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"

	"github.com/ethereum/go-ethereum/log"
)

// verifyMRSignerTrusted verifies MRSIGNER in test mode
// Test version: logs but always succeeds (allows any signing key for testing)
func verifyMRSignerTrusted(sigstruct []byte) error {
	if len(sigstruct) < 512 {
		log.Warn("SIGSTRUCT too small for MRSIGNER verification", "size", len(sigstruct))
		return nil // Allow in test mode
	}
	
	// Extract modulus and calculate MRSIGNER
	modulus := sigstruct[128:512]
	hash := sha256.Sum256(modulus)
	mrsigner := hash[:]
	mrSignerHex := hex.EncodeToString(mrsigner)
	
	log.Warn("Test build: skipping MRSIGNER whitelist verification",
		"mrsigner", mrSignerHex)
	log.Warn("THIS WOULD REQUIRE TRUSTED MRSIGNER IN PRODUCTION")
	
	// Always succeed in test mode
	return nil
}

// verifyManifestMREnclaveImpl compares manifest MRENCLAVE with runtime MRENCLAVE
// Test environment version: logs warning but always succeeds
func verifyManifestMREnclaveImpl(manifestMR, runtimeMR []byte) error {
	if len(manifestMR) != 32 {
		log.Warn("Invalid manifest MRENCLAVE length", "length", len(manifestMR))
	}
	if len(runtimeMR) != 32 {
		log.Warn("Invalid runtime MRENCLAVE length", "length", len(runtimeMR))
	}

	if !bytes.Equal(manifestMR, runtimeMR) {
		log.Warn("MRENCLAVE mismatch (allowed in test mode)",
			"manifest", hex.EncodeToString(manifestMR),
			"runtime", hex.EncodeToString(runtimeMR))
		log.Warn("Test build: accepting mismatched MRENCLAVE - THIS WOULD FAIL IN PRODUCTION")
	} else {
		log.Info("MRENCLAVE verification successful",
			"value", hex.EncodeToString(manifestMR))
	}

	// Always return nil in test mode
	return nil
}
