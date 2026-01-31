package sgx

import (
	"context"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
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

	if !bp.onDemandCtrl.ShouldProduceBlock(bp.lastBlockTime, pendingTxCount, pendingGasTotal) {
		return
	}

	// 生产区块
	if err := bp.produceBlock(); err != nil {
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
	gasUsed := uint64(0)
	
	// 收集交易直到达到 Gas 限制或交易数量限制
	for _, txs := range pending {
		for _, tx := range txs {
			if gasUsed+tx.Gas() > bp.config.MaxGasPerBlock {
				break
			}
			if len(transactions) >= int(bp.config.MaxTxPerBlock) {
				break
			}
			transactions = append(transactions, tx)
			gasUsed += tx.Gas()
		}
		if len(transactions) >= int(bp.config.MaxTxPerBlock) {
			break
		}
	}
	
	// 2. 获取父区块并创建区块头
	parent := bp.chain.CurrentBlock()
	if parent == nil {
		return ErrUnknownAncestor
	}
	
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     new(big.Int).Add(parent.Number, big.NewInt(1)),
		GasLimit:   bp.config.MaxGasPerBlock,
		GasUsed:    gasUsed,
		Time:       uint64(time.Now().Unix()),
		Difficulty: big.NewInt(1), // PoA-SGX 固定难度为 1
	}
	
	// 3. 调用 engine.Prepare() 准备区块头
	if err := bp.engine.Prepare(bp.chain, header); err != nil {
		return err
	}
	
	// 注意：实际的交易执行、状态更新、Finalize 和 Seal 操作
	// 应该由外部的区块生产流程（如 miner）来完成
	// 这里的 BlockProducer 主要负责按需出块的逻辑控制
	
	return nil
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

	// 注意：完整的区块生产（执行交易、Finalize、Seal）
	// 应该由调用者在适当的上下文中完成
	// 这里只返回准备好的区块头
	
	return types.NewBlock(header, &types.Body{Transactions: transactions}, nil, nil), nil
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
