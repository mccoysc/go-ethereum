package sgx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	internalsgx "github.com/ethereum/go-ethereum/internal/sgx"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/holiman/uint256"
)

// SGXEngine PoA-SGX 共识引擎
type SGXEngine struct {
	config *Config

	// 外部依赖
	attestor Attestor
	verifier Verifier

	// 内部组件
	blockProducer       *BlockProducer
	onDemandController  *OnDemandController
	blockQualityScorer  *BlockQualityScorer
	multiProducerReward *MultiProducerRewardCalculator
	forkChoiceRule      *ForkChoiceRule
	reorgHandler        *ReorgHandler
	uptimeCalculator    *UptimeCalculator
	reputationSystem    *ReputationSystem
	penaltyManager      *PenaltyManagerImpl
	onlineRewardCalc    *OnlineRewardCalculator
	nodeSelector        *NodeSelector
	comprehensiveReward *ComprehensiveRewardCalculator

	// 同步
	mu sync.RWMutex

	// 状态
	started bool
}

// New 创建 SGX 共识引擎
func New(config *Config, attestor Attestor, verifier Verifier) *SGXEngine {
	if config == nil {
		config = DefaultConfig()
	}

	engine := &SGXEngine{
		config:   config,
		attestor: attestor,
		verifier: verifier,
	}

	// 初始化内部组件
	engine.blockQualityScorer = NewBlockQualityScorer(config.QualityConfig)
	engine.multiProducerReward = NewMultiProducerRewardCalculator(config, engine.blockQualityScorer)
	engine.forkChoiceRule = NewForkChoiceRule()
	engine.reorgHandler = NewReorgHandler()
	engine.uptimeCalculator = NewUptimeCalculator(config.UptimeConfig)
	engine.penaltyManager = NewPenaltyManager(config.PenaltyConfig)
	engine.reputationSystem = NewReputationSystem(config.ReputationConfig, engine.uptimeCalculator, engine.penaltyManager)
	engine.onlineRewardCalc = NewOnlineRewardCalculator(config.RewardConfig)
	engine.nodeSelector = NewNodeSelector(engine.reputationSystem)
	engine.comprehensiveReward = NewComprehensiveRewardCalculator(config.RewardConfig)
	engine.onDemandController = NewOnDemandController(config)

	return engine
}

// NewFromParams creates an SGX consensus engine from genesis params configuration
func NewFromParams(paramsConfig *params.SGXConfig, db ethdb.Database) *SGXEngine {
	log.Info("=== Initializing SGX Consensus Engine ===")
	
	// Check environment - MUST be running under Gramine
	gramineVersion := os.Getenv("GRAMINE_VERSION")
	if gramineVersion == "" {
		log.Crit("SECURITY: GRAMINE_VERSION environment variable not set. " +
			"SGX consensus engine REQUIRES Gramine environment. " +
			"For testing: export GRAMINE_VERSION=test")
	}
	log.Info("Running under Gramine", "version", gramineVersion)
	
	// Step 1: Validate manifest integrity (signature verification)
	// 必须验证，无论任何情况
	log.Info("Step 1: Validating manifest integrity...")
	if err := internalsgx.ValidateManifestIntegrity(); err != nil {
		log.Crit("Manifest validation FAILED", "error", err)
	}
	log.Info("✓ Manifest signature verified")
	
	// Step 2: Use contract addresses from genesis config
	log.Info("Step 2: Using contract addresses from genesis...")
	
	governanceAddr := paramsConfig.GovernanceContract
	securityAddr := paramsConfig.SecurityConfig
	incentiveAddr := paramsConfig.IncentiveContract
	
	log.Info("Contract addresses from genesis",
		"governance", governanceAddr.Hex(),
		"security", securityAddr.Hex(),
		"incentive", incentiveAddr.Hex())
	
	// Use default config as base
	config := DefaultConfig()
	
	log.Info("SGX Configuration",
		"period", paramsConfig.Period,
		"epoch", paramsConfig.Epoch,
		"governance", governanceAddr.Hex(),
		"security", securityAddr.Hex(),
		"incentive", incentiveAddr.Hex())
	
	// Step 3: Load all modules
	log.Info("Step 3: Loading SGX modules...")
	log.Info("Loading Module 01: SGX Attestation")
	log.Info("Loading Module 02: SGX Consensus Engine")
	log.Info("Loading Module 03: Incentive Mechanism")
	log.Info("Loading Module 04: Precompiled Contracts (0x8000-0x8009)")
	log.Info("Loading Module 05: Governance System")
	log.Info("Loading Module 06: Encrypted Storage")
	log.Info("Loading Module 07: Gramine Integration")
	
	// Step 4: Create attestor and verifier
	log.Info("Step 4: Initializing SGX attestation...")
	
	// Use Gramine SGX attestation
	log.Info("Using Gramine SGX attestation")
	attestor, err := internalsgx.NewGramineAttestor()
	if err != nil {
		log.Crit("Failed to create Gramine attestor", "error", err)
	}
	
	// Create DCAP verifier directly to get concrete type
	verifier := internalsgx.NewDCAPVerifier(true)
	
	log.Info("=== SGX Consensus Engine Initialized ===")
	log.Info("Next: Contract addresses", 
		"governance", governanceAddr.Hex(),
		"security", securityAddr.Hex(),
		"incentive", incentiveAddr.Hex())
	
	return New(config, attestor, verifier)
}

