# 共识引擎模块开发文档

## 模块概述

共识引擎模块实现 X Chain 的 PoA-SGX 共识机制，这是一种基于 Intel SGX 远程证明的权威证明共识。与传统 PoS 不同，X Chain 通过 SGX 硬件验证确保节点运行可信代码，而非依赖经济质押。

## 负责团队

**共识/核心协议团队**

## 模块职责

1. 实现 `consensus.Engine` 接口
2. 区块生产与验证
3. 区块头扩展字段处理
4. 按需出块机制
5. 冲突处理与分叉选择

## 依赖关系

```
+------------------+
|  共识引擎模块    |
+------------------+
        |
        +---> SGX 证明模块（节点身份验证）
        |
        +---> P2P 网络模块（区块广播）
        |
        +---> 激励机制模块（奖励计算）
```

### 上游依赖
- SGX 证明模块（验证区块生产者身份）
- 核心 go-ethereum 框架

### 下游依赖（被以下模块使用）
- 交易池（交易确认）
- RPC 层（区块查询）

## 核心接口定义

### Engine 接口实现

```go
// consensus/sgx/consensus.go
package sgx

import (
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/consensus"
    "github.com/ethereum/go-ethereum/core/state"
    "github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/rpc"
)

// SGXEngine 实现 consensus.Engine 接口
type SGXEngine struct {
    config        *Config
    attestor      Attestor
    verifier      Verifier
    blockProducer *BlockProducer
}

// Author 返回区块的生产者地址
func (e *SGXEngine) Author(header *types.Header) (common.Address, error) {
    // 从区块头扩展字段中提取生产者地址
    return extractProducer(header)
}

// VerifyHeader 验证区块头
func (e *SGXEngine) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header) error {
    return e.verifyHeader(chain, header, nil)
}

// VerifyHeaders 批量验证区块头
func (e *SGXEngine) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header) (chan<- struct{}, <-chan error) {
    abort := make(chan struct{})
    results := make(chan error, len(headers))
    
    go func() {
        for _, header := range headers {
            err := e.verifyHeader(chain, header, nil)
            select {
            case <-abort:
                return
            case results <- err:
            }
        }
    }()
    
    return abort, results
}

// Prepare 准备区块头（设置共识相关字段）
func (e *SGXEngine) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
    parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
    if parent == nil {
        return consensus.ErrUnknownAncestor
    }
    
    // 设置区块头扩展字段
    header.Extra = e.prepareExtra(parent, header)
    
    return nil
}

// Finalize 完成区块（计算状态根，不包含奖励）
func (e *SGXEngine) Finalize(chain consensus.ChainHeaderReader, header *types.Header, 
    state *state.StateDB, body *types.Body) {
    // 计算并分配区块奖励
    e.accumulateRewards(chain, state, header)
}

// FinalizeAndAssemble 完成并组装区块
func (e *SGXEngine) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header,
    state *state.StateDB, body *types.Body, receipts []*types.Receipt) (*types.Block, error) {
    
    e.Finalize(chain, header, state, body)
    
    // 组装区块
    return types.NewBlock(header, body, receipts, trie.NewStackTrie(nil)), nil
}

// Seal 密封区块（生成区块签名）
func (e *SGXEngine) Seal(chain consensus.ChainHeaderReader, block *types.Block, 
    results chan<- *types.Block, stop <-chan struct{}) error {
    
    header := block.Header()
    
    // 生成 SGX 签名
    signature, err := e.signBlock(header)
    if err != nil {
        return err
    }
    
    // 将签名添加到区块头
    header.Extra = append(header.Extra, signature...)
    
    select {
    case results <- block.WithSeal(header):
    case <-stop:
        return nil
    }
    
    return nil
}

// CalcDifficulty 计算难度（PoA-SGX 中固定为 1）
func (e *SGXEngine) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, 
    parent *types.Header) *big.Int {
    return big.NewInt(1)
}

// APIs 返回共识引擎提供的 RPC API
func (e *SGXEngine) APIs(chain consensus.ChainHeaderReader) []rpc.API {
    return []rpc.API{
        {
            Namespace: "sgx",
            Service:   &API{engine: e, chain: chain},
        },
    }
}

// Close 关闭共识引擎
func (e *SGXEngine) Close() error {
    return nil
}
```

## 关键数据结构

### 区块头扩展字段

