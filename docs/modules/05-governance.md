# 治理模块开发文档

## 模块概述

治理模块实现 X Chain 的去中心化治理机制，包括验证者白名单管理、节点准入控制、硬分叉升级投票、渐进式权限机制和验证者质押管理。该模块确保网络升级和节点管理的安全性和透明性。

## 负责团队

**治理/协议团队**

## 模块职责

1. MRENCLAVE 白名单管理
2. 节点准入控制（基于 SGX 证明）
3. 硬分叉升级投票机制
4. 渐进式权限机制
5. 验证者质押与动态管理
6. 自动密钥迁移机制
7. 投票透明性查询
8. **网络引导机制**（Bootstrap）

## 网络引导机制（Bootstrap）

### 引导问题

X Chain 的安全参数从链上合约读取，但这存在一个"鸡和蛋"的问题：首次运行时还没有链，哪来的合约地址？

### 解决方案：创世区块预部署

治理合约和安全配置合约在创世区块中预部署，合约地址是确定性的（基于部署者地址和 nonce），可以预先计算并写入 Manifest。

**合约职责划分**：
- **安全配置合约（SecurityConfigContract）**：存储所有安全配置（白名单、准入策略、分叉配置、迁移策略等），被其他模块读取
- **治理合约（GovernanceContract）**：负责投票、管理投票人（有效性、合法性）、把投票结果写入安全配置合约

```go
// genesis/bootstrap.go
package genesis

// BootstrapConfig 引导配置
type BootstrapConfig struct {
    // 创始 MRENCLAVE（第一个版本代码的度量值）
    AllowedMREnclave [32]byte
    
    // 创始管理者数量上限
    MaxFounders uint64
    
    // 引导阶段结束后的投票阈值
    VotingThreshold uint64 // 百分比，如 67 表示 2/3
    
    // 预部署合约地址（确定性计算）
    GovernanceContract     common.Address
    SecurityConfigContract common.Address // 安全配置合约，由治理合约管理
}

// DefaultBootstrapConfig 默认引导配置
func DefaultBootstrapConfig() *BootstrapConfig {
    return &BootstrapConfig{
        MaxFounders:     5,  // 最多 5 个创始管理者
        VotingThreshold: 67, // 2/3 投票通过
    }
}
```

### 引导阶段流程

```
┌─────────────────────────────────────────────────────────────────┐
│                        引导阶段（Bootstrap Phase）                │
├─────────────────────────────────────────────────────────────────┤
│  1. 创世配置指定初始 MRENCLAVE + 创始管理者数量上限              │
│  2. 前 N 个不同 Instance ID 的节点自动成为创始管理者             │
│     （所有节点 MRENCLAVE 相同，通过 Instance ID 区分）           │
│  3. 达到上限后，引导阶段自动结束                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        正常阶段（Normal Phase）                   │
├─────────────────────────────────────────────────────────────────┤
│  1. 新管理者必须通过现有管理者投票添加                           │
│  2. 投票需要达到一定比例（如 2/3）                               │
│  3. 标准治理流程                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 创始管理者身份

**重要说明**：除了升级硬分叉期间，所有节点的 MRENCLAVE 都是完全相同的。区分不同节点的是 **instance id**（SGX 硬件唯一标识），而不是 MRENCLAVE。

创始管理者的选择基于：
1. **MRENCLAVE 验证**：确保运行的是正确的代码（所有节点相同）
2. **Instance ID 去重**：每个物理 CPU 只能注册一个创始管理者
3. **先到先得**：前 N 个注册的不同硬件实例成为创始管理者

```
创始管理者选择逻辑：
┌─────────────────────────────────────────────────────────────────┐
│  节点 A (Instance ID: 0x1234...)  ──┐                           │
│  节点 B (Instance ID: 0x5678...)  ──┼── 相同 MRENCLAVE          │
│  节点 C (Instance ID: 0x9abc...)  ──┘                           │
│                                                                  │
│  区分方式：Instance ID（硬件唯一标识）                           │
│  选择方式：前 N 个不同 Instance ID 的节点成为创始管理者          │
└─────────────────────────────────────────────────────────────────┘
```

**信任根**：
- 创始 MRENCLAVE 是第一个版本代码编译后的度量值
- 这是唯一的"信任根"，由项目方/社区确定
- 可以通过可重现构建（reproducible build）让任何人验证
- Instance ID 由 SGX 硬件提供，无法伪造

### 引导合约实现

```go
// governance/bootstrap_contract.go
package governance

import (
    "github.com/ethereum/go-ethereum/common"
)

// BootstrapContract 引导合约
type BootstrapContract struct {
    // 引导阶段是否结束
    BootstrapEnded bool
    
    // 当前创始管理者数量
    FounderCount uint64
    
    // 最大创始管理者数量
    MaxFounders uint64
    
    // 允许的创始 MRENCLAVE
    AllowedMREnclave [32]byte
    
    // 创始管理者列表
    Founders map[common.Address]bool
    
    // 硬件 ID 到管理者的映射（防止同一硬件多次注册）
    HardwareToFounder map[[32]byte]common.Address
}

// RegisterFounder 注册创始管理者
func (bc *BootstrapContract) RegisterFounder(
    caller common.Address,
    mrenclave [32]byte,
    hardwareID [32]byte,
    quote []byte,
) error {
    // 1. 检查引导阶段是否已结束
    if bc.BootstrapEnded {
        return ErrBootstrapEnded
    }
    
    // 2. 验证 MRENCLAVE 是否匹配
    if mrenclave != bc.AllowedMREnclave {
        return ErrInvalidMREnclave
    }
    
    // 3. 验证 SGX Quote
    if !VerifySGXQuote(quote, mrenclave) {
        return ErrInvalidQuote
    }
    
    // 4. 检查硬件 ID 是否已注册
    if _, exists := bc.HardwareToFounder[hardwareID]; exists {
        return ErrHardwareAlreadyRegistered
    }
    
    // 5. 检查是否已达到上限
    if bc.FounderCount >= bc.MaxFounders {
        bc.BootstrapEnded = true
        return ErrMaxFoundersReached
    }
    
    // 6. 注册创始管理者
    bc.Founders[caller] = true
    bc.HardwareToFounder[hardwareID] = caller
    bc.FounderCount++
    
    // 7. 检查是否达到上限，自动结束引导阶段
    if bc.FounderCount >= bc.MaxFounders {
        bc.BootstrapEnded = true
    }
    
    return nil
}

// IsFounder 检查是否为创始管理者
func (bc *BootstrapContract) IsFounder(addr common.Address) bool {
    return bc.Founders[addr]
}

// IsBootstrapPhase 检查是否处于引导阶段
func (bc *BootstrapContract) IsBootstrapPhase() bool {
    return !bc.BootstrapEnded
}
```

### 升级期间只读模式

在硬分叉升级期间（当白名单中存在多个 MRENCLAVE 时），新版本节点只允许读取类操作，所有会导致修改的操作（签名的交易）都被拒绝。这确保了升级过程中数据的一致性和安全性。

**判断条件**：
- 当 `SecurityConfigContract` 中的 MRENCLAVE 白名单包含多个值时，表示正在进行升级
- 新节点（运行新 MRENCLAVE 的节点）进入只读模式
- 旧节点继续正常处理交易，直到升级完成
- **升级完成区块高度**：当区块高度达到指定值时，即使白名单中还有多个 MRENCLAVE，新节点也认为升级完成

### 升级完成区块高度与秘密数据同步

为了提供明确的升级截止时间，避免升级过程无限期拖延，引入"升级完成区块高度"参数。同时，由于秘密数据（私钥等）与区块高度关联，新节点需要同步秘密数据到指定高度后才能认为升级完成。

**存储位置**：
- `UpgradeCompleteBlock` 是安全参数，存储在 **SecurityConfigContract** 中
- 由 **GovernanceContract** 通过投票机制管理和修改
- 在添加新 MRENCLAVE 到白名单时，同时通过投票设置 `UpgradeCompleteBlock` 参数

**升级完成条件**：
- 秘密数据已同步到 `UpgradeCompleteBlock` 高度（`secretDataSyncedBlock >= UpgradeCompleteBlock`）

注意：不需要单独检查当前区块高度，因为非秘密数据是直接复用的，秘密数据同步到指定高度本身就意味着节点已准备好处理该高度的数据。

**秘密数据同步机制**：
- 秘密数据与区块高度关联，每个区块可能有对应的秘密数据
- 新节点通过 RA-TLS 安全通道自动从旧节点同步秘密数据
- 同步过程记录当前已同步到的区块高度（`secretDataSyncedBlock`）
- 当 `secretDataSyncedBlock >= UpgradeCompleteBlock` 时，停止同步

```go
// security/upgrade_config.go
package security

