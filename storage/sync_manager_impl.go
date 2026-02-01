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

package storage

import (
	"context"
	"crypto/subtle"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/internal/sgx"
)

// PeerInfo stores information about a peer node
type PeerInfo struct {
	PeerID     common.Hash
	MREnclave  [32]byte
	Quote      []byte
	LastSync   uint64
	SyncStatus SyncStatus
}

// SyncManagerImpl implements SyncManager
type SyncManagerImpl struct {
	mu               sync.RWMutex
	partition        EncryptedPartition
	attestor         sgx.Attestor
	verifier         sgx.Verifier
	peers            map[common.Hash]*PeerInfo
	syncRequests     map[common.Hash]*SyncRequest
	allowedEnclaves  map[[32]byte]bool
	heartbeatRunning bool
}

// NewSyncManager creates a new sync manager
func NewSyncManager(partition EncryptedPartition, attestor sgx.Attestor, verifier sgx.Verifier) (*SyncManagerImpl, error) {
	return &SyncManagerImpl{
		partition:       partition,
		attestor:        attestor,
		verifier:        verifier,
		peers:           make(map[common.Hash]*PeerInfo),
		syncRequests:    make(map[common.Hash]*SyncRequest),
		allowedEnclaves: make(map[[32]byte]bool),
	}, nil
}

// UpdateAllowedEnclaves updates the list of allowed MRENCLAVE values
func (sm *SyncManagerImpl) UpdateAllowedEnclaves(enclaves [][32]byte) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.allowedEnclaves = make(map[[32]byte]bool)
	for _, enclave := range enclaves {
		sm.allowedEnclaves[enclave] = true
	}
}

// RequestSync initiates a sync request to a peer
func (sm *SyncManagerImpl) RequestSync(peerID common.Hash, secretTypes []SecretDataType) (common.Hash, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Verify peer exists and is allowed
	peer, exists := sm.peers[peerID]
	if !exists {
		return common.Hash{}, fmt.Errorf("peer not found")
	}

	// Verify peer's MRENCLAVE is in whitelist
	if !sm.allowedEnclaves[peer.MREnclave] {
		return common.Hash{}, fmt.Errorf("peer MRENCLAVE not in whitelist")
	}

	// Create sync request
	requestID := common.BytesToHash([]byte(fmt.Sprintf("%s-%d", peerID.Hex(), time.Now().UnixNano())))
	request := &SyncRequest{
		RequestID:   requestID,
		PeerID:      peerID,
		SecretTypes: secretTypes,
		Timestamp:   uint64(time.Now().Unix()),
	}

	sm.syncRequests[requestID] = request
	peer.SyncStatus = SyncStatusInProgress

	return requestID, nil
}

// HandleSyncRequest processes an incoming sync request
func (sm *SyncManagerImpl) HandleSyncRequest(request *SyncRequest) (*SyncResponse, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Verify the requesting peer is allowed
	peer, exists := sm.peers[request.PeerID]
	if !exists {
		return nil, fmt.Errorf("unknown peer")
	}

	if !sm.allowedEnclaves[peer.MREnclave] {
		return nil, fmt.Errorf("peer not in whitelist")
	}

	// Collect requested secrets
	secrets := make([]SecretData, 0)
	secretIDs, err := sm.partition.ListSecrets()
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	// Create a map of requested types for faster lookup
	requestedTypes := make(map[SecretDataType]bool)
	for _, st := range request.SecretTypes {
		requestedTypes[st] = true
	}

	for _, id := range secretIDs {
		data, err := sm.partition.ReadSecret(id)
		if err != nil {
			continue
		}

		// Parse the secret type from the ID or metadata
		// In a real implementation, the ID would encode the type or we'd store metadata
		// For now, we include all secrets if no specific types requested
		secret := SecretData{
			ID:        []byte(id),
			Data:      data,
			CreatedAt: uint64(time.Now().Unix()),
		}

		// If specific types requested and we can determine the type, filter
		// Since we don't have metadata storage yet, we'll include all secrets
		// when types are requested (the receiving end can filter)
		if len(requestedTypes) == 0 {
			// No filter, include all
			secrets = append(secrets, secret)
		} else {
			// Include all for now - in production, would parse type from ID/metadata
			secrets = append(secrets, secret)
		}
	}

	// Create response
	response := &SyncResponse{
		RequestID: request.RequestID,
		PeerID:    common.BytesToHash(sm.attestor.GetMREnclave()),
		Secrets:   secrets,
		Timestamp: uint64(time.Now().Unix()),
	}

	return response, nil
}

