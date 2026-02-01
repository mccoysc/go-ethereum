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
)

// RewardCalculator is the reward calculator.
type RewardCalculator struct {
	config *RewardConfig
}

// NewRewardCalculator creates a new reward calculator.
func NewRewardCalculator(config *RewardConfig) *RewardCalculator {
	return &RewardCalculator{
		config: config,
	}
}

// CalculateBlockReward calculates the block reward.
//
// Reward decay formula:
// reward = baseReward × (1 - decayRate)^(blockNumber / decayPeriod)
//
// Parameters:
//   blockNumber: Current block number
//
// Returns:
//   Base reward for the current block
func (r *RewardCalculator) CalculateBlockReward(blockNumber uint64) *big.Int {
	// Calculate the number of decay periods
	periods := blockNumber / r.config.DecayPeriod
	
	if periods == 0 {
		// Has not reached the first decay period, return base reward
		return new(big.Int).Set(r.config.BaseBlockReward)
	}
	
	// Calculate decayed reward
	// reward = baseReward × (1 - decayRate/100)^periods
	reward := new(big.Int).Set(r.config.BaseBlockReward)
	
	// Calculate (100 - decayRate)^periods / 100^periods
	decayMultiplier := big.NewInt(int64(100 - r.config.DecayRate))
	divisor := big.NewInt(100)
	
	for i := uint64(0); i < periods; i++ {
		reward.Mul(reward, decayMultiplier)
		reward.Div(reward, divisor)
	}
	
	// Ensure reward is not below the minimum
	if reward.Cmp(r.config.MinBlockReward) < 0 {
		reward = new(big.Int).Set(r.config.MinBlockReward)
	}
	
	return reward
}

// CalculateTotalReward calculates the total reward (block reward + transaction fees).
//
// Parameters:
//   blockNumber: Block number
//   totalFees: Total transaction fees
//
// Returns:
//   Total reward amount
func (r *RewardCalculator) CalculateTotalReward(blockNumber uint64, totalFees *big.Int) *big.Int {
	blockReward := r.CalculateBlockReward(blockNumber)
	total := new(big.Int).Add(blockReward, totalFees)
	return total
}
