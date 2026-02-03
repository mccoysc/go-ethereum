package sgx

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/trie"
)

// BlockProducer 区块生产者
type BlockProducer struct {
	config       *Config
	engine       *SGXEngine
	onDemandCtrl *OnDemandController
	txPool       TxPool      // 交易池接口
	chain        BlockChain  // 区块链接口

	mu            sync.Mutex
	producing     bool
	lastBlockTime time.Time
	stopCh        chan struct{}
}

// NewBlockProducer 创建区块生产者
func NewBlockProducer(config *Config, engine *SGXEngine, txPool TxPool, chain BlockChain) *BlockProducer {
	return &BlockProducer{
		config:        config,
		engine:        engine,
		onDemandCtrl:  NewOnDemandController(config),
		txPool:        txPool,
		chain:         chain,
		lastBlockTime: time.Now(),
		stopCh:        make(chan struct{}),
	}
}

// Start 启动区块生产
func (bp *BlockProducer) Start(ctx context.Context) error {
	bp.mu.Lock()
	if bp.producing {
		bp.mu.Unlock()
		return nil
	}
	bp.producing = true
	bp.mu.Unlock()

	go bp.produceLoop(ctx)
	return nil
}

// Stop 停止区块生产
func (bp *BlockProducer) Stop() {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if !bp.producing {
		return
	}

	close(bp.stopCh)
	bp.producing = false
}

// produceLoop 区块生产循环
func (bp *BlockProducer) produceLoop(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-bp.stopCh:
			return
		case <-ticker.C:
			bp.tryProduceBlock()
		}
	}
}

// tryProduceBlock 尝试生产区块
func (bp *BlockProducer) tryProduceBlock() {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	// 从交易池获取待处理交易统计
	pendingTxCount := 0
	pendingGasTotal := uint64(0)
	
	if bp.txPool != nil {
		pendingTxCount = bp.txPool.PendingCount()
		
		// 计算待处理交易的总 Gas
		pending := bp.txPool.Pending(false)
		for _, txs := range pending {
			for _, tx := range txs {
				pendingGasTotal += tx.Gas()
			}
		}
	}

	shouldProduce := bp.onDemandCtrl.ShouldProduceBlock(bp.lastBlockTime, pendingTxCount, pendingGasTotal)
	
	if !shouldProduce {
		return
	}

	// 生产区块
	log.Info("BlockProducer: Attempting to produce block", 
		"pendingTxs", pendingTxCount,
		"pendingGas", pendingGasTotal,
		"elapsed", time.Since(bp.lastBlockTime))
	
	if err := bp.produceBlock(); err != nil {
		log.Error("BlockProducer: Failed to produce block", "err", err)
		return
	}

	bp.lastBlockTime = time.Now()
}

// produceBlock 生产区块
func (bp *BlockProducer) produceBlock() error {
	// 1. 从交易池收集交易
	if bp.txPool == nil || bp.chain == nil {
		return ErrInvalidConfig
	}
	
	pending := bp.txPool.Pending(false)
	var transactions []*types.Transaction
	gasLimit := uint64(0)
	
	// 收集交易直到达到 Gas 限制或交易数量限制
	for _, txs := range pending {
		for _, tx := range txs {
			if gasLimit+tx.Gas() > bp.config.MaxGasPerBlock {
				break
			}
			if len(transactions) >= int(bp.config.MaxTxPerBlock) {
				break
			}
			transactions = append(transactions, tx)
			gasLimit += tx.Gas()
		}
		if len(transactions) >= int(bp.config.MaxTxPerBlock) {
			break
		}
	}
	
	log.Info("BlockProducer: Collected transactions", "count", len(transactions), "gasLimit", gasLimit)
	
	// 2. 获取父区块并创建区块头
	parent := bp.chain.CurrentBlock()
	if parent == nil {
		return ErrUnknownAncestor
	}
	
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     new(big.Int).Add(parent.Number, big.NewInt(1)),
		GasLimit:   bp.config.MaxGasPerBlock,
		GasUsed:    0, // Will be set after execution
		Time:       uint64(time.Now().Unix()),
		Difficulty: big.NewInt(1), // PoA-SGX 固定难度为 1
		Coinbase:   common.Address{}, // Will be set by Prepare
	}
	
	// Ensure timestamp is greater than parent (prevent collision)
	if header.Time <= parent.Time {
		header.Time = parent.Time + 1
		log.Warn("BlockProducer: Adjusted timestamp to avoid collision",
			"parent", parent.Time, "adjusted", header.Time)
	}
	
	// 3. 调用 engine.Prepare() 准备区块头
	if err := bp.engine.Prepare(bp.chain, header); err != nil {
		log.Error("BlockProducer: Failed to prepare header", "err", err)
		return err
	}
	
	// 4. 获取父区块的状态数据库
	parentBlock := bp.chain.GetBlock(parent.Hash(), parent.Number.Uint64())
	if parentBlock == nil {
		return fmt.Errorf("parent block not found")
	}
	
	// 5. 创建 StateDB - 使用 BlockChain 的接口
	// 我们需要访问底层的 core.BlockChain 来获取 StateAt
	coreChain, ok := bp.chain.(*core.BlockChain)
	if !ok {
		return fmt.Errorf("chain is not *core.BlockChain")
	}
	
	statedb, err := coreChain.StateAt(parentBlock.Root())
	if err != nil {
		log.Error("BlockProducer: Failed to get state", "err", err, "root", parentBlock.Root())
		return err
	}
	
	// 6. 执行交易
	processor := core.NewStateProcessor(coreChain)
	vmConfig := vm.Config{}
	
	body := &types.Body{
		Transactions: transactions,
		Uncles:       nil,
		Withdrawals:  nil,
	}
	
	// 创建临时区块用于处理
	tempBlock := types.NewBlock(header, body, nil, trie.NewStackTrie(nil))
	
	// 处理区块中的所有交易
	result, err := processor.Process(tempBlock, statedb, vmConfig)
	if err != nil {
		log.Error("BlockProducer: Failed to process transactions", "err", err)
		return err
	}
	
	// 更新区块头的 GasUsed, Bloom和ReceiptHash
	header.GasUsed = result.GasUsed
	header.Bloom = types.MergeBloom(result.Receipts)
	header.ReceiptHash = types.DeriveSha(result.Receipts, trie.NewStackTrie(nil))
	
	log.Info("BlockProducer: Transactions processed", "gasUsed", result.GasUsed, "receipts", len(result.Receipts))
	
	// 7. Finalize 区块（计算状态根，分配奖励）
	bp.engine.Finalize(bp.chain, header, statedb, body)
	
	// 提交状态更改 (需要3个参数)
	root, err := statedb.Commit(header.Number.Uint64(), true, true)
	if err != nil {
		log.Error("BlockProducer: Failed to commit state", "err", err)
		return err
	}
	header.Root = root
	
	// 8. 创建最终区块
	finalBlock := types.NewBlock(header, body, result.Receipts, trie.NewStackTrie(nil))
	
	// 9. Seal 区块（添加 SGX Quote）- 使用同步调用避免死锁
	log.Debug("BlockProducer: Sealing block", "number", finalBlock.NumberU64())
	
	// 创建sealBlockSync辅助函数来包装Seal调用
	sealedBlock, err := bp.sealBlockSync(finalBlock)
	if err != nil {
		log.Error("BlockProducer: Failed to seal block", "err", err)
		return err
	}
	
	log.Info("BlockProducer: Block sealed successfully", "number", sealedBlock.NumberU64())
	
	// 10. 插入区块到链中
	_, err = coreChain.InsertChain(types.Blocks{sealedBlock})
	if err != nil {
		log.Error("BlockProducer: Failed to insert block", "err", err, "number", sealedBlock.NumberU64())
		return err
	}
	
	log.Info("BlockProducer: Block produced successfully", 
		"number", sealedBlock.NumberU64(), 
		"hash", sealedBlock.Hash().Hex(),
		"txs", len(transactions),
		"gasUsed", header.GasUsed)
	
	return nil
}

