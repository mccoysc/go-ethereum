package sgx

import (
"github.com/ethereum/go-ethereum/common"
"github.com/ethereum/go-ethereum/core/txpool"
"github.com/ethereum/go-ethereum/core/types"
)

// TxPoolAdapter adapts core/txpool.TxPool to sgx.TxPool interface
type TxPoolAdapter struct {
pool *txpool.TxPool
}

// NewTxPoolAdapter creates a new adapter
func NewTxPoolAdapter(pool *txpool.TxPool) *TxPoolAdapter {
return &TxPoolAdapter{pool: pool}
}

// Pending implements sgx.TxPool interface
func (a *TxPoolAdapter) Pending(enforceTips bool) map[common.Address][]*types.Transaction {
// Convert bool to PendingFilter struct
filter := txpool.PendingFilter{
MinTip:      nil,
BaseFee:     nil,
BlobFee:     nil,
BlobTxs:     false,
GasLimitCap: 0,
BlobVersion: 0,
}

lazyTxs := a.pool.Pending(filter)
result := make(map[common.Address][]*types.Transaction)

// Convert LazyTransaction to Transaction
for addr, txList := range lazyTxs {
txs := make([]*types.Transaction, len(txList))
for i, lazyTx := range txList {
txs[i] = lazyTx.Resolve()
}
result[addr] = txs
}

return result
}

// PendingCount implements sgx.TxPool interface
func (a *TxPoolAdapter) PendingCount() int {
pending := a.Pending(false)
count := 0
for _, txs := range pending {
count += len(txs)
}
return count
}

// Add implements sgx.TxPool interface
func (a *TxPoolAdapter) Add(txs []*types.Transaction, sync bool) []error {
return a.pool.Add(txs, sync)
}

// Remove implements sgx.TxPool interface  
func (a *TxPoolAdapter) Remove(txHash common.Hash) {
// txpool doesn't have a public Remove method
// This is a no-op for now
}
