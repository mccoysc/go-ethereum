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
	"bytes"
	"encoding/binary"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	// Storage key prefixes
	reputationPrefix      = []byte("reputation")
	onlineStatusPrefix    = []byte("onlineStatus")
	penaltyRecordPrefix   = []byte("penaltyRecord")
	competitionPrefix     = []byte("competition")
	rewardHistoryPrefix   = []byte("rewardHistory")
	blockQualityPrefix    = []byte("blockQuality")
)

// StorageManager 状态存储管理器
type StorageManager struct {
	contractAddr common.Address
}

// NewStorageManager 创建存储管理器
func NewStorageManager(contractAddr common.Address) *StorageManager {
	return &StorageManager{
		contractAddr: contractAddr,
	}
}

// SaveReputation 保存节点声誉到 StateDB
func (sm *StorageManager) SaveReputation(stateDB *state.StateDB, addr common.Address, rep *NodeReputation) error {
	key := sm.makeKey(reputationPrefix, addr.Bytes())

	data := &reputationData{
		Score:           rep.Score,
		TotalBlocks:     rep.TotalBlocks,
		SuccessBlocks:   rep.SuccessBlocks,
		FailedBlocks:    rep.FailedBlocks,
		MaliciousCount:  rep.MaliciousCount,
		PenaltyCount:    rep.PenaltyCount,
		OfflineHours:    rep.OfflineHours,
		OnlineHours:     rep.OnlineHours,
		LastUpdateTime:  rep.LastUpdateTime.Unix(),
		LastDecayTime:   rep.LastDecayTime.Unix(),
		LastOnlineCheck: rep.LastOnlineCheck.Unix(),
	}

	encoded, err := rlp.EncodeToBytes(data)
	if err != nil {
		return err
	}

	stateDB.SetState(sm.contractAddr, key, common.BytesToHash(encoded))
	return nil
}

// LoadReputation 从 StateDB 加载节点声誉
func (sm *StorageManager) LoadReputation(stateDB *state.StateDB, addr common.Address) (*NodeReputation, error) {
	key := sm.makeKey(reputationPrefix, addr.Bytes())
	value := stateDB.GetState(sm.contractAddr, key)

	if value == (common.Hash{}) {
		return nil, nil
	}

	var data reputationData
	if err := rlp.DecodeBytes(value.Bytes(), &data); err != nil {
		return nil, err
	}

	return &NodeReputation{
		Address:         addr,
		Score:           data.Score,
		TotalBlocks:     data.TotalBlocks,
		SuccessBlocks:   data.SuccessBlocks,
		FailedBlocks:    data.FailedBlocks,
		MaliciousCount:  data.MaliciousCount,
		PenaltyCount:    data.PenaltyCount,
		OfflineHours:    data.OfflineHours,
		OnlineHours:     data.OnlineHours,
		LastUpdateTime:  time.Unix(data.LastUpdateTime, 0),
		LastDecayTime:   time.Unix(data.LastDecayTime, 0),
		LastOnlineCheck: time.Unix(data.LastOnlineCheck, 0),
	}, nil
}

// SaveOnlineStatus 保存在线状态到 StateDB
func (sm *StorageManager) SaveOnlineStatus(stateDB *state.StateDB, addr common.Address, status *NodeOnlineStatus) error {
	key := sm.makeKey(onlineStatusPrefix, addr.Bytes())

	data := &onlineStatusData{
		LastHeartbeat:     status.LastHeartbeat.Unix(),
		OnlineStartTime:   status.OnlineStartTime.Unix(),
		TotalOnlineTime:   int64(status.TotalOnlineTime),
		TotalOfflineTime:  int64(status.TotalOfflineTime),
		HeartbeatCount:    status.HeartbeatCount,
		MissedHeartbeats:  status.MissedHeartbeats,
		AccumulatedReward: status.AccumulatedReward.Bytes(),
		ClaimedReward:     status.ClaimedReward.Bytes(),
	}

	encoded, err := rlp.EncodeToBytes(data)
	if err != nil {
		return err
	}

	stateDB.SetState(sm.contractAddr, key, common.BytesToHash(encoded))
	return nil
}

// LoadOnlineStatus 从 StateDB 加载在线状态
func (sm *StorageManager) LoadOnlineStatus(stateDB *state.StateDB, addr common.Address) (*NodeOnlineStatus, error) {
	key := sm.makeKey(onlineStatusPrefix, addr.Bytes())
	value := stateDB.GetState(sm.contractAddr, key)

	if value == (common.Hash{}) {
		return nil, nil
	}

	var data onlineStatusData
	if err := rlp.DecodeBytes(value.Bytes(), &data); err != nil {
		return nil, err
	}

	return &NodeOnlineStatus{
		Address:           addr,
		LastHeartbeat:     time.Unix(data.LastHeartbeat, 0),
		OnlineStartTime:   time.Unix(data.OnlineStartTime, 0),
		TotalOnlineTime:   time.Duration(data.TotalOnlineTime),
		TotalOfflineTime:  time.Duration(data.TotalOfflineTime),
		HeartbeatCount:    data.HeartbeatCount,
		MissedHeartbeats:  data.MissedHeartbeats,
		AccumulatedReward: new(big.Int).SetBytes(data.AccumulatedReward),
		ClaimedReward:     new(big.Int).SetBytes(data.ClaimedReward),
	}, nil
}

