# 激励机制模块开发文档

## 模块概述

激励机制模块实现 X Chain 的节点激励系统，包括区块奖励、声誉系统、在线奖励和多维度竞争机制。该模块确保节点有动力诚实参与网络，同时惩罚恶意行为。

## 负责团队

**经济/激励团队**

## 模块职责

1. 区块奖励计算与分配
2. 多生产者奖励分配
3. 区块质量评分
4. 声誉系统管理
5. 在线奖励计算
6. 惩罚机制执行
7. 多维度竞争评估

## 依赖关系

```
+------------------+
|  激励机制模块    |
+------------------+
        |
        +---> 共识引擎模块（区块信息）
        |
        +---> 数据存储模块（状态持久化）
        |
        +---> P2P 网络模块（心跳检测）
        |
        +---> 治理模块（奖励参数配置）
```

### 上游依赖
- 共识引擎模块（提供区块生产信息、多生产者候选区块）
- 核心 go-ethereum StateDB
- 治理模块（通过 SecurityConfigContract 获取奖励配置参数）

### 下游依赖（被以下模块使用）
- 共识引擎模块（奖励分配、声誉系统影响出块权重）
- 治理模块（验证者质押收益、投票权重计算）

### 与治理模块的集成
- 奖励参数（如衰减率、质量权重）从链上 SecurityConfigContract 动态读取
- 治理投票可以修改激励机制参数，无需重启节点
- 升级期间，激励计算会考虑节点的版本和权限级别

## 核心数据结构

### 奖励配置

```go
// incentive/config.go
package incentive

import (
    "math/big"
    "time"
)

// RewardConfig 奖励配置
type RewardConfig struct {
    // 基础区块奖励
    BaseBlockReward *big.Int
    
    // 奖励衰减周期（区块数）
    DecayPeriod uint64
    
    // 衰减率（百分比）
    DecayRate uint64
    
    // 最小区块奖励
    MinBlockReward *big.Int
    
    // 多生产者奖励分配
    MultiProducerConfig *MultiProducerRewardConfig
    
    // 在线奖励配置
    OnlineRewardConfig *OnlineRewardConfig
    
    // 声誉配置
    ReputationConfig *ReputationConfig
}

// DefaultRewardConfig 默认奖励配置
func DefaultRewardConfig() *RewardConfig {
    return &RewardConfig{
        BaseBlockReward: big.NewInt(2e18),  // 2 X
        DecayPeriod:     4_000_000,         // 约 1 年
        DecayRate:       10,                // 10%
        MinBlockReward:  big.NewInt(1e17),  // 0.1 X
        MultiProducerConfig: DefaultMultiProducerRewardConfig(),
        OnlineRewardConfig:  DefaultOnlineRewardConfig(),
        ReputationConfig:    DefaultReputationConfig(),
    }
}
```

### 多生产者奖励

```go
// incentive/multi_producer.go
package incentive

import (
    "math/big"
    
    "github.com/ethereum/go-ethereum/common"
)

// MultiProducerRewardConfig 多生产者奖励配置
// 
// 设计目标：前三名都给收益，根据广播速度和区块质量综合调整收益分配，
// 避免"赢家通吃"导致的恶性抢先行为。
type MultiProducerRewardConfig struct {
    // 速度基础奖励比例（第1名=100%, 第2名=60%, 第3名=30%）
    SpeedRewardRatios []float64
    
    // 候选区块收集窗口（收到第一个区块后等待多久收集其他候选）
    CandidateWindow time.Duration
    
    // 最大候选区块数
    MaxCandidates int
    
    // 质量评分权重
    QualityScoreWeight uint64
    
    // 时间戳权重
    TimestampWeight uint64
}

// DefaultMultiProducerRewardConfig 默认配置
func DefaultMultiProducerRewardConfig() *MultiProducerRewardConfig {
    return &MultiProducerRewardConfig{
        SpeedRewardRatios: []float64{1.0, 0.6, 0.3}, // 100%, 60%, 30%
        CandidateWindow:   500 * time.Millisecond,   // 500ms 窗口
        MaxCandidates:     3,
        QualityScoreWeight: 60,
        TimestampWeight:    40,
    }
}

// MultiProducerRewardCalculator 多生产者奖励计算器
type MultiProducerRewardCalculator struct {
    config *MultiProducerRewardConfig
}

// NewMultiProducerRewardCalculator 创建计算器
func NewMultiProducerRewardCalculator(config *MultiProducerRewardConfig) *MultiProducerRewardCalculator {
    return &MultiProducerRewardCalculator{config: config}
}

// BlockCandidate 区块候选
type BlockCandidate struct {
    Block       *types.Block
    Producer    common.Address
    BlockHash   common.Hash
    ReceivedAt  time.Time      // 收到区块的时间
    Timestamp   uint64
    TxCount     int
    GasUsed     uint64
    Quality     *BlockQuality  // 区块质量评分
    QualityScore uint64
    Rank        int            // 排名 (1, 2, 3)
}

// BlockQuality 区块质量详情
type BlockQuality struct {
    TxCount          uint64  // 交易数量
    BlockSize        uint64  // 区块大小（字节）
    GasUsed          uint64  // Gas 使用量
    NewTxCount       uint64  // 新交易数（相对于第一名）
    UniqueSenders    uint64  // 不同发送者数量
    RewardMultiplier float64 // 质量奖励倍数
}

// CandidateReward 候选区块收益
type CandidateReward struct {
    Candidate       *BlockCandidate
    SpeedRatio      float64  // 速度奖励比例
    QualityMulti    float64  // 质量倍数
    FinalMultiplier float64  // 最终收益倍数 = SpeedRatio × QualityMulti
    Reward          *big.Int // 最终收益
}

// CalculateRewards 计算多生产者奖励分配
// 
// 前三名收益分配机制：
//   第 1 名: 速度基础奖励 100% × 区块质量倍数
//   第 2 名: 速度基础奖励  60% × 区块质量倍数
//   第 3 名: 速度基础奖励  30% × 区块质量倍数
//
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

// calculateScores 计算综合得分
func (c *MultiProducerRewardCalculator) calculateScores(candidates []*BlockCandidate) []uint64 {
    scores := make([]uint64, len(candidates))
    
    // 找出最早时间戳
    minTimestamp := candidates[0].Timestamp
    for _, cand := range candidates {
        if cand.Timestamp < minTimestamp {
            minTimestamp = cand.Timestamp
        }
    }
    
    for i, cand := range candidates {
        // 质量得分（0-100）
        qualityScore := cand.QualityScore
        
        // 时间得分（越早越高）
        timeDiff := cand.Timestamp - minTimestamp
        timeScore := uint64(100)
        if timeDiff > 0 {
            // 每延迟 100ms 扣 1 分
            penalty := timeDiff / 100
            if penalty > 100 {
                penalty = 100
            }
            timeScore = 100 - penalty
        }
        
        // 综合得分
        scores[i] = (qualityScore*c.config.QualityScoreWeight + 
                    timeScore*c.config.TimestampWeight) / 100
    }
    
    return scores
}
```