// Author 从区块头中提取出块者地址
func (e *SGXEngine) Author(header *types.Header) (common.Address, error) {
	if len(header.Extra) < 32 {
		return common.Address{}, ErrInvalidExtra
	}

	extra, err := DecodeSGXExtra(header.Extra)
	if err != nil {
		return common.Address{}, err
	}

	// 从 ProducerID 派生地址
	return common.BytesToAddress(crypto.Keccak256(extra.ProducerID)[:20]), nil
}

// VerifyHeader 验证单个区块头
func (e *SGXEngine) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header) error {
	return e.verifyHeader(chain, header, nil)
}

// VerifyHeaders 批量验证区块头
func (e *SGXEngine) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	go func() {
		for i, header := range headers {
			var parent *types.Header
			if i > 0 {
				parent = headers[i-1]
			}

			err := e.verifyHeader(chain, header, parent)

			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()

	return abort, results
}

// verifyHeader 内部验证逻辑
func (e *SGXEngine) verifyHeader(chain consensus.ChainHeaderReader, header *types.Header, parent *types.Header) error {
	// 验证时间戳
	if header.Time > uint64(time.Now().Add(15*time.Second).Unix()) {
		return consensus.ErrFutureBlock
	}

	// 获取父区块
	if parent == nil {
		parent = chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	}
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}

	// 验证时间戳顺序
	if header.Time <= parent.Time {
		return ErrInvalidTimestamp
	}

	// 验证难度（PoA-SGX 固定为 1）
	if header.Difficulty.Cmp(big.NewInt(1)) != 0 {
		return ErrInvalidDifficulty
	}

	// 解析 Extra 字段
	extra, err := DecodeSGXExtra(header.Extra)
	if err != nil {
		return ErrInvalidExtra
	}

	// 验证 SGX Quote（硬件级签名验证）
	// Quote验证包括：
	// 1. 硬件签名验证（Intel/AMD CPU签名，不可伪造）
	// 2. MRENCLAVE验证（确保enclave代码未被篡改）
	// 3. TCB状态检查
	// Quote的userData包含seal hash，在VerifyUncles中验证
	if err := e.verifier.VerifyQuote(extra.SGXQuote); err != nil {
		return ErrQuoteVerificationFailed
	}

	// 验证ProducerID：从Quote中提取CPU Instance ID并与声明的ProducerID比较
	// Instance ID确保一个物理CPU只能作为一个生产者，防止Sybil攻击
	instanceID, err := e.verifier.ExtractInstanceID(extra.SGXQuote)
	if err != nil {
		return fmt.Errorf("failed to extract instance ID from quote: %w", err)
	}
	
	// ProducerID应该是Instance ID的前20字节
	expectedProducerID := make([]byte, 20)
	if len(instanceID) >= 20 {
		copy(expectedProducerID, instanceID[:20])
	} else {
		copy(expectedProducerID, instanceID)
	}
	
	if !bytes.Equal(expectedProducerID, extra.ProducerID) {
		return ErrInvalidProducerID
	}

	// Quote本身就是签名，不需要额外的ECDSA验证

	return nil
}

