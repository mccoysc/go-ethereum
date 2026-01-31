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

## 共识机制核心理念

### 设计原则

X Chain 的共识机制基于以下核心原则：

1. **不依赖多数同意**：不使用 51% 权力维持共识
2. **确定性执行**：SGX 保证所有节点执行相同代码得到相同结果
3. **数据一致性即网络身份**：保持数据一致的节点属于同一网络
4. **修改即分叉**：任何节点修改数据都意味着硬分叉

### 节点身份验证

每个节点启动时必须通过 SGX 远程证明：

```
+-------------+                    +-------------+
|   新节点    |                    |  现有节点   |
+-------------+                    +-------------+
      |                                  |
      |  1. 请求加入网络                 |
      |--------------------------------->|
      |                                  |
      |  2. 发送 RA-TLS 证书请求         |
      |<---------------------------------|
      |                                  |
      |  3. 生成 SGX Quote               |
      |  (包含 MRENCLAVE, MRSIGNER)      |
      |                                  |
      |  4. 返回 RA-TLS 证书             |
      |--------------------------------->|
      |                                  |
      |  5. 验证 SGX Quote               |
      |  - 检查 MRENCLAVE (代码度量)     |
      |  - 检查 MRSIGNER (签名者)        |
      |  - 检查 TCB 状态                 |
      |                                  |
      |  6. 验证通过，允许加入           |
      |<---------------------------------|
      |                                  |
```

### 与以太坊原有共识机制的关系

X Chain 使用自定义的 PoA-SGX 共识引擎，**完全替换**（而非删除）以太坊原有的共识机制。

**设计决策**：

| 方面 | 以太坊原有机制 | X Chain PoA-SGX |
|------|----------------|-----------------|
| 出块方式 | 定时出块（PoS ~12秒/块） | 按需出块（有交易才出块） |
| 共识算法 | Casper FFG + LMD GHOST | SGX 远程证明 + 确定性执行 |
| 代码位置 | `consensus/beacon/` | `consensus/sgx/` |
| 启用方式 | 默认启用 | 通过配置指定 |

**代码保留策略**：

```
go-ethereum/consensus/
├── beacon/          # 以太坊 PoS 共识（保留，不启用）
├── clique/          # 以太坊 PoA 共识（保留，不启用）
├── ethash/          # 以太坊 PoW 共识（保留，不启用）
└── sgx/             # X Chain PoA-SGX 共识（新增，启用）
    ├── consensus.go # 实现 consensus.Engine 接口
    ├── attestor.go  # SGX 远程证明
    └── verifier.go  # Quote 验证
```

**为什么保留原有代码**：

1. **参考实现**：原有共识代码是成熟的参考实现，有助于理解 go-ethereum 的共识接口设计
2. **测试兼容性**：部分测试用例可能依赖原有共识逻辑
3. **降低维护成本**：删除代码可能导致大量依赖关系需要修改
4. **未来扩展**：如果需要支持多种共识模式，保留代码更灵活

**启动配置**：

```go
// cmd/geth/config.go
type ConsensusConfig struct {
    Engine string // "sgx" | "clique" | "beacon" (默认 "sgx")
}

// X Chain 启动时强制使用 SGX 共识引擎
func NewConsensusEngine(config *ConsensusConfig) consensus.Engine {
    switch config.Engine {
    case "sgx":
        return sgx.New(config.SGX)
    default:
        // X Chain 不支持其他共识引擎
        panic("X Chain only supports SGX consensus engine")
    }
}
```

**重要说明**：X Chain 节点启动时会强制使用 PoA-SGX 共识引擎。即使配置文件中指定了其他共识引擎，也会被忽略或报错。这确保了网络中所有节点使用相同的共识机制。

### 设计目标

| 以太坊问题 | X Chain 解决方案 |
|------------|------------------|
| 高 Gas 费用 | 无挖矿竞争，交易费极低或为零 |
| 共识慢（~12秒出块） | SGX 确定性执行，近乎即时确认 |
| 存储大（持续出块） | 按需出块，无空块，减少存储 |

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

### 按需出块原则

**核心规则**：有新交易时才出块，可批量打包多个交易。

```
传统 PoS/PoW:
时间 ─────────────────────────────────────────────────>
      [块1] [块2] [块3] [块4] [块5] [块6] [块7] ...
      固定间隔出块，可能有大量空块

X Chain PoA-SGX:
时间 ─────────────────────────────────────────────────>
      [块1]           [块2]     [块3]
      ↑               ↑         ↑
      有交易          有交易    有多个交易(批量打包)
      无交易时不出块，节省存储
```

## 交易确认时间

```
传统以太坊 PoS:
提交交易 ──> 等待下一个区块槽(~12秒) ──> 区块确认 ──> 等待最终性(~15分钟)
总时间: 12秒 ~ 15分钟

X Chain PoA-SGX:
提交交易 ──> 立即出块 ──> 即时确认
总时间: < 1秒（网络延迟）
```

**即时确认的原因**：
1. 无需等待区块槽 - 有交易就出块
2. 无需等待共识投票 - SGX 保证代码执行正确性
3. 无需等待最终性 - 所有节点执行相同代码得到相同结果

## 出块节点选择

由于 X Chain 不依赖 51% 共识，出块节点选择采用**先到先得**原则：