// UpgradeConfig 升级配置（存储在 SecurityConfigContract 中，由 GovernanceContract 管理）
type UpgradeConfig struct {
    // 新版本 MRENCLAVE
    NewMREnclave [32]byte
    
    // 升级完成区块高度（安全参数，由投票设置）
    // 当区块高度达到此值时，即使白名单中还有多个 MRENCLAVE，
    // 新节点也认为升级完成，只接受与自己一致度量值的节点
    UpgradeCompleteBlock uint64
    
    // 升级开始区块高度（添加新 MRENCLAVE 时的区块高度）
    UpgradeStartBlock uint64
}

// SecretDataSyncState 秘密数据同步状态（本地存储）
type SecretDataSyncState struct {
    // 已同步到的区块高度
    SyncedBlock uint64
    
    // 同步是否完成
    SyncComplete bool
    
    // 最后同步时间
    LastSyncTime int64
}

// SecurityConfigContract 安全配置合约接口
type SecurityConfigContract interface {
    // GetUpgradeConfig 获取升级配置
    GetUpgradeConfig() *UpgradeConfig
    
    // SetUpgradeConfig 设置升级配置（只能由 GovernanceContract 调用）
    SetUpgradeConfig(config *UpgradeConfig) error
}
```

```go
// governance/upgrade_mode.go
package governance

import (
    "errors"
    
    "github.com/ethereum/go-ethereum/core/types"
)

var (
    ErrUpgradeReadOnlyMode = errors.New("node is in upgrade read-only mode, write operations are rejected")
)

// UpgradeModeChecker 升级模式检查器
type UpgradeModeChecker struct {
    securityConfig SecurityConfigReader
    localMREnclave [32]byte
}

// NewUpgradeModeChecker 创建升级模式检查器
func NewUpgradeModeChecker(config SecurityConfigReader, localMR [32]byte) *UpgradeModeChecker {
    return &UpgradeModeChecker{
        securityConfig: config,
        localMREnclave: localMR,
    }
}

// SecurityConfigReader 安全配置读取接口
type SecurityConfigReader interface {
    GetMREnclaveWhitelist() []MREnclaveEntry
    GetUpgradeConfig() *UpgradeConfig
    GetSecretDataSyncState() *SecretDataSyncState
}

// IsUpgradeInProgress 检查是否正在进行升级
// 当白名单中存在多个 MRENCLAVE 时，表示正在进行升级
func (c *UpgradeModeChecker) IsUpgradeInProgress() bool {
    whitelist := c.securityConfig.GetMREnclaveWhitelist()
    return len(whitelist) > 1
}

// IsUpgradeComplete 检查升级是否已完成
// 升级完成条件：
// 1. 白名单中只有一个 MRENCLAVE，或
// 2. 秘密数据已同步到升级完成区块高度
func (c *UpgradeModeChecker) IsUpgradeComplete() bool {
    whitelist := c.securityConfig.GetMREnclaveWhitelist()
    
    // 条件 1: 白名单中只有一个 MRENCLAVE
    if len(whitelist) <= 1 {
        return true
    }
    
    // 条件 2: 秘密数据已同步到升级完成区块高度
    upgradeConfig := c.securityConfig.GetUpgradeConfig()
    syncState := c.securityConfig.GetSecretDataSyncState()
    if upgradeConfig != nil && upgradeConfig.UpgradeCompleteBlock > 0 && syncState != nil {
        if syncState.SyncedBlock >= upgradeConfig.UpgradeCompleteBlock {
            return true
        }
    }
    
    return false
}

// IsNewVersionNode 检查本节点是否是新版本节点
// 新版本节点的 MRENCLAVE 与白名单中最新添加的 MRENCLAVE 匹配
func (c *UpgradeModeChecker) IsNewVersionNode() bool {
    whitelist := c.securityConfig.GetMREnclaveWhitelist()
    if len(whitelist) <= 1 {
        return false
    }
    
    // 最新添加的 MRENCLAVE 是新版本
    latestMR := whitelist[len(whitelist)-1]
    return c.localMREnclave == latestMR.MRENCLAVE
}

// ShouldRejectWriteOperation 检查是否应该拒绝写操作
// 升级期间（未完成），新版本节点拒绝所有写操作
func (c *UpgradeModeChecker) ShouldRejectWriteOperation() bool {
    // 如果升级已完成，不拒绝写操作
    if c.IsUpgradeComplete() {
        return false
    }
    
    return c.IsUpgradeInProgress() && c.IsNewVersionNode()
}

// ShouldRejectOldVersionPeer 检查是否应该拒绝旧版本节点的连接
// 升级完成后，新节点只接受与自己一致度量值的节点
func (c *UpgradeModeChecker) ShouldRejectOldVersionPeer(peerMREnclave [32]byte) bool {
    // 如果升级已完成，只接受与自己一致度量值的节点
    if c.IsUpgradeComplete() && c.IsNewVersionNode() {
        return peerMREnclave != c.localMREnclave
    }
    return false
}

// ValidateTransaction 验证交易是否可以被处理
// 在升级期间（未完成），新版本节点拒绝所有签名的交易
func (c *UpgradeModeChecker) ValidateTransaction(tx *types.Transaction) error {
    if c.ShouldRejectWriteOperation() {
        // 升级期间，新版本节点只允许读取操作
        // 所有签名的交易（会导致状态修改）都被拒绝
        return ErrUpgradeReadOnlyMode
    }
    return nil
}
```

**升级期间只读模式流程**：

```
┌─────────────────────────────────────────────────────────────────┐
│                     升级期间只读模式                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  阶段 1: 升级进行中（区块高度 < UpgradeCompleteBlock）           │
│  白名单状态：[MRENCLAVE_OLD, MRENCLAVE_NEW]                      │
│                                                                  │
│  ┌─────────────────────┐      ┌─────────────────────┐           │
│  │  旧版本节点          │      │  新版本节点          │           │
│  │  MRENCLAVE_OLD      │      │  MRENCLAVE_NEW      │           │
│  ├─────────────────────┤      ├─────────────────────┤           │
│  │  正常模式            │      │  只读模式            │           │
│  │  - 处理交易 ✓        │      │  - 处理交易 ✗        │           │
│  │  - 出块 ✓            │      │  - 出块 ✗            │           │
│  │  - 读取状态 ✓        │      │  - 读取状态 ✓        │           │
│  │  - 同步区块 ✓        │      │  - 同步区块 ✓        │           │
│  │  - 接受新旧节点 ✓    │      │  - 接受新旧节点 ✓    │           │
│  └─────────────────────┘      └─────────────────────┘           │
│                                                                  │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  阶段 2: 升级完成（区块高度 >= UpgradeCompleteBlock）            │
│  白名单状态：可能仍为 [MRENCLAVE_OLD, MRENCLAVE_NEW]             │
│                                                                  │
│  ┌─────────────────────┐      ┌─────────────────────┐           │
│  │  旧版本节点          │      │  新版本节点          │           │
│  │  MRENCLAVE_OLD      │      │  MRENCLAVE_NEW      │           │
│  ├─────────────────────┤      ├─────────────────────┤           │
│  │  被隔离              │      │  正常模式            │           │
│  │  - 无法连接新节点    │      │  - 处理交易 ✓        │           │
│  │                      │      │  - 出块 ✓            │           │
│  │                      │      │  - 读取状态 ✓        │           │
│  │                      │      │  - 同步区块 ✓        │           │
│  │                      │      │  - 只接受新节点 ✓    │           │
│  └─────────────────────┘      └─────────────────────┘           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**升级完成条件**（满足任一即可）：
1. 白名单中只剩下一个 MRENCLAVE（通过投票移除旧版本）
2. 当前区块高度 >= `UpgradeCompleteBlock`（达到预设的升级截止高度）

**交易池集成**：

