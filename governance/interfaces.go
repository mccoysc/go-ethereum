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

	"github.com/ethereum/go-ethereum/common"
)

// WhitelistManager manages MRENCLAVE whitelist
type WhitelistManager interface {
	// IsAllowed checks if an MRENCLAVE is allowed
	IsAllowed(mrenclave [32]byte) bool

	// GetPermissionLevel returns the permission level of an MRENCLAVE
	GetPermissionLevel(mrenclave [32]byte) PermissionLevel

	// GetEntry returns the entry for an MRENCLAVE
	GetEntry(mrenclave [32]byte) (*MREnclaveEntry, error)

	// GetAllEntries returns all entries
	GetAllEntries() []*MREnclaveEntry

	// ProposeAdd proposes adding a new MRENCLAVE
	ProposeAdd(proposer common.Address, mrenclave [32]byte, version string) (common.Hash, error)

	// ProposeRemove proposes removing an MRENCLAVE
	ProposeRemove(proposer common.Address, mrenclave [32]byte, reason string) (common.Hash, error)

	// ProposeUpgrade proposes upgrading the permission level of an MRENCLAVE
	ProposeUpgrade(proposer common.Address, mrenclave [32]byte, newLevel PermissionLevel) (common.Hash, error)

	// AddEntry adds a new entry to the whitelist (internal, called by voting execution)
	AddEntry(entry *MREnclaveEntry)

	// RemoveEntry removes an entry from the whitelist (internal, called by voting execution)
	RemoveEntry(mrenclave [32]byte)
}

// VotingManager manages governance proposals and voting
type VotingManager interface {
	// CreateProposal creates a new proposal
	CreateProposal(proposal *Proposal) (common.Hash, error)

	// Vote votes on a proposal
	Vote(proposalID common.Hash, voter common.Address, support bool, signature []byte) error

	// GetProposal returns a proposal
	GetProposal(proposalID common.Hash) (*Proposal, error)

	// GetProposalVotes returns all votes for a proposal
	GetProposalVotes(proposalID common.Hash) ([]*Vote, error)

	// ExecuteProposal executes a passed proposal
	ExecuteProposal(proposalID common.Hash) error

	// GetActiveProposals returns all active proposals
	GetActiveProposals() []*Proposal

	// CheckProposalStatus checks and updates proposal status based on current block
	CheckProposalStatus(proposalID common.Hash, currentBlock uint64) error
}

// ValidatorManager manages validators and their staking
type ValidatorManager interface {
	// GetValidator returns information about a validator
	GetValidator(addr common.Address) (*ValidatorInfo, error)

	// GetAllValidators returns all validators
	GetAllValidators() []*ValidatorInfo

	// GetCoreValidators returns all core validators
	GetCoreValidators() []*ValidatorInfo

	// GetCommunityValidators returns all community validators
	GetCommunityValidators() []*ValidatorInfo

	// IsValidator checks if an address is a validator
	IsValidator(addr common.Address) bool

	// GetVoterType returns the voter type for an address
	GetVoterType(addr common.Address) VoterType

	// Stake adds stake for a validator
	Stake(addr common.Address, amount *big.Int) error

	// Unstake removes stake for a validator
	Unstake(addr common.Address, amount *big.Int) error

	// ClaimRewards claims staking rewards for a validator
	ClaimRewards(addr common.Address) (*big.Int, error)

	// Slash slashes a validator for misbehavior
	Slash(addr common.Address, reason string) error

	// UpdateMREnclave updates the MRENCLAVE for a validator
	UpdateMREnclave(addr common.Address, newMREnclave [32]byte) error
}

// AdmissionController manages node admission based on SGX attestation
type AdmissionController interface {
	// CheckAdmission checks if a node is allowed to join
	CheckAdmission(nodeID common.Hash, mrenclave [32]byte, quote []byte) (bool, error)

	// GetAdmissionStatus returns the admission status of a node
	GetAdmissionStatus(nodeID common.Hash) (*AdmissionStatus, error)

	// RecordConnection records a node connection
	RecordConnection(nodeID common.Hash, mrenclave [32]byte) error

	// RecordDisconnection records a node disconnection
	RecordDisconnection(nodeID common.Hash) error

	// GetHardwareBinding returns the hardware ID binding for a validator
	GetHardwareBinding(validatorAddr common.Address) (string, bool)

	// GetValidatorByHardware returns the validator address for a hardware ID
	GetValidatorByHardware(hardwareID string) (common.Address, bool)

	// UnregisterValidator removes the hardware binding for a validator
	UnregisterValidator(validatorAddr common.Address) error
}

// SGXVerifier is the interface for SGX quote verification
type SGXVerifier interface {
	// VerifyQuote verifies an SGX quote
	VerifyQuote(quote []byte) error

	// ExtractMREnclave extracts the MRENCLAVE from a quote
	ExtractMREnclave(quote []byte) ([32]byte, error)

	// ExtractHardwareID extracts the hardware ID from a quote
	ExtractHardwareID(quote []byte) (string, error)
}