// SavePenaltyRecord 保存惩罚记录到 StateDB
func (sm *StorageManager) SavePenaltyRecord(stateDB *state.StateDB, record *PenaltyRecord) error {
	// 使用组合键：地址 + 时间戳
	keyData := append(record.NodeAddress.Bytes(), sm.uint64ToBytes(uint64(record.Timestamp.Unix()))...)
	key := sm.makeKey(penaltyRecordPrefix, keyData)

	data := &penaltyRecordData{
		Type:        uint8(record.Type),
		Amount:      record.Amount.Bytes(),
		Reason:      record.Reason,
		Timestamp:   record.Timestamp.Unix(),
		BlockNumber: record.BlockNumber,
		Evidence:    record.Evidence,
	}

	encoded, err := rlp.EncodeToBytes(data)
	if err != nil {
		return err
	}

	stateDB.SetState(sm.contractAddr, key, common.BytesToHash(encoded))
	return nil
}

// SaveBlockQuality 保存区块质量评分到 StateDB
func (sm *StorageManager) SaveBlockQuality(stateDB *state.StateDB, blockHash common.Hash, quality *BlockQuality) error {
	key := sm.makeKey(blockQualityPrefix, blockHash.Bytes())

	data := &blockQualityData{
		TxCount:          quality.TxCount,
		BlockSize:        quality.BlockSize,
		GasUsed:          quality.GasUsed,
		NewTxCount:       quality.NewTxCount,
		UniqueSenders:    quality.UniqueSenders,
		RewardMultiplier: sm.float64ToBytes(quality.RewardMultiplier),
	}

	encoded, err := rlp.EncodeToBytes(data)
	if err != nil {
		return err
	}

	stateDB.SetState(sm.contractAddr, key, common.BytesToHash(encoded))
	return nil
}

// LoadBlockQuality 从 StateDB 加载区块质量评分
func (sm *StorageManager) LoadBlockQuality(stateDB *state.StateDB, blockHash common.Hash) (*BlockQuality, error) {
	key := sm.makeKey(blockQualityPrefix, blockHash.Bytes())
	value := stateDB.GetState(sm.contractAddr, key)

	if value == (common.Hash{}) {
		return nil, nil
	}

	var data blockQualityData
	if err := rlp.DecodeBytes(value.Bytes(), &data); err != nil {
		return nil, err
	}

	return &BlockQuality{
		TxCount:          data.TxCount,
		BlockSize:        data.BlockSize,
		GasUsed:          data.GasUsed,
		NewTxCount:       data.NewTxCount,
		UniqueSenders:    data.UniqueSenders,
		RewardMultiplier: sm.bytesToFloat64(data.RewardMultiplier),
	}, nil
}

// makeKey 生成存储键
func (sm *StorageManager) makeKey(prefix []byte, data []byte) common.Hash {
	combined := append(prefix, data...)
	return crypto.Keccak256Hash(combined)
}

// uint64ToBytes 将 uint64 转换为字节数组
func (sm *StorageManager) uint64ToBytes(val uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, val)
	return buf
}

// float64ToBytes 将 float64 转换为字节数组
func (sm *StorageManager) float64ToBytes(val float64) []byte {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, val); err != nil {
		return make([]byte, 8)
	}
	return buf.Bytes()
}

// bytesToFloat64 将字节数组转换为 float64
func (sm *StorageManager) bytesToFloat64(b []byte) float64 {
	var val float64
	buf := bytes.NewReader(b)
	if err := binary.Read(buf, binary.BigEndian, &val); err != nil {
		return 0
	}
	return val
}

// Storage data structures for RLP encoding

type reputationData struct {
	Score           int64
	TotalBlocks     uint64
	SuccessBlocks   uint64
	FailedBlocks    uint64
	MaliciousCount  uint64
	PenaltyCount    uint64
	OfflineHours    uint64
	OnlineHours     uint64
	LastUpdateTime  int64
	LastDecayTime   int64
	LastOnlineCheck int64
}

type onlineStatusData struct {
	LastHeartbeat     int64
	OnlineStartTime   int64
	TotalOnlineTime   int64
	TotalOfflineTime  int64
	HeartbeatCount    uint64
	MissedHeartbeats  uint64
	AccumulatedReward []byte
	ClaimedReward     []byte
}

type penaltyRecordData struct {
	Type        uint8
	Amount      []byte
	Reason      string
	Timestamp   int64
	BlockNumber uint64
	Evidence    []byte
}

type blockQualityData struct {
	TxCount          uint64
	BlockSize        uint64
	GasUsed          uint64
	NewTxCount       uint64
	UniqueSenders    uint64
	RewardMultiplier []byte
}
