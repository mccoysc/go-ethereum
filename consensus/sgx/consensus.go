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
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/holiman/uint256"
)

// SGXEngine PoA-SGX 共识引擎
type SGXEngine struct {
	config *Config

	// 外部依赖
	attestor  Attestor
	verifier  Verifier

	// 内部组件
	blockProducer        *BlockProducer
	onDemandController   *OnDemandController
	blockQualityScorer   *BlockQualityScorer
	multiProducerReward  *MultiProducerRewardCalculator
	forkChoiceRule       *ForkChoiceRule
	reorgHandler         *ReorgHandler
	uptimeCalculator     *UptimeCalculator
	reputationSystem     *ReputationSystem
	penaltyManager       *PenaltyManagerImpl
	onlineRewardCalc     *OnlineRewardCalculator
	nodeSelector         *NodeSelector
	comprehensiveReward  *ComprehensiveRewardCalculator

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
func (e *SGXEngine) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	// 设置难度
	header.Difficulty = big.NewInt(1)

	// 设置时间戳
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Time = uint64(time.Now().Unix())
	if header.Time <= parent.Time {
		header.Time = parent.Time + 1
	}

	// 生成 SGX Quote
	quote, err := e.attestor.GenerateQuote(header.ParentHash.Bytes())
	if err != nil {
		return err
	}

	// 获取 ProducerID
	producerID, err := e.attestor.GetProducerID()
	if err != nil {
		return err
	}

	// 构造 Extra 数据
	extra := &SGXExtra{
		SGXQuote:      quote,
		ProducerID:    producerID,
		AttestationTS: uint64(time.Now().Unix()),
		Signature:     []byte{}, // 签名在 Seal 阶段生成
	}

	extraData, err := extra.Encode()
	if err != nil {
		return err
	}
	header.Extra = extraData

	return nil
}

// Finalize 完成区块（计算状态根，不包含奖励）
func (e *SGXEngine) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, body *types.Body) {
	// 计算并分配区块奖励
	e.accumulateRewards(chain, state, header, body)
}

// FinalizeAndAssemble 完成并组装区块
func (e *SGXEngine) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, body *types.Body, receipts []*types.Receipt) (*types.Block, error) {
	// 执行 Finalize
	e.Finalize(chain, header, state, body)

	// 组装区块
	return types.NewBlock(header, body, receipts, trie.NewStackTrie(nil)), nil
}

// Seal 密封区块（生成签名）
func (e *SGXEngine) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	header := block.Header()

	// 计算 seal hash
	sealHash := e.SealHash(header)

	// 在 Enclave 内签名
	signature, err := e.attestor.SignInEnclave(sealHash.Bytes())
	if err != nil {
		return err
	}

	// 解析现有的 Extra
	extra, err := DecodeSGXExtra(header.Extra)
	if err != nil {
		return err
	}

	// 添加签名
	extra.Signature = signature
	extraData, err := extra.Encode()
	if err != nil {
		return err
	}
	header.Extra = extraData

	// 返回密封后的区块
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

	// 计算交易费总额
	totalFees := new(big.Int)
	for _, tx := range body.Transactions {
		// 注意: 实际实现中需要从 receipts 获取真实的 gas used
		// 这里简化处理
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
