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
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// EntryStatus represents the status of a whitelist entry
type EntryStatus uint8

const (
	StatusPending    EntryStatus = 0x00 // 待投票
	StatusApproved   EntryStatus = 0x01 // 已批准
	StatusActive     EntryStatus = 0x02 // 已激活
	StatusDeprecated EntryStatus = 0x03 // 已弃用
	StatusRejected   EntryStatus = 0x04 // 已拒绝
)

// PermissionLevel represents permission levels for MRENCLAVE entries
type PermissionLevel uint8

const (
	PermissionBasic    PermissionLevel = 0x01 // 基础权限
	PermissionStandard PermissionLevel = 0x02 // 标准权限
	PermissionFull     PermissionLevel = 0x03 // 完整权限
)

// MREnclaveEntry represents a whitelist entry for MRENCLAVE
type MREnclaveEntry struct {
	MRENCLAVE       [32]byte        // MRENCLAVE 值
	Version         string          // 版本号
	AddedAt         uint64          // 添加时间（区块号）
	AddBy           common.Address  // 添加者
	PermissionLevel PermissionLevel // 权限级别
	Status          EntryStatus     // 状态
}

// ProposalType represents the type of governance proposal
type ProposalType uint8

const (
	ProposalAddMREnclave      ProposalType = 0x01 // 添加 MRENCLAVE
	ProposalRemoveMREnclave   ProposalType = 0x02 // 移除 MRENCLAVE
	ProposalUpgradePermission ProposalType = 0x03 // 升级权限
	ProposalAddValidator      ProposalType = 0x04 // 添加验证者
	ProposalRemoveValidator   ProposalType = 0x05 // 移除验证者
	ProposalParameterChange   ProposalType = 0x06 // 参数修改
	ProposalNormalUpgrade     ProposalType = 0x07 // 普通升级
	ProposalEmergencyUpgrade  ProposalType = 0x08 // 紧急升级
)

// ProposalStatus represents the status of a proposal
type ProposalStatus uint8

const (
	ProposalStatusPending   ProposalStatus = 0x00 // 投票中
	ProposalStatusPassed    ProposalStatus = 0x01 // 已通过
	ProposalStatusRejected  ProposalStatus = 0x02 // 已拒绝
	ProposalStatusExecuted  ProposalStatus = 0x03 // 已执行
	ProposalStatusCancelled ProposalStatus = 0x04 // 已取消
	ProposalStatusExpired   ProposalStatus = 0x05 // 已过期
)

// Proposal represents a governance proposal
type Proposal struct {
	ID                common.Hash    // 提案 ID
	Type              ProposalType   // 提案类型
	Proposer          common.Address // 提案者
	Target            []byte         // 目标数据（如 MRENCLAVE）
	Description       string         // 描述
	CreatedAt         uint64         // 创建区块
	VotingEndsAt      uint64         // 投票截止区块
	ExecuteAfter      uint64         // 可执行区块
	Status            ProposalStatus // 状态
	CoreYesVotes      uint64         // 核心验证者赞成票
	CoreNoVotes       uint64         // 核心验证者反对票
	CommunityYesVotes uint64         // 社区验证者赞成票
	CommunityNoVotes  uint64         // 社区验证者反对票
}

// Vote represents a vote on a proposal
type Vote struct {
	ProposalID common.Hash    // 提案 ID
	Voter      common.Address // 投票者
	Support    bool           // true = 支持, false = 反对
	Weight     uint64         // 投票权重
	Timestamp  uint64         // 时间戳
	Signature  []byte         // 签名
}

// VoterType represents the type of voter
type VoterType uint8

const (
	VoterTypeCore      VoterType = 0x01 // 核心验证者
	VoterTypeCommunity VoterType = 0x02 // 社区验证者
)

// ValidatorStatus represents the status of a validator
type ValidatorStatus uint8

const (
	ValidatorStatusActive   ValidatorStatus = 0x01 // 活跃
	ValidatorStatusInactive ValidatorStatus = 0x02 // 不活跃
	ValidatorStatusJailed   ValidatorStatus = 0x03 // 监禁
	ValidatorStatusExiting  ValidatorStatus = 0x04 // 退出中
)

// ValidatorInfo represents information about a validator
type ValidatorInfo struct {
	Address      common.Address  // 验证者地址
	Type         VoterType       // 验证者类型
	MRENCLAVE    [32]byte        // 当前 MRENCLAVE
	StakeAmount  *big.Int        // 质押金额
	JoinedAt     uint64          // 加入区块
	LastActiveAt uint64          // 最后活跃区块
	VotingPower  uint64          // 投票权重
	Status       ValidatorStatus // 状态
}

