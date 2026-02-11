//go:build testenv
// +build testenv

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
"fmt"
"os"
"testing"
"time"

"github.com/ethereum/go-ethereum/common"
"github.com/ethereum/go-ethereum/internal/sgx"
)

// generateValidMockQuote generates a properly formatted mock SGX quote
func generateValidMockQuote(t *testing.T, reportData []byte) []byte {
t.Helper()

attestor, err := sgx.NewGramineAttestor()
if err != nil {
t.Fatalf("Failed to create attestor for quote generation: %v", err)
}

quote, err := attestor.GenerateQuote(reportData)
if err != nil {
t.Fatalf("Failed to generate quote: %v", err)
}

return quote
}

// createTestSyncManager creates a SyncManager with real SGX interfaces in test mode
func createTestSyncManager(t *testing.T, tmpDir string) (*SyncManagerImpl, error) {
t.Helper()

partition, err := NewEncryptedPartition(tmpDir)
if err != nil {
return nil, fmt.Errorf("failed to create partition: %w", err)
}

// Use real SGX interfaces in test mode
attestor, err := sgx.NewGramineAttestor()
if err != nil {
return nil, fmt.Errorf("failed to create attestor: %w", err)
}

verifier, err := sgx.NewGramineVerifier()
if err != nil {
return nil, fmt.Errorf("failed to create verifier: %w", err)
}

return NewSyncManager(partition, attestor, verifier)
}

func TestNewSyncManager(t *testing.T) {
setupTestEnvironment(t)
defer cleanupTestEnvironment(t)

tmpDir := t.TempDir()
os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

syncManager, err := createTestSyncManager(t, tmpDir)
if err != nil {
t.Fatalf("Failed to create sync manager: %v", err)
}

if syncManager == nil {
t.Fatal("Sync manager is nil")
}
}

func TestAddPeer(t *testing.T) {
setupTestEnvironment(t)
defer cleanupTestEnvironment(t)

tmpDir := t.TempDir()
os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

syncManager, err := createTestSyncManager(t, tmpDir)
if err != nil {
t.Fatalf("Failed to create sync manager: %v", err)
}

// Generate a valid mock quote
quote := generateValidMockQuote(t, []byte("test-data"))

// Add a peer
peerID := common.BytesToHash([]byte("peer1"))
mrenclave := [32]byte{4, 5, 6}

err = syncManager.AddPeer(peerID, mrenclave, quote)
if err != nil {
t.Fatalf("Failed to add peer: %v", err)
}

// Verify peer was added
status, err := syncManager.GetSyncStatus(peerID)
if err != nil {
t.Fatalf("Failed to get sync status: %v", err)
}

if status != SyncStatusPending {
t.Errorf("Expected status %v, got %v", SyncStatusPending, status)
}
}

func TestAddPeer_InvalidQuote(t *testing.T) {
setupTestEnvironment(t)
defer cleanupTestEnvironment(t)

tmpDir := t.TempDir()
os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

syncManager, err := createTestSyncManager(t, tmpDir)
if err != nil {
t.Fatalf("Failed to create sync manager: %v", err)
}

peerID := common.BytesToHash([]byte("peer1"))
mrenclave := [32]byte{4, 5, 6}
// Short quote that will fail parsing
invalidQuote := []byte("too-short")

err = syncManager.AddPeer(peerID, mrenclave, invalidQuote)
if err == nil {
t.Fatal("Expected error for invalid quote")
}
}

func TestRemovePeer(t *testing.T) {
setupTestEnvironment(t)
defer cleanupTestEnvironment(t)

tmpDir := t.TempDir()
os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

syncManager, err := createTestSyncManager(t, tmpDir)
if err != nil {
t.Fatalf("Failed to create sync manager: %v", err)
}

// Generate a valid mock quote
quote := generateValidMockQuote(t, []byte("test-data"))

peerID := common.BytesToHash([]byte("peer1"))
mrenclave := [32]byte{4, 5, 6}

// Add peer
err = syncManager.AddPeer(peerID, mrenclave, quote)
if err != nil {
t.Fatalf("Failed to add peer: %v", err)
}

// Remove peer
err = syncManager.RemovePeer(peerID)
if err != nil {
t.Fatalf("Failed to remove peer: %v", err)
}

// Verify peer was removed
_, err = syncManager.GetSyncStatus(peerID)
if err == nil {
t.Fatal("Expected error for removed peer")
}
}

