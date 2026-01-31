package sgx

import (
	"math/big"
	"time"
)

// Config SGX 共识引擎配置
type Config struct {
	// 基础配置
	MinBlockInterval time.Duration // 最小出块间隔
	MaxBlockInterval time.Duration // 最大出块间隔（用于心跳）
	MaxTxPerBlock    int           // 单区块最大交易数
	MaxGasPerBlock   uint64        // 单区块最大 Gas
	VerifyTimeout    time.Duration // 区块验证超时

	// 按需出块配置
	OnDemandEnabled bool   // 是否启用按需出块
	MinTxCount      int    // 触发出块的最小交易数
	MinGasTotal     uint64 // 触发出块的最小 Gas 总量

	// 多生产者收益配置
	CandidateWindowMs int       // 候选区块收集窗口（毫秒）
	MaxCandidates     int       // 最大候选区块数（前N名参与收益分配）
	SpeedRewardRatios []float64 // 速度基础奖励比例（第1名, 第2名, 第3名）

	// 区块质量评分配置
	QualityConfig *QualityConfig

	// 在线率计算配置
	UptimeConfig *UptimeConfig

	// 信誉系统配置
	ReputationConfig *ReputationConfig

	// 惩罚机制配置
	PenaltyConfig *PenaltyConfig

	// 奖励机制配置
	RewardConfig *RewardConfig
}

// QualityConfig 区块质量评分配置
type QualityConfig struct {
	TxCountWeight         float64 // 交易数量权重 (%)
	BlockSizeWeight       float64 // 区块大小权重 (%)
	GasUtilizationWeight  float64 // Gas 利用率权重 (%)
	TxDiversityWeight     float64 // 交易多样性权重 (%)
	MinTxThreshold        int     // 最小交易数阈值
	TargetBlockSize       uint64  // 目标区块大小（字节）
	TargetGasUtilization  float64 // 目标 Gas 利用率
}

// UptimeConfig 在线率计算配置
type UptimeConfig struct {
	HeartbeatWeight       float64       // SGX 心跳权重 (%)
	ConsensusWeight       float64       // 多节点共识权重 (%)
	TxParticipationWeight float64       // 交易参与度权重 (%)
	ResponseWeight        float64       // 响应时间权重 (%)
	HeartbeatInterval     time.Duration // 心跳间隔
	ConsensusThreshold    float64       // 共识阈值（例如 2/3）
	ResponseTimeTarget    uint64        // 目标响应时间（毫秒）
}

// ReputationConfig 信誉系统配置
type ReputationConfig struct {
	UptimeWeight      float64       // 在线率权重 (%)
	SuccessRateWeight float64       // 成功率权重 (%)
	PenaltyWeight     float64       // 惩罚权重 (%)
	MinUptimeScore    uint64        // 最小在线率评分
	MinSuccessRate    float64       // 最小成功率
	UpdateInterval    time.Duration // 更新间隔
}

// PenaltyConfig 惩罚机制配置
type PenaltyConfig struct {
	LowQualityThreshold  uint64        // 低质量区块阈值
	EmptyBlockThreshold  uint64        // 空区块阈值
	OfflineThreshold     time.Duration // 离线阈值
	PenaltyAmount        *big.Int      // 惩罚金额
	ExclusionPeriod      time.Duration // 排除期
	RecoveryPeriod       time.Duration // 恢复期
}

// RewardConfig 奖励机制配置
type RewardConfig struct {
	BaseBlockReward       *big.Int      // 基础出块奖励
	OnlineRewardPerEpoch  *big.Int      // 每个周期的在线奖励
	QualityBonusRate      float64       // 质量奖励比率
	ServiceBonusRate      float64       // 服务奖励比率
	HistoricalBonusRate   float64       // 历史贡献奖励比率
	EpochDuration         time.Duration // 奖励周期
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		// 基础配置
		MinBlockInterval: 1 * time.Second,
		MaxBlockInterval: 60 * time.Second,
		MaxTxPerBlock:    1000,
		MaxGasPerBlock:   30000000,
		VerifyTimeout:    10 * time.Second,

		// 按需出块配置
		OnDemandEnabled: true,
		MinTxCount:      1,
		MinGasTotal:     21000,

		// 多生产者收益配置
		CandidateWindowMs: 500,
		MaxCandidates:     3,
		SpeedRewardRatios: []float64{1.0, 0.6, 0.3},

		// 区块质量评分配置
		QualityConfig: &QualityConfig{
			TxCountWeight:        40.0,
			BlockSizeWeight:      30.0,
			GasUtilizationWeight: 20.0,
			TxDiversityWeight:    10.0,
			MinTxThreshold:       5,
			TargetBlockSize:      1048576, // 1MB
			TargetGasUtilization: 0.8,     // 80%
		},

		// 在线率计算配置
		UptimeConfig: &UptimeConfig{
			HeartbeatWeight:       40.0,
			ConsensusWeight:       30.0,
			TxParticipationWeight: 20.0,
			ResponseWeight:        10.0,
			HeartbeatInterval:     30 * time.Second,
			ConsensusThreshold:    0.67, // 2/3
			ResponseTimeTarget:    100,  // 100ms
		},

		// 信誉系统配置
		ReputationConfig: &ReputationConfig{
			UptimeWeight:      60.0,
			SuccessRateWeight: 30.0,
			PenaltyWeight:     10.0,
			MinUptimeScore:    6000,  // 60%
			MinSuccessRate:    0.8,   // 80%
			UpdateInterval:    1 * time.Hour,
		},

		// 惩罚机制配置
		PenaltyConfig: &PenaltyConfig{
			LowQualityThreshold: 3000, // 低于 30% 的质量评分
			EmptyBlockThreshold: 5,    // 连续 5 个空区块
			OfflineThreshold:    5 * time.Minute,
			PenaltyAmount:       big.NewInt(1e18), // 1 ETH
			ExclusionPeriod:     24 * time.Hour,
			RecoveryPeriod:      7 * 24 * time.Hour,
		},

		// 奖励机制配置
		RewardConfig: &RewardConfig{
			BaseBlockReward:      big.NewInt(2e18),    // 2 ETH
			OnlineRewardPerEpoch: big.NewInt(1e17),    // 0.1 ETH
			QualityBonusRate:     0.5,                 // 50% 质量奖励
			ServiceBonusRate:     0.3,                 // 30% 服务奖励
			HistoricalBonusRate:  0.2,                 // 20% 历史贡献奖励
			EpochDuration:        24 * time.Hour,      // 24小时周期
		},
	}
}

// Validate 验证配置有效性
func (c *Config) Validate() error {
	if c.MinBlockInterval <= 0 {
		return ErrInvalidConfig
	}
	if c.MaxBlockInterval < c.MinBlockInterval {
		return ErrInvalidConfig
	}
	if c.MaxTxPerBlock <= 0 {
		return ErrInvalidConfig
	}
	if c.MaxGasPerBlock == 0 {
		return ErrInvalidConfig
	}
	if c.QualityConfig == nil {
		return ErrInvalidConfig
	}
	if c.UptimeConfig == nil {
		return ErrInvalidConfig
	}
	if c.ReputationConfig == nil {
		return ErrInvalidConfig
	}
	if c.PenaltyConfig == nil {
		return ErrInvalidConfig
	}
	if c.RewardConfig == nil {
		return ErrInvalidConfig
	}
	return nil
}
