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

#### 3.3.2.1 与以太坊原有共识机制的关系

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

###### 3.3.8.2.0 前三名收益分配机制

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

**收益分配示例**：

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
│  区块质量得分: 7500                                          │
│  质量倍数: 1.42x                                             │
│                                                             │
│  速度基础奖励: 60%                                           │
│  最终倍数: 60% × 1.42 = 0.85                                 │
└─────────────────────────────────────────────────────────────┘

矿工 C (第3名，无新交易):
┌─────────────────────────────────────────────────────────────┐
│  排名: 第 3 名（稍慢 400ms）                                 │
│  交易数量: 15 笔                                             │
│  新交易数量: 0 笔（所有交易都已被矿工 A 包含）               │
│                                                             │
│  收益: 0 ETH（无新交易，不分配收益）                         │
└─────────────────────────────────────────────────────────────┘

收益分配 (假设总交易费 = 1 ETH):
┌─────────────────────────────────────────────────────────────┐
│  矿工 A: 2 笔交易（全部是新交易）                            │
│  矿工 B: 30 笔交易，其中 28 笔是新交易（A 没有的）           │
│  矿工 C: 15 笔交易，其中 0 笔是新交易（全部被 A 包含）       │
│                                                             │
│  矿工 A 最终倍数: 100% × 0.58 = 0.58                        │
│  矿工 B 最终倍数: 60% × 1.42 × (28/30) = 0.80               │
│  矿工 C 最终倍数: 0（无新交易，不参与分配）                  │
│                                                             │
│  总倍数: 0.58 + 0.80 = 1.38                                 │
│                                                             │
│  矿工 A 收益: 1 ETH × (0.58/1.38) = 0.420 ETH (42.0%)       │
│  矿工 B 收益: 1 ETH × (0.80/1.38) = 0.580 ETH (58.0%)       │
│  矿工 C 收益: 0 ETH (0%)                                     │
│                                                             │
│  结论: 矿工 B 因为包含大量新交易，收益最高！                 │
│        矿工 C 没有新交易，不获得收益。                       │
└─────────────────────────────────────────────────────────────┘
```

**激励效果**：
- 速度仍然重要（第1名基础奖励最高）
- 新交易贡献是关键（只有包含新交易才能获得收益）
- 防止"搭便车"（后续矿工如果没有新交易，不分配收益）
- 鼓励矿工尽可能收集更多不同的交易

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
- 观测者必须是活跃节点（有交易处理记录）
- 观测结果需要签名（可追溯责任）
- 异常观测模式会被检测（如某节点总是报告他人离线）
```

###### 3.3.8.3.3 交易参与追踪

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

// calculateNodeGasUsed 计算节点处理的总 Gas
func (t *TxParticipationTracker) calculateNodeGasUsed(records []TxParticipationRecord) uint64 {
    var totalGas uint64
    for _, record := range records {
        totalGas += record.GasUsed
    }
    return totalGas
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
    heartbeatMgr        *HeartbeatManager
    consensusMgr        *UptimeConsensus
    txParticipation     *TxParticipationTracker
    responseTracker     *ResponseTimeTracker
    config              *UptimeConfig
}

// UptimeConfig 在线率计算配置
type UptimeConfig struct {
    // 权重配置（总和 = 100）
    HeartbeatWeight       uint8 // SGX 心跳权重，默认 40
    ConsensusWeight       uint8 // 多节点共识权重，默认 30
    TxParticipationWeight uint8 // 交易参与权重，默认 20
    ResponseWeight        uint8 // 响应时间权重，默认 10
}

// CalculateComprehensiveUptime 计算综合在线率
func (c *UptimeCalculator) CalculateComprehensiveUptime(nodeID common.Hash) uint64 {
    cfg := c.config
    
    // 1. SGX 心跳得分
    heartbeatScore := c.heartbeatMgr.GetHeartbeatScore(nodeID)
    
    // 2. 多节点共识得分
    consensusScore, _ := c.consensusMgr.CalculateUptimeScore(nodeID)
    
    // 3. 交易参与得分
    txParticipationScore := c.txParticipation.CalculateParticipationScore(nodeID)
    
    // 4. 响应时间得分
    responseScore := c.responseTracker.CalculateResponseScore(nodeID)
    
    // 5. 加权计算
    totalScore := (heartbeatScore * uint64(cfg.HeartbeatWeight) +
                   consensusScore * uint64(cfg.ConsensusWeight) +
                   txParticipationScore * uint64(cfg.TxParticipationWeight) +
                   responseScore * uint64(cfg.ResponseWeight)) / 100
    
    return totalScore
}
```

**衡量机制总结**：

| 机制 | 权重 | 衡量内容 | 防伪造方式 |
|------|------|----------|------------|
| SGX 签名心跳 | 40% | 节点是否定期发送心跳 | SGX enclave 签名 + Quote |
| 多节点共识 | 30% | 多个节点观测的共识结果 | 2/3 共识 + 签名追溯 |
| 交易参与 | 20% | 处理的交易数量和 Gas 贡献比例 | 区块链不可篡改记录 |
| 响应时间 | 10% | 交易处理响应速度 | 交易哈希 + 时间戳 |

**为什么不使用"出块数量"作为衡量标准**：

X Chain 采用按需出块机制，只有在有用户交易时才会出块。这意味着出块频率完全取决于网络交易量，而不是节点的在线状态或贡献度。因此，使用"交易参与比例"（节点处理的交易数量占网络总交易数量的比例）更能准确反映节点的实际贡献。

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
    TotalTxProcessed uint64       // 累计处理的交易数
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

##### 3.3.8.5.1 在线奖励机制（解决按需出块的激励不足问题）

**核心问题**：X Chain 采用按需出块机制，如果网络交易量不足，矿工激励不足，会导致矿工数量自动缩减，形成恶性循环。

```
问题分析:
┌─────────────────────────────────────────────────────────────────────────┐
│  按需出块的激励困境:                                                     │
│                                                                         │
│  交易量少 → 出块少 → 矿工收入低 → 矿工退出 → 节点减少                   │
│       ↑                                                    |            │
│       +----------------------------------------------------+            │
│                                                                         │
│  目标: 即使交易量少，也要维持足够的矿工数量保证网络安全                  │
└─────────────────────────────────────────────────────────────────────────┘
```

**解决方案**：将挖矿定义为"长期稳定在线"，基于在线时间和在线质量定时发放"在线奖励"，但确保交易收益远高于在线奖励。

```go
// consensus/sgx/online_reward.go
package sgx

// OnlineRewardConfig 在线奖励配置
type OnlineRewardConfig struct {
    // 在线奖励发放间隔（默认 1 小时）
    RewardInterval      time.Duration
    
    // 基础在线奖励（每小时）
    BaseOnlineReward    *big.Int
    
    // 在线质量加成（高质量在线可获得更高奖励）
    QualityMultiplier   func(uptimeScore uint64) float64
    
    // 交易收益保护系数（确保交易收益远高于在线奖励）
    // 任何交易的收益必须 >= 前 N 个空块最高在线奖励 × 此系数
    TxRewardProtectionFactor  float64  // 默认 10.0
    TxRewardProtectionBlocks  int      // 默认 10
}

// DefaultOnlineRewardConfig 默认配置
func DefaultOnlineRewardConfig() *OnlineRewardConfig {
    return &OnlineRewardConfig{
        RewardInterval:           1 * time.Hour,
        BaseOnlineReward:         big.NewInt(1e15),  // 0.001 ETH/小时
        TxRewardProtectionFactor: 10.0,              // 交易收益 >= 10 × 最高在线奖励
        TxRewardProtectionBlocks: 10,                // 参考前 10 个空块
        QualityMultiplier: func(uptimeScore uint64) float64 {
            // 在线质量得分 >= 9500: 1.5x 奖励
            // 在线质量得分 >= 9000: 1.2x 奖励
            // 在线质量得分 >= 8000: 1.0x 奖励
            // 在线质量得分 < 8000: 0.5x 奖励
            switch {
            case uptimeScore >= 9500:
                return 1.5
            case uptimeScore >= 9000:
                return 1.2
            case uptimeScore >= 8000:
                return 1.0
            default:
                return 0.5
            }
        },
    }
}

// OnlineRewardManager 在线奖励管理器
type OnlineRewardManager struct {
    config          *OnlineRewardConfig
    uptimeCalc      *UptimeCalculator
    reputationMgr   *ReputationManager
    
    // 记录最近的在线奖励（用于交易收益保护计算）
    recentOnlineRewards []*OnlineRewardRecord
}

// OnlineRewardRecord 在线奖励记录
type OnlineRewardRecord struct {
    NodeID      common.Hash
    Timestamp   uint64
    Reward      *big.Int
    UptimeScore uint64
}

// CalculateOnlineReward 计算节点的在线奖励
func (m *OnlineRewardManager) CalculateOnlineReward(nodeID common.Hash) *big.Int {
    // 1. 获取节点的在线质量得分
    uptimeScore := m.uptimeCalc.CalculateComprehensiveUptime(nodeID)
    
    // 2. 检查是否满足最低在线要求
    if uptimeScore < 8000 {  // 低于 80% 在线率不发放奖励
        return big.NewInt(0)
    }
    
    // 3. 计算质量加成
    multiplier := m.config.QualityMultiplier(uptimeScore)
    
    // 4. 计算最终奖励
    reward := new(big.Int).Set(m.config.BaseOnlineReward)
    reward.Mul(reward, big.NewInt(int64(multiplier * 100)))
    reward.Div(reward, big.NewInt(100))
    
    return reward
}

// GetMinTxReward 获取最小交易收益（确保交易收益远高于在线奖励）
// 核心原则: 一旦有交易，交易获取的收益必须远高于前 N 个空块的最高收益
func (m *OnlineRewardManager) GetMinTxReward() *big.Int {
    // 1. 获取最近 N 个在线奖励中的最高值
    maxOnlineReward := big.NewInt(0)
    recentCount := min(len(m.recentOnlineRewards), m.config.TxRewardProtectionBlocks)
    
    for i := 0; i < recentCount; i++ {
        record := m.recentOnlineRewards[len(m.recentOnlineRewards)-1-i]
        if record.Reward.Cmp(maxOnlineReward) > 0 {
            maxOnlineReward = new(big.Int).Set(record.Reward)
        }
    }
    
    // 2. 计算最小交易收益 = 最高在线奖励 × 保护系数
    minTxReward := new(big.Int).Mul(
        maxOnlineReward,
        big.NewInt(int64(m.config.TxRewardProtectionFactor)),
    )
    
    return minTxReward
}

// DistributeOnlineRewards 定时发放在线奖励
func (m *OnlineRewardManager) DistributeOnlineRewards(activeNodes []common.Hash) map[common.Hash]*big.Int {
    rewards := make(map[common.Hash]*big.Int)
    
    for _, nodeID := range activeNodes {
        reward := m.CalculateOnlineReward(nodeID)
        if reward.Sign() > 0 {
            rewards[nodeID] = reward
            
            // 记录奖励（用于交易收益保护计算）
            m.recentOnlineRewards = append(m.recentOnlineRewards, &OnlineRewardRecord{
                NodeID:      nodeID,
                Timestamp:   uint64(time.Now().Unix()),
                Reward:      reward,
                UptimeScore: m.uptimeCalc.CalculateComprehensiveUptime(nodeID),
            })
        }
    }
    
    // 保留最近的记录
    if len(m.recentOnlineRewards) > 1000 {
        m.recentOnlineRewards = m.recentOnlineRewards[len(m.recentOnlineRewards)-1000:]
    }
    
    return rewards
}
```

**在线奖励机制的核心原则**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│  核心原则: 交易最重要                                                    │
│                                                                         │
│  1. 在线奖励是"保底收入"                                                │
│     - 即使没有交易，长期高质量在线的节点也能获得收益                     │
│     - 防止矿工因收入不稳定而退出                                        │
│                                                                         │
│  2. 交易收益必须远高于在线奖励                                          │
│     - 任何交易的收益 >= 前 10 个空块最高在线奖励 × 10                   │
│     - 确保矿工有强烈动机处理交易                                        │
│                                                                         │
│  3. 在线质量影响奖励                                                    │
│     - 高质量在线（响应快、稳定）获得更高奖励                            │
│     - 激励矿工提供优质服务                                              │
└─────────────────────────────────────────────────────────────────────────┘

收益对比示例:
┌─────────────────────────────────────────────────────────────────────────┐
│  假设: 基础在线奖励 = 0.001 ETH/小时                                    │
│                                                                         │
│  高质量在线节点 (95%+ 在线率):                                          │
│  - 在线奖励: 0.001 × 1.5 = 0.0015 ETH/小时                             │
│  - 24 小时收入: 0.036 ETH                                               │
│                                                                         │
│  一笔交易的最低收益:                                                    │
│  - 最小交易收益 = 0.0015 × 10 = 0.015 ETH                              │
│  - 一笔交易 >= 10 小时的在线奖励                                        │
│                                                                         │
│  结论: 矿工有强烈动机处理交易，同时在没有交易时也有保底收入              │
└─────────────────────────────────────────────────────────────────────────┘
```

