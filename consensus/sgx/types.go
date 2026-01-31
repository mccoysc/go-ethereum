// Package sgx implements the PoA-SGX consensus engine for X Chain
package sgx

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

// SGXExtra 区块头扩展数据结构
// 存储在 header.Extra 字段中
type SGXExtra struct {
	SGXQuote      []byte `json:"sgxQuote"`      // 出块节点的 SGX Quote（证明代码完整性）
	ProducerID    []byte `json:"producerId"`    // 出块节点标识（从 SGX Quote 中提取的公钥哈希）
	AttestationTS uint64 `json:"attestationTs"` // SGX 证明时间戳
	Signature     []byte `json:"signature"`     // 区块签名（使用节点私钥签名）
}

// Encode 序列化 SGX Extra 数据
func (e *SGXExtra) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(e)
}

// DecodeSGXExtra 反序列化 SGX Extra 数据
func DecodeSGXExtra(data []byte) (*SGXExtra, error) {
	var extra SGXExtra
	if err := rlp.DecodeBytes(data, &extra); err != nil {
		return nil, err
	}
	return &extra, nil
}

// BlockQuality 区块质量评分
type BlockQuality struct {
	// 原始指标
	TxCount        uint64 `json:"txCount"`        // 交易数量
	BlockSize      uint64 `json:"blockSize"`      // 区块大小（字节）
	GasUsed        uint64 `json:"gasUsed"`        // Gas 使用量
	DiversityScore uint64 `json:"diversityScore"` // 交易来源多样性

	// 评分（0-10000）
	TxCountScore       uint64 `json:"txCountScore"`       // 交易数量评分
	BlockSizeScore     uint64 `json:"blockSizeScore"`     // 区块大小评分
	GasUtilScore       uint64 `json:"gasUtilScore"`       // Gas 利用率评分
	DiversityScoreNorm uint64 `json:"diversityScoreNorm"` // 多样性评分

	// 总评分和倍数
	TotalScore       uint64  `json:"totalScore"`       // 总评分（加权）
	RewardMultiplier float64 `json:"rewardMultiplier"` // 收益倍数（0.1 - 2.0）

	// 新交易数（用于多生产者收益分配）
	NewTxCount uint64 `json:"newTxCount"` // 该区块中未被第一名包含的新交易数
}

// BlockCandidate 候选区块
type BlockCandidate struct {
	Block      *types.Block   `json:"block"`
	Producer   common.Address `json:"producer"`
	ReceivedAt time.Time      `json:"receivedAt"` // 收到区块的时间
	Quality    *BlockQuality  `json:"quality"`    // 区块质量评分
	Rank       int            `json:"rank"`       // 排名 (1, 2, 3)
}

// CandidateReward 候选区块收益
type CandidateReward struct {
	Candidate       *BlockCandidate `json:"candidate"`
	SpeedRatio      float64         `json:"speedRatio"`      // 速度奖励比例
	QualityMulti    float64         `json:"qualityMulti"`    // 质量倍数
	FinalMultiplier float64         `json:"finalMultiplier"` // 最终收益倍数 = SpeedRatio × QualityMulti
	Reward          *big.Int        `json:"reward"`          // 最终收益
}

// NodeReputation 节点信誉数据
type NodeReputation struct {
	Address         common.Address `json:"address"`
	UptimeScore     uint64         `json:"uptimeScore"`     // 在线率评分（0-10000）
	SuccessRate     float64        `json:"successRate"`     // 出块成功率
	PenaltyCount    uint64         `json:"penaltyCount"`    // 惩罚次数
	ReputationScore uint64         `json:"reputationScore"` // 综合信誉评分（0-10000）
	LastUpdateTime  time.Time      `json:"lastUpdateTime"`
}

// UptimeData 节点在线率数据
type UptimeData struct {
	Address              common.Address `json:"address"`
	HeartbeatScore       uint64         `json:"heartbeatScore"`       // SGX 心跳评分（0-10000）
	ConsensusScore       uint64         `json:"consensusScore"`       // 多节点共识评分（0-10000）
	TxParticipationScore uint64         `json:"txParticipationScore"` // 交易参与度评分（0-10000）
	ResponseScore        uint64         `json:"responseScore"`        // 响应时间评分（0-10000）
	ComprehensiveScore   uint64         `json:"comprehensiveScore"`   // 综合在线率评分（0-10000）
	LastUpdateTime       time.Time      `json:"lastUpdateTime"`
}

// HeartbeatMessage SGX 签名的心跳消息
type HeartbeatMessage struct {
	NodeID    common.Address `json:"nodeId"`
	Timestamp uint64         `json:"timestamp"`
	SGXQuote  []byte         `json:"sgxQuote"`  // SGX Quote 证明
	Signature []byte         `json:"signature"` // 签名
}

