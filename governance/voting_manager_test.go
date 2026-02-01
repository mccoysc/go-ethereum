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
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// MockValidatorManager is a mock implementation for testing
type MockValidatorManager struct {
	validators map[common.Address]*ValidatorInfo
}

func NewMockValidatorManager() *MockValidatorManager {
	return &MockValidatorManager{
		validators: make(map[common.Address]*ValidatorInfo),
	}
}

func (m *MockValidatorManager) AddMockValidator(addr common.Address, vtype VoterType, votingPower uint64) {
	m.validators[addr] = &ValidatorInfo{
		Address:     addr,
		Type:        vtype,
		VotingPower: votingPower,
		Status:      ValidatorStatusActive,
		StakeAmount: big.NewInt(10000),
	}
}

func (m *MockValidatorManager) GetValidator(addr common.Address) (*ValidatorInfo, error) {
	v, exists := m.validators[addr]
	if !exists {
		return nil, ErrValidatorNotFound
	}
	return v, nil
}

func (m *MockValidatorManager) GetAllValidators() []*ValidatorInfo {
	result := make([]*ValidatorInfo, 0, len(m.validators))
	for _, v := range m.validators {
		result = append(result, v)
	}
	return result
}

func (m *MockValidatorManager) GetCoreValidators() []*ValidatorInfo {
	result := make([]*ValidatorInfo, 0)
	for _, v := range m.validators {
		if v.Type == VoterTypeCore && v.Status == ValidatorStatusActive {
			result = append(result, v)
		}
	}
	return result
}

func (m *MockValidatorManager) GetCommunityValidators() []*ValidatorInfo {
	result := make([]*ValidatorInfo, 0)
	for _, v := range m.validators {
		if v.Type == VoterTypeCommunity && v.Status == ValidatorStatusActive {
			result = append(result, v)
		}
	}
	return result
}

func (m *MockValidatorManager) IsValidator(addr common.Address) bool {
	_, exists := m.validators[addr]
	return exists
}

func (m *MockValidatorManager) GetVoterType(addr common.Address) VoterType {
	v, exists := m.validators[addr]
	if !exists {
		return VoterTypeCommunity
	}
	return v.Type
}

func (m *MockValidatorManager) Stake(addr common.Address, amount *big.Int) error {
	return nil
}

func (m *MockValidatorManager) Unstake(addr common.Address, amount *big.Int) error {
	return nil
}

func (m *MockValidatorManager) ClaimRewards(addr common.Address) (*big.Int, error) {
	return big.NewInt(0), nil
}

func (m *MockValidatorManager) Slash(addr common.Address, reason string) error {
	return nil
}

func (m *MockValidatorManager) UpdateMREnclave(addr common.Address, newMREnclave [32]byte) error {
	return nil
}