func TestRequestSync(t *testing.T) {
setupTestEnvironment(t)
defer cleanupTestEnvironment(t)

tmpDir := t.TempDir()
os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

syncManager, err := createTestSyncManager(t, tmpDir)
if err != nil {
t.Fatalf("Failed to create sync manager: %v", err)
}

// Generate a valid mock quote
quote := generateValidMockQuote(t, []byte("test-data"))

// Add peer to whitelist
peerID := common.BytesToHash([]byte("peer1"))
mrenclave := [32]byte{4, 5, 6}
syncManager.AddPeer(peerID, mrenclave, quote)
syncManager.UpdateAllowedEnclaves([][32]byte{mrenclave})

// Request sync
secretTypes := []SecretDataType{SecretTypePrivateKey, SecretTypeNodeIdentity}
requestID, err := syncManager.RequestSync(peerID, secretTypes)
if err != nil {
t.Fatalf("Failed to request sync: %v", err)
}

if requestID == (common.Hash{}) {
t.Error("Request ID is empty")
}
}

func TestRequestSync_PeerNotInWhitelist(t *testing.T) {
setupTestEnvironment(t)
defer cleanupTestEnvironment(t)

tmpDir := t.TempDir()
os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

syncManager, err := createTestSyncManager(t, tmpDir)
if err != nil {
t.Fatalf("Failed to create sync manager: %v", err)
}

// Generate a valid mock quote
quote := generateValidMockQuote(t, []byte("test-data"))

// Add peer but don't add to whitelist
peerID := common.BytesToHash([]byte("peer1"))
mrenclave := [32]byte{4, 5, 6}
syncManager.AddPeer(peerID, mrenclave, quote)

// Request sync should fail
_, err = syncManager.RequestSync(peerID, []SecretDataType{SecretTypePrivateKey})
if err == nil {
t.Fatal("Expected error for peer not in whitelist")
}
}

func TestHandleSyncRequest(t *testing.T) {
setupTestEnvironment(t)
defer cleanupTestEnvironment(t)

tmpDir := t.TempDir()
os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

partition, err := NewEncryptedPartition(tmpDir)
if err != nil {
t.Fatalf("Failed to create partition: %v", err)
}

// Write some test secrets
partition.WriteSecret("secret1", []byte("data1"))
partition.WriteSecret("secret2", []byte("data2"))

syncManager, err := createTestSyncManager(t, tmpDir)
if err != nil {
t.Fatalf("Failed to create sync manager: %v", err)
}

// Generate a valid mock quote
quote := generateValidMockQuote(t, []byte("test-data"))

// Add peer to whitelist
peerID := common.BytesToHash([]byte("peer1"))
mrenclave := [32]byte{4, 5, 6}
syncManager.AddPeer(peerID, mrenclave, quote)
syncManager.UpdateAllowedEnclaves([][32]byte{mrenclave})

// Create sync request
request := &SyncRequest{
RequestID:   common.BytesToHash([]byte("request1")),
PeerID:      peerID,
SecretTypes: []SecretDataType{SecretTypePrivateKey},
Timestamp:   uint64(time.Now().Unix()),
}

// Handle request
response, err := syncManager.HandleSyncRequest(request)
if err != nil {
t.Fatalf("Failed to handle sync request: %v", err)
}

if response == nil {
t.Fatal("Response is nil")
}

if response.RequestID != request.RequestID {
t.Error("Response request ID doesn't match")
}
}

