package sgx

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

// ReorgHandler 重组处理器
type ReorgHandler struct {
}

// NewReorgHandler 创建重组处理器
func NewReorgHandler() *ReorgHandler {
	return &ReorgHandler{}
}

// HandleReorg 处理链重组
func (rh *ReorgHandler) HandleReorg(chain BlockChain, txPool TxPool, oldHead, newHead *types.Block) error {
	if oldHead == nil || newHead == nil {
		return nil
	}

	log.Info("Handling chain reorganization",
		"old", oldHead.Hash().Hex(),
		"oldNum", oldHead.NumberU64(),
		"new", newHead.Hash().Hex(),
		"newNum", newHead.NumberU64())

	// 1. 找到共同祖先
	ancestor := rh.findCommonAncestor(chain, oldHead, newHead)
	if ancestor == nil {
		log.Error("Failed to find common ancestor")
		return ErrUnknownAncestor
	}

	log.Debug("Found common ancestor", "hash", ancestor.Hash().Hex(), "number", ancestor.NumberU64())

	// 2. 回滚旧链上的交易到交易池
	oldBlocks := rh.getBlocksFromAncestor(chain, oldHead, ancestor)
	txsToRevert := make([]*types.Transaction, 0)
	for _, block := range oldBlocks {
		for _, tx := range block.Transactions() {
			txsToRevert = append(txsToRevert, tx)
		}
	}

	if len(txsToRevert) > 0 {
		log.Info("Reverting transactions from old chain", "count", len(txsToRevert))
		// 将交易重新加入交易池
		errs := txPool.Add(txsToRevert, false, false)
		for i, err := range errs {
			if err != nil {
				log.Debug("Failed to revert transaction", "tx", txsToRevert[i].Hash().Hex(), "err", err)
			}
		}
	}

	// 3. 从交易池移除新链上已确认的交易
	newBlocks := rh.getBlocksFromAncestor(chain, newHead, ancestor)
	for _, block := range newBlocks {
		for _, tx := range block.Transactions() {
			txPool.Remove(tx.Hash())
		}
	}

	log.Info("Chain reorganization completed",
		"reverted", len(txsToRevert),
		"confirmed", len(newBlocks))

	return nil
}

// findCommonAncestor 找到共同祖先
func (rh *ReorgHandler) findCommonAncestor(chain BlockChain, a, b *types.Block) *types.Block {
	if a == nil || b == nil {
		return nil
	}

	// 将两个区块回溯到相同高度
	for a.NumberU64() > b.NumberU64() {
		parent := chain.GetBlock(a.ParentHash(), a.NumberU64()-1)
		if parent == nil {
			return nil
		}
		a = parent
	}
	for b.NumberU64() > a.NumberU64() {
		parent := chain.GetBlock(b.ParentHash(), b.NumberU64()-1)
		if parent == nil {
			return nil
		}
		b = parent
	}

	// 同时回溯直到找到相同区块
	for a.Hash() != b.Hash() {
		if a.NumberU64() == 0 {
			// 到达创世区块但仍未找到共同祖先
			return nil
		}

		parentA := chain.GetBlock(a.ParentHash(), a.NumberU64()-1)
		parentB := chain.GetBlock(b.ParentHash(), b.NumberU64()-1)
		
		if parentA == nil || parentB == nil {
			return nil
		}

		a = parentA
		b = parentB
	}

	return a
}

// getBlocksFromAncestor 获取从祖先到目标的所有区块
func (rh *ReorgHandler) getBlocksFromAncestor(chain BlockChain, target, ancestor *types.Block) []*types.Block {
	var blocks []*types.Block
	
	if target == nil || ancestor == nil {
		return blocks
	}

	current := target

	for current.Hash() != ancestor.Hash() {
		blocks = append(blocks, current)
		
		if current.NumberU64() == 0 {
			// 到达创世区块
			break
		}

		parent := chain.GetBlock(current.ParentHash(), current.NumberU64()-1)
		if parent == nil {
			break
		}
		current = parent
	}

	// 反转顺序（从祖先到目标）
	for i, j := 0, len(blocks)-1; i < j; i, j = i+1, j-1 {
		blocks[i], blocks[j] = blocks[j], blocks[i]
	}

	return blocks
}

// EstimateReorgDepth 估算重组深度
func (rh *ReorgHandler) EstimateReorgDepth(chain BlockChain, oldHead, newHead *types.Block) uint64 {
	if oldHead == nil || newHead == nil {
		return 0
	}

	ancestor := rh.findCommonAncestor(chain, oldHead, newHead)
	if ancestor == nil {
		return 0
	}

	// 计算旧链从祖先到头部的深度
	depth := oldHead.NumberU64() - ancestor.NumberU64()
	return depth
}

// GetAffectedAddresses 获取受重组影响的地址列表
func (rh *ReorgHandler) GetAffectedAddresses(chain BlockChain, oldHead, newHead *types.Block) []common.Address {
	addressSet := make(map[common.Address]bool)

	ancestor := rh.findCommonAncestor(chain, oldHead, newHead)
	if ancestor == nil {
		return nil
	}

	// 收集旧链上的交易地址
	oldBlocks := rh.getBlocksFromAncestor(chain, oldHead, ancestor)
	for _, block := range oldBlocks {
		for _, tx := range block.Transactions() {
			if from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx); err == nil {
				addressSet[from] = true
			}
			if tx.To() != nil {
				addressSet[*tx.To()] = true
			}
		}
	}

	// 收集新链上的交易地址
	newBlocks := rh.getBlocksFromAncestor(chain, newHead, ancestor)
	for _, block := range newBlocks {
		for _, tx := range block.Transactions() {
			if from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx); err == nil {
				addressSet[from] = true
			}
			if tx.To() != nil {
				addressSet[*tx.To()] = true
			}
		}
	}

	// 转换为列表
	addresses := make([]common.Address, 0, len(addressSet))
	for addr := range addressSet {
		addresses = append(addresses, addr)
	}

	return addresses
}