func TestVotingManager_CreateProposal(t *testing.T) {
	config := DefaultWhitelistConfig()
	validators := NewMockValidatorManager()
	vm := NewInMemoryVotingManager(config, validators)

	proposer := common.HexToAddress("0x1")
	proposal := &Proposal{
		Type:        ProposalAddMREnclave,
		Proposer:    proposer,
		Target:      []byte{1, 2, 3},
		Description: "Test proposal",
		CreatedAt:   100,
	}

	proposalID, err := vm.CreateProposal(proposal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Retrieve proposal
	retrieved, err := vm.GetProposal(proposalID)
	if err != nil {
		t.Fatalf("failed to get proposal: %v", err)
	}

	if retrieved.Status != ProposalStatusPending {
		t.Errorf("expected status %v, got %v", ProposalStatusPending, retrieved.Status)
	}

	if retrieved.VotingEndsAt != 100+config.VotingPeriod {
		t.Error("voting end time not set correctly")
	}

	if retrieved.ExecuteAfter != 100+config.VotingPeriod+config.ExecutionDelay {
		t.Error("execution time not set correctly")
	}
}

func TestVotingManager_Vote(t *testing.T) {
	config := DefaultWhitelistConfig()
	validators := NewMockValidatorManager()
	vm := NewInMemoryVotingManager(config, validators)

	// Add core validators
	core1 := common.HexToAddress("0x1")
	core2 := common.HexToAddress("0x2")
	validators.AddMockValidator(core1, VoterTypeCore, 1)
	validators.AddMockValidator(core2, VoterTypeCore, 1)

	// Create proposal
	proposal := &Proposal{
		Type:      ProposalAddMREnclave,
		Proposer:  core1,
		Target:    []byte{1, 2, 3},
		CreatedAt: 100,
	}
	proposalID, _ := vm.CreateProposal(proposal)

	// Vote
	err := vm.Vote(proposalID, core1, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify vote was recorded
	retrieved, _ := vm.GetProposal(proposalID)
	if retrieved.CoreYesVotes != 1 {
		t.Errorf("expected 1 yes vote, got %d", retrieved.CoreYesVotes)
	}

	// Try to vote again (should fail)
	err = vm.Vote(proposalID, core1, false, nil)
	if err != ErrAlreadyVoted {
		t.Errorf("expected error %v, got %v", ErrAlreadyVoted, err)
	}

	// Vote from non-validator (should fail)
	nonValidator := common.HexToAddress("0x999")
	err = vm.Vote(proposalID, nonValidator, true, nil)
	if err != ErrInvalidVoter {
		t.Errorf("expected error %v, got %v", ErrInvalidVoter, err)
	}
}

func TestVotingManager_VoteWithSignature(t *testing.T) {
	config := DefaultWhitelistConfig()
	validators := NewMockValidatorManager()
	vm := NewInMemoryVotingManager(config, validators)

	// Generate key for validator
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)
	validators.AddMockValidator(addr, VoterTypeCore, 1)

	// Create proposal
	proposal := &Proposal{
		Type:      ProposalAddMREnclave,
		Proposer:  addr,
		Target:    []byte{1, 2, 3},
		CreatedAt: 100,
	}
	proposalID, _ := vm.CreateProposal(proposal)

	// Create signature
	support := true
	voteData := boolToBytes(support)
	voteData = append(proposalID[:], voteData...)
	hash := crypto.Keccak256Hash(voteData)
	signature, _ := crypto.Sign(hash.Bytes(), key)

	// Vote with signature
	err := vm.Vote(proposalID, addr, support, signature)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify vote
	votes, _ := vm.GetProposalVotes(proposalID)
	if len(votes) != 1 {
		t.Fatalf("expected 1 vote, got %d", len(votes))
	}
	if votes[0].Support != support {
		t.Error("vote support mismatch")
	}
}

func TestVotingManager_CheckProposalStatus(t *testing.T) {
	config := DefaultWhitelistConfig()
	validators := NewMockValidatorManager()
	vm := NewInMemoryVotingManager(config, validators)

	// Add 6 core validators (need 2/3 = 67%, so 4 yes votes needed)
	core1 := common.HexToAddress("0x1")
	core2 := common.HexToAddress("0x2")
	core3 := common.HexToAddress("0x3")
	core4 := common.HexToAddress("0x4")
	core5 := common.HexToAddress("0x5")
	core6 := common.HexToAddress("0x6")
	validators.AddMockValidator(core1, VoterTypeCore, 1)
	validators.AddMockValidator(core2, VoterTypeCore, 1)
	validators.AddMockValidator(core3, VoterTypeCore, 1)
	validators.AddMockValidator(core4, VoterTypeCore, 1)
	validators.AddMockValidator(core5, VoterTypeCore, 1)
	validators.AddMockValidator(core6, VoterTypeCore, 1)

	// Create proposal
	proposal := &Proposal{
		Type:      ProposalAddMREnclave,
		Proposer:  core1,
		Target:    []byte{1, 2, 3},
		CreatedAt: 100,
	}
	proposalID, _ := vm.CreateProposal(proposal)

	// Vote yes from 4 validators (4/6 = 66.67%, rounds to 66% in integer division, need to vote 5 for 83%)
	// Actually, 4*100/6 = 400/6 = 66, which is < 67, so need 5 votes
	vm.Vote(proposalID, core1, true, nil)
	vm.Vote(proposalID, core2, true, nil)
	vm.Vote(proposalID, core3, true, nil)
	vm.Vote(proposalID, core4, true, nil)
	vm.Vote(proposalID, core5, true, nil) // 5 votes = 83%

	// Check status after voting period ends
	currentBlock := 100 + config.VotingPeriod + 1
	err := vm.CheckProposalStatus(proposalID, currentBlock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be passed
	retrieved, _ := vm.GetProposal(proposalID)
	if retrieved.Status != ProposalStatusPassed {
		t.Errorf("expected status %v, got %v", ProposalStatusPassed, retrieved.Status)
	}
}

func TestVotingManager_CheckProposalStatus_Rejected(t *testing.T) {
	config := DefaultWhitelistConfig()
	validators := NewMockValidatorManager()
	vm := NewInMemoryVotingManager(config, validators)

	// Add 3 core validators
	core1 := common.HexToAddress("0x1")
	core2 := common.HexToAddress("0x2")
	core3 := common.HexToAddress("0x3")
	validators.AddMockValidator(core1, VoterTypeCore, 1)
	validators.AddMockValidator(core2, VoterTypeCore, 1)
	validators.AddMockValidator(core3, VoterTypeCore, 1)

	// Create proposal
	proposal := &Proposal{
		Type:      ProposalAddMREnclave,
		Proposer:  core1,
		Target:    []byte{1, 2, 3},
		CreatedAt: 100,
	}
	proposalID, _ := vm.CreateProposal(proposal)

	// Vote no from 2 validators
	vm.Vote(proposalID, core1, false, nil)
	vm.Vote(proposalID, core2, false, nil)

	// Check status after voting period ends
	currentBlock := 100 + config.VotingPeriod + 1
	vm.CheckProposalStatus(proposalID, currentBlock)

	// Should be rejected
	retrieved, _ := vm.GetProposal(proposalID)
	if retrieved.Status != ProposalStatusRejected {
		t.Errorf("expected status %v, got %v", ProposalStatusRejected, retrieved.Status)
	}
}

func TestVotingManager_CommunityVeto(t *testing.T) {
	config := DefaultWhitelistConfig()
	validators := NewMockValidatorManager()
	vm := NewInMemoryVotingManager(config, validators)

	// Add 3 core validators (all vote yes)
	core1 := common.HexToAddress("0x1")
	core2 := common.HexToAddress("0x2")
	core3 := common.HexToAddress("0x3")
	validators.AddMockValidator(core1, VoterTypeCore, 1)
	validators.AddMockValidator(core2, VoterTypeCore, 1)
	validators.AddMockValidator(core3, VoterTypeCore, 1)

	// Add 3 community validators (all vote no - 100% rejection)
	comm1 := common.HexToAddress("0x11")
	comm2 := common.HexToAddress("0x12")
	comm3 := common.HexToAddress("0x13")
	validators.AddMockValidator(comm1, VoterTypeCommunity, 1)
	validators.AddMockValidator(comm2, VoterTypeCommunity, 1)
	validators.AddMockValidator(comm3, VoterTypeCommunity, 1)

	// Create proposal
	proposal := &Proposal{
		Type:      ProposalAddMREnclave,
		Proposer:  core1,
		Target:    []byte{1, 2, 3},
		CreatedAt: 100,
	}
	proposalID, _ := vm.CreateProposal(proposal)

	// Core validators vote yes (100%)
	vm.Vote(proposalID, core1, true, nil)
	vm.Vote(proposalID, core2, true, nil)
	vm.Vote(proposalID, core3, true, nil)

	// Community validators vote no (100% veto)
	vm.Vote(proposalID, comm1, false, nil)
	vm.Vote(proposalID, comm2, false, nil)
	vm.Vote(proposalID, comm3, false, nil)

	// Check status after voting period ends
	currentBlock := 100 + config.VotingPeriod + 1
	vm.CheckProposalStatus(proposalID, currentBlock)

	// Should be rejected due to community veto
	retrieved, _ := vm.GetProposal(proposalID)
	if retrieved.Status != ProposalStatusRejected {
		t.Errorf("expected status %v (vetoed), got %v", ProposalStatusRejected, retrieved.Status)
	}
}

func TestVotingManager_ExecuteProposal(t *testing.T) {
	config := DefaultWhitelistConfig()
	validators := NewMockValidatorManager()
	vm := NewInMemoryVotingManager(config, validators)

	proposer := common.HexToAddress("0x1")
	proposal := &Proposal{
		Type:      ProposalAddMREnclave,
		Proposer:  proposer,
		Target:    []byte{1, 2, 3},
		CreatedAt: 100,
	}
	proposalID, _ := vm.CreateProposal(proposal)

	// Manually set status to passed for testing
	vm.mu.Lock()
	vm.proposals[proposalID].Status = ProposalStatusPassed
	vm.mu.Unlock()

	// Execute
	err := vm.ExecuteProposal(proposalID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be executed
	retrieved, _ := vm.GetProposal(proposalID)
	if retrieved.Status != ProposalStatusExecuted {
		t.Errorf("expected status %v, got %v", ProposalStatusExecuted, retrieved.Status)
	}

	// Try to execute again (should fail)
	err = vm.ExecuteProposal(proposalID)
	if err != ErrProposalAlreadyExecuted {
		t.Errorf("expected error %v, got %v", ErrProposalAlreadyExecuted, err)
	}
}

func TestVotingManager_GetActiveProposals(t *testing.T) {
	config := DefaultWhitelistConfig()
	validators := NewMockValidatorManager()
	vm := NewInMemoryVotingManager(config, validators)

	proposer := common.HexToAddress("0x1")

	// Create 3 proposals with different statuses
	p1 := &Proposal{Type: ProposalAddMREnclave, Proposer: proposer, Target: []byte{1}, CreatedAt: 100}
	p2 := &Proposal{Type: ProposalAddMREnclave, Proposer: proposer, Target: []byte{2}, CreatedAt: 100}
	p3 := &Proposal{Type: ProposalAddMREnclave, Proposer: proposer, Target: []byte{3}, CreatedAt: 100}

	id1, _ := vm.CreateProposal(p1)
	id2, _ := vm.CreateProposal(p2)
	id3, _ := vm.CreateProposal(p3)

	// Set different statuses
	vm.proposals[id1].Status = ProposalStatusPending
	vm.proposals[id2].Status = ProposalStatusPassed
	vm.proposals[id3].Status = ProposalStatusExecuted

	// Get active proposals
	active := vm.GetActiveProposals()

	// Should return pending and passed (2 proposals)
	if len(active) != 2 {
		t.Errorf("expected 2 active proposals, got %d", len(active))
	}
}

func TestVotingManager_EmergencyUpgrade_RequiresUnanimous(t *testing.T) {
	config := DefaultWhitelistConfig()
	validators := NewMockValidatorManager()
	vm := NewInMemoryVotingManager(config, validators)

	// Add 3 core validators
	core1 := common.HexToAddress("0x1")
	core2 := common.HexToAddress("0x2")
	core3 := common.HexToAddress("0x3")
	validators.AddMockValidator(core1, VoterTypeCore, 1)
	validators.AddMockValidator(core2, VoterTypeCore, 1)
	validators.AddMockValidator(core3, VoterTypeCore, 1)

	// Create emergency upgrade proposal
	proposal := &Proposal{
		Type:      ProposalEmergencyUpgrade,
		Proposer:  core1,
		Target:    []byte{1, 2, 3},
		CreatedAt: 100,
	}
	proposalID, _ := vm.CreateProposal(proposal)

	// Only 2 out of 3 vote yes (not unanimous)
	vm.Vote(proposalID, core1, true, nil)
	vm.Vote(proposalID, core2, true, nil)

	// Check status after voting period ends
	currentBlock := 100 + config.VotingPeriod + 1
	vm.CheckProposalStatus(proposalID, currentBlock)

	// Should be rejected (not 100%)
	retrieved, _ := vm.GetProposal(proposalID)
	if retrieved.Status != ProposalStatusRejected {
		t.Errorf("emergency upgrade without unanimous vote should be rejected, got %v", retrieved.Status)
	}
}

func TestVotingManager_EmergencyUpgrade_Unanimous(t *testing.T) {
	config := DefaultWhitelistConfig()
	validators := NewMockValidatorManager()
	vm := NewInMemoryVotingManager(config, validators)

	// Add 3 core validators
	core1 := common.HexToAddress("0x1")
	core2 := common.HexToAddress("0x2")
	core3 := common.HexToAddress("0x3")
	validators.AddMockValidator(core1, VoterTypeCore, 1)
	validators.AddMockValidator(core2, VoterTypeCore, 1)
	validators.AddMockValidator(core3, VoterTypeCore, 1)

	// Create emergency upgrade proposal
	proposal := &Proposal{
		Type:      ProposalEmergencyUpgrade,
		Proposer:  core1,
		Target:    []byte{1, 2, 3},
		CreatedAt: 100,
	}
	proposalID, _ := vm.CreateProposal(proposal)

	// All 3 vote yes (unanimous)
	vm.Vote(proposalID, core1, true, nil)
	vm.Vote(proposalID, core2, true, nil)
	vm.Vote(proposalID, core3, true, nil)

	// Check status after voting period ends
	currentBlock := 100 + config.VotingPeriod + 1
	vm.CheckProposalStatus(proposalID, currentBlock)

	// Should pass
	retrieved, _ := vm.GetProposal(proposalID)
	if retrieved.Status != ProposalStatusPassed {
		t.Errorf("emergency upgrade with unanimous vote should pass, got %v", retrieved.Status)
	}
}

func TestVotingManager_EmergencyUpgrade_StricterVeto(t *testing.T) {
	config := DefaultWhitelistConfig()
	validators := NewMockValidatorManager()
	vm := NewInMemoryVotingManager(config, validators)

	// Add 3 core validators (all vote yes)
	core1 := common.HexToAddress("0x1")
	core2 := common.HexToAddress("0x2")
	core3 := common.HexToAddress("0x3")
	validators.AddMockValidator(core1, VoterTypeCore, 1)
	validators.AddMockValidator(core2, VoterTypeCore, 1)
	validators.AddMockValidator(core3, VoterTypeCore, 1)

	// Add 4 community validators
	comm1 := common.HexToAddress("0x11")
	comm2 := common.HexToAddress("0x12")
	comm3 := common.HexToAddress("0x13")
	comm4 := common.HexToAddress("0x14")
	validators.AddMockValidator(comm1, VoterTypeCommunity, 1)
	validators.AddMockValidator(comm2, VoterTypeCommunity, 1)
	validators.AddMockValidator(comm3, VoterTypeCommunity, 1)
	validators.AddMockValidator(comm4, VoterTypeCommunity, 1)

	// Create emergency upgrade proposal
	proposal := &Proposal{
		Type:      ProposalEmergencyUpgrade,
		Proposer:  core1,
		Target:    []byte{1, 2, 3},
		CreatedAt: 100,
	}
	proposalID, _ := vm.CreateProposal(proposal)

	// All core validators vote yes
	vm.Vote(proposalID, core1, true, nil)
	vm.Vote(proposalID, core2, true, nil)
	vm.Vote(proposalID, core3, true, nil)

	// 2 out of 4 community validators vote no (50% - should veto)
	vm.Vote(proposalID, comm1, false, nil)
	vm.Vote(proposalID, comm2, false, nil)

	// Check status after voting period ends
	currentBlock := 100 + config.VotingPeriod + 1
	vm.CheckProposalStatus(proposalID, currentBlock)

	// Should be rejected due to 1/2 community veto
	retrieved, _ := vm.GetProposal(proposalID)
	if retrieved.Status != ProposalStatusRejected {
		t.Errorf("emergency upgrade should be vetoed by 50%% community, got %v", retrieved.Status)
	}
}
