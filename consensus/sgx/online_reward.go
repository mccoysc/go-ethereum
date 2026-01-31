package sgx

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// OnlineRewardCalculator 在线奖励计算器
type OnlineRewardCalculator struct {
	config *RewardConfig
}

// NewOnlineRewardCalculator 创建在线奖励计算器
func NewOnlineRewardCalculator(config *RewardConfig) *OnlineRewardCalculator {
	return &OnlineRewardCalculator{
		config: config,
	}
}

// CalculateOnlineReward 计算在线奖励
func (orc *OnlineRewardCalculator) CalculateOnlineReward(address common.Address, uptimeScore uint64) (*big.Int, error) {
	// 基础奖励
	baseReward := new(big.Int).Set(orc.config.OnlineRewardPerEpoch)

	// 根据在线率调整
	multiplier := float64(uptimeScore) / 10000.0
	reward := new(big.Int).Mul(baseReward, big.NewInt(int64(multiplier*1e18)))
	reward.Div(reward, big.NewInt(1e18))

	return reward, nil
}
