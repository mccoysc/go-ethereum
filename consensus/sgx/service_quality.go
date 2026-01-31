package sgx

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// ServiceQualityScorer 服务质量评分器
type ServiceQualityScorer struct {
	responseTracker *ResponseTracker

	mu          sync.RWMutex
	qualityData map[common.Address]*ServiceQualityData
}

// NewServiceQualityScorer 创建服务质量评分器
func NewServiceQualityScorer(responseTracker *ResponseTracker) *ServiceQualityScorer {
	return &ServiceQualityScorer{
		responseTracker: responseTracker,
		qualityData:     make(map[common.Address]*ServiceQualityData),
	}
}

// CalculateQualityScore 计算服务质量评分
func (sqs *ServiceQualityScorer) CalculateQualityScore(address common.Address) *ServiceQualityData {
	sqs.mu.Lock()
	defer sqs.mu.Unlock()

	data, exists := sqs.qualityData[address]
	if !exists {
		data = &ServiceQualityData{
			Address: address,
		}
		sqs.qualityData[address] = data
	}

	// 获取响应时间数据
	responseData := sqs.responseTracker.GetResponseData(address)
	if responseData != nil {
		// 响应时间评分
		data.ResponseScore = sqs.calculateResponseScore(responseData.P95ResponseMs)

		// 吞吐量评分（基于响应时间估算）
		data.ThroughputScore = sqs.calculateThroughputScore(responseData.P95ResponseMs)
	}

	// 综合服务质量评分（50% 响应 + 50% 吞吐量）
	data.QualityScore = (data.ResponseScore + data.ThroughputScore) / 2

	return data
}

// calculateResponseScore 计算响应时间评分
func (sqs *ServiceQualityScorer) calculateResponseScore(responseMs uint64) uint64 {
	// 目标响应时间 100ms
	targetMs := uint64(100)

	if responseMs <= targetMs {
		return 10000
	}

	// 线性降低评分
	ratio := float64(targetMs) / float64(responseMs)
	score := uint64(ratio * 10000)

	if score > 10000 {
		score = 10000
	}

	return score
}

// calculateThroughputScore 计算吞吐量评分
func (sqs *ServiceQualityScorer) calculateThroughputScore(responseMs uint64) uint64 {
	// 响应时间越短，吞吐量越高
	if responseMs == 0 {
		return 10000
	}

	// 假设目标吞吐量对应 50ms 响应时间
	targetMs := uint64(50)

	if responseMs <= targetMs {
		return 10000
	}

	// 线性降低评分
	ratio := float64(targetMs) / float64(responseMs)
	score := uint64(ratio * 10000)

	if score > 10000 {
		score = 10000
	}

	return score
}

// GetQualityData 获取质量数据
func (sqs *ServiceQualityScorer) GetQualityData(address common.Address) *ServiceQualityData {
	sqs.mu.RLock()
	defer sqs.mu.RUnlock()

	data, exists := sqs.qualityData[address]
	if !exists {
		return nil
	}

	dataCopy := *data
	return &dataCopy
}
