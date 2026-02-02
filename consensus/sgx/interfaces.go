package sgx

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	internalsgx "github.com/ethereum/go-ethereum/internal/sgx"
	"github.com/ethereum/go-ethereum/params"
)

// Attestor SGX 证明接口
// 用于生成 SGX Quote 和签名区块
type Attestor interface {
	// GenerateQuote 生成 SGX Quote
	// data: 要包含在 Quote 中的数据（最大 64 字节）
	GenerateQuote(data []byte) ([]byte, error)

	// SignInEnclave 在 Enclave 内签名数据
	// 返回 ECDSA 签名（65 字节）
	SignInEnclave(data []byte) ([]byte, error)

	// GetProducerID 获取出块者 ID（以太坊地址，20 字节）
	GetProducerID() ([]byte, error)
	
	// GetSigningPublicKey 获取签名公钥（用于写入Quote）
	// 返回未压缩格式的secp256k1公钥（65字节：0x04 + X + Y）
	GetSigningPublicKey() []byte
}

// Verifier SGX 验证接口
// 用于验证 SGX Quote 和区块签名
type Verifier interface {
	// VerifyQuote 验证 SGX Quote 的有效性
	// 包括签名验证和 TCB 状态检查
	VerifyQuote(quote []byte) error

	// VerifyQuoteComplete 执行完整的 Quote 验证并返回所有提取的数据
	// 这与 gramine sgx-quote-verify.js 的 verifyQuote() 函数相匹配
	// 输入可以是: RA-TLS 证书 (PEM 格式), 原始 quote 字节, 或 Base64 编码的 quote
	// options 可以包括: apiKey (Intel SGX API key), cacheDir (证书缓存目录)
	VerifyQuoteComplete(input []byte, options map[string]interface{}) (*internalsgx.QuoteVerificationResult, error)

	// VerifySignature 验证 ECDSA 签名
	// data: 被签名的数据
	// signature: ECDSA 签名（65 字节）
	// publicKey: 签名公钥（65字节未压缩格式：0x04 + X + Y）
	VerifySignature(data, signature, publicKey []byte) error

	// ExtractProducerID 从 SGX Quote 中提取出块者 ID
	ExtractProducerID(quote []byte) ([]byte, error)
	
	// ExtractQuoteUserData 从 SGX Quote 中提取 userData 字段
	// 用于验证Quote中嵌入的数据（如区块哈希或公钥）
	ExtractQuoteUserData(quote []byte) ([]byte, error)
	
	// ExtractPublicKeyFromQuote 从 SGX Quote 的 ReportData 中提取公钥
	// 返回未压缩格式的secp256k1公钥（65字节：0x04 + X + Y）
	ExtractPublicKeyFromQuote(quote []byte) ([]byte, error)
	
	// ExtractInstanceID 从 SGX Quote 中提取CPU实例ID
	// Instance ID用于确保一个CPU只能作为一个生产者
	ExtractInstanceID(quote []byte) ([]byte, error)
}

// TxPool 交易池接口
type TxPool interface {
	// Pending 获取待处理交易
	Pending(enforceTips bool) map[common.Address][]*types.Transaction

	// PendingCount 获取待处理交易数量
	PendingCount() int

	// Add 添加交易
	Add(txs []*types.Transaction, sync bool) []error

	// Remove 移除交易
	Remove(txHash common.Hash)
}

// BlockChain 区块链接口
type BlockChain interface {
	// Config 获取区块链配置
	Config() *params.ChainConfig

	// CurrentBlock 获取当前区块
	CurrentBlock() *types.Header

	// CurrentHeader 获取当前区块头（与 CurrentBlock 相同）
	CurrentHeader() *types.Header

	// GetHeader 获取区块头
	GetHeader(hash common.Hash, number uint64) *types.Header

	// GetHeaderByNumber 根据区块号获取区块头
	GetHeaderByNumber(number uint64) *types.Header

	// GetHeaderByHash 根据哈希获取区块头
	GetHeaderByHash(hash common.Hash) *types.Header

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
	// networkObservers: 网络中的总观测者数量
	// networkTotalTxs: 网络总交易数
	// networkTotalGas: 网络总 Gas 使用量
	CalculateUptimeScore(address common.Address, networkObservers int, networkTotalTxs, networkTotalGas uint64) (*UptimeData, error)
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
