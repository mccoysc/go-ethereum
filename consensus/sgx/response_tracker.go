package sgx

import (
"sort"
"sync"
"time"

"github.com/ethereum/go-ethereum/common"
)

// ResponseTracker 响应时间追踪器
type ResponseTracker struct {
mu        sync.RWMutex
responses map[common.Address]*ResponseTimeData
samples   map[common.Address][]uint64 // 保存最近的样本用于计算百分位
}

// NewResponseTracker 创建响应时间追踪器
func NewResponseTracker() *ResponseTracker {
return &ResponseTracker{
responses: make(map[common.Address]*ResponseTimeData),
samples:   make(map[common.Address][]uint64),
}
}

// RecordResponse 记录响应时间
func (rt *ResponseTracker) RecordResponse(address common.Address, responseMs uint64) {
rt.mu.Lock()
defer rt.mu.Unlock()

data, exists := rt.responses[address]
if !exists {
data = &ResponseTimeData{
Address: address,
}
rt.responses[address] = data
}

// 更新样本
samples := rt.samples[address]
samples = append(samples, responseMs)

// 只保留最近 1000 个样本
if len(samples) > 1000 {
samples = samples[len(samples)-1000:]
}
rt.samples[address] = samples

// 计算统计数据
data.AvgResponseMs = calculateAverage(samples)
data.P50ResponseMs = calculatePercentile(samples, 0.50)
data.P95ResponseMs = calculatePercentile(samples, 0.95)
data.P99ResponseMs = calculatePercentile(samples, 0.99)
data.SampleCount = uint64(len(samples))
data.LastUpdateTime = time.Now()
}

// GetResponseData 获取响应数据
func (rt *ResponseTracker) GetResponseData(address common.Address) *ResponseTimeData {
rt.mu.RLock()
defer rt.mu.RUnlock()

data, exists := rt.responses[address]
if !exists {
return nil
}

dataCopy := *data
return &dataCopy
}

// CalculateResponseScore 计算响应评分
func (rt *ResponseTracker) CalculateResponseScore(address common.Address, targetMs uint64) uint64 {
rt.mu.RLock()
defer rt.mu.RUnlock()

data, exists := rt.responses[address]
if !exists {
return 0
}

// 使用 P95 响应时间作为评分依据
responseMs := data.P95ResponseMs
if responseMs == 0 {
responseMs = data.AvgResponseMs
}

// 响应时间越短，评分越高
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

// calculateAverage 计算平均值
func calculateAverage(samples []uint64) uint64 {
if len(samples) == 0 {
return 0
}

sum := uint64(0)
for _, v := range samples {
sum += v
}

return sum / uint64(len(samples))
}

// calculatePercentile 计算百分位
func calculatePercentile(samples []uint64, percentile float64) uint64 {
if len(samples) == 0 {
return 0
}

// 复制并排序
sorted := make([]uint64, len(samples))
copy(sorted, samples)
sort.Slice(sorted, func(i, j int) bool {
return sorted[i] < sorted[j]
})

index := int(float64(len(sorted)-1) * percentile)
return sorted[index]
}