### 区块质量评分

```go
// incentive/block_quality.go
package incentive

import (
    "github.com/ethereum/go-ethereum/core/types"
)

// BlockQualityConfig 区块质量评分配置
type BlockQualityConfig struct {
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

// DefaultBlockQualityConfig 默认配置
func DefaultBlockQualityConfig() *BlockQualityConfig {
    return &BlockQualityConfig{
        TxCountWeight:        40,
        BlockSizeWeight:      30,
        GasUtilizationWeight: 20,
        TxDiversityWeight:    10,
        MinTxThreshold:       5,           // 至少 5 笔交易
        TargetBlockSize:      1024 * 1024, // 1MB
        TargetGasUtilization: 0.8,         // 80% Gas 利用率
    }
}

// BlockQualityScorer 区块质量评分器
type BlockQualityScorer struct {
    config *BlockQualityConfig
}

// NewBlockQualityScorer 创建评分器
func NewBlockQualityScorer(config *BlockQualityConfig) *BlockQualityScorer {
    return &BlockQualityScorer{config: config}
}

// ScoreBlock 评估区块质量
// 
// 区块质量评分考虑多个维度：
// 1. 交易数量：更多交易意味着更高的网络效用
// 2. 区块大小：接近目标大小的区块得分更高
// 3. Gas 利用率：接近目标利用率（80%）得分最高
// 4. 交易多样性：不同类型的交易提高得分
//
// 返回值范围：0-100
func (s *BlockQualityScorer) ScoreBlock(block *types.Block, gasLimit uint64) uint64 {
    // 1. 交易数量得分
    txCountScore := s.scoreTxCount(len(block.Transactions()))
    
    // 2. 区块大小得分
    blockSizeScore := s.scoreBlockSize(block.Size())
    
    // 3. Gas 利用率得分
    gasUtilScore := s.scoreGasUtilization(block.GasUsed(), gasLimit)
    
    // 4. 交易多样性得分
    diversityScore := s.scoreTxDiversity(block.Transactions())
    
    // 综合得分（权重总和为 100）
    totalScore := (txCountScore*uint64(s.config.TxCountWeight) +
                  blockSizeScore*uint64(s.config.BlockSizeWeight) +
                  gasUtilScore*uint64(s.config.GasUtilizationWeight) +
                  diversityScore*uint64(s.config.TxDiversityWeight)) / 100
    
    return totalScore
}

// CalculateQuality 计算区块质量详情（用于多生产者奖励）
func (s *BlockQualityScorer) CalculateQuality(block *types.Block) *BlockQuality {
    gasLimit := block.GasLimit()
    qualityScore := s.ScoreBlock(block, gasLimit)
    
    // 统计不同发送者
    txs := block.Transactions()
    senders := make(map[common.Address]bool)
    for _, tx := range txs {
        from, _ := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
        senders[from] = true
    }
    
    // 质量倍数：质量分数映射到奖励倍数
    // 质量分数 100 -> 倍数 2.0
    // 质量分数 50  -> 倍数 1.0
    // 质量分数 0   -> 倍数 0.5
    multiplier := 0.5 + (float64(qualityScore) / 100.0 * 1.5)
    
    return &BlockQuality{
        TxCount:          uint64(len(txs)),
        BlockSize:        block.Size(),
        GasUsed:          block.GasUsed(),
        UniqueSenders:    uint64(len(senders)),
        RewardMultiplier: multiplier,
    }
}

// scoreTxCount 交易数量得分
// 
// 评分策略：
// - 0 笔交易：0 分
// - 1-10 笔：线性增长（每笔 10 分）
// - 11-100 笔：继续增长但速度放缓
// - 100+ 笔：满分 100
func (s *BlockQualityScorer) scoreTxCount(txCount int) uint64 {
    if txCount == 0 {
        return 0
    }
    if txCount >= 100 {
        return 100
    }
    if txCount <= 10 {
        return uint64(txCount * 10)
    }
    // 10-100 笔之间，逐渐增长但速度放缓
    // 此处简化为恒定增长，实际可使用对数函数
    return uint64(10 + (txCount-10))
}

// scoreGasUtilization Gas 利用率得分
//
// 评分策略（以目标利用率 80% 为最优）：
// - 利用率 = 目标（80%）：满分 100
// - 偏离目标：按偏离程度扣分
// - 过低或过高利用率都会降低得分
func (s *BlockQualityScorer) scoreGasUtilization(gasUsed, gasLimit uint64) uint64 {
    if gasLimit == 0 {
        return 0
    }
    
    utilization := gasUsed * 100 / gasLimit
    target := uint64(s.config.TargetGasUtilization * 100)
    
    // 计算偏离度
    var deviation uint64
    if utilization >= target {
        deviation = utilization - target
    } else {
        deviation = target - utilization
    }
    
    // 偏离越大，扣分越多
    // 偏离 20% 以内：扣分较少
    // 偏离超过 20%：大幅扣分
    if deviation <= 20 {
        return 100 - deviation*2
    }
    return 100 - 40 - (deviation-20)*3
}

// scoreBlockSize 区块大小得分
//
// 评分策略（以目标区块大小为最优）：
// - 区块大小 = 目标（1MB）：满分 100
// - 偏离目标：按偏离程度扣分
// - 过小说明交易不足，过大可能影响传播
func (s *BlockQualityScorer) scoreBlockSize(blockSize uint64) uint64 {
    if blockSize == 0 {
        return 0
    }
    
    target := s.config.TargetBlockSize
    
    // 计算比例（百分比）
    var ratio uint64
    if blockSize >= target {
        ratio = blockSize * 100 / target // 大于目标
    } else {
        ratio = blockSize * 100 / target // 小于目标
    }
    
    // 最优范围：目标的 70%-130%
    if ratio >= 70 && ratio <= 130 {
        // 在最优范围内，得满分或接近满分
        if ratio >= 90 && ratio <= 110 {
            return 100 // 完美区间
        }
        // 稍微偏离，轻微扣分
        var deviation uint64
        if ratio < 90 {
            deviation = 90 - ratio
        } else {
            deviation = ratio - 110
        }
        return 100 - deviation/2
    }
    
    // 偏离较大
    if ratio < 70 {
        // 过小，按比例扣分
        return ratio * 100 / 70
    }
    
    // 过大，扣分更多
    deviation := ratio - 130
    score := uint64(100)
    if deviation > score {
        return 0
    }
    return score - deviation
}

// scoreTxDiversity 交易多样性得分
//
// 评分策略：
// - 单一类型交易：基础分
// - 多种类型交易：额外加分
// - 包含合约交互：额外加分
func (s *BlockQualityScorer) scoreTxDiversity(txs []*types.Transaction) uint64 {
    if len(txs) == 0 {
        return 0
    }
    
    hasTransfer := false
    hasContractCall := false
    hasContractCreation := false
    uniqueContracts := make(map[common.Address]bool)
    
    for _, tx := range txs {
        if tx.To() == nil {
            hasContractCreation = true
        } else if len(tx.Data()) > 0 {
            hasContractCall = true
            uniqueContracts[*tx.To()] = true
        } else {
            hasTransfer = true
        }
    }
    
    score := uint64(50) // 基础分
    
    // 有多种交易类型
    typeCount := 0
    if hasTransfer {
        typeCount++
    }
    if hasContractCall {
        typeCount++
    }
    if hasContractCreation {
        typeCount++
    }
    
    score += uint64(typeCount * 15)
    
    // 合约多样性（最多加 20 分）
    contractDiversity := len(uniqueContracts)
    if contractDiversity > 10 {
        score += 20
    } else {
        score += uint64(contractDiversity * 2)
    }
    
    if score > 100 {
        score = 100
    }
    
    return score
}
    
    utilization := gasUsed * 100 / gasLimit
    target := s.config.TargetGasUtilization
    
    // 接近目标利用率得分最高
    if utilization >= target {
        return 100
    }
    
    // 低于目标按比例扣分
    return utilization * 100 / target
}

// scoreTxDiversity 交易多样性得分
func (s *BlockQualityScorer) scoreTxDiversity(txs types.Transactions) uint64 {
    if len(txs) == 0 {
        return 0
    }
    
    // 统计不同发送者数量
    senders := make(map[common.Address]bool)
    for _, tx := range txs {
        sender, _ := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
        senders[sender] = true
    }
    
    // 多样性 = 不同发送者数 / 总交易数
    diversity := uint64(len(senders)) * 100 / uint64(len(txs))
    return diversity
}
```

