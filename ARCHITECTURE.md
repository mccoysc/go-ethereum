# X Chain 技术架构文档

基于 Intel SGX 远程证明的以太坊兼容区块链

## 1. 概述

### 1.1 项目背景

X Chain 是一个基于 go-ethereum (Geth) 的新型区块链，使用 Intel SGX 远程证明替代传统的 PoS (Proof of Stake) 共识机制。通过 Gramine LibOS 运行时，所有节点在 SGX 可信执行环境 (TEE) 中运行，确保代码执行的正确性和数据的完整性。

### 1.2 核心特性

- **完全兼容以太坊主网**：兼容现有的智能合约和交易格式
- **SGX 远程证明共识**：不依赖 51% 多数同意，而是基于硬件可信执行环境的确定性共识
- **安全密钥管理**：通过预编译合约提供密钥创建、签名、验签、ECDH 等能力，私钥永不离开可信环境
- **硬件真随机数**：通过 SGX RDRAND 指令提供硬件级真随机数
- **数据一致性即共识**：任何节点修改数据都意味着硬分叉

### 1.3 链参数

| 参数 | 值 |
|------|-----|
| 链名称 | X |
| Chain ID | 762385986 (0x2d711642) |
| Chain ID 计算方式 | sha256("x") 前 4 字节 |

## 2. 系统架构

### 2.1 整体架构图

```
+------------------------------------------------------------------+
|                        X Chain 节点                               |
|  +------------------------------------------------------------+  |
|  |                    SGX Enclave (Gramine)                   |  |
|  |  +------------------------------------------------------+  |  |
|  |  |                   修改后的 Geth                       |  |  |
|  |  |  +----------------+  +----------------+              |  |  |
|  |  |  | SGX 共识引擎   |  | 预编译合约     |              |  |  |
|  |  |  | (PoA-SGX)      |  | (密钥管理)     |              |  |  |
|  |  |  +----------------+  +----------------+              |  |  |
|  |  |  +----------------+  +----------------+              |  |  |
|  |  |  | P2P 网络层     |  | EVM 执行层     |              |  |  |
|  |  |  | (RA-TLS)       |  |                |              |  |  |
|  |  |  +----------------+  +----------------+              |  |  |
|  |  +------------------------------------------------------+  |  |
|  |                           |                                |  |
|  |  +------------------------------------------------------+  |  |
|  |  |              Gramine 加密分区                         |  |  |
|  |  |  - 私钥存储                                          |  |  |
|  |  |  - 派生秘密 (ECDH 结果等)                            |  |  |
|  |  |  - 区块链数据                                        |  |  |
|  |  +------------------------------------------------------+  |  |
|  +------------------------------------------------------------+  |
+------------------------------------------------------------------+
                              |
                    RA-TLS 加密通道
                              |
+------------------------------------------------------------------+
|                      其他 X Chain 节点                            |
+------------------------------------------------------------------+
```

### 2.2 核心组件

#### 2.2.1 SGX 共识引擎 (PoA-SGX)

新的共识引擎实现 `consensus.Engine` 接口，基于 SGX 远程证明：

```go
// consensus/sgx/consensus.go
package sgx

type SGXConsensus struct {
    config     *params.SGXConfig
    attestor   *SGXAttestor      // SGX 远程证明器
    keyManager *KeyManager       // 密钥管理器
}

// 实现 consensus.Engine 接口
func (s *SGXConsensus) VerifyHeader(chain ChainHeaderReader, header *types.Header) error
func (s *SGXConsensus) Seal(chain ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error
func (s *SGXConsensus) Finalize(chain ChainHeaderReader, header *types.Header, state vm.StateDB, body *types.Body)
```

#### 2.2.2 预编译合约系统

新增预编译合约地址范围：`0x8000` - `0x80FF`（从 32768 开始，避免与以太坊未来预编译地址冲突）

| 地址 | 功能 | 描述 |
|------|------|------|
| 0x8000 | SGX_KEY_CREATE | 创建密钥对 |
| 0x8001 | SGX_KEY_GET_PUBLIC | 获取公钥 |
| 0x8002 | SGX_SIGN | 签名 |
| 0x8003 | SGX_VERIFY | 验签 |
| 0x8004 | SGX_ECDH | ECDH 密钥交换 |
| 0x8005 | SGX_RANDOM | 硬件真随机数 |
| 0x8006 | SGX_ENCRYPT | 对称加密 |
| 0x8007 | SGX_DECRYPT | 对称解密 |
| 0x8008 | SGX_KEY_DERIVE | 密钥派生 |

#### 2.2.3 Gramine 运行时集成

节点通过 Gramine LibOS 在 SGX enclave 中运行：

```
gramine-sgx geth --datadir /app/wallet/chaindata --networkid 762385986
```

## 3. 共识机制详细设计

### 3.1 核心理念

X Chain 的共识机制基于以下核心原则：

1. **不依赖多数同意**：不使用 51% 权力维持共识
2. **确定性执行**：SGX 保证所有节点执行相同代码得到相同结果
3. **数据一致性即网络身份**：保持数据一致的节点属于同一网络
4. **修改即分叉**：任何节点修改数据都意味着硬分叉

### 3.2 节点身份验证

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

### 3.3 区块生产机制

X Chain 采用**按需出块**的 PoA-SGX 模式，解决以太坊的三大问题：高成本、慢共识、大存储。

#### 3.3.1 设计目标

| 以太坊问题 | X Chain 解决方案 |
|------------|------------------|
| 高 Gas 费用 | 无挖矿竞争，交易费极低或为零 |
| 共识慢（~12秒出块） | SGX 确定性执行，近乎即时确认 |
| 存储大（持续出块） | 按需出块，无空块，减少存储 |

#### 3.3.2 按需出块原则

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

#### 3.3.3 出块触发条件

```go
// consensus/sgx/block_producer.go
package sgx

type BlockProducer struct {
    txPool      *TxPool
    chain       *BlockChain
    sgxAttestor *SGXAttestor
    
    // 配置参数
    maxTxPerBlock   int           // 每块最大交易数
    maxWaitTime     time.Duration // 最大等待时间（可选）
    minTxForBlock   int           // 触发出块的最小交易数
}

// ShouldProduceBlock 判断是否应该出块
func (p *BlockProducer) ShouldProduceBlock() bool {
    pendingTxs := p.txPool.Pending()
    
    // 条件1：有待处理交易
    if len(pendingTxs) == 0 {
        return false
    }
    
    // 条件2：达到最小交易数阈值（可配置，默认为1）
    if len(pendingTxs) >= p.minTxForBlock {
        return true
    }
    
    // 条件3：超过最大等待时间（可选，防止交易长时间不被处理）
    if p.maxWaitTime > 0 {
        oldestTx := p.txPool.OldestPendingTime()
        if time.Since(oldestTx) > p.maxWaitTime {
            return true
        }
    }
    
    return false
}

// ProduceBlock 生产新区块
func (p *BlockProducer) ProduceBlock() (*types.Block, error) {
    // 1. 获取待处理交易（按 Gas 价格排序，最多 maxTxPerBlock 个）
    txs := p.txPool.GetPendingTxs(p.maxTxPerBlock)
    if len(txs) == 0 {
        return nil, ErrNoTransactions
    }
    
    // 2. 创建区块头
    parent := p.chain.CurrentBlock()
    header := &types.Header{
        ParentHash: parent.Hash(),
        Number:     new(big.Int).Add(parent.Number(), big.NewInt(1)),
        GasLimit:   p.calculateGasLimit(parent),
        Time:       uint64(time.Now().Unix()),
        Coinbase:   p.coinbase,
    }
    
    // 3. 执行交易，生成状态根
    stateRoot, receipts, err := p.executeTransactions(txs, header)
    if err != nil {
        return nil, err
    }
    header.Root = stateRoot
    
    // 4. 添加 SGX 证明数据到 Extra 字段
    sgxExtra, err := p.createSGXExtra()
    if err != nil {
        return nil, err
    }
    header.Extra = sgxExtra
    
    // 5. 组装区块
    block := types.NewBlock(header, &types.Body{Transactions: txs}, receipts, nil)
    
    // 6. 签名区块
    signedBlock, err := p.signBlock(block)
    if err != nil {
        return nil, err
    }
    
    return signedBlock, nil
}
```

#### 3.3.4 区块头扩展字段

```go
// SGX 扩展数据结构（存储在 Header.Extra 中）
type SGXExtra struct {
    // 出块节点的 SGX Quote（证明代码完整性）
    SGXQuote      []byte  `json:"sgxQuote"`
    
    // 出块节点标识（从 SGX Quote 中提取的公钥哈希）
    ProducerID    []byte  `json:"producerId"`
    
    // SGX 证明时间戳
    AttestationTS uint64  `json:"attestationTs"`
    
    // 区块签名（使用节点私钥签名）
    Signature     []byte  `json:"signature"`
}

// 序列化 SGX Extra 数据
func (e *SGXExtra) Encode() ([]byte, error) {
    return rlp.EncodeToBytes(e)
}

// 反序列化 SGX Extra 数据
func DecodeSGXExtra(data []byte) (*SGXExtra, error) {
    var extra SGXExtra
    if err := rlp.DecodeBytes(data, &extra); err != nil {
        return nil, err
    }
    return &extra, nil
}
```

#### 3.3.5 出块节点选择

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

#### 3.3.6 交易确认时间

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

#### 3.3.7 冲突处理

当多个节点同时出块时：

```go
// 区块冲突解决策略
func (p *BlockProducer) ResolveConflict(blocks []*types.Block) *types.Block {
    // 规则1：选择包含更多交易的区块
    sort.Slice(blocks, func(i, j int) bool {
        return len(blocks[i].Transactions()) > len(blocks[j].Transactions())
    })
    
    // 规则2：交易数相同时，选择时间戳更早的
    if len(blocks) > 1 && len(blocks[0].Transactions()) == len(blocks[1].Transactions()) {
        sort.Slice(blocks, func(i, j int) bool {
            return blocks[i].Time() < blocks[j].Time()
        })
    }
    
    // 规则3：时间戳也相同时，选择区块哈希更小的（确定性）
    if len(blocks) > 1 && blocks[0].Time() == blocks[1].Time() {
        sort.Slice(blocks, func(i, j int) bool {
            return bytes.Compare(blocks[i].Hash().Bytes(), blocks[j].Hash().Bytes()) < 0
        })
    }
    
    return blocks[0]
}
```

#### 3.3.8 节点激励模型

X Chain 采用**低成本效用模型**结合**稳定性激励机制**，确保节点长期稳定在线。

##### 3.3.8.1 基础激励来源

| 激励来源 | 说明 |
|----------|------|
| 交易手续费 | 出块节点收取极低的交易费（可配置，甚至为零） |
| 效用价值 | 节点运营者可使用链上密钥管理等功能 |
| 服务收益 | 为用户提供交易处理服务的间接收益 |

**无区块奖励**：
- 不产生新代币，无通胀
- 降低运营成本，无需高算力或大量质押

##### 3.3.8.2 出块权竞争与区块质量收益调整

**设计目标**：保持"先广播先得"的简单出块逻辑，同时通过收益调整机制激励矿工批量打包交易，避免为抢先出块而拒绝打包更多交易。

```
问题场景:
┌─────────────────────────────────────────────────────────────────────────┐
│  矿工 A: 收到 1 笔交易，立即出块广播（抢先）                              │
│  矿工 B: 等待收集 10 笔交易，准备批量打包                                │
│                                                                         │
│  如果只按"先到先得"，矿工 A 获得出块权，但只打包了 1 笔交易              │
│  这导致: 区块碎片化、网络效率低、存储浪费                                │
└─────────────────────────────────────────────────────────────────────────┘

解决方案:
┌─────────────────────────────────────────────────────────────────────────┐
│  出块权: 保持"先广播先得"（简单、确定性）                                │
│  收益:   根据区块质量调整（激励批量打包）                                │
│                                                                         │
│  结果: 矿工 A 虽然抢到出块权，但收益很低（区块质量差）                   │
│        矿工 B 下次会更积极批量打包，因为高质量区块收益更高               │
└─────────────────────────────────────────────────────────────────────────┘
```

###### 3.3.8.2.1 区块质量评分

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

###### 3.3.8.2.2 收益计算

```go
// consensus/sgx/block_reward.go
package sgx

import (
    "math/big"
    
    "github.com/ethereum/go-ethereum/core/types"
)

// BlockRewardCalculator 区块收益计算器
type BlockRewardCalculator struct {
    qualityScorer *BlockQualityScorer
}

// BlockReward 区块收益
type BlockReward struct {
    Block           *types.Block
    Quality         *BlockQuality
    
    BaseFees        *big.Int // 基础交易费总和
    AdjustedReward  *big.Int // 调整后的收益
    
    // 收益明细
    TxFeeReward     *big.Int // 交易费收益
    QualityBonus    *big.Int // 质量奖励
}

// CalculateReward 计算区块收益
func (c *BlockRewardCalculator) CalculateReward(block *types.Block, receipts []*types.Receipt) *BlockReward {
    // 1. 计算基础交易费
    baseFees := big.NewInt(0)
    for i, tx := range block.Transactions() {
        if i < len(receipts) {
            gasUsed := big.NewInt(int64(receipts[i].GasUsed))
            gasPrice := tx.GasPrice()
            fee := new(big.Int).Mul(gasUsed, gasPrice)
            baseFees.Add(baseFees, fee)
        }
    }
    
    // 2. 计算区块质量
    quality := c.qualityScorer.CalculateQuality(block)
    
    // 3. 应用质量倍数
    multiplier := big.NewFloat(quality.RewardMultiplier)
    baseFloat := new(big.Float).SetInt(baseFees)
    adjustedFloat := new(big.Float).Mul(baseFloat, multiplier)
    
    adjustedReward := new(big.Int)
    adjustedFloat.Int(adjustedReward)
    
    // 4. 计算质量奖励（调整后收益 - 基础费用）
    qualityBonus := new(big.Int).Sub(adjustedReward, baseFees)
    if qualityBonus.Sign() < 0 {
        qualityBonus = big.NewInt(0)
    }
    
    return &BlockReward{
        Block:          block,
        Quality:        quality,
        BaseFees:       baseFees,
        AdjustedReward: adjustedReward,
        TxFeeReward:    baseFees,
        QualityBonus:   qualityBonus,
    }
}
```

###### 3.3.8.2.3 收益调整示例

```
场景对比:

矿工 A (抢先出块，低质量):
┌─────────────────────────────────────────────────────────────┐
│  交易数量: 1 笔                                              │
│  区块大小: 500 字节                                          │
│  Gas 利用率: 2%                                              │
│  交易多样性: 100% (1/1)                                      │
│                                                             │
│  交易数量得分: 2000 (低于阈值 5 笔)                          │
│  区块大小得分: 500 (远低于目标 1MB)                          │
│  Gas 利用率得分: 250 (远低于目标 80%)                        │
│  多样性得分: 10000 (满分)                                    │
│                                                             │
│  综合得分: 2000*40% + 500*30% + 250*20% + 10000*10%         │
│          = 800 + 150 + 50 + 1000 = 2000                     │
│                                                             │
│  收益倍数: 0.5x                                              │
│  基础交易费: 0.001 ETH                                       │
│  实际收益: 0.0005 ETH                                        │
└─────────────────────────────────────────────────────────────┘

矿工 B (批量打包，高质量):
┌─────────────────────────────────────────────────────────────┐
│  交易数量: 50 笔                                             │
│  区块大小: 100KB                                             │
│  Gas 利用率: 60%                                             │
│  交易多样性: 80% (40/50)                                     │
│                                                             │
│  交易数量得分: 9500 (超过阈值，对数增长)                     │
│  区块大小得分: 1000 (10% 目标大小)                           │
│  Gas 利用率得分: 7500 (75% 目标利用率)                       │
│  多样性得分: 8000 (80% 多样性)                               │
│                                                             │
│  综合得分: 9500*40% + 1000*30% + 7500*20% + 8000*10%        │
│          = 3800 + 300 + 1500 + 800 = 6400                   │
│                                                             │
│  收益倍数: 1.23x                                             │
│  基础交易费: 0.05 ETH (50 笔交易)                            │
│  实际收益: 0.0615 ETH                                        │
└─────────────────────────────────────────────────────────────┘

结论:
- 矿工 A 抢先出块，但收益只有 0.0005 ETH
- 矿工 B 批量打包，收益 0.0615 ETH (是 A 的 123 倍)
- 激励效果: 矿工会倾向于等待更多交易再出块
```

###### 3.3.8.2.4 防止恶意行为