```go
// 出块节点选择策略
type ProducerSelection int

const (
    // 先到先得：第一个广播有效区块的节点获得出块权
    FirstComeFirstServed ProducerSelection = iota
    
    // 交易提交者优先：交易提交到的节点优先处理
    TransactionSubmitterFirst
)

// 处理新交易
func (p *BlockProducer) OnNewTransaction(tx *types.Transaction, fromLocal bool) {
    // 添加到交易池
    p.txPool.Add(tx)
    
    // 如果是本地提交的交易，立即尝试出块
    if fromLocal && p.ShouldProduceBlock() {
        go p.TryProduceBlock()
    }
}

// 尝试出块（非阻塞）
func (p *BlockProducer) TryProduceBlock() {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    // 检查是否已有其他节点出块
    if p.hasNewerBlock() {
        return
    }
    
    block, err := p.ProduceBlock()
    if err != nil {
        log.Warn("Failed to produce block", "err", err)
        return
    }
    
    // 广播区块
    p.broadcastBlock(block)
    
    // 本地确认
    p.chain.InsertBlock(block)
}
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

## 节点激励模型

X Chain 采用**低成本效用模型**结合**稳定性激励机制**，确保节点长期稳定在线。

### 基础激励来源

| 激励来源 | 说明 |
|----------|------|
| 交易手续费 | 出块节点收取极低的交易费（可配置，甚至为零） |
| 效用价值 | 节点运营者可使用链上密钥管理等功能 |
| 服务收益 | 为用户提供交易处理服务的间接收益 |

**无区块奖励**：
- 不产生新代币，无通胀
- 降低运营成本，无需高算力或大量质押

### 出块权竞争与区块质量收益调整

**设计目标**：前三名都给收益，根据广播速度和区块质量综合调整收益分配，避免"赢家通吃"导致的恶性抢先行为。

```
问题场景:
┌─────────────────────────────────────────────────────────────────────────┐
│  传统"赢家通吃"模式的问题:                                              │
│  - 矿工为抢第一名，宁愿只打包 1 笔交易也要抢先广播                       │
│  - 第二、三名完全没有收益，浪费了已经打包好的区块                        │
│  - 导致区块碎片化、网络效率低、存储浪费                                  │
└─────────────────────────────────────────────────────────────────────────┘

解决方案: 前三名收益分配
┌─────────────────────────────────────────────────────────────────────────┐
│  第 1 名: 速度基础奖励 100% × 区块质量倍数                               │
│  第 2 名: 速度基础奖励  60% × 区块质量倍数                               │
│  第 3 名: 速度基础奖励  30% × 区块质量倍数                               │
│                                                                         │
│  结果:                                                                  │
│  - 速度快但质量低的区块: 第1名但收益可能低于高质量的第2名               │
│  - 速度慢但质量高的区块: 虽然是第2/3名，但收益可能更高                  │
│  - 激励矿工在速度和质量之间找到最优平衡                                  │
└─────────────────────────────────────────────────────────────────────────┘
```

#### 前三名收益分配机制

```go
// consensus/sgx/multi_producer_reward.go
package sgx

import (
    "math/big"
    "sort"
    "time"
    
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/core/types"
)

// BlockCandidate 候选区块
type BlockCandidate struct {
    Block       *types.Block
    Producer    common.Address
    ReceivedAt  time.Time      // 收到区块的时间
    Quality     *BlockQuality  // 区块质量评分
    Rank        int            // 排名 (1, 2, 3)
}

// MultiProducerRewardConfig 多生产者收益配置
type MultiProducerRewardConfig struct {
    // 速度基础奖励比例 (第1名=100%, 第2名=60%, 第3名=30%)
    SpeedRewardRatios []float64
    
    // 候选区块收集窗口（收到第一个区块后等待多久收集其他候选）
    CandidateWindow time.Duration
    
    // 最大候选区块数
    MaxCandidates int
}

// DefaultMultiProducerConfig 默认配置
func DefaultMultiProducerConfig() *MultiProducerRewardConfig {
    return &MultiProducerRewardConfig{
        SpeedRewardRatios: []float64{1.0, 0.6, 0.3}, // 100%, 60%, 30%
        CandidateWindow:   500 * time.Millisecond,   // 500ms 窗口
        MaxCandidates:     3,
    }
}

// MultiProducerRewardCalculator 多生产者收益计算器
type MultiProducerRewardCalculator struct {
    config        *MultiProducerRewardConfig
    qualityScorer *BlockQualityScorer
}

// CandidateReward 候选区块收益
type CandidateReward struct {
    Candidate       *BlockCandidate
    SpeedRatio      float64  // 速度奖励比例
    QualityMulti    float64  // 质量倍数
    FinalMultiplier float64  // 最终收益倍数 = SpeedRatio × QualityMulti
    Reward          *big.Int // 最终收益
}