### 声誉系统

```go
// incentive/reputation.go
package incentive

import (
    "math/big"
    "sync"
    "time"
    
    "github.com/ethereum/go-ethereum/common"
)

// ReputationConfig 声誉配置
type ReputationConfig struct {
    // 初始声誉值
    InitialReputation uint64
    
    // 最大声誉值
    MaxReputation uint64
    
    // 最小声誉值
    MinReputation uint64
    
    // 成功出块奖励
    BlockSuccessBonus uint64
    
    // 出块失败惩罚
    BlockFailurePenalty uint64
    
    // 恶意行为惩罚
    MaliciousPenalty uint64
    
    // 离线惩罚（每小时）
    OfflinePenaltyPerHour uint64
    
    // 在线恢复奖励（每小时）
    OnlineRecoveryPerHour uint64
    
    // 最大累积惩罚次数（超过后会被排除）
    MaxPenaltyCount uint64
    
    // 声誉衰减周期
    DecayPeriod time.Duration
    
    // 声誉衰减率（百分比）
    DecayRate uint64
}

// DefaultReputationConfig 默认配置
//
// 基于 ARCHITECTURE.md 第 3.3.8.4 节的声誉系统设计：
// - 离线惩罚：10 分/小时
// - 在线恢复：50 分/小时（恢复速度是惩罚的 5 倍）
// - 最大惩罚次数：10 次（超过后会被排除出网络）
func DefaultReputationConfig() *ReputationConfig {
    return &ReputationConfig{
        InitialReputation:     1000,
        MaxReputation:         10000,
        MinReputation:         0,
        BlockSuccessBonus:     10,
        BlockFailurePenalty:   20,
        MaliciousPenalty:      500,
        OfflinePenaltyPerHour: 10,     // 按 ARCHITECTURE.md 设定
        OnlineRecoveryPerHour: 50,     // 恢复速度是惩罚的 5 倍
        MaxPenaltyCount:       10,     // 最大累积惩罚次数
        DecayPeriod:           24 * time.Hour,
        DecayRate:             1,      // 每天衰减 1%
    }
}

// ReputationManager 声誉管理器
type ReputationManager struct {
    config      *ReputationConfig
    mu          sync.RWMutex
    reputations map[common.Address]*NodeReputation
}

// NodeReputation 节点声誉
type NodeReputation struct {
    Address         common.Address
    Score           uint64
    TotalBlocks     uint64
    SuccessBlocks   uint64
    FailedBlocks    uint64
    MaliciousCount  uint64
    PenaltyCount    uint64        // 累积惩罚次数
    OfflineHours    uint64        // 累计离线小时数
    OnlineHours     uint64        // 累计在线小时数
    LastUpdateTime  time.Time
    LastDecayTime   time.Time
    LastOnlineCheck time.Time     // 上次在线状态检查时间
}

// NewReputationManager 创建声誉管理器
func NewReputationManager(config *ReputationConfig) *ReputationManager {
    return &ReputationManager{
        config:      config,
        reputations: make(map[common.Address]*NodeReputation),
    }
}

// GetReputation 获取节点声誉
func (rm *ReputationManager) GetReputation(addr common.Address) *NodeReputation {
    rm.mu.RLock()
    defer rm.mu.RUnlock()
    
    rep, ok := rm.reputations[addr]
    if !ok {
        return &NodeReputation{
            Address: addr,
            Score:   rm.config.InitialReputation,
        }
    }
    
    return rep
}

// RecordBlockSuccess 记录成功出块
func (rm *ReputationManager) RecordBlockSuccess(addr common.Address) {
    rm.mu.Lock()
    defer rm.mu.Unlock()
    
    rep := rm.getOrCreateReputation(addr)
    
    // 应用衰减
    rm.applyDecay(rep)
    
    // 增加声誉
    rep.Score += rm.config.BlockSuccessBonus
    if rep.Score > rm.config.MaxReputation {
        rep.Score = rm.config.MaxReputation
    }
    
    rep.TotalBlocks++
    rep.SuccessBlocks++
    rep.LastUpdateTime = time.Now()
}

// RecordBlockFailure 记录出块失败
func (rm *ReputationManager) RecordBlockFailure(addr common.Address) {
    rm.mu.Lock()
    defer rm.mu.Unlock()
    
    rep := rm.getOrCreateReputation(addr)
    
    // 应用衰减
    rm.applyDecay(rep)
    
    // 减少声誉
    if rep.Score >= rm.config.BlockFailurePenalty {
        rep.Score -= rm.config.BlockFailurePenalty
    } else {
        rep.Score = rm.config.MinReputation
    }
    
    rep.TotalBlocks++
    rep.FailedBlocks++
    rep.LastUpdateTime = time.Now()
}

// RecordMaliciousBehavior 记录恶意行为
func (rm *ReputationManager) RecordMaliciousBehavior(addr common.Address) {
    rm.mu.Lock()
    defer rm.mu.Unlock()
    
    rep := rm.getOrCreateReputation(addr)
    
    // 大幅减少声誉
    if rep.Score >= rm.config.MaliciousPenalty {
        rep.Score -= rm.config.MaliciousPenalty
    } else {
        rep.Score = rm.config.MinReputation
    }
    
    rep.MaliciousCount++
    rep.PenaltyCount++
    rep.LastUpdateTime = time.Now()
}

// RecordOffline 记录节点离线
// 
// 基于 ARCHITECTURE.md 的声誉衰减机制：
// - 每小时离线扣除 10 分声誉
// - 累积惩罚次数超过 MaxPenaltyCount 将被排除出网络
func (rm *ReputationManager) RecordOffline(addr common.Address, duration time.Duration) {
    rm.mu.Lock()
    defer rm.mu.Unlock()
    
    rep := rm.getOrCreateReputation(addr)
    
    // 计算离线小时数
    hours := uint64(duration.Hours())
    if hours == 0 && duration > 0 {
        hours = 1 // 至少计为 1 小时
    }
    
    // 应用离线惩罚
    penalty := hours * rm.config.OfflinePenaltyPerHour
    if rep.Score >= penalty {
        rep.Score -= penalty
    } else {
        rep.Score = rm.config.MinReputation
    }
    
    rep.OfflineHours += hours
    rep.PenaltyCount++
    rep.LastUpdateTime = time.Now()
    rep.LastOnlineCheck = time.Now()
}

// RecordOnline 记录节点在线
// 
// 基于 ARCHITECTURE.md 的声誉恢复机制：
// - 每小时在线恢复 50 分声誉（是离线惩罚的 5 倍）
// - 帮助节点快速恢复声誉，鼓励长期稳定在线
func (rm *ReputationManager) RecordOnline(addr common.Address, duration time.Duration) {
    rm.mu.Lock()
    defer rm.mu.Unlock()
    
    rep := rm.getOrCreateReputation(addr)
    
    // 计算在线小时数
    hours := uint64(duration.Hours())
    if hours == 0 && duration > 0 {
        hours = 1 // 至少计为 1 小时
    }
    
    // 应用在线恢复奖励
    recovery := hours * rm.config.OnlineRecoveryPerHour
    rep.Score += recovery
    if rep.Score > rm.config.MaxReputation {
        rep.Score = rm.config.MaxReputation
    }
    
    rep.OnlineHours += hours
    rep.LastUpdateTime = time.Now()
    rep.LastOnlineCheck = time.Now()
}

// IsExcluded 检查节点是否因惩罚过多而被排除
func (rm *ReputationManager) IsExcluded(addr common.Address) bool {
    rep := rm.GetReputation(addr)
    return rep.PenaltyCount >= rm.config.MaxPenaltyCount
}

// getOrCreateReputation 获取或创建声誉记录
func (rm *ReputationManager) getOrCreateReputation(addr common.Address) *NodeReputation {
    rep, ok := rm.reputations[addr]
    if !ok {
        now := time.Now()
        rep = &NodeReputation{
            Address:         addr,
            Score:           rm.config.InitialReputation,
            LastUpdateTime:  now,
            LastDecayTime:   now,
            LastOnlineCheck: now,
        }
        rm.reputations[addr] = rep
    }
    return rep
}

// applyDecay 应用声誉衰减
func (rm *ReputationManager) applyDecay(rep *NodeReputation) {
    elapsed := time.Since(rep.LastDecayTime)
    periods := int(elapsed / rm.config.DecayPeriod)
    
    if periods > 0 {
        for i := 0; i < periods; i++ {
            decay := rep.Score * rm.config.DecayRate / 100
            if rep.Score >= decay {
                rep.Score -= decay
            }
        }
        rep.LastDecayTime = time.Now()
    }
}

// GetReputationScore 获取声誉分数
func (rm *ReputationManager) GetReputationScore(addr common.Address) uint64 {
    rep := rm.GetReputation(addr)
    return rep.Score
}

// IsReputationSufficient 检查声誉是否足够
func (rm *ReputationManager) IsReputationSufficient(addr common.Address, threshold uint64) bool {
    return rm.GetReputationScore(addr) >= threshold
}
```

