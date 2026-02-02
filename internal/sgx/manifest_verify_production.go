//go:build !testenv

package sgx

import (
"bytes"
"encoding/hex"
"fmt"

"github.com/ethereum/go-ethereum/log"
)

// verifyManifestMREnclave compares manifest MRENCLAVE with runtime MRENCLAVE
// Production version: strict validation, mismatch causes failure
func (v *ManifestSignatureVerifier) verifyManifestMREnclave(manifestMR, runtimeMR []byte) error {
if len(manifestMR) != 32 {
return fmt.Errorf("invalid manifest MRENCLAVE length: %d", len(manifestMR))
}
if len(runtimeMR) != 32 {
return fmt.Errorf("invalid runtime MRENCLAVE length: %d", len(runtimeMR))
}

if !bytes.Equal(manifestMR, runtimeMR) {
log.Error("MRENCLAVE mismatch - security violation",
"manifest", hex.EncodeToString(manifestMR),
"runtime", hex.EncodeToString(runtimeMR))
return fmt.Errorf("MRENCLAVE mismatch: manifest=%x runtime=%x",
manifestMR, runtimeMR)
}

log.Info("MRENCLAVE verification successful",
"value", hex.EncodeToString(manifestMR))
return nil
}