```go
// consensus/sgx/types.go
package sgx

import (
    "github.com/ethereum/go-ethereum/common"
)

// ExtraData 区块头扩展数据结构
// 存储在 header.Extra 字段中
type ExtraData struct {
    // 生产者 SGX Quote（远程证明）
    ProducerQuote []byte
    
    // 生产者签名
    ProducerSignature []byte
    
    // 生产者地址
    ProducerAddress common.Address
    
    // 区块生产时间戳（纳秒精度）
    ProducerTimestamp uint64
    
    // 父区块的 SGX 证明哈希
    ParentQuoteHash common.Hash
}

// EncodeExtraData 编码扩展数据
func EncodeExtraData(extra *ExtraData) ([]byte, error) {
    return rlp.EncodeToBytes(extra)
}

// DecodeExtraData 解码扩展数据
func DecodeExtraData(data []byte) (*ExtraData, error) {
    var extra ExtraData
    if err := rlp.DecodeBytes(data, &extra); err != nil {
        return nil, err
    }
    return &extra, nil
}
```

### 区块生产者

```go
// consensus/sgx/block_producer.go
package sgx

import (
    "context"
    "sync"
    "time"
    
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/core/types"
)

// BlockProducerConfig 区块生产者配置
type BlockProducerConfig struct {
    // 最小出块间隔
    MinBlockInterval time.Duration
    
    // 最大等待交易时间
    MaxWaitTime time.Duration
    
    // 单区块最大交易数
    MaxTxPerBlock int
    
    // 单区块最大 Gas
    MaxGasPerBlock uint64
}

// DefaultBlockProducerConfig 默认配置
func DefaultBlockProducerConfig() *BlockProducerConfig {
    return &BlockProducerConfig{
        MinBlockInterval: 1 * time.Second,
        MaxWaitTime:      5 * time.Second,
        MaxTxPerBlock:    1000,
        MaxGasPerBlock:   30000000,
    }
}

// BlockProducer 区块生产者
type BlockProducer struct {
    config    *BlockProducerConfig
    attestor  Attestor
    txPool    TxPool
    chain     BlockChain
    
    mu        sync.Mutex
    producing bool
    lastBlock time.Time
}

// NewBlockProducer 创建区块生产者
func NewBlockProducer(config *BlockProducerConfig, attestor Attestor, 
    txPool TxPool, chain BlockChain) *BlockProducer {
    return &BlockProducer{
        config:   config,
        attestor: attestor,
        txPool:   txPool,
        chain:    chain,
    }
}

// Start 启动区块生产
func (bp *BlockProducer) Start(ctx context.Context) error {
    go bp.produceLoop(ctx)
    return nil
}

// produceLoop 区块生产循环
func (bp *BlockProducer) produceLoop(ctx context.Context) {
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
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
    
    // 检查是否满足出块条件
    if !bp.shouldProduceBlock() {
        return
    }
    
    // 获取待打包交易
    txs := bp.txPool.Pending()
    if len(txs) == 0 {
        return // 按需出块：无交易不出块
    }
    
    // 生产区块
    block, err := bp.produceBlock(txs)
    if err != nil {
        log.Error("Failed to produce block", "err", err)
        return
    }
    
    // 广播区块
    bp.chain.InsertBlock(block)
    bp.lastBlock = time.Now()
}

// shouldProduceBlock 检查是否应该生产区块
func (bp *BlockProducer) shouldProduceBlock() bool {
    // 检查最小出块间隔
    if time.Since(bp.lastBlock) < bp.config.MinBlockInterval {
        return false
    }
    
    // 检查是否有待处理交易
    if bp.txPool.PendingCount() == 0 {
        return false
    }
    
    return true
}

// produceBlock 生产区块
func (bp *BlockProducer) produceBlock(txs []*types.Transaction) (*types.Block, error) {
    // 1. 获取父区块
    parent := bp.chain.CurrentBlock()
    
    // 2. 创建区块头
    header := &types.Header{
        ParentHash: parent.Hash(),
        Number:     new(big.Int).Add(parent.Number(), big.NewInt(1)),
        GasLimit:   bp.config.MaxGasPerBlock,
        Time:       uint64(time.Now().Unix()),
    }
    
    // 3. 生成 SGX Quote
    quote, err := bp.attestor.GenerateQuote(header.Hash().Bytes())
    if err != nil {
        return nil, fmt.Errorf("failed to generate quote: %w", err)
    }
    
    // 4. 设置扩展数据
    extra := &ExtraData{
        ProducerQuote:     quote,
        ProducerAddress:   bp.attestor.GetAddress(),
        ProducerTimestamp: uint64(time.Now().UnixNano()),
    }
    header.Extra, _ = EncodeExtraData(extra)
    
    // 5. 执行交易并组装区块
    // ... (调用 EVM 执行交易)
    
    return nil, nil
}
```

