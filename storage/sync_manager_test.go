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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// MockAttestor is a mock implementation of sgx.Attestor for testing
type MockAttestor struct {
	mrenclave [32]byte
}

func (m *MockAttestor) GetMREnclave() []byte {
	return m.mrenclave[:]
}

func (m *MockAttestor) GetMRSigner() []byte {
	return make([]byte, 32)
}

func (m *MockAttestor) GenerateQuote(reportData []byte) ([]byte, error) {
	return []byte("mock-quote"), nil
}

func (m *MockAttestor) GenerateCertificate() (*tls.Certificate, error) {
	return &tls.Certificate{}, nil
}

// MockVerifier is a mock implementation of sgx.Verifier for testing
type MockVerifier struct {
	shouldPass bool
}

func (m *MockVerifier) VerifyQuote(quote []byte) error {
	if !m.shouldPass {
		return fmt.Errorf("quote verification failed")
	}
	return nil
}

func (m *MockVerifier) VerifyCertificate(cert *x509.Certificate) error {
	if !m.shouldPass {
		return fmt.Errorf("certificate verification failed")
	}
	return nil
}

func (m *MockVerifier) IsAllowedMREnclave(mrenclave []byte) bool {
	return m.shouldPass
}

func (m *MockVerifier) AddAllowedMREnclave(mrenclave []byte) {
}

func (m *MockVerifier) RemoveAllowedMREnclave(mrenclave []byte) {
}

func TestNewSyncManager(t *testing.T) {
	tmpDir := t.TempDir()

	partition, err := NewEncryptedPartition(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create partition: %v", err)
	}

	attestor := &MockAttestor{mrenclave: [32]byte{1, 2, 3}}
	verifier := &MockVerifier{shouldPass: true}

	syncManager, err := NewSyncManager(partition, attestor, verifier)
	if err != nil {
		t.Fatalf("Failed to create sync manager: %v", err)
	}

	if syncManager == nil {
		t.Fatal("Sync manager is nil")
	}
}

func TestAddPeer(t *testing.T) {
	tmpDir := t.TempDir()

	partition, err := NewEncryptedPartition(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create partition: %v", err)
	}

	attestor := &MockAttestor{mrenclave: [32]byte{1, 2, 3}}
	verifier := &MockVerifier{shouldPass: true}

	syncManager, err := NewSyncManager(partition, attestor, verifier)
	if err != nil {
		t.Fatalf("Failed to create sync manager: %v", err)
	}

	// Add a peer
	peerID := common.BytesToHash([]byte("peer1"))
	mrenclave := [32]byte{4, 5, 6}
	quote := []byte("test-quote")

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
	tmpDir := t.TempDir()

	partition, err := NewEncryptedPartition(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create partition: %v", err)
	}

	attestor := &MockAttestor{mrenclave: [32]byte{1, 2, 3}}
	verifier := &MockVerifier{shouldPass: false} // Fail quote verification

	syncManager, err := NewSyncManager(partition, attestor, verifier)
	if err != nil {
		t.Fatalf("Failed to create sync manager: %v", err)
	}

	peerID := common.BytesToHash([]byte("peer1"))
	mrenclave := [32]byte{4, 5, 6}
	quote := []byte("invalid-quote")

	err = syncManager.AddPeer(peerID, mrenclave, quote)
	if err == nil {
		t.Fatal("Expected error for invalid quote")
	}
}

func TestRemovePeer(t *testing.T) {
	tmpDir := t.TempDir()

	partition, err := NewEncryptedPartition(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create partition: %v", err)
	}

	attestor := &MockAttestor{mrenclave: [32]byte{1, 2, 3}}
	verifier := &MockVerifier{shouldPass: true}

	syncManager, err := NewSyncManager(partition, attestor, verifier)
	if err != nil {
		t.Fatalf("Failed to create sync manager: %v", err)
	}

	peerID := common.BytesToHash([]byte("peer1"))
	mrenclave := [32]byte{4, 5, 6}
	quote := []byte("test-quote")

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
	tmpDir := t.TempDir()

	partition, err := NewEncryptedPartition(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create partition: %v", err)
	}

	attestor := &MockAttestor{mrenclave: [32]byte{1, 2, 3}}
	verifier := &MockVerifier{shouldPass: true}

	syncManager, err := NewSyncManager(partition, attestor, verifier)
	if err != nil {
		t.Fatalf("Failed to create sync manager: %v", err)
	}

	// Add peer to whitelist
	peerID := common.BytesToHash([]byte("peer1"))
	mrenclave := [32]byte{4, 5, 6}
	syncManager.AddPeer(peerID, mrenclave, []byte("quote"))
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
	tmpDir := t.TempDir()

	partition, err := NewEncryptedPartition(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create partition: %v", err)
	}

	attestor := &MockAttestor{mrenclave: [32]byte{1, 2, 3}}
	verifier := &MockVerifier{shouldPass: true}

	syncManager, err := NewSyncManager(partition, attestor, verifier)
	if err != nil {
		t.Fatalf("Failed to create sync manager: %v", err)
	}

	// Add peer but don't add to whitelist
	peerID := common.BytesToHash([]byte("peer1"))
	mrenclave := [32]byte{4, 5, 6}
	syncManager.AddPeer(peerID, mrenclave, []byte("quote"))

	// Request sync should fail
	_, err = syncManager.RequestSync(peerID, []SecretDataType{SecretTypePrivateKey})
	if err == nil {
		t.Fatal("Expected error for peer not in whitelist")
	}
}

func TestHandleSyncRequest(t *testing.T) {
	tmpDir := t.TempDir()

	partition, err := NewEncryptedPartition(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create partition: %v", err)
	}

	// Write some test secrets
	partition.WriteSecret("secret1", []byte("data1"))
	partition.WriteSecret("secret2", []byte("data2"))

	attestor := &MockAttestor{mrenclave: [32]byte{1, 2, 3}}
	verifier := &MockVerifier{shouldPass: true}

	syncManager, err := NewSyncManager(partition, attestor, verifier)
	if err != nil {
		t.Fatalf("Failed to create sync manager: %v", err)
	}

	// Add peer to whitelist
	peerID := common.BytesToHash([]byte("peer1"))
	mrenclave := [32]byte{4, 5, 6}
	syncManager.AddPeer(peerID, mrenclave, []byte("quote"))
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
	tmpDir := t.TempDir()

	partition, err := NewEncryptedPartition(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create partition: %v", err)
	}

	attestor := &MockAttestor{mrenclave: [32]byte{1, 2, 3}}
	verifier := &MockVerifier{shouldPass: true}

	syncManager, err := NewSyncManager(partition, attestor, verifier)
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
	tmpDir := t.TempDir()

	partition, err := NewEncryptedPartition(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create partition: %v", err)
	}

	attestor := &MockAttestor{mrenclave: [32]byte{1, 2, 3}}
	verifier := &MockVerifier{shouldPass: true}

	syncManager, err := NewSyncManager(partition, attestor, verifier)
	if err != nil {
		t.Fatalf("Failed to create sync manager: %v", err)
	}

	// Add peer and update whitelist
	peerID := common.BytesToHash([]byte("peer1"))
	mrenclave := [32]byte{4, 5, 6}
	syncManager.AddPeer(peerID, mrenclave, []byte("quote"))
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
