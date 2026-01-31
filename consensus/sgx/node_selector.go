package sgx

import (
	"sort"

	"github.com/ethereum/go-ethereum/common"
)

// NodeSelector 节点选择器
type NodeSelector struct {
	reputationSystem *ReputationSystem
}

// NewNodeSelector 创建节点选择器
func NewNodeSelector(reputationSystem *ReputationSystem) *NodeSelector {
	return &NodeSelector{
		reputationSystem: reputationSystem,
	}
}

// SelectNodes 选择节点（按优先级排序）
func (ns *NodeSelector) SelectNodes(candidates []common.Address, count int) []common.Address {
	if count <= 0 || len(candidates) == 0 {
		return nil
	}

	// 获取所有节点的优先级
	type nodePriority struct {
		address  common.Address
		priority uint64
	}

	priorities := make([]nodePriority, 0, len(candidates))
	for _, addr := range candidates {
		priority, _ := ns.reputationSystem.GetNodePriority(addr)
		priorities = append(priorities, nodePriority{
			address:  addr,
			priority: priority,
		})
	}

	// 按优先级排序
	sort.Slice(priorities, func(i, j int) bool {
		return priorities[i].priority > priorities[j].priority
	})

	// 选择前 count 个
	if count > len(priorities) {
		count = len(priorities)
	}

	selected := make([]common.Address, count)
	for i := 0; i < count; i++ {
		selected[i] = priorities[i].address
	}

	return selected
}