### 在线奖励

```go
// incentive/online_reward.go
package incentive

import (
    "math/big"
    "sync"
    "time"
    
    "github.com/ethereum/go-ethereum/common"
)

// OnlineRewardConfig 在线奖励配置
type OnlineRewardConfig struct {
    // 心跳间隔
    HeartbeatInterval time.Duration
    
    // 心跳超时（超过此时间视为离线）
    HeartbeatTimeout time.Duration
    
    // 每小时在线奖励
    HourlyReward *big.Int
    
    // 最小在线时长要求（获得奖励）
    MinOnlineTime time.Duration
    
    // 在线率阈值（低于此值无奖励）
    MinUptimeRatio float64
}

// DefaultOnlineRewardConfig 默认配置
func DefaultOnlineRewardConfig() *OnlineRewardConfig {
    return &OnlineRewardConfig{
        HeartbeatInterval: 30 * time.Second,
        HeartbeatTimeout:  2 * time.Minute,
        HourlyReward:      big.NewInt(1e16), // 0.01 X
        MinOnlineTime:     1 * time.Hour,
        MinUptimeRatio:    0.9, // 90%
    }
}

// OnlineRewardManager 在线奖励管理器
type OnlineRewardManager struct {
    config     *OnlineRewardConfig
    mu         sync.RWMutex
    nodeStatus map[common.Address]*NodeOnlineStatus
}

// NodeOnlineStatus 节点在线状态
type NodeOnlineStatus struct {
    Address           common.Address
    LastHeartbeat     time.Time
    OnlineStartTime   time.Time
    TotalOnlineTime   time.Duration
    TotalOfflineTime  time.Duration
    HeartbeatCount    uint64
    MissedHeartbeats  uint64
    AccumulatedReward *big.Int
    ClaimedReward     *big.Int
}

// NewOnlineRewardManager 创建在线奖励管理器
func NewOnlineRewardManager(config *OnlineRewardConfig) *OnlineRewardManager {
    return &OnlineRewardManager{
        config:     config,
        nodeStatus: make(map[common.Address]*NodeOnlineStatus),
    }
}

// RecordHeartbeat 记录心跳
func (orm *OnlineRewardManager) RecordHeartbeat(addr common.Address) {
    orm.mu.Lock()
    defer orm.mu.Unlock()
    
    status := orm.getOrCreateStatus(addr)
    now := time.Now()
    
    // 检查是否从离线恢复
    if time.Since(status.LastHeartbeat) > orm.config.HeartbeatTimeout {
        // 记录离线时间
        if !status.LastHeartbeat.IsZero() {
            status.TotalOfflineTime += time.Since(status.LastHeartbeat)
        }
        status.OnlineStartTime = now
    } else {
        // 累计在线时间
        status.TotalOnlineTime += time.Since(status.LastHeartbeat)
    }
    
    status.LastHeartbeat = now
    status.HeartbeatCount++
}

// CalculateReward 计算在线奖励
func (orm *OnlineRewardManager) CalculateReward(addr common.Address) *big.Int {
    orm.mu.RLock()
    defer orm.mu.RUnlock()
    
    status, ok := orm.nodeStatus[addr]
    if !ok {
        return big.NewInt(0)
    }
    
    // 检查最小在线时长
    if status.TotalOnlineTime < orm.config.MinOnlineTime {
        return big.NewInt(0)
    }
    
    // 计算在线率
    totalTime := status.TotalOnlineTime + status.TotalOfflineTime
    if totalTime == 0 {
        return big.NewInt(0)
    }
    
    uptimeRatio := float64(status.TotalOnlineTime) / float64(totalTime)
    if uptimeRatio < orm.config.MinUptimeRatio {
        return big.NewInt(0)
    }
    
    // 计算奖励
    hours := int64(status.TotalOnlineTime / time.Hour)
    reward := new(big.Int).Mul(orm.config.HourlyReward, big.NewInt(hours))
    
    // 应用在线率加成
    bonus := new(big.Int).Mul(reward, big.NewInt(int64(uptimeRatio*100)))
    bonus.Div(bonus, big.NewInt(100))
    
    return bonus
}

// GetUptimeRatio 获取在线率
func (orm *OnlineRewardManager) GetUptimeRatio(addr common.Address) float64 {
    orm.mu.RLock()
    defer orm.mu.RUnlock()
    
    status, ok := orm.nodeStatus[addr]
    if !ok {
        return 0
    }
    
    totalTime := status.TotalOnlineTime + status.TotalOfflineTime
    if totalTime == 0 {
        return 0
    }
    
    return float64(status.TotalOnlineTime) / float64(totalTime)
}

// IsOnline 检查节点是否在线
func (orm *OnlineRewardManager) IsOnline(addr common.Address) bool {
    orm.mu.RLock()
    defer orm.mu.RUnlock()
    
    status, ok := orm.nodeStatus[addr]
    if !ok {
        return false
    }
    
    return time.Since(status.LastHeartbeat) <= orm.config.HeartbeatTimeout
}

// getOrCreateStatus 获取或创建状态
func (orm *OnlineRewardManager) getOrCreateStatus(addr common.Address) *NodeOnlineStatus {
    status, ok := orm.nodeStatus[addr]
    if !ok {
        status = &NodeOnlineStatus{
            Address:           addr,
            AccumulatedReward: big.NewInt(0),
            ClaimedReward:     big.NewInt(0),
        }
        orm.nodeStatus[addr] = status
    }
    return status
}
```