```go
// 防止恶意行为的额外规则

// 1. 最小交易数惩罚
// 如果区块只有 1-2 笔交易，收益倍数最高只有 0.3x
func (s *BlockQualityScorer) applyMinTxPenalty(quality *BlockQuality) {
    if quality.TxCount <= 2 {
        if quality.RewardMultiplier > 0.3 {
            quality.RewardMultiplier = 0.3
        }
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

##### 3.3.8.3 节点稳定性激励机制

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

##### 3.3.8.3 稳定在线的准确衡量机制

**核心挑战**：在去中心化网络中，如何准确、可验证、防伪造地衡量节点的在线状态？

```
衡量要求:
┌─────────────────────────────────────────────────────────────┐
│  1. 去中心化 - 无单点故障，无中心化监控                      │
│  2. 可验证性 - 在线状态可被密码学证明                        │
│  3. 防伪造   - 节点无法伪造在线记录                          │
│  4. 抗串谋   - 多个节点无法串谋伪造彼此的在线状态            │
│  5. 低开销   - 衡量机制不应显著增加网络负担                  │
└─────────────────────────────────────────────────────────────┘
```

###### 3.3.8.3.1 SGX 签名心跳机制

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

###### 3.3.8.3.2 多节点共识观测

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

// RecordObservation 记录观测结果
func (c *UptimeConsensus) RecordObservation(obs *UptimeObservation) error {
    // 1. 验证观测者签名
    if err := c.verifyObservation(obs); err != nil {
        return err
    }
    
    // 2. 检查观测者是否有资格（必须是活跃节点）
    if !c.isQualifiedObserver(obs.ObserverID) {
        return ErrUnqualifiedObserver
    }
    
    // 3. 防止自我观测
    if obs.ObserverID == obs.TargetID {
        return ErrSelfObservation
    }
    
    // 4. 记录观测
    c.observations[obs.TargetID] = append(c.observations[obs.TargetID], *obs)
    
    return nil
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
- 观测者必须是活跃节点（有出块记录）
- 观测结果需要签名（可追溯责任）
- 异常观测模式会被检测（如某节点总是报告他人离线）
```

###### 3.3.8.3.3 区块生产追踪

对于参与出块的节点，区块生产记录是最直接的在线证明：

```go
// consensus/sgx/block_tracker.go
package sgx

// BlockProductionTracker 区块生产追踪器
type BlockProductionTracker struct {
    productionLog map[common.Hash][]BlockProductionRecord
    config        *TrackerConfig
}

// BlockProductionRecord 区块生产记录
type BlockProductionRecord struct {
    NodeID      common.Hash
    BlockNumber uint64
    BlockHash   common.Hash
    Timestamp   uint64
    TxCount     int
}

// TrackerConfig 追踪器配置
type TrackerConfig struct {
    WindowBlocks    uint64  // 统计窗口（区块数），默认 1000
    MinBlocksForScore uint64 // 计算得分的最小区块数，默认 10
}

// CalculateProductionScore 计算区块生产得分
func (t *BlockProductionTracker) CalculateProductionScore(nodeID common.Hash) uint64 {
    records := t.getRecentRecords(nodeID)
    
    if len(records) < int(t.config.MinBlocksForScore) {
        return 0 // 出块太少，无法评估
    }
    
    // 计算出块频率和质量
    var totalScore uint64
    
    // 1. 出块数量得分（占 50%）
    blockCount := uint64(len(records))
    expectedBlocks := t.getExpectedBlocks(nodeID) // 基于节点活跃时长
    if expectedBlocks > 0 {
        blockScore := min(blockCount*10000/expectedBlocks, 10000)
        totalScore += blockScore * 50 / 100
    }
    
    // 2. 出块间隔稳定性得分（占 30%）
    intervalScore := t.calculateIntervalStability(records)
    totalScore += intervalScore * 30 / 100
    
    // 3. 区块质量得分（交易数量）（占 20%）
    qualityScore := t.calculateBlockQuality(records)
    totalScore += qualityScore * 20 / 100
    
    return totalScore
}

// calculateIntervalStability 计算出块间隔稳定性
func (t *BlockProductionTracker) calculateIntervalStability(records []BlockProductionRecord) uint64 {
    if len(records) < 2 {
        return 0
    }
    
    // 计算间隔的标准差
    var intervals []uint64
    for i := 1; i < len(records); i++ {
        interval := records[i].Timestamp - records[i-1].Timestamp
        intervals = append(intervals, interval)
    }
    
    // 标准差越小，得分越高
    stdDev := t.calculateStdDev(intervals)
    avgInterval := t.calculateAvg(intervals)
    
    if avgInterval == 0 {
        return 0
    }
    
    // 变异系数 (CV) = stdDev / avg
    // CV 越小越稳定，得分越高
    cv := float64(stdDev) / float64(avgInterval)
    if cv > 1.0 {
        return 0
    }
    return uint64((1.0 - cv) * 10000)
}
```

###### 3.3.8.3.4 交易响应时间追踪

测量节点处理交易的响应速度：

```go
// consensus/sgx/response_tracker.go
package sgx

// ResponseTimeTracker 响应时间追踪器
type ResponseTimeTracker struct {
    responseLogs map[common.Hash][]ResponseRecord
    config       *ResponseConfig
}

// ResponseRecord 响应记录
type ResponseRecord struct {
    NodeID       common.Hash
    TxHash       common.Hash
    SubmitTime   uint64 // 交易提交时间
    ResponseTime uint64 // 收到响应时间
    Success      bool   // 是否成功处理
}

// ResponseConfig 响应配置
type ResponseConfig struct {
    WindowSize      int           // 统计窗口大小，默认 100
    ExcellentMs     uint32        // 优秀响应时间，默认 100ms
    GoodMs          uint32        // 良好响应时间，默认 500ms
    AcceptableMs    uint32        // 可接受响应时间，默认 2000ms
}

// CalculateResponseScore 计算响应得分
func (t *ResponseTimeTracker) CalculateResponseScore(nodeID common.Hash) uint64 {
    records := t.getRecentRecords(nodeID)
    
    if len(records) == 0 {
        return 5000 // 无记录时给中等分数
    }
    
    var totalScore uint64
    var validCount int
    
    for _, record := range records {
        if !record.Success {
            continue // 失败的不计入响应时间
        }
        
        validCount++
        responseMs := uint32((record.ResponseTime - record.SubmitTime) * 1000)
        
        // 根据响应时间计算得分
        var score uint64
        switch {
        case responseMs <= t.config.ExcellentMs:
            score = 10000 // 优秀
        case responseMs <= t.config.GoodMs:
            score = 8000 // 良好
        case responseMs <= t.config.AcceptableMs:
            score = 6000 // 可接受
        default:
            score = 3000 // 较慢
        }
        
        totalScore += score
    }
    
    if validCount == 0 {
        return 5000
    }
    
    return totalScore / uint64(validCount)
}
```

###### 3.3.8.3.5 综合在线率计算

将以上四种衡量机制综合计算：

```go
// consensus/sgx/uptime_calculator.go
package sgx

// UptimeCalculator 综合在线率计算器
type UptimeCalculator struct {
    heartbeatMgr    *HeartbeatManager
    consensusMgr    *UptimeConsensus
    blockTracker    *BlockProductionTracker
    responseTracker *ResponseTimeTracker
    config          *UptimeConfig
}

// UptimeConfig 在线率计算配置
type UptimeConfig struct {
    // 权重配置（总和 = 100）
    HeartbeatWeight   uint8 // SGX 心跳权重，默认 40
    ConsensusWeight   uint8 // 多节点共识权重，默认 30
    BlockWeight       uint8 // 区块生产权重，默认 20
    ResponseWeight    uint8 // 响应时间权重，默认 10
}

// CalculateComprehensiveUptime 计算综合在线率
func (c *UptimeCalculator) CalculateComprehensiveUptime(nodeID common.Hash) uint64 {
    cfg := c.config
    
    // 1. SGX 心跳得分
    heartbeatScore := c.heartbeatMgr.GetHeartbeatScore(nodeID)
    
    // 2. 多节点共识得分
    consensusScore, _ := c.consensusMgr.CalculateUptimeScore(nodeID)
    
    // 3. 区块生产得分
    blockScore := c.blockTracker.CalculateProductionScore(nodeID)
    
    // 4. 响应时间得分
    responseScore := c.responseTracker.CalculateResponseScore(nodeID)
    
    // 5. 加权计算
    totalScore := (heartbeatScore * uint64(cfg.HeartbeatWeight) +
                   consensusScore * uint64(cfg.ConsensusWeight) +
                   blockScore * uint64(cfg.BlockWeight) +
                   responseScore * uint64(cfg.ResponseWeight)) / 100
    
    return totalScore
}
```

**衡量机制总结**：

| 机制 | 权重 | 衡量内容 | 防伪造方式 |
|------|------|----------|------------|
| SGX 签名心跳 | 40% | 节点是否定期发送心跳 | SGX enclave 签名 + Quote |
| 多节点共识 | 30% | 多个节点观测的共识结果 | 2/3 共识 + 签名追溯 |
| 区块生产 | 20% | 实际出块数量和质量 | 区块链不可篡改记录 |
| 响应时间 | 10% | 交易处理响应速度 | 交易哈希 + 时间戳 |

##### 3.3.8.4 信誉系统设计

```go
// consensus/sgx/reputation.go
package sgx

// NodeReputation 节点信誉数据
type NodeReputation struct {
    NodeID          common.Hash   // 节点标识
    UptimeScore     uint64        // 在线时长得分 (0-10000, 代表 0%-100%)
    ResponseScore   uint64        // 响应速度得分
    SuccessRate     uint64        // 交易处理成功率
    TotalBlocks     uint64        // 累计出块数
    LastActiveTime  uint64        // 最后活跃时间戳
    PenaltyCount    uint64        // 惩罚次数
    ReputationScore uint64        // 综合信誉分 (0-10000)
}

// ReputationManager 信誉管理器
type ReputationManager struct {
    reputations map[common.Hash]*NodeReputation
    config      *ReputationConfig
}

// ReputationConfig 信誉系统配置
type ReputationConfig struct {
    // 权重配置 (总和 = 100)
    UptimeWeight      uint8  // 在线时长权重，默认 40
    ResponseWeight    uint8  // 响应速度权重，默认 20
    SuccessRateWeight uint8  // 成功率权重，默认 30
    HistoryWeight     uint8  // 历史记录权重，默认 10
    
    // 阈值配置
    MinUptimeForReward    uint64        // 获得奖励的最低在线率，默认 95%
    PenaltyThreshold      uint64        // 触发惩罚的在线率阈值，默认 80%
    RecoveryPeriod        time.Duration // 惩罚恢复期，默认 24 小时
}

// CalculateReputationScore 计算综合信誉分
func (m *ReputationManager) CalculateReputationScore(r *NodeReputation) uint64 {
    cfg := m.config
    
    // 加权计算
    score := (r.UptimeScore * uint64(cfg.UptimeWeight) +
              r.ResponseScore * uint64(cfg.ResponseWeight) +
              r.SuccessRate * uint64(cfg.SuccessRateWeight) +
              m.calculateHistoryScore(r) * uint64(cfg.HistoryWeight)) / 100
    
    // 惩罚扣分
    if r.PenaltyCount > 0 {
        penaltyDeduction := r.PenaltyCount * 500 // 每次惩罚扣 5%
        if penaltyDeduction > score {
            score = 0
        } else {
            score -= penaltyDeduction
        }
    }
    
    return score
}

// UpdateUptime 更新在线时长
func (m *ReputationManager) UpdateUptime(nodeID common.Hash, isOnline bool) {
    r := m.getOrCreateReputation(nodeID)
    
    now := uint64(time.Now().Unix())
    
    if isOnline {
        // 在线：增加得分
        r.UptimeScore = min(r.UptimeScore + 1, 10000)
        r.LastActiveTime = now
    } else {
        // 离线：减少得分
        if r.UptimeScore > 10 {
            r.UptimeScore -= 10 // 离线惩罚更重
        } else {
            r.UptimeScore = 0
        }
    }
    
    r.ReputationScore = m.CalculateReputationScore(r)
}
```

##### 3.3.8.4 交易费加权分配

高信誉节点获得更高比例的交易费：

```go
// 交易费分配策略
type FeeDistribution struct {
    // 基础费用分配
    BaseFee *big.Int
    
    // 信誉加权系数
    ReputationMultiplier func(score uint64) *big.Int
}

// CalculateFeeShare 计算节点的交易费份额
func (d *FeeDistribution) CalculateFeeShare(
    totalFee *big.Int,
    nodeReputation uint64,
    allNodes []*NodeReputation,
) *big.Int {
    // 计算所有节点的加权总分
    var totalWeightedScore uint64
    for _, node := range allNodes {
        totalWeightedScore += d.getWeightedScore(node.ReputationScore)
    }
    
    if totalWeightedScore == 0 {
        return big.NewInt(0)
    }
    
    // 按加权比例分配
    nodeWeightedScore := d.getWeightedScore(nodeReputation)
    share := new(big.Int).Mul(totalFee, big.NewInt(int64(nodeWeightedScore)))
    share.Div(share, big.NewInt(int64(totalWeightedScore)))
    
    return share
}

// getWeightedScore 获取加权得分（高信誉节点获得更高权重）
func (d *FeeDistribution) getWeightedScore(reputationScore uint64) uint64 {
    // 信誉分 >= 9000 (90%): 权重 x2.0
    // 信誉分 >= 8000 (80%): 权重 x1.5
    // 信誉分 >= 7000 (70%): 权重 x1.0
    // 信誉分 < 7000: 权重 x0.5
    
    switch {
    case reputationScore >= 9000:
        return reputationScore * 2
    case reputationScore >= 8000:
        return reputationScore * 3 / 2
    case reputationScore >= 7000:
        return reputationScore
    default:
        return reputationScore / 2
    }
}
```

##### 3.3.8.5 惩罚机制

```go
// PenaltyManager 惩罚管理器
type PenaltyManager struct {
    reputationMgr *ReputationManager
    config        *PenaltyConfig
}

// PenaltyConfig 惩罚配置
type PenaltyConfig struct {
    // 离线惩罚
    OfflineThreshold   time.Duration // 离线多久触发惩罚，默认 10 分钟
    OfflinePenalty     uint64        // 离线惩罚扣分，默认 500 (5%)
    
    // 频繁离线惩罚
    FrequentOfflineCount    int           // 频繁离线次数阈值，默认 3 次/天
    FrequentOfflinePenalty  uint64        // 频繁离线惩罚，默认 1000 (10%)
    
    // 恢复机制
    RecoveryRate       uint64        // 每小时恢复的惩罚分，默认 50
    MaxPenaltyCount    uint64        // 最大惩罚次数（超过则暂时排除），默认 10
}

// CheckAndPenalize 检查并执行惩罚
func (p *PenaltyManager) CheckAndPenalize(nodeID common.Hash) {
    r := p.reputationMgr.getReputation(nodeID)
    if r == nil {
        return
    }
    
    now := uint64(time.Now().Unix())
    offlineDuration := now - r.LastActiveTime
    
    // 检查是否超过离线阈值
    if offlineDuration > uint64(p.config.OfflineThreshold.Seconds()) {
        r.PenaltyCount++
        r.ReputationScore = p.reputationMgr.CalculateReputationScore(r)
        
        log.Warn("Node penalized for being offline",
            "nodeID", nodeID,
            "offlineDuration", offlineDuration,
            "penaltyCount", r.PenaltyCount)
    }
    
    // 检查是否应该暂时排除
    if r.PenaltyCount >= p.config.MaxPenaltyCount {
        p.excludeNode(nodeID)
    }
}

// excludeNode 暂时排除节点（不参与出块）
func (p *PenaltyManager) excludeNode(nodeID common.Hash) {
    log.Warn("Node excluded from block production due to excessive penalties",
        "nodeID", nodeID)
    // 节点仍可同步数据，但不能出块
    // 需要连续在线一段时间后才能恢复
}
```

##### 3.3.8.6 节点优先级排序

用户提交交易时，优先选择高信誉节点：