## 按需出块机制

X Chain 采用按需出块机制，只有在有待处理交易时才生产区块。

### 出块条件

```go
// consensus/sgx/on_demand.go
package sgx

import (
    "time"
)

// OnDemandConfig 按需出块配置
type OnDemandConfig struct {
    // 最小出块间隔（防止过快出块）
    MinInterval time.Duration
    
    // 最大出块间隔（即使无交易也出块，用于心跳）
    MaxInterval time.Duration
    
    // 触发出块的最小交易数
    MinTxCount int
    
    // 触发出块的最小 Gas 总量
    MinGasTotal uint64
}

// DefaultOnDemandConfig 默认配置
func DefaultOnDemandConfig() *OnDemandConfig {
    return &OnDemandConfig{
        MinInterval: 1 * time.Second,
        MaxInterval: 60 * time.Second,
        MinTxCount:  1,
        MinGasTotal: 21000, // 一笔普通转账
    }
}

// ShouldProduceBlock 判断是否应该出块
func (c *OnDemandConfig) ShouldProduceBlock(
    lastBlockTime time.Time,
    pendingTxCount int,
    pendingGasTotal uint64,
    upgradeChecker *UpgradeModeChecker,
) bool {
    // 条件 0: 升级期间，新版本节点不参与出块
    // 当白名单中存在多个 MRENCLAVE 时，新版本节点进入只读模式
    if upgradeChecker != nil && upgradeChecker.ShouldRejectWriteOperation() {
        return false
    }
    
    elapsed := time.Since(lastBlockTime)
    
    // 条件 1: 达到最大间隔，强制出块（心跳）
    if elapsed >= c.MaxInterval {
        return true
    }
    
    // 条件 2: 未达到最小间隔，不出块
    if elapsed < c.MinInterval {
        return false
    }
    
    // 条件 3: 有足够的待处理交易
    if pendingTxCount >= c.MinTxCount {
        return true
    }
    
    // 条件 4: 有足够的待处理 Gas
    if pendingGasTotal >= c.MinGasTotal {
        return true
    }
    
    return false
}
```

### 出块流程图

```
+------------------+
|   检查出块条件   |
+------------------+
        |
        v
+------------------+     是
| 升级期间只读模式?|---------> 不出块
+------------------+
        | 否
        v
+------------------+     否
| 距上次出块 > 1s? |--------+
+------------------+        |
        | 是               |
        v                  |
+------------------+       |
|  有待处理交易?   |       |
+------------------+       |
        | 是               |
        v                  |
+------------------+       |
|   收集交易       |       |
+------------------+       |
        |                  |
        v                  |
+------------------+       |
|   执行交易       |       |
+------------------+       |
        |                  |
        v                  |
+------------------+       |
|  生成 SGX Quote  |       |
+------------------+       |
        |                  |
        v                  |
+------------------+       |
|   签名区块       |       |
+------------------+       |
        |                  |
        v                  |
+------------------+       |
|   广播区块       |       |
+------------------+       |
        |                  |
        +<-----------------+
        |
        v
+------------------+
|   等待下一轮     |
+------------------+
```

## 区块验证

### 验证流程