### 惩罚机制

```go
// incentive/penalty.go
package incentive

import (
    "math/big"
    "time"
    
    "github.com/ethereum/go-ethereum/common"
)

// PenaltyType 惩罚类型
type PenaltyType uint8

const (
    PenaltyDoubleSign    PenaltyType = 0x01 // 双重签名
    PenaltyOffline       PenaltyType = 0x02 // 长期离线
    PenaltyInvalidBlock  PenaltyType = 0x03 // 无效区块
    PenaltyMalicious     PenaltyType = 0x04 // 恶意行为
)

// PenaltyConfig 惩罚配置
type PenaltyConfig struct {
    // 双重签名惩罚（百分比）
    DoubleSignPenaltyRate uint64
    
    // 离线惩罚（每小时）
    OfflinePenaltyPerHour *big.Int
    
    // 无效区块惩罚
    InvalidBlockPenalty *big.Int
    
    // 恶意行为惩罚（百分比）
    MaliciousPenaltyRate uint64
    
    // 惩罚冷却期
    PenaltyCooldown time.Duration
}

// DefaultPenaltyConfig 默认配置
func DefaultPenaltyConfig() *PenaltyConfig {
    return &PenaltyConfig{
        DoubleSignPenaltyRate: 50,                 // 50%
        OfflinePenaltyPerHour: big.NewInt(1e16),   // 0.01 X
        InvalidBlockPenalty:   big.NewInt(1e17),   // 0.1 X
        MaliciousPenaltyRate:  100,                // 100%
    }
}

// PenaltyRecord 惩罚记录
type PenaltyRecord struct {
    NodeAddress common.Address
    Type        PenaltyType
    Amount      *big.Int
    Reason      string
    Timestamp   time.Time
    BlockNumber uint64
    Evidence    []byte
}

// PenaltyManager 惩罚管理器
type PenaltyManager struct {
    config  *PenaltyConfig
    records []*PenaltyRecord
}

// NewPenaltyManager 创建惩罚管理器
func NewPenaltyManager(config *PenaltyConfig) *PenaltyManager {
    return &PenaltyManager{
        config:  config,
        records: make([]*PenaltyRecord, 0),
    }
}

// CalculatePenalty 计算惩罚金额
func (pm *PenaltyManager) CalculatePenalty(
    penaltyType PenaltyType,
    nodeBalance *big.Int,
    additionalInfo interface{},
) *big.Int {
    switch penaltyType {
    case PenaltyDoubleSign:
        // 双重签名：罚没一定比例
        penalty := new(big.Int).Mul(nodeBalance, big.NewInt(int64(pm.config.DoubleSignPenaltyRate)))
        penalty.Div(penalty, big.NewInt(100))
        return penalty
        
    case PenaltyOffline:
        // 离线：按小时计算
        hours := additionalInfo.(int64)
        penalty := new(big.Int).Mul(pm.config.OfflinePenaltyPerHour, big.NewInt(hours))
        return penalty
        
    case PenaltyInvalidBlock:
        // 无效区块：固定惩罚
        return new(big.Int).Set(pm.config.InvalidBlockPenalty)
        
    case PenaltyMalicious:
        // 恶意行为：罚没全部
        penalty := new(big.Int).Mul(nodeBalance, big.NewInt(int64(pm.config.MaliciousPenaltyRate)))
        penalty.Div(penalty, big.NewInt(100))
        return penalty
        
    default:
        return big.NewInt(0)
    }
}

// RecordPenalty 记录惩罚
func (pm *PenaltyManager) RecordPenalty(record *PenaltyRecord) {
    pm.records = append(pm.records, record)
}

// GetPenaltyHistory 获取惩罚历史
func (pm *PenaltyManager) GetPenaltyHistory(addr common.Address) []*PenaltyRecord {
    var history []*PenaltyRecord
    for _, record := range pm.records {
        if record.NodeAddress == addr {
            history = append(history, record)
        }
    }
    return history
}
```

