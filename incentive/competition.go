// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package incentive

import (
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
)

// CompetitionDimension 竞争维度
type CompetitionDimension uint8

const (
	DimensionReputation     CompetitionDimension = 0x01
	DimensionUptime         CompetitionDimension = 0x02
	DimensionBlockQuality   CompetitionDimension = 0x03
	DimensionServiceQuality CompetitionDimension = 0x04
)

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
	config             *CompetitionConfig
	reputationMgr      *ReputationManager
	onlineRewardMgr    *OnlineRewardManager
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
		config:             config,
		reputationMgr:      reputationMgr,
		onlineRewardMgr:    onlineRewardMgr,
		blockQualityScorer: blockQualityScorer,
	}
}

// CalculateComprehensiveScore 计算综合得分
//
// 综合得分基于四个维度：
// 1. 声誉得分（权重 30%）
// 2. 在线率得分（权重 25%）
// 3. 区块质量得分（权重 25%）
// 4. 服务质量得分（权重 20%）
//
// 参数：
//   - metrics: 节点指标
//
// 返回值：
//   - 综合得分（0-100）
func (cm *CompetitionManager) CalculateComprehensiveScore(metrics *NodeMetrics) uint64 {
	score := uint64(0)

	// 声誉得分（归一化到 0-100）
	normalizedReputation := metrics.Reputation
	if normalizedReputation > 100 {
		normalizedReputation = 100
	}
	reputationScore := normalizedReputation * uint64(cm.config.ReputationWeight*100) / 100
	score += reputationScore

	// 在线率得分（0-100）
	uptimeScore := uint64(metrics.UptimeRatio*100) * uint64(cm.config.UptimeWeight*100) / 100
	score += uptimeScore

	// 区块质量得分（0-100）
	qualityScore := metrics.BlockQuality * uint64(cm.config.BlockQualityWeight*100) / 100
	score += qualityScore

	// 服务质量得分（0-100）
	serviceScore := metrics.ServiceQuality * uint64(cm.config.ServiceQualityWeight*100) / 100
	score += serviceScore

	return score
}

// RankNodes 对节点进行排名
//
// 根据综合得分对节点进行排名，得分高的排在前面
//
// 参数：
//   - nodes: 节点指标列表
//
// 返回值：
//   - 排序后的节点列表
func (cm *CompetitionManager) RankNodes(nodes []*NodeMetrics) []*NodeMetrics {
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

	// 按得分排序（从高到低）
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
//
// 根据排名分配奖励给前 10 名节点
// 奖励比例：30%, 20%, 15%, 10%, 10%, 5%, 5%, 3%, 1%, 1%
//
// 参数：
//   - totalReward: 总奖励池
//   - rankedNodes: 已排名的节点列表
//
// 返回值：
//   - 节点地址到奖励金额的映射
func (cm *CompetitionManager) DistributeRankingRewards(
	totalReward *big.Int,
	rankedNodes []*NodeMetrics,
) map[common.Address]*big.Int {
	rewards := make(map[common.Address]*big.Int)

	for i, node := range rankedNodes {
		if i >= len(cm.config.RankingRewards) {
			break
		}

		rewardRatio := cm.config.RankingRewards[i]
		reward := new(big.Int).Mul(totalReward, big.NewInt(int64(rewardRatio*100)))
		reward.Div(reward, big.NewInt(100))

		rewards[node.Address] = reward
	}

	return rewards
}

// GetNodeMetrics 获取节点的多维度指标
//
// 从各个管理器收集节点的指标数据
//
// 参数：
//   - addr: 节点地址
//   - blockQuality: 区块质量评分
//   - serviceQuality: 服务质量评分
//
// 返回值：
//   - 节点指标
func (cm *CompetitionManager) GetNodeMetrics(
	addr common.Address,
	blockQuality uint64,
	serviceQuality uint64,
) *NodeMetrics {
	// 获取声誉分数
	reputation := cm.reputationMgr.GetReputationScore(addr)
	if reputation < 0 {
		reputation = 0
	}

	// 获取在线率
	uptimeRatio := cm.onlineRewardMgr.GetUptimeRatio(addr)

	return &NodeMetrics{
		Address:        addr,
		Reputation:     uint64(reputation),
		UptimeRatio:    uptimeRatio,
		BlockQuality:   blockQuality,
		ServiceQuality: serviceQuality,
	}
}

// GetTopNodes 获取排名前 N 的节点
//
// 参数：
//   - nodes: 节点指标列表
//   - n: 返回的节点数量
//
// 返回值：
//   - 排名前 N 的节点列表
func (cm *CompetitionManager) GetTopNodes(nodes []*NodeMetrics, n int) []*NodeMetrics {
	ranked := cm.RankNodes(nodes)
	if len(ranked) > n {
		return ranked[:n]
	}
	return ranked
}
