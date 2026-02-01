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
	"crypto/sha256"
	"encoding/binary"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// InMemoryVotingManager implements VotingManager with in-memory storage
type InMemoryVotingManager struct {
	config     *WhitelistConfig
	mu         sync.RWMutex
	proposals  map[common.Hash]*Proposal
	votes      map[common.Hash][]*Vote
	validators ValidatorManager
}

// NewInMemoryVotingManager creates a new in-memory voting manager
func NewInMemoryVotingManager(config *WhitelistConfig, validators ValidatorManager) *InMemoryVotingManager {
	return &InMemoryVotingManager{
		config:     config,
		proposals:  make(map[common.Hash]*Proposal),
		votes:      make(map[common.Hash][]*Vote),
		validators: validators,
	}
}

// CreateProposal creates a new proposal
func (vm *InMemoryVotingManager) CreateProposal(proposal *Proposal) (common.Hash, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Generate proposal ID from hash of type, proposer, target, and timestamp
	idData := make([]byte, 1+20+len(proposal.Target)+8)
	idData[0] = byte(proposal.Type)
	copy(idData[1:21], proposal.Proposer[:])
	copy(idData[21:21+len(proposal.Target)], proposal.Target)
	binary.BigEndian.PutUint64(idData[21+len(proposal.Target):], proposal.CreatedAt)
	
	proposal.ID = crypto.Keccak256Hash(idData)

	// Set voting periods
	proposal.VotingEndsAt = proposal.CreatedAt + vm.config.VotingPeriod
	proposal.ExecuteAfter = proposal.VotingEndsAt + vm.config.ExecutionDelay
	proposal.Status = ProposalStatusPending

	// Store proposal
	proposalCopy := *proposal
	vm.proposals[proposal.ID] = &proposalCopy
	vm.votes[proposal.ID] = make([]*Vote, 0)

	return proposal.ID, nil
}

// Vote votes on a proposal
func (vm *InMemoryVotingManager) Vote(proposalID common.Hash, voter common.Address, support bool, signature []byte) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Get proposal
	proposal, exists := vm.proposals[proposalID]
	if !exists {
		return ErrProposalNotFound
	}

	// Check proposal status
	if proposal.Status != ProposalStatusPending {
		return ErrProposalNotPending
	}

	// Check if voter is a validator
	if !vm.validators.IsValidator(voter) {
		return ErrInvalidVoter
	}

	// Check if voter has already voted
	existingVotes := vm.votes[proposalID]
	for _, v := range existingVotes {
		if v.Voter == voter {
			return ErrAlreadyVoted
		}
	}

	// Verify signature
	if len(signature) > 0 {
		voteData := boolToBytes(support)
		voteData = append(proposalID[:], voteData...)
		hash := crypto.Keccak256Hash(voteData)
		
		pubkey, err := crypto.SigToPub(hash.Bytes(), signature)
		if err != nil || crypto.PubkeyToAddress(*pubkey) != voter {
			return ErrInvalidSignature
		}
	}

	// Get voter type and weight
	voterType := vm.validators.GetVoterType(voter)
	validatorInfo, _ := vm.validators.GetValidator(voter)
	weight := uint64(1)
	if validatorInfo != nil {
		weight = validatorInfo.VotingPower
	}

	// Record vote
	vote := &Vote{
		ProposalID: proposalID,
		Voter:      voter,
		Support:    support,
		Weight:     weight,
		Timestamp:  proposal.CreatedAt, // Use current time in real implementation
		Signature:  signature,
	}
	vm.votes[proposalID] = append(vm.votes[proposalID], vote)

	// Update vote counts
	if voterType == VoterTypeCore {
		if support {
			proposal.CoreYesVotes += weight
		} else {
			proposal.CoreNoVotes += weight
		}
	} else {
		if support {
			proposal.CommunityYesVotes += weight
		} else {
			proposal.CommunityNoVotes += weight
		}
	}

	return nil
}

// CheckProposalStatus checks and updates proposal status based on current block
func (vm *InMemoryVotingManager) CheckProposalStatus(proposalID common.Hash, currentBlock uint64) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	proposal, exists := vm.proposals[proposalID]
	if !exists {
		return ErrProposalNotFound
	}

	// Only check pending proposals
	if proposal.Status != ProposalStatusPending {
		return nil
	}

	// Check if voting period has ended
	if currentBlock < proposal.VotingEndsAt {
		return nil // Still voting
	}

	// Count validators
	coreValidators := vm.validators.GetCoreValidators()
	communityValidators := vm.validators.GetCommunityValidators()

	var totalCoreVotingPower, totalCommunityVotingPower uint64
	for _, v := range coreValidators {
		totalCoreVotingPower += v.VotingPower
	}
	for _, v := range communityValidators {
		totalCommunityVotingPower += v.VotingPower
	}

	// Check if proposal passed
	passed := false

	// Core validator threshold check (2/3 majority)
	if totalCoreVotingPower > 0 {
		coreApprovalRate := (proposal.CoreYesVotes * 100) / totalCoreVotingPower
		if coreApprovalRate >= vm.config.CoreValidatorThreshold {
			passed = true
		}
	}

	// Community validator veto check (1/3 can veto)
	if passed && totalCommunityVotingPower > 0 {
		communityRejectionRate := (proposal.CommunityNoVotes * 100) / totalCommunityVotingPower
		vetoThreshold := uint64(34) // 1/3
		if communityRejectionRate >= vetoThreshold {
			passed = false
		}
	}

	// Update status
	if passed {
		proposal.Status = ProposalStatusPassed
	} else {
		proposal.Status = ProposalStatusRejected
	}

	return nil
}

// ExecuteProposal executes a passed proposal
func (vm *InMemoryVotingManager) ExecuteProposal(proposalID common.Hash) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	proposal, exists := vm.proposals[proposalID]
	if !exists {
		return ErrProposalNotFound
	}

	if proposal.Status != ProposalStatusPassed {
		return ErrProposalNotPassed
	}

	if proposal.Status == ProposalStatusExecuted {
		return ErrProposalAlreadyExecuted
	}

	// Mark as executed
	proposal.Status = ProposalStatusExecuted

	return nil
}

// GetProposal returns a proposal
func (vm *InMemoryVotingManager) GetProposal(proposalID common.Hash) (*Proposal, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	proposal, exists := vm.proposals[proposalID]
	if !exists {
		return nil, ErrProposalNotFound
	}

	proposalCopy := *proposal
	return &proposalCopy, nil
}

// GetProposalVotes returns all votes for a proposal
func (vm *InMemoryVotingManager) GetProposalVotes(proposalID common.Hash) ([]*Vote, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	votes, exists := vm.votes[proposalID]
	if !exists {
		return nil, ErrProposalNotFound
	}

	votesCopy := make([]*Vote, len(votes))
	for i, v := range votes {
		voteCopy := *v
		votesCopy[i] = &voteCopy
	}

	return votesCopy, nil
}

// GetActiveProposals returns all active proposals
func (vm *InMemoryVotingManager) GetActiveProposals() []*Proposal {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	active := make([]*Proposal, 0)
	for _, p := range vm.proposals {
		if p.Status == ProposalStatusPending || p.Status == ProposalStatusPassed {
			proposalCopy := *p
			active = append(active, &proposalCopy)
		}
	}

	return active
}

// boolToBytes converts a boolean to a byte slice
func boolToBytes(b bool) []byte {
	if b {
		return []byte{1}
	}
	return []byte{0}
}