```go
// consensus/sgx/verify.go
package sgx

import (
    "errors"
    "fmt"
    
    "github.com/ethereum/go-ethereum/consensus"
    "github.com/ethereum/go-ethereum/core/types"
)

var (
    ErrInvalidQuote     = errors.New("invalid SGX quote")
    ErrInvalidSignature = errors.New("invalid block signature")
    ErrInvalidProducer  = errors.New("invalid block producer")
    ErrFutureBlock      = errors.New("block in the future")
)

// verifyHeader 验证区块头
func (e *SGXEngine) verifyHeader(chain consensus.ChainHeaderReader, 
    header *types.Header, parents []*types.Header) error {
    
    // 1. 基本验证
    if err := e.verifyBasic(header); err != nil {
        return err
    }
    
    // 2. 验证父区块
    var parent *types.Header
    if len(parents) > 0 {
        parent = parents[len(parents)-1]
    } else {
        parent = chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
    }
    if parent == nil {
        return consensus.ErrUnknownAncestor
    }
    
    // 3. 验证时间戳
    if err := e.verifyTimestamp(header, parent); err != nil {
        return err
    }
    
    // 4. 验证 SGX Quote
    if err := e.verifyQuote(header); err != nil {
        return err
    }
    
    // 5. 验证签名
    if err := e.verifySignature(header); err != nil {
        return err
    }
    
    return nil
}

// verifyBasic 基本验证
func (e *SGXEngine) verifyBasic(header *types.Header) error {
    // 验证区块号
    if header.Number == nil {
        return errors.New("block number is nil")
    }
    
    // 验证 Extra 字段长度
    if len(header.Extra) < MinExtraDataLength {
        return fmt.Errorf("extra data too short: %d < %d", 
            len(header.Extra), MinExtraDataLength)
    }
    
    if len(header.Extra) > MaxExtraDataLength {
        return fmt.Errorf("extra data too long: %d > %d", 
            len(header.Extra), MaxExtraDataLength)
    }
    
    return nil
}

// verifyTimestamp 验证时间戳
func (e *SGXEngine) verifyTimestamp(header, parent *types.Header) error {
    // 区块时间必须大于父区块
    if header.Time <= parent.Time {
        return errors.New("block timestamp not greater than parent")
    }
    
    // 区块时间不能太超前
    if header.Time > uint64(time.Now().Add(15*time.Second).Unix()) {
        return ErrFutureBlock
    }
    
    return nil
}

// verifyQuote 验证 SGX Quote
func (e *SGXEngine) verifyQuote(header *types.Header) error {
    extra, err := DecodeExtraData(header.Extra)
    if err != nil {
        return fmt.Errorf("failed to decode extra data: %w", err)
    }
    
    // 验证 Quote
    if err := e.verifier.VerifyQuote(extra.ProducerQuote); err != nil {
        return fmt.Errorf("%w: %v", ErrInvalidQuote, err)
    }
    
    // 验证 Quote 中的 reportData 包含区块哈希
    // 这确保 Quote 是为这个特定区块生成的
    expectedReportData := header.Hash().Bytes()
    if !verifyReportData(extra.ProducerQuote, expectedReportData) {
        return errors.New("quote reportData does not match block hash")
    }
    
    return nil
}

// verifySignature 验证区块签名
func (e *SGXEngine) verifySignature(header *types.Header) error {
    extra, err := DecodeExtraData(header.Extra)
    if err != nil {
        return err
    }
    
    // 从 Quote 中提取公钥
    pubKey, err := extractPublicKeyFromQuote(extra.ProducerQuote)
    if err != nil {
        return err
    }
    
    // 验证签名
    sigHash := sigHash(header)
    if !verifySignature(pubKey, sigHash, extra.ProducerSignature) {
        return ErrInvalidSignature
    }
    
    return nil
}
```

## 冲突处理

当多个节点同时生产区块时，需要确定性的冲突解决机制。

### 分叉选择规则

```go
// consensus/sgx/fork_choice.go
package sgx

import (
    "bytes"
    
    "github.com/ethereum/go-ethereum/core/types"
)

// ForkChoice 分叉选择器
type ForkChoice struct {
    chain BlockChain
}

// SelectCanonicalBlock 选择规范区块
// 当两个区块高度相同时，使用以下规则选择：
// 1. 优先选择交易数更多的区块
// 2. 交易数相同时，选择时间戳更早的区块
// 3. 时间戳相同时，选择区块哈希更小的区块
func (fc *ForkChoice) SelectCanonicalBlock(a, b *types.Block) *types.Block {
    // 规则 1: 交易数
    if len(a.Transactions()) != len(b.Transactions()) {
        if len(a.Transactions()) > len(b.Transactions()) {
            return a
        }
        return b
    }
    
    // 规则 2: 时间戳
    if a.Time() != b.Time() {
        if a.Time() < b.Time() {
            return a
        }
        return b
    }
    
    // 规则 3: 区块哈希（确定性）
    if bytes.Compare(a.Hash().Bytes(), b.Hash().Bytes()) < 0 {
        return a
    }
    return b
}

// IsCanonical 检查区块是否是规范链上的区块
func (fc *ForkChoice) IsCanonical(block *types.Block) bool {
    canonical := fc.chain.GetBlockByNumber(block.NumberU64())
    if canonical == nil {
        return false
    }
    return canonical.Hash() == block.Hash()
}
```