```go
// NodeSelector 节点选择器
type NodeSelector struct {
    reputationMgr *ReputationManager
    nodes         []*NodeInfo
}

// SelectBestNodes 选择最佳节点（按信誉排序）
func (s *NodeSelector) SelectBestNodes(count int) []*NodeInfo {
    // 获取所有在线节点
    onlineNodes := s.getOnlineNodes()
    
    // 按信誉分排序
    sort.Slice(onlineNodes, func(i, j int) bool {
        ri := s.reputationMgr.getReputation(onlineNodes[i].ID)
        rj := s.reputationMgr.getReputation(onlineNodes[j].ID)
        return ri.ReputationScore > rj.ReputationScore
    })
    
    // 返回前 N 个高信誉节点
    if len(onlineNodes) > count {
        return onlineNodes[:count]
    }
    return onlineNodes
}

// GetNodePriority 获取节点处理交易的优先级
func (s *NodeSelector) GetNodePriority(nodeID common.Hash) int {
    r := s.reputationMgr.getReputation(nodeID)
    if r == nil {
        return 0
    }
    
    // 信誉分 >= 9000: 优先级 3 (最高)
    // 信誉分 >= 8000: 优先级 2
    // 信誉分 >= 7000: 优先级 1
    // 信誉分 < 7000: 优先级 0 (最低)
    
    switch {
    case r.ReputationScore >= 9000:
        return 3
    case r.ReputationScore >= 8000:
        return 2
    case r.ReputationScore >= 7000:
        return 1
    default:
        return 0
    }
}
```

##### 3.3.8.7 多维度差异化竞争机制

**核心问题**：如果所有节点都稳定在线，仅靠在线率无法产生差异化竞争，激励机制会触及天花板。

**解决方案**：稳定在线是"入场券"，不是"天花板"。节点必须在多个维度竞争才能获得更高收益。

```
激励模型架构:
┌─────────────────────────────────────────────────────────────────────┐
│                        收益 = 基础收益 + 竞争收益                    │
├─────────────────────────────────────────────────────────────────────┤
│  基础层（入场券）                                                    │
│  ├─ 稳定在线率 >= 95%  ───────────────────────────────────────────┐ │
│  │  满足条件才能参与出块和获得收益                                 │ │
│  └─────────────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────────────┤
│  竞争层（无上限）                                                    │
│  ├─ 服务质量维度 ─────────────────────────────────────────────────┐ │
│  │  响应速度、吞吐量、成功率                                       │ │
│  ├─ 交易量维度 ───────────────────────────────────────────────────┤ │
│  │  处理更多交易 = 更多收入（直接激励）                            │ │
│  ├─ 增值服务维度 ─────────────────────────────────────────────────┤ │
│  │  API 服务、数据索引、优先处理等                                 │ │
│  └─ 历史贡献维度 ─────────────────────────────────────────────────┘ │
│     运营时长、累计出块数、网络贡献                                   │
└─────────────────────────────────────────────────────────────────────┘
```

###### 3.3.8.7.1 服务质量竞争

即使所有节点都稳定在线，服务质量仍可产生差异：

```go
// consensus/sgx/service_quality.go
package sgx

// ServiceQualityMetrics 服务质量指标
type ServiceQualityMetrics struct {
    NodeID              common.Hash
    
    // 响应速度指标
    AvgResponseTimeMs   uint32    // 平均响应时间（毫秒）
    P95ResponseTimeMs   uint32    // P95 响应时间
    P99ResponseTimeMs   uint32    // P99 响应时间
    
    // 吞吐量指标
    TxPerSecond         float64   // 每秒处理交易数
    PeakTxPerSecond     float64   // 峰值吞吐量
    
    // 成功率指标
    SuccessRate         float64   // 交易处理成功率 (0-1)
    ErrorRate           float64   // 错误率
    
    // 可用性指标
    AvailabilityRate    float64   // 可用性 (0-1)
}

// ServiceQualityScorer 服务质量评分器
type ServiceQualityScorer struct {
    config *QualityConfig
}

// QualityConfig 质量评分配置
type QualityConfig struct {
    // 响应时间阈值（毫秒）
    ExcellentResponseMs  uint32  // 优秀: < 50ms
    GoodResponseMs       uint32  // 良好: < 200ms
    AcceptableResponseMs uint32  // 可接受: < 1000ms
    
    // 吞吐量阈值
    HighThroughput       float64 // 高吞吐: > 100 tx/s
    MediumThroughput     float64 // 中吞吐: > 50 tx/s
    
    // 权重配置
    ResponseWeight       uint8   // 响应时间权重，默认 40
    ThroughputWeight     uint8   // 吞吐量权重，默认 30
    SuccessRateWeight    uint8   // 成功率权重，默认 30
}

// CalculateQualityScore 计算服务质量得分 (0-10000)
func (s *ServiceQualityScorer) CalculateQualityScore(m *ServiceQualityMetrics) uint64 {
    cfg := s.config
    
    // 1. 响应时间得分
    var responseScore uint64
    switch {
    case m.AvgResponseTimeMs <= cfg.ExcellentResponseMs:
        responseScore = 10000
    case m.AvgResponseTimeMs <= cfg.GoodResponseMs:
        responseScore = 8000
    case m.AvgResponseTimeMs <= cfg.AcceptableResponseMs:
        responseScore = 6000
    default:
        responseScore = 3000
    }
    
    // 2. 吞吐量得分
    var throughputScore uint64
    switch {
    case m.TxPerSecond >= cfg.HighThroughput:
        throughputScore = 10000
    case m.TxPerSecond >= cfg.MediumThroughput:
        throughputScore = 7000
    default:
        throughputScore = 4000
    }
    
    // 3. 成功率得分
    successScore := uint64(m.SuccessRate * 10000)
    
    // 4. 加权计算
    totalScore := (responseScore * uint64(cfg.ResponseWeight) +
                   throughputScore * uint64(cfg.ThroughputWeight) +
                   successScore * uint64(cfg.SuccessRateWeight)) / 100
    
    return totalScore
}
```

###### 3.3.8.7.2 交易量竞争

处理更多交易直接带来更多收入，这是最直接的激励：

```go
// consensus/sgx/transaction_volume.go
package sgx

// TransactionVolumeTracker 交易量追踪器
type TransactionVolumeTracker struct {
    volumeLog map[common.Hash][]VolumeRecord
    config    *VolumeConfig
}

// VolumeRecord 交易量记录
type VolumeRecord struct {
    NodeID      common.Hash
    Period      uint64    // 统计周期（区块高度）
    TxCount     uint64    // 交易数量
    TotalGas    uint64    // 总 Gas 消耗
    TotalFees   *big.Int  // 总交易费
}

// VolumeConfig 交易量配置
type VolumeConfig struct {
    WindowBlocks    uint64  // 统计窗口（区块数），默认 1000
    BonusThreshold  uint64  // 奖励阈值（交易数），默认 10000
    BonusMultiplier float64 // 奖励倍数，默认 1.5
}

// CalculateVolumeBonus 计算交易量奖励
func (t *TransactionVolumeTracker) CalculateVolumeBonus(nodeID common.Hash) *big.Int {
    records := t.getRecentRecords(nodeID)
    
    var totalTxCount uint64
    totalFees := big.NewInt(0)
    
    for _, record := range records {
        totalTxCount += record.TxCount
        totalFees.Add(totalFees, record.TotalFees)
    }
    
    // 基础收益 = 总交易费
    bonus := new(big.Int).Set(totalFees)
    
    // 如果超过阈值，获得额外奖励
    if totalTxCount >= t.config.BonusThreshold {
        // 额外奖励 = 基础收益 * (倍数 - 1)
        extraBonus := new(big.Int).Mul(bonus, big.NewInt(int64((t.config.BonusMultiplier-1)*100)))
        extraBonus.Div(extraBonus, big.NewInt(100))
        bonus.Add(bonus, extraBonus)
    }
    
    return bonus
}

// GetMarketShare 获取节点市场份额
func (t *TransactionVolumeTracker) GetMarketShare(nodeID common.Hash) float64 {
    nodeVolume := t.getTotalVolume(nodeID)
    networkVolume := t.getNetworkTotalVolume()
    
    if networkVolume == 0 {
        return 0
    }
    
    return float64(nodeVolume) / float64(networkVolume)
}
```

**交易量激励的优势**：
- 无上限：处理越多交易，收入越高
- 直接激励：节点有动力吸引用户、提供更好服务
- 市场驱动：用户自然选择服务更好的节点

###### 3.3.8.7.3 增值服务竞争

节点可以提供额外服务获得收入：

```go
// consensus/sgx/value_added_services.go
package sgx

// ValueAddedService 增值服务定义
type ValueAddedService struct {
    ServiceID   string    // 服务标识
    Name        string    // 服务名称
    Description string    // 服务描述
    PricePerUse *big.Int  // 每次使用价格
    IsEnabled   bool      // 是否启用
}

// 预定义增值服务
var PredefinedServices = []ValueAddedService{
    {
        ServiceID:   "priority_tx",
        Name:        "优先交易处理",
        Description: "交易优先进入下一个区块",
        PricePerUse: big.NewInt(1000000000), // 1 Gwei
    },
    {
        ServiceID:   "fast_confirm",
        Name:        "快速确认",
        Description: "交易确认后立即通知",
        PricePerUse: big.NewInt(500000000), // 0.5 Gwei
    },
    {
        ServiceID:   "tx_history_api",
        Name:        "交易历史 API",
        Description: "查询历史交易记录",
        PricePerUse: big.NewInt(100000000), // 0.1 Gwei
    },
    {
        ServiceID:   "event_subscription",
        Name:        "事件订阅",
        Description: "订阅合约事件通知",
        PricePerUse: big.NewInt(200000000), // 0.2 Gwei
    },
    {
        ServiceID:   "data_indexing",
        Name:        "数据索引服务",
        Description: "提供高效的数据查询索引",
        PricePerUse: big.NewInt(300000000), // 0.3 Gwei
    },
}

// ValueAddedServiceManager 增值服务管理器
type ValueAddedServiceManager struct {
    services    map[string]*ValueAddedService
    usageLog    map[common.Hash][]ServiceUsageRecord
}

// ServiceUsageRecord 服务使用记录
type ServiceUsageRecord struct {
    NodeID      common.Hash
    ServiceID   string
    UserAddress common.Address
    Timestamp   uint64
    Fee         *big.Int
}

// CalculateServiceRevenue 计算增值服务收入
func (m *ValueAddedServiceManager) CalculateServiceRevenue(nodeID common.Hash, period time.Duration) *big.Int {
    records := m.getRecentUsage(nodeID, period)
    
    totalRevenue := big.NewInt(0)
    for _, record := range records {
        totalRevenue.Add(totalRevenue, record.Fee)
    }
    
    return totalRevenue
}
```

###### 3.3.8.7.4 历史贡献竞争

长期稳定运营的节点获得额外奖励：

```go
// consensus/sgx/historical_contribution.go
package sgx

// HistoricalContribution 历史贡献记录
type HistoricalContribution struct {
    NodeID              common.Hash
    FirstActiveBlock    uint64        // 首次活跃区块
    TotalBlocksProduced uint64        // 累计出块数
    TotalTxProcessed    uint64        // 累计处理交易数
    TotalUptime         time.Duration // 累计在线时长
    ConsecutiveDays     uint64        // 连续在线天数
    NetworkContribution uint64        // 网络贡献分（引入新用户等）
}

// HistoricalContributionScorer 历史贡献评分器
type HistoricalContributionScorer struct {
    config *ContributionConfig
}

// ContributionConfig 贡献评分配置
type ContributionConfig struct {
    // 运营时长奖励
    DaysForBronze   uint64  // 铜牌: 30 天
    DaysForSilver   uint64  // 银牌: 90 天
    DaysForGold     uint64  // 金牌: 365 天
    DaysForDiamond  uint64  // 钻石: 1000 天
    
    // 奖励倍数
    BronzeMultiplier   float64 // 1.1x
    SilverMultiplier   float64 // 1.2x
    GoldMultiplier     float64 // 1.5x
    DiamondMultiplier  float64 // 2.0x
}

// CalculateContributionMultiplier 计算历史贡献倍数
func (s *HistoricalContributionScorer) CalculateContributionMultiplier(c *HistoricalContribution) float64 {
    cfg := s.config
    
    // 根据连续在线天数确定等级
    switch {
    case c.ConsecutiveDays >= cfg.DaysForDiamond:
        return cfg.DiamondMultiplier // 2.0x
    case c.ConsecutiveDays >= cfg.DaysForGold:
        return cfg.GoldMultiplier // 1.5x
    case c.ConsecutiveDays >= cfg.DaysForSilver:
        return cfg.SilverMultiplier // 1.2x
    case c.ConsecutiveDays >= cfg.DaysForBronze:
        return cfg.BronzeMultiplier // 1.1x
    default:
        return 1.0 // 无奖励
    }
}

// GetContributionTier 获取贡献等级
func (s *HistoricalContributionScorer) GetContributionTier(c *HistoricalContribution) string {
    cfg := s.config
    
    switch {
    case c.ConsecutiveDays >= cfg.DaysForDiamond:
        return "Diamond"
    case c.ConsecutiveDays >= cfg.DaysForGold:
        return "Gold"
    case c.ConsecutiveDays >= cfg.DaysForSilver:
        return "Silver"
    case c.ConsecutiveDays >= cfg.DaysForBronze:
        return "Bronze"
    default:
        return "None"
    }
}
```

###### 3.3.8.7.5 综合收益计算

```go
// consensus/sgx/comprehensive_reward.go
package sgx

// ComprehensiveRewardCalculator 综合收益计算器
type ComprehensiveRewardCalculator struct {
    uptimeCalc       *UptimeCalculator
    qualityScorer    *ServiceQualityScorer
    volumeTracker    *TransactionVolumeTracker
    serviceManager   *ValueAddedServiceManager
    contributionScorer *HistoricalContributionScorer
    config           *RewardConfig
}

// RewardConfig 收益配置
type RewardConfig struct {
    MinUptimeForReward float64 // 最低在线率要求，默认 0.95 (95%)
}

// CalculateTotalReward 计算节点总收益
func (c *ComprehensiveRewardCalculator) CalculateTotalReward(
    nodeID common.Hash,
    period time.Duration,
) (*TotalReward, error) {
    
    // 1. 检查是否满足基础条件（入场券）
    uptimeScore := c.uptimeCalc.CalculateComprehensiveUptime(nodeID)
    if float64(uptimeScore)/10000 < c.config.MinUptimeForReward {
        return &TotalReward{
            NodeID:     nodeID,
            IsEligible: false,
            Reason:     "在线率不足 95%，不满足参与条件",
        }, nil
    }
    
    // 2. 计算交易费收入（基础收益）
    txFeeRevenue := c.volumeTracker.CalculateVolumeBonus(nodeID)
    
    // 3. 计算增值服务收入
    serviceRevenue := c.serviceManager.CalculateServiceRevenue(nodeID, period)
    
    // 4. 计算服务质量奖励
    qualityMetrics := c.getQualityMetrics(nodeID)
    qualityScore := c.qualityScorer.CalculateQualityScore(qualityMetrics)
    qualityBonus := c.calculateQualityBonus(txFeeRevenue, qualityScore)
    
    // 5. 计算历史贡献倍数
    contribution := c.getHistoricalContribution(nodeID)
    contributionMultiplier := c.contributionScorer.CalculateContributionMultiplier(contribution)
    
    // 6. 计算总收益
    // 总收益 = (交易费 + 增值服务 + 质量奖励) * 历史贡献倍数
    baseReward := new(big.Int).Add(txFeeRevenue, serviceRevenue)
    baseReward.Add(baseReward, qualityBonus)
    
    totalReward := new(big.Int).Mul(baseReward, big.NewInt(int64(contributionMultiplier*100)))
    totalReward.Div(totalReward, big.NewInt(100))
    
    return &TotalReward{
        NodeID:                 nodeID,
        IsEligible:             true,
        UptimeScore:            uptimeScore,
        TxFeeRevenue:           txFeeRevenue,
        ServiceRevenue:         serviceRevenue,
        QualityBonus:           qualityBonus,
        ContributionMultiplier: contributionMultiplier,
        ContributionTier:       c.contributionScorer.GetContributionTier(contribution),
        TotalReward:            totalReward,
    }, nil
}

// TotalReward 总收益结构
type TotalReward struct {
    NodeID                 common.Hash
    IsEligible             bool
    Reason                 string
    UptimeScore            uint64
    TxFeeRevenue           *big.Int
    ServiceRevenue         *big.Int
    QualityBonus           *big.Int
    ContributionMultiplier float64
    ContributionTier       string
    TotalReward            *big.Int
}
```

###### 3.3.8.7.6 激励机制总结

| 维度 | 类型 | 上限 | 激励效果 |
|------|------|------|----------|
| 稳定在线 | 入场券 | 95% 阈值 | 必须达到才能参与 |
| 服务质量 | 竞争 | 无上限 | 更快响应 = 更高奖励 |
| 交易量 | 竞争 | 无上限 | 更多交易 = 更多收入 |
| 增值服务 | 竞争 | 无上限 | 更多服务 = 更多收入 |
| 历史贡献 | 倍数 | 2.0x | 长期运营 = 收益翻倍 |

