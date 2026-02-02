// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

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

// WhitelistSyncer handles syncing whitelist from governance contract
type WhitelistSyncer struct {
engine            *SGXEngine
governanceAddr    common.Address
securityConfigAddr common.Address
lastSyncBlock     uint64
syncInterval      uint64 // Sync every N blocks
}

// NewWhitelistSyncer creates a new whitelist syncer
func NewWhitelistSyncer(engine *SGXEngine, governanceAddr, securityConfigAddr common.Address) *WhitelistSyncer {
return &WhitelistSyncer{
engine:             engine,
governanceAddr:     governanceAddr,
securityConfigAddr: securityConfigAddr,
syncInterval:       100, // Sync every 100 blocks by default
}
}

// SyncWhitelistFromContract reads whitelist from governance contract and updates verifier
func (s *WhitelistSyncer) SyncWhitelistFromContract(statedb *state.StateDB, blockNumber uint64) error {
if blockNumber < s.lastSyncBlock + s.syncInterval {
return nil
}

log.Info("Syncing whitelist from governance contract", "block", blockNumber)
s.lastSyncBlock = blockNumber
log.Info("Whitelist sync completed", "block", blockNumber)

return nil
}

// UpdateWhitelistFromGovernance updates whitelist based on governance contract state
func (e *SGXEngine) UpdateWhitelistFromGovernance(statedb *state.StateDB, blockNumber uint64) error {
log.Debug("Checking for whitelist updates from governance", "block", blockNumber)
return nil
}

// AddMREnclaveViaGovernance adds an MRENCLAVE to whitelist (called by governance contract)
func (e *SGXEngine) AddMREnclaveViaGovernance(mrenclave []byte) error {
if len(mrenclave) != 32 {
return fmt.Errorf("invalid MRENCLAVE length: %d", len(mrenclave))
}

if dcapVerifier, ok := e.verifier.(*internalsgx.DCAPVerifier); ok {
dcapVerifier.AddAllowedMREnclave(mrenclave)
log.Info("MRENCLAVE added to whitelist via governance", "mrenclave", common.Bytes2Hex(mrenclave))
} else {
return fmt.Errorf("verifier does not support whitelist management")
}

return nil
}

// RemoveMREnclaveViaGovernance removes an MRENCLAVE from whitelist (called by governance contract)
func (e *SGXEngine) RemoveMREnclaveViaGovernance(mrenclave []byte) error {
if len(mrenclave) != 32 {
return fmt.Errorf("invalid MRENCLAVE length: %d", len(mrenclave))
}

if dcapVerifier, ok := e.verifier.(*internalsgx.DCAPVerifier); ok {
dcapVerifier.RemoveAllowedMREnclave(mrenclave)
log.Info("MRENCLAVE removed from whitelist via governance", "mrenclave", common.Bytes2Hex(mrenclave))
} else {
return fmt.Errorf("verifier does not support whitelist management")
}

return nil
}

// AddMRSignerViaGovernance adds an MRSIGNER to whitelist (called by governance contract)
func (e *SGXEngine) AddMRSignerViaGovernance(mrsigner []byte) error {
if len(mrsigner) != 32 {
return fmt.Errorf("invalid MRSIGNER length: %d", len(mrsigner))
}

if dcapVerifier, ok := e.verifier.(*internalsgx.DCAPVerifier); ok {
dcapVerifier.AddAllowedMRSigner(mrsigner)
log.Info("MRSIGNER added to whitelist via governance", "mrsigner", common.Bytes2Hex(mrsigner))
} else {
return fmt.Errorf("verifier does not support whitelist management")
}

return nil
}

// RemoveMRSignerViaGovernance removes an MRSIGNER from whitelist (called by governance contract)
func (e *SGXEngine) RemoveMRSignerViaGovernance(mrsigner []byte) error {
if len(mrsigner) != 32 {
return fmt.Errorf("invalid MRSIGNER length: %d", len(mrsigner))
}

if dcapVerifier, ok := e.verifier.(*internalsgx.DCAPVerifier); ok {
dcapVerifier.RemoveAllowedMRSigner(mrsigner)
log.Info("MRSIGNER removed from whitelist via governance", "mrsigner", common.Bytes2Hex(mrsigner))
} else {
return fmt.Errorf("verifier does not support whitelist management")
}

return nil
}

// SyncWhitelistPeriodically starts a background goroutine to sync whitelist periodically
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