### 多维度竞争

```go
// incentive/competition.go
package incentive

import (
    "math/big"
    "sort"
    
    "github.com/ethereum/go-ethereum/common"
)

// CompetitionDimension 竞争维度
type CompetitionDimension uint8

const (
    DimensionReputation   CompetitionDimension = 0x01 // 声誉
    DimensionUptime       CompetitionDimension = 0x02 // 在线率
    DimensionBlockQuality CompetitionDimension = 0x03 // 区块质量
    DimensionServiceQuality CompetitionDimension = 0x04 // 服务质量
)

// CompetitionConfig 竞争配置
type CompetitionConfig struct {
    // 各维度权重
    Weights map[CompetitionDimension]uint64
    
    // 排名奖励比例
    RankingRewardRates []uint64
}

// DefaultCompetitionConfig 默认配置
func DefaultCompetitionConfig() *CompetitionConfig {
    return &CompetitionConfig{
        Weights: map[CompetitionDimension]uint64{
            DimensionReputation:     30,
            DimensionUptime:         25,
            DimensionBlockQuality:   25,
            DimensionServiceQuality: 20,
        },
        RankingRewardRates: []uint64{
            30, // 第 1 名 30%
            20, // 第 2 名 20%
            15, // 第 3 名 15%
            10, // 第 4 名 10%
            10, // 第 5 名 10%
            5,  // 第 6 名 5%
            5,  // 第 7 名 5%
            3,  // 第 8 名 3%
            1,  // 第 9 名 1%
            1,  // 第 10 名 1%
        },
    }
}

// NodeMetrics 节点指标
type NodeMetrics struct {
    Address        common.Address
    Reputation     uint64
    UptimeRatio    float64
    BlockQuality   uint64
    ServiceQuality uint64
}

// CompetitionManager 竞争管理器
type CompetitionManager struct {
    config            *CompetitionConfig
    reputationMgr     *ReputationManager
    onlineRewardMgr   *OnlineRewardManager
    blockQualityScorer *BlockQualityScorer
}

// NewCompetitionManager 创建竞争管理器
func NewCompetitionManager(
    config *CompetitionConfig,
    reputationMgr *ReputationManager,
    onlineRewardMgr *OnlineRewardManager,
    blockQualityScorer *BlockQualityScorer,
) *CompetitionManager {
    return &CompetitionManager{
        config:            config,
        reputationMgr:     reputationMgr,
        onlineRewardMgr:   onlineRewardMgr,
        blockQualityScorer: blockQualityScorer,
    }
}

// CalculateComprehensiveScore 计算综合得分
func (cm *CompetitionManager) CalculateComprehensiveScore(metrics *NodeMetrics) uint64 {
    score := uint64(0)
    
    // 声誉得分
    reputationScore := metrics.Reputation * cm.config.Weights[DimensionReputation] / 100
    score += reputationScore
    
    // 在线率得分
    uptimeScore := uint64(metrics.UptimeRatio * 100) * cm.config.Weights[DimensionUptime] / 100
    score += uptimeScore
    
    // 区块质量得分
    qualityScore := metrics.BlockQuality * cm.config.Weights[DimensionBlockQuality] / 100
    score += qualityScore
    
    // 服务质量得分
    serviceScore := metrics.ServiceQuality * cm.config.Weights[DimensionServiceQuality] / 100
    score += serviceScore
    
    return score
}

// RankNodes 对节点进行排名
func (cm *CompetitionManager) RankNodes(nodes []*NodeMetrics) []*NodeMetrics {
    // 计算综合得分
    type scoredNode struct {
        metrics *NodeMetrics
        score   uint64
    }
    
    scored := make([]scoredNode, len(nodes))
    for i, node := range nodes {
        scored[i] = scoredNode{
            metrics: node,
            score:   cm.CalculateComprehensiveScore(node),
        }
    }
    
    // 按得分排序
    sort.Slice(scored, func(i, j int) bool {
        return scored[i].score > scored[j].score
    })
    
    // 返回排序后的节点
    result := make([]*NodeMetrics, len(nodes))
    for i, s := range scored {
        result[i] = s.metrics
    }
    
    return result
}

// DistributeRankingRewards 分配排名奖励
func (cm *CompetitionManager) DistributeRankingRewards(
    totalReward *big.Int,
    rankedNodes []*NodeMetrics,
) map[common.Address]*big.Int {
    rewards := make(map[common.Address]*big.Int)
    
    for i, node := range rankedNodes {
        if i >= len(cm.config.RankingRewardRates) {
            break
        }
        
        rate := cm.config.RankingRewardRates[i]
        reward := new(big.Int).Mul(totalReward, big.NewInt(int64(rate)))
        reward.Div(reward, big.NewInt(100))
        
        rewards[node.Address] = reward
    }
    
    return rewards
}
```