// VerifyUncles 验证叔块（PoA-SGX 不支持叔块）
// 同时验证SGX Quote中的userData是否匹配seal hash
func (e *SGXEngine) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	// PoA-SGX不支持叔块
	if len(block.Uncles()) > 0 {
		return errors.New("uncles not allowed in PoA-SGX")
	}
	
	// ===== 关键安全验证：Quote userData必须匹配seal hash =====
	// 这确保了：
	// 1. Quote确实是为这个特定区块生成的
	// 2. 区块数据未被篡改
	// 3. 防止恶意节点用其他区块的Quote替换
	
	header := block.Header()
	extra, err := DecodeSGXExtra(header.Extra)
	if err != nil {
		return ErrInvalidExtra
	}
	
	// 计算seal hash（不包含Extra的区块哈希）
	// 这与Seal()时使用的哈希一致
	sealHash := e.SealHash(header)
	
	// 从Quote中提取userData
	userData, err := e.verifier.ExtractQuoteUserData(extra.SGXQuote)
	if err != nil {
		return errors.New("failed to extract userData from Quote")
	}
	
	// 验证userData必须等于seal hash
	if len(userData) < 32 {
		return fmt.Errorf("invalid userData length: got %d, expected at least 32", len(userData))
	}
	
	// 比较前32字节（seal hash）
	if !bytes.Equal(userData[:32], sealHash.Bytes()) {
		log.Error("Quote userData mismatch",
			"expected", sealHash.Hex(),
			"got", common.BytesToHash(userData[:32]).Hex())
		return errors.New("Quote userData does not match seal hash - possible tampering")
	}
	
	log.Debug("✓ Quote userData verified", "sealHash", sealHash.Hex())
	
	return nil
}

// Prepare 准备区块头
// 除SGX相关字段外，其他处理与以太坊完全一致
func (e *SGXEngine) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	// 标准以太坊处理：设置难度（PoA固定为1）
	header.Difficulty = big.NewInt(1)

	// 标准以太坊处理：设置时间戳
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Time = uint64(time.Now().Unix())
	if header.Time <= parent.Time {
		header.Time = parent.Time + 1
	}

	// SGX特有：预留Extra空间用于后续在Seal阶段填充
	// 此时还没有完整的区块信息，所以只预留空间
	// 实际的SGX Quote将在Seal阶段生成（因为需要完整的区块哈希作为userData）
	extra := &SGXExtra{
		SGXQuote:      []byte{}, // Seal阶段生成
		ProducerID:    []byte{}, // Seal阶段填充
		AttestationTS: 0,        // Seal阶段填充
		Signature:     []byte{}, // Seal阶段生成
	}

	extraData, err := extra.Encode()
	if err != nil {
		return err
	}
	header.Extra = extraData

	return nil
}

// Finalize 完成区块（计算状态根，不包含奖励）
func (e *SGXEngine) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state vm.StateDB, body *types.Body) {
	// No block rewards in mock implementation
	// TODO: Integrate with Module 03 (Incentive) for reward distribution
}

// FinalizeAndAssemble 完成并组装区块
func (e *SGXEngine) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, body *types.Body, receipts []*types.Receipt) (*types.Block, error) {
	// Finalize block
	e.Finalize(chain, header, state, body)
	
	// Calculate state root
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
	
	// Assemble and return block
	return types.NewBlock(header, body, receipts, trie.NewStackTrie(nil)), nil
}

// Seal 密封区块（生成SGX远程证明）
// 这是唯一涉及SGX的核心函数，其他所有处理与以太坊一致
func (e *SGXEngine) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	header := block.Header()

	// ===== SGX核心功能：远程证明 =====
	// 使用seal hash（不包含Extra的哈希）作为userData生成Quote
	// Quote本身就是签名！不需要额外的ECDSA签名
	
	// 1. 计算seal hash（不包含Extra/签名的区块哈希）
	//    这是标准以太坊PoA的做法，因为Extra在Seal时才填充
	sealHash := e.SealHash(header)
	
	// 2. 生成SGX Quote，将seal hash写入userData
	//    Quote包含：
	//    - MRENCLAVE（证明代码未被篡改）
	//    - userData（seal hash）
	//    - 硬件签名（Intel/AMD CPU签名，不可伪造）
	//    验证Quote即可确保：
	//    - 区块来自合法SGX enclave
	//    - 区块数据完整性（哈希匹配）
	//    - 无需额外的ECDSA签名或密钥管理
	quote, err := e.attestor.GenerateQuote(sealHash.Bytes())
	if err != nil {
		return err
	}

	// 3. 获取出块者ID（可以从MRENCLAVE派生，或使用固定值）
	producerID, err := e.attestor.GetProducerID()
	if err != nil {
		return err
	}

	// 4. 构造包含SGX证明的Extra数据
	extra := &SGXExtra{
		SGXQuote:      quote,                       // SGX Quote（硬件级签名，包含seal hash）
		ProducerID:    producerID,                  // 出块者身份标识
		AttestationTS: uint64(time.Now().Unix()),  // 证明时间戳
		Signature:     []byte{},                    // 空，Quote本身就是签名
	}

	extraData, err := extra.Encode()
	if err != nil {
		return err
	}
	header.Extra = extraData

	// 标准以太坊处理：返回密封后的区块
	select {
	case results <- block.WithSeal(header):
	case <-stop:
		return nil
	}

	return nil
}