**激励效果示例**：

```
场景：两个节点都 100% 在线

节点 A（新节点）:
- 在线率: 100% ✓ (满足入场条件)
- 服务质量: 一般 (响应 500ms)
- 交易量: 1000 tx/天
- 增值服务: 无
- 历史贡献: 10 天 (无等级)
- 收益倍数: 1.0x
- 日收益: 100 X

节点 B（老节点）:
- 在线率: 100% ✓ (满足入场条件)
- 服务质量: 优秀 (响应 50ms) → +20% 质量奖励
- 交易量: 5000 tx/天 → 5x 交易费
- 增值服务: 3 项 → +30% 服务收入
- 历史贡献: 400 天 (Gold) → 1.5x 倍数
- 日收益: (500 + 100 + 30) * 1.5 = 945 X

差距: 节点 B 收益是节点 A 的 9.45 倍
```

**这种设计确保**：
1. 稳定在线是必要条件，但不是充分条件
2. 即使所有节点都稳定在线，仍有多个维度可以竞争
3. 长期运营的节点有明显优势，激励节点持续稳定运营
4. 新节点可以通过提高服务质量和交易量快速追赶

##### 3.3.8.8 激励机制完整总结

| 机制 | 目的 | 效果 |
|------|------|------|
| 信誉系统 | 跟踪节点稳定性 | 量化节点表现 |
| 交易费加权 | 奖励稳定节点 | 高信誉节点收入更高 |
| 惩罚机制 | 惩罚不稳定节点 | 降低不稳定节点收益 |
| 优先级排序 | 引导用户选择 | 稳定节点获得更多交易 |
| 暂时排除 | 保护网络质量 | 严重不稳定节点无法出块 |
| 服务质量竞争 | 激励提升服务 | 更好服务 = 更高收益 |
| 交易量竞争 | 激励吸引用户 | 更多交易 = 更多收入 |
| 增值服务 | 激励创新 | 提供更多价值 = 更多收入 |
| 历史贡献 | 激励长期运营 | 长期稳定 = 收益倍增 |

**激励效果**：

```
节点稳定在线 → 信誉分提高 → 交易费加权提高 → 收入增加
                         → 优先级提高 → 获得更多交易 → 收入增加

节点频繁离线 → 信誉分降低 → 交易费加权降低 → 收入减少
                         → 优先级降低 → 获得更少交易 → 收入减少
                         → 惩罚累积 → 暂时排除 → 无收入

所有节点都稳定在线时:
节点 A (服务好) → 质量奖励高 → 用户选择多 → 交易量大 → 收入高
节点 B (服务差) → 质量奖励低 → 用户选择少 → 交易量小 → 收入低
```

```go
// 交易费配置
type FeeConfig struct {
    // 基础费用（可以为0）
    BaseFee *big.Int
    
    // 每 Gas 单位费用
    GasPrice *big.Int
    
    // 费用分配：100% 给出块节点（按信誉加权）
    ProducerShare uint8 // 100
    
    // 信誉加权开关
    UseReputationWeighting bool
    
    // 多维度竞争开关
    UseMultiDimensionalCompetition bool
}

// 默认配置：极低费用 + 信誉加权 + 多维度竞争
var DefaultFeeConfig = FeeConfig{
    BaseFee:                        big.NewInt(0),           // 无基础费
    GasPrice:                       big.NewInt(1),           // 1 wei per gas
    ProducerShare:                  100,
    UseReputationWeighting:         true,                    // 启用信誉加权
    UseMultiDimensionalCompetition: true,                    // 启用多维度竞争
}
```

#### 3.3.9 存储优化

按需出块显著减少存储需求：

```
假设场景：每天 10,000 笔交易

以太坊 PoS (12秒出块):
- 每天区块数: 86400 / 12 = 7,200 块
- 大量空块或低交易量块

X Chain PoA-SGX (按需出块):
- 假设每块平均 100 笔交易
- 每天区块数: 10,000 / 100 = 100 块
- 存储减少: 7,200 / 100 = 72 倍
```

#### 3.3.10 配置参数

```go
// consensus/sgx/config.go
type SGXConsensusConfig struct {
    // 出块配置
    MaxTxPerBlock   int           `json:"maxTxPerBlock"`   // 每块最大交易数，默认 1000
    MinTxForBlock   int           `json:"minTxForBlock"`   // 触发出块最小交易数，默认 1
    MaxWaitTime     time.Duration `json:"maxWaitTime"`     // 最大等待时间，默认 0（无限制）
    
    // 费用配置
    BaseFee         *big.Int      `json:"baseFee"`         // 基础费用，默认 0
    MinGasPrice     *big.Int      `json:"minGasPrice"`     // 最低 Gas 价格，默认 1 wei
    
    // SGX 配置
    MREnclaveWhitelist []string   `json:"mrEnclaveWhitelist"` // 允许的 MRENCLAVE 列表
    MRSignerWhitelist  []string   `json:"mrSignerWhitelist"`  // 允许的 MRSIGNER 列表
}

// 默认配置
var DefaultSGXConsensusConfig = SGXConsensusConfig{
    MaxTxPerBlock:   1000,
    MinTxForBlock:   1,
    MaxWaitTime:     0,
    BaseFee:         big.NewInt(0),
    MinGasPrice:     big.NewInt(1),
}
```

### 3.4 区块验证流程

```go
func (s *SGXConsensus) VerifyHeader(chain ChainHeaderReader, header *types.Header) error {
    // 1. 验证基本区块头字段
    if err := s.verifyBasicHeader(header); err != nil {
        return err
    }
    
    // 2. 解析 Extra 字段中的 SGX 证明数据
    sgxData, err := s.parseSGXExtra(header.Extra)
    if err != nil {
        return err
    }
    
    // 3. 验证 SGX Quote
    if err := s.verifyQuote(sgxData.SGXQuote); err != nil {
        return err
    }
    
    // 4. 验证 MRENCLAVE 是否在白名单中
    if !s.isValidMREnclave(sgxData.MRENCLAVE) {
        return ErrInvalidMREnclave
    }
    
    // 5. 验证区块签名
    return s.verifyBlockSignature(header, sgxData)
}
```

## 4. 预编译合约详细设计

### 4.1 密钥管理架构

```
+------------------------------------------------------------------+
|                        密钥管理系统                               |
|  +------------------------------------------------------------+  |
|  |                     权限控制层                              |  |
|  |  - 密钥所有权验证 (msg.sender == keyOwner)                 |  |
|  |  - 操作权限检查                                            |  |
|  +------------------------------------------------------------+  |
|  |                     密钥操作层                              |  |
|  |  +------------+  +------------+  +------------+            |  |
|  |  | 签名/验签  |  | ECDH       |  | 加密/解密  |            |  |
|  |  +------------+  +------------+  +------------+            |  |
|  +------------------------------------------------------------+  |
|  |                     密钥存储层                              |  |
|  |  +------------------------------------------------------+  |  |
|  |  |              Gramine 加密分区                         |  |  |
|  |  |  /app/wallet/keys/{keyId}/                           |  |  |
|  |  |    - private.key (私钥，永不离开 enclave)            |  |  |
|  |  |    - public.key (公钥)                               |  |  |
|  |  |    - metadata.json (所有者、曲线类型等)              |  |  |
|  |  +------------------------------------------------------+  |  |
|  +------------------------------------------------------------+  |
+------------------------------------------------------------------+
```

### 4.2 支持的椭圆曲线

| 曲线名称 | 标识符 | 用途 |
|----------|--------|------|
| secp256k1 | 0x01 | 以太坊兼容签名 |
| secp256r1 (P-256) | 0x02 | TLS、通用签名 |
| secp384r1 (P-384) | 0x03 | 高安全性签名 |
| ed25519 | 0x04 | 高性能签名 |
| x25519 | 0x05 | ECDH 密钥交换 |

### 4.3 预编译合约接口定义

#### 4.3.1 SGX_KEY_CREATE (0x8000)

创建新的密钥对，私钥存储在加密分区。

**输入格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0      | 曲线类型 (1=secp256k1, 2=P-256, 3=P-384, 4=ed25519, 5=x25519) |
+--------+--------+
```

**输出格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0-31   | keyId (密钥标识符，sha256(owner || nonce)) |
+--------+--------+
```

**Gas 消耗：** 50000

**实现：**
```go
// core/vm/contracts_sgx.go
type sgxKeyCreate struct{}

func (c *sgxKeyCreate) RequiredGas(input []byte) uint64 {
    return 50000
}

func (c *sgxKeyCreate) Run(input []byte, caller common.Address, evm *EVM) ([]byte, error) {
    if len(input) < 1 {
        return nil, ErrInvalidInput
    }
    
    curveType := input[0]
    
    // 生成密钥对
    keyPair, err := generateKeyPair(curveType)
    if err != nil {
        return nil, err
    }
    
    // 计算 keyId
    nonce := evm.StateDB.GetNonce(caller)
    keyId := crypto.Keccak256Hash(caller.Bytes(), common.BigToHash(big.NewInt(int64(nonce))).Bytes())
    
    // 存储密钥到加密分区
    if err := storeKeyToEncryptedPartition(keyId, keyPair, caller); err != nil {
        return nil, err
    }
    
    // 记录密钥所有权到状态
    evm.StateDB.SetKeyOwner(keyId, caller)
    
    return keyId.Bytes(), nil
}
```

#### 4.3.2 SGX_KEY_GET_PUBLIC (0x8001)

获取指定密钥的公钥。

**输入格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0-31   | keyId  |
+--------+--------+
```

**输出格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0      | 曲线类型 |
| 1-N    | 公钥数据 (压缩或非压缩格式) |
+--------+--------+
```

**Gas 消耗：** 3000

#### 4.3.3 SGX_SIGN (0x8002)

使用私钥签名消息。

**输入格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0-31   | keyId  |
| 32-63  | 消息哈希 (32 字节) |
+--------+--------+
```

**输出格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0-N    | 签名数据 (格式取决于曲线类型) |
+--------+--------+
```

**Gas 消耗：** 10000

**权限检查：**
```go
func (c *sgxSign) Run(input []byte, caller common.Address, evm *EVM) ([]byte, error) {
    keyId := common.BytesToHash(input[0:32])
    
    // 权限检查：只有密钥所有者可以签名
    owner := evm.StateDB.GetKeyOwner(keyId)
    if owner != caller {
        return nil, ErrNotKeyOwner
    }
    
    // 从加密分区加载私钥
    privateKey, err := loadPrivateKeyFromEncryptedPartition(keyId)
    if err != nil {
        return nil, err
    }
    
    // 签名
    messageHash := input[32:64]
    signature, err := sign(privateKey, messageHash)
    if err != nil {
        return nil, err
    }
    
    return signature, nil
}
```

#### 4.3.4 SGX_VERIFY (0x8003)

验证签名。

**输入格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0      | 曲线类型 |
| 1-N    | 公钥数据 |
| N+1-N+32 | 消息哈希 |
| N+33-M | 签名数据 |
+--------+--------+
```

**输出格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0-31   | 验证结果 (1=成功, 0=失败) |
+--------+--------+
```

**Gas 消耗：** 5000

#### 4.3.5 SGX_ECDH (0x8004)

执行 ECDH 密钥交换，派生的共享秘密存储在加密分区。

**输入格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0-31   | 本方私钥 keyId |
| 32     | 对方公钥曲线类型 |
| 33-N   | 对方公钥数据 |
+--------+--------+
```

**输出格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0-31   | 派生秘密的 keyId (可用于后续加密操作) |
+--------+--------+
```

**Gas 消耗：** 20000

**实现：**
```go
func (c *sgxECDH) Run(input []byte, caller common.Address, evm *EVM) ([]byte, error) {
    privateKeyId := common.BytesToHash(input[0:32])
    
    // 权限检查
    owner := evm.StateDB.GetKeyOwner(privateKeyId)
    if owner != caller {
        return nil, ErrNotKeyOwner
    }
    
    // 加载私钥
    privateKey, err := loadPrivateKeyFromEncryptedPartition(privateKeyId)
    if err != nil {
        return nil, err
    }
    
    // 解析对方公钥
    peerPublicKey, err := parsePublicKey(input[32:])
    if err != nil {
        return nil, err
    }
    
    // 执行 ECDH
    sharedSecret, err := ecdh(privateKey, peerPublicKey)
    if err != nil {
        return nil, err
    }
    
    // 派生秘密也遵循密钥管理逻辑，存储到加密分区
    derivedKeyId := crypto.Keccak256Hash(privateKeyId.Bytes(), peerPublicKey)
    if err := storeDerivedSecretToEncryptedPartition(derivedKeyId, sharedSecret, caller); err != nil {
        return nil, err
    }
    
    // 记录派生秘密所有权
    evm.StateDB.SetKeyOwner(derivedKeyId, caller)
    
    return derivedKeyId.Bytes(), nil
}
```

#### 4.3.6 SGX_RANDOM (0x8005)

获取硬件真随机数。

**输入格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0-31   | 请求的随机数长度 (最大 32 字节) |
+--------+--------+
```

**输出格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0-N    | 随机数据 |
+--------+--------+
```

**Gas 消耗：** 1000 + 100 * 字节数

**实现：**
```go
func (c *sgxRandom) Run(input []byte) ([]byte, error) {
    length := new(big.Int).SetBytes(input).Uint64()
    if length > 32 {
        length = 32
    }
    
    // 使用 SGX RDRAND 指令获取硬件随机数
    randomBytes := make([]byte, length)
    if err := sgxRdrand(randomBytes); err != nil {
        return nil, err
    }
    
    return common.LeftPadBytes(randomBytes, 32), nil
}
```

#### 4.3.7 SGX_ENCRYPT (0x8006)

使用对称密钥加密数据。

**输入格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0-31   | keyId (对称密钥或 ECDH 派生密钥) |
| 32-N   | 明文数据 |
+--------+--------+
```

**输出格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0-11   | Nonce (12 字节) |
| 12-N   | 密文 + Tag |
+--------+--------+
```

**Gas 消耗：** 5000 + 10 * 数据长度

#### 4.3.8 SGX_DECRYPT (0x8007)

使用对称密钥解密数据。

**输入格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0-31   | keyId |
| 32-43  | Nonce |
| 44-N   | 密文 + Tag |
+--------+--------+
```

**输出格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0-N    | 明文数据 |
+--------+--------+
```

**Gas 消耗：** 5000 + 10 * 数据长度

#### 4.3.9 SGX_KEY_DERIVE (0x8008)

从现有密钥派生新密钥。

**输入格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0-31   | 源 keyId |
| 32-63  | 派生路径/盐值 |
+--------+--------+
```

**输出格式：**
```
+--------+--------+
| 字节   | 描述   |
+--------+--------+
| 0-31   | 新 keyId |
+--------+--------+
```

**Gas 消耗：** 10000

### 4.4 权限管理机制

#### 4.4.1 密钥所有权

每个密钥都有唯一的所有者（创建者的地址）：

```go
// 状态存储结构
type KeyMetadata struct {
    Owner     common.Address  // 密钥所有者
    CurveType uint8          // 曲线类型
    CreatedAt uint64         // 创建时间（区块号）
    KeyType   uint8          // 密钥类型 (0=非对称, 1=对称, 2=派生)
    ParentKey common.Hash    // 父密钥 (用于派生密钥)
}
```

#### 4.4.2 操作权限

| 操作 | 权限要求 |
|------|----------|
| 获取公钥 | 任何人 |
| 签名 | 仅所有者 |
| ECDH | 仅所有者 |
| 加密 | 仅所有者 |
| 解密 | 仅所有者 |
| 派生密钥 | 仅所有者 |

#### 4.4.3 派生秘密管理

ECDH 等操作产生的派生秘密也遵循相同的权限管理逻辑：

```go
// 派生秘密继承原始密钥的所有权
func (c *sgxECDH) Run(input []byte, caller common.Address, evm *EVM) ([]byte, error) {
    // ... ECDH 计算 ...
    
    // 派生秘密的所有者与原始私钥所有者相同
    evm.StateDB.SetKeyOwner(derivedKeyId, caller)
    evm.StateDB.SetKeyMetadata(derivedKeyId, KeyMetadata{
        Owner:     caller,
        KeyType:   2, // 派生密钥
        ParentKey: privateKeyId,
    })
    
    return derivedKeyId.Bytes(), nil
}
```

#### 4.4.4 身份验证机制

访问秘密数据**必须通过签名验证身份**。这是通过以太坊标准的交易签名机制实现的：

```
+------------------+                    +------------------+
|   用户钱包       |                    |   X Chain 节点   |
+------------------+                    +------------------+
        |                                       |
        | 1. 构造交易 (调用 SGX_SIGN)           |
        |                                       |
        | 2. 用以太坊私钥签名交易               |
        |   signature = sign(tx, privateKey)    |
        |                                       |
        | 3. 提交签名交易                       |
        |-------------------------------------->|
        |                                       |
        |                    4. EVM 验证交易签名 |
        |                    sender = ecrecover(tx, sig)
        |                                       |
        |                    5. 提取 msg.sender |
        |                                       |
        |                    6. 预编译合约检查权限
        |                    if msg.sender != keyOwner:
        |                        revert("Not key owner")
        |                                       |
        |                    7. 权限验证通过    |
        |                    执行签名操作       |
        |                                       |
        | 8. 返回签名结果                       |
        |<--------------------------------------|