## 文件结构

```
incentive/
├── config.go              # 配置定义
├── reward.go              # 基础奖励计算
├── multi_producer.go      # 多生产者奖励
├── block_quality.go       # 区块质量评分
├── reputation.go          # 声誉系统
├── online_reward.go       # 在线奖励
├── penalty.go             # 惩罚机制
├── competition.go         # 多维度竞争
├── storage.go             # 状态存储
└── incentive_test.go      # 测试
```

## 单元测试指南

### 奖励计算测试

```go
// incentive/reward_test.go
package incentive

import (
    "math/big"
    "testing"
)

func TestMultiProducerReward(t *testing.T) {
    config := DefaultMultiProducerRewardConfig()
    calculator := NewMultiProducerRewardCalculator(config)
    
    totalReward := big.NewInt(1e18) // 1 X
    
    candidates := []*BlockCandidate{
        {Producer: common.HexToAddress("0x1"), Timestamp: 1000, QualityScore: 80},
        {Producer: common.HexToAddress("0x2"), Timestamp: 1100, QualityScore: 90},
    }
    
    rewards := calculator.CalculateRewards(totalReward, candidates)
    
    // 验证奖励总和等于总奖励
    total := big.NewInt(0)
    for _, reward := range rewards {
        total.Add(total, reward)
    }
    
    if total.Cmp(totalReward) != 0 {
        t.Errorf("Total rewards mismatch: got %s, want %s", total, totalReward)
    }
}

func TestSingleProducerReward(t *testing.T) {
    config := DefaultMultiProducerRewardConfig()
    calculator := NewMultiProducerRewardCalculator(config)
    
    totalReward := big.NewInt(1e18)
    producer := common.HexToAddress("0x1")
    
    candidates := []*BlockCandidate{
        {Producer: producer, Timestamp: 1000, QualityScore: 100},
    }
    
    rewards := calculator.CalculateRewards(totalReward, candidates)
    
    // 单一生产者应获得全部奖励
    if rewards[producer].Cmp(totalReward) != 0 {
        t.Errorf("Single producer should get all reward")
    }
}
```

### 声誉系统测试

