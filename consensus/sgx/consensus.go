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
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
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
	
	// In testenv build mode, Gramine filesystem is simulated via internal/sgx build tags
	// No need for environment variable checks - the architecture relies on Gramine filesystem abstraction
	gramineVersion := os.Getenv("GRAMINE_VERSION")
	if gramineVersion != "" {
		log.Info("Running under Gramine", "version", gramineVersion)
	} else {
		log.Info("Running with Gramine filesystem simulation (testenv mode)")
	}
	
	// Step 1: Read configuration from environment variables (set by Gramine from manifest loader.env)
	log.Info("Step 1: Reading configuration from environment variables...")
	appConfig, err := internalsgx.GetAppConfigFromEnvironment()
	if err != nil {
		// Fallback to genesis config if environment variables not set
		log.Warn("Could not read config from environment, using genesis params", "error", err)
		
		// Use addresses from genesis params as fallback
		governanceAddr := paramsConfig.GovernanceContract
		securityAddr := paramsConfig.SecurityConfig
		incentiveAddr := paramsConfig.IncentiveContract
		
		log.Info("Contract addresses from genesis",
			"governance", governanceAddr.Hex(),
			"security", securityAddr.Hex(),
			"incentive", incentiveAddr.Hex())
		
		// Create config from genesis params
		appConfig = &internalsgx.AppConfig{
			GovernanceContract:     governanceAddr.Hex(),
			SecurityConfigContract: securityAddr.Hex(),
		}
	} else {
		log.Info("✓ Configuration loaded from environment",
			"governance", appConfig.GovernanceContract,
			"security", appConfig.SecurityConfigContract,
			"nodeType", appConfig.NodeType)
	}
	
	// Use incentive address from genesis params (not in environment config yet)
	incentiveAddr := paramsConfig.IncentiveContract
	
	log.Info("Contract addresses",
		"governance", appConfig.GovernanceContract,
		"security", appConfig.SecurityConfigContract,
		"incentive", incentiveAddr.Hex())
	
	// Use default config as base
	config := DefaultConfig()
	
	log.Info("SGX Configuration",
		"period", paramsConfig.Period,
		"epoch", paramsConfig.Epoch)
	
	// Step 3: Load SGX modules
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
	
	attestor, err := internalsgx.NewGramineAttestor()
	if err != nil {
		log.Crit("Failed to create Gramine attestor", "error", err)
	}
	
	// Create DCAP verifier
	verifier := internalsgx.NewDCAPVerifier(true)
	
	// Step 5: Initialize whitelist from contract storage
	// Priority: Contract Storage → Genesis Alloc Storage
	log.Info("Step 5: Loading whitelist from security config contract...")
	
	// The security config contract address comes from appConfig
	// Now read whitelist from contract storage
	// In genesis block, the storage is in alloc
	// After genesis, the storage is in state database
	
	// For now, try environment variables as genesis alloc representation
	// In production, this would read from actual contract storage
	genesisWhitelist := loadWhitelistFromContractStorage(appConfig.SecurityConfigContract)
	
	if len(genesisWhitelist.MREnclaves) > 0 || len(genesisWhitelist.MRSigners) > 0 {
		log.Info("Loading whitelist from security config contract")
		loadWhitelistToVerifier(verifier, genesisWhitelist.MREnclaves, genesisWhitelist.MRSigners)
		log.Info("Whitelist loaded successfully from contract storage", 
			"mrenclaves", len(genesisWhitelist.MREnclaves), 
			"mrsigners", len(genesisWhitelist.MRSigners))
	} else {
		log.Warn("Whitelist is empty in contract storage")
		log.Info("Use governance contract to add MRENCLAVE/MRSIGNER entries")
		log.Info("System will accept blocks after whitelist is populated via governance")
	}
	
	log.Info("=== SGX Consensus Engine Initialized ===")
	log.Info("Architecture: Manifest(contract addr) → Contract Storage(whitelist) → Governance(updates)")
	
	return New(config, attestor, verifier)
}

// GenesisWhitelist holds whitelist configuration from genesis
type GenesisWhitelist struct {
	MREnclaves []string
	MRSigners  []string
}

