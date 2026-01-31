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
```

### 上游依赖
- 共识引擎模块（提供区块生产信息）
- 核心 go-ethereum StateDB

### 下游依赖（被以下模块使用）
- 共识引擎模块（奖励分配）
- 治理模块（验证者质押收益）

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
type MultiProducerRewardConfig struct {
    // 主生产者奖励比例（百分比）
    PrimaryProducerShare uint64
    
    // 次生产者奖励比例（百分比）
    SecondaryProducerShare uint64
    
    // 质量评分权重
    QualityScoreWeight uint64
    
    // 时间戳权重
    TimestampWeight uint64
}

// DefaultMultiProducerRewardConfig 默认配置
func DefaultMultiProducerRewardConfig() *MultiProducerRewardConfig {
    return &MultiProducerRewardConfig{
        PrimaryProducerShare:   70,
        SecondaryProducerShare: 30,
        QualityScoreWeight:     60,
        TimestampWeight:        40,
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
    Producer     common.Address
    BlockHash    common.Hash
    Timestamp    uint64
    TxCount      int
    GasUsed      uint64
    QualityScore uint64
}

// CalculateRewards 计算多生产者奖励分配
func (c *MultiProducerRewardCalculator) CalculateRewards(
    totalReward *big.Int,
    candidates []*BlockCandidate,
) map[common.Address]*big.Int {
    rewards := make(map[common.Address]*big.Int)
    
    if len(candidates) == 0 {
        return rewards
    }
    
    if len(candidates) == 1 {
        // 单一生产者获得全部奖励
        rewards[candidates[0].Producer] = new(big.Int).Set(totalReward)
        return rewards
    }
    
    // 计算综合得分
    scores := c.calculateScores(candidates)
    totalScore := uint64(0)
    for _, score := range scores {
        totalScore += score
    }
    
    // 按得分比例分配奖励
    for i, candidate := range candidates {
        share := new(big.Int).Mul(totalReward, big.NewInt(int64(scores[i])))
        share.Div(share, big.NewInt(int64(totalScore)))
        rewards[candidate.Producer] = share
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
    // 交易数量权重
    TxCountWeight uint64
    
    // Gas 利用率权重
    GasUtilizationWeight uint64
    
    // 交易多样性权重
    TxDiversityWeight uint64
    
    // 目标 Gas 利用率
    TargetGasUtilization uint64
}

// DefaultBlockQualityConfig 默认配置
func DefaultBlockQualityConfig() *BlockQualityConfig {
    return &BlockQualityConfig{
        TxCountWeight:        30,
        GasUtilizationWeight: 50,
        TxDiversityWeight:    20,
        TargetGasUtilization: 80, // 80%
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
func (s *BlockQualityScorer) ScoreBlock(block *types.Block, gasLimit uint64) uint64 {
    // 1. 交易数量得分
    txCountScore := s.scoreTxCount(len(block.Transactions()))
    
    // 2. Gas 利用率得分
    gasUtilScore := s.scoreGasUtilization(block.GasUsed(), gasLimit)
    
    // 3. 交易多样性得分
    diversityScore := s.scoreTxDiversity(block.Transactions())
    
    // 综合得分
    totalScore := (txCountScore*s.config.TxCountWeight +
                  gasUtilScore*s.config.GasUtilizationWeight +
                  diversityScore*s.config.TxDiversityWeight) / 100
    
    return totalScore
}

// scoreTxCount 交易数量得分
func (s *BlockQualityScorer) scoreTxCount(txCount int) uint64 {
    // 交易数越多得分越高，但有上限
    if txCount == 0 {
        return 0
    }
    if txCount >= 100 {
        return 100
    }
    return uint64(txCount)
}

// scoreGasUtilization Gas 利用率得分
func (s *BlockQualityScorer) scoreGasUtilization(gasUsed, gasLimit uint64) uint64 {
    if gasLimit == 0 {
        return 0
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
    
    // 声誉衰减周期
    DecayPeriod time.Duration
    
    // 声誉衰减率（百分比）
    DecayRate uint64
}

// DefaultReputationConfig 默认配置
func DefaultReputationConfig() *ReputationConfig {
    return &ReputationConfig{
        InitialReputation:   1000,
        MaxReputation:       10000,
        MinReputation:       0,
        BlockSuccessBonus:   10,
        BlockFailurePenalty: 20,
        MaliciousPenalty:    500,
        DecayPeriod:         24 * time.Hour,
        DecayRate:           1, // 每天衰减 1%
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
    LastUpdateTime  time.Time
    LastDecayTime   time.Time
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
    rep.LastUpdateTime = time.Now()
}

// getOrCreateReputation 获取或创建声誉记录
func (rm *ReputationManager) getOrCreateReputation(addr common.Address) *NodeReputation {
    rep, ok := rm.reputations[addr]
    if !ok {
        rep = &NodeReputation{
            Address:        addr,
            Score:          rm.config.InitialReputation,
            LastUpdateTime: time.Now(),
            LastDecayTime:  time.Now(),
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
