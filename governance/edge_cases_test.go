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

// Edge case tests to achieve 100% coverage

func TestCheckAdmission_ExtractMREnclaveError(t *testing.T) {
whitelistCfg := DefaultWhitelistConfig()
voting := NewMockVotingManager()
whitelist := NewInMemoryWhitelistManager(whitelistCfg, voting)

mrenclave := [32]byte{1, 2, 3}
whitelist.AddEntry(&MREnclaveEntry{
MRENCLAVE: mrenclave,
Version:   "v1.0.0",
Status:    StatusActive,
})

// Verifier that returns error on ExtractMREnclave
verifier := &MockSGXVerifier{
shouldFailExtract:  true,
mrenclaveToReturn:  mrenclave,
hardwareIDToReturn: "hw1",
}

ac := NewSGXAdmissionController(whitelist, verifier)
nodeID := common.BytesToHash([]byte("node1"))
quote := []byte("valid-quote")

allowed, err := ac.CheckAdmission(nodeID, mrenclave, quote)
if allowed {
t.Error("should not be allowed when MRENCLAVE extraction fails")
}
if err == nil {
t.Error("should return error when MRENCLAVE extraction fails")
}

// Verify status was recorded
status, _ := ac.GetAdmissionStatus(nodeID)
if status.Allowed {
t.Error("status should show not allowed")
}
}

func TestGetAdmissionStatus_NotFound(t *testing.T) {
whitelistCfg := DefaultWhitelistConfig()
voting := NewMockVotingManager()
whitelist := NewInMemoryWhitelistManager(whitelistCfg, voting)
verifier := &MockSGXVerifier{}
ac := NewSGXAdmissionController(whitelist, verifier)

nodeID := common.BytesToHash([]byte("nonexistent"))
_, err := ac.GetAdmissionStatus(nodeID)
if err != ErrNodeNotFound {
t.Errorf("expected error %v, got %v", ErrNodeNotFound, err)
}
}

func TestUnregisterValidator_NotFound(t *testing.T) {
whitelistCfg := DefaultWhitelistConfig()
voting := NewMockVotingManager()
whitelist := NewInMemoryWhitelistManager(whitelistCfg, voting)
verifier := &MockSGXVerifier{}
ac := NewSGXAdmissionController(whitelist, verifier)

addr := common.HexToAddress("0x999")
err := ac.UnregisterValidator(addr)
if err != ErrValidatorNotFound {
t.Errorf("expected error %v, got %v", ErrValidatorNotFound, err)
}
}

func TestRegisterFounder_MaxFoundersEdgeCase(t *testing.T) {
allowedMR := [32]byte{1, 2, 3}
verifier := &MockSGXVerifier{
mrenclaveToReturn:  allowedMR,
hardwareIDToReturn: "hw",
}

bc := NewBootstrapContract(allowedMR, 2, verifier)

// Register up to max
addr1 := common.HexToAddress("0x1")
hwID1 := [32]byte{1}
bc.RegisterFounder(addr1, allowedMR, hwID1, []byte("quote"))

addr2 := common.HexToAddress("0x2")
hwID2 := [32]byte{2}
bc.RegisterFounder(addr2, allowedMR, hwID2, []byte("quote"))

// Try to register one more (should fail - bootstrap ended)
addr3 := common.HexToAddress("0x3")
hwID3 := [32]byte{3}
err := bc.RegisterFounder(addr3, allowedMR, hwID3, []byte("quote"))
if err != ErrBootstrapEnded {
t.Errorf("expected error %v, got %v", ErrBootstrapEnded, err)
}

// Verify bootstrap ended
if !bc.BootstrapEnded {
t.Error("bootstrap should have ended")
}
}

func TestCheckUpgrade_NoExistingPermission(t *testing.T) {
config := DefaultProgressivePermissionConfig()
pm := NewProgressivePermissionManager(config)

mrenclave := [32]byte{1, 2, 3}
currentBlock := uint64(1000)
upgraded, level := pm.CheckUpgrade(mrenclave, currentBlock, 0.95)

if upgraded {
t.Error("should not upgrade when node doesn't exist yet")
}
if level != PermissionBasic {
t.Error("should return basic level for new node")
}
}

func TestCalculateAverageUptime_EmptyHistory(t *testing.T) {
config := DefaultProgressivePermissionConfig()
pm := NewProgressivePermissionManager(config)

avg := pm.calculateAverageUptime([]float64{})
if avg != 0.0 {
t.Errorf("average of empty history should be 0, got %f", avg)
}
}

