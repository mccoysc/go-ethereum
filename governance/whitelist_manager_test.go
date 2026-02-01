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
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// MockVotingManager is a mock implementation of VotingManager for testing
type MockVotingManager struct {
	proposals     map[common.Hash]*Proposal
	createError   error
	lastProposalID common.Hash
}

func NewMockVotingManager() *MockVotingManager {
	return &MockVotingManager{
		proposals: make(map[common.Hash]*Proposal),
	}
}

func (m *MockVotingManager) CreateProposal(proposal *Proposal) (common.Hash, error) {
	if m.createError != nil {
		return common.Hash{}, m.createError
	}
	
	id := common.BytesToHash([]byte{byte(len(m.proposals))})
	proposal.ID = id
	m.proposals[id] = proposal
	m.lastProposalID = id
	return id, nil
}

func (m *MockVotingManager) Vote(proposalID common.Hash, voter common.Address, support bool, signature []byte) error {
	return nil
}

func (m *MockVotingManager) GetProposal(proposalID common.Hash) (*Proposal, error) {
	p, exists := m.proposals[proposalID]
	if !exists {
		return nil, ErrProposalNotFound
	}
	return p, nil
}

func (m *MockVotingManager) GetProposalVotes(proposalID common.Hash) ([]*Vote, error) {
	return nil, nil
}

func (m *MockVotingManager) ExecuteProposal(proposalID common.Hash) error {
	return nil
}

func (m *MockVotingManager) GetActiveProposals() []*Proposal {
	return nil
}

func (m *MockVotingManager) CheckProposalStatus(proposalID common.Hash, currentBlock uint64) error {
	return nil
}

func TestWhitelistManager_IsAllowed(t *testing.T) {
	config := DefaultWhitelistConfig()
	voting := NewMockVotingManager()
	wm := NewInMemoryWhitelistManager(config, voting)

	mrenclave := [32]byte{1, 2, 3}

	// Not allowed initially
	if wm.IsAllowed(mrenclave) {
		t.Error("MRENCLAVE should not be allowed initially")
	}

	// Add to whitelist as active
	entry := &MREnclaveEntry{
		MRENCLAVE:       mrenclave,
		Version:         "v1.0.0",
		Status:          StatusActive,
		PermissionLevel: PermissionFull,
		AddBy:           common.HexToAddress("0x1"),
		AddedAt:         100,
	}
	wm.AddEntry(entry)

	// Should be allowed now
	if !wm.IsAllowed(mrenclave) {
		t.Error("MRENCLAVE should be allowed after adding")
	}

	// Mark as deprecated
	entry.Status = StatusDeprecated
	wm.AddEntry(entry)

	// Should not be allowed when deprecated
	if wm.IsAllowed(mrenclave) {
		t.Error("MRENCLAVE should not be allowed when deprecated")
	}
}

func TestWhitelistManager_GetPermissionLevel(t *testing.T) {
	config := DefaultWhitelistConfig()
	voting := NewMockVotingManager()
	wm := NewInMemoryWhitelistManager(config, voting)

	mrenclave := [32]byte{1, 2, 3}

	// Default permission level
	level := wm.GetPermissionLevel(mrenclave)
	if level != PermissionBasic {
		t.Errorf("expected default permission %v, got %v", PermissionBasic, level)
	}

	// Add with Standard permission
	entry := &MREnclaveEntry{
		MRENCLAVE:       mrenclave,
		Version:         "v1.0.0",
		Status:          StatusActive,
		PermissionLevel: PermissionStandard,
		AddBy:           common.HexToAddress("0x1"),
		AddedAt:         100,
	}
	wm.AddEntry(entry)

	level = wm.GetPermissionLevel(mrenclave)
	if level != PermissionStandard {
		t.Errorf("expected permission %v, got %v", PermissionStandard, level)
	}
}