```

**身份验证的安全保证：**

| 攻击场景 | 防护机制 |
|----------|----------|
| 未签名交易 | EVM 拒绝执行，交易无效 |
| 签名错误 | ecrecover 恢复出错误地址，权限检查失败 |
| 重放攻击 | 交易 nonce 机制防止重放 |
| 伪造 msg.sender | 不可能，msg.sender 由签名密码学保证 |
| 知道 keyId 但无签名 | 无法提交有效交易，无法访问秘密 |

**实现代码：**

```go
// core/vm/contracts_sgx.go

// 通用权限检查函数
func checkKeyOwnership(evm *EVM, keyId common.Hash, caller common.Address) error {
    // caller 是通过交易签名验证后提取的 msg.sender
    // 这个值由 EVM 保证其真实性，无法伪造
    
    owner := evm.StateDB.GetKeyOwner(keyId)
    if owner == (common.Address{}) {
        return ErrKeyNotFound
    }
    if owner != caller {
        return ErrNotKeyOwner
    }
    return nil
}

// 签名操作示例
func (c *sgxSign) Run(input []byte, caller common.Address, evm *EVM) ([]byte, error) {
    keyId := common.BytesToHash(input[0:32])
    
    // 权限检查：caller 必须是密钥所有者
    // caller 的身份已通过交易签名验证
    if err := checkKeyOwnership(evm, keyId, caller); err != nil {
        return nil, err
    }
    
    // 身份验证通过，执行签名操作
    // ...
}
```

**合约调用场景：**

当智能合约调用预编译合约时，`msg.sender` 是调用合约的地址，而非原始交易发起者（EOA）：

```
EOA (0x1234...) --调用--> 合约 A (0xAAAA...) --调用--> SGX_SIGN
                                                        |
                                              msg.sender = 0xAAAA...
                                              (不是 0x1234...)
```

这意味着：
- 如果合约 A 是密钥所有者，合约 A 可以使用该密钥
- 如果 EOA 是密钥所有者，合约 A 无法代替 EOA 使用该密钥
- 这提供了细粒度的权限控制，防止未授权的合约访问用户密钥

**无签名 = 无访问：**

```
攻击者知道 keyId = 0xABCD...
攻击者想调用 SGX_SIGN(keyId, msgHash)

但是：
1. 攻击者没有密钥所有者的以太坊私钥
2. 攻击者无法签名有效交易
3. 即使构造交易，EVM 也会拒绝（签名无效）
4. 即使通过某种方式提交，msg.sender 也不会是所有者
5. 权限检查失败，操作被拒绝

结论：没有所有者的签名，绝对无法访问秘密数据
```

## 5. 数据存储与同步

### 5.1 存储架构

```
/app/wallet/                          # Gramine 加密分区根目录
├── chaindata/                        # 区块链数据
│   ├── ancient/                      # 历史数据
│   └── leveldb/                      # 当前状态
├── keys/                             # 密钥存储
│   └── {keyId}/
│       ├── private.key               # 私钥 (永不离开 enclave)
│       ├── public.key                # 公钥
│       └── metadata.json             # 元数据
├── derived/                          # 派生秘密
│   └── {derivedKeyId}/
│       └── secret.key
└── node/                             # 节点配置
    ├── nodekey                       # 节点私钥
    └── attestation/                  # 证明数据缓存
```

### 5.2 数据同步机制

#### 5.2.1 节点发现

与以太坊保持一致，使用 discv5 协议进行节点发现：

```go
// 节点发现时附加 SGX 证明信息
type SGXNodeRecord struct {
    *enode.Node
    MRENCLAVE []byte  // 代码度量值
    MRSIGNER  []byte  // 签名者度量值
    QuoteHash []byte  // 最新 Quote 哈希
}
```

#### 5.2.2 数据同步流程

```
+-------------+                    +-------------+
|   节点 A    |                    |   节点 B    |
+-------------+                    +-------------+
      |                                  |
      |  1. RA-TLS 握手                  |
      |<-------------------------------->|
      |  (双向 SGX 远程证明)             |
      |                                  |
      |  2. 交换区块头                   |
      |<-------------------------------->|
      |                                  |
      |  3. 验证数据一致性               |
      |  (比较状态根)                    |
      |                                  |
      |  4. 同步缺失区块                 |
      |<-------------------------------->|
      |                                  |
      |  5. 同步加密分区数据             |
      |  (密钥元数据，不含私钥)          |
      |<-------------------------------->|
      |                                  |
```

#### 5.2.3 加密分区数据同步（秘密数据同步）

加密分区中的**所有数据**（包括秘密数据）都需要在节点间同步，以保持网络一致性。通过**度量值检测**确保秘密数据只会同步到运行相同可信代码的节点，防止泄露。

**同步的数据：**
- 密钥元数据（所有者、曲线类型、创建时间）
- 公钥数据
- **私钥数据**（通过 RA-TLS 安全通道传输）
- **派生秘密**（ECDH 结果等）
- 密钥所有权记录

**秘密数据同步的安全保证：**

```
+------------------+                    +------------------+
|   源节点 A       |                    |   目标节点 B     |
| (已有秘密数据)   |                    | (需要同步数据)   |
+------------------+                    +------------------+
        |                                       |
        |  1. RA-TLS 握手开始                   |
        |<------------------------------------->|
        |                                       |
        |  2. 双向 SGX 远程证明                 |
        |  A 验证 B 的度量值:                   |
        |  - MRENCLAVE 是否在白名单?            |
        |  - MRSIGNER 是否在白名单?             |
        |  - TCB 状态是否可接受?                |
        |                                       |
        |  B 验证 A 的度量值:                   |
        |  - MRENCLAVE 是否在白名单?            |
        |  - MRSIGNER 是否在白名单?             |
        |  - TCB 状态是否可接受?                |
        |                                       |
        |  3. 双向验证通过                      |
        |  (确认双方都运行相同的可信代码)       |
        |<------------------------------------->|
        |                                       |
        |  4. A 解封本地加密分区数据            |
        |  (在 enclave 内部解密)                |
        |                                       |
        |  5. 通过 RA-TLS 通道传输秘密数据      |
        |  (传输过程中 TLS 加密保护)            |
        |-------------------------------------->|
        |                                       |
        |                    6. B 接收秘密数据  |
        |                    在 enclave 内部    |
        |                                       |
        |                    7. B 重新封装数据  |
        |                    用 B 的 seal key   |
        |                    存入加密分区       |
        |                                       |
```

**度量值检测防止泄露：**

| 攻击场景 | 防护机制 |
|----------|----------|
| 恶意节点伪装 | RA-TLS 验证 SGX Quote，无法伪造 MRENCLAVE |
| 修改过的代码 | MRENCLAVE 不匹配，拒绝同步 |
| 中间人攻击 | RA-TLS 端到端加密，无法窃听 |
| 重放攻击 | TLS 会话密钥唯一，Quote 包含时间戳 |
| 非 SGX 节点 | 无法生成有效 SGX Quote，验证失败 |

**实现代码：**

```go
// internal/sgx/secret_sync.go
package sgx

// SecretSyncManager 管理秘密数据的节点间同步
type SecretSyncManager struct {
    ratls       *RATLSTransport
    keyStore    *EncryptedKeyStore
    whitelist   *MeasurementWhitelist
}

// SyncSecretsFromPeer 从对等节点同步秘密数据
func (m *SecretSyncManager) SyncSecretsFromPeer(ctx context.Context, peer *Peer) error {
    // 1. 建立 RA-TLS 连接（双向验证度量值）
    conn, err := m.ratls.Connect(peer.Address)
    if err != nil {
        return fmt.Errorf("RA-TLS connection failed: %w", err)
    }
    defer conn.Close()
    
    // 2. 验证对方度量值是否在白名单中
    peerQuote := conn.PeerQuote()
    if !m.whitelist.IsAllowed(peerQuote.MRENCLAVE, peerQuote.MRSIGNER) {
        return ErrPeerNotInWhitelist
    }
    
    // 3. 请求秘密数据列表
    keyList, err := m.requestKeyList(conn)
    if err != nil {
        return err
    }
    
    // 4. 同步每个密钥（包括私钥）
    for _, keyId := range keyList {
        // 检查本地是否已有
        if m.keyStore.Exists(keyId) {
            continue
        }
        
        // 请求完整密钥数据（包括私钥）
        keyData, err := m.requestKeyData(conn, keyId)
        if err != nil {
            return fmt.Errorf("failed to sync key %s: %w", keyId.Hex(), err)
        }
        
        // 存储到本地加密分区（自动用本地 seal key 重新封装）
        if err := m.keyStore.Store(keyId, keyData); err != nil {
            return fmt.Errorf("failed to store key %s: %w", keyId.Hex(), err)
        }
    }
    
    return nil
}

// ServeSecretSync 响应其他节点的秘密数据同步请求
func (m *SecretSyncManager) ServeSecretSync(conn *RATLSConn) error {
    // 1. 验证请求方度量值
    peerQuote := conn.PeerQuote()
    if !m.whitelist.IsAllowed(peerQuote.MRENCLAVE, peerQuote.MRSIGNER) {
        return ErrPeerNotInWhitelist
    }
    
    // 2. 只有度量值验证通过，才提供秘密数据
    // 这确保秘密数据只会发送给运行相同可信代码的节点
    
    for {
        req, err := conn.ReadRequest()
        if err != nil {
            return err
        }
        
        switch req.Type {
        case RequestKeyList:
            keys := m.keyStore.ListKeys()
            conn.WriteResponse(keys)
            
        case RequestKeyData:
            keyId := common.BytesToHash(req.Data)
            // 从加密分区读取（在 enclave 内解密）
            keyData, err := m.keyStore.Load(keyId)
            if err != nil {
                conn.WriteError(err)
                continue
            }
            // 通过 RA-TLS 通道发送（传输中加密）
            conn.WriteResponse(keyData)
        }
    }
}
```

**关键安全原则：**

1. **度量值验证是前提**：在传输任何秘密数据之前，必须先通过 RA-TLS 验证对方的 MRENCLAVE/MRSIGNER
2. **只信任相同代码**：只有运行完全相同代码（相同 MRENCLAVE）的节点才能接收秘密数据
3. **端到端加密**：秘密数据在 enclave 内解密，通过 TLS 传输，在目标 enclave 内重新封装
4. **无明文暴露**：秘密数据在整个同步过程中从不以明文形式暴露给主机操作系统

```
秘密数据生命周期：

源节点 enclave          RA-TLS 通道           目标节点 enclave
[seal key A 加密] --解密--> [明文] --TLS加密--> [明文] --加密--> [seal key B 加密]
     |                        |                   |                    |
     |                        |                   |                    |
  存储在磁盘              仅在 enclave 内       仅在 enclave 内      存储在磁盘
  (加密状态)              (受 SGX 保护)        (受 SGX 保护)        (加密状态)
```

### 5.3 数据一致性验证

```go
// 验证两个节点是否属于同一网络
func (s *SGXConsensus) VerifyNetworkConsistency(peer *Peer) error {
    // 1. 比较创世区块哈希
    if peer.GenesisHash != s.genesisHash {
        return ErrDifferentGenesis
    }
    
    // 2. 比较最新区块状态根
    localHead := s.chain.CurrentHeader()
    peerHead := peer.Head()
    
    if localHead.Number.Cmp(peerHead.Number) == 0 {
        if localHead.Root != peerHead.Root {
            return ErrHardFork // 数据不一致，视为硬分叉
        }
    }
    
    // 3. 验证 MRENCLAVE 一致
    if !bytes.Equal(peer.MRENCLAVE, s.localMREnclave) {
        return ErrDifferentCode
    }
    
    return nil
}
```

### 5.4 侧信道攻击防护（代码级方案）

SGX enclave 虽然提供了内存隔离，但仍然容易受到侧信道攻击。以下防护方案**完全依赖代码逻辑实现**，不依赖任何硬件特性。

#### 5.4.1 攻击类型与防护策略

| 攻击类型 | 攻击原理 | 代码级防护策略 |
|----------|----------|----------------|
| 时序攻击 | 测量执行时间推断秘密 | 常量时间操作 |
| 缓存攻击 | 观察缓存访问模式 | 预加载 + 避免秘密索引 |
| 页面错误攻击 | 观察内存页访问模式 | 内存访问模式混淆 |
| 分支预测攻击 | 观察分支预测行为 | 控制流混淆 |
| 推测执行攻击 | Spectre/Meltdown 变种 | 序列化屏障 + 常量时间 |

#### 5.4.2 常量时间操作

**核心原则**：代码执行时间不能依赖于秘密数据的值。

```go
// internal/sgx/constant_time.go
package sgx

// ConstantTimeCompare 常量时间比较两个字节切片
// 无论内容是否相同，执行时间都相同
func ConstantTimeCompare(a, b []byte) bool {
    if len(a) != len(b) {
        // 长度不同时，仍然遍历较长的切片以保持常量时间
        maxLen := len(a)
        if len(b) > maxLen {
            maxLen = len(b)
        }
        var result byte = 1 // 长度不同，结果为 false
        for i := 0; i < maxLen; i++ {
            var x, y byte
            if i < len(a) {
                x = a[i]
            }
            if i < len(b) {
                y = b[i]
            }
            result |= x ^ y
        }
        return false
    }
    
    // 长度相同，逐字节比较
    var result byte = 0
    for i := 0; i < len(a); i++ {
        result |= a[i] ^ b[i]
    }
    return result == 0
}

// ConstantTimeSelect 常量时间条件选择
// 根据 condition 选择 a 或 b，不使用分支
func ConstantTimeSelect(condition bool, a, b []byte) []byte {
    result := make([]byte, len(a))
    
    // 将 bool 转换为掩码：true -> 0xFF, false -> 0x00
    var mask byte
    if condition {
        mask = 0xFF
    }
    
    for i := 0; i < len(a); i++ {
        // result[i] = (a[i] & mask) | (b[i] & ^mask)
        result[i] = (a[i] & mask) | (b[i] & (^mask))
    }
    return result
}

// ConstantTimeCopy 常量时间条件复制
// 如果 condition 为 true，将 src 复制到 dst
func ConstantTimeCopy(condition bool, dst, src []byte) {
    var mask byte
    if condition {
        mask = 0xFF
    }
    
    for i := 0; i < len(dst) && i < len(src); i++ {
        dst[i] = (src[i] & mask) | (dst[i] & (^mask))
    }
}
```

#### 5.4.3 避免数据依赖的分支

**错误示例（有侧信道泄露）：**
```go
// 危险：分支依赖于秘密数据
func checkPassword(input, secret []byte) bool {
    if len(input) != len(secret) {
        return false  // 早期退出泄露长度信息
    }
    for i := 0; i < len(input); i++ {
        if input[i] != secret[i] {
            return false  // 早期退出泄露匹配位置
        }
    }
    return true
}
```

**正确示例（常量时间）：**
```go
// 安全：执行时间不依赖于秘密数据
func checkPasswordConstantTime(input, secret []byte) bool {
    // 使用常量时间比较
    return ConstantTimeCompare(input, secret)
}
```

#### 5.4.4 避免秘密索引的内存访问

**错误示例（缓存侧信道）：**
```go
// 危险：使用秘密值作为数组索引
var sbox = [256]byte{...}  // S-box 查找表

func lookupSbox(secretByte byte) byte {
    return sbox[secretByte]  // 缓存访问模式泄露 secretByte
}
```

**正确示例（全表扫描）：**
```go
// 安全：访问所有表项，使用掩码选择
func lookupSboxConstantTime(secretByte byte) byte {
    var result byte = 0
    for i := 0; i < 256; i++ {
        // 当 i == secretByte 时，mask = 0xFF，否则 mask = 0x00
        mask := constantTimeByteEq(byte(i), secretByte)
        result |= sbox[i] & mask
    }
    return result
}

