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

	mu            sync.Mutex
	producing     bool
	lastBlockTime time.Time
	stopCh        chan struct{}
}

// NewBlockProducer 创建区块生产者
func NewBlockProducer(config *Config, engine *SGXEngine) *BlockProducer {
	return &BlockProducer{
		config:        config,
		engine:        engine,
		onDemandCtrl:  NewOnDemandController(config),
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

	// 检查是否应该出块
	// 注意：实际实现中需要从交易池获取待处理交易
	pendingTxCount := 0          // TODO: 从交易池获取
	pendingGasTotal := uint64(0) // TODO: 从交易池获取

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
	// TODO: 实际实现需要：
	// 1. 从交易池收集交易
	// 2. 创建区块头
	// 3. 调用 engine.Prepare() 准备区块
	// 4. 执行交易
	// 5. 调用 engine.FinalizeAndAssemble() 完成区块
	// 6. 调用 engine.Seal() 密封区块
	// 7. 广播区块

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
	// 创建新区块头
	_ = &types.Header{
		ParentHash: parent.Hash(),
		Number:     new(big.Int).Add(parent.Number, big.NewInt(1)),
		GasLimit:   bp.config.MaxGasPerBlock,
		Time:       uint64(time.Now().Unix()),
		Coinbase:   coinbase,
		Difficulty: big.NewInt(1),
	}

	// TODO: 实际实现需要完整的区块生产流程

	return nil, nil
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