// CalculateRewards 计算所有候选区块的收益
// 重要改进：只有包含新交易的候选区块才能获得收益
func (c *MultiProducerRewardCalculator) CalculateRewards(
    candidates []*BlockCandidate,
    totalFees *big.Int,
) []*CandidateReward {
    if len(candidates) == 0 {
        return nil
    }
    
    // 1. 按收到时间排序（确定速度排名）
    sort.Slice(candidates, func(i, j int) bool {
        return candidates[i].ReceivedAt.Before(candidates[j].ReceivedAt)
    })
    
    // 2. 计算每个候选的质量评分，并检查是否有新交易
    firstCandidateTxSet := make(map[common.Hash]bool)
    for _, tx := range candidates[0].Block.Transactions() {
        firstCandidateTxSet[tx.Hash()] = true
    }
    
    for i, candidate := range candidates {
        candidate.Rank = i + 1
        candidate.Quality = c.qualityScorer.CalculateQuality(candidate.Block)
        
        // 计算该候选区块包含的新交易数（第一名之外的交易）
        if i > 0 {
            newTxCount := 0
            for _, tx := range candidate.Block.Transactions() {
                if !firstCandidateTxSet[tx.Hash()] {
                    newTxCount++
                }
            }
            candidate.Quality.NewTxCount = uint64(newTxCount)
        } else {
            // 第一名的所有交易都是"新"交易
            candidate.Quality.NewTxCount = candidate.Quality.TxCount
        }
    }
    
    // 3. 计算收益（只有包含新交易的候选才能获得收益）
    rewards := make([]*CandidateReward, 0, len(candidates))
    totalMultiplier := 0.0
    
    for i, candidate := range candidates {
        if i >= c.config.MaxCandidates {
            break
        }
        
        // 关键改进：如果后续候选没有新交易，不分配收益
        if i > 0 && candidate.Quality.NewTxCount == 0 {
            // 该候选的所有交易都已被第一名包含，不分配收益
            continue
        }
        
        speedRatio := c.config.SpeedRewardRatios[i]
        qualityMulti := candidate.Quality.RewardMultiplier
        
        // 对于后续候选，收益按新交易比例调整
        if i > 0 {
            newTxRatio := float64(candidate.Quality.NewTxCount) / float64(candidate.Quality.TxCount)
            qualityMulti *= newTxRatio  // 只有新交易部分才计入收益
        }
        
        finalMulti := speedRatio * qualityMulti
        
        rewards = append(rewards, &CandidateReward{
            Candidate:       candidate,
            SpeedRatio:      speedRatio,
            QualityMulti:    qualityMulti,
            FinalMultiplier: finalMulti,
        })
        
        totalMultiplier += finalMulti
    }
    
    // 4. 按比例分配总交易费
    for _, reward := range rewards {
        share := reward.FinalMultiplier / totalMultiplier
        reward.Reward = new(big.Int).Mul(
            totalFees,
            big.NewInt(int64(share * 10000)),
        )
        reward.Reward.Div(reward.Reward, big.NewInt(10000))
    }
    
    return rewards
}
```

#### 区块质量评分

```go
// consensus/sgx/block_quality.go
package sgx

import (
    "math/big"
    
    "github.com/ethereum/go-ethereum/core/types"
)

// BlockQualityScorer 区块质量评分器
type BlockQualityScorer struct {
    config *QualityConfig
}

// QualityConfig 质量评分配置
type QualityConfig struct {
    // 交易数量权重 (默认 40%)
    TxCountWeight uint8
    
    // 区块大小权重 (默认 30%)
    BlockSizeWeight uint8
    
    // Gas 利用率权重 (默认 20%)
    GasUtilizationWeight uint8
    
    // 交易多样性权重 (默认 10%)
    TxDiversityWeight uint8
    
    // 最小交易数阈值（低于此值收益大幅降低）
    MinTxThreshold uint64
    
    // 目标区块大小（字节）
    TargetBlockSize uint64
    
    // 目标 Gas 利用率
    TargetGasUtilization float64
}

// DefaultQualityConfig 默认配置
func DefaultQualityConfig() *QualityConfig {
    return &QualityConfig{
        TxCountWeight:        40,
        BlockSizeWeight:      30,
        GasUtilizationWeight: 20,
        TxDiversityWeight:    10,
        MinTxThreshold:       5,           // 至少 5 笔交易
        TargetBlockSize:      1024 * 1024, // 1MB
        TargetGasUtilization: 0.8,         // 80% Gas 利用率
    }
}

// BlockQuality 区块质量评分结果
type BlockQuality struct {
    TxCount          uint64  // 交易数量
    NewTxCount       uint64  // 新交易数量（相对于第一名候选区块）
    BlockSize        uint64  // 区块大小（字节）
    GasUsed          uint64  // 使用的 Gas
    GasLimit         uint64  // Gas 上限
    UniqueSenders    uint64  // 不同发送者数量
    
    TxCountScore     uint16  // 交易数量得分 (0-10000)
    BlockSizeScore   uint16  // 区块大小得分 (0-10000)
    GasUtilScore     uint16  // Gas 利用率得分 (0-10000)
    DiversityScore   uint16  // 多样性得分 (0-10000)
    
    TotalScore       uint16  // 综合得分 (0-10000)
    RewardMultiplier float64 // 收益倍数 (0.1 - 2.0)
}

// CalculateQuality 计算区块质量
func (s *BlockQualityScorer) CalculateQuality(block *types.Block) *BlockQuality {
    txs := block.Transactions()
    
    quality := &BlockQuality{
        TxCount:   uint64(len(txs)),
        BlockSize: uint64(block.Size()),
        GasUsed:   block.GasUsed(),
        GasLimit:  block.GasLimit(),
    }
    
    // 统计不同发送者
    senders := make(map[common.Address]bool)
    for _, tx := range txs {
        from, _ := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
        senders[from] = true
    }
    quality.UniqueSenders = uint64(len(senders))
    
    // 1. 交易数量得分
    quality.TxCountScore = s.calculateTxCountScore(quality.TxCount)
    
    // 2. 区块大小得分
    quality.BlockSizeScore = s.calculateBlockSizeScore(quality.BlockSize)
    
    // 3. Gas 利用率得分
    quality.GasUtilScore = s.calculateGasUtilScore(quality.GasUsed, quality.GasLimit)
    
    // 4. 交易多样性得分
    quality.DiversityScore = s.calculateDiversityScore(quality.TxCount, quality.UniqueSenders)
    
    // 计算综合得分
    quality.TotalScore = uint16(
        (uint32(quality.TxCountScore) * uint32(s.config.TxCountWeight) +
         uint32(quality.BlockSizeScore) * uint32(s.config.BlockSizeWeight) +
         uint32(quality.GasUtilScore) * uint32(s.config.GasUtilizationWeight) +
         uint32(quality.DiversityScore) * uint32(s.config.TxDiversityWeight)) / 100,
    )
    
    // 计算收益倍数
    quality.RewardMultiplier = s.calculateRewardMultiplier(quality)
    
    return quality
}