```go
// core/txpool/upgrade_filter.go
package txpool

import (
    "github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/governance"
)

// UpgradeFilter 升级期间的交易过滤器
type UpgradeFilter struct {
    checker *governance.UpgradeModeChecker
}

// NewUpgradeFilter 创建升级过滤器
func NewUpgradeFilter(checker *governance.UpgradeModeChecker) *UpgradeFilter {
    return &UpgradeFilter{checker: checker}
}

// Filter 过滤交易
// 升级期间，新版本节点拒绝所有交易
func (f *UpgradeFilter) Filter(tx *types.Transaction) error {
    return f.checker.ValidateTransaction(tx)
}
```

**共识引擎集成**：

```go
// consensus/poa_sgx/upgrade_check.go
package poa_sgx

import (
    "github.com/ethereum/go-ethereum/governance"
)

// CanProduceBlock 检查是否可以出块
// 升级期间，新版本节点不能出块
func (e *Engine) CanProduceBlock() bool {
    if e.upgradeChecker.ShouldRejectWriteOperation() {
        return false
    }
    return true
}
```

**安全保证**：
- 升级期间，新版本节点只能同步和验证区块，不能产生新区块或处理交易
- 这确保了升级过程中不会出现分叉或数据不一致
- 只有当旧版本 MRENCLAVE 从白名单中移除后，新版本节点才能正常工作
- 升级完成的标志是白名单中只剩下一个 MRENCLAVE

### 创世配置示例

```json
{
  "config": {
    "chainId": 1337,
    "xchain": {
      "bootstrap": {
        "allowedMREnclave": "abc123def456789...",
        "maxFounders": 5,
        "votingThreshold": 67
      }
    }
  },
  "alloc": {
    "0x1234567890abcdef1234567890abcdef12345678": {
      "code": "0x...",
      "storage": {
        "0x0": "0x05",
        "0x1": "0xabc123def456789..."
      }
    }
  }
}
```

### 合约地址确定性计算

```go
// genesis/address.go
package genesis

import (
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/crypto"
)

// CalculateContractAddress 计算确定性合约地址
// 基于部署者地址和 nonce
func CalculateContractAddress(deployer common.Address, nonce uint64) common.Address {
    return crypto.CreateAddress(deployer, nonce)
}

// 预计算的合约地址（用于 Manifest）
const (
    // 假设部署者地址为 0x0000...0000，nonce 从 0 开始
    GovernanceContractAddress     = "0x1234567890abcdef1234567890abcdef12345678"
    SecurityConfigContractAddress = "0xabcdef1234567890abcdef1234567890abcdef12" // 安全配置合约
)
```

## 依赖关系

```
+------------------+
|    治理模块      |
+------------------+
        |
        +---> SGX 证明模块（MRENCLAVE 验证）
        |
        +---> 共识引擎模块（区块确认、升级模式协调）
        |
        +---> 数据存储模块（状态持久化、秘密数据迁移触发）
```

### 上游依赖
- SGX 证明模块（验证节点 MRENCLAVE）
- 共识引擎模块（区块最终性）

### 下游依赖（被以下模块使用）
- P2P 网络层（节点准入）
- 共识引擎模块（验证者集合、升级模式检查）
- 数据存储模块（通过 SecurityConfigContract 提供参数配置）
- 激励机制模块（奖励参数配置）

### 与数据存储模块的交互

治理模块通过以下方式影响数据存储模块：

1. **参数管理**：
   - SecurityConfigContract 存储所有安全配置参数
   - 数据存储模块从 SecurityConfigContract 读取 MRENCLAVE 白名单
   - 密钥迁移阈值、准入策略等参数由治理投票决定

2. **秘密数据迁移触发**：
   - 当治理投票添加新 MRENCLAVE 到白名单时
   - 数据存储模块的 AutoMigrationManager 自动触发秘密数据同步
   - 基于 PermissionLevel 限制每日迁移次数

3. **升级协调**：
   - 治理设置 `UpgradeCompleteBlock` 参数
   - 数据存储模块确保在该区块高度前完成秘密数据迁移
   - 共识引擎在升级完成前将新版本节点置于只读模式

### 与共识引擎的协调

治理模块通过升级模式检查器（UpgradeModeChecker）影响共识引擎：

1. **升级期间只读模式**：
   - 白名单中存在多个 MRENCLAVE 时，表示升级正在进行
   - 新版本节点进入只读模式，不能出块或处理交易
   - 只能同步区块和验证区块

2. **升级完成判定**：
   - 区块高度达到 `UpgradeCompleteBlock` 或
   - 白名单中只剩下一个 MRENCLAVE
   - 新版本节点恢复正常模式，旧版本节点被隔离

## 核心数据结构

### 白名单配置

```go
// governance/whitelist.go
package governance

import (
    "github.com/ethereum/go-ethereum/common"
)

// WhitelistConfig 白名单配置
// 所有配置参数存储在 SecurityConfigContract 中，可以通过 GovernanceContract 投票修改
type WhitelistConfig struct {
    // 核心验证者投票阈值（百分比，默认 67 表示 2/3）
    CoreValidatorThreshold uint64
    
    // 社区验证者投票阈值（百分比，默认 51 表示简单多数）
    CommunityValidatorThreshold uint64
    
    // 投票期限（区块数，默认 40320 ≈ 7天）
    VotingPeriod uint64
    
    // 执行延迟（区块数，默认 5760 ≈ 1天）
    ExecutionDelay uint64
    
    // 最小投票参与率（百分比，默认 50%）
    MinParticipation uint64
}

// DefaultWhitelistConfig 默认配置
// 注意：这些是创世区块的初始值，实际值从 SecurityConfigContract 中读取
func DefaultWhitelistConfig() *WhitelistConfig {
    return &WhitelistConfig{
        CoreValidatorThreshold:      67,    // 2/3 核心验证者
        CommunityValidatorThreshold: 51,    // 简单多数社区验证者
        VotingPeriod:                40320, // 约 7 天（按 15 秒/块计算）
        ExecutionDelay:              5760,  // 约 1 天
        MinParticipation:            50,    // 50% 参与率
    }
}

// CoreValidatorConfig 核心验证者配置
// 所有配置参数存储在 SecurityConfigContract 中，可以通过 GovernanceContract 投票修改
type CoreValidatorConfig struct {
    MinMembers      int     // 最小成员数（默认 5）
    MaxMembers      int     // 最大成员数（默认 7）
    QuorumThreshold float64 // 投票通过阈值（默认 0.667 表示 2/3）
}

// DefaultCoreValidatorConfig 默认核心验证者配置
// 注意：这些是创世区块的初始值，实际值从 SecurityConfigContract 中读取
func DefaultCoreValidatorConfig() *CoreValidatorConfig {
    return &CoreValidatorConfig{
        MinMembers:      5,
        MaxMembers:      7,
        QuorumThreshold: 0.667, // 2/3
    }
}

// CommunityValidatorConfig 社区验证者配置
// 所有配置参数存储在 SecurityConfigContract 中，可以通过 GovernanceContract 投票修改
type CommunityValidatorConfig struct {
    MinUptime     time.Duration // 最小运行时间（默认 30 天）
    MinStake      *big.Int      // 最小质押量（初始值 10,000 X，可通过治理投票修改）
    VetoThreshold float64       // 否决阈值（默认 0.334 表示 1/3）
}

// DefaultCommunityValidatorConfig 默认社区验证者配置
// 注意：这些是创世区块的初始值，实际值从 SecurityConfigContract 中读取
func DefaultCommunityValidatorConfig() *CommunityValidatorConfig {
    return &CommunityValidatorConfig{
        MinUptime:     30 * 24 * time.Hour,                              // 30 天
        MinStake:      new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e18)), // 初始值：10,000 X（可通过治理投票修改）
        VetoThreshold: 0.334,                                            // 1/3
    }
}

// MREnclaveEntry 白名单条目
type MREnclaveEntry struct {
    MRENCLAVE     [32]byte       // MRENCLAVE 值
    Version       string         // 版本号
    AddedAt       uint64         // 添加时间（区块号）
    AddedBy       common.Address // 添加者
    PermissionLevel PermissionLevel // 权限级别
    Status        EntryStatus    // 状态
}

// EntryStatus 条目状态
type EntryStatus uint8

const (
    StatusPending  EntryStatus = 0x00 // 待投票
    StatusApproved EntryStatus = 0x01 // 已批准
    StatusActive   EntryStatus = 0x02 // 已激活
    StatusDeprecated EntryStatus = 0x03 // 已弃用
    StatusRejected EntryStatus = 0x04 // 已拒绝
)

// PermissionLevel 权限级别
type PermissionLevel uint8

const (
    PermissionBasic    PermissionLevel = 0x01 // 基础权限
    PermissionStandard PermissionLevel = 0x02 // 标准权限
    PermissionFull     PermissionLevel = 0x03 // 完整权限
)
```