**激励效果**：

| 场景 | 矿工行为 | 收益来源 |
|------|----------|----------|
| 交易量充足 | 积极处理交易 | 主要来自交易费（远高于在线奖励） |
| 交易量不足 | 保持高质量在线 | 在线奖励（保底收入） |
| 长期稳定运营 | 持续提供服务 | 在线奖励 + 交易费 + 历史贡献加成 |

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
│     运营时长、累计处理交易数、网络贡献                               │
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
    TotalTxProcessed    uint64        // 累计处理交易数
    TotalGasProcessed   uint64        // 累计处理的 Gas
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
        |  4. A 读取本地加密分区数据            |
        |  (Gramine 自动解封，应用获得明文)     |
        |                                       |
        |  5. 通过 RA-TLS 通道传输秘密数据      |
        |  (传输过程中 TLS 加密保护)            |
        |-------------------------------------->|
        |                                       |
        |                    6. B 接收秘密数据  |
        |                    在 enclave 内部    |
        |                                       |
        |                    7. B 写入加密分区  |
        |                    (Gramine 自动加密) |
        |                                       |
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
        
        // 存储到本地加密分区（应用只需普通文件写入，Gramine 自动加密）
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
3. **端到端加密**：秘密数据由 Gramine 自动解密，通过 TLS 传输，目标节点 Gramine 自动加密存储
4. **无明文暴露**：秘密数据在整个同步过程中从不以明文形式暴露给主机操作系统

**注意**：Gramine 透明处理加密分区的加解密，应用只需进行标准文件 I/O 操作。

```
秘密数据生命周期（Gramine 透明加密）：

源节点 enclave          RA-TLS 通道           目标节点 enclave
[Gramine 加密] --自动解密--> [明文] --TLS加密--> [明文] --自动加密--> [Gramine 加密]
     |                        |                   |                    |
     |                        |                   |                    |
  存储在磁盘              仅在 enclave 内       仅在 enclave 内      存储在磁盘
  (加密状态)              (受 SGX 保护)        (受 SGX 保护)        (加密状态)
  
应用代码只需：
  读取: data := os.ReadFile("/encrypted/key.bin")  // Gramine 自动解密
  写入: os.WriteFile("/encrypted/key.bin", data, 0600)  // Gramine 自动加密
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
|                  |  【新版本节点进入只读模式】
+------------------+
  |
  v
+------------------+
| 阶段 3: 升级     |  节点逐步升级到新版本
|                  |  新节点同步区块，但不处理交易
|                  |  旧节点继续正常出块和处理交易
+------------------+
  |
  v
+------------------+
| 阶段 4: 完成     |  移除旧版 MRENCLAVE
|                  |  mrenclave = ["新版本"]
|                  |  新版本节点恢复正常模式
|                  |  未升级节点被隔离（硬分叉）
+------------------+
```

**升级期间只读模式：**

在过渡阶段（白名单中存在多个 MRENCLAVE 时），新版本节点进入只读模式：
- **允许的操作**：同步区块、验证区块、读取状态
- **禁止的操作**：处理交易、出块、任何会导致状态修改的操作

**升级完成区块高度与秘密数据同步：**

为了提供明确的升级截止时间，引入 `UpgradeCompleteBlock` 参数。该参数是安全参数，存储在 **SecurityConfigContract** 中，由 **GovernanceContract** 通过投票机制管理。

由于秘密数据（私钥等）与区块高度关联，新节点需要通过 RA-TLS 安全通道自动从旧节点同步秘密数据。同步过程记录当前已同步到的区块高度（`secretDataSyncedBlock`）。

升级完成条件（满足任一即可）：
1. 白名单中只剩下一个 MRENCLAVE（通过投票移除旧版本）
2. 秘密数据已同步到 `UpgradeCompleteBlock` 高度（`secretDataSyncedBlock >= UpgradeCompleteBlock`）

注意：不需要单独检查当前区块高度，因为非秘密数据是直接复用的，秘密数据同步到指定高度本身就意味着节点已准备好处理该高度的数据。

当升级完成后，即使合约还没把度量值改成一个，新节点也只接受与自己一致度量值的节点，旧版本节点将被隔离。

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

#### 6.1.0.0.1 硬分叉安全风险与防护

硬分叉机制存在一个根本性的安全矛盾：允许硬分叉意味着允许新版本代码访问旧版本的所有加密分区数据（包括私钥）。如果恶意代码的 MRENCLAVE 被加入白名单，所有历史机密数据都会泄露。

**风险分析：**

| 风险类型 | 描述 | 影响 |
|----------|------|------|
| 白名单管理被攻破 | 攻击者获得白名单管理权限 | 可添加恶意 MRENCLAVE |
| 供应链攻击 | 官方发布的代码被植入后门 | 恶意代码获得合法 MRENCLAVE |
| 内部威胁 | 核心开发者作恶 | 发布包含数据泄露功能的代码 |
| 社会工程 | 欺骗白名单管理者 | 恶意代码被误加入白名单 |

**核心矛盾：**

```
信任根的转移:
┌─────────────────────────────────────────────────────────────────────────┐
│  SGX 原本的安全模型: "只信任硬件"                                        │
│                                                                         │
│  支持硬分叉后: "信任硬件 + 信任白名单管理者"                             │
│                                                                         │
│  问题: 白名单管理成为单点故障                                            │
│        一旦白名单被攻破，所有历史数据永久泄露                            │
└─────────────────────────────────────────────────────────────────────────┘
```

#### 6.1.0.0.2 安全参数架构

X Chain 的安全参数采用**链上合约存储 + Manifest 固定合约地址**的架构：

**合约职责划分：**

| 合约 | 职责 |
|------|------|
| **安全配置合约（SecurityConfigContract）** | 存储安全相关配置（MRENCLAVE 白名单、升级配置、密钥迁移策略等），被其他模块读取 |
| **治理合约（GovernanceContract）** | 负责投票、管理验证者、存储治理配置（质押金额、投票参数、验证者配置等），把安全配置变更结果写入 SecurityConfigContract |

**Manifest 固定参数：**

这些参数在 Docker 镜像构建时嵌入 Gramine manifest，影响 MRENCLAVE 度量值：

| 参数 | 说明 |
|------|------|
| `XCHAIN_ENCRYPTED_PATH` | 加密分区路径 |
| `XCHAIN_SECRET_PATH` | 秘密数据路径 |
| `XCHAIN_GOVERNANCE_CONTRACT` | 治理合约地址（写死，作为安全锚点） |
| `XCHAIN_SECURITY_CONFIG_CONTRACT` | 安全配置合约地址（写死，作为安全锚点） |

**重要说明**：合约地址写死在 Manifest 中，影响 MRENCLAVE，攻击者无法修改合约地址而不改变度量值。

**链上安全参数（从 SecurityConfigContract 读取）：**

所有安全、准入、秘密数据管理策略等相关配置都从安全配置合约动态读取：

| 参数 | 说明 |
|------|------|
| MRENCLAVE 白名单 | 允许的 enclave 代码度量值 |
| MRSIGNER 白名单 | 允许的签名者度量值 |
| 密钥迁移阈值 | 密钥迁移所需的最小节点数 |
| 节点准入策略 | 是否严格验证 Quote |
| 分叉配置 | 硬分叉升级相关配置 |
| 数据迁移策略 | 加密数据迁移相关配置 |

**网络引导机制（Bootstrap）：**

X Chain 的安全参数从链上合约读取，但首次运行时还没有链。解决方案是**创世区块预部署合约**：

1. 治理合约和安全配置合约在创世区块中预部署
2. 合约地址是确定性的（基于部署者地址和 nonce），可以预先计算
3. Manifest 中写死这个预计算的合约地址
4. 引导阶段：前 N 个（如 5 个）不同 SGX Instance ID 的节点自动成为创始管理者
5. 正常阶段：新管理者必须通过现有管理者投票添加

**创始管理者选择机制：**

除了升级硬分叉期间，所有节点的 MRENCLAVE 都是完全相同的。区分不同节点的是 **SGX Instance ID**（硬件唯一标识），而不是 MRENCLAVE。

创始管理者的选择基于：
- **MRENCLAVE 验证**：确保运行的是正确的代码（所有节点相同）
- **Instance ID 去重**：每个物理 CPU 只能注册一个创始管理者
- **先到先得**：前 N 个注册的不同硬件实例成为创始管理者

这确保了创始管理者来自不同的物理硬件，防止单个实体通过多个软件实例控制网络引导过程。

#### 6.1.0.0.3 分层验证者治理机制

为解决安全配置管理的单点故障问题，X Chain 采用分层验证者机制，通过 2/3 多数投票来管理安全配置更新。

**分层验证者架构：**

```
+------------------------------------------------------------------+
|                     白名单治理架构                                 |
+------------------------------------------------------------------+
|                                                                  |
|  ┌────────────────────────────────────────────────────────────┐  |
|  │                    核心验证者层                              │  |
|  │  (5-7 个固定成员，负责日常升级决策)                          │  |
|  │                                                            │  |
|  │  成员构成:                                                  │  |
|  │  - 项目核心开发者 (2-3 人)                                  │  |
|  │  - 知名安全审计机构 (1-2 人)                                │  |
|  │  - 社区选举代表 (1-2 人)                                    │  |
|  │  - 合作伙伴/生态项目代表 (1 人)                             │  |
|  │                                                            │  |
|  │  投票规则: 2/3 多数同意 (如 7 人中需 5 人同意)              │  |
|  └────────────────────────────────────────────────────────────┘  |
|                              │                                   |
|                              │ 提案                              |
|                              ▼                                   |
|  ┌────────────────────────────────────────────────────────────┐  |
|  │                    社区验证者层                              │  |
|  │  (动态成员，对重大升级有否决权)                              │  |
|  │                                                            │  |
|  │  准入条件:                                                  │  |
|  │  - 节点运行时间 > 30 天                                     │  |
|  │  - 质押代币 > 10,000 X                                      │  |
|  │  - SGX 硬件验证通过                                         │  |
|  │  - 无历史恶意行为记录                                       │  |
|  │                                                            │  |
|  │  否决规则: > 1/3 社区验证者反对则否决升级                   │  |
|  └────────────────────────────────────────────────────────────┘  |
|                                                                  |
+------------------------------------------------------------------+
```

**验证者身份与投票权：**