// calculateTxCountScore 计算交易数量得分
func (s *BlockQualityScorer) calculateTxCountScore(txCount uint64) uint16 {
    if txCount == 0 {
        return 0
    }
    
    // 低于最小阈值，得分很低
    if txCount < s.config.MinTxThreshold {
        // 线性递减: 1 笔交易 = 20%, 4 笔交易 = 80%
        return uint16(txCount * 2000 / s.config.MinTxThreshold)
    }
    
    // 达到阈值后，对数增长（避免无限追求大区块）
    // 5 笔 = 8000, 10 笔 = 8500, 50 笔 = 9500, 100+ 笔 = 10000
    baseScore := uint16(8000)
    bonus := uint16(2000 * min(txCount-s.config.MinTxThreshold, 95) / 95)
    
    return baseScore + bonus
}

// calculateBlockSizeScore 计算区块大小得分
func (s *BlockQualityScorer) calculateBlockSizeScore(blockSize uint64) uint16 {
    if blockSize == 0 {
        return 0
    }
    
    // 目标大小附近得分最高
    ratio := float64(blockSize) / float64(s.config.TargetBlockSize)
    
    if ratio <= 1.0 {
        // 未达到目标大小，线性增长
        return uint16(ratio * 10000)
    }
    
    // 超过目标大小，轻微惩罚（避免过大区块）
    penalty := (ratio - 1.0) * 1000
    if penalty > 2000 {
        penalty = 2000
    }
    return uint16(10000 - penalty)
}

// calculateGasUtilScore 计算 Gas 利用率得分
func (s *BlockQualityScorer) calculateGasUtilScore(gasUsed, gasLimit uint64) uint16 {
    if gasLimit == 0 {
        return 0
    }
    
    utilization := float64(gasUsed) / float64(gasLimit)
    target := s.config.TargetGasUtilization
    
    if utilization <= target {
        // 未达到目标利用率，线性增长
        return uint16(utilization / target * 10000)
    }
    
    // 超过目标利用率，满分
    return 10000
}

// calculateDiversityScore 计算交易多样性得分
func (s *BlockQualityScorer) calculateDiversityScore(txCount, uniqueSenders uint64) uint16 {
    if txCount == 0 {
        return 0
    }
    
    // 多样性 = 不同发送者数量 / 交易数量
    diversity := float64(uniqueSenders) / float64(txCount)
    
    // 多样性越高越好（避免单一用户刷交易）
    return uint16(diversity * 10000)
}

// calculateRewardMultiplier 计算收益倍数
func (s *BlockQualityScorer) calculateRewardMultiplier(quality *BlockQuality) float64 {
    // 基于综合得分计算收益倍数
    // 得分 0-2000: 倍数 0.1-0.5 (惩罚低质量区块)
    // 得分 2000-5000: 倍数 0.5-1.0 (正常区块)
    // 得分 5000-8000: 倍数 1.0-1.5 (高质量区块)
    // 得分 8000-10000: 倍数 1.5-2.0 (优质区块)
    
    score := float64(quality.TotalScore)
    
    if score < 2000 {
        return 0.1 + (score/2000)*0.4
    } else if score < 5000 {
        return 0.5 + ((score-2000)/3000)*0.5
    } else if score < 8000 {
        return 1.0 + ((score-5000)/3000)*0.5
    } else {
        return 1.5 + ((score-8000)/2000)*0.5
    }
}
```

#### 收益分配示例

```
场景: 三个矿工同时出块

矿工 A (第1名，低质量):
┌─────────────────────────────────────────────────────────────┐
│  排名: 第 1 名（最先广播）                                   │
│  交易数量: 2 笔                                              │
│  区块质量得分: 2500                                          │
│  质量倍数: 0.58x                                             │
│                                                             │
│  速度基础奖励: 100%                                          │
│  最终倍数: 100% × 0.58 = 0.58                                │
└─────────────────────────────────────────────────────────────┘

矿工 B (第2名，高质量):
┌─────────────────────────────────────────────────────────────┐
│  排名: 第 2 名（稍慢 200ms）                                 │
│  交易数量: 30 笔                                             │
│  新交易数量: 28 笔（A 没有的交易）                           │
│  区块质量得分: 7500                                          │
│  质量倍数: 1.42x × (28/30) = 1.33x                           │
│                                                             │
│  速度基础奖励: 60%                                           │
│  最终倍数: 60% × 1.33 = 0.80                                 │
└─────────────────────────────────────────────────────────────┘

矿工 C (第3名，无新交易):
┌─────────────────────────────────────────────────────────────┐
│  排名: 第 3 名（稍慢 400ms）                                 │
│  交易数量: 15 笔                                             │
│  新交易数量: 0 笔（所有交易都已被 A 包含）                   │
│                                                             │
│  收益: 0 ETH（无新交易，不分配收益）                         │
└─────────────────────────────────────────────────────────────┘

收益分配 (假设总交易费 = 1 ETH):
┌─────────────────────────────────────────────────────────────┐
│  总倍数: 0.58 + 0.80 = 1.38                                 │
│                                                             │
│  矿工 A 收益: 1 ETH × (0.58/1.38) = 0.420 ETH (42.0%)       │
│  矿工 B 收益: 1 ETH × (0.80/1.38) = 0.580 ETH (58.0%)       │
│  矿工 C 收益: 0 ETH (0%)                                     │
│                                                             │
│  结论: 矿工 B 虽然是第2名，但因高质量和新交易贡献最大，      │
│        收益反而超过了第1名！                                │
└─────────────────────────────────────────────────────────────┘
```

**激励效果**：
- 速度仍然重要（第1名基础奖励最高）
- 新交易贡献是关键（只有包含新交易才能获得收益）
- 防止"搭便车"（后续矿工如果没有新交易，不分配收益）
- 鼓励矿工尽可能收集更多不同的交易

### 防止恶意行为

```go
// 防止恶意行为的额外规则