func constantTimeByteEq(a, b byte) byte {
    // 如果 a == b，返回 0xFF；否则返回 0x00
    x := a ^ b
    // 将非零值映射到 0，零值映射到 1
    x = ^x
    x &= x >> 4
    x &= x >> 2
    x &= x >> 1
    x &= 1
    // 扩展到全字节
    return byte(int8(x<<7) >> 7)
}
```

#### 5.4.5 内存访问模式混淆（ORAM 简化版）

对于需要随机访问加密分区数据的场景，使用混淆访问模式：

```go
// internal/sgx/oblivious_access.go
package sgx

// ObliviousKeyStore 混淆访问模式的密钥存储
type ObliviousKeyStore struct {
    keys      []KeyEntry
    positions map[common.Hash]int  // 真实位置映射
    rng       *SecureRNG
}

// ObliviousRead 混淆读取 - 访问所有位置，只返回目标数据
func (s *ObliviousKeyStore) ObliviousRead(keyId common.Hash) (*KeyEntry, error) {
    targetPos := s.positions[keyId]
    
    var result *KeyEntry
    
    // 访问所有位置（混淆真实访问模式）
    for i := 0; i < len(s.keys); i++ {
        entry := &s.keys[i]
        
        // 常量时间选择：如果是目标位置，保存结果
        isTarget := constantTimeIntEq(i, targetPos)
        if isTarget == 1 {
            result = entry
        }
        
        // 即使不是目标，也执行相同的内存访问
        _ = entry.PrivateKey[0]  // 触发缓存加载
    }
    
    return result, nil
}

// ObliviousWrite 混淆写入 - 访问所有位置，只修改目标
func (s *ObliviousKeyStore) ObliviousWrite(keyId common.Hash, newData *KeyEntry) error {
    targetPos := s.positions[keyId]
    
    for i := 0; i < len(s.keys); i++ {
        isTarget := constantTimeIntEq(i, targetPos)
        
        // 常量时间条件写入
        ConstantTimeCopy(isTarget == 1, s.keys[i].PrivateKey, newData.PrivateKey)
    }
    
    return nil
}

func constantTimeIntEq(a, b int) int {
    x := uint64(a ^ b)
    x = ^x
    x &= x >> 32
    x &= x >> 16
    x &= x >> 8
    x &= x >> 4
    x &= x >> 2
    x &= x >> 1
    return int(x & 1)
}
```

#### 5.4.6 控制流混淆

对于必须有分支的代码，执行两个分支并选择结果：

```go
// 安全：执行两个分支，常量时间选择结果
func processKeyOperation(op int, key *KeyEntry) ([]byte, error) {
    // 执行所有可能的操作
    signResult := performSign(key)      // 总是执行
    verifyResult := performVerify(key)  // 总是执行
    ecdhResult := performECDH(key)      // 总是执行
    
    // 常量时间选择正确的结果
    var result []byte
    result = ConstantTimeSelect(op == OpSign, signResult, result)
    result = ConstantTimeSelect(op == OpVerify, verifyResult, result)
    result = ConstantTimeSelect(op == OpECDH, ecdhResult, result)
    
    return result, nil
}
```

#### 5.4.7 密码学库要求

X Chain 必须使用经过侧信道审计的密码学库：

| 操作 | 推荐库 | 要求 |
|------|--------|------|
| 椭圆曲线运算 | libsodium / BearSSL | 常量时间标量乘法 |
| AES 加密 | AES-NI 指令 / bitsliced | 避免 T-table 实现 |
| SHA256 哈希 | 标准实现 | 无秘密依赖分支 |
| RSA 签名 | 带盲化的实现 | Montgomery 乘法 + 盲化 |
| ECDSA 签名 | RFC 6979 确定性 | 常量时间 k 生成 |

**Go 语言实现要求：**

```go
// internal/sgx/crypto_requirements.go
package sgx

import (
    "crypto/subtle"  // Go 标准库的常量时间操作
    "golang.org/x/crypto/curve25519"  // 常量时间 X25519
)

// 使用 Go 标准库的常量时间比较
func secureCompare(a, b []byte) bool {
    return subtle.ConstantTimeCompare(a, b) == 1
}

// 使用常量时间的椭圆曲线库
func performX25519(privateKey, publicKey []byte) ([]byte, error) {
    var shared [32]byte
    var priv, pub [32]byte
    copy(priv[:], privateKey)
    copy(pub[:], publicKey)
    
    // curve25519.ScalarMult 是常量时间实现
    curve25519.ScalarMult(&shared, &priv, &pub)
    return shared[:], nil
}
```

#### 5.4.8 预编译合约的侧信道防护

所有 SGX 预编译合约必须遵循以下规则：

```go
// core/vm/contracts_sgx_secure.go
package vm

// SGX 预编译合约的安全包装器
type SecureSGXPrecompile struct {
    inner PrecompiledContract
}

func (s *SecureSGXPrecompile) Run(input []byte, caller common.Address, evm *EVM) ([]byte, error) {
    // 1. 输入长度标准化（防止长度泄露）
    paddedInput := padToFixedLength(input, MaxInputLength)
    
    // 2. 执行操作（内部使用常量时间实现）
    result, err := s.inner.Run(paddedInput, caller, evm)
    
    // 3. 输出长度标准化
    paddedResult := padToFixedLength(result, MaxOutputLength)
    
    // 4. 添加随机延迟（弱防护，但增加攻击难度）
    // 注意：这不是主要防护手段，只是额外层
    addRandomDelay()
    
    return paddedResult, err
}

func padToFixedLength(data []byte, length int) []byte {
    result := make([]byte, length)
    copy(result, data)
    return result
}

func addRandomDelay() {
    // 使用硬件随机数生成随机延迟
    var delay [1]byte
    sgxRandom(delay[:])
    
    // 执行空操作循环（编译器不会优化掉）
    for i := 0; i < int(delay[0]); i++ {
        runtime.Gosched()
    }
}
```

#### 5.4.9 侧信道防护检查清单

实现密码学操作时，必须检查以下项目：

```
[ ] 所有比较操作使用常量时间函数
[ ] 没有基于秘密数据的条件分支
[ ] 没有使用秘密值作为数组索引
[ ] 没有基于秘密数据的循环次数
[ ] 使用经过审计的密码学库
[ ] 输入/输出长度不泄露信息
[ ] 错误处理不泄露时序信息
[ ] 内存访问模式不依赖秘密数据
```

#### 5.4.10 测试与验证

```go
// internal/sgx/sidechannel_test.go
package sgx

import (
    "testing"
    "time"
)

// 测试常量时间比较
func TestConstantTimeCompare(t *testing.T) {
    secret := []byte("secret_password_12345")
    
    // 测试不同输入的执行时间
    inputs := [][]byte{
        []byte("wrong_password_12345"),  // 完全不同
        []byte("secret_password_12344"),  // 最后一位不同
        []byte("aecret_password_12345"),  // 第一位不同
        []byte("secret_password_12345"),  // 完全相同
    }
    
    var times []time.Duration
    iterations := 10000
    
    for _, input := range inputs {
        start := time.Now()
        for i := 0; i < iterations; i++ {
            ConstantTimeCompare(input, secret)
        }
        times = append(times, time.Since(start))
    }
    
    // 验证所有执行时间在统计误差范围内
    avgTime := averageDuration(times)
    for i, d := range times {
        deviation := float64(d-avgTime) / float64(avgTime)
        if deviation > 0.05 { // 允许 5% 误差
            t.Errorf("Input %d has timing deviation: %.2f%%", i, deviation*100)
        }
    }
}
```

## 6. P2P 网络层

### 6.1 节点连接准入控制

准入控制是**双向的**，同时控制入站和出站连接：

1. **被动准入（入站控制）**：控制允许谁连接到我 - 验证入站连接的度量值
2. **主动准入（出站控制）**：控制我只主动连接谁 - 验证出站连接的度量值

只有满足以下条件的节点才能建立连接：

1. **度量值匹配**：对方节点的 MRENCLAVE/MRSIGNER 必须在允许列表中
2. **Chain ID 匹配**：对方节点的 Chain ID 必须与本节点一致

**双向准入控制示意图：**

```
+------------------+                    +------------------+
|     节点 A       |                    |     节点 B       |
| 白名单: [B, C]   |                    | 白名单: [A, C]   |
+------------------+                    +------------------+
        |                                       |
        |  主动连接 (出站)                      |
        |  A 验证 B 的度量值是否在 A 的白名单   |
        |-------------------------------------->|
        |                                       |
        |                    被动接受 (入站)    |
        |                    B 验证 A 的度量值是否在 B 的白名单
        |                                       |
        |  双向验证通过，连接建立               |
        |<------------------------------------->|
        |                                       |

场景 1: A 主动连接 B
  - A 检查 B 的 MRENCLAVE 是否在 A 的白名单中 (主动准入)
  - B 检查 A 的 MRENCLAVE 是否在 B 的白名单中 (被动准入)
  - 两者都通过才能建立连接

场景 2: 恶意节点 X 尝试连接 A
  - X 的 MRENCLAVE 不在 A 的白名单中
  - A 拒绝连接 (被动准入失败)

场景 3: A 尝试连接未知节点 Y
  - Y 的 MRENCLAVE 不在 A 的白名单中
  - A 主动放弃连接 (主动准入失败)
```

**双向准入的安全意义：**

| 控制方向 | 作用 | 防护场景 |
|----------|------|----------|
| 被动准入 | 防止恶意节点连接 | 阻止未授权节点接入网络 |
| 主动准入 | 防止连接到恶意节点 | 避免同步到被篡改的数据 |

#### 6.1.0 设计目的：支持硬分叉升级

准入控制的核心目的是**支持硬分叉升级**，类似于以太坊的 EIP 实现机制。当需要发布新版代码实现新特性时，通过更新 MRENCLAVE 白名单来控制网络升级。

**硬分叉升级场景：**

1. **新特性发布**：类似 EIP-1559、EIP-4844 等协议升级，需要所有节点运行新版代码
2. **安全修复**：发现安全漏洞后，强制所有节点升级到修复版本
3. **性能优化**：优化后的代码产生不同的 MRENCLAVE，需要协调升级

**升级流程：**

```
时间线
  |
  v
+------------------+
| 阶段 1: 准备     |  发布新版代码，公布新 MRENCLAVE
+------------------+
  |
  v
+------------------+
| 阶段 2: 过渡     |  节点配置同时允许新旧 MRENCLAVE
|                  |  mrenclave = ["旧版本", "新版本"]
+------------------+
  |
  v
+------------------+
| 阶段 3: 升级     |  节点逐步升级到新版本
|                  |  新旧节点可以互相连接和同步
+------------------+
  |
  v
+------------------+
| 阶段 4: 完成     |  移除旧版 MRENCLAVE
|                  |  mrenclave = ["新版本"]
|                  |  未升级节点被隔离（硬分叉）
+------------------+
```

**版本兼容性管理示例：**

```toml
# config.toml - 过渡期配置
[sgx]
# 同时允许 v1.0.0 和 v1.1.0 版本
mrenclave = [
    "abc123...",  # v1.0.0 - 当前稳定版
    "def456...",  # v1.1.0 - 新版本（包含 XIP-001 特性）
]

# 升级完成后的配置
[sgx]
mrenclave = [
    "def456...",  # v1.1.0 - 仅允许新版本
]
# 运行 v1.0.0 的节点将无法连接，形成硬分叉
```

**与以太坊 EIP 的对比：**

| 特性 | 以太坊 EIP | X Chain XIP |
|------|-----------|-------------|
| 升级触发 | 区块高度 | MRENCLAVE 白名单 |
| 强制升级 | 需要社区共识 | 通过准入控制强制 |
| 回滚可能 | 困难 | 恢复旧 MRENCLAVE 即可 |
| 验证方式 | 区块验证规则 | SGX 远程证明 |

#### 6.1.0.1 硬分叉数据迁移与保留

硬分叉时必须保留分叉前的所有数据，包括区块链状态、账户余额、合约存储、以及加密分区中的私钥数据。

**数据分类：**

| 数据类型 | 存储位置 | 迁移策略 |
|----------|----------|----------|
| 区块链状态 | LevelDB | 直接继承，无需迁移 |
| 账户余额 | StateDB | 直接继承，无需迁移 |
| 合约存储 | StateDB | 直接继承，无需迁移 |
| 私钥数据 | 加密分区 | 需要重新封装 (Re-sealing) |
| 密钥元数据 | 加密分区 | 需要重新封装 |
| 派生秘密 | 加密分区 | 需要重新封装 |

**加密分区数据迁移机制：**

由于 SGX sealing 使用 MRENCLAVE 作为密钥派生因子，新版本代码的 MRENCLAVE 不同，无法直接解密旧版本封装的数据。因此需要特殊的迁移机制：

```
+------------------+                    +------------------+
|   旧版本节点     |                    |   新版本节点     |
| MRENCLAVE: ABC   |                    | MRENCLAVE: DEF   |
+------------------+                    +------------------+
        |                                       |
        |  1. 旧版本解封数据                    |
        |  (使用 MRENCLAVE=ABC 的密钥)          |
        |                                       |
        |  2. 通过 RA-TLS 安全通道传输          |
        |-------------------------------------->|
        |                                       |
        |                    3. 新版本重新封装  |
        |                    (使用 MRENCLAVE=DEF 的密钥)
        |                                       |
```

**迁移实现：**

```go
// internal/sgx/migration.go
package sgx

// DataMigrator 处理硬分叉时的数据迁移
type DataMigrator struct {
    oldEnclave *EnclaveConnection  // 连接到旧版本节点
    newEnclave *EnclaveConnection  // 本地新版本 enclave
    ratls      *RATLSTransport     // RA-TLS 安全通道
}

// MigrateEncryptedData 迁移加密分区数据
func (m *DataMigrator) MigrateEncryptedData(ctx context.Context) error {
    // 1. 建立 RA-TLS 连接到旧版本节点
    conn, err := m.ratls.Connect(m.oldEnclave.Address)
    if err != nil {
        return fmt.Errorf("failed to connect to old enclave: %w", err)
    }
    defer conn.Close()
    
    // 2. 请求旧版本节点解封并传输数据
    // 数据在 RA-TLS 通道中传输，保证安全性
    keys, err := m.requestKeyMigration(conn)
    if err != nil {
        return fmt.Errorf("failed to migrate keys: %w", err)
    }
    
    // 3. 在新版本 enclave 中重新封装
    for _, key := range keys {
        if err := m.newEnclave.SealKey(key); err != nil {
            return fmt.Errorf("failed to seal key %s: %w", key.ID, err)
        }
    }
    
    return nil
}

// KeyMigrationRequest 密钥迁移请求
type KeyMigrationRequest struct {
    KeyIDs    []common.Hash  // 要迁移的密钥 ID 列表
    Requester common.Address // 请求者地址（必须是密钥所有者）
    Signature []byte         // 请求者签名
}

// KeyMigrationResponse 密钥迁移响应
type KeyMigrationResponse struct {
    Keys []MigrationKeyData  // 解封后的密钥数据
}

type MigrationKeyData struct {
    ID         common.Hash
    CurveType  uint8
    PrivateKey []byte  // 明文私钥（仅在 RA-TLS 通道中传输）
    PublicKey  []byte
    Owner      common.Address
    Metadata   KeyMetadata
}
```

**迁移命令行工具：**

```bash
# 从旧版本节点迁移数据到新版本
geth migrate \
    --from "enode://old-node@192.168.1.100:30303" \
    --datadir /app/wallet/chaindata \
    --keys-only  # 仅迁移密钥数据，区块链数据自动继承
```

**迁移流程图：**

```
硬分叉数据迁移流程
==================

1. 准备阶段
   ├── 新版本节点启动
   ├── 检测到本地加密分区为空或版本不匹配
   └── 进入迁移模式

2. 连接阶段
   ├── 扫描网络中的旧版本节点
   ├── 建立 RA-TLS 连接
   └── 验证对方 MRENCLAVE 在允许列表中

3. 数据传输阶段
   ├── 旧节点解封私钥数据
   ├── 通过 RA-TLS 加密通道传输
   └── 新节点接收并验证数据完整性

4. 重新封装阶段
   ├── 使用新 MRENCLAVE 派生的密钥封装
   ├── 写入新版本加密分区
   └── 验证封装成功