```go
// governance/validator.go
package governance

import (
    "crypto/ecdsa"
    "time"
    
    "github.com/ethereum/go-ethereum/common"
)

// ValidatorType 验证者类型
type ValidatorType uint8

const (
    CoreValidator      ValidatorType = 0x01  // 核心验证者
    CommunityValidator ValidatorType = 0x02  // 社区验证者
)

// Validator 验证者信息
type Validator struct {
    Address       common.Address  // 验证者地址
    Type          ValidatorType   // 验证者类型
    PublicKey     *ecdsa.PublicKey // 投票公钥
    JoinedAt      time.Time       // 加入时间
    StakedAmount  *big.Int        // 质押数量 (仅社区验证者)
    NodeUptime    time.Duration   // 节点运行时间
    SGXVerified   bool            // SGX 硬件验证状态
    VotingPower   uint64          // 投票权重
}

// CoreValidatorConfig 核心验证者配置
type CoreValidatorConfig struct {
    MinMembers       int     // 最小成员数 (默认 5)
    MaxMembers       int     // 最大成员数 (默认 7)
    QuorumThreshold  float64 // 法定人数阈值 (默认 2/3)
}

// CommunityValidatorConfig 社区验证者配置
type CommunityValidatorConfig struct {
    MinUptime        time.Duration // 最小运行时间 (默认 30 天)
    MinStake         *big.Int      // 最小质押量 (初始值 10000 X，从 GovernanceContract 读取)
    VetoThreshold    float64       // 否决阈值 (默认 1/3)
}

// DefaultCoreValidatorConfig 默认核心验证者配置
func DefaultCoreValidatorConfig() *CoreValidatorConfig {
    return &CoreValidatorConfig{
        MinMembers:      5,
        MaxMembers:      7,
        QuorumThreshold: 0.667, // 2/3
    }
}

// DefaultCommunityValidatorConfig 默认社区验证者配置
// 注意：这些是创世区块的初始值，实际值从 GovernanceContract 中读取
func DefaultCommunityValidatorConfig() *CommunityValidatorConfig {
    return &CommunityValidatorConfig{
        MinUptime:     30 * 24 * time.Hour, // 30 天
        MinStake:      new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e18)),   // 初始值：10000 X（从 GovernanceContract 读取）
        VetoThreshold: 0.334,               // 1/3
    }
}
```

**白名单更新提案与投票流程：**

```
白名单更新流程
==============

1. 提案阶段
   ├── 核心验证者提交 MRENCLAVE 更新提案
   ├── 提案内容: 新 MRENCLAVE、版本说明、审计报告链接
   └── 提案进入公示期

2. 核心验证者投票阶段 (3 天)
   ├── 核心验证者审查代码和审计报告
   ├── 核心验证者投票 (同意/反对/弃权)
   └── 需要 2/3 多数同意才能进入下一阶段

3. 社区公示与否决阶段 (7 天)
   ├── 提案向全网公示
   ├── 社区验证者可以投否决票
   ├── 如果 > 1/3 社区验证者否决，提案被拒绝
   └── 公示期结束且未被否决，提案通过

4. 生效阶段
   ├── 新 MRENCLAVE 加入白名单
   ├── 节点可以开始升级
   └── 数据迁移通道开启

紧急升级流程 (安全漏洞修复)
===========================

1. 紧急提案
   ├── 需要 100% 核心验证者同意
   ├── 必须附带安全漏洞详情和修复说明
   └── 只能用于安全修复，不能添加新功能

2. 快速公示期 (24 小时)
   ├── 社区验证者仍有否决权
   └── 否决阈值提高到 1/2

3. 立即生效
   └── 公示期结束后立即生效
```

**投票合约实现：**

```go
// governance/whitelist_governance.go
package governance

import (
    "errors"
    "sync"
    "time"
    
    "github.com/ethereum/go-ethereum/common"
)

// ProposalType 提案类型
type ProposalType uint8

const (
    ProposalAddMREnclave      ProposalType = 0x01 // 添加 MRENCLAVE
    ProposalRemoveMREnclave   ProposalType = 0x02 // 移除 MRENCLAVE
    ProposalUpgradePermission ProposalType = 0x03 // 升级权限
    ProposalAddValidator      ProposalType = 0x04 // 添加验证者
    ProposalRemoveValidator   ProposalType = 0x05 // 移除验证者
    ProposalParameterChange   ProposalType = 0x06 // 参数修改
    ProposalNormalUpgrade     ProposalType = 0x07 // 普通升级
    ProposalEmergencyUpgrade  ProposalType = 0x08 // 紧急升级（安全漏洞修复）
)

// 升级提案的投票规则：
// 1. 普通升级（ProposalNormalUpgrade）：
//    - 核心验证者：需要 2/3 通过
//    - 社区验证者：可以行使否决权，1/3 否决即可拒绝提案
// 2. 紧急升级（ProposalEmergencyUpgrade）：
//    - 核心验证者：需要 100% 通过
//    - 社区验证者：否决权阈值提高到 1/2（更高的否决门槛）
//    - 必须附带安全漏洞详情和修复说明

// ProposalStatus 提案状态
type ProposalStatus uint8

const (
    ProposalPending      ProposalStatus = 0x00  // 待投票
    ProposalCoreVoting   ProposalStatus = 0x01  // 核心验证者投票中
    ProposalPublicReview ProposalStatus = 0x02  // 社区公示中
    ProposalApproved     ProposalStatus = 0x03  // 已通过
    ProposalRejected     ProposalStatus = 0x04  // 已拒绝
    ProposalExpired      ProposalStatus = 0x05  // 已过期
)

// WhitelistProposal MRENCLAVE 白名单更新提案
type WhitelistProposal struct {
    ID              common.Hash     // 提案 ID
    Proposer        common.Address  // 提案者
    Type            ProposalType    // 提案类型
    NewMREnclave    []byte          // 新的 MRENCLAVE
    VersionInfo     string          // 版本说明
    AuditReportURL  string          // 审计报告链接
    CreatedAt       time.Time       // 创建时间
    Status          ProposalStatus  // 当前状态
    
    // 核心验证者投票
    CoreVotes       map[common.Address]bool  // true=同意, false=反对
    CoreVoteDeadline time.Time               // 核心投票截止时间
    
    // 社区验证者否决
    CommunityVetos  map[common.Address]bool  // true=否决
    PublicReviewEnd time.Time                // 公示期结束时间
}

// WhitelistGovernance 白名单治理合约
type WhitelistGovernance struct {
    mu sync.RWMutex
    
    config          *GovernanceConfig             // 治理配置（从 GovernanceContract 读取）
    coreConfig      *CoreValidatorConfig
    communityConfig *CommunityValidatorConfig
    
    coreValidators      map[common.Address]*Validator
    communityValidators map[common.Address]*Validator
    
    proposals map[common.Hash]*WhitelistProposal
    whitelist map[string]bool  // MRENCLAVE 白名单
}

// GovernanceConfig 治理配置
// 所有配置参数存储在 GovernanceContract 中，可以通过投票机制修改
type GovernanceConfig struct {
    // 核心验证者投票阈值（百分比，默认 67 表示 2/3）
    CoreValidatorThreshold uint64
    
    // 社区验证者投票阈值（百分比，默认 51 表示简单多数）
    CommunityValidatorThreshold uint64
    
    // 投票期限（区块数，默认 40320 ≈ 7天）
    VotingPeriod uint64
    
    // 执行延迟（区块数，默认 5760 ≈ 1天）
    ExecutionDelay uint64
    
    // 最小投票参与率（百分比，默认 50%）
    MinParticipation uint64
}

// DefaultGovernanceConfig 默认治理配置
// 注意：这些是创世区块的初始值，实际值从 GovernanceContract 中读取
func DefaultGovernanceConfig() *GovernanceConfig {
    return &GovernanceConfig{
        CoreValidatorThreshold:      67,    // 2/3 核心验证者
        CommunityValidatorThreshold: 51,    // 简单多数社区验证者
        VotingPeriod:                40320, // 约 7 天（按 15 秒/块计算）
        ExecutionDelay:              5760,  // 约 1 天
        MinParticipation:            50,    // 50% 参与率
    }
}

// NewWhitelistGovernance 创建白名单治理实例
func NewWhitelistGovernance(
    config *GovernanceConfig,
    coreConfig *CoreValidatorConfig,
    communityConfig *CommunityValidatorConfig,
) *WhitelistGovernance {
    return &WhitelistGovernance{
        config:              config,
        coreConfig:          coreConfig,
        communityConfig:     communityConfig,
        coreValidators:      make(map[common.Address]*Validator),
        communityValidators: make(map[common.Address]*Validator),
        proposals:           make(map[common.Hash]*WhitelistProposal),
        whitelist:           make(map[string]bool),
    }
}

// SubmitProposal 提交白名单更新提案
func (g *WhitelistGovernance) SubmitProposal(
    proposer common.Address,
    proposalType ProposalType,
    newMREnclave []byte,
    versionInfo string,
    auditReportURL string,
) (*WhitelistProposal, error) {
    g.mu.Lock()
    defer g.mu.Unlock()
    
    // 验证提案者是核心验证者
    if _, ok := g.coreValidators[proposer]; !ok {
        return nil, errors.New("only core validators can submit proposals")
    }
    
    // 创建提案
    proposal := &WhitelistProposal{
        ID:              common.BytesToHash(newMREnclave),
        Proposer:        proposer,
        Type:            proposalType,
        NewMREnclave:    newMREnclave,
        VersionInfo:     versionInfo,
        AuditReportURL:  auditReportURL,
        CreatedAt:       time.Now(),
        Status:          ProposalCoreVoting,
        CoreVotes:       make(map[common.Address]bool),
        CommunityVetos:  make(map[common.Address]bool),
    }
    
    // 设置投票截止时间
    if proposalType == ProposalEmergencyUpgrade {
        proposal.CoreVoteDeadline = time.Now().Add(6 * time.Hour)
        proposal.PublicReviewEnd = time.Now().Add(24 * time.Hour)
    } else {
        proposal.CoreVoteDeadline = time.Now().Add(3 * 24 * time.Hour)
        proposal.PublicReviewEnd = time.Now().Add(10 * 24 * time.Hour) // 3天投票 + 7天公示
    }
    
    g.proposals[proposal.ID] = proposal
    return proposal, nil
}

// CoreVote 核心验证者投票
func (g *WhitelistGovernance) CoreVote(
    proposalID common.Hash,
    voter common.Address,
    approve bool,
    signature []byte,
) error {
    g.mu.Lock()
    defer g.mu.Unlock()
    
    proposal, ok := g.proposals[proposalID]
    if !ok {
        return errors.New("proposal not found")
    }
    
    if proposal.Status != ProposalCoreVoting {
        return errors.New("proposal not in core voting phase")
    }
    
    if time.Now().After(proposal.CoreVoteDeadline) {
        return errors.New("core voting period ended")
    }
    
    // 验证投票者是核心验证者
    validator, ok := g.coreValidators[voter]
    if !ok || validator.Type != CoreValidator {
        return errors.New("only core validators can vote")
    }
    
    // TODO: 验证签名
    
    // 记录投票
    proposal.CoreVotes[voter] = approve
    
    // 检查是否达到法定人数
    g.checkCoreVotingResult(proposal)
    
    return nil
}

// checkCoreVotingResult 检查核心投票结果
func (g *WhitelistGovernance) checkCoreVotingResult(proposal *WhitelistProposal) {
    totalCoreValidators := len(g.coreValidators)
    approveCount := 0
    rejectCount := 0
    
    for _, approve := range proposal.CoreVotes {
        if approve {
            approveCount++
        } else {
            rejectCount++
        }
    }
    
    // 紧急升级需要 100% 同意
    if proposal.Type == ProposalEmergencyUpgrade {
        if approveCount == totalCoreValidators {
            proposal.Status = ProposalPublicReview
        } else if rejectCount > 0 {
            proposal.Status = ProposalRejected
        }
        return
    }
    
    // 普通升级需要 2/3 同意
    threshold := int(float64(totalCoreValidators) * g.coreConfig.QuorumThreshold)
    if approveCount >= threshold {
        proposal.Status = ProposalPublicReview
    } else if rejectCount > totalCoreValidators-threshold {
        // 反对票已经足够阻止通过
        proposal.Status = ProposalRejected
    }
}

// CommunityVeto 社区验证者否决
func (g *WhitelistGovernance) CommunityVeto(
    proposalID common.Hash,
    voter common.Address,
    signature []byte,
) error {
    g.mu.Lock()
    defer g.mu.Unlock()
    
    proposal, ok := g.proposals[proposalID]
    if !ok {
        return errors.New("proposal not found")
    }
    
    if proposal.Status != ProposalPublicReview {
        return errors.New("proposal not in public review phase")
    }
    
    if time.Now().After(proposal.PublicReviewEnd) {
        return errors.New("public review period ended")
    }
    
    // 验证投票者是社区验证者
    validator, ok := g.communityValidators[voter]
    if !ok || validator.Type != CommunityValidator {
        return errors.New("only community validators can veto")
    }
    
    // TODO: 验证签名
    
    // 记录否决
    proposal.CommunityVetos[voter] = true
    
    // 检查是否达到否决阈值
    g.checkCommunityVetoResult(proposal)
    
    return nil
}

// checkCommunityVetoResult 检查社区否决结果
func (g *WhitelistGovernance) checkCommunityVetoResult(proposal *WhitelistProposal) {
    totalCommunityValidators := len(g.communityValidators)
    vetoCount := len(proposal.CommunityVetos)
    
    // 确定否决阈值
    var threshold float64
    if proposal.Type == ProposalEmergencyUpgrade {
        threshold = 0.5  // 紧急升级需要 1/2 否决
    } else {
        threshold = g.communityConfig.VetoThreshold  // 普通升级需要 1/3 否决
    }
    
    vetoThreshold := int(float64(totalCommunityValidators) * threshold)
    if vetoCount > vetoThreshold {
        proposal.Status = ProposalRejected
    }
}

// FinalizeProposal 完成提案（公示期结束后调用）
func (g *WhitelistGovernance) FinalizeProposal(proposalID common.Hash) error {
    g.mu.Lock()
    defer g.mu.Unlock()
    
    proposal, ok := g.proposals[proposalID]
    if !ok {
        return errors.New("proposal not found")
    }
    
    if proposal.Status != ProposalPublicReview {
        return errors.New("proposal not in public review phase")
    }
    
    if time.Now().Before(proposal.PublicReviewEnd) {
        return errors.New("public review period not ended")
    }
    
    // 提案通过，加入白名单
    proposal.Status = ProposalApproved
    g.whitelist[string(proposal.NewMREnclave)] = true
    
    return nil
}

// IsWhitelisted 检查 MRENCLAVE 是否在白名单中
func (g *WhitelistGovernance) IsWhitelisted(mrenclave []byte) bool {
    g.mu.RLock()
    defer g.mu.RUnlock()
    return g.whitelist[string(mrenclave)]
}
```