// 1. 交易数量评分（考虑网络状态）
// 重要：交易量少是网络状态问题，不是矿工的问题，不应惩罚矿工
// 只有在矿工明显"抢跑"（网络中有更多交易但矿工只打包少量）时才降低收益
func (s *BlockQualityScorer) evaluateTxCount(
    quality *BlockQuality,
    pendingTxCount uint64,  // 当前交易池中的待处理交易数
    maxWaitTime time.Duration,  // 最大等待时间
    actualWaitTime time.Duration,  // 实际等待时间
) {
    // 如果交易池中交易很少，矿工打包所有可用交易，不惩罚
    if quality.TxCount >= pendingTxCount {
        // 矿工已打包所有可用交易，给予满分
        return
    }
    
    // 如果已经等待到最大等待时间，不惩罚（矿工已尽力等待）
    if actualWaitTime >= maxWaitTime {
        return
    }
    
    // 只有在交易池中有更多交易，但矿工提前出块时才降低收益
    // 这是为了防止矿工"抢跑"（故意只打包少量交易以快速获得收益）
    packingRatio := float64(quality.TxCount) / float64(pendingTxCount)
    if packingRatio < 0.5 {
        // 打包比例低于 50%，降低收益
        quality.RewardMultiplier *= packingRatio + 0.5  // 最低 50% 收益
    }
}

// 2. 连续低质量区块惩罚
// 如果矿工连续出低质量区块，累积惩罚
type ProducerPenalty struct {
    ConsecutiveLowQuality int     // 连续低质量区块数
    PenaltyMultiplier     float64 // 惩罚倍数
}

func (p *ProducerPenalty) UpdatePenalty(quality *BlockQuality) {
    if quality.TotalScore < 3000 {
        p.ConsecutiveLowQuality++
        // 每连续 1 个低质量区块，惩罚 10%
        p.PenaltyMultiplier = 1.0 - float64(p.ConsecutiveLowQuality)*0.1
        if p.PenaltyMultiplier < 0.5 {
            p.PenaltyMultiplier = 0.5 // 最低 50%
        }
    } else {
        // 出高质量区块，重置惩罚
        p.ConsecutiveLowQuality = 0
        p.PenaltyMultiplier = 1.0
    }
}

// 3. 自我交易检测
// 如果区块中大部分交易来自出块者自己，降低收益
func (s *BlockQualityScorer) detectSelfTransactions(
    block *types.Block,
    producer common.Address,
) float64 {
    selfTxCount := 0
    for _, tx := range block.Transactions() {
        from, _ := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
        if from == producer {
            selfTxCount++
        }
    }
    
    selfRatio := float64(selfTxCount) / float64(len(block.Transactions()))
    
    // 自我交易比例超过 50%，收益降低
    if selfRatio > 0.5 {
        return 1.0 - (selfRatio-0.5) // 50% 自我交易 = 100% 收益，100% 自我交易 = 50% 收益
    }
    
    return 1.0
}
```

### 节点稳定性激励机制

**核心问题**：节点必须稳定在线提供服务，否则会损害用户体验，降低使用积极性，进而减少矿工收入，形成恶性循环。

```
恶性循环:
节点不稳定 → 用户体验差 → 使用减少 → 交易费减少 → 矿工收入降低 → 更少人运营节点
                                    ↑                                    |
                                    +------------------------------------+

良性循环 (目标):
节点稳定 → 用户体验好 → 使用增加 → 交易费增加 → 矿工收入提高 → 更多人运营节点
                                    ↑                                    |
                                    +------------------------------------+
```

#### SGX 签名心跳机制

利用 SGX enclave 的签名能力，节点定期发送可验证的心跳消息：

```go
// consensus/sgx/heartbeat.go
package sgx

// Heartbeat SGX 签名心跳消息
type Heartbeat struct {
    NodeID      common.Hash   // 节点标识
    Timestamp   uint64        // 心跳时间戳（Unix 秒）
    BlockHeight uint64        // 当前区块高度
    Challenge   [32]byte      // 随机挑战值（防重放）
    SGXQuote    []byte        // SGX 远程证明 Quote
    Signature   []byte        // enclave 内私钥签名
}

// HeartbeatManager 心跳管理器
type HeartbeatManager struct {
    sgxAttestor   *SGXAttestor
    peers         map[common.Hash]*PeerHeartbeatState
    config        *HeartbeatConfig
    
    // 心跳记录（用于计算在线率）
    heartbeatLog  map[common.Hash][]HeartbeatRecord
}

// HeartbeatConfig 心跳配置
type HeartbeatConfig struct {
    Interval          time.Duration // 心跳间隔，默认 30 秒
    Timeout           time.Duration // 心跳超时，默认 90 秒（3 个间隔）
    WindowSize        int           // 统计窗口大小，默认 1000 个心跳
    MinObservers      int           // 最少观测者数量，默认 3
    QuoteRefreshRate  int           // SGX Quote 刷新频率，默认每 100 个心跳
}

// GenerateHeartbeat 生成 SGX 签名心跳
func (m *HeartbeatManager) GenerateHeartbeat() (*Heartbeat, error) {
    // 1. 获取当前状态
    now := uint64(time.Now().Unix())
    blockHeight := m.chain.CurrentBlock().Number().Uint64()
    
    // 2. 生成随机挑战值（防重放攻击）
    var challenge [32]byte
    if _, err := rand.Read(challenge[:]); err != nil {
        return nil, err
    }
    
    // 3. 构造心跳数据
    hb := &Heartbeat{
        NodeID:      m.nodeID,
        Timestamp:   now,
        BlockHeight: blockHeight,
        Challenge:   challenge,
    }
    
    // 4. 在 SGX enclave 内签名
    dataToSign := m.serializeHeartbeatData(hb)
    signature, err := m.sgxAttestor.SignInEnclave(dataToSign)
    if err != nil {
        return nil, err
    }
    hb.Signature = signature
    
    // 5. 定期附加 SGX Quote（证明 enclave 身份）
    if m.shouldRefreshQuote() {
        quote, err := m.sgxAttestor.GenerateQuote(dataToSign)
        if err != nil {
            return nil, err
        }
        hb.SGXQuote = quote
    }
    
    return hb, nil
}