5. 完成阶段
   ├── 标记迁移完成
   ├── 断开与旧节点连接
   └── 开始正常运行
```

**使用 MRSIGNER 模式简化迁移：**

如果使用 `--sgx.verify-mode mrsigner` 模式，且新旧版本使用相同的签名密钥，则可以使用 MRSIGNER 作为 sealing 密钥派生因子，避免数据迁移：

```toml
# manifest.template - 使用 MRSIGNER 作为 sealing 密钥
[[fs.mounts]]
type = "encrypted"
path = "/app/wallet"
uri = "file:/data/wallet"
key_name = "_sgx_mrsigner"  # 使用 MRSIGNER 而非 MRENCLAVE
```

**MRENCLAVE vs MRSIGNER sealing 对比：**

| 特性 | MRENCLAVE sealing | MRSIGNER sealing |
|------|-------------------|------------------|
| 安全性 | 更高（代码绑定） | 较低（签名者绑定） |
| 升级便利性 | 需要数据迁移 | 无需迁移 |
| 适用场景 | 高安全要求 | 频繁升级场景 |
| 回滚风险 | 低 | 旧版本可访问新数据 |

**推荐策略：**

1. **生产环境**：使用 MRENCLAVE sealing + 数据迁移机制
2. **测试环境**：使用 MRSIGNER sealing 简化升级流程
3. **混合策略**：核心私钥使用 MRENCLAVE，临时数据使用 MRSIGNER

#### 6.1.1 命令行参数

```bash
# 启动 X Chain 节点
geth \
    --networkid 762385986 \
    --sgx.mrenclave "abc123...,def456..." \
    --sgx.mrsigner "789abc..." \
    --sgx.verify-mode "mrenclave" \
    --datadir /app/wallet/chaindata
```

| 参数 | 描述 | 默认值 |
|------|------|--------|
| `--networkid` | Chain ID，必须匹配才能连接 | 762385986 |
| `--sgx.mrenclave` | 允许的 MRENCLAVE 列表（逗号分隔） | 本节点 MRENCLAVE |
| `--sgx.mrsigner` | 允许的 MRSIGNER 列表（逗号分隔） | 本节点 MRSIGNER |
| `--sgx.verify-mode` | 验证模式：`mrenclave`（严格）或 `mrsigner`（宽松） | `mrenclave` |
| `--sgx.tcb-allow-outdated` | 是否允许 TCB 过期的节点连接 | `false` |

#### 6.1.2 配置文件方式

```toml
# config.toml
[sgx]
# 允许的 MRENCLAVE 列表
mrenclave = [
    "abc123def456789...",  # v1.0.0 版本
    "def456789abc123...",  # v1.0.1 版本
]

# 允许的 MRSIGNER 列表
mrsigner = [
    "789abc123def456...",  # 官方签名者
]

# 验证模式
verify_mode = "mrenclave"  # 或 "mrsigner"

# TCB 策略
tcb_allow_outdated = false
```

#### 6.1.3 连接准入流程

```
+-------------+                    +-------------+
|   节点 A    |                    |   节点 B    |
+-------------+                    +-------------+
      |                                  |
      |  1. TCP 连接                     |
      |--------------------------------->|
      |                                  |
      |  2. RA-TLS 握手开始              |
      |<-------------------------------->|
      |                                  |
      |  3. 交换 SGX Quote               |
      |  (包含 MRENCLAVE, MRSIGNER)      |
      |<-------------------------------->|
      |                                  |
      |  4. 验证 Quote                   |
      |  - 检查 MRENCLAVE 是否在白名单   |
      |  - 检查 MRSIGNER 是否在白名单    |
      |  - 检查 TCB 状态                 |
      |                                  |
      |  5. 交换 Chain ID                |
      |<-------------------------------->|
      |                                  |
      |  6. 验证 Chain ID 匹配           |
      |  if (peerChainId != localChainId)|
      |      断开连接                    |
      |                                  |
      |  7. 连接建立成功                 |
      |<-------------------------------->|
      |                                  |
```

#### 6.1.4 准入控制实现

```go
// p2p/ratls/admission.go
package ratls

// AdmissionConfig 定义节点准入配置
type AdmissionConfig struct {
    ChainID          uint64    // Chain ID，必须匹配
    AllowedMREnclave [][]byte  // 允许的 MRENCLAVE 列表
    AllowedMRSigner  [][]byte  // 允许的 MRSIGNER 列表
    VerifyMode       string    // "mrenclave" 或 "mrsigner"
    AllowOutdatedTCB bool      // 是否允许 TCB 过期
}

// AdmissionController 控制节点连接准入
type AdmissionController struct {
    config *AdmissionConfig
}

func NewAdmissionController(config *AdmissionConfig) *AdmissionController {
    return &AdmissionController{config: config}
}

// VerifyPeer 验证对方节点是否允许连接
func (ac *AdmissionController) VerifyPeer(peerQuote []byte, peerChainID uint64) error {
    // 1. 验证 Chain ID
    if peerChainID != ac.config.ChainID {
        return fmt.Errorf("chain ID mismatch: expected %d, got %d", 
            ac.config.ChainID, peerChainID)
    }
    
    // 2. 解析 Quote
    mrenclave, mrsigner, tcbStatus, err := parseQuote(peerQuote)
    if err != nil {
        return fmt.Errorf("failed to parse quote: %w", err)
    }
    
    // 3. 验证 TCB 状态
    if !ac.config.AllowOutdatedTCB && tcbStatus != TCB_UP_TO_DATE {
        return fmt.Errorf("TCB status not up to date: %d", tcbStatus)
    }
    
    // 4. 根据验证模式检查度量值
    switch ac.config.VerifyMode {
    case "mrenclave":
        if !ac.isAllowedMREnclave(mrenclave) {
            return fmt.Errorf("MRENCLAVE not in allowed list: %x", mrenclave)
        }
    case "mrsigner":
        if !ac.isAllowedMRSigner(mrsigner) {
            return fmt.Errorf("MRSIGNER not in allowed list: %x", mrsigner)
        }
    default:
        // 默认使用 mrenclave 模式
        if !ac.isAllowedMREnclave(mrenclave) {
            return fmt.Errorf("MRENCLAVE not in allowed list: %x", mrenclave)
        }
    }
    
    return nil
}

func (ac *AdmissionController) isAllowedMREnclave(mrenclave []byte) bool {
    for _, allowed := range ac.config.AllowedMREnclave {
        if bytes.Equal(mrenclave, allowed) {
            return true
        }
    }
    return false
}

func (ac *AdmissionController) isAllowedMRSigner(mrsigner []byte) bool {
    for _, allowed := range ac.config.AllowedMRSigner {
        if bytes.Equal(mrsigner, allowed) {
            return true
        }
    }
    return false
}
```

### 6.2 RA-TLS 传输层

所有节点间通信使用 RA-TLS 加密通道：

```go
// p2p/ratls/transport.go
type RATLSTransport struct {
    localKey    *ecdsa.PrivateKey
    attestor    *SGXAttestor
    verifier    *SGXVerifier
    admission   *AdmissionController  // 准入控制器
}

func (t *RATLSTransport) Handshake(conn net.Conn) (*RATLSConn, error) {
    // 1. 生成 RA-TLS 证书
    cert, err := t.attestor.GenerateCertificate()
    if err != nil {
        return nil, err
    }
    
    // 2. TLS 握手
    tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{cert},
        VerifyPeerCertificate: t.verifyPeerCertificate,
    }
    
    tlsConn := tls.Server(conn, tlsConfig)
    if err := tlsConn.Handshake(); err != nil {
        return nil, err
    }
    
    return &RATLSConn{Conn: tlsConn}, nil
}

func (t *RATLSTransport) verifyPeerCertificate(rawCerts [][]byte, _ [][]*x509.Certificate) error {
    // 1. 解析证书中的 SGX Quote
    quote, err := extractSGXQuote(rawCerts[0])
    if err != nil {
        return err
    }
    
    // 2. 验证 Quote
    if err := t.verifier.VerifyQuote(quote); err != nil {
        return err
    }
    
    // 3. 检查 MRENCLAVE 是否在白名单中
    mrenclave := extractMREnclave(quote)
    if !t.isAllowedMREnclave(mrenclave) {
        return ErrInvalidMREnclave
    }
    
    return nil
}
```

### 6.2 消息协议扩展

```go
// eth/protocols/sgx/protocol.go
const (
    SGXProtocolName    = "sgx"
    SGXProtocolVersion = 1
)

// 新增消息类型
const (
    SGXStatusMsg          = 0x00  // SGX 状态信息
    SGXAttestationMsg     = 0x01  // 证明请求/响应
    SGXKeySyncMsg         = 0x02  // 密钥同步
    SGXConsistencyCheckMsg = 0x03 // 一致性检查
)

type SGXStatusPacket struct {
    MRENCLAVE     []byte
    MRSIGNER      []byte
    TCBStatus     uint8
    AttestationTS uint64
}
```

## 7. Gramine 集成

### 7.1 Manifest 配置

```toml
# geth.manifest.template

[libos]
entrypoint = "/usr/local/bin/geth"

[loader]
entrypoint = "file:{{ gramine.libos }}"
log_level = "warning"

[loader.env]
LD_LIBRARY_PATH = "/lib:/usr/lib:/usr/local/lib"
HOME = "/app"

[sys]
insecure__allow_eventfd = true
stack.size = "2M"
brk.max_size = "256M"

[sgx]
debug = false
enclave_size = "8G"
max_threads = 64
remote_attestation = "dcap"
trusted_files = [
    "file:/usr/local/bin/geth",
    "file:{{ gramine.libos }}",
    # ... 其他信任文件
]

# 加密分区配置
[[fs.mounts]]
type = "encrypted"
path = "/app/wallet"
uri = "file:/data/wallet"
key_name = "_sgx_mrenclave"

# 允许写入的文件
[sgx.allowed_files]
"/app/logs" = true
```

### 7.2 启动脚本

```bash
#!/bin/bash
# start-x-chain.sh

# 设置环境变量
export SGX_AESM_ADDR=1
export GRAMINE_LIBOS_PATH=/usr/lib/x86_64-linux-gnu/gramine/libsysdb.so

# 启动 Geth
exec gramine-sgx geth \
    --datadir /app/wallet/chaindata \
    --networkid 762385986 \
    --syncmode full \
    --gcmode archive \
    --http \
    --http.addr 0.0.0.0 \
    --http.port 8545 \
    --http.api eth,net,web3,sgx \
    --ws \
    --ws.addr 0.0.0.0 \
    --ws.port 8546 \
    --ws.api eth,net,web3,sgx
```

## 8. 模块拆解与实现指南

### 8.1 模块依赖关系

```
+------------------+
|  应用层 (RPC)    |
+------------------+
         |
+------------------+
|  预编译合约层    |
+------------------+
         |
+------------------+     +------------------+
|  共识引擎层      |<--->|  P2P 网络层      |
+------------------+     +------------------+
         |                        |
+------------------+     +------------------+
|  SGX 证明层      |     |  RA-TLS 层       |
+------------------+     +------------------+
         |                        |
+------------------+     +------------------+
|  Gramine 运行时  |<--->|  加密存储层      |
+------------------+     +------------------+
```

### 8.2 实现优先级

#### 第一阶段：基础设施

1. **Gramine 集成** (2 周)
   - 编写 Geth 的 Gramine manifest
   - 配置加密分区
   - 测试基本运行

2. **SGX 证明模块** (2 周)
   - 实现 SGX Quote 生成
   - 实现 Quote 验证
   - 集成 DCAP 库

#### 第二阶段：共识机制

3. **SGX 共识引擎** (3 周)
   - 实现 `consensus.Engine` 接口
   - 区块头扩展字段
   - 区块验证逻辑

4. **P2P RA-TLS 集成** (2 周)
   - 替换 RLPx 为 RA-TLS
   - 节点身份验证
   - 消息协议扩展

#### 第三阶段：预编译合约

5. **密钥管理预编译合约** (3 周)
   - SGX_KEY_CREATE
   - SGX_KEY_GET_PUBLIC
   - SGX_SIGN / SGX_VERIFY

6. **高级密码学预编译合约** (2 周)
   - SGX_ECDH
   - SGX_ENCRYPT / SGX_DECRYPT
   - SGX_KEY_DERIVE

7. **硬件随机数预编译合约** (1 周)
   - SGX_RANDOM

#### 第四阶段：数据同步

8. **加密分区数据同步** (2 周)
   - 密钥元数据同步协议
   - 一致性验证

9. **测试与优化** (2 周)
   - 单元测试
   - 集成测试
   - 性能优化

### 8.3 关键文件修改清单

```
go-ethereum/
├── consensus/
│   └── sgx/                          # 新增：SGX 共识引擎
│       ├── consensus.go              # 共识引擎实现
│       ├── attestor.go               # SGX 证明器
│       └── verifier.go               # Quote 验证器
├── core/
│   └── vm/
│       ├── contracts.go              # 修改：添加预编译合约注册
│       └── contracts_sgx.go          # 新增：SGX 预编译合约实现
├── p2p/
│   └── ratls/                        # 新增：RA-TLS 传输层
│       ├── transport.go
│       ├── handshake.go
│       └── certificate.go
├── eth/
│   └── protocols/
│       └── sgx/                      # 新增：SGX 协议
│           ├── protocol.go
│           ├── handler.go
│           └── peer.go
├── internal/
│   └── sgx/                          # 新增：SGX 内部工具
│       ├── keystore.go               # 加密分区密钥存储
│       ├── sealing.go                # SGX sealing
│       └── rdrand.go                 # 硬件随机数
└── params/
    └── config.go                     # 修改：添加 SGX 配置
```

### 8.4 接口定义

#### 8.4.1 SGX 证明器接口

```go
// internal/sgx/attestor.go
type Attestor interface {
    // 生成 SGX Quote
    GenerateQuote(reportData []byte) ([]byte, error)
    
    // 生成 RA-TLS 证书
    GenerateCertificate() (*tls.Certificate, error)
    
    // 获取本地 MRENCLAVE
    GetMREnclave() []byte
    
    // 获取本地 MRSIGNER
    GetMRSigner() []byte
}
```

#### 8.4.2 Quote 验证器接口

```go
// internal/sgx/verifier.go
type Verifier interface {
    // 验证 SGX Quote
    VerifyQuote(quote []byte) error
    
    // 验证 RA-TLS 证书
    VerifyCertificate(cert *x509.Certificate) error
    
    // 检查 MRENCLAVE 是否在白名单
    IsAllowedMREnclave(mrenclave []byte) bool
    
    // 添加 MRENCLAVE 到白名单
    AddAllowedMREnclave(mrenclave []byte)
}
```

#### 8.4.3 密钥存储接口

```go
// internal/sgx/keystore.go
type KeyStore interface {
    // 创建密钥对
    CreateKey(curveType uint8, owner common.Address) (common.Hash, error)
    
    // 获取公钥
    GetPublicKey(keyId common.Hash) ([]byte, error)
    
    // 签名
    Sign(keyId common.Hash, message []byte) ([]byte, error)
    
    // ECDH
    ECDH(keyId common.Hash, peerPublicKey []byte) (common.Hash, error)
    
    // 获取密钥所有者
    GetOwner(keyId common.Hash) (common.Address, error)
    
    // 验证所有权
    VerifyOwnership(keyId common.Hash, caller common.Address) bool
}
```

## 9. 安全考虑

### 9.1 威胁模型

| 威胁 | 缓解措施 |
|------|----------|
| 恶意节点运行篡改代码 | MRENCLAVE 验证确保代码完整性 |
| 私钥泄露 | 私钥存储在 SGX 加密分区，永不离开 enclave |
| 中间人攻击 | RA-TLS 双向认证 |
| 重放攻击 | Quote 包含时间戳和 nonce |
| 侧信道攻击 | 使用 SGX 最新安全补丁，避免敏感数据依赖的分支 |

### 9.2 密钥安全

1. **私钥隔离**：私钥永不离开 SGX enclave
2. **Sealing 保护**：使用 MRENCLAVE-based sealing 保护持久化密钥
3. **权限控制**：只有密钥所有者可以使用私钥
4. **派生秘密保护**：ECDH 等派生秘密同样存储在加密分区

### 9.3 网络安全

1. **双向认证**：所有节点通信都需要双向 SGX 远程证明
2. **MRENCLAVE 白名单**：只允许运行相同代码的节点加入网络
3. **TCB 检查**：验证节点的 TCB 状态是否最新

## 10. 测试策略

### 10.1 单元测试

```go
// 预编译合约测试
func TestSGXKeyCreate(t *testing.T) {
    // 测试密钥创建
}

