package sgx

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
)

// API RPC API for SGX consensus
type API struct {
	engine *SGXEngine
	chain  consensus.ChainHeaderReader
}

// NewAPI creates a new SGX API
func NewAPI(engine *SGXEngine, chain consensus.ChainHeaderReader) *API {
	return &API{
		engine: engine,
		chain:  chain,
	}
}

// GetBlockQuality returns the quality score for a block
// Note: This function requires access to full block data.
// If the chain only provides headers, quality calculation will be limited.
func (api *API) GetBlockQuality(blockHash common.Hash) (*BlockQuality, error) {
	header := api.chain.GetHeaderByHash(blockHash)
	if header == nil {
		return nil, ErrInvalidBlock
	}

	// Try to get full block if chain supports it
	// The ChainHeaderReader interface doesn't include GetBlock,
	// so we need to type assert if we want the full block
	var block *types.Block
	if chainWithBlocks, ok := api.chain.(interface {
		GetBlock(hash common.Hash, number uint64) *types.Block
	}); ok {
		block = chainWithBlocks.GetBlock(blockHash, header.Number.Uint64())
	}

	if block == nil {
		// Block not found in chain
		return nil, ErrInvalidBlock
	}

	// Calculate block quality using the quality scorer
	quality := api.engine.blockQualityScorer.CalculateQuality(block)
	return quality, nil
}

// GetNodeReputation returns the reputation data for a node
func (api *API) GetNodeReputation(address common.Address) (*NodeReputation, error) {
	return api.engine.reputationSystem.GetReputation(address)
}

// GetUptimeScore returns the uptime score for a node
func (api *API) GetUptimeScore(address common.Address) (*UptimeData, error) {
	// Network statistics constants for uptime calculation
	const (
		defaultObservers = 10
		defaultTotalTxs  = uint64(10000)
		defaultTotalGas  = uint64(300000000)
	)
	
	uptimeData := api.engine.uptimeCalculator.CalculateUptimeScore(
		address,
		defaultObservers,
		defaultTotalTxs,
		defaultTotalGas,
	)
	return uptimeData, nil
}

// IsNodeExcluded checks if a node is excluded due to penalties
func (api *API) IsNodeExcluded(address common.Address) bool {
	return api.engine.reputationSystem.IsExcluded(address)
}

// GetConfig returns the current SGX engine configuration
func (api *API) GetConfig() *Config {
	return api.engine.config
}

// GetPenaltyCount returns the penalty count for a node
func (api *API) GetPenaltyCount(address common.Address) (uint64, error) {
	return api.engine.penaltyManager.GetPenaltyCount(address)
}

// GetNodePriority returns the priority score for a node
func (api *API) GetNodePriority(address common.Address) (uint64, error) {
	return api.engine.reputationSystem.GetNodePriority(address)
}