### 投票系统

```go
// governance/voting.go
package governance

import (
    "github.com/ethereum/go-ethereum/common"
)

// ProposalType 提案类型
type ProposalType uint8

const (
    ProposalAddMREnclave    ProposalType = 0x01 // 添加 MRENCLAVE
    ProposalRemoveMREnclave ProposalType = 0x02 // 移除 MRENCLAVE
    ProposalUpgradePermission ProposalType = 0x03 // 升级权限
    ProposalAddValidator    ProposalType = 0x04 // 添加验证者
    ProposalRemoveValidator ProposalType = 0x05 // 移除验证者
    ProposalParameterChange ProposalType = 0x06 // 参数修改
    ProposalNormalUpgrade   ProposalType = 0x07 // 普通升级
    ProposalEmergencyUpgrade ProposalType = 0x08 // 紧急升级（安全漏洞修复）
)

// 升级提案的投票规则：
// 1. 普通升级（ProposalNormalUpgrade）：
//    - 核心验证者：需要 2/3 通过
//    - 社区验证者：可以行使否决权，1/3 否决即可拒绝提案
// 2. 紧急升级（ProposalEmergencyUpgrade）：
//    - 核心验证者：需要 100% 通过
//    - 社区验证者：否决权阈值提高到 1/2（更高的否决门槛）
//    - 必须附带安全漏洞详情和修复说明

// Proposal 提案
type Proposal struct {
    ID            common.Hash    // 提案 ID
    Type          ProposalType   // 提案类型
    Proposer      common.Address // 提案者
    Target        []byte         // 目标数据（如 MRENCLAVE）
    Description   string         // 描述
    CreatedAt     uint64         // 创建区块
    VotingEndsAt  uint64         // 投票截止区块
    ExecuteAfter  uint64         // 可执行区块
    Status        ProposalStatus // 状态
    
    // 投票统计
    CoreYesVotes      uint64
    CoreNoVotes       uint64
    CommunityYesVotes uint64
    CommunityNoVotes  uint64
}

// ProposalStatus 提案状态
type ProposalStatus uint8

const (
    ProposalStatusPending   ProposalStatus = 0x00 // 投票中
    ProposalStatusPassed    ProposalStatus = 0x01 // 已通过
    ProposalStatusRejected  ProposalStatus = 0x02 // 已拒绝
    ProposalStatusExecuted  ProposalStatus = 0x03 // 已执行
    ProposalStatusCancelled ProposalStatus = 0x04 // 已取消
    ProposalStatusExpired   ProposalStatus = 0x05 // 已过期
)

// Vote 投票
type Vote struct {
    ProposalID common.Hash
    Voter      common.Address
    Support    bool   // true = 支持, false = 反对
    Weight     uint64 // 投票权重
    Timestamp  uint64
    Signature  []byte
}

// VoterType 投票者类型
type VoterType uint8

const (
    VoterTypeCore      VoterType = 0x01 // 核心验证者
    VoterTypeCommunity VoterType = 0x02 // 社区验证者
)
```

### 验证者管理

```go
// governance/validator.go
package governance

import (
    "math/big"
    
    "github.com/ethereum/go-ethereum/common"
)

// ValidatorInfo 验证者信息
type ValidatorInfo struct {
    Address       common.Address // 验证者地址
    Type          VoterType      // 验证者类型
    MRENCLAVE     [32]byte       // 当前 MRENCLAVE
    StakeAmount   *big.Int       // 质押金额
    JoinedAt      uint64         // 加入区块
    LastActiveAt  uint64         // 最后活跃区块
    VotingPower   uint64         // 投票权重
    Status        ValidatorStatus // 状态
}

// ValidatorStatus 验证者状态
type ValidatorStatus uint8

const (
    ValidatorStatusActive   ValidatorStatus = 0x01 // 活跃
    ValidatorStatusInactive ValidatorStatus = 0x02 // 不活跃
    ValidatorStatusJailed   ValidatorStatus = 0x03 // 监禁
    ValidatorStatusExiting  ValidatorStatus = 0x04 // 退出中
)

// StakingConfig 质押配置
// 所有配置参数存储在 SecurityConfigContract 中，可以通过 GovernanceContract 投票修改
type StakingConfig struct {
    // 最小质押金额（存储在合约中，可通过治理投票修改）
    MinStakeAmount *big.Int
    
    // 解除质押锁定期（区块数）
    UnstakeLockPeriod uint64
    
    // 质押奖励率（年化百分比）
    AnnualRewardRate uint64
    
    // 惩罚率（百分比）
    SlashingRate uint64
}

// DefaultStakingConfig 默认配置
// 注意：这些是初始值，实际值从 SecurityConfigContract 中读取，可以通过治理投票修改
func DefaultStakingConfig() *StakingConfig {
    return &StakingConfig{
        MinStakeAmount:    new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e18)), // 初始值：10000 X（可通过治理合约修改）
        UnstakeLockPeriod: 40320,                // 约 7 天
        AnnualRewardRate:  5,                    // 5%
        SlashingRate:      10,                   // 10%
    }
}
```

## 核心接口定义

### 白名单管理器

```go
// governance/whitelist_manager.go
package governance

import (
    "github.com/ethereum/go-ethereum/common"
)

// WhitelistManager 白名单管理器接口
type WhitelistManager interface {
    // IsAllowed 检查 MRENCLAVE 是否在白名单中
    IsAllowed(mrenclave [32]byte) bool
    
    // GetPermissionLevel 获取 MRENCLAVE 的权限级别
    GetPermissionLevel(mrenclave [32]byte) PermissionLevel
    
    // GetEntry 获取白名单条目
    GetEntry(mrenclave [32]byte) (*MREnclaveEntry, error)
    
    // GetAllEntries 获取所有白名单条目
    GetAllEntries() []*MREnclaveEntry
    
    // ProposeAdd 提议添加新 MRENCLAVE
    ProposeAdd(proposer common.Address, mrenclave [32]byte, version string) (common.Hash, error)
    
    // ProposeRemove 提议移除 MRENCLAVE
    ProposeRemove(proposer common.Address, mrenclave [32]byte, reason string) (common.Hash, error)
    
    // ProposeUpgrade 提议升级权限级别
    ProposeUpgrade(proposer common.Address, mrenclave [32]byte, newLevel PermissionLevel) (common.Hash, error)
}
```

### 投票管理器

```go
// governance/voting_manager.go
package governance

import (
    "github.com/ethereum/go-ethereum/common"
)

// VotingManager 投票管理器接口
type VotingManager interface {
    // CreateProposal 创建提案
    CreateProposal(proposal *Proposal) (common.Hash, error)
    
    // Vote 投票
    Vote(proposalID common.Hash, voter common.Address, support bool, signature []byte) error
    
    // GetProposal 获取提案
    GetProposal(proposalID common.Hash) (*Proposal, error)
    
    // GetProposalVotes 获取提案的所有投票
    GetProposalVotes(proposalID common.Hash) ([]*Vote, error)
    
    // ExecuteProposal 执行已通过的提案
    ExecuteProposal(proposalID common.Hash) error
    
    // GetActiveProposals 获取活跃提案
    GetActiveProposals() []*Proposal
    
    // CheckProposalStatus 检查并更新提案状态
    CheckProposalStatus(proposalID common.Hash, currentBlock uint64) error
}
```

### 验证者管理器

```go
// governance/validator_manager.go
package governance

import (
    "math/big"
    
    "github.com/ethereum/go-ethereum/common"
)

// ValidatorManager 验证者管理器接口
type ValidatorManager interface {
    // GetValidator 获取验证者信息
    GetValidator(addr common.Address) (*ValidatorInfo, error)
    
    // GetAllValidators 获取所有验证者
    GetAllValidators() []*ValidatorInfo
    
    // GetCoreValidators 获取核心验证者
    GetCoreValidators() []*ValidatorInfo
    
    // GetCommunityValidators 获取社区验证者
    GetCommunityValidators() []*ValidatorInfo
    
    // IsValidator 检查是否是验证者
    IsValidator(addr common.Address) bool
    
    // GetVoterType 获取投票者类型
    GetVoterType(addr common.Address) VoterType
    
    // Stake 质押
    Stake(addr common.Address, amount *big.Int) error
    
    // Unstake 解除质押
    Unstake(addr common.Address, amount *big.Int) error
    
    // ClaimRewards 领取奖励
    ClaimRewards(addr common.Address) (*big.Int, error)
    
    // Slash 惩罚
    Slash(addr common.Address, reason string) error
    
    // UpdateMREnclave 更新验证者的 MRENCLAVE
    UpdateMREnclave(addr common.Address, newMREnclave [32]byte) error
}
```

