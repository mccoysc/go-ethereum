package sgx

import (
	"bytes"
	"errors"
	"math/big"
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
	
	// Step 2: Read contract addresses from manifest environment variables
	// 关键：必须先验证manifest签名，然后才能读取配置
	log.Info("Step 2: Reading contract addresses from manifest file...")
	
	// 默认使用genesis配置
	governanceAddr := paramsConfig.GovernanceContract
	securityAddr := paramsConfig.SecurityConfig
	incentiveAddr := paramsConfig.IncentiveContract
	
	// 必须从manifest文件读取合约地址（已验证签名）
	manifestGov, manifestSec, err := internalsgx.ReadContractAddressesFromManifest()
	if err != nil {
		// 无法读取manifest → CRITICAL ERROR
		// 不允许降级到genesis配置，因为manifest是安全关键配置
		log.Crit("SECURITY: Failed to read contract addresses from manifest file. " +
			"Manifest reading is REQUIRED for security. " +
			"Cannot fall back to genesis config.",
			"error", err,
			"hint", "Ensure manifest file is present and properly signed")
		return nil, err  // This line won't be reached due to log.Crit
	} else {
		// Manifest读取成功，比对genesis配置
		manifestGovAddr := common.HexToAddress(manifestGov)
		manifestSecAddr := common.HexToAddress(manifestSec)
		
		if manifestGovAddr != governanceAddr {
			log.Error("SECURITY WARNING: Manifest governance address differs from genesis!",
				"manifest", manifestGov,
				"genesis", governanceAddr.Hex())
			// 使用genesis地址（更可信）
		} else {
			log.Info("✓ Manifest addresses match genesis config")
		}
		
		if manifestSecAddr != securityAddr {
			log.Error("SECURITY WARNING: Manifest security config address differs from genesis!",
				"manifest", manifestSec,
				"genesis", securityAddr.Hex())
		}
	}
	
	// Convert params.SGXConfig to internal Config
	config := &Config{
		Period: paramsConfig.Period,
		Epoch:  paramsConfig.Epoch,
		// Use default configs for other fields
		QualityConfig:    DefaultQualityConfig(),
		UptimeConfig:     DefaultUptimeConfig(),
		RewardConfig:     DefaultRewardConfig(),
		PenaltyConfig:    DefaultPenaltyConfig(),
		ReputationConfig: DefaultReputationConfig(),
		// Store contract addresses from genesis (as common.Address)
		GovernanceContract: governanceAddr,
		SecurityConfig:     securityAddr,
		IncentiveContract:  incentiveAddr,
	}
	
	log.Info("SGX Configuration",
		"period", config.Period,
		"epoch", config.Epoch,
		"governance", config.GovernanceContract,
		"security", config.SecurityConfig,
		"incentive", config.IncentiveContract)
	
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
	
	var attestor Attestor
	var verifier Verifier
	var err error
	
	if isGramine {
		// In Gramine: use real SGX attestation
		log.Info("Using Gramine SGX attestation (production mode)")
		attestor, err = NewGramineAttestor()
		if err != nil {
			// Gramine环境必须有正确的环境变量
			log.Crit("Failed to create Gramine attestor", "error", err)
		}
		
		verifier, err = NewGramineVerifier()
		if err != nil {
			log.Crit("Failed to create Gramine verifier", "error", err)
		}
	} else {
		// Not in Gramine environment (GRAMINE_VERSION not set)
		// 即使环境变量可以模拟，检测到非Gramine环境也必须退出
		log.Crit("SECURITY: GRAMINE_VERSION environment variable not set. " +
			"Application MUST run under Gramine SGX. " +
			"Cannot proceed without Gramine runtime.",
			"hint", "For testing: export GRAMINE_VERSION=test (but this requires proper test infrastructure)")
		return nil, fmt.Errorf("GRAMINE_VERSION not set - must run under Gramine SGX")
		
		attestor, err = NewTestAttestor(testDataDir)
		if err != nil {
			// 测试数据不存在 → 可以退出（用户可以提供测试数据文件）
			log.Crit("Failed to create test attestor - test data not found", 
				"error", err,
				"hint", fmt.Sprintf("Provide test data in %s (mrenclave.txt, mrsigner.txt)", testDataDir))
		}
		
		verifier, err = NewTestVerifier(testDataDir)
		if err != nil {
			log.Crit("Failed to create test verifier", "error", err)
		}
	}
	
	log.Info("=== SGX Consensus Engine Initialized ===")
	log.Info("Next: Security parameters will be read from contract", "contract", config.SecurityConfig)
	
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

	// 验证 SGX Quote
	if err := e.verifier.VerifyQuote(extra.SGXQuote); err != nil {
		return ErrQuoteVerificationFailed
	}

	// 验证 ProducerID 匹配
	producerID, err := e.verifier.ExtractProducerID(extra.SGXQuote)
	if err != nil {
		return ErrInvalidProducerID
	}
	if !bytes.Equal(producerID, extra.ProducerID) {
		return ErrInvalidProducerID
	}

	// 验证签名
	sealHash := e.SealHash(header)
	if err := e.verifier.VerifySignature(sealHash.Bytes(), extra.Signature, extra.ProducerID); err != nil {
		return ErrInvalidSignature
	}

	return nil
}

// VerifyUncles 验证叔块（PoA-SGX 不支持叔块）
func (e *SGXEngine) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	if len(block.Uncles()) > 0 {
		return errors.New("uncles not allowed in PoA-SGX")
	}
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

// Seal 密封区块（生成SGX远程证明并签名）
// 这是唯一涉及SGX的核心函数，其他所有处理与以太坊一致
func (e *SGXEngine) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	header := block.Header()

	// 标准以太坊处理：计算seal hash（不包含签名的区块哈希）
	sealHash := e.SealHash(header)

	// ===== SGX核心功能：远程证明 =====
	// 1. 使用区块哈希作为userData生成SGX Quote
	//    这证明了该区块确实是在SGX Enclave内产生的
	//    Quote包含：MRENCLAVE、userData(blockHash)、timestamp等
	quote, err := e.attestor.GenerateQuote(sealHash.Bytes())
	if err != nil {
		return err
	}

	// 2. 获取出块者ID（从SGX证书中提取）
	producerID, err := e.attestor.GetProducerID()
	if err != nil {
		return err
	}

	// 3. 在Enclave内对区块进行签名
	//    私钥永远不会离开SGX Enclave，保证安全性
	signature, err := e.attestor.SignInEnclave(sealHash.Bytes())
	if err != nil {
		return err
	}

	// 4. 构造包含SGX证明的Extra数据
	extra := &SGXExtra{
		SGXQuote:      quote,                       // SGX远程证明报告
		ProducerID:    producerID,                  // 出块者身份
		AttestationTS: uint64(time.Now().Unix()),  // 证明时间戳
		Signature:     signature,                   // Enclave内的签名
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