// AdmissionStatus represents the admission status of a node
type AdmissionStatus struct {
	NodeID       common.Hash // 节点 ID
	MRENCLAVE    [32]byte    // MRENCLAVE
	Allowed      bool        // 是否允许
	Reason       string      // 原因
	ConnectedAt  uint64      // 连接时间
	LastVerified uint64      // 最后验证时间
}

// NodePermission represents the permission level of a node
type NodePermission struct {
	MRENCLAVE     [32]byte        // MRENCLAVE 值
	CurrentLevel  PermissionLevel // 当前权限级别
	ActivatedAt   uint64          // 激活区块高度
	LastUpgradeAt uint64          // 最后升级区块高度
	UptimeHistory []float64       // 在线率历史
}

// WhitelistConfig holds the configuration for whitelist management
type WhitelistConfig struct {
	CoreValidatorThreshold      uint64 // 核心验证者投票阈值（百分比，默认 67 表示 2/3）
	CommunityValidatorThreshold uint64 // 社区验证者投票阈值（百分比，默认 51）
	VotingPeriod                uint64 // 投票期限（区块数，默认 40320）
	ExecutionDelay              uint64 // 执行延迟（区块数，默认 5760）
	MinParticipation            uint64 // 最小投票参与率（百分比，默认 50%）
}

// DefaultWhitelistConfig returns the default whitelist configuration
func DefaultWhitelistConfig() *WhitelistConfig {
	return &WhitelistConfig{
		CoreValidatorThreshold:      67,    // 2/3
		CommunityValidatorThreshold: 51,    // 简单多数
		VotingPeriod:                40320, // 约 7 天（15s/块）
		ExecutionDelay:              5760,  // 约 1 天
		MinParticipation:            50,    // 50%
	}
}

// CoreValidatorConfig holds the configuration for core validators
type CoreValidatorConfig struct {
	MinMembers      int     // 最小成员数（默认 5）
	MaxMembers      int     // 最大成员数（默认 7）
	QuorumThreshold float64 // 投票通过阈值（默认 0.667 表示 2/3）
}

// DefaultCoreValidatorConfig returns the default core validator configuration
func DefaultCoreValidatorConfig() *CoreValidatorConfig {
	return &CoreValidatorConfig{
		MinMembers:      5,
		MaxMembers:      7,
		QuorumThreshold: 0.667, // 2/3
	}
}

// CommunityValidatorConfig holds the configuration for community validators
type CommunityValidatorConfig struct {
	MinUptime     time.Duration // 最小运行时间（默认 30 天）
	MinStake      *big.Int      // 最小质押量（初始值 10000 X）
	VetoThreshold float64       // 否决阈值（默认 0.334 表示 1/3）
}

// DefaultCommunityValidatorConfig returns the default community validator configuration
func DefaultCommunityValidatorConfig() *CommunityValidatorConfig {
	return &CommunityValidatorConfig{
		MinUptime:     30 * 24 * time.Hour, // 30 天
		MinStake:      new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e18)), // 10000 X
		VetoThreshold: 0.334, // 1/3
	}
}

// StakingConfig holds the configuration for validator staking
type StakingConfig struct {
	MinStakeAmount    *big.Int // 最小质押金额
	UnstakeLockPeriod uint64   // 解除质押锁定期（区块数）
	AnnualRewardRate  uint64   // 质押奖励率（年化百分比）
	SlashingRate      uint64   // 惩罚率（百分比）
}

// DefaultStakingConfig returns the default staking configuration
func DefaultStakingConfig() *StakingConfig {
	return &StakingConfig{
		MinStakeAmount:    new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e18)), // 10000 X
		UnstakeLockPeriod: 172800, // 约 30 天（15s/块）
		AnnualRewardRate:  5,      // 5%
		SlashingRate:      10,     // 10%
	}
}

// ProgressivePermissionConfig holds the configuration for progressive permissions
type ProgressivePermissionConfig struct {
	BasicDuration           uint64  // 基础权限持续时间（区块数）
	StandardDuration        uint64  // 标准权限持续时间（区块数）
	StandardUptimeThreshold float64 // 升级到标准权限的最小在线率
	FullUptimeThreshold     float64 // 升级到完整权限的最小在线率
}

// DefaultProgressivePermissionConfig returns the default progressive permission configuration
func DefaultProgressivePermissionConfig() *ProgressivePermissionConfig {
	return &ProgressivePermissionConfig{
		BasicDuration:           40320,  // 约 7 天
		StandardDuration:        120960, // 约 21 天
		StandardUptimeThreshold: 0.95,   // 95%
		FullUptimeThreshold:     0.99,   // 99%
	}
}