// VerifyHeartbeat 验证心跳消息
func (m *HeartbeatManager) VerifyHeartbeat(hb *Heartbeat) error {
    // 1. 验证时间戳（不能太旧或太新）
    now := uint64(time.Now().Unix())
    if hb.Timestamp < now-60 || hb.Timestamp > now+10 {
        return ErrInvalidTimestamp
    }
    
    // 2. 验证签名
    dataToVerify := m.serializeHeartbeatData(hb)
    if !m.verifySignature(hb.NodeID, dataToVerify, hb.Signature) {
        return ErrInvalidSignature
    }
    
    // 3. 如果包含 SGX Quote，验证 Quote
    if len(hb.SGXQuote) > 0 {
        if err := m.sgxAttestor.VerifyQuote(hb.SGXQuote, dataToVerify); err != nil {
            return fmt.Errorf("invalid SGX quote: %w", err)
        }
    }
    
    // 4. 检查重放攻击（挑战值不能重复）
    if m.isReplayedChallenge(hb.NodeID, hb.Challenge) {
        return ErrReplayAttack
    }
    
    return nil
}
```

**SGX 签名心跳的安全性**：

| 攻击类型 | 防护机制 |
|----------|----------|
| 伪造心跳 | SGX enclave 内签名，无法在 enclave 外伪造 |
| 重放攻击 | 随机挑战值 + 时间戳验证 |
| 时间欺骗 | 多节点观测 + 时间戳范围检查 |
| 身份冒充 | SGX Quote 验证 MRENCLAVE |

#### 多节点共识观测

单个节点的观测可能不准确（网络分区、恶意报告），因此采用多节点共识：

```go
// consensus/sgx/uptime_observer.go
package sgx

// UptimeObservation 单次在线观测记录
type UptimeObservation struct {
    ObserverID  common.Hash // 观测者节点 ID
    TargetID    common.Hash // 被观测节点 ID
    Timestamp   uint64      // 观测时间
    IsOnline    bool        // 是否在线
    ResponseMs  uint32      // 响应时间（毫秒）
    Signature   []byte      // 观测者签名
}

// UptimeConsensus 在线率共识计算
type UptimeConsensus struct {
    observations map[common.Hash][]UptimeObservation // 按目标节点分组
    config       *ConsensusConfig
}

// ConsensusConfig 共识配置
type ConsensusConfig struct {
    MinObservers        int     // 最少观测者数量，默认 3
    ConsensusThreshold  float64 // 共识阈值，默认 0.67 (2/3)
    ObservationWindow   time.Duration // 观测窗口，默认 1 小时
}

// CalculateUptimeScore 计算节点在线率得分
func (c *UptimeConsensus) CalculateUptimeScore(nodeID common.Hash) (uint64, error) {
    observations := c.getRecentObservations(nodeID)
    
    // 1. 检查观测者数量
    observers := c.getUniqueObservers(observations)
    if len(observers) < c.config.MinObservers {
        return 0, ErrInsufficientObservers
    }
    
    // 2. 按时间槽分组观测结果
    timeSlots := c.groupByTimeSlot(observations)
    
    // 3. 对每个时间槽计算共识结果
    var onlineSlots, totalSlots int
    for _, slotObs := range timeSlots {
        totalSlots++
        
        // 计算该时间槽的在线观测比例
        onlineCount := 0
        for _, obs := range slotObs {
            if obs.IsOnline {
                onlineCount++
            }
        }
        
        // 如果超过 2/3 观测者认为在线，则该时间槽计为在线
        if float64(onlineCount)/float64(len(slotObs)) >= c.config.ConsensusThreshold {
            onlineSlots++
        }
    }
    
    // 4. 计算在线率得分 (0-10000)
    if totalSlots == 0 {
        return 0, nil
    }
    score := uint64(onlineSlots * 10000 / totalSlots)
    
    return score, nil
}
```

**多节点共识的优势**：

```
单节点观测问题:
节点 A 观测节点 B → A 可能因网络问题误判 B 离线
                 → A 可能恶意报告 B 离线

多节点共识解决:
节点 A ─┐
节点 C ─┼─→ 共识: 2/3 以上认为在线 → 判定为在线
节点 D ─┘

防串谋机制:
- 观测者必须是活跃节点（有交易处理记录）
- 观测结果需要签名（可追溯责任）
- 异常观测模式会被检测（如某节点总是报告他人离线）
```

#### 交易参与追踪

**重要说明**：X Chain 采用按需出块机制（有交易才出块），因此不能使用"出块频率"或"出块数量"作为节点贡献的衡量标准。正确的衡量方式是**交易参与比例**，即节点处理的交易数量占网络总交易数量的比例。

```go
// consensus/sgx/tx_participation_tracker.go
package sgx

// TxParticipationTracker 交易参与追踪器
type TxParticipationTracker struct {
    participationLog map[common.Hash][]TxParticipationRecord
    networkStats     *NetworkTxStats
    config           *ParticipationConfig
}

// TxParticipationRecord 交易参与记录
type TxParticipationRecord struct {
    NodeID      common.Hash
    TxHash      common.Hash
    BlockNumber uint64
    Timestamp   uint64
    GasUsed     uint64
}

