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

// CompetitionDimension represents the competition dimension.
type CompetitionDimension uint8

const (
	DimensionReputation     CompetitionDimension = 0x01
	DimensionUptime         CompetitionDimension = 0x02
	DimensionBlockQuality   CompetitionDimension = 0x03
	DimensionServiceQuality CompetitionDimension = 0x04
)

// NodeMetrics represents the node metrics.
type NodeMetrics struct {
	Address        common.Address
	Reputation     uint64
	UptimeRatio    float64
	BlockQuality   uint64
	ServiceQuality uint64
}

// CompetitionManager is the competition manager.
type CompetitionManager struct {
	config             *CompetitionConfig
	reputationMgr      *ReputationManager
	onlineRewardMgr    *OnlineRewardManager
	blockQualityScorer *BlockQualityScorer
}

// NewCompetitionManager creates a new competition manager.
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

// CalculateComprehensiveScore calculates the comprehensive score.
//
// The comprehensive score is based on four dimensions:
// 1. Reputation score (weight 30%)
// 2. Uptime ratio score (weight 25%)
// 3. Block quality score (weight 25%)
// 4. Service quality score (weight 20%)
//
// Parameters:
//   - metrics: Node metrics
//
// Returns:
//   - Comprehensive score (0-100)
func (cm *CompetitionManager) CalculateComprehensiveScore(metrics *NodeMetrics) uint64 {
	score := uint64(0)

	// Reputation score (normalized to 0-100)
	normalizedReputation := metrics.Reputation
	if normalizedReputation > 100 {
		normalizedReputation = 100
	}
	reputationScore := normalizedReputation * uint64(cm.config.ReputationWeight*100) / 100
	score += reputationScore

	// Uptime ratio score (0-100)
	uptimeScore := uint64(metrics.UptimeRatio*100) * uint64(cm.config.UptimeWeight*100) / 100
	score += uptimeScore

	// Block quality score (0-100)
	qualityScore := metrics.BlockQuality * uint64(cm.config.BlockQualityWeight*100) / 100
	score += qualityScore

	// Service quality score (0-100)
	serviceScore := metrics.ServiceQuality * uint64(cm.config.ServiceQualityWeight*100) / 100
	score += serviceScore

	return score
}

// RankNodes ranks the nodes.
//
// Ranks nodes based on their comprehensive score, with higher scores ranked first.
//
// Parameters:
//   - nodes: List of node metrics
//
// Returns:
//   - Sorted list of nodes
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

	// Sort by score (from high to low)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return sorted nodes
	result := make([]*NodeMetrics, len(nodes))
	for i, s := range scored {
		result[i] = s.metrics
	}

	return result
}

// DistributeRankingRewards distributes ranking rewards.
//
// Distributes rewards to the top 10 nodes based on their ranking.
// Reward ratios: 30%, 20%, 15%, 10%, 10%, 5%, 5%, 3%, 1%, 1%
//
// Parameters:
//   - totalReward: Total reward pool
//   - rankedNodes: List of ranked nodes
//
// Returns:
//   - Mapping from node address to reward amount
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

// GetNodeMetrics retrieves the node's multi-dimensional metrics.
//
// Collects the node's metric data from various managers.
//
// Parameters:
//   - addr: Node address
//   - blockQuality: Block quality score
//   - serviceQuality: Service quality score
//
// Returns:
//   - Node metrics
func (cm *CompetitionManager) GetNodeMetrics(
	addr common.Address,
	blockQuality uint64,
	serviceQuality uint64,
) *NodeMetrics {
	// Get reputation score
	reputation := cm.reputationMgr.GetReputationScore(addr)
	if reputation < 0 {
		reputation = 0
	}

	// Get uptime ratio
	uptimeRatio := cm.onlineRewardMgr.GetUptimeRatio(addr)

	return &NodeMetrics{
		Address:        addr,
		Reputation:     uint64(reputation),
		UptimeRatio:    uptimeRatio,
		BlockQuality:   blockQuality,
		ServiceQuality: serviceQuality,
	}
}

// GetTopNodes retrieves the top N ranked nodes.
//
// Parameters:
//   - nodes: List of node metrics
//   - n: Number of nodes to return
//
// Returns:
//   - List of top N nodes
func (cm *CompetitionManager) GetTopNodes(nodes []*NodeMetrics, n int) []*NodeMetrics {
	ranked := cm.RankNodes(nodes)
	if len(ranked) > n {
		return ranked[:n]
	}
	return ranked
}
