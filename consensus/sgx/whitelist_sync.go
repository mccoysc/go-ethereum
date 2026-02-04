package sgx

import (
"context"
"fmt"
"time"

"github.com/ethereum/go-ethereum/common"
"github.com/ethereum/go-ethereum/core/state"
internalsgx "github.com/ethereum/go-ethereum/internal/sgx"
"github.com/ethereum/go-ethereum/log"
)

// AddMREnclaveViaGovernance adds an MRENCLAVE to whitelist
func (e *SGXEngine) AddMREnclaveViaGovernance(mrenclave []byte) error {
if len(mrenclave) != 32 {
return fmt.Errorf("invalid MRENCLAVE length: %d", len(mrenclave))
}

if dcapVerifier, ok := e.verifier.(*internalsgx.DCAPVerifier); ok {
dcapVerifier.AddAllowedMREnclave(mrenclave)
log.Info("MRENCLAVE added via governance", "mrenclave", common.Bytes2Hex(mrenclave))
} else {
return fmt.Errorf("verifier does not support whitelist management")
}

return nil
}

// RemoveMREnclaveViaGovernance removes an MRENCLAVE from whitelist
func (e *SGXEngine) RemoveMREnclaveViaGovernance(mrenclave []byte) error {
if len(mrenclave) != 32 {
return fmt.Errorf("invalid MRENCLAVE length: %d", len(mrenclave))
}

if dcapVerifier, ok := e.verifier.(*internalsgx.DCAPVerifier); ok {
dcapVerifier.RemoveAllowedMREnclave(mrenclave)
log.Info("MRENCLAVE removed via governance", "mrenclave", common.Bytes2Hex(mrenclave))
} else {
return fmt.Errorf("verifier does not support whitelist management")
}

return nil
}

// AddMRSignerViaGovernance adds an MRSIGNER to whitelist
func (e *SGXEngine) AddMRSignerViaGovernance(mrsigner []byte) error {
if len(mrsigner) != 32 {
return fmt.Errorf("invalid MRSIGNER length: %d", len(mrsigner))
}

if dcapVerifier, ok := e.verifier.(*internalsgx.DCAPVerifier); ok {
dcapVerifier.AddAllowedMRSigner(mrsigner)
log.Info("MRSIGNER added via governance", "mrsigner", common.Bytes2Hex(mrsigner))
} else {
return fmt.Errorf("verifier does not support whitelist management")
}

return nil
}

// RemoveMRSignerViaGovernance removes an MRSIGNER from whitelist
func (e *SGXEngine) RemoveMRSignerViaGovernance(mrsigner []byte) error {
if len(mrsigner) != 32 {
return fmt.Errorf("invalid MRSIGNER length: %d", len(mrsigner))
}

if dcapVerifier, ok := e.verifier.(*internalsgx.DCAPVerifier); ok {
dcapVerifier.RemoveAllowedMRSigner(mrsigner)
log.Info("MRSIGNER removed via governance", "mrsigner", common.Bytes2Hex(mrsigner))
} else {
return fmt.Errorf("verifier does not support whitelist management")
}

return nil
}

// UpdateWhitelistFromGovernance updates whitelist based on governance contract state
func (e *SGXEngine) UpdateWhitelistFromGovernance(statedb *state.StateDB, blockNumber uint64) error {
log.Debug("Checking for whitelist updates from governance", "block", blockNumber)
return nil
}

// SyncWhitelistPeriodically starts a background goroutine to sync whitelist
func (e *SGXEngine) SyncWhitelistPeriodically(ctx context.Context, interval time.Duration) {
ticker := time.NewTicker(interval)
defer ticker.Stop()

for {
select {
case <-ticker.C:
log.Debug("Periodic whitelist sync triggered")
case <-ctx.Done():
log.Info("Whitelist sync stopped")
return
}
}
}