func TestDowngrade_NodeNotFound(t *testing.T) {
config := DefaultProgressivePermissionConfig()
pm := NewProgressivePermissionManager(config)

mrenclave := [32]byte{9, 9, 9}
pm.Downgrade(mrenclave, "test")

// Should not crash, just be a no-op
_, exists := pm.GetNodePermission(mrenclave)
if exists {
t.Error("node should not exist after downgrade of non-existent node")
}
}

func TestActivateNode_AlreadyExists(t *testing.T) {
config := DefaultProgressivePermissionConfig()
pm := NewProgressivePermissionManager(config)

mrenclave := [32]byte{1, 2, 3}
pm.ActivateNode(mrenclave, 100)

// Activate again
pm.ActivateNode(mrenclave, 200)

// Should keep original activation
perm, _ := pm.GetNodePermission(mrenclave)
if perm.ActivatedAt != 100 {
t.Error("should keep original activation time")
}
}

func TestIsNewVersionNode_SingleMREnclave(t *testing.T) {
localMR := [32]byte{1, 2, 3}

config := &MockSecurityConfigReader{
whitelist: []MREnclaveEntry{
{MRENCLAVE: localMR, Status: StatusActive},
},
}

checker := NewUpgradeModeChecker(config, localMR)
if checker.IsNewVersionNode() {
t.Error("single MRENCLAVE should not be considered new version")
}
}

func TestGetValidator_NotFound(t *testing.T) {
config := DefaultStakingConfig()
vm := NewInMemoryValidatorManager(config)

addr := common.HexToAddress("0x999")
_, err := vm.GetValidator(addr)
if err != ErrValidatorNotFound {
t.Errorf("expected error %v, got %v", ErrValidatorNotFound, err)
}
}

func TestUnstake_BelowMinimum(t *testing.T) {
config := DefaultStakingConfig()
vm := NewInMemoryValidatorManager(config)

addr := common.HexToAddress("0x1")
stakeAmount := new(big.Int).Mul(big.NewInt(20000), big.NewInt(1e18))

// Stake
vm.Stake(addr, stakeAmount)

// Unstake leaving below minimum - should succeed but mark inactive
unstakeAmount := new(big.Int).Mul(big.NewInt(15000), big.NewInt(1e18))
err := vm.Unstake(addr, unstakeAmount)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}

// Verify validator was marked inactive
validator, _ := vm.GetValidator(addr)
if validator.Status != ValidatorStatusInactive {
t.Error("validator should be inactive when stake below minimum")
}
}

func TestSlash_ValidatorNotFound(t *testing.T) {
config := DefaultStakingConfig()
vm := NewInMemoryValidatorManager(config)

addr := common.HexToAddress("0x999")
err := vm.Slash(addr, "test")
if err != ErrValidatorNotFound {
t.Errorf("expected error %v, got %v", ErrValidatorNotFound, err)
}
}

func TestUpdateMREnclave_NotFound(t *testing.T) {
config := DefaultStakingConfig()
vm := NewInMemoryValidatorManager(config)

addr := common.HexToAddress("0x999")
newMR := [32]byte{1, 2, 3}
err := vm.UpdateMREnclave(addr, newMR)
if err != ErrValidatorNotFound {
t.Errorf("expected error %v, got %v", ErrValidatorNotFound, err)
}
}

func TestVote_ProposalNotFound(t *testing.T) {
config := DefaultWhitelistConfig()
validators := NewMockValidatorManager()
vm := NewInMemoryVotingManager(config, validators)

proposalID := common.HexToHash("nonexistent")
voter := common.HexToAddress("0x1")

err := vm.Vote(proposalID, voter, true, nil)
if err != ErrProposalNotFound {
t.Errorf("expected error %v, got %v", ErrProposalNotFound, err)
}
}

func TestVote_AlreadyVoted(t *testing.T) {
config := DefaultWhitelistConfig()
validators := NewMockValidatorManager()
vm := NewInMemoryVotingManager(config, validators)

proposer := common.HexToAddress("0x1")
validators.AddMockValidator(proposer, VoterTypeCore, 1)

proposal := &Proposal{
Type:      ProposalAddMREnclave,
Proposer:  proposer,
Target:    []byte{1},
CreatedAt: 100,
}
proposalID, _ := vm.CreateProposal(proposal)

// Vote once
vm.Vote(proposalID, proposer, true, nil)

// Vote again
err := vm.Vote(proposalID, proposer, false, nil)
if err != ErrAlreadyVoted {
t.Errorf("expected error %v, got %v", ErrAlreadyVoted, err)
}
}