// VerifyAndApplySync verifies and applies a sync response
func (sm *SyncManagerImpl) VerifyAndApplySync(response *SyncResponse) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Verify the response corresponds to a known request
	_, exists := sm.syncRequests[response.RequestID]
	if !exists {
		return fmt.Errorf("unknown sync request")
	}

	// Verify peer
	peer, exists := sm.peers[response.PeerID]
	if !exists {
		return fmt.Errorf("unknown peer")
	}

	// Verify MRENCLAVE using constant-time comparison
	if !sm.verifyMREnclaveConstantTime(peer.MREnclave) {
		return fmt.Errorf("peer MRENCLAVE verification failed")
	}

	// Apply secrets to encrypted partition
	for _, secret := range response.Secrets {
		if err := sm.partition.WriteSecret(string(secret.ID), secret.Data); err != nil {
			return fmt.Errorf("failed to write secret: %w", err)
		}
	}

	// Update peer status
	peer.LastSync = uint64(time.Now().Unix())
	peer.SyncStatus = SyncStatusCompleted

	// Clean up request
	delete(sm.syncRequests, response.RequestID)

	return nil
}

// verifyMREnclaveConstantTime verifies MRENCLAVE in constant time to prevent timing attacks
func (sm *SyncManagerImpl) verifyMREnclaveConstantTime(mrenclave [32]byte) bool {
	for allowedMR := range sm.allowedEnclaves {
		if subtle.ConstantTimeCompare(mrenclave[:], allowedMR[:]) == 1 {
			return true
		}
	}
	return false
}

// AddPeer adds a new peer with its MRENCLAVE and quote
func (sm *SyncManagerImpl) AddPeer(peerID common.Hash, mrenclave [32]byte, quote []byte) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Verify the quote
	if err := sm.verifier.VerifyQuote(quote); err != nil {
		return fmt.Errorf("quote verification failed: %w", err)
	}

	// Add or update peer
	sm.peers[peerID] = &PeerInfo{
		PeerID:     peerID,
		MREnclave:  mrenclave,
		Quote:      quote,
		LastSync:   0,
		SyncStatus: SyncStatusPending,
	}

	return nil
}

// RemovePeer removes a peer
func (sm *SyncManagerImpl) RemovePeer(peerID common.Hash) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.peers, peerID)
	return nil
}

// GetSyncStatus gets the sync status for a peer
func (sm *SyncManagerImpl) GetSyncStatus(peerID common.Hash) (SyncStatus, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	peer, exists := sm.peers[peerID]
	if !exists {
		return SyncStatusFailed, fmt.Errorf("peer not found")
	}

	return peer.SyncStatus, nil
}

// StartHeartbeat starts the heartbeat mechanism
func (sm *SyncManagerImpl) StartHeartbeat(ctx context.Context) error {
	sm.mu.Lock()
	if sm.heartbeatRunning {
		sm.mu.Unlock()
		return fmt.Errorf("heartbeat already running")
	}
	sm.heartbeatRunning = true
	sm.mu.Unlock()

	go sm.heartbeatLoop(ctx)
	return nil
}

// heartbeatLoop runs the heartbeat loop
func (sm *SyncManagerImpl) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			sm.mu.Lock()
			sm.heartbeatRunning = false
			sm.mu.Unlock()
			return
		case <-ticker.C:
			// Heartbeat logic - check peer health
			sm.checkPeerHealth()
		}
	}
}

// checkPeerHealth checks the health of all peers
func (sm *SyncManagerImpl) checkPeerHealth() {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	now := uint64(time.Now().Unix())
	for _, peer := range sm.peers {
		// If a peer hasn't synced in 1 hour, mark it as pending
		if now-peer.LastSync > 3600 && peer.SyncStatus == SyncStatusCompleted {
			peer.SyncStatus = SyncStatusPending
		}
	}
}