### 节点准入控制器

```go
// governance/admission.go
package governance

import (
    "github.com/ethereum/go-ethereum/common"
)

// AdmissionController 节点准入控制器接口
type AdmissionController interface {
    // CheckAdmission 检查节点是否允许连接
    CheckAdmission(nodeID common.Hash, mrenclave [32]byte, quote []byte) (bool, error)
    
    // GetAdmissionStatus 获取节点准入状态
    GetAdmissionStatus(nodeID common.Hash) (*AdmissionStatus, error)
    
    // RecordConnection 记录连接
    RecordConnection(nodeID common.Hash, mrenclave [32]byte) error
    
    // RecordDisconnection 记录断开
    RecordDisconnection(nodeID common.Hash) error
}

// AdmissionStatus 准入状态
type AdmissionStatus struct {
    NodeID        common.Hash
    MRENCLAVE     [32]byte
    Allowed       bool
    Reason        string
    ConnectedAt   uint64
    LastVerified  uint64
}
```

## 实现详情

### 白名单管理器实现

```go
// governance/whitelist_manager_impl.go
package governance

import (
    "errors"
    "sync"
    
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/crypto"
)

// InMemoryWhitelistManager 内存白名单管理器
type InMemoryWhitelistManager struct {
    config  *WhitelistConfig
    mu      sync.RWMutex
    entries map[[32]byte]*MREnclaveEntry
    voting  VotingManager
}

// NewInMemoryWhitelistManager 创建白名单管理器
func NewInMemoryWhitelistManager(config *WhitelistConfig, voting VotingManager) *InMemoryWhitelistManager {
    return &InMemoryWhitelistManager{
        config:  config,
        entries: make(map[[32]byte]*MREnclaveEntry),
        voting:  voting,
    }
}

// IsAllowed 检查 MRENCLAVE 是否在白名单中
func (wm *InMemoryWhitelistManager) IsAllowed(mrenclave [32]byte) bool {
    wm.mu.RLock()
    defer wm.mu.RUnlock()
    
    entry, ok := wm.entries[mrenclave]
    if !ok {
        return false
    }
    
    return entry.Status == StatusActive || entry.Status == StatusApproved
}

// GetPermissionLevel 获取权限级别
func (wm *InMemoryWhitelistManager) GetPermissionLevel(mrenclave [32]byte) PermissionLevel {
    wm.mu.RLock()
    defer wm.mu.RUnlock()
    
    entry, ok := wm.entries[mrenclave]
    if !ok {
        return 0
    }
    
    return entry.PermissionLevel
}

// GetEntry 获取白名单条目
func (wm *InMemoryWhitelistManager) GetEntry(mrenclave [32]byte) (*MREnclaveEntry, error) {
    wm.mu.RLock()
    defer wm.mu.RUnlock()
    
    entry, ok := wm.entries[mrenclave]
    if !ok {
        return nil, errors.New("entry not found")
    }
    
    return entry, nil
}

// GetAllEntries 获取所有条目
func (wm *InMemoryWhitelistManager) GetAllEntries() []*MREnclaveEntry {
    wm.mu.RLock()
    defer wm.mu.RUnlock()
    
    entries := make([]*MREnclaveEntry, 0, len(wm.entries))
    for _, entry := range wm.entries {
        entries = append(entries, entry)
    }
    
    return entries
}

// ProposeAdd 提议添加新 MRENCLAVE
func (wm *InMemoryWhitelistManager) ProposeAdd(proposer common.Address, mrenclave [32]byte, version string) (common.Hash, error) {
    wm.mu.Lock()
    defer wm.mu.Unlock()
    
    // 检查是否已存在
    if _, ok := wm.entries[mrenclave]; ok {
        return common.Hash{}, errors.New("MRENCLAVE already exists")
    }
    
    // 创建提案
    proposal := &Proposal{
        ID:          crypto.Keccak256Hash(mrenclave[:], []byte(version)),
        Type:        ProposalAddMREnclave,
        Proposer:    proposer,
        Target:      mrenclave[:],
        Description: "Add MRENCLAVE " + version,
    }
    
    return wm.voting.CreateProposal(proposal)
}

// ProposeRemove 提议移除 MRENCLAVE
func (wm *InMemoryWhitelistManager) ProposeRemove(proposer common.Address, mrenclave [32]byte, reason string) (common.Hash, error) {
    wm.mu.Lock()
    defer wm.mu.Unlock()
    
    // 检查是否存在
    if _, ok := wm.entries[mrenclave]; !ok {
        return common.Hash{}, errors.New("MRENCLAVE not found")
    }
    
    // 创建提案
    proposal := &Proposal{
        ID:          crypto.Keccak256Hash(mrenclave[:], []byte("remove")),
        Type:        ProposalRemoveMREnclave,
        Proposer:    proposer,
        Target:      mrenclave[:],
        Description: reason,
    }
    
    return wm.voting.CreateProposal(proposal)
}

// AddEntry 添加条目（内部方法，由投票执行调用）
func (wm *InMemoryWhitelistManager) AddEntry(entry *MREnclaveEntry) {
    wm.mu.Lock()
    defer wm.mu.Unlock()
    
    wm.entries[entry.MRENCLAVE] = entry
}

// RemoveEntry 移除条目（内部方法）
func (wm *InMemoryWhitelistManager) RemoveEntry(mrenclave [32]byte) {
    wm.mu.Lock()
    defer wm.mu.Unlock()
    
    if entry, ok := wm.entries[mrenclave]; ok {
        entry.Status = StatusDeprecated
    }
}
```

### 投票管理器实现