// TxParticipation 交易参与数据
type TxParticipation struct {
	Address        common.Address `json:"address"`
	ProcessedTxs   uint64         `json:"processedTxs"` // 处理的交易数
	ProcessedGas   uint64         `json:"processedGas"` // 处理的 Gas 总量
	TotalBlocks    uint64         `json:"totalBlocks"`  // 总区块数
	LastUpdateTime time.Time      `json:"lastUpdateTime"`
}

// ResponseTimeData 响应时间数据
type ResponseTimeData struct {
	Address        common.Address `json:"address"`
	AvgResponseMs  uint64         `json:"avgResponseMs"` // 平均响应时间（毫秒）
	P50ResponseMs  uint64         `json:"p50ResponseMs"` // P50 响应时间
	P95ResponseMs  uint64         `json:"p95ResponseMs"` // P95 响应时间
	P99ResponseMs  uint64         `json:"p99ResponseMs"` // P99 响应时间
	SampleCount    uint64         `json:"sampleCount"`   // 样本数量
	LastUpdateTime time.Time      `json:"lastUpdateTime"`
}

// PenaltyRecord 惩罚记录
type PenaltyRecord struct {
	Address       common.Address `json:"address"`
	PenaltyType   string         `json:"penaltyType"`   // 惩罚类型
	PenaltyAmount *big.Int       `json:"penaltyAmount"` // 惩罚金额
	Timestamp     time.Time      `json:"timestamp"`
	Reason        string         `json:"reason"` // 惩罚原因
}

// ServiceQualityData 服务质量数据
type ServiceQualityData struct {
	Address         common.Address `json:"address"`
	ResponseScore   uint64         `json:"responseScore"`   // 响应时间评分（0-10000）
	ThroughputScore uint64         `json:"throughputScore"` // 吞吐量评分（0-10000）
	QualityScore    uint64         `json:"qualityScore"`    // 综合服务质量评分（0-10000）
	LastUpdateTime  time.Time      `json:"lastUpdateTime"`
}

// TransactionVolumeData 交易量数据
type TransactionVolumeData struct {
	Address        common.Address `json:"address"`
	TxCount        uint64         `json:"txCount"`     // 交易数量
	GasUsed        uint64         `json:"gasUsed"`     // Gas 使用量
	MarketShare    float64        `json:"marketShare"` // 市场份额（0-1）
	VolumeScore    uint64         `json:"volumeScore"` // 交易量评分（0-10000）
	LastUpdateTime time.Time      `json:"lastUpdateTime"`
}

// HistoricalContribution 历史贡献数据
type HistoricalContribution struct {
	Address                common.Address `json:"address"`
	TotalBlocks            uint64         `json:"totalBlocks"`            // 历史总区块数
	TotalTxs               uint64         `json:"totalTxs"`               // 历史总交易数
	ActiveDays             uint64         `json:"activeDays"`             // 活跃天数
	ContributionMultiplier float64        `json:"contributionMultiplier"` // 贡献倍数（1.0 - 2.0）
	FirstContributionTime  time.Time      `json:"firstContributionTime"`  // 首次贡献时间
	LastUpdateTime         time.Time      `json:"lastUpdateTime"`
}

// ComprehensiveReward 综合奖励数据
type ComprehensiveReward struct {
	Address         common.Address `json:"address"`
	BlockReward     *big.Int       `json:"blockReward"`     // 出块奖励
	OnlineReward    *big.Int       `json:"onlineReward"`    // 在线奖励
	QualityBonus    *big.Int       `json:"qualityBonus"`    // 质量奖励
	ServiceBonus    *big.Int       `json:"serviceBonus"`    // 服务奖励
	HistoricalBonus *big.Int       `json:"historicalBonus"` // 历史贡献奖励
	TotalReward     *big.Int       `json:"totalReward"`     // 总奖励
	Timestamp       time.Time      `json:"timestamp"`
}

// ProducerPenalty 出块者惩罚数据
type ProducerPenalty struct {
	Address         common.Address `json:"address"`
	LowQualityCount uint64         `json:"lowQualityCount"` // 低质量区块数
	EmptyBlockCount uint64         `json:"emptyBlockCount"` // 空区块数
	TotalPenalty    *big.Int       `json:"totalPenalty"`    // 总惩罚
	ExcludedUntil   time.Time      `json:"excludedUntil"`   // 排除截止时间
	LastPenaltyTime time.Time      `json:"lastPenaltyTime"`
}

// ValueAddedService 增值服务数据
type ValueAddedService struct {
	ServiceID      string         `json:"serviceId"`
	Provider       common.Address `json:"provider"`
	ServiceType    string         `json:"serviceType"` // 服务类型
	PremiumRate    float64        `json:"premiumRate"` // 溢价率
	Enabled        bool           `json:"enabled"`
	LastUpdateTime time.Time      `json:"lastUpdateTime"`
}