### 重组处理

```go
// consensus/sgx/reorg.go
package sgx

import (
    "github.com/ethereum/go-ethereum/core/types"
)

// ReorgHandler 重组处理器
type ReorgHandler struct {
    chain    BlockChain
    txPool   TxPool
}

// HandleReorg 处理链重组
func (rh *ReorgHandler) HandleReorg(oldHead, newHead *types.Block) error {
    // 1. 找到共同祖先
    ancestor := rh.findCommonAncestor(oldHead, newHead)
    
    // 2. 回滚旧链上的交易到交易池
    oldBlocks := rh.getBlocksFromAncestor(oldHead, ancestor)
    for _, block := range oldBlocks {
        for _, tx := range block.Transactions() {
            rh.txPool.AddLocal(tx)
        }
    }
    
    // 3. 从交易池移除新链上已确认的交易
    newBlocks := rh.getBlocksFromAncestor(newHead, ancestor)
    for _, block := range newBlocks {
        for _, tx := range block.Transactions() {
            rh.txPool.RemoveTx(tx.Hash())
        }
    }
    
    return nil
}

// findCommonAncestor 找到共同祖先
func (rh *ReorgHandler) findCommonAncestor(a, b *types.Block) *types.Block {
    // 将两个区块回溯到相同高度
    for a.NumberU64() > b.NumberU64() {
        a = rh.chain.GetBlock(a.ParentHash(), a.NumberU64()-1)
    }
    for b.NumberU64() > a.NumberU64() {
        b = rh.chain.GetBlock(b.ParentHash(), b.NumberU64()-1)
    }
    
    // 同时回溯直到找到相同区块
    for a.Hash() != b.Hash() {
        a = rh.chain.GetBlock(a.ParentHash(), a.NumberU64()-1)
        b = rh.chain.GetBlock(b.ParentHash(), b.NumberU64()-1)
    }
    
    return a
}

// getBlocksFromAncestor 获取从祖先到目标的所有区块
func (rh *ReorgHandler) getBlocksFromAncestor(target, ancestor *types.Block) []*types.Block {
    var blocks []*types.Block
    current := target
    
    for current.Hash() != ancestor.Hash() {
        blocks = append(blocks, current)
        current = rh.chain.GetBlock(current.ParentHash(), current.NumberU64()-1)
    }
    
    // 反转顺序（从祖先到目标）
    for i, j := 0, len(blocks)-1; i < j; i, j = i+1, j-1 {
        blocks[i], blocks[j] = blocks[j], blocks[i]
    }
    
    return blocks
}
```

## 文件结构

```
consensus/sgx/
├── consensus.go          # Engine 接口实现
├── types.go              # 数据结构定义
├── block_producer.go     # 区块生产者
├── on_demand.go          # 按需出块逻辑
├── verify.go             # 区块验证
├── fork_choice.go        # 分叉选择
├── reorg.go              # 重组处理
├── api.go                # RPC API
├── config.go             # 配置
└── consensus_test.go     # 测试
```

## 单元测试指南

### 区块生产测试

```go
// consensus/sgx/block_producer_test.go
package sgx

import (
    "context"
    "testing"
    "time"
)

func TestBlockProduction(t *testing.T) {
    // 创建 Mock 组件
    attestor := NewMockAttestor()
    txPool := NewMockTxPool()
    chain := NewMockBlockChain()
    
    config := DefaultBlockProducerConfig()
    producer := NewBlockProducer(config, attestor, txPool, chain)
    
    // 添加测试交易
    txPool.AddTx(createTestTx())
    
    // 启动生产者
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    producer.Start(ctx)
    
    // 等待区块生产
    time.Sleep(2 * time.Second)
    
    // 验证区块已生产
    if chain.CurrentBlock().NumberU64() == 0 {
        t.Error("No block produced")
    }
}

func TestOnDemandNoTx(t *testing.T) {
    // 无交易时不应该出块
    attestor := NewMockAttestor()
    txPool := NewMockTxPool() // 空交易池
    chain := NewMockBlockChain()
    
    config := DefaultBlockProducerConfig()
    producer := NewBlockProducer(config, attestor, txPool, chain)
    
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()
    
    producer.Start(ctx)
    time.Sleep(2 * time.Second)
    
    // 验证没有新区块
    if chain.CurrentBlock().NumberU64() != 0 {
        t.Error("Block produced without transactions")
    }
}
```