```go
// governance/voting_manager_impl.go
package governance

import (
    "errors"
    "sync"
    
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/crypto"
)

// InMemoryVotingManager 内存投票管理器
type InMemoryVotingManager struct {
    config     *WhitelistConfig
    mu         sync.RWMutex
    proposals  map[common.Hash]*Proposal
    votes      map[common.Hash][]*Vote
    validators ValidatorManager
}

// NewInMemoryVotingManager 创建投票管理器
func NewInMemoryVotingManager(config *WhitelistConfig, validators ValidatorManager) *InMemoryVotingManager {
    return &InMemoryVotingManager{
        config:     config,
        proposals:  make(map[common.Hash]*Proposal),
        votes:      make(map[common.Hash][]*Vote),
        validators: validators,
    }
}

// CreateProposal 创建提案
func (vm *InMemoryVotingManager) CreateProposal(proposal *Proposal) (common.Hash, error) {
    vm.mu.Lock()
    defer vm.mu.Unlock()
    
    // 验证提案者是验证者
    if !vm.validators.IsValidator(proposal.Proposer) {
        return common.Hash{}, errors.New("proposer is not a validator")
    }
    
    // 设置投票期限
    // 注意：实际实现中需要获取当前区块号
    proposal.VotingEndsAt = proposal.CreatedAt + vm.config.VotingPeriod
    proposal.ExecuteAfter = proposal.VotingEndsAt + vm.config.ExecutionDelay
    proposal.Status = ProposalStatusPending
    
    vm.proposals[proposal.ID] = proposal
    vm.votes[proposal.ID] = make([]*Vote, 0)
    
    return proposal.ID, nil
}

// Vote 投票
func (vm *InMemoryVotingManager) Vote(proposalID common.Hash, voter common.Address, support bool, signature []byte) error {
    vm.mu.Lock()
    defer vm.mu.Unlock()
    
    // 获取提案
    proposal, ok := vm.proposals[proposalID]
    if !ok {
        return errors.New("proposal not found")
    }
    
    // 检查提案状态
    if proposal.Status != ProposalStatusPending {
        return errors.New("proposal is not pending")
    }
    
    // 验证投票者是验证者
    if !vm.validators.IsValidator(voter) {
        return errors.New("voter is not a validator")
    }
    
    // 检查是否已投票
    for _, v := range vm.votes[proposalID] {
        if v.Voter == voter {
            return errors.New("already voted")
        }
    }
    
    // 验证签名
    voteHash := crypto.Keccak256Hash(proposalID.Bytes(), voter.Bytes(), boolToBytes(support))
    pubKey, err := crypto.SigToPub(voteHash.Bytes(), signature)
    if err != nil {
        return errors.New("invalid signature")
    }
    if crypto.PubkeyToAddress(*pubKey) != voter {
        return errors.New("signature does not match voter")
    }
    
    // 获取投票权重
    validatorInfo, _ := vm.validators.GetValidator(voter)
    weight := validatorInfo.VotingPower
    
    // 记录投票
    vote := &Vote{
        ProposalID: proposalID,
        Voter:      voter,
        Support:    support,
        Weight:     weight,
        Signature:  signature,
    }
    vm.votes[proposalID] = append(vm.votes[proposalID], vote)
    
    // 更新投票统计
    voterType := vm.validators.GetVoterType(voter)
    if support {
        if voterType == VoterTypeCore {
            proposal.CoreYesVotes += weight
        } else {
            proposal.CommunityYesVotes += weight
        }
    } else {
        if voterType == VoterTypeCore {
            proposal.CoreNoVotes += weight
        } else {
            proposal.CommunityNoVotes += weight
        }
    }
    
    return nil
}

// CheckProposalStatus 检查并更新提案状态
func (vm *InMemoryVotingManager) CheckProposalStatus(proposalID common.Hash, currentBlock uint64) error {
    vm.mu.Lock()
    defer vm.mu.Unlock()
    
    proposal, ok := vm.proposals[proposalID]
    if !ok {
        return errors.New("proposal not found")
    }
    
    if proposal.Status != ProposalStatusPending {
        return nil
    }
    
    // 检查投票是否结束
    if currentBlock < proposal.VotingEndsAt {
        return nil
    }
    
    // 计算投票结果
    coreValidators := vm.validators.GetCoreValidators()
    communityValidators := vm.validators.GetCommunityValidators()
    
    totalCoreWeight := uint64(0)
    for _, v := range coreValidators {
        totalCoreWeight += v.VotingPower
    }
    
    totalCommunityWeight := uint64(0)
    for _, v := range communityValidators {
        totalCommunityWeight += v.VotingPower
    }
    
    // 检查核心验证者投票
    coreYesRatio := uint64(0)
    if totalCoreWeight > 0 {
        coreYesRatio = proposal.CoreYesVotes * 100 / totalCoreWeight
    }
    
    // 检查社区验证者投票
    communityYesRatio := uint64(0)
    if totalCommunityWeight > 0 {
        communityYesRatio = proposal.CommunityYesVotes * 100 / totalCommunityWeight
    }
    
    // 判断是否通过
    corePassed := coreYesRatio >= vm.config.CoreValidatorThreshold
    communityPassed := communityYesRatio >= vm.config.CommunityValidatorThreshold
    
    if corePassed && communityPassed {
        proposal.Status = ProposalStatusPassed
    } else {
        proposal.Status = ProposalStatusRejected
    }
    
    return nil
}

// ExecuteProposal 执行提案
func (vm *InMemoryVotingManager) ExecuteProposal(proposalID common.Hash) error {
    vm.mu.Lock()
    defer vm.mu.Unlock()
    
    proposal, ok := vm.proposals[proposalID]
    if !ok {
        return errors.New("proposal not found")
    }
    
    if proposal.Status != ProposalStatusPassed {
        return errors.New("proposal not passed")
    }
    
    // 执行提案（具体逻辑由调用者实现）
    proposal.Status = ProposalStatusExecuted
    
    return nil
}

// GetProposal 获取提案
func (vm *InMemoryVotingManager) GetProposal(proposalID common.Hash) (*Proposal, error) {
    vm.mu.RLock()
    defer vm.mu.RUnlock()
    
    proposal, ok := vm.proposals[proposalID]
    if !ok {
        return nil, errors.New("proposal not found")
    }
    
    return proposal, nil
}

// GetProposalVotes 获取提案投票
func (vm *InMemoryVotingManager) GetProposalVotes(proposalID common.Hash) ([]*Vote, error) {
    vm.mu.RLock()
    defer vm.mu.RUnlock()
    
    votes, ok := vm.votes[proposalID]
    if !ok {
        return nil, errors.New("proposal not found")
    }
    
    return votes, nil
}

// GetActiveProposals 获取活跃提案
func (vm *InMemoryVotingManager) GetActiveProposals() []*Proposal {
    vm.mu.RLock()
    defer vm.mu.RUnlock()
    
    active := make([]*Proposal, 0)
    for _, p := range vm.proposals {
        if p.Status == ProposalStatusPending {
            active = append(active, p)
        }
    }
    
    return active
}

func boolToBytes(b bool) []byte {
    if b {
        return []byte{1}
    }
    return []byte{0}
}
```

### 节点准入控制器实现

```go
// governance/admission_impl.go
package governance

import (
    "errors"
    "sync"
    "time"
    
    "github.com/ethereum/go-ethereum/common"
)

// SGXAdmissionController SGX 准入控制器
type SGXAdmissionController struct {
    mu                  sync.RWMutex
    whitelist           WhitelistManager
    verifier            SGXVerifier
    status              map[common.Hash]*AdmissionStatus
    
    // 硬件唯一性约束：每个 SGX CPU 只能运行一个节点
    hardwareToValidator map[string]common.Address  // 硬件 ID -> 验证者地址
    validatorToHardware map[common.Address]string  // 验证者地址 -> 硬件 ID
}

// SGXVerifier SGX 验证器接口
type SGXVerifier interface {
    VerifyQuote(quote []byte) error
    ExtractMREnclave(quote []byte) ([32]byte, error)
    ExtractHardwareID(quote []byte) (string, error)  // 提取硬件唯一标识
}

// NewSGXAdmissionController 创建准入控制器
func NewSGXAdmissionController(whitelist WhitelistManager, verifier SGXVerifier) *SGXAdmissionController {
    return &SGXAdmissionController{
        whitelist:           whitelist,
        verifier:            verifier,
        status:              make(map[common.Hash]*AdmissionStatus),
        hardwareToValidator: make(map[string]common.Address),
        validatorToHardware: make(map[common.Address]string),
    }
}

// CheckAdmission 检查节点准入
func (ac *SGXAdmissionController) CheckAdmission(nodeID common.Hash, mrenclave [32]byte, quote []byte) (bool, error) {
    // 1. 验证 SGX Quote
    if err := ac.verifier.VerifyQuote(quote); err != nil {
        return false, errors.New("invalid SGX quote: " + err.Error())
    }
    
    // 2. 从 Quote 中提取 MRENCLAVE
    extractedMREnclave, err := ac.verifier.ExtractMREnclave(quote)
    if err != nil {
        return false, errors.New("failed to extract MRENCLAVE: " + err.Error())
    }
    
    // 3. 验证 MRENCLAVE 匹配
    if extractedMREnclave != mrenclave {
        return false, errors.New("MRENCLAVE mismatch")
    }
    
    // 4. 检查白名单
    if !ac.whitelist.IsAllowed(mrenclave) {
        return false, errors.New("MRENCLAVE not in whitelist")
    }
    
    // 5. 检查硬件唯一性（每个 SGX CPU 只能运行一个节点）
    hardwareID, err := ac.verifier.ExtractHardwareID(quote)
    if err != nil {
        return false, errors.New("failed to extract hardware ID: " + err.Error())
    }
    
    ac.mu.Lock()
    defer ac.mu.Unlock()
    
    if existingValidator, exists := ac.hardwareToValidator[hardwareID]; exists {
        // 该硬件已注册其他验证者
        return false, fmt.Errorf(
            "hardware ID %s already registered to validator %s, each SGX CPU can only run one node",
            hardwareID, existingValidator.Hex(),
        )
    }
    
    // 6. 记录准入状态和硬件绑定
    validatorAddr := common.BytesToAddress(nodeID[:20])
    ac.hardwareToValidator[hardwareID] = validatorAddr
    ac.validatorToHardware[validatorAddr] = hardwareID
    
    ac.status[nodeID] = &AdmissionStatus{
        NodeID:       nodeID,
        MRENCLAVE:    mrenclave,
        Allowed:      true,
        ConnectedAt:  uint64(time.Now().Unix()),
        LastVerified: uint64(time.Now().Unix()),
    }
    
    return true, nil
}

// GetAdmissionStatus 获取准入状态
func (ac *SGXAdmissionController) GetAdmissionStatus(nodeID common.Hash) (*AdmissionStatus, error) {
    ac.mu.RLock()
    defer ac.mu.RUnlock()
    
    status, ok := ac.status[nodeID]
    if !ok {
        return nil, errors.New("node not found")
    }
    
    return status, nil
}

// RecordConnection 记录连接
func (ac *SGXAdmissionController) RecordConnection(nodeID common.Hash, mrenclave [32]byte) error {
    ac.mu.Lock()
    defer ac.mu.Unlock()
    
    ac.status[nodeID] = &AdmissionStatus{
        NodeID:      nodeID,
        MRENCLAVE:   mrenclave,
        Allowed:     true,
        ConnectedAt: uint64(time.Now().Unix()),
    }
    
    return nil
}

// RecordDisconnection 记录断开
func (ac *SGXAdmissionController) RecordDisconnection(nodeID common.Hash) error {
    ac.mu.Lock()
    defer ac.mu.Unlock()
    
    delete(ac.status, nodeID)
    return nil
}

// GetHardwareBinding 获取硬件绑定信息
func (ac *SGXAdmissionController) GetHardwareBinding(validatorAddr common.Address) (string, bool) {
    ac.mu.RLock()
    defer ac.mu.RUnlock()
    
    hardwareID, exists := ac.validatorToHardware[validatorAddr]
    return hardwareID, exists
}

// GetValidatorByHardware 根据硬件 ID 获取验证者地址
func (ac *SGXAdmissionController) GetValidatorByHardware(hardwareID string) (common.Address, bool) {
    ac.mu.RLock()
    defer ac.mu.RUnlock()
    
    validator, exists := ac.hardwareToValidator[hardwareID]
    return validator, exists
}

// UnregisterValidator 注销验证者（释放硬件绑定）
func (ac *SGXAdmissionController) UnregisterValidator(validatorAddr common.Address) error {
    ac.mu.Lock()
    defer ac.mu.Unlock()
    
    hardwareID, exists := ac.validatorToHardware[validatorAddr]
    if !exists {
        return errors.New("validator not registered")
    }
    
    delete(ac.hardwareToValidator, hardwareID)
    delete(ac.validatorToHardware, validatorAddr)
    
    return nil
}
```

