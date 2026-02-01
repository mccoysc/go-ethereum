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
"github.com/ethereum/go-ethereum/common"
)

// GovernanceContract is a unified interface that aggregates all governance functionality.
// It serves as the central governance component mentioned in architecture documentation.
// This is a facade that combines WhitelistManager, VotingManager, and ValidatorManager.
type GovernanceContract struct {
whitelist WhitelistManager
voting    VotingManager
validator ValidatorManager
}

// NewGovernanceContract creates a new governance contract instance
func NewGovernanceContract(
whitelist WhitelistManager,
voting VotingManager,
validator ValidatorManager,
) *GovernanceContract {
return &GovernanceContract{
whitelist: whitelist,
voting:    voting,
validator: validator,
}
}

// GetWhitelistManager returns the whitelist manager
func (gc *GovernanceContract) GetWhitelistManager() WhitelistManager {
return gc.whitelist
}

// GetVotingManager returns the voting manager
func (gc *GovernanceContract) GetVotingManager() VotingManager {
return gc.voting
}

// GetValidatorManager returns the validator manager
func (gc *GovernanceContract) GetValidatorManager() ValidatorManager {
return gc.validator
}

// IsAllowed checks if an MRENCLAVE is allowed
func (gc *GovernanceContract) IsAllowed(mrenclave [32]byte) bool {
return gc.whitelist.IsAllowed(mrenclave)
}

// CreateProposal creates a new governance proposal
func (gc *GovernanceContract) CreateProposal(proposal *Proposal) (common.Hash, error) {
return gc.voting.CreateProposal(proposal)
}

// Vote casts a vote on a proposal
func (gc *GovernanceContract) Vote(proposalID common.Hash, voter common.Address, support bool, signature []byte) error {
return gc.voting.Vote(proposalID, voter, support, signature)
}

// IsValidator checks if an address is a validator
func (gc *GovernanceContract) IsValidator(addr common.Address) bool {
return gc.validator.IsValidator(addr)
}

// GetCoreValidators returns all core validators
func (gc *GovernanceContract) GetCoreValidators() []*ValidatorInfo {
return gc.validator.GetCoreValidators()
}

// GetCommunityValidators returns all community validators
func (gc *GovernanceContract) GetCommunityValidators() []*ValidatorInfo {
return gc.validator.GetCommunityValidators()
}