**防止女巫攻击：**

社区验证者的准入条件设计用于防止女巫攻击：

| 防护措施 | 作用 | 攻击成本 |
|----------|------|----------|
| 最小运行时间 30 天 | 攻击者需要提前部署节点 | 时间成本 |
| 最小质押 10,000 X | 攻击者需要大量资金 | 资金成本 |
| SGX 硬件验证 | 每个物理 CPU 只能注册一个验证者 | 硬件成本 |
| 历史行为记录 | 恶意行为会被记录并禁止 | 声誉成本 |

```go
// governance/sybil_protection.go
package governance

import (
    "errors"
    "time"
    
    "github.com/ethereum/go-ethereum/common"
)

// SybilProtection 女巫攻击防护
type SybilProtection struct {
    // SGX 硬件 ID 到验证者地址的映射（防止同一硬件注册多个验证者）
    hardwareToValidator map[string]common.Address
    
    // 验证者黑名单
    blacklist map[common.Address]time.Time
}

// RegisterCommunityValidator 注册社区验证者
func (s *SybilProtection) RegisterCommunityValidator(
    address common.Address,
    sgxQuote []byte,
    stakeAmount *big.Int,
    nodeUptime time.Duration,
    config *CommunityValidatorConfig,
) error {
    // 1. 检查是否在黑名单中
    if banTime, ok := s.blacklist[address]; ok {
        if time.Now().Before(banTime) {
            return errors.New("address is blacklisted")
        }
        delete(s.blacklist, address)
    }
    
    // 2. 验证 SGX Quote 并提取硬件 ID
    hardwareID, err := extractHardwareID(sgxQuote)
    if err != nil {
        return errors.New("invalid SGX quote")
    }
    
    // 3. 检查硬件是否已被其他验证者使用
    if existingValidator, ok := s.hardwareToValidator[hardwareID]; ok {
        if existingValidator != address {
            return errors.New("hardware already registered by another validator")
        }
    }
    
    // 4. 检查质押数量
    if stakeAmount.Cmp(config.MinStake) < 0 {
        return errors.New("insufficient stake amount")
    }
    
    // 5. 检查节点运行时间
    if nodeUptime < config.MinUptime {
        return errors.New("insufficient node uptime")
    }
    
    // 6. 注册成功
    s.hardwareToValidator[hardwareID] = address
    return nil
}

// extractHardwareID 从 SGX Quote 中提取硬件唯一标识
func extractHardwareID(sgxQuote []byte) (string, error) {
    // 从 Quote 中提取 EPID 或 DCAP 硬件标识
    // 这个标识对于每个物理 CPU 是唯一的
    // TODO: 实现具体的提取逻辑
    return "", nil
}
```

**投票人列表链上记录与迁移前置条件：**

为防止恶意节点自己运营多个节点进行投票并导出数据，X Chain 在每个区块中记录投票人列表快照，并在数据迁移前强制验证投票合法性。

```
┌─────────────────────────────────────────────────────────────────────────┐
│  核心安全原则: 没有合法投票和合法共识，用户秘密数据不允许迁移             │
│                                                                         │
│  1. 投票人列表链上记录 - 每个区块记录当前投票人列表快照                   │
│  2. 投票完整性验证 - 迁移前验证投票是否达到法定人数                       │
│  3. SGX 内部强制执行 - enclave 代码强制检查投票合法性                    │
│  4. 可追溯性 - 任何人都能验证"该投票的人是否投了票"                      │
└─────────────────────────────────────────────────────────────────────────┘
```

```go
// governance/voter_list_snapshot.go
package governance

import (
    "crypto/sha256"
    "encoding/binary"
    "time"
    
    "github.com/ethereum/go-ethereum/common"
)

// VoterListSnapshot 投票人列表快照
type VoterListSnapshot struct {
    BlockNumber     uint64                      // 区块高度
    BlockHash       common.Hash                 // 区块哈希
    Timestamp       uint64                      // 时间戳
    
    // 核心验证者列表
    CoreValidators  []ValidatorInfo             // 核心验证者信息
    CoreListRoot    common.Hash                 // 核心验证者列表 Merkle 根
    
    // 社区验证者列表
    CommunityValidators []ValidatorInfo         // 社区验证者信息
    CommunityListRoot   common.Hash             // 社区验证者列表 Merkle 根
    
    // 综合根哈希（记录在区块头中）
    VoterListRoot   common.Hash                 // 投票人列表综合 Merkle 根
}

// ValidatorInfo 验证者信息（用于快照）
type ValidatorInfo struct {
    Address       common.Address  // 验证者地址
    PublicKey     []byte          // 投票公钥
    VotingPower   uint64          // 投票权重
    JoinedBlock   uint64          // 加入时的区块高度
    SGXQuoteHash  common.Hash     // SGX Quote 哈希（用于验证硬件唯一性）
}

// BlockHeaderExtension 区块头扩展字段
type BlockHeaderExtension struct {
    // 原有字段...
    
    // 新增：投票人列表根哈希
    VoterListRoot common.Hash  // 当前区块生成时的投票人列表 Merkle 根
}

// CalculateVoterListRoot 计算投票人列表根哈希
func CalculateVoterListRoot(snapshot *VoterListSnapshot) common.Hash {
    // 1. 计算核心验证者列表 Merkle 根
    coreRoot := calculateValidatorListRoot(snapshot.CoreValidators)
    
    // 2. 计算社区验证者列表 Merkle 根
    communityRoot := calculateValidatorListRoot(snapshot.CommunityValidators)
    
    // 3. 组合计算综合根
    combined := append(coreRoot[:], communityRoot[:]...)
    hash := sha256.Sum256(combined)
    return common.BytesToHash(hash[:])
}

// calculateValidatorListRoot 计算验证者列表 Merkle 根
func calculateValidatorListRoot(validators []ValidatorInfo) common.Hash {
    if len(validators) == 0 {
        return common.Hash{}
    }
    
    // 构建 Merkle 树
    leaves := make([]common.Hash, len(validators))
    for i, v := range validators {
        leaves[i] = hashValidatorInfo(&v)
    }
    
    return buildMerkleRoot(leaves)
}
```

**投票记录链上存储：**

```go
// governance/vote_record.go
package governance

// VoteRecord 投票记录（存储在链上）
type VoteRecord struct {
    ProposalID      common.Hash     // 提案 ID
    Voter           common.Address  // 投票者地址
    VoteType        VoteType        // 投票类型
    Timestamp       uint64          // 投票时间戳
    BlockNumber     uint64          // 投票所在区块
    Signature       []byte          // 投票签名
    
    // 投票时的投票人列表快照引用
    VoterListRoot   common.Hash     // 投票时的投票人列表根哈希
}

type VoteType uint8

const (
    VoteApprove VoteType = 0x01  // 同意
    VoteReject  VoteType = 0x02  // 反对
    VoteAbstain VoteType = 0x03  // 弃权
)

// ProposalVotingState 提案投票状态（链上存储）
type ProposalVotingState struct {
    ProposalID          common.Hash             // 提案 ID
    VoterListSnapshot   common.Hash             // 投票开始时的投票人列表快照
    ExpectedVoters      []common.Address        // 应该投票的验证者列表
    ActualVotes         map[common.Address]*VoteRecord  // 实际投票记录
    
    // 投票统计
    ApproveCount        int                     // 同意票数
    RejectCount         int                     // 反对票数
    AbstainCount        int                     // 弃权票数
    NotVotedCount       int                     // 未投票数
    
    // 状态
    IsComplete          bool                    // 投票是否完成
    IsValid             bool                    // 投票是否有效（达到法定人数）
}

// GetNotVotedValidators 获取未投票的验证者列表
func (s *ProposalVotingState) GetNotVotedValidators() []common.Address {
    notVoted := make([]common.Address, 0)
    for _, voter := range s.ExpectedVoters {
        if _, ok := s.ActualVotes[voter]; !ok {
            notVoted = append(notVoted, voter)
        }
    }
    return notVoted
}

// ValidateVotingResult 验证投票结果是否合法
func (s *ProposalVotingState) ValidateVotingResult(threshold float64) error {
    totalExpected := len(s.ExpectedVoters)
    if totalExpected == 0 {
        return errors.New("no expected voters")
    }
    
    // 检查是否达到法定人数
    requiredVotes := int(float64(totalExpected) * threshold)
    if s.ApproveCount < requiredVotes {
        return fmt.Errorf("insufficient votes: got %d, need %d (%.0f%% of %d)",
            s.ApproveCount, requiredVotes, threshold*100, totalExpected)
    }
    
    // 检查所有投票是否来自预期的投票人列表
    for voter := range s.ActualVotes {
        found := false
        for _, expected := range s.ExpectedVoters {
            if voter == expected {
                found = true
                break
            }
        }
        if !found {
            return fmt.Errorf("vote from unexpected voter: %s", voter.Hex())
        }
    }
    
    return nil
}
```

**迁移前置条件验证（SGX enclave 内部强制执行）：**