### 硬件唯一性验证说明

**设计原理**：每个 SGX CPU 实例只能运行一个验证节点，防止女巫攻击和重复投票。

**硬件 ID 提取**：从 SGX Quote 中提取硬件唯一标识，该标识对于每个物理 CPU 是唯一的。

```go
// extractHardwareID 从 SGX Quote 中提取硬件唯一标识
func extractHardwareID(quote []byte) (string, error) {
    // SGX Quote 结构中包含硬件相关信息：
    // - EPID 模式：使用 EPID Group ID
    // - DCAP 模式：使用 QE_ID（Quoting Enclave ID）
    
    if len(quote) < 48 {
        return "", errors.New("quote too short")
    }
    
    // 提取 Quote 头部的版本信息
    version := binary.LittleEndian.Uint16(quote[0:2])
    
    switch version {
    case 2: // EPID Quote
        // EPID Group ID 位于 Quote 的特定偏移位置
        epidGroupID := quote[4:8]
        return hex.EncodeToString(epidGroupID), nil
        
    case 3: // DCAP Quote
        // QE_ID 位于 Quote Body 中
        // 实际实现需要解析完整的 Quote 结构
        qeID := quote[16:32]
        return hex.EncodeToString(qeID), nil
        
    default:
        return "", fmt.Errorf("unsupported quote version: %d", version)
    }
}
```

**安全保障**：

| 攻击场景 | 防护机制 | 效果 |
|----------|----------|------|
| 同一 CPU 运行多个节点 | 硬件 ID 唯一性检查 | 拒绝重复注册 |
| 伪造硬件 ID | SGX Quote 签名验证 | 无法伪造有效 Quote |
| 恶意节点多次投票 | 硬件绑定 + 链上记录 | 每个物理 CPU 只能投一票 |

### 渐进式权限管理器

```go
// governance/progressive_permission.go
package governance

import (
    "sync"
    "time"
)

// ProgressivePermissionConfig 渐进式权限配置
type ProgressivePermissionConfig struct {
    // 基础权限持续时间（区块数）
    BasicDuration uint64
    
    // 标准权限持续时间（区块数）
    StandardDuration uint64
    
    // 升级到标准权限的最小在线率
    StandardUptimeThreshold float64
    
    // 升级到完整权限的最小在线率
    FullUptimeThreshold float64
}

// DefaultProgressivePermissionConfig 默认配置
func DefaultProgressivePermissionConfig() *ProgressivePermissionConfig {
    return &ProgressivePermissionConfig{
        BasicDuration:           40320,  // 约 7 天
        StandardDuration:        120960, // 约 21 天
        StandardUptimeThreshold: 0.95,   // 95%
        FullUptimeThreshold:     0.99,   // 99%
    }
}

// ProgressivePermissionManager 渐进式权限管理器
type ProgressivePermissionManager struct {
    config    *ProgressivePermissionConfig
    mu        sync.RWMutex
    nodePerms map[[32]byte]*NodePermission
}

// NodePermission 节点权限
type NodePermission struct {
    MRENCLAVE       [32]byte
    CurrentLevel    PermissionLevel
    ActivatedAt     uint64
    LastUpgradeAt   uint64
    UptimeHistory   []float64
}

// NewProgressivePermissionManager 创建管理器
func NewProgressivePermissionManager(config *ProgressivePermissionConfig) *ProgressivePermissionManager {
    return &ProgressivePermissionManager{
        config:    config,
        nodePerms: make(map[[32]byte]*NodePermission),
    }
}

// GetPermissionLevel 获取当前权限级别
func (pm *ProgressivePermissionManager) GetPermissionLevel(mrenclave [32]byte, currentBlock uint64) PermissionLevel {
    pm.mu.RLock()
    defer pm.mu.RUnlock()
    
    perm, ok := pm.nodePerms[mrenclave]
    if !ok {
        return PermissionBasic
    }
    
    return perm.CurrentLevel
}

// CheckUpgrade 检查是否可以升级权限
func (pm *ProgressivePermissionManager) CheckUpgrade(mrenclave [32]byte, currentBlock uint64, uptime float64) (bool, PermissionLevel) {
    pm.mu.Lock()
    defer pm.mu.Unlock()
    
    perm, ok := pm.nodePerms[mrenclave]
    if !ok {
        // 新节点，初始化为基础权限
        pm.nodePerms[mrenclave] = &NodePermission{
            MRENCLAVE:    mrenclave,
            CurrentLevel: PermissionBasic,
            ActivatedAt:  currentBlock,
        }
        return false, PermissionBasic
    }
    
    // 记录在线率
    perm.UptimeHistory = append(perm.UptimeHistory, uptime)
    
    // 计算平均在线率
    avgUptime := pm.calculateAverageUptime(perm.UptimeHistory)
    
    switch perm.CurrentLevel {
    case PermissionBasic:
        // 检查是否可以升级到标准权限
        elapsed := currentBlock - perm.ActivatedAt
        if elapsed >= pm.config.BasicDuration && avgUptime >= pm.config.StandardUptimeThreshold {
            perm.CurrentLevel = PermissionStandard
            perm.LastUpgradeAt = currentBlock
            return true, PermissionStandard
        }
        
    case PermissionStandard:
        // 检查是否可以升级到完整权限
        elapsed := currentBlock - perm.LastUpgradeAt
        if elapsed >= pm.config.StandardDuration && avgUptime >= pm.config.FullUptimeThreshold {
            perm.CurrentLevel = PermissionFull
            perm.LastUpgradeAt = currentBlock
            return true, PermissionFull
        }
    }
    
    return false, perm.CurrentLevel
}

// calculateAverageUptime 计算平均在线率
func (pm *ProgressivePermissionManager) calculateAverageUptime(history []float64) float64 {
    if len(history) == 0 {
        return 0
    }
    
    sum := 0.0
    for _, u := range history {
        sum += u
    }
    
    return sum / float64(len(history))
}

// Downgrade 降级权限
func (pm *ProgressivePermissionManager) Downgrade(mrenclave [32]byte, reason string) {
    pm.mu.Lock()
    defer pm.mu.Unlock()
    
    perm, ok := pm.nodePerms[mrenclave]
    if !ok {
        return
    }
    
    // 降级一级
    switch perm.CurrentLevel {
    case PermissionFull:
        perm.CurrentLevel = PermissionStandard
    case PermissionStandard:
        perm.CurrentLevel = PermissionBasic
    }
}
```

