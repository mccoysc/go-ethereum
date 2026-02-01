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

// PenaltyType represents the penalty type.
type PenaltyType uint8

const (
	PenaltyDoubleSign   PenaltyType = 0x01
	PenaltyOffline      PenaltyType = 0x02
	PenaltyInvalidBlock PenaltyType = 0x03
	PenaltyMalicious    PenaltyType = 0x04
)

// PenaltyRecord represents a penalty record.
type PenaltyRecord struct {
	NodeAddress common.Address
	Type        PenaltyType
	Amount      *big.Int
	Reason      string
	Timestamp   time.Time
	BlockNumber uint64
	Evidence    []byte
}

// PenaltyManager is the penalty manager.
type PenaltyManager struct {
	config  *PenaltyConfig
	records []*PenaltyRecord
}

// NewPenaltyManager creates a new penalty manager.
func NewPenaltyManager(config *PenaltyConfig) *PenaltyManager {
	return &PenaltyManager{
		config:  config,
		records: make([]*PenaltyRecord, 0),
	}
}

// CalculateDoubleSignPenalty calculates the double signing penalty.
//
// Double signing is one of the most serious malicious behaviors, resulting in slashing a certain percentage of the node's staked amount.
//
// Parameters:
//   - nodeBalance: Node's current balance
//
// Returns:
//   - Penalty amount (percentage of node's balance)
func (pm *PenaltyManager) CalculateDoubleSignPenalty(nodeBalance *big.Int) *big.Int {
	penalty := new(big.Int).Mul(nodeBalance, big.NewInt(int64(pm.config.DoubleSignPenaltyPercent)))
	penalty.Div(penalty, big.NewInt(100))
	return penalty
}

// CalculateOfflinePenalty calculates the offline penalty.
//
// Nodes that remain offline for extended periods will be penalized, calculated on an hourly basis.
//
// Parameters:
//   - offlineHours: Number of hours offline
//
// Returns:
//   - Penalty amount (fixed amount per hour)
func (pm *PenaltyManager) CalculateOfflinePenalty(offlineHours uint64) *big.Int {
	penalty := new(big.Int).Mul(pm.config.OfflinePenaltyPerHour, big.NewInt(int64(offlineHours)))
	return penalty
}

// CalculateInvalidBlockPenalty calculates the invalid block penalty.
//
// Producing invalid blocks will result in a fixed penalty amount.
//
// Returns:
//   - Penalty amount (fixed amount)
func (pm *PenaltyManager) CalculateInvalidBlockPenalty() *big.Int {
	return new(big.Int).Set(pm.config.InvalidBlockPenalty)
}

// CalculateMaliciousPenalty calculates the malicious behavior penalty.
//
// Other malicious behaviors (such as data tampering, denial of service attacks, etc.) will result in severe penalties.
//
// Parameters:
//   - nodeBalance: Node's current balance
//
// Returns:
//   - Penalty amount (percentage of node's balance, potentially all of it)
func (pm *PenaltyManager) CalculateMaliciousPenalty(nodeBalance *big.Int) *big.Int {
	penalty := new(big.Int).Mul(nodeBalance, big.NewInt(int64(pm.config.MaliciousPenaltyPercent)))
	penalty.Div(penalty, big.NewInt(100))
	return penalty
}

// CalculatePenalty calculates the penalty amount (generic method).
//
// Calculates the penalty amount based on the penalty type and related information.
//
// Parameters:
//   - penaltyType: Penalty type
//   - nodeBalance: Node balance
//   - additionalInfo: Additional information (such as offline hours)
//
// Returns:
//   - Penalty amount
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

// RecordPenalty records a penalty.
func (pm *PenaltyManager) RecordPenalty(record *PenaltyRecord) {
	pm.records = append(pm.records, record)
}

// GetPenaltyHistory retrieves the node's penalty history.
func (pm *PenaltyManager) GetPenaltyHistory(addr common.Address) []*PenaltyRecord {
	var history []*PenaltyRecord
	for _, record := range pm.records {
		if record.NodeAddress == addr {
			history = append(history, record)
		}
	}
	return history
}

// GetTotalPenalty retrieves the node's total penalty amount.
func (pm *PenaltyManager) GetTotalPenalty(addr common.Address) *big.Int {
	total := big.NewInt(0)
	for _, record := range pm.records {
		if record.NodeAddress == addr {
			total.Add(total, record.Amount)
		}
	}
	return total
}

// GetPenaltyCount retrieves the node's penalty count.
func (pm *PenaltyManager) GetPenaltyCount(addr common.Address) int {
	count := 0
	for _, record := range pm.records {
		if record.NodeAddress == addr {
			count++
		}
	}
	return count
}

// GetPenaltyByType retrieves the node's penalty count for a specific type.
func (pm *PenaltyManager) GetPenaltyByType(addr common.Address, penaltyType PenaltyType) int {
	count := 0
	for _, record := range pm.records {
		if record.NodeAddress == addr && record.Type == penaltyType {
			count++
		}
	}
	return count
}