func TestCheckProposalStatus_NotPending(t *testing.T) {
config := DefaultWhitelistConfig()
validators := NewMockValidatorManager()
vm := NewInMemoryVotingManager(config, validators)

proposer := common.HexToAddress("0x1")
proposal := &Proposal{
Type:      ProposalAddMREnclave,
Proposer:  proposer,
Target:    []byte{1},
CreatedAt: 100,
}
proposalID, _ := vm.CreateProposal(proposal)

// Manually set to passed
vm.proposals[proposalID].Status = ProposalStatusPassed

// Try to check again
err := vm.CheckProposalStatus(proposalID, 1000)
if err != nil {
t.Error("should not error on non-pending proposal")
}

// Status should remain passed
p, _ := vm.GetProposal(proposalID)
if p.Status != ProposalStatusPassed {
t.Error("status should remain passed")
}
}

func TestCheckProposalStatus_StillVoting(t *testing.T) {
config := DefaultWhitelistConfig()
validators := NewMockValidatorManager()
vm := NewInMemoryVotingManager(config, validators)

proposer := common.HexToAddress("0x1")
proposal := &Proposal{
Type:      ProposalAddMREnclave,
Proposer:  proposer,
Target:    []byte{1},
CreatedAt: 100,
}
proposalID, _ := vm.CreateProposal(proposal)

// Check before voting period ends
currentBlock := uint64(100 + config.VotingPeriod - 1)
err := vm.CheckProposalStatus(proposalID, currentBlock)
if err != nil {
t.Error("should not error while still voting")
}

// Status should remain pending
p, _ := vm.GetProposal(proposalID)
if p.Status != ProposalStatusPending {
t.Error("status should remain pending during voting period")
}
}

func TestExecuteProposal_NotFound(t *testing.T) {
config := DefaultWhitelistConfig()
validators := NewMockValidatorManager()
vm := NewInMemoryVotingManager(config, validators)

proposalID := common.HexToHash("nonexistent")
err := vm.ExecuteProposal(proposalID)
if err != ErrProposalNotFound {
t.Errorf("expected error %v, got %v", ErrProposalNotFound, err)
}
}

func TestGetProposal_NotFound(t *testing.T) {
config := DefaultWhitelistConfig()
validators := NewMockValidatorManager()
vm := NewInMemoryVotingManager(config, validators)

proposalID := common.HexToHash("nonexistent")
_, err := vm.GetProposal(proposalID)
if err != ErrProposalNotFound {
t.Errorf("expected error %v, got %v", ErrProposalNotFound, err)
}
}

func TestGetProposalVotes_NotFound(t *testing.T) {
config := DefaultWhitelistConfig()
validators := NewMockValidatorManager()
vm := NewInMemoryVotingManager(config, validators)

proposalID := common.HexToHash("nonexistent")
_, err := vm.GetProposalVotes(proposalID)
if err != ErrProposalNotFound {
t.Errorf("expected error %v, got %v", ErrProposalNotFound, err)
}
}

func TestProposeAdd_VotingManagerError(t *testing.T) {
config := DefaultWhitelistConfig()
voting := NewMockVotingManager()
voting.shouldFailCreate = true
wm := NewInMemoryWhitelistManager(config, voting)

proposer := common.HexToAddress("0x1")
mrenclave := [32]byte{9, 9, 9}

_, err := wm.ProposeAdd(proposer, mrenclave, "v1.0.0")
if err == nil {
t.Error("should fail when voting manager fails")
}
}

func TestProposeRemove_VotingManagerError(t *testing.T) {
config := DefaultWhitelistConfig()
voting := NewMockVotingManager()
wm := NewInMemoryWhitelistManager(config, voting)

// Add entry first
mrenclave := [32]byte{1, 2, 3}
wm.AddEntry(&MREnclaveEntry{
MRENCLAVE: mrenclave,
Version:   "v1.0.0",
Status:    StatusActive,
})

voting.shouldFailCreate = true
proposer := common.HexToAddress("0x1")

_, err := wm.ProposeRemove(proposer, mrenclave, "test reason")
if err == nil {
t.Error("should fail when voting manager fails")
}
}

func TestProposeUpgrade_VotingManagerError(t *testing.T) {
config := DefaultWhitelistConfig()
voting := NewMockVotingManager()
wm := NewInMemoryWhitelistManager(config, voting)

// Add entry
mrenclave := [32]byte{1, 2, 3}
wm.AddEntry(&MREnclaveEntry{
MRENCLAVE:       mrenclave,
Version:         "v1.0.0",
Status:          StatusActive,
PermissionLevel: PermissionBasic,
})

voting.shouldFailCreate = true
proposer := common.HexToAddress("0x1")

_, err := wm.ProposeUpgrade(proposer, mrenclave, PermissionStandard)
if err == nil {
t.Error("should fail when voting manager fails")
}
}