```go
// keystore/migration_precondition.go
package keystore

import (
    "errors"
    "fmt"
    
    "github.com/ethereum/go-ethereum/common"
)

// MigrationPrecondition 迁移前置条件验证器
// 注意：此代码在 SGX enclave 内部运行，无法被绕过
type MigrationPrecondition struct {
    governance      *WhitelistGovernance
    chainReader     ChainReader
}

// MigrationRequest 迁移请求
type MigrationRequest struct {
    TargetMREnclave []byte          // 目标 MRENCLAVE
    SourceNodeID    common.Hash     // 源节点 ID
    TargetNodeID    common.Hash     // 目标节点 ID
    KeyIDs          []common.Hash   // 要迁移的密钥 ID 列表
}

// ValidateMigrationRequest 验证迁移请求是否合法
// 此函数在 SGX enclave 内部执行，确保无法绕过
func (p *MigrationPrecondition) ValidateMigrationRequest(req *MigrationRequest) error {
    // 1. 验证目标 MRENCLAVE 是否在白名单中
    if !p.governance.IsWhitelisted(req.TargetMREnclave) {
        return errors.New("target MRENCLAVE not in whitelist")
    }
    
    // 2. 获取将目标 MRENCLAVE 加入白名单的提案
    proposal, err := p.governance.GetApprovedProposal(req.TargetMREnclave)
    if err != nil {
        return fmt.Errorf("failed to get approved proposal: %w", err)
    }
    
    // 3. 验证投票是否合法
    votingState, err := p.governance.GetProposalVotingState(proposal.ID)
    if err != nil {
        return fmt.Errorf("failed to get voting state: %w", err)
    }
    
    // 4. 验证投票完整性
    if err := p.validateVotingIntegrity(votingState, proposal); err != nil {
        return fmt.Errorf("voting integrity check failed: %w", err)
    }
    
    // 5. 验证投票人列表快照
    if err := p.validateVoterListSnapshot(votingState); err != nil {
        return fmt.Errorf("voter list snapshot validation failed: %w", err)
    }
    
    return nil
}

// validateVotingIntegrity 验证投票完整性
func (p *MigrationPrecondition) validateVotingIntegrity(
    votingState *ProposalVotingState,
    proposal *WhitelistProposal,
) error {
    // 1. 检查投票是否完成
    if !votingState.IsComplete {
        return errors.New("voting not complete")
    }
    
    // 2. 检查投票是否有效
    if !votingState.IsValid {
        return errors.New("voting result not valid")
    }
    
    // 3. 验证投票结果（核心验证者 2/3 同意）
    threshold := 0.67  // 2/3
    if err := votingState.ValidateVotingResult(threshold); err != nil {
        return err
    }
    
    // 4. 检查是否有未投票的核心验证者（警告，但不阻止）
    notVoted := votingState.GetNotVotedValidators()
    if len(notVoted) > 0 {
        // 记录警告日志，但只要达到 2/3 就允许迁移
        logWarning("validators did not vote: %v", notVoted)
    }
    
    return nil
}

// validateVoterListSnapshot 验证投票人列表快照
func (p *MigrationPrecondition) validateVoterListSnapshot(
    votingState *ProposalVotingState,
) error {
    // 1. 获取投票时的投票人列表快照
    snapshotRoot := votingState.VoterListSnapshot
    
    // 2. 从链上获取该快照对应的区块
    snapshot, err := p.chainReader.GetVoterListSnapshot(snapshotRoot)
    if err != nil {
        return fmt.Errorf("failed to get voter list snapshot: %w", err)
    }
    
    // 3. 验证快照的 Merkle 根
    calculatedRoot := CalculateVoterListRoot(snapshot)
    if calculatedRoot != snapshotRoot {
        return errors.New("voter list snapshot root mismatch")
    }
    
    // 4. 验证所有投票者都在快照的投票人列表中
    for voter := range votingState.ActualVotes {
        if !isValidatorInSnapshot(voter, snapshot) {
            return fmt.Errorf("voter %s not in snapshot", voter.Hex())
        }
    }
    
    return nil
}

// isValidatorInSnapshot 检查验证者是否在快照中
func isValidatorInSnapshot(voter common.Address, snapshot *VoterListSnapshot) bool {
    for _, v := range snapshot.CoreValidators {
        if v.Address == voter {
            return true
        }
    }
    for _, v := range snapshot.CommunityValidators {
        if v.Address == voter {
            return true
        }
    }
    return false
}
```

**投票透明性查询接口：**

```go
// governance/voting_transparency.go
package governance

// VotingTransparencyQuery 投票透明性查询
type VotingTransparencyQuery struct {
    chainReader ChainReader
    governance  *WhitelistGovernance
}

// QueryVotingStatus 查询投票状态（任何人都可以调用）
func (q *VotingTransparencyQuery) QueryVotingStatus(proposalID common.Hash) (*VotingStatusReport, error) {
    votingState, err := q.governance.GetProposalVotingState(proposalID)
    if err != nil {
        return nil, err
    }
    
    return &VotingStatusReport{
        ProposalID:       proposalID,
        TotalExpected:    len(votingState.ExpectedVoters),
        TotalVoted:       len(votingState.ActualVotes),
        ApproveCount:     votingState.ApproveCount,
        RejectCount:      votingState.RejectCount,
        AbstainCount:     votingState.AbstainCount,
        NotVotedCount:    votingState.NotVotedCount,
        NotVotedList:     votingState.GetNotVotedValidators(),
        IsComplete:       votingState.IsComplete,
        IsValid:          votingState.IsValid,
        VoterListRoot:    votingState.VoterListSnapshot,
    }, nil
}

// VotingStatusReport 投票状态报告
type VotingStatusReport struct {
    ProposalID       common.Hash
    TotalExpected    int                 // 应该投票的总人数
    TotalVoted       int                 // 实际投票人数
    ApproveCount     int                 // 同意票数
    RejectCount      int                 // 反对票数
    AbstainCount     int                 // 弃权票数
    NotVotedCount    int                 // 未投票人数
    NotVotedList     []common.Address    // 未投票的验证者列表
    IsComplete       bool                // 投票是否完成
    IsValid          bool                // 投票是否有效
    VoterListRoot    common.Hash         // 投票人列表快照根
}

// VerifyVoteLegitimacy 验证投票合法性（任何人都可以调用）
func (q *VotingTransparencyQuery) VerifyVoteLegitimacy(proposalID common.Hash) (*LegitimacyReport, error) {
    votingState, err := q.governance.GetProposalVotingState(proposalID)
    if err != nil {
        return nil, err
    }
    
    report := &LegitimacyReport{
        ProposalID: proposalID,
        Checks:     make([]LegitimacyCheck, 0),
    }
    
    // 检查 1: 投票人数是否达到法定人数
    threshold := 0.67
    requiredVotes := int(float64(len(votingState.ExpectedVoters)) * threshold)
    check1 := LegitimacyCheck{
        Name:     "QuorumCheck",
        Passed:   votingState.ApproveCount >= requiredVotes,
        Details:  fmt.Sprintf("需要 %d 票，实际 %d 票", requiredVotes, votingState.ApproveCount),
    }
    report.Checks = append(report.Checks, check1)
    
    // 检查 2: 所有投票是否来自预期的投票人列表
    allVotersValid := true
    for voter := range votingState.ActualVotes {
        found := false
        for _, expected := range votingState.ExpectedVoters {
            if voter == expected {
                found = true
                break
            }
        }
        if !found {
            allVotersValid = false
            break
        }
    }
    check2 := LegitimacyCheck{
        Name:     "VoterValidityCheck",
        Passed:   allVotersValid,
        Details:  "所有投票者都在预期的投票人列表中",
    }
    report.Checks = append(report.Checks, check2)
    
    // 检查 3: 投票人列表快照是否有效
    snapshot, err := q.chainReader.GetVoterListSnapshot(votingState.VoterListSnapshot)
    snapshotValid := err == nil && snapshot != nil
    check3 := LegitimacyCheck{
        Name:     "SnapshotValidityCheck",
        Passed:   snapshotValid,
        Details:  fmt.Sprintf("投票人列表快照根: %s", votingState.VoterListSnapshot.Hex()),
    }
    report.Checks = append(report.Checks, check3)
    
    // 综合结果
    report.IsLegitimate = check1.Passed && check2.Passed && check3.Passed
    
    return report, nil
}

// LegitimacyReport 合法性报告
type LegitimacyReport struct {
    ProposalID   common.Hash
    IsLegitimate bool
    Checks       []LegitimacyCheck
}

// LegitimacyCheck 合法性检查项
type LegitimacyCheck struct {
    Name    string
    Passed  bool
    Details string
}
```

**安全保障总结：**

| 攻击场景 | 防护机制 | 效果 |
|----------|----------|------|
| 恶意节点自己运营多个节点投票 | SGX 硬件唯一性验证 + 投票人列表链上记录 | 每个物理 CPU 只能注册一个验证者 |
| 伪造投票记录 | 投票签名 + 链上存储 | 投票不可伪造、不可篡改 |
| 绕过投票直接迁移数据 | SGX enclave 内部强制验证 | 代码在 TEE 中运行，无法绕过 |
| 投票人列表被篡改 | Merkle 根记录在区块头 | 任何篡改都会被检测到 |
| 少数节点自己投票通过 | 2/3 法定人数要求 + 透明查询 | 任何人都能查看"谁没投票" |

#### 6.1.0.0.3 自动密钥迁移机制

硬分叉升级时，密钥迁移自动进行，无需用户授权。这简化了升级流程，提高了用户体验。

**设计原则：**

```
┌─────────────────────────────────────────────────────────────────────────┐
│  核心原则: 信任治理机制，简化用户体验                                     │
│                                                                         │
│  密钥迁移自动进行，因为:                                                 │
│  - 2/3 核心验证者投票 + 社区验证者否决权已提供足够安全保障               │
│  - 渐进式权限机制限制了恶意代码的破坏能力                                │
│  - 用户无需理解复杂的技术细节即可安全使用                                │
│  - 避免因用户不操作导致密钥无法迁移的问题                                │
└─────────────────────────────────────────────────────────────────────────┘
```

**自动迁移机制：**

