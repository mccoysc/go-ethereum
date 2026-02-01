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

package incentive

import (
	"math/big"
	"time"
)

// RewardConfig represents the reward configuration.
type RewardConfig struct {
	// BaseBlockReward is the base block reward
	BaseBlockReward *big.Int
	
	// DecayPeriod is the reward decay period (in blocks)
	DecayPeriod uint64
	
	// DecayRate is the decay rate (percentage)
	DecayRate uint64
	
	// MinBlockReward is the minimum block reward
	MinBlockReward *big.Int
	
	// MultiProducerConfig is the multi-producer reward configuration
	MultiProducerConfig *MultiProducerRewardConfig
	
	// OnlineRewardConfig is the online reward configuration
	OnlineRewardConfig *OnlineRewardConfig
	
	// ReputationConfig is the reputation configuration
	ReputationConfig *ReputationConfig
	
	// PenaltyConfig is the penalty configuration
	PenaltyConfig *PenaltyConfig
	
	// CompetitionConfig is the competition configuration
	CompetitionConfig *CompetitionConfig
}

// DefaultRewardConfig returns the default reward configuration.
func DefaultRewardConfig() *RewardConfig {
	return &RewardConfig{
		BaseBlockReward:     big.NewInt(2e18),  // 2 X
		DecayPeriod:         4_000_000,         // approximately 1 year
		DecayRate:           10,                // 10%
		MinBlockReward:      big.NewInt(1e17),  // 0.1 X
		MultiProducerConfig: DefaultMultiProducerRewardConfig(),
		OnlineRewardConfig:  DefaultOnlineRewardConfig(),
		ReputationConfig:    DefaultReputationConfig(),
		PenaltyConfig:       DefaultPenaltyConfig(),
		CompetitionConfig:   DefaultCompetitionConfig(),
	}
}

// MultiProducerRewardConfig represents the multi-producer reward configuration.
type MultiProducerRewardConfig struct {
	// SpeedRewardRatios defines the speed-based reward ratios (1st=100%, 2nd=60%, 3rd=30%)
	SpeedRewardRatios []float64
	
	// CandidateWindow is the candidate block collection window (how long to wait for other candidates after receiving the first block)
	CandidateWindow time.Duration
	
	// MaxCandidates is the maximum number of candidate blocks
	MaxCandidates int
	
	// QualityScoreWeight is the quality score weight
	QualityScoreWeight uint64
	
	// TimestampWeight is the timestamp weight
	TimestampWeight uint64
}

// DefaultMultiProducerRewardConfig returns the default multi-producer configuration.
func DefaultMultiProducerRewardConfig() *MultiProducerRewardConfig {
	return &MultiProducerRewardConfig{
		SpeedRewardRatios:  []float64{1.0, 0.6, 0.3}, // 100%, 60%, 30%
		CandidateWindow:    500 * time.Millisecond,   // 500ms window
		MaxCandidates:      3,
		QualityScoreWeight: 60,
		TimestampWeight:    40,
	}
}

// OnlineRewardConfig represents the online reward configuration.
type OnlineRewardConfig struct {
	// HeartbeatInterval is the heartbeat interval
	HeartbeatInterval time.Duration
	
	// HeartbeatTimeout is the heartbeat timeout (after which a node is marked as offline)
	HeartbeatTimeout time.Duration
	
	// HourlyReward is the online reward per hour
	HourlyReward *big.Int
	
	// MinOnlineDuration is the minimum online duration requirement (in hours)
	MinOnlineDuration time.Duration
	
	// MinUptimeRatio is the minimum uptime ratio requirement (percentage)
	MinUptimeRatio float64
}

// DefaultOnlineRewardConfig returns the default online reward configuration.
func DefaultOnlineRewardConfig() *OnlineRewardConfig {
	return &OnlineRewardConfig{
		HeartbeatInterval: 30 * time.Second,
		HeartbeatTimeout:  2 * time.Minute,
		HourlyReward:      big.NewInt(1e16), // 0.01 X
		MinOnlineDuration: 1 * time.Hour,
		MinUptimeRatio:    0.9, // 90%
	}
}