// SealHash 计算区块头的 seal hash（不包含签名）
func (e *SGXEngine) SealHash(header *types.Header) common.Hash {
	// 临时移除签名字段
	extra, _ := DecodeSGXExtra(header.Extra)
	if extra != nil {
		extraCopy := *extra
		extraCopy.Signature = []byte{}
		extraData, _ := extraCopy.Encode()

		headerCopy := types.CopyHeader(header)
		headerCopy.Extra = extraData
		return headerCopy.Hash()
	}
	return header.Hash()
}

// CalcDifficulty 计算难度（PoA-SGX 固定为 1）
func (e *SGXEngine) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	return big.NewInt(1)
}

// APIs 返回 RPC API
func (e *SGXEngine) APIs(chain consensus.ChainHeaderReader) []rpc.API {
	return []rpc.API{
		{
			Namespace: "sgx",
			Service:   NewAPI(e, chain),
		},
	}
}

// Close 关闭引擎
func (e *SGXEngine) Close() error {
	return nil
}

// accumulateRewards 累积奖励
func (e *SGXEngine) accumulateRewards(chain consensus.ChainHeaderReader, state *state.StateDB, header *types.Header, body *types.Body) {
	// 获取区块生产者地址
	producer, err := e.Author(header)
	if err != nil {
		return
	}

	// Calculate total transaction fees
	// Using tx.Gas() (gas limit) for fee calculation in the Finalize stage.
	// Gas used accounting happens during transaction execution.
	totalFees := new(big.Int)
	for _, tx := range body.Transactions {
		gasPrice := tx.GasPrice()
		if gasPrice != nil {
			fee := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(tx.Gas()))
			totalFees.Add(totalFees, fee)
		}
	}

	// 基础出块奖励
	blockReward := new(big.Int).Set(e.config.RewardConfig.BaseBlockReward)

	// 添加交易费
	blockReward.Add(blockReward, totalFees)

	// 计算区块质量倍数
	block := types.NewBlock(header, body, nil, nil)
	quality := e.blockQualityScorer.CalculateQuality(block)

	// 应用质量倍数
	qualityBonus := new(big.Int).SetUint64(uint64(float64(blockReward.Uint64()) * (quality.RewardMultiplier - 1.0)))
	if qualityBonus.Sign() > 0 {
		blockReward.Add(blockReward, qualityBonus)
	}

	// 分配奖励到生产者
	reward, overflow := uint256.FromBig(blockReward)
	if !overflow {
		state.AddBalance(producer, reward, 0)
	}
}

// SetBlockProducer 设置区块生产者（用于测试）
func (e *SGXEngine) SetBlockProducer(bp *BlockProducer) {
	e.blockProducer = bp
}

// InitBlockProducer 初始化并启动区块生产者
// 必须在 txPool 和 blockchain 都可用后调用
func (e *SGXEngine) InitBlockProducer(txPool TxPool, chain BlockChain) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if e.blockProducer != nil {
		return nil // 已经初始化
	}
	
	e.blockProducer = NewBlockProducer(e.config, e, txPool, chain)
	return e.blockProducer.Start(context.Background())
}

// GetConfig 获取配置
func (e *SGXEngine) GetConfig() *Config {
	return e.config
}

// GetBlockQualityScorer 获取质量评分器
func (e *SGXEngine) GetBlockQualityScorer() *BlockQualityScorer {
	return e.blockQualityScorer
}

// GetMultiProducerReward 获取多生产者奖励计算器
func (e *SGXEngine) GetMultiProducerReward() *MultiProducerRewardCalculator {
	return e.multiProducerReward
}

// GetForkChoiceRule 获取分叉选择规则
func (e *SGXEngine) GetForkChoiceRule() *ForkChoiceRule {
	return e.forkChoiceRule
}

// GetReputationSystem 获取信誉系统
func (e *SGXEngine) GetReputationSystem() *ReputationSystem {
	return e.reputationSystem
}

// GetUptimeCalculator 获取在线率计算器
func (e *SGXEngine) GetUptimeCalculator() *UptimeCalculator {
	return e.uptimeCalculator
}
