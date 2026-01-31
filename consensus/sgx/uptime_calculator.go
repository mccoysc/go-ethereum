package sgx

import (
	"github.com/ethereum/go-ethereum/common"
)

// UptimeCalculator 综合在线率计算器
type UptimeCalculator struct {
	config                 *UptimeConfig
	heartbeatTracker       *HeartbeatTracker
	uptimeObserver         *UptimeObserver
	txParticipationTracker *TxParticipationTracker
	responseTracker        *ResponseTracker
}

// NewUptimeCalculator 创建在线率计算器
func NewUptimeCalculator(config *UptimeConfig) *UptimeCalculator {
	return &UptimeCalculator{
		config:                 config,
		heartbeatTracker:       NewHeartbeatTracker(),
		uptimeObserver:         NewUptimeObserver(config.ConsensusThreshold),
		txParticipationTracker: NewTxParticipationTracker(),
		responseTracker:        NewResponseTracker(),
	}
}

// CalculateUptimeScore 计算综合在线率评分
func (uc *UptimeCalculator) CalculateUptimeScore(address common.Address) *UptimeData {
	// 1. SGX 心跳评分（40%）
	heartbeatScore := uc.heartbeatTracker.CalculateHeartbeatScore(address, uc.config.HeartbeatInterval)

	// 2. 多节点共识评分（30%）
	consensusScore := uc.uptimeObserver.CalculateConsensusScore(address, 10) // TODO: 获取实际观测者数量

	// 3. 交易参与度评分（20%）
	txParticipationScore := uc.txParticipationTracker.CalculateParticipationScore(address, 1000, 30000000) // TODO: 获取实际总量

	// 4. 响应时间评分（10%）
	responseScore := uc.responseTracker.CalculateResponseScore(address, uc.config.ResponseTimeTarget)

	// 计算综合评分
	comprehensiveScore := uint64(
		float64(heartbeatScore)*uc.config.HeartbeatWeight/100.0 +
			float64(consensusScore)*uc.config.ConsensusWeight/100.0 +
			float64(txParticipationScore)*uc.config.TxParticipationWeight/100.0 +
			float64(responseScore)*uc.config.ResponseWeight/100.0,
	)

	return &UptimeData{
		Address:              address,
		HeartbeatScore:       heartbeatScore,
		ConsensusScore:       consensusScore,
		TxParticipationScore: txParticipationScore,
		ResponseScore:        responseScore,
		ComprehensiveScore:   comprehensiveScore,
	}
}

// RecordHeartbeat 记录心跳
func (uc *UptimeCalculator) RecordHeartbeat(msg *HeartbeatMessage) error {
	return uc.heartbeatTracker.RecordHeartbeat(msg)
}

// RecordObservation 记录观测
func (uc *UptimeCalculator) RecordObservation(observed, observer common.Address) {
	uc.uptimeObserver.RecordObservation(observed, observer)
}

// RecordTxParticipation 记录交易参与
func (uc *UptimeCalculator) RecordTxParticipation(address common.Address, txCount, gasUsed uint64) {
	uc.txParticipationTracker.RecordParticipation(address, txCount, gasUsed)
}

// RecordResponseTime 记录响应时间
func (uc *UptimeCalculator) RecordResponseTime(address common.Address, responseMs uint64) {
	uc.responseTracker.RecordResponse(address, responseMs)
}