```go
// incentive/reputation_test.go
package incentive

import (
    "testing"
    
    "github.com/ethereum/go-ethereum/common"
)

func TestReputationIncrease(t *testing.T) {
    config := DefaultReputationConfig()
    manager := NewReputationManager(config)
    
    addr := common.HexToAddress("0x1")
    
    // 记录成功出块
    manager.RecordBlockSuccess(addr)
    
    rep := manager.GetReputation(addr)
    expected := config.InitialReputation + config.BlockSuccessBonus
    
    if rep.Score != expected {
        t.Errorf("Reputation mismatch: got %d, want %d", rep.Score, expected)
    }
}

func TestReputationDecrease(t *testing.T) {
    config := DefaultReputationConfig()
    manager := NewReputationManager(config)
    
    addr := common.HexToAddress("0x1")
    
    // 记录出块失败
    manager.RecordBlockFailure(addr)
    
    rep := manager.GetReputation(addr)
    expected := config.InitialReputation - config.BlockFailurePenalty
    
    if rep.Score != expected {
        t.Errorf("Reputation mismatch: got %d, want %d", rep.Score, expected)
    }
}

func TestReputationMaxCap(t *testing.T) {
    config := DefaultReputationConfig()
    manager := NewReputationManager(config)
    
    addr := common.HexToAddress("0x1")
    
    // 多次成功出块
    for i := 0; i < 1000; i++ {
        manager.RecordBlockSuccess(addr)
    }
    
    rep := manager.GetReputation(addr)
    
    if rep.Score > config.MaxReputation {
        t.Errorf("Reputation exceeded max: got %d, max %d", rep.Score, config.MaxReputation)
    }
}
```

### 在线奖励测试

```go
// incentive/online_reward_test.go
package incentive

import (
    "math/big"
    "testing"
    "time"
    
    "github.com/ethereum/go-ethereum/common"
)

func TestOnlineRewardCalculation(t *testing.T) {
    config := DefaultOnlineRewardConfig()
    config.MinOnlineTime = 1 * time.Minute // 测试用短时间
    manager := NewOnlineRewardManager(config)
    
    addr := common.HexToAddress("0x1")
    
    // 模拟在线
    for i := 0; i < 120; i++ {
        manager.RecordHeartbeat(addr)
        time.Sleep(1 * time.Second)
    }
    
    reward := manager.CalculateReward(addr)
    
    if reward.Cmp(big.NewInt(0)) <= 0 {
        t.Error("Should have positive reward after being online")
    }
}

func TestOfflineDetection(t *testing.T) {
    config := DefaultOnlineRewardConfig()
    config.HeartbeatTimeout = 1 * time.Second // 测试用短超时
    manager := NewOnlineRewardManager(config)
    
    addr := common.HexToAddress("0x1")
    
    // 记录心跳
    manager.RecordHeartbeat(addr)
    
    // 等待超时
    time.Sleep(2 * time.Second)
    
    if manager.IsOnline(addr) {
        t.Error("Node should be offline after timeout")
    }
}
```

### 竞争排名测试

```go
// incentive/competition_test.go
package incentive

import (
    "testing"
    
    "github.com/ethereum/go-ethereum/common"
)

func TestNodeRanking(t *testing.T) {
    config := DefaultCompetitionConfig()
    manager := NewCompetitionManager(config, nil, nil, nil)
    
    nodes := []*NodeMetrics{
        {Address: common.HexToAddress("0x1"), Reputation: 50, UptimeRatio: 0.9, BlockQuality: 80, ServiceQuality: 70},
        {Address: common.HexToAddress("0x2"), Reputation: 90, UptimeRatio: 0.95, BlockQuality: 85, ServiceQuality: 90},
        {Address: common.HexToAddress("0x3"), Reputation: 70, UptimeRatio: 0.8, BlockQuality: 90, ServiceQuality: 80},
    }
    
    ranked := manager.RankNodes(nodes)
    
    // 验证排名顺序
    if ranked[0].Address != common.HexToAddress("0x2") {
        t.Error("Node 0x2 should be ranked first")
    }
}

func TestRankingRewardDistribution(t *testing.T) {
    config := DefaultCompetitionConfig()
    manager := NewCompetitionManager(config, nil, nil, nil)
    
    nodes := []*NodeMetrics{
        {Address: common.HexToAddress("0x1")},
        {Address: common.HexToAddress("0x2")},
        {Address: common.HexToAddress("0x3")},
    }
    
    totalReward := big.NewInt(1e18)
    rewards := manager.DistributeRankingRewards(totalReward, nodes)
    
    // 验证第一名获得最多奖励
    if rewards[nodes[0].Address].Cmp(rewards[nodes[1].Address]) <= 0 {
        t.Error("First place should get more reward than second")
    }
}
```

## 配置参数

```toml
# config.toml
[incentive]
# 基础区块奖励
base_block_reward = "2000000000000000000"  # 2 X

# 奖励衰减周期（区块数）
decay_period = 4000000

# 衰减率（百分比）
decay_rate = 10

[incentive.reputation]
# 初始声誉
initial_reputation = 1000

# 最大声誉
max_reputation = 10000

# 成功出块奖励
block_success_bonus = 10

# 出块失败惩罚
block_failure_penalty = 20

[incentive.online]
# 心跳间隔（秒）
heartbeat_interval = 30

# 心跳超时（秒）
heartbeat_timeout = 120

# 每小时在线奖励
hourly_reward = "10000000000000000"  # 0.01 X

[incentive.competition]
# 维度权重
reputation_weight = 30
uptime_weight = 25
block_quality_weight = 25
service_quality_weight = 20
```

## 实现优先级

| 优先级 | 功能 | 预计工时 |
|--------|------|----------|
| P0 | 基础奖励计算 | 2 天 |
| P0 | 多生产者奖励分配 | 3 天 |
| P1 | 区块质量评分 | 2 天 |
| P1 | 声誉系统 | 3 天 |
| P1 | 在线奖励 | 3 天 |
| P2 | 惩罚机制 | 2 天 |
| P2 | 多维度竞争 | 3 天 |

**总计：约 2.5 周**

## 注意事项

1. **状态持久化**：声誉和在线状态需要持久化到 StateDB
2. **精度问题**：大数计算注意精度损失
3. **公平性**：确保奖励分配算法公平透明
4. **防作弊**：防止节点通过作弊获取不当奖励
