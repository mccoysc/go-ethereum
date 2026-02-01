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

// RewardConfig 奖励配置
type RewardConfig struct {
	// 基础区块奖励
	BaseBlockReward *big.Int
	
	// 奖励衰减周期（区块数）
	DecayPeriod uint64
	
	// 衰减率（百分比）
	DecayRate uint64
	
	// 最小区块奖励
	MinBlockReward *big.Int
	
	// 多生产者奖励配置
	MultiProducerConfig *MultiProducerRewardConfig
	
	// 在线奖励配置
	OnlineRewardConfig *OnlineRewardConfig
	
	// 声誉配置
	ReputationConfig *ReputationConfig
	
	// 惩罚配置
	PenaltyConfig *PenaltyConfig
	
	// 竞争配置
	CompetitionConfig *CompetitionConfig
}

// DefaultRewardConfig 默认奖励配置
func DefaultRewardConfig() *RewardConfig {
	return &RewardConfig{
		BaseBlockReward:     big.NewInt(2e18),  // 2 X
		DecayPeriod:         4_000_000,         // 约 1 年
		DecayRate:           10,                // 10%
		MinBlockReward:      big.NewInt(1e17),  // 0.1 X
		MultiProducerConfig: DefaultMultiProducerRewardConfig(),
		OnlineRewardConfig:  DefaultOnlineRewardConfig(),
		ReputationConfig:    DefaultReputationConfig(),
		PenaltyConfig:       DefaultPenaltyConfig(),
		CompetitionConfig:   DefaultCompetitionConfig(),
	}
}

// MultiProducerRewardConfig 多生产者奖励配置
type MultiProducerRewardConfig struct {
	// 速度基础奖励比例（第1名=100%, 第2名=60%, 第3名=30%）
	SpeedRewardRatios []float64
	
	// 候选区块收集窗口（收到第一个区块后等待多久收集其他候选）
	CandidateWindow time.Duration
	
	// 最大候选区块数
	MaxCandidates int
	
	// 质量评分权重
	QualityScoreWeight uint64
	
	// 时间戳权重
	TimestampWeight uint64
}

// DefaultMultiProducerRewardConfig 默认多生产者配置
func DefaultMultiProducerRewardConfig() *MultiProducerRewardConfig {
	return &MultiProducerRewardConfig{
		SpeedRewardRatios:  []float64{1.0, 0.6, 0.3}, // 100%, 60%, 30%
		CandidateWindow:    500 * time.Millisecond,   // 500ms 窗口
		MaxCandidates:      3,
		QualityScoreWeight: 60,
		TimestampWeight:    40,
	}
}

// OnlineRewardConfig 在线奖励配置
type OnlineRewardConfig struct {
	// 心跳间隔
	HeartbeatInterval time.Duration
	
	// 心跳超时（标记为离线）
	HeartbeatTimeout time.Duration
	
	// 每小时在线奖励
	HourlyReward *big.Int
	
	// 最小在线时长要求（小时）
	MinOnlineDuration time.Duration
	
	// 最小在线率要求（百分比）
	MinUptimeRatio float64
}

// DefaultOnlineRewardConfig 默认在线奖励配置
func DefaultOnlineRewardConfig() *OnlineRewardConfig {
	return &OnlineRewardConfig{
		HeartbeatInterval: 30 * time.Second,
		HeartbeatTimeout:  2 * time.Minute,
		HourlyReward:      big.NewInt(1e16), // 0.01 X
		MinOnlineDuration: 1 * time.Hour,
		MinUptimeRatio:    0.9, // 90%
	}
}

// ReputationConfig 声誉配置
type ReputationConfig struct {
	// 初始声誉值
	InitialReputation int64
	
	// 最大声誉值
	MaxReputation int64
	
	// 最小声誉值
	MinReputation int64
	
	// 成功出块奖励
	SuccessBonus int64
	
	// 出块失败惩罚
	FailurePenalty int64
	
	// 恶意行为惩罚
	MaliciousPenalty int64
	
	// 离线惩罚（每小时）
	OfflinePenaltyPerHour int64
	
	// 恢复速度（每小时）
	RecoveryPerHour int64
	
	// 最大惩罚次数（超过后排除）
	MaxPenaltyCount int
	
	// 衰减率（每 24 小时）
	DecayRate float64
}

// DefaultReputationConfig 默认声誉配置
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

// PenaltyConfig 惩罚配置
type PenaltyConfig struct {
	// 双重签名惩罚（余额的百分比）
	DoubleSignPenaltyPercent int
	
	// 离线惩罚（每小时）
	OfflinePenaltyPerHour *big.Int
	
	// 无效区块惩罚（固定金额）
	InvalidBlockPenalty *big.Int
	
	// 恶意行为惩罚（余额的百分比）
	MaliciousPenaltyPercent int
}

// DefaultPenaltyConfig 默认惩罚配置
func DefaultPenaltyConfig() *PenaltyConfig {
	return &PenaltyConfig{
		DoubleSignPenaltyPercent: 50,                  // 50%
		OfflinePenaltyPerHour:    big.NewInt(1e16),    // 0.01 X
		InvalidBlockPenalty:      big.NewInt(1e17),    // 0.1 X
		MaliciousPenaltyPercent:  100,                 // 100%
	}
}

// CompetitionConfig 竞争配置
type CompetitionConfig struct {
	// 声誉权重
	ReputationWeight float64
	
	// 在线率权重
	UptimeWeight float64
	
	// 区块质量权重
	BlockQualityWeight float64
	
	// 服务质量权重
	ServiceQualityWeight float64
	
	// 排名奖励分配比例（前 10 名）
	RankingRewards []float64
}

// DefaultCompetitionConfig 默认竞争配置
func DefaultCompetitionConfig() *CompetitionConfig {
	return &CompetitionConfig{
		ReputationWeight:     0.30,
		UptimeWeight:         0.25,
		BlockQualityWeight:   0.25,
		ServiceQualityWeight: 0.20,
		RankingRewards:       []float64{0.30, 0.20, 0.15, 0.10, 0.10, 0.05, 0.05, 0.03, 0.01, 0.01},
	}
}

// BlockQualityConfig 区块质量评分配置
type BlockQualityConfig struct {
	// 交易数量权重 (默认 40%)
	TxCountWeight uint8
	
	// 区块大小权重 (默认 30%)
	BlockSizeWeight uint8
	
	// Gas 利用率权重 (默认 20%)
	GasUtilizationWeight uint8
	
	// 交易多样性权重 (默认 10%)
	TxDiversityWeight uint8
	
	// 最小交易数阈值（低于此值收益大幅降低）
	MinTxThreshold uint64
	
	// 目标区块大小（字节）
	TargetBlockSize uint64
	
	// 目标 Gas 利用率
	TargetGasUtilization float64
}

// DefaultBlockQualityConfig 默认区块质量配置
func DefaultBlockQualityConfig() *BlockQualityConfig {
	return &BlockQualityConfig{
		TxCountWeight:        40,
		BlockSizeWeight:      30,
		GasUtilizationWeight: 20,
		TxDiversityWeight:    10,
		MinTxThreshold:       5,           // 至少 5 笔交易
		TargetBlockSize:      1024 * 1024, // 1MB
		TargetGasUtilization: 0.8,         // 80% Gas 利用率
	}
}
