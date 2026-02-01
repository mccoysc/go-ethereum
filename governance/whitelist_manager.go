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

package governance

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// InMemoryWhitelistManager implements WhitelistManager with in-memory storage
type InMemoryWhitelistManager struct {
	config  *WhitelistConfig
	mu      sync.RWMutex
	entries map[[32]byte]*MREnclaveEntry
	voting  VotingManager
}

// NewInMemoryWhitelistManager creates a new in-memory whitelist manager
func NewInMemoryWhitelistManager(config *WhitelistConfig, voting VotingManager) *InMemoryWhitelistManager {
	return &InMemoryWhitelistManager{
		config:  config,
		entries: make(map[[32]byte]*MREnclaveEntry),
		voting:  voting,
	}
}

// IsAllowed checks if an MRENCLAVE is allowed
func (wm *InMemoryWhitelistManager) IsAllowed(mrenclave [32]byte) bool {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	entry, exists := wm.entries[mrenclave]
	if !exists {
		return false
	}

	// Only active and approved entries are allowed
	return entry.Status == StatusActive || entry.Status == StatusApproved
}

// GetPermissionLevel returns the permission level of an MRENCLAVE
func (wm *InMemoryWhitelistManager) GetPermissionLevel(mrenclave [32]byte) PermissionLevel {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	entry, exists := wm.entries[mrenclave]
	if !exists {
		return PermissionBasic
	}

	return entry.PermissionLevel
}

// GetEntry returns the entry for an MRENCLAVE
func (wm *InMemoryWhitelistManager) GetEntry(mrenclave [32]byte) (*MREnclaveEntry, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	entry, exists := wm.entries[mrenclave]
	if !exists {
		return nil, ErrMREnclaveNotFound
	}

	// Return a copy to prevent external modification
	entryCopy := *entry
	return &entryCopy, nil
}

// GetAllEntries returns all entries
func (wm *InMemoryWhitelistManager) GetAllEntries() []*MREnclaveEntry {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	entries := make([]*MREnclaveEntry, 0, len(wm.entries))
	for _, entry := range wm.entries {
		entryCopy := *entry
		entries = append(entries, &entryCopy)
	}

	return entries
}

// ProposeAdd proposes adding a new MRENCLAVE
func (wm *InMemoryWhitelistManager) ProposeAdd(proposer common.Address, mrenclave [32]byte, version string) (common.Hash, error) {
	wm.mu.RLock()
	_, exists := wm.entries[mrenclave]
	wm.mu.RUnlock()

	if exists {
		return common.Hash{}, ErrMREnclaveNotFound // MRENCLAVE already exists, cannot add
	}

	// Create proposal
	proposal := &Proposal{
		Type:        ProposalAddMREnclave,
		Proposer:    proposer,
		Target:      mrenclave[:],
		Description: "Add MRENCLAVE version " + version,
	}

	return wm.voting.CreateProposal(proposal)
}

// ProposeRemove proposes removing an MRENCLAVE
func (wm *InMemoryWhitelistManager) ProposeRemove(proposer common.Address, mrenclave [32]byte, reason string) (common.Hash, error) {
	wm.mu.RLock()
	_, exists := wm.entries[mrenclave]
	wm.mu.RUnlock()

	if !exists {
		return common.Hash{}, ErrMREnclaveNotFound
	}

	// Create proposal
	proposal := &Proposal{
		Type:        ProposalRemoveMREnclave,
		Proposer:    proposer,
		Target:      mrenclave[:],
		Description: "Remove MRENCLAVE: " + reason,
	}

	return wm.voting.CreateProposal(proposal)
}

// ProposeUpgrade proposes upgrading the permission level of an MRENCLAVE
func (wm *InMemoryWhitelistManager) ProposeUpgrade(proposer common.Address, mrenclave [32]byte, newLevel PermissionLevel) (common.Hash, error) {
	wm.mu.RLock()
	entry, exists := wm.entries[mrenclave]
	wm.mu.RUnlock()

	if !exists {
		return common.Hash{}, ErrMREnclaveNotFound
	}

	if entry.PermissionLevel >= newLevel {
		return common.Hash{}, ErrInvalidPermissionLevel
	}

	// Create proposal with level as target
	target := make([]byte, 33)
	copy(target[:32], mrenclave[:])
	target[32] = byte(newLevel)

	proposal := &Proposal{
		Type:        ProposalUpgradePermission,
		Proposer:    proposer,
		Target:      target,
		Description: "Upgrade permission level",
	}

	return wm.voting.CreateProposal(proposal)
}

// AddEntry adds a new entry to the whitelist (internal use only)
func (wm *InMemoryWhitelistManager) AddEntry(entry *MREnclaveEntry) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	entryCopy := *entry
	wm.entries[entry.MRENCLAVE] = &entryCopy
}

// RemoveEntry removes an entry from the whitelist (internal use only)
func (wm *InMemoryWhitelistManager) RemoveEntry(mrenclave [32]byte) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if entry, exists := wm.entries[mrenclave]; exists {
		entry.Status = StatusDeprecated
	}
}