### 区块验证测试

```go
// consensus/sgx/verify_test.go
package sgx

import (
    "testing"
    
    "github.com/ethereum/go-ethereum/core/types"
)

func TestVerifyValidBlock(t *testing.T) {
    engine := createTestEngine()
    chain := NewMockChainReader()
    
    // 创建有效区块
    block := createValidTestBlock(engine)
    
    // 验证应该通过
    err := engine.VerifyHeader(chain, block.Header())
    if err != nil {
        t.Errorf("Valid block verification failed: %v", err)
    }
}

func TestVerifyInvalidQuote(t *testing.T) {
    engine := createTestEngine()
    chain := NewMockChainReader()
    
    // 创建带无效 Quote 的区块
    block := createBlockWithInvalidQuote()
    
    // 验证应该失败
    err := engine.VerifyHeader(chain, block.Header())
    if err == nil {
        t.Error("Invalid quote should be rejected")
    }
}

func TestVerifyFutureBlock(t *testing.T) {
    engine := createTestEngine()
    chain := NewMockChainReader()
    
    // 创建未来时间戳的区块
    block := createFutureBlock()
    
    // 验证应该失败
    err := engine.VerifyHeader(chain, block.Header())
    if err != ErrFutureBlock {
        t.Errorf("Expected ErrFutureBlock, got: %v", err)
    }
}
```

### 分叉选择测试

```go
// consensus/sgx/fork_choice_test.go
package sgx

import (
    "testing"
)

func TestForkChoiceMoreTx(t *testing.T) {
    fc := &ForkChoice{}
    
    // 区块 A 有更多交易
    blockA := createBlockWithTxCount(10)
    blockB := createBlockWithTxCount(5)
    
    selected := fc.SelectCanonicalBlock(blockA, blockB)
    if selected.Hash() != blockA.Hash() {
        t.Error("Should select block with more transactions")
    }
}

func TestForkChoiceEarlierTimestamp(t *testing.T) {
    fc := &ForkChoice{}
    
    // 交易数相同，区块 A 时间戳更早
    blockA := createBlockWithTimestamp(1000)
    blockB := createBlockWithTimestamp(1001)
    
    selected := fc.SelectCanonicalBlock(blockA, blockB)
    if selected.Hash() != blockA.Hash() {
        t.Error("Should select block with earlier timestamp")
    }
}

func TestForkChoiceDeterministic(t *testing.T) {
    fc := &ForkChoice{}
    
    // 完全相同的条件，使用哈希决定
    blockA := createBlockWithHash([]byte{0x01})
    blockB := createBlockWithHash([]byte{0x02})
    
    // 多次调用应该返回相同结果
    for i := 0; i < 100; i++ {
        selected := fc.SelectCanonicalBlock(blockA, blockB)
        if selected.Hash() != blockA.Hash() {
            t.Error("Fork choice should be deterministic")
        }
    }
}
```

## 配置参数

```toml
# config.toml
[consensus.sgx]
# 最小出块间隔（秒）
min_block_interval = 1

# 最大出块间隔（秒，用于心跳）
max_block_interval = 60

# 单区块最大交易数
max_tx_per_block = 1000

# 单区块最大 Gas
max_gas_per_block = 30000000

# 区块验证超时（秒）
verify_timeout = 10

# 是否启用按需出块
on_demand_enabled = true
```

## 实现优先级

| 优先级 | 功能 | 预计工时 |
|--------|------|----------|
| P0 | Engine 接口基本实现 | 5 天 |
| P0 | 区块头扩展字段 | 2 天 |
| P0 | 区块验证逻辑 | 3 天 |
| P1 | 按需出块机制 | 3 天 |
| P1 | 分叉选择规则 | 2 天 |
| P2 | 重组处理 | 2 天 |
| P2 | RPC API | 2 天 |

**总计：约 3 周**

## 注意事项

1. **与 go-ethereum 兼容**：确保实现完全兼容 `consensus.Engine` 接口
2. **SGX 依赖**：区块签名和验证依赖 SGX 证明模块
3. **性能考虑**：Quote 验证可能较慢，考虑缓存机制
4. **测试覆盖**：确保所有边界条件都有测试覆盖