func TestWhitelistManager_GetEntry(t *testing.T) {
	config := DefaultWhitelistConfig()
	voting := NewMockVotingManager()
	wm := NewInMemoryWhitelistManager(config, voting)

	mrenclave := [32]byte{1, 2, 3}

	// Entry not found
	_, err := wm.GetEntry(mrenclave)
	if err != ErrMREnclaveNotFound {
		t.Errorf("expected error %v, got %v", ErrMREnclaveNotFound, err)
	}

	// Add entry
	entry := &MREnclaveEntry{
		MRENCLAVE:       mrenclave,
		Version:         "v1.0.0",
		Status:          StatusActive,
		PermissionLevel: PermissionFull,
		AddBy:           common.HexToAddress("0x1"),
		AddedAt:         100,
	}
	wm.AddEntry(entry)

	// Get entry
	retrieved, err := wm.GetEntry(mrenclave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.MRENCLAVE != mrenclave {
		t.Error("MRENCLAVE mismatch")
	}
	if retrieved.Version != "v1.0.0" {
		t.Error("version mismatch")
	}
	if retrieved.Status != StatusActive {
		t.Error("status mismatch")
	}
}

func TestWhitelistManager_GetAllEntries(t *testing.T) {
	config := DefaultWhitelistConfig()
	voting := NewMockVotingManager()
	wm := NewInMemoryWhitelistManager(config, voting)

	// Add multiple entries
	mr1 := [32]byte{1}
	mr2 := [32]byte{2}
	mr3 := [32]byte{3}

	wm.AddEntry(&MREnclaveEntry{
		MRENCLAVE: mr1,
		Version:   "v1",
		Status:    StatusActive,
	})
	wm.AddEntry(&MREnclaveEntry{
		MRENCLAVE: mr2,
		Version:   "v2",
		Status:    StatusActive,
	})
	wm.AddEntry(&MREnclaveEntry{
		MRENCLAVE: mr3,
		Version:   "v3",
		Status:    StatusDeprecated,
	})

	entries := wm.GetAllEntries()
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestWhitelistManager_ProposeAdd(t *testing.T) {
	config := DefaultWhitelistConfig()
	voting := NewMockVotingManager()
	wm := NewInMemoryWhitelistManager(config, voting)

	proposer := common.HexToAddress("0x1")
	mrenclave := [32]byte{1, 2, 3}

	// Create proposal
	proposalID, err := wm.ProposeAdd(proposer, mrenclave, "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check proposal was created
	proposal, err := voting.GetProposal(proposalID)
	if err != nil {
		t.Fatalf("proposal not found: %v", err)
	}

	if proposal.Type != ProposalAddMREnclave {
		t.Errorf("expected type %v, got %v", ProposalAddMREnclave, proposal.Type)
	}
	if proposal.Proposer != proposer {
		t.Error("proposer mismatch")
	}
}

func TestWhitelistManager_ProposeRemove(t *testing.T) {
	config := DefaultWhitelistConfig()
	voting := NewMockVotingManager()
	wm := NewInMemoryWhitelistManager(config, voting)

	proposer := common.HexToAddress("0x1")
	mrenclave := [32]byte{1, 2, 3}

	// Add entry first
	wm.AddEntry(&MREnclaveEntry{
		MRENCLAVE: mrenclave,
		Version:   "v1",
		Status:    StatusActive,
	})

	// Create removal proposal
	proposalID, err := wm.ProposeRemove(proposer, mrenclave, "outdated")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check proposal was created
	proposal, err := voting.GetProposal(proposalID)
	if err != nil {
		t.Fatalf("proposal not found: %v", err)
	}

	if proposal.Type != ProposalRemoveMREnclave {
		t.Errorf("expected type %v, got %v", ProposalRemoveMREnclave, proposal.Type)
	}
}

func TestWhitelistManager_ProposeUpgrade(t *testing.T) {
	config := DefaultWhitelistConfig()
	voting := NewMockVotingManager()
	wm := NewInMemoryWhitelistManager(config, voting)

	proposer := common.HexToAddress("0x1")
	mrenclave := [32]byte{1, 2, 3}

	// Add entry with Basic permission
	wm.AddEntry(&MREnclaveEntry{
		MRENCLAVE:       mrenclave,
		Version:         "v1",
		Status:          StatusActive,
		PermissionLevel: PermissionBasic,
	})

	// Propose upgrade to Standard
	proposalID, err := wm.ProposeUpgrade(proposer, mrenclave, PermissionStandard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check proposal was created
	proposal, err := voting.GetProposal(proposalID)
	if err != nil {
		t.Fatalf("proposal not found: %v", err)
	}

	if proposal.Type != ProposalUpgradePermission {
		t.Errorf("expected type %v, got %v", ProposalUpgradePermission, proposal.Type)
	}

	// Try to downgrade (should fail)
	_, err = wm.ProposeUpgrade(proposer, mrenclave, PermissionBasic)
	if err != ErrInvalidPermissionLevel {
		t.Errorf("expected error %v, got %v", ErrInvalidPermissionLevel, err)
	}
}

func TestWhitelistManager_RemoveEntry(t *testing.T) {
	config := DefaultWhitelistConfig()
	voting := NewMockVotingManager()
	wm := NewInMemoryWhitelistManager(config, voting)

	mrenclave := [32]byte{1, 2, 3}

	// Add entry
	wm.AddEntry(&MREnclaveEntry{
		MRENCLAVE: mrenclave,
		Version:   "v1",
		Status:    StatusActive,
	})

	// Verify it's active
	if !wm.IsAllowed(mrenclave) {
		t.Error("entry should be allowed")
	}

	// Remove entry
	wm.RemoveEntry(mrenclave)

	// Verify it's deprecated
	if wm.IsAllowed(mrenclave) {
		t.Error("entry should not be allowed after removal")
	}

	entry, _ := wm.GetEntry(mrenclave)
	if entry.Status != StatusDeprecated {
		t.Errorf("expected status %v, got %v", StatusDeprecated, entry.Status)
	}
}

func TestWhitelistManager_ConcurrentAccess(t *testing.T) {
	config := DefaultWhitelistConfig()
	voting := NewMockVotingManager()
	wm := NewInMemoryWhitelistManager(config, voting)

	// Test concurrent reads and writes
	done := make(chan bool)
	
	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			mr := [32]byte{byte(i)}
			wm.AddEntry(&MREnclaveEntry{
				MRENCLAVE: mr,
				Version:   "v1",
				Status:    StatusActive,
			})
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			mr := [32]byte{byte(i)}
			wm.IsAllowed(mr)
			wm.GetPermissionLevel(mr)
		}
		done <- true
	}()

	// Wait for both to complete
	<-done
	<-done

	// Verify entries were added
	entries := wm.GetAllEntries()
	if len(entries) != 100 {
		t.Errorf("expected 100 entries, got %d", len(entries))
	}
}