func TestStartHeartbeat(t *testing.T) {
setupTestEnvironment(t)
defer cleanupTestEnvironment(t)

tmpDir := t.TempDir()
os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

syncManager, err := createTestSyncManager(t, tmpDir)
if err != nil {
t.Fatalf("Failed to create sync manager: %v", err)
}

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Start heartbeat
err = syncManager.StartHeartbeat(ctx)
if err != nil {
t.Fatalf("Failed to start heartbeat: %v", err)
}

// Try to start again (should fail)
err = syncManager.StartHeartbeat(ctx)
if err == nil {
t.Fatal("Expected error when starting heartbeat twice")
}

// Cancel context to stop heartbeat
cancel()
time.Sleep(100 * time.Millisecond) // Give it time to stop
}

func TestVerifyAndApplySync(t *testing.T) {
setupTestEnvironment(t)
defer cleanupTestEnvironment(t)

tmpDir := t.TempDir()
os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

partition, err := NewEncryptedPartition(tmpDir)
if err != nil {
t.Fatalf("Failed to create partition: %v", err)
}

syncManager, err := createTestSyncManager(t, tmpDir)
if err != nil {
t.Fatalf("Failed to create sync manager: %v", err)
}

// Generate a valid mock quote
quote := generateValidMockQuote(t, []byte("test-data"))

// Add peer and update whitelist
peerID := common.BytesToHash([]byte("peer1"))
mrenclave := [32]byte{4, 5, 6}
syncManager.AddPeer(peerID, mrenclave, quote)
syncManager.UpdateAllowedEnclaves([][32]byte{mrenclave})

// Create a sync request first
requestID, err := syncManager.RequestSync(peerID, []SecretDataType{SecretTypePrivateKey})
if err != nil {
t.Fatalf("Failed to request sync: %v", err)
}

// Create sync response
response := &SyncResponse{
RequestID: requestID,
PeerID:    peerID,
Secrets: []SecretData{
{
ID:   []byte("secret1"),
Data: []byte("secret-data-1"),
},
},
Timestamp: uint64(time.Now().Unix()),
}

// Verify and apply
err = syncManager.VerifyAndApplySync(response)
if err != nil {
t.Fatalf("Failed to verify and apply sync: %v", err)
}

// Verify secret was written
data, err := partition.ReadSecret("secret1")
if err != nil {
t.Fatalf("Failed to read synced secret: %v", err)
}

	if string(data) != "secret-data-1" {
		t.Errorf("Expected 'secret-data-1', got %s", string(data))
	}
}

func TestCheckPeerHealth(t *testing.T) {
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	tmpDir := t.TempDir()
	os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
	defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

	syncManager, err := createTestSyncManager(t, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create sync manager: %v", err)
	}

	// Generate a valid mock quote
	quote := generateValidMockQuote(t, []byte("test-data"))

	// Add a peer with old sync time
	peerID := common.BytesToHash([]byte("peer1"))
	mrenclave := [32]byte{4, 5, 6}
	syncManager.AddPeer(peerID, mrenclave, quote)

	// Manually set the peer's sync status and last sync time to simulate old sync
	syncManager.mu.Lock()
	if peer, exists := syncManager.peers[peerID]; exists {
		peer.SyncStatus = SyncStatusCompleted
		peer.LastSync = uint64(time.Now().Unix()) - 3700 // More than 1 hour ago
	}
	syncManager.mu.Unlock()

	// Call checkPeerHealth
	syncManager.checkPeerHealth()

	// Verify the peer status changed to pending
	syncManager.mu.RLock()
	peer := syncManager.peers[peerID]
	syncManager.mu.RUnlock()

	if peer.SyncStatus != SyncStatusPending {
		t.Errorf("Expected peer status to be Pending after health check, got %v", peer.SyncStatus)
	}
}

