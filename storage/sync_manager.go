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

	"github.com/ethereum/go-ethereum/common"
)

// SyncRequest represents a request to sync secret data
type SyncRequest struct {
	RequestID   common.Hash
	PeerID      common.Hash
	SecretTypes []SecretDataType
	Timestamp   uint64
}

// SyncResponse represents a response to a sync request
type SyncResponse struct {
	RequestID common.Hash
	PeerID    common.Hash
	Secrets   []SecretData
	Signature []byte
	Timestamp uint64
}

// SyncManager manages secret data synchronization between nodes
type SyncManager interface {
	// RequestSync initiates a sync request to a peer
	RequestSync(peerID common.Hash, secretTypes []SecretDataType) (common.Hash, error)

	// HandleSyncRequest processes an incoming sync request
	HandleSyncRequest(request *SyncRequest) (*SyncResponse, error)

	// VerifyAndApplySync verifies and applies a sync response
	VerifyAndApplySync(response *SyncResponse) error

	// AddPeer adds a new peer with its MRENCLAVE and quote
	AddPeer(peerID common.Hash, mrenclave [32]byte, quote []byte) error

	// RemovePeer removes a peer
	RemovePeer(peerID common.Hash) error

	// GetSyncStatus gets the sync status for a peer
	GetSyncStatus(peerID common.Hash) (SyncStatus, error)

	// StartHeartbeat starts the heartbeat mechanism
	StartHeartbeat(ctx context.Context) error
}