func TestSGXSign(t *testing.T) {
    // 测试签名
}

func TestSGXECDH(t *testing.T) {
    // 测试 ECDH
}
```

### 10.2 集成测试

```go
// 多节点测试
func TestMultiNodeConsensus(t *testing.T) {
    // 启动多个节点
    // 验证共识达成
    // 验证数据一致性
}

func TestNodeJoin(t *testing.T) {
    // 测试新节点加入
    // 验证 SGX 远程证明
}
```

### 10.3 安全测试

```go
// 权限测试
func TestKeyOwnershipEnforcement(t *testing.T) {
    // 测试非所有者无法使用私钥
}

// 证明测试
func TestInvalidQuoteRejection(t *testing.T) {
    // 测试无效 Quote 被拒绝
}
```

## 11. 部署指南

### 11.1 硬件要求

- Intel CPU with SGX support (SGX2 recommended)
- 至少 16GB EPC (Enclave Page Cache)
- 支持 DCAP 的 SGX 驱动

### 11.2 软件要求

- Ubuntu 22.04 LTS
- Intel SGX SDK 2.19+
- Intel SGX DCAP 1.16+
- Gramine 1.5+
- Go 1.21+

### 11.3 部署步骤

```bash
# 1. 安装 SGX 驱动和 SDK
sudo apt install -y sgx-aesm-service libsgx-dcap-ql

# 2. 安装 Gramine
sudo apt install -y gramine

# 3. 构建 X Chain
cd go-ethereum
make geth

# 4. 生成 Gramine 签名
gramine-sgx-sign --manifest geth.manifest --output geth.manifest.sgx

# 5. 启动节点
./start-x-chain.sh
```

## 12. 硬件抽象层 (HAL)

### 12.1 设计目标

X Chain 默认使用 Intel SGX，但架构设计支持未来扩展到其他满足**恶意模型 (Malicious Model)** 的可信执行环境硬件。

### 12.2 安全模型要求

#### 12.2.1 恶意模型 (Malicious Model)

X Chain 要求底层硬件必须满足恶意模型，即：

- **不信任任何人**：包括云服务商、系统管理员、特权软件
- **只信任硬件本身**：安全性完全依赖硬件的密码学保证
- **抵抗特权攻击**：即使攻击者拥有 root 权限或物理访问权限，也无法窃取 enclave 内的秘密

#### 12.2.2 硬件分类

| 硬件 | 安全模型 | 是否支持 | 原因 |
|------|----------|----------|------|
| Intel SGX | 恶意模型 | 支持（默认） | 不信任 OS/Hypervisor，硬件级隔离 |
| Intel TDX | 恶意模型 | 未来支持 | VM 级 TEE，不信任 Hypervisor |
| RISC-V Keystone | 恶意模型 | 未来支持 | 开源 TEE，硬件级隔离 |
| ARM TrustZone | 半诚实模型 | 不支持 | 信任 Secure World 特权软件 |
| AMD SEV/SEV-SNP | 半诚实模型 | 不支持 | 信任 AMD 固件，内存加密但无完整性保护 |
| AWS Nitro Enclaves | 半诚实模型 | 不支持 | 信任 AWS Hypervisor |

### 12.3 硬件抽象层接口

```go
// internal/tee/hal.go
package tee

// TEEType 定义支持的 TEE 类型
type TEEType uint8

const (
    TEE_SGX      TEEType = 0x01  // Intel SGX (默认)
    TEE_TDX      TEEType = 0x02  // Intel TDX (未来)
    TEE_KEYSTONE TEEType = 0x03  // RISC-V Keystone (未来)
)

// TEEProvider 是硬件抽象层的核心接口
// 任何新的 TEE 硬件都必须实现此接口
type TEEProvider interface {
    // 基本信息
    Type() TEEType
    Name() string
    
    // 远程证明
    GenerateQuote(reportData []byte) ([]byte, error)
    VerifyQuote(quote []byte) (*QuoteVerificationResult, error)
    
    // 证书生成 (用于 RA-TLS)
    GenerateCertificate(privateKey crypto.PrivateKey) (*x509.Certificate, error)
    VerifyCertificate(cert *x509.Certificate) error
    
    // 代码度量
    GetCodeMeasurement() ([]byte, error)      // 类似 MRENCLAVE
    GetSignerMeasurement() ([]byte, error)    // 类似 MRSIGNER
    
    // 数据密封
    Seal(data []byte, policy SealPolicy) ([]byte, error)
    Unseal(sealedData []byte) ([]byte, error)
    
    // 硬件随机数
    GetRandomBytes(length int) ([]byte, error)
    
    // 安全模型验证
    SecurityModel() SecurityModel
    ValidateMaliciousModel() error  // 验证是否满足恶意模型
}

// SecurityModel 定义安全模型类型
type SecurityModel uint8

const (
    MODEL_MALICIOUS    SecurityModel = 0x01  // 恶意模型 (必需)
    MODEL_SEMI_HONEST  SecurityModel = 0x02  // 半诚实模型 (不支持)
)

// SealPolicy 定义数据密封策略
type SealPolicy uint8

const (
    SEAL_TO_ENCLAVE SealPolicy = 0x01  // 密封到特定 enclave (MRENCLAVE)
    SEAL_TO_SIGNER  SealPolicy = 0x02  // 密封到签名者 (MRSIGNER)
)

// QuoteVerificationResult 包含 Quote 验证结果
type QuoteVerificationResult struct {
    Valid           bool
    CodeMeasurement []byte
    SignerMeasurement []byte
    TCBStatus       TCBStatus
    Timestamp       time.Time
    AdditionalData  map[string]interface{}
}

// TCBStatus 定义 TCB 状态
type TCBStatus uint8

const (
    TCB_UP_TO_DATE      TCBStatus = 0x00
    TCB_OUT_OF_DATE     TCBStatus = 0x01
    TCB_REVOKED         TCBStatus = 0x02
    TCB_CONFIGURATION_NEEDED TCBStatus = 0x03
)
```

### 12.4 SGX 实现

```go
// internal/tee/sgx/provider.go
package sgx

type SGXProvider struct {
    dcapClient *DCAPClient
    config     *SGXConfig
}

func NewSGXProvider(config *SGXConfig) (*SGXProvider, error) {
    // 验证 SGX 可用性
    if !isSGXAvailable() {
        return nil, ErrSGXNotAvailable
    }
    
    return &SGXProvider{
        dcapClient: NewDCAPClient(),
        config:     config,
    }, nil
}

func (p *SGXProvider) Type() TEEType {
    return TEE_SGX
}

func (p *SGXProvider) Name() string {
    return "Intel SGX"
}

func (p *SGXProvider) SecurityModel() SecurityModel {
    return MODEL_MALICIOUS
}

func (p *SGXProvider) ValidateMaliciousModel() error {
    // SGX 满足恶意模型，直接返回 nil
    return nil
}

func (p *SGXProvider) GenerateQuote(reportData []byte) ([]byte, error) {
    // 通过 Gramine 的 /dev/attestation 接口生成 Quote
    // 1. 写入 report_data
    if err := os.WriteFile("/dev/attestation/user_report_data", reportData, 0600); err != nil {
        return nil, err
    }
    
    // 2. 读取 Quote
    quote, err := os.ReadFile("/dev/attestation/quote")
    if err != nil {
        return nil, err
    }
    
    return quote, nil
}

func (p *SGXProvider) VerifyQuote(quote []byte) (*QuoteVerificationResult, error) {
    // 使用 DCAP 验证 Quote
    return p.dcapClient.VerifyQuote(quote)
}

func (p *SGXProvider) GetRandomBytes(length int) ([]byte, error) {
    // 使用 RDRAND 指令获取硬件随机数
    buf := make([]byte, length)
    if _, err := rand.Read(buf); err != nil {
        return nil, err
    }
    return buf, nil
}
```

### 12.5 未来硬件扩展指南

当需要支持新的 TEE 硬件时，必须：

1. **验证安全模型**：确认硬件满足恶意模型要求
2. **实现 TEEProvider 接口**：实现所有必需的方法
3. **添加 TEEType 常量**：在 `TEEType` 中添加新的硬件类型
4. **实现远程证明**：提供 Quote 生成和验证功能
5. **实现数据密封**：提供与 SGX sealing 等效的功能
6. **测试验证**：通过所有安全测试

```go
// 示例：未来 Intel TDX 实现
// internal/tee/tdx/provider.go
package tdx

type TDXProvider struct {
    // TDX 特定配置
}

func (p *TDXProvider) Type() TEEType {
    return TEE_TDX
}

func (p *TDXProvider) SecurityModel() SecurityModel {
    return MODEL_MALICIOUS  // TDX 满足恶意模型
}

func (p *TDXProvider) ValidateMaliciousModel() error {
    // TDX 满足恶意模型
    return nil
}

// ... 实现其他接口方法
```

### 12.6 运行时硬件检测

```go
// internal/tee/detect.go
package tee

// DetectTEE 自动检测可用的 TEE 硬件
func DetectTEE() (TEEProvider, error) {
    // 优先检测 SGX
    if isSGXAvailable() {
        provider, err := sgx.NewSGXProvider(nil)
        if err == nil {
            return provider, nil
        }
    }
    
    // 未来：检测 TDX
    // if isTDXAvailable() {
    //     return tdx.NewTDXProvider(nil)
    // }
    
    // 未来：检测 Keystone
    // if isKeystoneAvailable() {
    //     return keystone.NewKeystoneProvider(nil)
    // }
    
    return nil, ErrNoTEEAvailable
}

// ValidateTEEProvider 验证 TEE 提供者是否满足要求
func ValidateTEEProvider(provider TEEProvider) error {
    // 1. 验证安全模型
    if provider.SecurityModel() != MODEL_MALICIOUS {
        return ErrNotMaliciousModel
    }
    
    // 2. 验证恶意模型实现
    if err := provider.ValidateMaliciousModel(); err != nil {
        return err
    }
    
    return nil
}
```

### 12.7 配置文件

```toml
# config.toml

[tee]
# 默认使用 SGX，未来可配置为其他满足恶意模型的硬件
type = "sgx"  # 可选值: "sgx", "tdx", "keystone"

# 是否强制验证恶意模型
require_malicious_model = true

[tee.sgx]
# SGX 特定配置
dcap_url = "https://api.trustedservices.intel.com/sgx/certification/v4"
allowed_tcb_status = ["UpToDate", "SWHardeningNeeded"]

# [tee.tdx]
# TDX 特定配置 (未来)

# [tee.keystone]
# Keystone 特定配置 (未来)
```

## 13. 区块浏览器与数据可见性

### 13.1 设计原则

X Chain 的架构设计确保敏感数据永远不会出现在公开的区块链数据中。区块浏览器**无需特殊处理**即可安全运行，因为所有链上数据本身就是设计为公开的。

### 13.2 数据可见性分类

| 数据类型 | 存储位置 | 区块浏览器可见 | 说明 |
|----------|----------|----------------|------|
| 交易哈希 | 区块链 | 是 | 标准以太坊数据 |
| 发送者/接收者地址 | 区块链 | 是 | 标准以太坊数据 |
| Gas、Value | 区块链 | 是 | 标准以太坊数据 |
| 合约调用输入数据 | 区块链 | 是 | 包括预编译合约参数 |
| 公钥 | 区块链状态 | 是 | 通过 SGX_KEY_GET_PUBLIC 返回 |
| 密钥 ID | 区块链状态 | 是 | 只是标识符，不含敏感信息 |
| 签名数据 | 交易返回值 | 是 | 签名本身是公开的 |
| 派生秘密 ID | 区块链状态 | 是 | 只是标识符，不含实际秘密 |
| **私钥** | Gramine 加密分区 | **否** | 永不离开 SGX enclave |
| **ECDH 派生秘密** | Gramine 加密分区 | **否** | 实际值存储在加密分区 |
| **对称加密密钥** | Gramine 加密分区 | **否** | 派生的加密密钥 |

### 13.3 预编译合约调用的数据流

```
+------------------+                    +------------------+
|   智能合约       |                    |   区块浏览器     |
+------------------+                    +------------------+
        |                                       |
        | 调用 SGX_KEY_CREATE(curveType=1)      |
        |-------------------------------------->| 可见: curveType=1
        |                                       |
        | 返回: keyId                           |
        |<--------------------------------------| 可见: keyId
        |                                       |
        | 调用 SGX_SIGN(keyId, msgHash)         |
        |-------------------------------------->| 可见: keyId, msgHash
        |                                       |
        | 返回: signature                       |
        |<--------------------------------------| 可见: signature
        |                                       |
        | 调用 SGX_ECDH(myKeyId, peerPubKey)    |
        |-------------------------------------->| 可见: myKeyId, peerPubKey
        |                                       |
        | 返回: derivedSecretId                 |
        |<--------------------------------------| 可见: derivedSecretId
        |                                       |   (不可见: 实际的派生秘密值)
```

### 13.4 安全保证

1. **私钥隔离**：私钥在 SGX enclave 内生成，存储在 Gramine 加密分区，永不出现在交易数据或区块链状态中。

2. **派生秘密保护**：ECDH 等操作产生的派生秘密只返回一个 ID 给合约，实际秘密值存储在加密分区中，只能通过后续的加密/解密操作使用。

3. **操作可审计**：虽然私钥和秘密值不可见，但所有操作（谁创建了密钥、谁进行了签名、谁执行了 ECDH）都记录在链上，可供审计。

4. **元数据可见性**：密钥的元数据（所有者地址、曲线类型、创建时间）是公开的，这是设计如此，便于合约逻辑和用户查询。

### 13.5 区块浏览器实现建议

区块浏览器可以像标准以太坊浏览器一样实现，额外支持以下功能：

```go
// 解析 SGX 预编译合约调用
func ParseSGXPrecompileCall(tx *types.Transaction) *SGXCallInfo {
    to := tx.To()
    if to == nil {
        return nil
    }
    
    // 检查是否是 SGX 预编译合约地址 (0x8000 - 0x80FF)
    addr := to.Big().Uint64()
    if addr < 0x8000 || addr > 0x80FF {
        return nil
    }
    
    input := tx.Data()
    
    switch addr {
    case 0x8000: // SGX_KEY_CREATE
        return &SGXCallInfo{
            Type:      "KEY_CREATE",
            CurveType: getCurveName(input[0]),
        }
    case 0x8002: // SGX_SIGN
        return &SGXCallInfo{
            Type:    "SIGN",
            KeyID:   common.BytesToHash(input[0:32]).Hex(),
            MsgHash: common.BytesToHash(input[32:64]).Hex(),
        }
    case 0x8004: // SGX_ECDH
        return &SGXCallInfo{
            Type:       "ECDH",
            LocalKeyID: common.BytesToHash(input[0:32]).Hex(),
            // 对方公钥是公开的
        }
    // ... 其他预编译合约
    }
    
    return nil
}
```

### 13.6 隐私考虑

虽然私钥和秘密值是安全的，但以下信息是公开可见的，用户应当了解：

| 可见信息 | 隐私影响 | 缓解措施 |
|----------|----------|----------|
| 密钥创建时间 | 可推断用户活动模式 | 使用批量创建或延迟创建 |
| 签名操作频率 | 可推断交易活动 | 使用代理合约聚合操作 |
| ECDH 参与方 | 可推断通信关系 | 使用中间密钥或混淆 |
| 密钥所有者地址 | 关联用户身份 | 使用多个地址分散密钥 |

这些是区块链透明性的固有特性，与传统以太坊相同。X Chain 的安全保证是：**即使所有链上数据都被分析，私钥和派生秘密的实际值仍然是安全的**。

## 14. 附录

### 14.1 参考资料

- [Intel SGX Developer Reference](https://download.01.org/intel-sgx/sgx-linux/2.19/docs/)
- [Gramine Documentation](https://gramine.readthedocs.io/)
- [go-ethereum Documentation](https://geth.ethereum.org/docs)
- [RA-TLS Specification](https://gramine.readthedocs.io/en/stable/attestation.html)

### 14.2 术语表

| 术语 | 定义 |
|------|------|
| SGX | Intel Software Guard Extensions，硬件可信执行环境 |
| Enclave | SGX 保护的内存区域 |
| MRENCLAVE | Enclave 代码和数据的度量值 |
| MRSIGNER | Enclave 签名者的度量值 |
| Quote | SGX 远程证明数据结构 |
| RA-TLS | Remote Attestation TLS，带远程证明的 TLS |
| DCAP | Data Center Attestation Primitives |
| Sealing | SGX 数据持久化加密机制 |
| TCB | Trusted Computing Base |
