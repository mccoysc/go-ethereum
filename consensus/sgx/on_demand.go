package sgx

import (
	"time"
)

// OnDemandController 按需出块控制器
type OnDemandController struct {
	config *Config
}

// NewOnDemandController 创建按需出块控制器
func NewOnDemandController(config *Config) *OnDemandController {
	return &OnDemandController{
		config: config,
	}
}

// ShouldProduceBlock 判断是否应该出块
func (c *OnDemandController) ShouldProduceBlock(
	lastBlockTime time.Time,
	pendingTxCount int,
	pendingGasTotal uint64,
) bool {
	elapsed := time.Since(lastBlockTime)

	// 条件 1: 达到最大间隔，强制出块（心跳）
	if elapsed >= c.config.MaxBlockInterval {
		return true
	}

	// 条件 2: 未达到最小间隔，不出块
	if elapsed < c.config.MinBlockInterval {
		return false
	}

	// 条件 3: 有足够的待处理交易
	if pendingTxCount >= c.config.MinTxCount {
		return true
	}

	// 条件 4: 有足够的待处理 Gas
	if pendingGasTotal >= c.config.MinGasTotal {
		return true
	}

	return false
}

// CanProduceNow 检查当前是否可以立即出块
func (c *OnDemandController) CanProduceNow(lastBlockTime time.Time) bool {
	elapsed := time.Since(lastBlockTime)
	return elapsed >= c.config.MinBlockInterval
}

// TimeUntilNextBlock 计算距离下次可出块的时间
func (c *OnDemandController) TimeUntilNextBlock(lastBlockTime time.Time) time.Duration {
	elapsed := time.Since(lastBlockTime)
	if elapsed >= c.config.MinBlockInterval {
		return 0
	}
	return c.config.MinBlockInterval - elapsed
}

// ShouldForceHeartbeat 检查是否应该强制心跳出块
func (c *OnDemandController) ShouldForceHeartbeat(lastBlockTime time.Time) bool {
	elapsed := time.Since(lastBlockTime)
	return elapsed >= c.config.MaxBlockInterval
}

// IsWithinCandidateWindow 检查是否在候选区块收集窗口内
func (c *OnDemandController) IsWithinCandidateWindow(firstCandidateTime time.Time) bool {
	elapsed := time.Since(firstCandidateTime)
	windowDuration := time.Duration(c.config.CandidateWindowMs) * time.Millisecond
	return elapsed < windowDuration
}

// GetMinBlockInterval 获取最小出块间隔
func (c *OnDemandController) GetMinBlockInterval() time.Duration {
	return c.config.MinBlockInterval
}

// GetMaxBlockInterval 获取最大出块间隔
func (c *OnDemandController) GetMaxBlockInterval() time.Duration {
	return c.config.MaxBlockInterval
}

// GetMinTxCount 获取最小交易数
func (c *OnDemandController) GetMinTxCount() int {
	return c.config.MinTxCount
}

// GetMinGasTotal 获取最小 Gas 总量
func (c *OnDemandController) GetMinGasTotal() uint64 {
	return c.config.MinGasTotal
}
