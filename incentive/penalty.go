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
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// PenaltyType 惩罚类型
type PenaltyType uint8

const (
	PenaltyDoubleSign   PenaltyType = 0x01
	PenaltyOffline      PenaltyType = 0x02
	PenaltyInvalidBlock PenaltyType = 0x03
	PenaltyMalicious    PenaltyType = 0x04
)

// PenaltyRecord 惩罚记录
type PenaltyRecord struct {
	NodeAddress common.Address
	Type        PenaltyType
	Amount      *big.Int
	Reason      string
	Timestamp   time.Time
	BlockNumber uint64
	Evidence    []byte
}

// PenaltyManager 惩罚管理器
type PenaltyManager struct {
	config  *PenaltyConfig
	records []*PenaltyRecord
}

// NewPenaltyManager 创建惩罚管理器
func NewPenaltyManager(config *PenaltyConfig) *PenaltyManager {
	return &PenaltyManager{
		config:  config,
		records: make([]*PenaltyRecord, 0),
	}
}

// CalculateDoubleSignPenalty 计算双重签名惩罚
//
// 双重签名是最严重的恶意行为之一，罚没节点质押金额的一定比例
//
// 参数：
//   - nodeBalance: 节点当前余额
//
// 返回值：
//   - 惩罚金额（节点余额的百分比）
func (pm *PenaltyManager) CalculateDoubleSignPenalty(nodeBalance *big.Int) *big.Int {
	penalty := new(big.Int).Mul(nodeBalance, big.NewInt(int64(pm.config.DoubleSignPenaltyPercent)))
	penalty.Div(penalty, big.NewInt(100))
	return penalty
}

// CalculateOfflinePenalty 计算离线惩罚
//
// 节点长期离线会受到惩罚，按小时计算
//
// 参数：
//   - offlineHours: 离线小时数
//
// 返回值：
//   - 惩罚金额（每小时固定金额）
func (pm *PenaltyManager) CalculateOfflinePenalty(offlineHours uint64) *big.Int {
	penalty := new(big.Int).Mul(pm.config.OfflinePenaltyPerHour, big.NewInt(int64(offlineHours)))
	return penalty
}

// CalculateInvalidBlockPenalty 计算无效区块惩罚
//
// 生产无效区块会受到固定金额的惩罚
//
// 返回值：
//   - 惩罚金额（固定金额）
func (pm *PenaltyManager) CalculateInvalidBlockPenalty() *big.Int {
	return new(big.Int).Set(pm.config.InvalidBlockPenalty)
}

// CalculateMaliciousPenalty 计算恶意行为惩罚
//
// 其他恶意行为（如数据篡改、拒绝服务攻击等）会受到严厉惩罚
//
// 参数：
//   - nodeBalance: 节点当前余额
//
// 返回值：
//   - 惩罚金额（节点余额的百分比，可能是全部）
func (pm *PenaltyManager) CalculateMaliciousPenalty(nodeBalance *big.Int) *big.Int {
	penalty := new(big.Int).Mul(nodeBalance, big.NewInt(int64(pm.config.MaliciousPenaltyPercent)))
	penalty.Div(penalty, big.NewInt(100))
	return penalty
}

// CalculatePenalty 计算惩罚金额（通用方法）
//
// 根据惩罚类型和相关信息计算惩罚金额
//
// 参数：
//   - penaltyType: 惩罚类型
//   - nodeBalance: 节点余额
//   - additionalInfo: 额外信息（如离线小时数）
//
// 返回值：
//   - 惩罚金额
func (pm *PenaltyManager) CalculatePenalty(
	penaltyType PenaltyType,
	nodeBalance *big.Int,
	additionalInfo interface{},
) *big.Int {
	switch penaltyType {
	case PenaltyDoubleSign:
		return pm.CalculateDoubleSignPenalty(nodeBalance)

	case PenaltyOffline:
		hours, ok := additionalInfo.(uint64)
		if !ok {
			return big.NewInt(0)
		}
		return pm.CalculateOfflinePenalty(hours)

	case PenaltyInvalidBlock:
		return pm.CalculateInvalidBlockPenalty()

	case PenaltyMalicious:
		return pm.CalculateMaliciousPenalty(nodeBalance)

	default:
		return big.NewInt(0)
	}
}

// RecordPenalty 记录惩罚
func (pm *PenaltyManager) RecordPenalty(record *PenaltyRecord) {
	pm.records = append(pm.records, record)
}

// GetPenaltyHistory 获取节点的惩罚历史
func (pm *PenaltyManager) GetPenaltyHistory(addr common.Address) []*PenaltyRecord {
	var history []*PenaltyRecord
	for _, record := range pm.records {
		if record.NodeAddress == addr {
			history = append(history, record)
		}
	}
	return history
}

// GetTotalPenalty 获取节点的总惩罚金额
func (pm *PenaltyManager) GetTotalPenalty(addr common.Address) *big.Int {
	total := big.NewInt(0)
	for _, record := range pm.records {
		if record.NodeAddress == addr {
			total.Add(total, record.Amount)
		}
	}
	return total
}

// GetPenaltyCount 获取节点的惩罚次数
func (pm *PenaltyManager) GetPenaltyCount(addr common.Address) int {
	count := 0
	for _, record := range pm.records {
		if record.NodeAddress == addr {
			count++
		}
	}
	return count
}

// GetPenaltyByType 获取节点特定类型的惩罚次数
func (pm *PenaltyManager) GetPenaltyByType(addr common.Address, penaltyType PenaltyType) int {
	count := 0
	for _, record := range pm.records {
		if record.NodeAddress == addr && record.Type == penaltyType {
			count++
		}
	}
	return count
}