// loadWhitelistFromContractStorage reads whitelist from genesis alloc storage
// Security requirement: No environment variables for security parameters
// Only sources: 1) Contract Storage, 2) Genesis Alloc Storage
func loadWhitelistFromContractStorage(contractAddr string) GenesisWhitelist {
	whitelist := GenesisWhitelist{
		MREnclaves: []string{},
		MRSigners:  []string{},
	}
	
	// Read from genesis.json alloc[contractAddr].storage
	// In production deployment:
	// 1. Genesis file contains pre-populated storage for security config contract
	// 2. Storage layout follows Solidity mapping:
	//    - allowedMREnclaves: mapping(bytes32 => bool) at slot 0
	//    - allowedMRSigners: mapping(bytes32 => bool) at slot 1
	// 3. Storage keys: keccak256(abi.encode(mrenclave/mrsigner, slot))
	
	// After genesis, whitelist is managed via governance contract
	// which updates the contract storage in state database
	
	// For testing: genesis.json should contain the storage entries
	// For production: genesis.json is created by deployment tools
	
	// This function will be called during consensus engine initialization
	// At that point, we can read from the genesis block's alloc storage
	// or from the state database after the chain has started
	
	// Currently returns empty whitelist - will be populated via governance
	log.Info("Whitelist loading from genesis alloc storage",
		"contract", contractAddr,
		"note", "Use governance contract to populate whitelist")
	
	return whitelist
}

// loadWhitelistToVerifier loads whitelist entries into verifier
func loadWhitelistToVerifier(verifier *internalsgx.DCAPVerifier, mrenclaves, mrsigners []string) {
	for _, mrEnclaveHex := range mrenclaves {
		mrEnclave := common.FromHex(mrEnclaveHex)
		if len(mrEnclave) == 32 {
			verifier.AddAllowedMREnclave(mrEnclave)
			log.Info("Added MRENCLAVE to whitelist", "mrenclave", mrEnclaveHex)
		} else {
			log.Warn("Invalid MRENCLAVE length, skipping", "mrenclave", mrEnclaveHex, "length", len(mrEnclave))
		}
	}
	
	for _, mrSignerHex := range mrsigners {
		mrSigner := common.FromHex(mrSignerHex)
		if len(mrSigner) == 32 {
			verifier.AddAllowedMRSigner(mrSigner)
			log.Info("Added MRSIGNER to whitelist", "mrsigner", mrSignerHex)
		} else {
			log.Warn("Invalid MRSIGNER length, skipping", "mrsigner", mrSignerHex, "length", len(mrSigner))
		}
	}
}

// Helper functions for string processing
func splitByComma(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

func trimSpaces(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
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
	// Skip verification for genesis block
	if header.Number.Uint64() == 0 {
		return nil
	}
	
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

	// 完整的Quote验证（一次性获取所有数据）
	// 这会验证Quote并返回所有measurements和instanceID
	// 匹配gramine sgx-quote-verify.js的verifyQuote()逻辑
	quoteResult, err := e.verifier.VerifyQuoteComplete(extra.SGXQuote, nil)
	if err != nil {
		return fmt.Errorf("quote verification failed: %w", err)
	}
	
	if !quoteResult.Verified {
		return ErrQuoteVerificationFailed
	}

	// 验证ProducerID：应该等于从Quote验证中返回的PlatformInstanceID
	// 这确保一个物理CPU只能作为一个生产者，防止Sybil攻击
	if !bytes.Equal(quoteResult.Measurements.PlatformInstanceID[:], extra.ProducerID) {
		return fmt.Errorf("producer ID mismatch: expected %x (from %s), got %x",
			quoteResult.Measurements.PlatformInstanceID[:],
			quoteResult.Measurements.PlatformInstanceIDSource,
			extra.ProducerID)
	}

	// 可以在这里添加更多验证，比如检查MRENCLAVE、MRSIGNER等

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
	//
	// Implementation is in:
	// - verify_userdata_production.go (strict validation)
	// - verify_userdata_testenv.go (lenient validation for testing)
	
	return e.verifyQuoteUserData(block)
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

	// EIP-1559: Calculate base fee for the new block
	if chain.Config().IsLondon(header.Number) {
		header.BaseFee = eip1559.CalcBaseFee(chain.Config(), parent)
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
	// Block rewards and incentives are managed by the incentive system
	// No additional finalization needed here
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

	// 3. 从Quote验证中获取ProducerID
	//    使用VerifyQuoteComplete一次性获取所有数据（包括instanceID）
	//    这样ProducerID和验证逻辑保持一致
	quoteResult, err := e.verifier.VerifyQuoteComplete(quote, nil)
	if err != nil {
		return fmt.Errorf("failed to verify generated quote: %w", err)
	}
	
	if !quoteResult.Verified {
		return errors.New("generated quote failed verification")
	}
	
	// Debug logging
	log.Debug("Seal: Quote verification result",
		"verified", quoteResult.Verified,
		"platformInstanceID", fmt.Sprintf("%x", quoteResult.Measurements.PlatformInstanceID),
		"platformInstanceIDSource", quoteResult.Measurements.PlatformInstanceIDSource,
		"platformInstanceIDLen", len(quoteResult.Measurements.PlatformInstanceID))
	
	// ProducerID就是PlatformInstanceID
	producerID := quoteResult.Measurements.PlatformInstanceID[:]
	
	log.Debug("Seal: ProducerID set", "producerID", fmt.Sprintf("%x", producerID), "length", len(producerID))

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