```go
// keystore/auto_migration.go
package keystore

import (
    "context"
    "time"
    
    "github.com/ethereum/go-ethereum/common"
)

// AutoMigrationConfig 自动迁移配置
type AutoMigrationConfig struct {
    // 迁移批次大小
    BatchSize int
    
    // 迁移间隔（避免一次性迁移过多密钥）
    MigrationInterval time.Duration
    
    // 重试次数
    MaxRetries int
    
    // 重试间隔
    RetryInterval time.Duration
}

// DefaultAutoMigrationConfig 默认配置
func DefaultAutoMigrationConfig() *AutoMigrationConfig {
    return &AutoMigrationConfig{
        BatchSize:         100,
        MigrationInterval: 1 * time.Second,
        MaxRetries:        3,
        RetryInterval:     5 * time.Second,
    }
}

// AutoMigrationManager 自动迁移管理器
type AutoMigrationManager struct {
    config          *AutoMigrationConfig
    ratls           *RATLSTransport
    permissionMgr   *PermissionManager  // 渐进式权限管理器
}

// MigrateAllKeys 自动迁移所有密钥
func (m *AutoMigrationManager) MigrateAllKeys(
    ctx context.Context,
    sourceMREnclave []byte,
    targetMREnclave []byte,
) (*MigrationResult, error) {
    // 1. 验证目标 MRENCLAVE 在白名单中
    if !m.isWhitelisted(targetMREnclave) {
        return nil, errors.New("target MRENCLAVE not in whitelist")
    }
    
    // 2. 检查渐进式权限限制
    permLevel := m.permissionMgr.GetPermissionLevel(targetMREnclave)
    dailyLimit := m.getDailyMigrationLimit(permLevel)
    
    // 3. 获取所有需要迁移的密钥
    keys, err := m.getAllKeys()
    if err != nil {
        return nil, err
    }
    
    // 4. 按批次迁移
    result := &MigrationResult{
        TotalKeys:     len(keys),
        MigratedKeys:  0,
        FailedKeys:    0,
        SkippedKeys:   0,
    }
    
    migratedToday := m.getMigratedCountToday()
    
    for i := 0; i < len(keys); i += m.config.BatchSize {
        // 检查每日限制
        if migratedToday >= dailyLimit {
            result.SkippedKeys = len(keys) - i
            result.Message = "达到每日迁移限制，剩余密钥将在明天继续迁移"
            break
        }
        
        end := i + m.config.BatchSize
        if end > len(keys) {
            end = len(keys)
        }
        
        batch := keys[i:end]
        
        // 迁移当前批次
        for _, key := range batch {
            if migratedToday >= dailyLimit {
                break
            }
            
            err := m.migrateKey(ctx, key, sourceMREnclave, targetMREnclave)
            if err != nil {
                result.FailedKeys++
                // 记录失败，稍后重试
                m.recordFailedMigration(key, err)
            } else {
                result.MigratedKeys++
                migratedToday++
            }
        }
        
        // 批次间隔
        time.Sleep(m.config.MigrationInterval)
    }
    
    return result, nil
}

// getDailyMigrationLimit 根据权限级别获取每日迁移限制
func (m *AutoMigrationManager) getDailyMigrationLimit(level PermissionLevel) int {
    switch level {
    case PermissionBasic:
        return 10    // 基础权限: 每天 10 个
    case PermissionStandard:
        return 100   // 标准权限: 每天 100 个
    case PermissionFull:
        return -1    // 完全权限: 无限制
    default:
        return 0
    }
}

// MigrationResult 迁移结果
type MigrationResult struct {
    TotalKeys    int
    MigratedKeys int
    FailedKeys   int
    SkippedKeys  int
    Message      string
}

// migrateKey 迁移单个密钥
func (m *AutoMigrationManager) migrateKey(
    ctx context.Context,
    key *KeyInfo,
    sourceMREnclave []byte,
    targetMREnclave []byte,
) error {
    // 1. 通过 RA-TLS 连接到旧版本节点
    conn, err := m.ratls.Connect(key.SourceNode)
    if err != nil {
        return err
    }
    defer conn.Close()
    
    // 2. 请求密钥数据（在 RA-TLS 通道中传输）
    keyData, err := m.requestKeyData(conn, key.ID)
    if err != nil {
        return err
    }
    
    // 3. 使用新 MRENCLAVE 重新封装
    err = m.sealWithNewMREnclave(keyData, targetMREnclave)
    if err != nil {
        return err
    }
    
    // 4. 记录迁移完成
    m.recordMigrationComplete(key.ID, sourceMREnclave, targetMREnclave)
    
    return nil
}
```

**自动迁移流程：**

```
自动密钥迁移流程
================

1. 升级触发
   ├── 新版本 MRENCLAVE 通过 2/3 投票加入白名单
   ├── 社区验证者否决期结束（无否决）
   └── 新版本节点开始运行

2. 自动迁移启动
   ├── 新版本节点检测到需要迁移的密钥
   ├── 验证自身 MRENCLAVE 在白名单中
   ├── 检查当前权限级别和每日限制
   └── 开始批量迁移

3. 密钥迁移
   ├── 建立 RA-TLS 安全通道到旧版本节点
   ├── 旧版本节点解封密钥数据
   ├── 通过 RA-TLS 传输到新版本节点
   └── 新版本节点重新封装

4. 迁移完成
   ├── 记录迁移日志
   ├── 更新密钥元数据
   └── 用户无感知，继续正常使用

渐进式迁移限制
==============

Day 0-7 (基础权限期):
├── 每天最多迁移 10 个密钥
├── 大量密钥需要多天完成迁移
└── 给社区时间发现问题

Day 7-30 (标准权限期):
├── 每天最多迁移 100 个密钥
└── 加速迁移进度

Day 30+ (完全权限):
├── 无迁移限制
└── 可一次性完成所有迁移
```

**安全保证：**

| 攻击场景 | 防护机制 |
|----------|----------|
| 恶意升级 | 2/3 核心验证者投票 + 社区否决权 |
| 快速大规模泄露 | 渐进式权限限制每日迁移数量 |
| 中间人攻击 | RA-TLS 端到端加密 |
| 未授权迁移 | 只有白名单中的 MRENCLAVE 可接收密钥 |

**与渐进式权限的配合：**

自动迁移机制与渐进式权限机制紧密配合，即使恶意代码通过投票加入白名单：
1. 前 7 天每天只能迁移 10 个密钥，大规模泄露需要很长时间
2. 社区有充足时间发现异常并发起否决/回滚
3. 迁移日志公开透明，异常行为容易被发现

#### 6.1.0.0.4 渐进式权限机制

新版本代码在获得白名单批准后，不会立即获得完全权限，而是需要经过一段验证期。

```go
// governance/progressive_permission.go
package governance

import (
    "time"
)

// PermissionLevel 权限级别
type PermissionLevel uint8

const (
    PermissionBasic    PermissionLevel = 0x01  // 基础权限
    PermissionStandard PermissionLevel = 0x02  // 标准权限
    PermissionFull     PermissionLevel = 0x03  // 完全权限
)

// ProgressivePermissionConfig 渐进式权限配置
type ProgressivePermissionConfig struct {
    // 基础权限期 (默认 7 天)
    // 只能执行基本操作，不能批量迁移密钥
    BasicPeriod time.Duration
    
    // 标准权限期 (默认 30 天)
    // 可以执行大部分操作，但有速率限制
    StandardPeriod time.Duration
    
    // 完全权限
    // 无限制
}

// DefaultProgressivePermissionConfig 默认配置
func DefaultProgressivePermissionConfig() *ProgressivePermissionConfig {
    return &ProgressivePermissionConfig{
        BasicPeriod:    7 * 24 * time.Hour,   // 7 天
        StandardPeriod: 30 * 24 * time.Hour,  // 30 天
    }
}

// PermissionManager 权限管理器
type PermissionManager struct {
    config *ProgressivePermissionConfig
    
    // MRENCLAVE 加入白名单的时间
    whitelistTime map[string]time.Time
}

// GetPermissionLevel 获取 MRENCLAVE 的当前权限级别
func (p *PermissionManager) GetPermissionLevel(mrenclave []byte) PermissionLevel {
    joinTime, ok := p.whitelistTime[string(mrenclave)]
    if !ok {
        return 0  // 不在白名单中
    }
    
    elapsed := time.Since(joinTime)
    
    if elapsed < p.config.BasicPeriod {
        return PermissionBasic
    }
    
    if elapsed < p.config.StandardPeriod {
        return PermissionStandard
    }
    
    return PermissionFull
}

// PermissionRestrictions 各权限级别的限制
type PermissionRestrictions struct {
    // 基础权限限制
    Basic struct {
        MaxKeyMigrationsPerDay int  // 每天最多迁移密钥数 (默认 10)
        AllowBatchMigration    bool // 是否允许批量迁移 (默认 false)
        AllowKeyExport         bool // 是否允许密钥导出 (默认 false)
    }
    
    // 标准权限限制
    Standard struct {
        MaxKeyMigrationsPerDay int  // 每天最多迁移密钥数 (默认 100)
        AllowBatchMigration    bool // 是否允许批量迁移 (默认 true)
        AllowKeyExport         bool // 是否允许密钥导出 (默认 false)
    }
    
    // 完全权限
    Full struct {
        // 无限制
    }
}

// DefaultPermissionRestrictions 默认限制
func DefaultPermissionRestrictions() *PermissionRestrictions {
    r := &PermissionRestrictions{}
    
    r.Basic.MaxKeyMigrationsPerDay = 10
    r.Basic.AllowBatchMigration = false
    r.Basic.AllowKeyExport = false
    
    r.Standard.MaxKeyMigrationsPerDay = 100
    r.Standard.AllowBatchMigration = true
    r.Standard.AllowKeyExport = false
    
    return r
}
```

**渐进式权限时间线：**

```
新版本加入白名单后的权限演进
============================

Day 0-7: 基础权限期
├── 可以: 正常区块生产、交易处理、新密钥创建
├── 限制: 每天最多迁移 10 个密钥
├── 禁止: 批量迁移、密钥导出
└── 目的: 给社区时间发现潜在问题

Day 7-30: 标准权限期
├── 可以: 大部分正常操作
├── 限制: 每天最多迁移 100 个密钥
├── 禁止: 密钥导出
└── 目的: 逐步放开限制，观察运行情况

Day 30+: 完全权限
├── 可以: 所有操作
├── 限制: 无
└── 目的: 版本已经过充分验证
```

**安全效果：**

即使恶意代码通过了 2/3 投票并加入白名单：
1. 前 7 天只能每天迁移 10 个密钥，大规模数据泄露需要很长时间
2. 社区有 7-30 天时间发现异常并发起否决/回滚
3. 迁移日志公开透明，异常行为容易被社区发现

#### 6.1.0.0.5 验证者质押收益与动态管理机制

社区验证者通过质押代币参与白名单治理，需要有合理的收益激励机制来保证验证者的积极参与，同时需要动态管理机制来剔除不活跃或恶意的验证者。

**收益来源：**

```
验证者收益池资金来源
====================

1. 交易手续费分成 (主要来源)
   └── 每笔交易手续费的 5% 进入验证者收益池

2. 区块奖励分成
   └── 每个区块奖励的 2% 进入验证者收益池

3. 惩罚金没收
   └── 被剔除验证者的部分质押金进入收益池

4. 协议储备金
   └── 初始阶段由协议储备金补贴，确保早期验证者有足够激励
```

**收益计算模型：**

验证者收益由三个维度动态计算：质押量权重、投票贡献度、质押时长加成。

