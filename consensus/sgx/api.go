package sgx

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
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
func (api *API) GetBlockQuality(blockHash common.Hash) (*BlockQuality, error) {
	header := api.chain.GetHeaderByHash(blockHash)
	if header == nil {
		return nil, ErrInvalidBlock
	}

	// Note: In a real implementation, we would need to get the full block
	// This is a simplified version
	return nil, nil
}

// GetNodeReputation returns the reputation data for a node
func (api *API) GetNodeReputation(address common.Address) (*NodeReputation, error) {
	return api.engine.reputationSystem.GetReputation(address)
}

// GetUptimeScore returns the uptime score for a node
func (api *API) GetUptimeScore(address common.Address) (*UptimeData, error) {
	// Use default network statistics for API calls
	// In production, these should come from a network statistics tracker
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