// sealBlockSync 同步调用Seal方法，避免channel死锁
func (bp *BlockProducer) sealBlockSync(block *types.Block) (*types.Block, error) {
	resultCh := make(chan *types.Block, 1)
	stopCh := make(chan struct{})
	
	// 启动Seal
	err := bp.engine.Seal(bp.chain, block, resultCh, stopCh)
	if err != nil {
		close(stopCh)
		return nil, fmt.Errorf("failed to start sealing: %w", err)
	}
	
	// 等待结果，带超时
	select {
	case sealedBlock := <-resultCh:
		close(stopCh)
		return sealedBlock, nil
	case <-time.After(10 * time.Second):
		close(stopCh)
		return nil, fmt.Errorf("timeout waiting for block seal (10s)")
	}
}

// SetLastBlockTime 设置最后出块时间（用于测试）
func (bp *BlockProducer) SetLastBlockTime(t time.Time) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.lastBlockTime = t
}

// GetLastBlockTime 获取最后出块时间
func (bp *BlockProducer) GetLastBlockTime() time.Time {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.lastBlockTime
}

// IsProducing 检查是否正在生产
func (bp *BlockProducer) IsProducing() bool {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.producing
}

// ProduceBlockNow 立即生产区块（用于测试）
func (bp *BlockProducer) ProduceBlockNow(
	parent *types.Header,
	transactions []*types.Transaction,
	coinbase common.Address,
) (*types.Block, error) {
	if bp.chain == nil {
		return nil, ErrInvalidConfig
	}
	
	// 计算总 Gas
	gasUsed := uint64(0)
	for _, tx := range transactions {
		gasUsed += tx.Gas()
	}
	
	// 创建新区块头
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     new(big.Int).Add(parent.Number, big.NewInt(1)),
		GasLimit:   bp.config.MaxGasPerBlock,
		GasUsed:    gasUsed,
		Time:       uint64(time.Now().Unix()),
		Coinbase:   coinbase,
		Difficulty: big.NewInt(1),
	}

	// 准备区块头
	if err := bp.engine.Prepare(bp.chain, header); err != nil {
		return nil, err
	}

	// Full block production (transaction execution, Finalize, Seal)
	// is completed by the caller in the appropriate context.
	// This function returns the prepared block header.
	
	// Use empty trie for simple block creation
	return types.NewBlock(header, &types.Body{Transactions: transactions}, nil, trie.NewStackTrie(nil)), nil
}

// EstimateNextBlockTime 估算下次出块时间
func (bp *BlockProducer) EstimateNextBlockTime() time.Time {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	timeUntilNext := bp.onDemandCtrl.TimeUntilNextBlock(bp.lastBlockTime)
	return time.Now().Add(timeUntilNext)
}

// CanProduceNow 检查当前是否可以出块
func (bp *BlockProducer) CanProduceNow() bool {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.onDemandCtrl.CanProduceNow(bp.lastBlockTime)
}