```go
// governance/staking_reward.go
package governance

import (
    "math/big"
    "time"
    
    "github.com/ethereum/go-ethereum/common"
)

// RewardConfig 收益配置
type RewardConfig struct {
    // 各维度权重 (总和为 100)
    StakeAmountWeight      uint64  // 质押量权重 (默认 40)
    VotingContributionWeight uint64  // 投票贡献权重 (默认 40)
    StakeDurationWeight    uint64  // 质押时长权重 (默认 20)
    
    // 时长加成配置
    DurationBonusThresholds []DurationBonus
    
    // 收益池分配周期
    DistributionPeriod time.Duration  // 默认每天分配一次
}

// DurationBonus 质押时长加成
type DurationBonus struct {
    MinDuration time.Duration  // 最小质押时长
    BonusRate   uint64         // 加成比例 (百分比)
}

// DefaultRewardConfig 默认收益配置
func DefaultRewardConfig() *RewardConfig {
    return &RewardConfig{
        StakeAmountWeight:        40,
        VotingContributionWeight: 40,
        StakeDurationWeight:      20,
        DurationBonusThresholds: []DurationBonus{
            {MinDuration: 30 * 24 * time.Hour, BonusRate: 0},    // 30天: 无加成
            {MinDuration: 90 * 24 * time.Hour, BonusRate: 10},   // 90天: +10%
            {MinDuration: 180 * 24 * time.Hour, BonusRate: 25},  // 180天: +25%
            {MinDuration: 365 * 24 * time.Hour, BonusRate: 50},  // 365天: +50%
        },
        DistributionPeriod: 24 * time.Hour,
    }
}

// ValidatorRewardState 验证者收益状态
type ValidatorRewardState struct {
    Address           common.Address
    StakedAmount      *big.Int      // 质押数量
    StakeStartTime    time.Time     // 质押开始时间
    TotalVotes        uint64        // 总投票次数
    ValidVotes        uint64        // 有效投票次数 (投票结果与最终结果一致)
    LastVoteTime      time.Time     // 最后投票时间
    AccumulatedReward *big.Int      // 累计收益
    ClaimedReward     *big.Int      // 已领取收益
}

// RewardCalculator 收益计算器
type RewardCalculator struct {
    config *RewardConfig
}

// CalculateRewardShare 计算验证者的收益份额
func (r *RewardCalculator) CalculateRewardShare(
    validator *ValidatorRewardState,
    totalStaked *big.Int,
    totalValidVotes uint64,
    rewardPool *big.Int,
) *big.Int {
    // 1. 计算质押量得分 (0-100)
    stakeScore := r.calculateStakeScore(validator.StakedAmount, totalStaked)
    
    // 2. 计算投票贡献得分 (0-100)
    votingScore := r.calculateVotingScore(validator, totalValidVotes)
    
    // 3. 计算时长加成
    durationBonus := r.calculateDurationBonus(validator.StakeStartTime)
    
    // 4. 综合得分
    // 综合得分 = (质押得分 * 质押权重 + 投票得分 * 投票权重) * (1 + 时长加成)
    weightedScore := new(big.Int)
    
    stakeComponent := new(big.Int).Mul(
        big.NewInt(int64(stakeScore)),
        big.NewInt(int64(r.config.StakeAmountWeight)),
    )
    
    votingComponent := new(big.Int).Mul(
        big.NewInt(int64(votingScore)),
        big.NewInt(int64(r.config.VotingContributionWeight)),
    )
    
    weightedScore.Add(stakeComponent, votingComponent)
    
    // 应用时长加成
    bonusMultiplier := big.NewInt(100 + int64(durationBonus))
    weightedScore.Mul(weightedScore, bonusMultiplier)
    weightedScore.Div(weightedScore, big.NewInt(100))
    
    // 5. 计算实际收益
    // 收益 = 收益池 * (验证者得分 / 总得分)
    // 简化：这里假设已经有总得分，实际实现需要遍历所有验证者
    reward := new(big.Int).Mul(rewardPool, weightedScore)
    reward.Div(reward, big.NewInt(10000))  // 归一化
    
    return reward
}

// calculateStakeScore 计算质押量得分
func (r *RewardCalculator) calculateStakeScore(staked, totalStaked *big.Int) uint64 {
    if totalStaked.Sign() == 0 {
        return 0
    }
    
    // 得分 = (个人质押 / 总质押) * 100
    score := new(big.Int).Mul(staked, big.NewInt(100))
    score.Div(score, totalStaked)
    
    return score.Uint64()
}

// calculateVotingScore 计算投票贡献得分
func (r *RewardCalculator) calculateVotingScore(validator *ValidatorRewardState, totalValidVotes uint64) uint64 {
    if totalValidVotes == 0 {
        return 0
    }
    
    // 投票参与率
    participationRate := float64(validator.TotalVotes) / float64(totalValidVotes) * 100
    
    // 投票准确率 (投票结果与最终结果一致的比例)
    accuracyRate := float64(0)
    if validator.TotalVotes > 0 {
        accuracyRate = float64(validator.ValidVotes) / float64(validator.TotalVotes) * 100
    }
    
    // 综合得分 = 参与率 * 0.6 + 准确率 * 0.4
    score := participationRate*0.6 + accuracyRate*0.4
    
    if score > 100 {
        score = 100
    }
    
    return uint64(score)
}

// calculateDurationBonus 计算质押时长加成
func (r *RewardCalculator) calculateDurationBonus(stakeStartTime time.Time) uint64 {
    duration := time.Since(stakeStartTime)
    
    var bonus uint64 = 0
    for _, threshold := range r.config.DurationBonusThresholds {
        if duration >= threshold.MinDuration {
            bonus = threshold.BonusRate
        }
    }
    
    return bonus
}
```

**收益分配流程：**

```
每日收益分配流程
================

1. 收益池结算 (每天 UTC 00:00)
   ├── 统计过去 24 小时的交易手续费分成
   ├── 统计过去 24 小时的区块奖励分成
   ├── 加入惩罚金没收 (如有)
   └── 计算总收益池金额

2. 验证者得分计算
   ├── 遍历所有活跃验证者
   ├── 计算每个验证者的综合得分
   │   ├── 质押量得分 (权重 40%)
   │   ├── 投票贡献得分 (权重 40%)
   │   └── 时长加成 (权重 20%)
   └── 汇总总得分

3. 收益分配
   ├── 按得分比例分配收益池
   ├── 更新每个验证者的累计收益
   └── 记录分配日志

4. 收益领取
   ├── 验证者可随时领取累计收益
   ├── 领取时扣除已领取金额
   └── 转账到验证者地址
```

**动态剔除机制：**

为保证验证者群体的活跃度和可靠性，系统会根据多个维度动态剔除不合格的验证者。

```go
// governance/validator_removal.go
package governance

import (
    "time"
    
    "github.com/ethereum/go-ethereum/common"
)

// RemovalReason 剔除原因
type RemovalReason uint8

const (
    RemovalInactive       RemovalReason = 0x01  // 长期不活跃
    RemovalLowStake       RemovalReason = 0x02  // 质押量不足
    RemovalMalicious      RemovalReason = 0x03  // 恶意行为
    RemovalVoluntary      RemovalReason = 0x04  // 主动退出
    RemovalSGXInvalid     RemovalReason = 0x05  // SGX 验证失效
)

// RemovalConfig 剔除配置
type RemovalConfig struct {
    // 不活跃剔除
    MaxInactiveDays        int           // 最大不活跃天数 (默认 30 天)
    MinVotingParticipation float64       // 最低投票参与率 (默认 50%)
    
    // 质押量要求
    MinStakeAmount         *big.Int      // 最低质押量 (初始值 10000 X，从 GovernanceContract 读取)
    StakeGracePeriod       time.Duration // 质押不足宽限期 (默认 7 天)
    
    // 恶意行为惩罚
    MaliciousSlashRate     uint64        // 恶意行为罚没比例 (默认 50%)
    
    // SGX 验证
    SGXRevalidationPeriod  time.Duration // SGX 重新验证周期 (默认 30 天)
}

// DefaultRemovalConfig 默认剔除配置
// 注意：MinStakeAmount 的实际值从 GovernanceContract 中读取
func DefaultRemovalConfig() *RemovalConfig {
    return &RemovalConfig{
        MaxInactiveDays:        30,
        MinVotingParticipation: 0.5,
        MinStakeAmount:         new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e18)),  // 初始值：10000 X（从 GovernanceContract 读取）
        StakeGracePeriod:       7 * 24 * time.Hour,
        MaliciousSlashRate:     50,
        SGXRevalidationPeriod:  30 * 24 * time.Hour,
    }
}

// ValidatorHealthCheck 验证者健康检查
type ValidatorHealthCheck struct {
    config *RemovalConfig
}

// CheckResult 检查结果
type CheckResult struct {
    ShouldRemove bool
    Reason       RemovalReason
    SlashAmount  *big.Int      // 罚没金额 (如有)
    GracePeriod  time.Duration // 宽限期 (如有)
    Details      string
}

// CheckValidator 检查验证者状态
func (h *ValidatorHealthCheck) CheckValidator(validator *ValidatorRewardState, recentProposals int) *CheckResult {
    // 1. 检查活跃度
    if result := h.checkActivity(validator, recentProposals); result.ShouldRemove {
        return result
    }
    
    // 2. 检查质押量
    if result := h.checkStakeAmount(validator); result.ShouldRemove {
        return result
    }
    
    // 3. 检查恶意行为 (由外部报告触发)
    // 这里只是占位，实际恶意行为检测需要更复杂的逻辑
    
    return &CheckResult{ShouldRemove: false}
}

// checkActivity 检查活跃度
func (h *ValidatorHealthCheck) checkActivity(validator *ValidatorRewardState, recentProposals int) *CheckResult {
    // 检查最后投票时间
    daysSinceLastVote := int(time.Since(validator.LastVoteTime).Hours() / 24)
    
    if daysSinceLastVote > h.config.MaxInactiveDays {
        return &CheckResult{
            ShouldRemove: true,
            Reason:       RemovalInactive,
            SlashAmount:  big.NewInt(0),  // 不活跃不罚没，只剔除
            Details:      fmt.Sprintf("超过 %d 天未参与投票", daysSinceLastVote),
        }
    }
    
    // 检查投票参与率
    if recentProposals > 0 {
        participationRate := float64(validator.TotalVotes) / float64(recentProposals)
        if participationRate < h.config.MinVotingParticipation {
            return &CheckResult{
                ShouldRemove: true,
                Reason:       RemovalInactive,
                SlashAmount:  big.NewInt(0),
                Details:      fmt.Sprintf("投票参与率 %.1f%% 低于最低要求 %.1f%%", 
                    participationRate*100, h.config.MinVotingParticipation*100),
            }
        }
    }
    
    return &CheckResult{ShouldRemove: false}
}

// checkStakeAmount 检查质押量
func (h *ValidatorHealthCheck) checkStakeAmount(validator *ValidatorRewardState) *CheckResult {
    if validator.StakedAmount.Cmp(h.config.MinStakeAmount) < 0 {
        return &CheckResult{
            ShouldRemove: true,
            Reason:       RemovalLowStake,
            SlashAmount:  big.NewInt(0),
            GracePeriod:  h.config.StakeGracePeriod,
            Details:      fmt.Sprintf("质押量 %s 低于最低要求 %s", 
                validator.StakedAmount.String(), h.config.MinStakeAmount.String()),
        }
    }
    
    return &CheckResult{ShouldRemove: false}
}

// ReportMaliciousBehavior 报告恶意行为
type MaliciousBehaviorReport struct {
    Validator   common.Address
    Reporter    common.Address
    BehaviorType string
    Evidence    []byte
    Timestamp   time.Time
}

// ProcessMaliciousReport 处理恶意行为报告
func (h *ValidatorHealthCheck) ProcessMaliciousReport(
    report *MaliciousBehaviorReport,
    validator *ValidatorRewardState,
) *CheckResult {
    // 恶意行为类型及对应惩罚
    // - 双重投票: 罚没 50% 质押
    // - 投票贿赂: 罚没 100% 质押
    // - 虚假报告: 罚没 30% 质押
    
    slashRate := h.config.MaliciousSlashRate
    slashAmount := new(big.Int).Mul(validator.StakedAmount, big.NewInt(int64(slashRate)))
    slashAmount.Div(slashAmount, big.NewInt(100))
    
    return &CheckResult{
        ShouldRemove: true,
        Reason:       RemovalMalicious,
        SlashAmount:  slashAmount,
        Details:      fmt.Sprintf("恶意行为: %s, 罚没 %d%% 质押", report.BehaviorType, slashRate),
    }
}
```

**动态剔除流程：**

```
验证者动态剔除流程
==================

定期检查 (每天执行)
├── 遍历所有社区验证者
├── 执行健康检查
│   ├── 活跃度检查
│   │   ├── 最后投票时间 > 30 天 → 标记为待剔除
│   │   └── 投票参与率 < 50% → 标记为待剔除
│   ├── 质押量检查
│   │   └── 质押量 < 10,000 X → 给予 7 天宽限期
│   └── SGX 验证检查
│       └── SGX 证明过期 → 要求重新验证
└── 执行剔除/警告

恶意行为处理 (事件触发)
├── 接收恶意行为报告
├── 核心验证者审核
│   ├── 2/3 核心验证者确认 → 执行惩罚
│   └── 未达到 2/3 → 驳回报告
└── 执行惩罚
    ├── 罚没部分/全部质押
    ├── 从验证者列表剔除
    └── 记录恶意行为历史

主动退出
├── 验证者提交退出申请
├── 进入 14 天冷却期
│   ├── 冷却期内仍需参与投票
│   └── 冷却期内可取消退出
└── 冷却期结束
    ├── 返还全部质押
    └── 从验证者列表移除
```

**收益与惩罚汇总表：**

| 行为 | 收益/惩罚 | 说明 |
|------|-----------|------|
| 正常质押 | 基础收益 | 按质押量比例分配 |
| 积极投票 | +40% 权重 | 投票参与率和准确率越高收益越高 |
| 长期质押 (90天+) | +10% 加成 | 鼓励长期参与 |
| 长期质押 (180天+) | +25% 加成 | 鼓励长期参与 |
| 长期质押 (365天+) | +50% 加成 | 鼓励长期参与 |
| 不活跃 (30天+) | 剔除 | 不罚没，返还质押 |
| 质押不足 | 7天宽限后剔除 | 不罚没，返还质押 |
| 恶意行为 | 罚没 50-100% | 根据严重程度 |

