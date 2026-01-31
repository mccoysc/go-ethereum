package sgx

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	internalsgx "github.com/ethereum/go-ethereum/internal/sgx"
)

// Attestor extends the internal/sgx.Attestor interface with consensus-specific methods
type Attestor interface {
	internalsgx.Attestor

	// SignInEnclave 在 Enclave 内签名
	SignInEnclave(data []byte) ([]byte, error)

	// GetProducerID 获取出块者 ID
	GetProducerID() ([]byte, error)
}

// Verifier extends the internal/sgx.Verifier interface with consensus-specific methods
type Verifier interface {
	internalsgx.Verifier

	// VerifySignature 验证签名
	VerifySignature(data, signature, producerID []byte) error

	// ExtractProducerID 从 Quote 中提取生产者 ID
	ExtractProducerID(quote []byte) ([]byte, error)
}

// TxPool 交易池接口
type TxPool interface {
	// Pending 获取待处理交易
	Pending(enforceTips bool) map[common.Address][]*types.Transaction

	// PendingCount 获取待处理交易数量
	PendingCount() int

	// Add 添加交易
	Add(txs []*types.Transaction, local bool, sync bool) []error

	// Remove 移除交易
	Remove(txHash common.Hash)
}

// BlockChain 区块链接口
type BlockChain interface {
	// CurrentBlock 获取当前区块
	CurrentBlock() *types.Header

	// GetHeader 获取区块头
	GetHeader(hash common.Hash, number uint64) *types.Header

	// GetBlock 获取完整区块
	GetBlock(hash common.Hash, number uint64) *types.Block

	// InsertChain 插入区块链
	InsertChain(chain types.Blocks) (int, error)

	// HasBlock 检查区块是否存在
	HasBlock(hash common.Hash, number uint64) bool
}

// BlockBroadcaster 区块广播接口
type BlockBroadcaster interface {
	// BroadcastBlock 广播区块
	BroadcastBlock(block *types.Block, propagate bool)

	// BroadcastHeader 广播区块头
	BroadcastHeader(header *types.Header)
}

// RewardDistributor 奖励分配接口
type RewardDistributor interface {
	// DistributeRewards 分配奖励
	DistributeRewards(candidates []*BlockCandidate, totalFees *big.Int) ([]*CandidateReward, error)

	// CalculateOnlineReward 计算在线奖励
	CalculateOnlineReward(address common.Address, uptimeScore uint64) (*big.Int, error)

	// CalculateComprehensiveReward 计算综合奖励
	CalculateComprehensiveReward(address common.Address) (*ComprehensiveReward, error)
}

// ReputationManager 信誉管理接口
type ReputationManager interface {
	// GetReputation 获取节点信誉
	GetReputation(address common.Address) (*NodeReputation, error)

	// UpdateReputation 更新节点信誉
	UpdateReputation(address common.Address) error

	// IsExcluded 检查节点是否被排除
	IsExcluded(address common.Address) bool

	// GetNodePriority 获取节点优先级
	GetNodePriority(address common.Address) (uint64, error)
}

// UptimeTracker 在线率追踪接口
type UptimeTracker interface {
	// RecordHeartbeat 记录心跳
	RecordHeartbeat(msg *HeartbeatMessage) error

	// RecordTxParticipation 记录交易参与
	RecordTxParticipation(address common.Address, txCount, gasUsed uint64) error

	// RecordResponseTime 记录响应时间
	RecordResponseTime(address common.Address, responseMs uint64) error

	// CalculateUptimeScore 计算在线率评分
	CalculateUptimeScore(address common.Address) (*UptimeData, error)
}

// PenaltyManager 惩罚管理接口
type PenaltyManager interface {
	// RecordPenalty 记录惩罚
	RecordPenalty(address common.Address, penaltyType string, amount *big.Int, reason string) error

	// GetPenaltyCount 获取惩罚次数
	GetPenaltyCount(address common.Address) (uint64, error)

	// IsExcluded 检查是否被排除
	IsExcluded(address common.Address) bool

	// GetExclusionEndTime 获取排除结束时间
	GetExclusionEndTime(address common.Address) (time.Time, error)
}