// NetworkTxStats 网络交易统计
type NetworkTxStats struct {
    TotalTxCount    uint64    // 统计窗口内的总交易数
    TotalGasUsed    uint64    // 统计窗口内的总 Gas 消耗
    WindowStart     uint64    // 统计窗口开始时间
    WindowEnd       uint64    // 统计窗口结束时间
}

// ParticipationConfig 参与追踪配置
type ParticipationConfig struct {
    WindowDuration    time.Duration // 统计窗口时长，默认 7 天
    MinTxForScore     uint64        // 计算得分的最小交易数，默认 10
}

// CalculateParticipationScore 计算交易参与得分
func (t *TxParticipationTracker) CalculateParticipationScore(nodeID common.Hash) uint64 {
    records := t.getRecentRecords(nodeID)
    
    if len(records) < int(t.config.MinTxForScore) {
        return 0 // 参与交易太少，无法评估
    }
    
    var totalScore uint64
    
    // 1. 交易数量参与比例得分（占 60%）
    nodeTxCount := uint64(len(records))
    networkTxCount := t.networkStats.TotalTxCount
    if networkTxCount > 0 {
        // 计算节点处理的交易占网络总交易的比例
        // 乘以节点数量进行归一化（假设理想情况下每个节点处理相等比例的交易）
        activeNodes := t.getActiveNodeCount()
        expectedShare := networkTxCount / activeNodes
        if expectedShare > 0 {
            participationScore := min(nodeTxCount*10000/expectedShare, 10000)
            totalScore += participationScore * 60 / 100
        }
    }
    
    // 2. Gas 贡献比例得分（占 40%）
    // 处理高 Gas 交易说明节点承担了更多计算负载
    nodeGasUsed := t.calculateNodeGasUsed(records)
    networkGasUsed := t.networkStats.TotalGasUsed
    if networkGasUsed > 0 {
        activeNodes := t.getActiveNodeCount()
        expectedGasShare := networkGasUsed / activeNodes
        if expectedGasShare > 0 {
            gasScore := min(nodeGasUsed*10000/expectedGasShare, 10000)
            totalScore += gasScore * 40 / 100
        }
    }
    
    return totalScore
}
```

## 文件结构

```
consensus/sgx/
├── consensus.go              # Engine 接口实现
├── types.go                  # 数据结构定义
├── block_producer.go         # 区块生产者
├── on_demand.go              # 按需出块逻辑
├── verify.go                 # 区块验证
├── fork_choice.go            # 分叉选择
├── reorg.go                  # 重组处理
├── block_quality.go          # 区块质量评分器
├── multi_producer_reward.go  # 多生产者收益分配
├── heartbeat.go              # SGX 签名心跳机制
├── uptime_observer.go        # 多节点在线率观测
├── tx_participation_tracker.go # 交易参与追踪
├── producer_penalty.go       # 出块者惩罚机制
├── api.go                    # RPC API
├── config.go                 # 配置
└── consensus_test.go         # 测试
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

### 区块质量评分测试

```go
// consensus/sgx/block_quality_test.go
package sgx

import (
    "testing"
    
    "github.com/ethereum/go-ethereum/core/types"
)

func TestQualityScoring(t *testing.T) {
    scorer := &BlockQualityScorer{
        config: DefaultQualityConfig(),
    }
    
    // 高质量区块：30笔交易，多样性高
    highQualityBlock := createBlockWithTxs(30, 25) // 30笔交易，25个不同发送者
    quality := scorer.CalculateQuality(highQualityBlock)
    
    if quality.TotalScore < 7000 {
        t.Errorf("High quality block should score >7000, got %d", quality.TotalScore)
    }
    
    if quality.RewardMultiplier < 1.3 {
        t.Errorf("High quality block should have multiplier >1.3, got %.2f", quality.RewardMultiplier)
    }
}

func TestLowTransactionPenalty(t *testing.T) {
    scorer := &BlockQualityScorer{
        config: DefaultQualityConfig(),
    }
    
    // 低质量区块：仅2笔交易
    lowQualityBlock := createBlockWithTxs(2, 2)
    quality := scorer.CalculateQuality(lowQualityBlock)
    
    if quality.TotalScore > 3000 {
        t.Errorf("Low quality block should score <3000, got %d", quality.TotalScore)
    }
    
    if quality.RewardMultiplier > 0.7 {
        t.Errorf("Low quality block should have multiplier <0.7, got %.2f", quality.RewardMultiplier)
    }
}

func TestDiversityPenalty(t *testing.T) {
    scorer := &BlockQualityScorer{
        config: DefaultQualityConfig(),
    }
    
    // 低多样性：50笔交易，仅1个发送者（自我刷交易）
    lowDiversityBlock := createBlockWithTxs(50, 1)
    quality := scorer.CalculateQuality(lowDiversityBlock)
    
    // 多样性得分应该很低
    if quality.DiversityScore > 2000 {
        t.Errorf("Low diversity block should have diversity score <2000, got %d", quality.DiversityScore)
    }
}
```

### 多生产者收益分配测试