**激励机制设计原则：**

```
+------------------------------------------------------------------+
|                     验证者激励机制设计原则                         |
+------------------------------------------------------------------+
|                                                                  |
|  1. 正向激励为主                                                  |
|     ├── 收益与贡献正相关                                          |
|     ├── 长期参与有额外奖励                                        |
|     └── 避免过度惩罚导致参与意愿下降                              |
|                                                                  |
|  2. 惩罚适度                                                      |
|     ├── 不活跃只剔除不罚没 (可能是技术原因)                       |
|     ├── 质押不足给予宽限期 (可能是市场波动)                       |
|     └── 只有恶意行为才罚没 (需要充分证据)                         |
|                                                                  |
|  3. 透明可预期                                                    |
|     ├── 所有规则链上公开                                          |
|     ├── 收益计算可验证                                            |
|     └── 剔除前有警告和宽限期                                      |
|                                                                  |
|  4. 防止垄断                                                      |
|     ├── 单个验证者质押上限 (如总质押的 10%)                       |
|     ├── 投票贡献权重与质押量权重平衡                              |
|     └── 鼓励更多小额质押者参与                                    |
|                                                                  |
+------------------------------------------------------------------+
```

#### 6.1.0.1 硬分叉数据迁移与保留

硬分叉升级时，**非加密分区的数据直接复用**，不需要在不同版本的节点间同步。**唯一需要从旧节点迁移到新节点的只有秘密数据**（加密分区中的私钥等）。

**重要说明**：
- **数据迁移逻辑仅限于新旧版本节点之间**（不同 MRENCLAVE 的节点）
- **同版本节点之间**：不管是非秘密数据还是秘密数据，日常都正常同步（使用以太坊原有的节点数据同步逻辑，参见第 5.2 节）
- **硬分叉升级场景**（新旧版本节点之间）：本地已有的非秘密数据可以直接复用，只有秘密数据需要从旧版本节点迁移到新版本节点

**数据分类：**

| 数据类型 | 存储位置 | 迁移策略 |
|----------|----------|----------|
| 区块链状态 | LevelDB | **直接复用**，无需迁移 |
| 账户余额 | StateDB | **直接复用**，无需迁移 |
| 合约存储 | StateDB | **直接复用**，无需迁移 |
| 交易历史 | LevelDB | **直接复用**，无需迁移 |
| 私钥数据 | 加密分区 | **需要迁移** (Gramine 重新加密) |
| 密钥元数据 | 加密分区 | **需要迁移** |
| 派生秘密 | 加密分区 | **需要迁移** |

**重要说明**：
- 此处描述的是**硬分叉升级场景**下新旧版本节点之间的数据复用和迁移策略
- **同版本节点之间**：所有数据（非秘密数据和秘密数据）都正常同步（参见第 5.2 节）
- **不同版本节点之间**（硬分叉场景）：
  - 本地已有的非加密分区数据（区块链状态、账户余额、合约存储等）可以直接复用
  - 只有加密分区中的秘密数据需要通过 RA-TLS 安全通道从旧版本节点迁移到新版本节点
- **Gramine 透明处理加密**：应用代码只需在旧节点读取文件、在新节点写入文件，Gramine 自动处理加解密

**秘密数据迁移机制：**

由于 Gramine 的 SGX sealing 使用 MRENCLAVE 作为密钥派生因子，新版本代码的 MRENCLAVE 不同，Gramine 无法直接解密旧版本的加密文件。因此需要通过 RA-TLS 安全通道迁移：

```
硬分叉升级流程：

1. 非加密数据（直接复用）：
   ┌─────────────────────────────────────────────────────────────┐
   │  旧数据目录                      新节点                      │
   │  /data/chaindata/  ─────────────> 直接读取                  │
   │  (区块、状态、交易)               无需迁移                   │
   └─────────────────────────────────────────────────────────────┘

2. 秘密数据（需要迁移）：
   ┌─────────────────────────────────────────────────────────────┐
   │  旧版本节点                       新版本节点                 │
   │  MRENCLAVE: ABC                   MRENCLAVE: DEF            │
   │       │                                 │                    │
   │       │  1. 应用读取文件                │                    │
   │       │  (Gramine 自动解密)             │                    │
   │       │                                 │                    │
   │       │  2. RA-TLS 安全通道传输         │                    │
   │       │────────────────────────────────>│                    │
   │       │                                 │                    │
   │       │                   3. 应用写入文件                    │
   │       │                   (Gramine 自动加密)                 │
   └─────────────────────────────────────────────────────────────┘
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
// 注意：应用只需读写文件，Gramine 自动处理加解密
func (m *DataMigrator) MigrateEncryptedData(ctx context.Context) error {
    // 1. 建立 RA-TLS 连接到旧版本节点
    conn, err := m.ratls.Connect(m.oldEnclave.Address)
    if err != nil {
        return fmt.Errorf("failed to connect to old enclave: %w", err)
    }
    defer conn.Close()
    
    // 2. 请求旧版本节点读取文件并传输数据
    // 旧节点应用读取文件 -> Gramine 自动解密 -> RA-TLS 传输
    keys, err := m.requestKeyMigration(conn)
    if err != nil {
        return fmt.Errorf("failed to migrate keys: %w", err)
    }
    
    // 3. 新节点应用写入文件，Gramine 自动加密
    for _, key := range keys {
        // 应用只需普通文件写入，Gramine 透明处理加密
        if err := m.newEnclave.keyStore.Store(key.ID, key.Data); err != nil {
            return fmt.Errorf("failed to store key %s: %w", key.ID, err)
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
    Keys []MigrationKeyData  // 密钥数据（明文，在 enclave 内）
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
│       ├── keystore.go               # 加密分区密钥存储（Gramine 透明加解密）
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

1. **私钥隔离**：私钥永不离开 SGX enclave，应用在 enclave 内处理
2. **Gramine 透明加密**：Gramine 自动加密加密分区中的所有文件，应用无需处理加密
3. **权限控制**：只有密钥所有者可以使用私钥
4. **派生秘密保护**：ECDH 等派生秘密同样存储在加密分区，由 Gramine 自动保护

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

## 12. 硬件支持说明

### 12.1 运行时依赖

X Chain 基于 **Gramine** 运行时运行，硬件支持完全跟随 Gramine 的支持情况。X Chain 本身不实现硬件抽象层（HAL），而是直接使用 Gramine 提供的 TEE 抽象。

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         X Chain 硬件依赖关系                             │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                        X Chain 应用层                            │   │
│  │  (密钥托管、交易签名、远程证明等业务逻辑)                         │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                  │                                      │
│                                  ▼                                      │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                      Gramine 运行时                              │   │
│  │  - 提供 TEE 抽象 (远程证明、数据密封、加密文件系统)               │   │
│  │  - 提供 /dev/attestation 接口                                    │   │
│  │  - 管理 enclave 生命周期                                         │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                  │                                      │
│                                  ▼                                      │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                      TEE 硬件                                    │   │
│  │  当前: Intel SGX                                                 │   │
│  │  未来: 取决于 Gramine 的支持                                     │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 12.2 当前支持状态

**Gramine 当前仅支持 Intel SGX**，因此 X Chain 目前也仅支持 Intel SGX。

如果 Gramine 未来支持其他 TEE 硬件，X Chain 可以直接受益，无需修改代码。但需要注意的是，新硬件必须满足 X Chain 的安全模型要求（见 12.3 节）。

### 12.3 安全模型要求

X Chain 要求底层硬件必须满足**恶意模型 (Malicious Model)**，即：

- **不信任任何人**：包括云服务商、系统管理员、节点控制人
- **只信任硬件本身**：安全性完全依赖硬件的密码学保证
- **抵抗特权攻击**：即使攻击者拥有 root 权限或物理访问权限，也无法窃取 enclave 内的秘密

### 12.4 硬件分类与支持情况

| 硬件 | 安全模型 | Gramine 支持 | X Chain 支持 | 原因 |
|------|----------|--------------|--------------|------|
| Intel SGX | 恶意模型 | 支持 | 支持（当前唯一） | 不信任 OS/Hypervisor/节点控制人，私钥对任何人不可见 |
| Intel TDX | 半诚实模型 | 开发中 | 不支持 | 信任 VM 管理员，节点控制人可登录 VM 访问私钥 |
| RISC-V Keystone | 恶意模型 | 不支持 | 不支持 | Gramine 未支持 |
| ARM TrustZone | 半诚实模型 | 不支持 | 不支持 | 信任 Secure World 特权软件 |
| AMD SEV/SEV-SNP | 半诚实模型 | 不支持 | 不支持 | 信任 AMD 固件 |

**重要说明**：即使 Gramine 未来支持 TDX，X Chain 也**不会支持 TDX**，因为 TDX 不满足恶意模型要求。

### 12.5 SGX vs TDX 信任模型对比

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    SGX vs TDX 信任边界对比                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  Intel SGX (支持)                    Intel TDX (不支持)                 │
│  ================                    ==================                 │
│                                                                         │
│  信任边界: Enclave 内部              信任边界: 整个 VM 内部              │
│                                                                         │
│  ┌─────────────────────┐            ┌─────────────────────┐            │
│  │      主机 OS        │            │     Hypervisor      │            │
│  │   (不可信/无法访问)  │            │   (不可信/无法访问)  │            │
│  │  ┌───────────────┐  │            │  ┌───────────────┐  │            │
│  │  │   应用程序    │  │            │  │   Guest VM    │  │            │
│  │  │ (不可信/无法访问)│  │            │  │  ┌─────────┐  │  │            │
│  │  │ ┌───────────┐ │  │            │  │  │ Guest OS │  │  │            │
│  │  │ │  Enclave  │ │  │            │  │  │ (可信)   │  │  │            │
│  │  │ │  (可信)   │ │  │            │  │  │ ┌───────┐│  │  │            │
│  │  │ │ ┌───────┐ │ │  │            │  │  │ │ 应用  ││  │  │            │
│  │  │ │ │ 私钥  │ │ │  │            │  │  │ │ 私钥  ││  │  │            │
│  │  │ │ └───────┘ │ │  │            │  │  │ └───────┘│  │  │            │
│  │  │ └───────────┘ │  │            │  │  └─────────┘  │  │            │
│  │  └───────────────┘  │            │  └───────────────┘  │            │
│  └─────────────────────┘            └─────────────────────┘            │
│                                                                         │
│  节点控制人: 无法访问私钥            节点控制人: 可登录 VM，访问私钥    │
│  云服务商: 无法访问私钥              云服务商: 无法访问私钥              │
│  root 用户: 无法访问私钥             root 用户: 可访问私钥              │
│                                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│  结论:                                                                  │
│  - SGX 适合密钥托管场景，私钥对所有人（包括节点运营者）不可见           │
│  - TDX 适合保护 VM 工作负载不被云厂商窥探，但 VM 管理员可访问所有数据   │
│  - X Chain 需要保护私钥不被节点控制人访问，因此只能使用 SGX             │
└─────────────────────────────────────────────────────────────────────────┘
```

### 12.6 Gramine 提供的 TEE 能力

X Chain 通过 Gramine 使用以下 TEE 能力：

| 能力 | Gramine 接口 | X Chain 用途 |
|------|--------------|--------------|
| 远程证明 | `/dev/attestation/quote` | 节点身份验证、RA-TLS |
| 数据密封 | Gramine encrypted files | 私钥持久化存储 |
| 代码度量 | MRENCLAVE/MRSIGNER | 白名单治理、硬分叉控制 |
| 硬件随机数 | RDRAND 指令 | 密钥生成 |

### 12.7 未来硬件支持

如果 Gramine 未来支持新的 TEE 硬件，X Chain 将评估该硬件是否满足恶意模型要求：

1. **满足恶意模型**：可以支持（如 RISC-V Keystone）
2. **不满足恶意模型**：不支持（如 TDX、ARM TrustZone）

X Chain 不会为了支持更多硬件而降低安全要求。

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
