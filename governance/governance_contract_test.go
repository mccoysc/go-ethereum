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
)

func TestGovernanceContract_Creation(t *testing.T) {
whitelistCfg := DefaultWhitelistConfig()
voting := NewMockVotingManager()
whitelist := NewInMemoryWhitelistManager(whitelistCfg, voting)
validators := NewMockValidatorManager()
votingMgr := NewInMemoryVotingManager(whitelistCfg, validators)
validatorMgr := NewInMemoryValidatorManager(DefaultStakingConfig())

gc := NewGovernanceContract(whitelist, votingMgr, validatorMgr)
if gc == nil {
t.Fatal("governance contract should not be nil")
}

// Verify managers are accessible
if gc.GetWhitelistManager() == nil {
t.Error("whitelist manager should be accessible")
}
if gc.GetVotingManager() == nil {
t.Error("voting manager should be accessible")
}
if gc.GetValidatorManager() == nil {
t.Error("validator manager should be accessible")
}
}

func TestGovernanceContract_IsAllowed(t *testing.T) {
whitelistCfg := DefaultWhitelistConfig()
voting := NewMockVotingManager()
whitelist := NewInMemoryWhitelistManager(whitelistCfg, voting)
validators := NewMockValidatorManager()
votingMgr := NewInMemoryVotingManager(whitelistCfg, validators)
validatorMgr := NewInMemoryValidatorManager(DefaultStakingConfig())

gc := NewGovernanceContract(whitelist, votingMgr, validatorMgr)

mrenclave := [32]byte{1, 2, 3}
whitelist.AddEntry(&MREnclaveEntry{
MRENCLAVE: mrenclave,
Version:   "v1.0.0",
Status:    StatusActive,
})

if !gc.IsAllowed(mrenclave) {
t.Error("MRENCLAVE should be allowed")
}
}

func TestGovernanceContract_CreateProposal(t *testing.T) {
whitelistCfg := DefaultWhitelistConfig()
voting := NewMockVotingManager()
whitelist := NewInMemoryWhitelistManager(whitelistCfg, voting)
validators := NewMockValidatorManager()
votingMgr := NewInMemoryVotingManager(whitelistCfg, validators)
validatorMgr := NewInMemoryValidatorManager(DefaultStakingConfig())

gc := NewGovernanceContract(whitelist, votingMgr, validatorMgr)

proposal := &Proposal{
Type:      ProposalAddMREnclave,
Proposer:  common.HexToAddress("0x1"),
Target:    []byte{1, 2, 3},
CreatedAt: 100,
}

proposalID, err := gc.CreateProposal(proposal)
if err != nil {
t.Fatalf("failed to create proposal: %v", err)
}
if proposalID == (common.Hash{}) {
t.Error("proposal ID should not be zero")
}
}

func TestGovernanceContract_Vote(t *testing.T) {
whitelistCfg := DefaultWhitelistConfig()
voting := NewMockVotingManager()
whitelist := NewInMemoryWhitelistManager(whitelistCfg, voting)
validators := NewMockValidatorManager()
votingMgr := NewInMemoryVotingManager(whitelistCfg, validators)
validatorMgr := NewInMemoryValidatorManager(DefaultStakingConfig())

gc := NewGovernanceContract(whitelist, votingMgr, validatorMgr)

// Add validator
voter := common.HexToAddress("0x1")
validators.AddMockValidator(voter, VoterTypeCore, 1)

// Create proposal
proposal := &Proposal{
Type:      ProposalAddMREnclave,
Proposer:  voter,
Target:    []byte{1, 2, 3},
CreatedAt: 100,
}
proposalID, _ := gc.CreateProposal(proposal)

// Vote
err := gc.Vote(proposalID, voter, true, nil)
if err != nil {
t.Fatalf("failed to vote: %v", err)
}
}

func TestGovernanceContract_IsValidator(t *testing.T) {
whitelistCfg := DefaultWhitelistConfig()
voting := NewMockVotingManager()
whitelist := NewInMemoryWhitelistManager(whitelistCfg, voting)
validators := NewMockValidatorManager()
votingMgr := NewInMemoryVotingManager(whitelistCfg, validators)
validatorMgr := NewInMemoryValidatorManager(DefaultStakingConfig())

gc := NewGovernanceContract(whitelist, votingMgr, validatorMgr)

addr := common.HexToAddress("0x1")
if gc.IsValidator(addr) {
t.Error("should not be validator initially")
}

// Add validator using Stake which is the proper way
stakeAmount := new(big.Int).Mul(big.NewInt(15000), big.NewInt(1e18))
validatorMgr.Stake(addr, stakeAmount)

if !gc.IsValidator(addr) {
t.Error("should be validator after staking")
}
}

func TestGovernanceContract_GetValidators(t *testing.T) {
whitelistCfg := DefaultWhitelistConfig()
voting := NewMockVotingManager()
whitelist := NewInMemoryWhitelistManager(whitelistCfg, voting)
validators := NewMockValidatorManager()
votingMgr := NewInMemoryVotingManager(whitelistCfg, validators)
validatorMgr := NewInMemoryValidatorManager(DefaultStakingConfig())

gc := NewGovernanceContract(whitelist, votingMgr, validatorMgr)

// Add validators using AddValidator which is test-friendly
core := &ValidatorInfo{
Address:     common.HexToAddress("0x1"),
Type:        VoterTypeCore,
Status:      ValidatorStatusActive,
StakeAmount: big.NewInt(1000),
VotingPower: 1,
}
validatorMgr.AddValidator(core)

community := &ValidatorInfo{
Address:     common.HexToAddress("0x2"),
Type:        VoterTypeCommunity,
Status:      ValidatorStatusActive,
StakeAmount: big.NewInt(1000),
VotingPower: 1,
}
validatorMgr.AddValidator(community)

coreValidators := gc.GetCoreValidators()
if len(coreValidators) != 1 {
t.Errorf("expected 1 core validator, got %d", len(coreValidators))
}

communityValidators := gc.GetCommunityValidators()
if len(communityValidators) != 1 {
t.Errorf("expected 1 community validator, got %d", len(communityValidators))
}
}

func TestValidatorInfo_HelperMethods(t *testing.T) {
validator := &ValidatorInfo{
Address:     common.HexToAddress("0x1"),
Type:        VoterTypeCore,
JoinedAt:    1000,
StakeAmount: big.NewInt(5000),
VotingPower: 1,
}

// Test GetJoinedTime
joinTime := validator.GetJoinedTime(15) // 15 seconds per block
expectedTime := uint64(1000 * 15)
if joinTime != expectedTime {
t.Errorf("expected join time %d, got %d", expectedTime, joinTime)
}

// Test GetValidatorType
if validator.GetValidatorType() != VoterTypeCore {
t.Error("GetValidatorType should return VoterTypeCore")
}

// Test StakedAmount alias
if validator.StakedAmount().Cmp(big.NewInt(5000)) != 0 {
t.Error("StakedAmount should match StakeAmount")
}
}

func TestValidatorType_Aliases(t *testing.T) {
// Test that aliases work correctly
var vtype ValidatorType = CoreValidator
if vtype != VoterTypeCore {
t.Error("CoreValidator alias should equal VoterTypeCore")
}

vtype = CommunityValidator
if vtype != VoterTypeCommunity {
t.Error("CommunityValidator alias should equal VoterTypeCommunity")
}
}
