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

// RewardCalculator 奖励计算器
type RewardCalculator struct {
	config *RewardConfig
}

// NewRewardCalculator 创建奖励计算器
func NewRewardCalculator(config *RewardConfig) *RewardCalculator {
	return &RewardCalculator{
		config: config,
	}
}

// CalculateBlockReward 计算区块奖励
//
// 奖励衰减公式：
// reward = baseReward × (1 - decayRate)^(blockNumber / decayPeriod)
//
// 参数：
//   blockNumber: 当前区块号
//
// 返回值：
//   当前区块的基础奖励
func (r *RewardCalculator) CalculateBlockReward(blockNumber uint64) *big.Int {
	// 计算衰减周期数
	periods := blockNumber / r.config.DecayPeriod
	
	if periods == 0 {
		// 未达到第一个衰减周期，返回基础奖励
		return new(big.Int).Set(r.config.BaseBlockReward)
	}
	
	// 计算衰减后的奖励
	// reward = baseReward × (1 - decayRate/100)^periods
	reward := new(big.Int).Set(r.config.BaseBlockReward)
	
	// 计算 (100 - decayRate)^periods / 100^periods
	decayMultiplier := big.NewInt(int64(100 - r.config.DecayRate))
	divisor := big.NewInt(100)
	
	for i := uint64(0); i < periods; i++ {
		reward.Mul(reward, decayMultiplier)
		reward.Div(reward, divisor)
	}
	
	// 确保不低于最小奖励
	if reward.Cmp(r.config.MinBlockReward) < 0 {
		reward = new(big.Int).Set(r.config.MinBlockReward)
	}
	
	return reward
}

// CalculateTotalReward 计算总奖励（区块奖励 + 交易费）
//
// 参数：
//   blockNumber: 区块号
//   totalFees: 总交易费
//
// 返回值：
//   总奖励金额
func (r *RewardCalculator) CalculateTotalReward(blockNumber uint64, totalFees *big.Int) *big.Int {
	blockReward := r.CalculateBlockReward(blockNumber)
	total := new(big.Int).Add(blockReward, totalFees)
	return total
}
