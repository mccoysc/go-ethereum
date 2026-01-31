package sgx

import (
	"bytes"

	"github.com/ethereum/go-ethereum/core/types"
)

// ForkChoiceRule 分叉选择规则
type ForkChoiceRule struct {
}

// NewForkChoiceRule 创建分叉选择规则
func NewForkChoiceRule() *ForkChoiceRule {
	return &ForkChoiceRule{}
}

// SelectCanonicalBlock 选择规范区块
// 当两个区块高度相同时，使用以下规则选择：
// 1. 优先选择交易数更多的区块
// 2. 交易数相同时，选择时间戳更早的区块
// 3. 时间戳相同时，选择区块哈希更小的区块（确定性）
func (fc *ForkChoiceRule) SelectCanonicalBlock(a, b *types.Block) *types.Block {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}

	// 规则 1: 交易数
	if len(a.Transactions()) != len(b.Transactions()) {
		if len(a.Transactions()) > len(b.Transactions()) {
			return a
		}
		return b
	}

	// 规则 2: 时间戳（更早的优先）
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

// SelectCanonicalHeader 选择规范区块头
// Note: Since headers don't contain the full transaction list,
// we use a simplified rule set compared to SelectCanonicalBlock.
// For full fork choice including transaction count, use SelectCanonicalBlock.
func (fc *ForkChoiceRule) SelectCanonicalHeader(a, b *types.Header) *types.Header {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}

	// 规则 1: 时间戳（更早的优先）
	if a.Time != b.Time {
		if a.Time < b.Time {
			return a
		}
		return b
	}

	// 规则 2: 区块哈希（确定性）
	if bytes.Compare(a.Hash().Bytes(), b.Hash().Bytes()) < 0 {
		return a
	}
	return b
}

// CompareBlocks 比较两个区块
// 返回值: -1 表示 a 更优, 0 表示相等, 1 表示 b 更优
func (fc *ForkChoiceRule) CompareBlocks(a, b *types.Block) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return 1
	}
	if b == nil {
		return -1
	}

	// 比较交易数
	aTxCount := len(a.Transactions())
	bTxCount := len(b.Transactions())
	if aTxCount != bTxCount {
		if aTxCount > bTxCount {
			return -1
		}
		return 1
	}

	// 比较时间戳
	if a.Time() != b.Time() {
		if a.Time() < b.Time() {
			return -1
		}
		return 1
	}

	// 比较哈希
	cmp := bytes.Compare(a.Hash().Bytes(), b.Hash().Bytes())
	if cmp < 0 {
		return -1
	} else if cmp > 0 {
		return 1
	}
	return 0
}

// SelectBestCandidate 从多个候选区块中选择最优区块
func (fc *ForkChoiceRule) SelectBestCandidate(candidates []*BlockCandidate) *BlockCandidate {
	if len(candidates) == 0 {
		return nil
	}

	best := candidates[0]
	for i := 1; i < len(candidates); i++ {
		if fc.CompareBlocks(candidates[i].Block, best.Block) < 0 {
			best = candidates[i]
		}
	}

	return best
}