func TestVerifyAndApplySync_InvalidRequestID(t *testing.T) {
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	tmpDir := t.TempDir()
	os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
	defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

	syncManager, err := createTestSyncManager(t, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create sync manager: %v", err)
	}

	// Create sync response with invalid request ID
	response := &SyncResponse{
		RequestID: common.BytesToHash([]byte("invalid")),
		PeerID:    common.BytesToHash([]byte("peer1")),
		Secrets:   []SecretData{},
		Timestamp: uint64(time.Now().Unix()),
	}

	// Should fail with invalid request ID
	err = syncManager.VerifyAndApplySync(response)
	if err == nil {
		t.Fatal("Expected error for invalid request ID")
	}
}

func TestVerifyAndApplySync_PeerNotInWhitelist(t *testing.T) {
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	tmpDir := t.TempDir()
	os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
	defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

	syncManager, err := createTestSyncManager(t, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create sync manager: %v", err)
	}

	// Generate a valid mock quote
	quote := generateValidMockQuote(t, []byte("test-data"))

	// Add peer but not to whitelist
	peerID := common.BytesToHash([]byte("peer1"))
	mrenclave := [32]byte{4, 5, 6}
	syncManager.AddPeer(peerID, mrenclave, quote)

	// Create a sync request first so there's a valid request ID
	syncManager.mu.Lock()
	requestID := common.BytesToHash([]byte("request1"))
	syncManager.syncRequests[requestID] = &SyncRequest{
		RequestID:   requestID,
		PeerID:      peerID,
		SecretTypes: []SecretDataType{SecretTypePrivateKey},
		Timestamp:   uint64(time.Now().Unix()),
	}
	syncManager.mu.Unlock()

	// Create sync response
	response := &SyncResponse{
		RequestID: requestID,
		PeerID:    peerID,
		Secrets:   []SecretData{},
		Timestamp: uint64(time.Now().Unix()),
	}

	// Should fail - peer not in whitelist
	err = syncManager.VerifyAndApplySync(response)
	if err == nil {
		t.Fatal("Expected error for peer not in whitelist")
	}
}

func TestHandleSyncRequest_PeerNotInWhitelist(t *testing.T) {
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	tmpDir := t.TempDir()
	os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
	defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

	syncManager, err := createTestSyncManager(t, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create sync manager: %v", err)
	}

	// Generate a valid mock quote
	quote := generateValidMockQuote(t, []byte("test-data"))

	// Add peer but not to whitelist
	peerID := common.BytesToHash([]byte("peer1"))
	mrenclave := [32]byte{4, 5, 6}
	syncManager.AddPeer(peerID, mrenclave, quote)

	// Create sync request
	request := &SyncRequest{
		RequestID:   common.BytesToHash([]byte("request1")),
		PeerID:      peerID,
		SecretTypes: []SecretDataType{SecretTypePrivateKey},
		Timestamp:   uint64(time.Now().Unix()),
	}

	// Should fail - peer not in whitelist
	_, err = syncManager.HandleSyncRequest(request)
	if err == nil {
		t.Fatal("Expected error for peer not in whitelist")
	}
}

func TestVerifyMREnclaveConstantTime_Mismatch(t *testing.T) {
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	tmpDir := t.TempDir()
	os.Setenv("GRAMINE_ENCRYPTED_PATHS", tmpDir)
	defer os.Unsetenv("GRAMINE_ENCRYPTED_PATHS")

	syncManager, err := createTestSyncManager(t, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create sync manager: %v", err)
	}

	// Test MRENCLAVE verification
	// Add an allowed enclave
	allowed := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	syncManager.UpdateAllowedEnclaves([][32]byte{allowed})

	// Test with matching MRENCLAVE
	if !syncManager.verifyMREnclaveConstantTime(allowed) {
		t.Error("Expected MRENCLAVE verification to succeed for allowed enclave")
	}

	// Test with non-matching MRENCLAVE
	notAllowed := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 99}
	if syncManager.verifyMREnclaveConstantTime(notAllowed) {
		t.Error("Expected MRENCLAVE verification to fail for non-allowed enclave")
	}
}