```go
// consensus/sgx/multi_producer_reward_test.go
package sgx

import (
    "math/big"
    "testing"
    "time"
)

func TestMultiProducerRewardDistribution(t *testing.T) {
    config := DefaultMultiProducerConfig()
    scorer := &BlockQualityScorer{config: DefaultQualityConfig()}
    calculator := &MultiProducerRewardCalculator{
        config:        config,
        qualityScorer: scorer,
    }
    
    // 三个候选区块
    candidates := []*BlockCandidate{
        {
            Block:      createBlockWithTxs(2, 2),   // 第1名，低质量
            ReceivedAt: time.Now(),
        },
        {
            Block:      createBlockWithTxs(30, 25), // 第2名，高质量
            ReceivedAt: time.Now().Add(200 * time.Millisecond),
        },
        {
            Block:      createBlockWithTxs(15, 10), // 第3名，中等质量
            ReceivedAt: time.Now().Add(400 * time.Millisecond),
        },
    }
    
    totalFees := big.NewInt(1000000000000000000) // 1 ETH
    rewards := calculator.CalculateRewards(candidates, totalFees)
    
    // 验证：第2名高质量区块应该获得最高收益
    if rewards[1].Reward.Cmp(rewards[0].Reward) <= 0 {
        t.Error("High quality 2nd place should earn more than low quality 1st place")
    }
    
    // 验证：总收益应该等于总交易费
    totalReward := big.NewInt(0)
    for _, reward := range rewards {
        totalReward.Add(totalReward, reward.Reward)
    }
    
    if totalReward.Cmp(totalFees) != 0 {
        t.Errorf("Total rewards should equal total fees: got %s, want %s", 
            totalReward.String(), totalFees.String())
    }
}

func TestNoRewardForDuplicateTransactions(t *testing.T) {
    config := DefaultMultiProducerConfig()
    scorer := &BlockQualityScorer{config: DefaultQualityConfig()}
    calculator := &MultiProducerRewardCalculator{
        config:        config,
        qualityScorer: scorer,
    }
    
    // 创建相同交易的候选区块
    sameTxs := createTestTransactions(10)
    candidates := []*BlockCandidate{
        {
            Block:      createBlockWithSpecificTxs(sameTxs), // 第1名
            ReceivedAt: time.Now(),
        },
        {
            Block:      createBlockWithSpecificTxs(sameTxs), // 第2名，相同交易
            ReceivedAt: time.Now().Add(200 * time.Millisecond),
        },
    }
    
    totalFees := big.NewInt(1000000000000000000)
    rewards := calculator.CalculateRewards(candidates, totalFees)
    
    // 第2名没有新交易，不应该获得收益
    if len(rewards) > 1 {
        t.Error("Second candidate with no new transactions should not receive reward")
    }
    
    // 第1名应该获得全部收益
    if rewards[0].Reward.Cmp(totalFees) != 0 {
        t.Error("First candidate should receive all rewards when others have no new transactions")
    }
}

func TestPartialNewTransactions(t *testing.T) {
    config := DefaultMultiProducerConfig()
    scorer := &BlockQualityScorer{config: DefaultQualityConfig()}
    calculator := &MultiProducerRewardCalculator{
        config:        config,
        qualityScorer: scorer,
    }
    
    // 第1名：10笔交易
    firstTxs := createTestTransactions(10)
    // 第2名：20笔交易，其中5笔与第1名相同，15笔是新的
    secondTxs := append(firstTxs[:5], createTestTransactions(15)...)
    
    candidates := []*BlockCandidate{
        {
            Block:      createBlockWithSpecificTxs(firstTxs),
            ReceivedAt: time.Now(),
        },
        {
            Block:      createBlockWithSpecificTxs(secondTxs),
            ReceivedAt: time.Now().Add(200 * time.Millisecond),
        },
    }
    
    totalFees := big.NewInt(1000000000000000000)
    rewards := calculator.CalculateRewards(candidates, totalFees)
    
    // 第2名应该获得收益，但按新交易比例调整
    if len(rewards) != 2 {
        t.Error("Both candidates should receive rewards")
    }
    
    // 验证第2名的质量倍数应该按新交易比例调整
    if rewards[1].Candidate.Quality.NewTxCount != 15 {
        t.Errorf("Second candidate should have 15 new transactions, got %d", 
            rewards[1].Candidate.Quality.NewTxCount)
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

# 触发出块的最小交易数
min_tx_count = 1

# 触发出块的最小 Gas 总量
min_gas_total = 21000

# 候选区块收集窗口（毫秒）
candidate_window_ms = 500

# 最大候选区块数（前N名参与收益分配）
max_candidates = 3

# 区块质量评分配置
[consensus.sgx.quality]
# 交易数量权重 (%)
tx_count_weight = 40

# 区块大小权重 (%)
block_size_weight = 30

# Gas 利用率权重 (%)
gas_utilization_weight = 20

# 交易多样性权重 (%)
tx_diversity_weight = 10

# 最小交易数阈值
min_tx_threshold = 5

# 目标区块大小（字节）
target_block_size = 1048576  # 1MB

# 目标 Gas 利用率
target_gas_utilization = 0.8  # 80%

# 多生产者收益配置
[consensus.sgx.reward]
# 速度基础奖励比例（第1名, 第2名, 第3名）
speed_reward_ratios = [1.0, 0.6, 0.3]
```

## 实现优先级

| 优先级 | 功能 | 预计工时 |
|--------|------|----------|
| P0 | Engine 接口基本实现 | 5 天 |
| P0 | 区块头扩展字段 | 2 天 |
| P0 | 区块验证逻辑 | 3 天 |
| P0 | 区块质量评分系统 | 3 天 |
| P1 | 按需出块机制 | 3 天 |
| P1 | 分叉选择规则 | 2 天 |
| P1 | 多生产者收益分配 | 4 天 |
| P1 | 新交易追踪机制 | 2 天 |
| P2 | 重组处理 | 2 天 |
| P2 | 升级模式检查器 | 2 天 |
| P2 | SGX 签名心跳机制 | 3 天 |
| P2 | 多节点在线率观测 | 3 天 |
| P2 | 交易参与追踪 | 2 天 |
| P2 | 防止恶意行为规则 | 2 天 |
| P3 | RPC API | 2 天 |

**总计：约 5-6 周**

## 注意事项

1. **与 go-ethereum 兼容**：确保实现完全兼容 `consensus.Engine` 接口
2. **SGX 依赖**：区块签名和验证依赖 SGX 证明模块
3. **性能考虑**：Quote 验证可能较慢，考虑缓存机制
4. **测试覆盖**：确保所有边界条件都有测试覆盖
