//go:build testenv

package sgx

import (
	"bytes"
	"encoding/hex"

	"github.com/ethereum/go-ethereum/log"
)

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