// ReputationConfig represents the reputation configuration.
type ReputationConfig struct {
	// InitialReputation is the initial reputation value
	InitialReputation int64
	
	// MaxReputation is the maximum reputation value
	MaxReputation int64
	
	// MinReputation is the minimum reputation value
	MinReputation int64
	
	// SuccessBonus is the reward for successful block production
	SuccessBonus int64
	
	// FailurePenalty is the penalty for block production failure
	FailurePenalty int64
	
	// MaliciousPenalty is the penalty for malicious behavior
	MaliciousPenalty int64
	
	// OfflinePenaltyPerHour is the offline penalty (per hour)
	OfflinePenaltyPerHour int64
	
	// RecoveryPerHour is the recovery rate (per hour)
	RecoveryPerHour int64
	
	// MaxPenaltyCount is the maximum penalty count (after which the node is excluded)
	MaxPenaltyCount int
	
	// DecayRate is the decay rate (per 24 hours)
	DecayRate float64
}

// DefaultReputationConfig returns the default reputation configuration.
func DefaultReputationConfig() *ReputationConfig {
	return &ReputationConfig{
		InitialReputation:     1000,
		MaxReputation:         10000,
		MinReputation:         0,
		SuccessBonus:          10,
		FailurePenalty:        20,
		MaliciousPenalty:      500,
		OfflinePenaltyPerHour: 10,
		RecoveryPerHour:       50,
		MaxPenaltyCount:       10,
		DecayRate:             0.01, // 1% per day
	}
}

// PenaltyConfig represents the penalty configuration.
type PenaltyConfig struct {
	// DoubleSignPenaltyPercent is the double signing penalty (percentage of balance)
	DoubleSignPenaltyPercent int
	
	// OfflinePenaltyPerHour is the offline penalty (per hour)
	OfflinePenaltyPerHour *big.Int
	
	// InvalidBlockPenalty is the invalid block penalty (fixed amount)
	InvalidBlockPenalty *big.Int
	
	// MaliciousPenaltyPercent is the malicious behavior penalty (percentage of balance)
	MaliciousPenaltyPercent int
}

// DefaultPenaltyConfig returns the default penalty configuration.
func DefaultPenaltyConfig() *PenaltyConfig {
	return &PenaltyConfig{
		DoubleSignPenaltyPercent: 50,                  // 50%
		OfflinePenaltyPerHour:    big.NewInt(1e16),    // 0.01 X
		InvalidBlockPenalty:      big.NewInt(1e17),    // 0.1 X
		MaliciousPenaltyPercent:  100,                 // 100%
	}
}

// CompetitionConfig represents the competition configuration.
type CompetitionConfig struct {
	// ReputationWeight is the reputation weight
	ReputationWeight float64
	
	// UptimeWeight is the uptime ratio weight
	UptimeWeight float64
	
	// BlockQualityWeight is the block quality weight
	BlockQualityWeight float64
	
	// ServiceQualityWeight is the service quality weight
	ServiceQualityWeight float64
	
	// RankingRewards defines the reward distribution ratios for ranking (top 10)
	RankingRewards []float64
}

// DefaultCompetitionConfig returns the default competition configuration.
func DefaultCompetitionConfig() *CompetitionConfig {
	return &CompetitionConfig{
		ReputationWeight:     0.30,
		UptimeWeight:         0.25,
		BlockQualityWeight:   0.25,
		ServiceQualityWeight: 0.20,
		RankingRewards:       []float64{0.30, 0.20, 0.15, 0.10, 0.10, 0.05, 0.05, 0.03, 0.01, 0.01},
	}
}

// BlockQualityConfig represents the block quality scoring configuration.
type BlockQualityConfig struct {
	// TxCountWeight is the transaction count weight (default 40%)
	TxCountWeight uint8
	
	// BlockSizeWeight is the block size weight (default 30%)
	BlockSizeWeight uint8
	
	// GasUtilizationWeight is the gas utilization weight (default 20%)
	GasUtilizationWeight uint8
	
	// TxDiversityWeight is the transaction diversity weight (default 10%)
	TxDiversityWeight uint8
	
	// MinTxThreshold is the minimum transaction count threshold (below which rewards are significantly reduced)
	MinTxThreshold uint64
	
	// TargetBlockSize is the target block size (in bytes)
	TargetBlockSize uint64
	
	// TargetGasUtilization is the target gas utilization ratio
	TargetGasUtilization float64
}

// DefaultBlockQualityConfig returns the default block quality configuration.
func DefaultBlockQualityConfig() *BlockQualityConfig {
	return &BlockQualityConfig{
		TxCountWeight:        40,
		BlockSizeWeight:      30,
		GasUtilizationWeight: 20,
		TxDiversityWeight:    10,
		MinTxThreshold:       5,           // at least 5 transactions
		TargetBlockSize:      1024 * 1024, // 1MB
		TargetGasUtilization: 0.8,         // 80% gas utilization
	}
}