## 文件结构

```
governance/
├── whitelist.go                  # 白名单数据结构
├── whitelist_manager.go          # 白名单管理器接口
├── whitelist_manager_impl.go     # 白名单管理器实现
├── voting.go                     # 投票数据结构
├── voting_manager.go             # 投票管理器接口
├── voting_manager_impl.go        # 投票管理器实现
├── validator.go                  # 验证者数据结构
├── validator_manager.go          # 验证者管理器接口
├── validator_manager_impl.go     # 验证者管理器实现
├── admission.go                  # 准入控制接口
├── admission_impl.go             # 准入控制实现
├── progressive_permission.go     # 渐进式权限
├── staking.go                    # 质押管理
└── governance_test.go            # 测试
```

## 单元测试指南

### 白名单测试

```go
// governance/whitelist_test.go
package governance

import (
    "testing"
    
    "github.com/ethereum/go-ethereum/common"
)

func TestWhitelistAdd(t *testing.T) {
    config := DefaultWhitelistConfig()
    validators := NewMockValidatorManager()
    voting := NewInMemoryVotingManager(config, validators)
    whitelist := NewInMemoryWhitelistManager(config, voting)
    
    proposer := common.HexToAddress("0x1234")
    validators.AddValidator(proposer, VoterTypeCore)
    
    mrenclave := [32]byte{1, 2, 3}
    
    // 提议添加
    proposalID, err := whitelist.ProposeAdd(proposer, mrenclave, "v1.0.0")
    if err != nil {
        t.Fatalf("ProposeAdd failed: %v", err)
    }
    
    if proposalID == (common.Hash{}) {
        t.Error("Expected non-zero proposal ID")
    }
}

func TestWhitelistCheck(t *testing.T) {
    config := DefaultWhitelistConfig()
    validators := NewMockValidatorManager()
    voting := NewInMemoryVotingManager(config, validators)
    whitelist := NewInMemoryWhitelistManager(config, voting)
    
    mrenclave := [32]byte{1, 2, 3}
    
    // 未添加时应该返回 false
    if whitelist.IsAllowed(mrenclave) {
        t.Error("MRENCLAVE should not be allowed before adding")
    }
    
    // 添加后应该返回 true
    whitelist.AddEntry(&MREnclaveEntry{
        MRENCLAVE: mrenclave,
        Status:    StatusActive,
    })
    
    if !whitelist.IsAllowed(mrenclave) {
        t.Error("MRENCLAVE should be allowed after adding")
    }
}
```

### 投票测试

```go
// governance/voting_test.go
package governance

import (
    "testing"
    
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/crypto"
)

func TestVoting(t *testing.T) {
    config := DefaultWhitelistConfig()
    validators := NewMockValidatorManager()
    voting := NewInMemoryVotingManager(config, validators)
    
    // 添加验证者
    voter1 := common.HexToAddress("0x1111")
    voter2 := common.HexToAddress("0x2222")
    validators.AddValidator(voter1, VoterTypeCore)
    validators.AddValidator(voter2, VoterTypeCore)
    
    // 创建提案
    proposal := &Proposal{
        ID:        common.HexToHash("0x1234"),
        Type:      ProposalAddMREnclave,
        Proposer:  voter1,
        CreatedAt: 100,
    }
    
    proposalID, err := voting.CreateProposal(proposal)
    if err != nil {
        t.Fatalf("CreateProposal failed: %v", err)
    }
    
    // 投票
    privateKey, _ := crypto.GenerateKey()
    voteHash := crypto.Keccak256Hash(proposalID.Bytes(), voter1.Bytes(), []byte{1})
    signature, _ := crypto.Sign(voteHash.Bytes(), privateKey)
    
    // 注意：实际测试中需要使用正确的私钥
    // 这里简化处理
}

func TestVotingThreshold(t *testing.T) {
    config := DefaultWhitelistConfig()
    config.CoreValidatorThreshold = 67 // 2/3
    
    validators := NewMockValidatorManager()
    voting := NewInMemoryVotingManager(config, validators)
    
    // 添加 3 个核心验证者
    for i := 0; i < 3; i++ {
        addr := common.BigToAddress(big.NewInt(int64(i + 1)))
        validators.AddValidator(addr, VoterTypeCore)
    }
    
    // 创建提案并投票
    // 2/3 = 67%，需要至少 2 票
}
```

### 准入控制测试

```go
// governance/admission_test.go
package governance

import (
    "testing"
    
    "github.com/ethereum/go-ethereum/common"
)

func TestAdmissionControl(t *testing.T) {
    whitelist := NewMockWhitelistManager()
    verifier := NewMockSGXVerifier()
    admission := NewSGXAdmissionController(whitelist, verifier)
    
    nodeID := common.HexToHash("0x1234")
    mrenclave := [32]byte{1, 2, 3}
    quote := []byte("mock quote")
    
    // 未在白名单中
    allowed, err := admission.CheckAdmission(nodeID, mrenclave, quote)
    if allowed {
        t.Error("Should not be allowed when not in whitelist")
    }
    
    // 添加到白名单
    whitelist.Allow(mrenclave)
    
    // 现在应该允许
    allowed, err = admission.CheckAdmission(nodeID, mrenclave, quote)
    if err != nil {
        t.Fatalf("CheckAdmission failed: %v", err)
    }
    if !allowed {
        t.Error("Should be allowed when in whitelist")
    }
}
```

## 配置参数

**重要说明**：以下配置参数的值存储在 **SecurityConfigContract** 中，可以通过 **GovernanceContract** 的投票机制进行修改。代码中的默认值仅用于创世区块初始化，实际运行时必须从合约中读取最新配置。

### 参数修改流程

```
参数修改投票流程
================

1. 提案阶段
   ├── 核心验证者提交参数修改提案
   ├── 提案类型：ProposalParameterChange
   └── 包含：参数名、新值、修改理由

2. 投票阶段
   ├── 核心验证者投票（需要 2/3 通过）
   └── 社区验证者可以行使否决权（1/3 否决）

3. 执行阶段
   ├── 投票通过后进入执行延迟期
   ├── GovernanceContract 调用 SecurityConfigContract.SetParameter()
   └── 参数更新生效

4. 生效机制
   ├── 所有节点从 SecurityConfigContract 读取最新配置
   └── 下一个区块开始使用新参数
```

### 配置示例

```toml
# config.toml
[governance]
# 核心验证者投票阈值（百分比）
core_validator_threshold = 67

# 社区验证者投票阈值（百分比）
community_validator_threshold = 51

# 投票期限（区块数）
voting_period = 40320

# 执行延迟（区块数）
execution_delay = 5760

# 最小投票参与率（百分比）
min_participation = 50

[governance.staking]
# 最小质押金额（初始值，可通过治理投票修改）
min_stake_amount = "10000000000000000000000"  # 初始值：10000 X

# 解除质押锁定期（区块数）
unstake_lock_period = 40320

# 年化奖励率（百分比）
annual_reward_rate = 5

[governance.progressive]
# 基础权限持续时间（区块数）
basic_duration = 40320

# 标准权限持续时间（区块数）
standard_duration = 120960

# 升级到标准权限的最小在线率
standard_uptime_threshold = 0.95

# 升级到完整权限的最小在线率
full_uptime_threshold = 0.99
```

## 实现优先级

| 优先级 | 功能 | 预计工时 |
|--------|------|----------|
| P0 | 白名单管理器 | 3 天 |
| P0 | 节点准入控制 | 3 天 |
| P0 | 投票系统基础 | 4 天 |
| P1 | 验证者管理 | 3 天 |
| P1 | 渐进式权限 | 2 天 |
| P2 | 质押管理 | 3 天 |
| P2 | 投票透明性查询 | 2 天 |

**总计：约 3 周**

## 注意事项

1. **安全性**：投票签名必须验证，防止伪造投票
2. **原子性**：提案执行必须是原子操作
3. **状态一致性**：确保所有节点的治理状态一致
4. **升级兼容**：硬分叉升级时保持向后兼容
5. **防女巫攻击**：通过质押和声誉防止女巫攻击
