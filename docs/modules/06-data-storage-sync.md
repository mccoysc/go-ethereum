# 模块 06: 数据存储与同步

## 1. 模块概述

本模块实现 X Chain 的数据持久化和节点间秘密数据同步，管理加密分区存储、秘密数据安全传输及节点间数据一致性。

## 2. 模块职责

### 2.1 核心功能

1. **加密分区管理**：管理 Gramine 加密文件系统的数据存储
2. **秘密数据存储**：存储私钥、ECDH 派生秘密、节点身份等敏感数据
3. **节点间秘密数据同步**：通过 RA-TLS 实现跨节点秘密数据安全传输
4. **数据一致性验证**：保证所有节点的秘密数据和区块链数据一致性
5. **参数处理**：合并 Manifest、链上、命令行参数，确保安全参数不被覆盖
6. **侧信道攻击防护**：实现常量时间操作防止时序泄露

## 3. 架构设计

### 3.1 依赖关系

```
+----------------------+
|  数据存储与同步模块  |
+----------------------+
        |
        +---> SGX 证明模块（RA-TLS 传输、度量值验证）
        |
        +---> 治理模块（MRENCLAVE 白名单、权限级别、升级协调）
        |
        +---> Gramine 运行时（加密文件系统）
        |
        +---> 共识引擎模块（UpgradeCompleteBlock 同步）
```

### 3.2 上游依赖
- SGX 证明模块（RA-TLS 安全通道、双向度量值验证）
- 治理模块（通过 SecurityConfigContract 获取 MRENCLAVE 白名单、PermissionLevel、迁移策略）
- Gramine LibOS（加密文件系统、密钥封装/解封）
- 共识引擎（读取当前区块高度、UpgradeCompleteBlock 参数）

### 3.3 下游依赖
- 预编译合约模块（密钥存储、ECDH 秘密存储）
- 共识引擎模块（状态持久化、区块数据存储）
- 治理模块（通过加密分区存储投票记录）

### 3.4 治理模块集成

本模块通过以下机制与治理模块集成：

**MRENCLAVE 白名单验证**
- 从 SecurityConfigContract 读取白名单
- 秘密数据同步前验证对端节点 MRENCLAVE
- 动态更新通过链上投票管理

**权限级别（PermissionLevel）机制**
- 新 MRENCLAVE 添加时具有渐进式权限：
  - `Basic`（7 天）：日迁移限制 10 次
  - `Standard`（30 天）：日迁移限制 100 次
  - `Full`（永久）：无迁移限制
- AutoMigrationManager 实现权限级别检查和迁移频率限制

**升级协调机制**
- 治理设置 `UpgradeCompleteBlock` 参数控制升级时间窗口
- AutoMigrationManager 在该区块高度前完成秘密数据迁移
- 迁移完成条件：`secretDataSyncedBlock >= UpgradeCompleteBlock`

### 3.5 秘密数据同步触发机制

秘密数据同步由以下事件触发：

**新节点启动**
- 触发条件：`localSecretDataVersion == 0`
- 实现：检测加密分区为空，从现有节点请求同步

**MRENCLAVE 白名单更新（自动）**
- 触发条件：`newMREnclave ∈ whitelist AND permissionLevel >= Basic`
- 实现：AutoMigrationManager 监控白名单变化，自动启动迁移

**升级协调**
- 触发条件：`currentBlock < UpgradeCompleteBlock AND !migrationComplete`
- 实现：AutoMigrationManager 根据 UpgradeCompleteBlock 调度迁移任务

## 4. 参数管理

### 4.1 参数分类

X Chain 配置参数分为三类：

| 类别 | 控制方式 | 特点 | 示例 |
|------|----------|------|------|
| **Manifest 固定参数** | Gramine Manifest | 影响度量值，不可外部修改 | 本地路径配置、链上合约地址 |
| **链上安全参数** | 链上合约 | 通过投票管理，动态生效 | 白名单、密钥迁移阈值、准入策略 |
| **非安全参数** | 命令行参数 | 不影响安全性，可灵活配置 | 出块间隔、RPC 端口、日志级别 |

### 4.2 Manifest 固定参数

Manifest 中存储本地配置和链上合约地址，作为安全锚点。

```toml
# Gramine manifest 固定参数
[loader.env]
# 本地路径
XCHAIN_ENCRYPTED_PATH = "/data/encrypted"
XCHAIN_SECRET_PATH = "/data/secrets"

# 链上合约地址（影响 MRENCLAVE）
XCHAIN_GOVERNANCE_CONTRACT = "0x1234567890abcdef1234567890abcdef12345678"
XCHAIN_SECURITY_CONFIG_CONTRACT = "0xabcdef1234567890abcdef1234567890abcdef12"
```

### 4.3 链上安全参数

从链上合约动态读取安全参数，通过投票管理。

| 参数 | 链上合约 | 说明 |
|------|----------|------|
| MRENCLAVE 白名单 | SecurityConfigContract | 允许的 enclave 代码度量值 |
| MRSIGNER 白名单 | SecurityConfigContract | 允许的签名者度量值 |
| 密钥迁移阈值 | SecurityConfigContract | 密钥迁移所需的最小节点数 |
| 节点准入策略 | SecurityConfigContract | 是否严格验证 Quote |
| 分叉配置 | SecurityConfigContract | 硬分叉升级相关配置 |
| 数据迁移策略 | SecurityConfigContract | 加密数据迁移相关配置 |
| 投票阈值 | GovernanceContract | 提案通过所需的投票比例 |
| 投票期限 | GovernanceContract | 提案投票的区块数 |

**合约职责**
- `SecurityConfigContract`：存储安全配置，被其他模块读取
- `GovernanceContract`：管理投票流程，更新 SecurityConfigContract

```go
// 链上参数同步实现
type OnChainConfigSync struct {
    governanceContract     common.Address
    securityConfigContract common.Address
    client                 *ethclient.Client
}

func NewOnChainConfigSync() (*OnChainConfigSync, error) {
    govAddr := os.Getenv("XCHAIN_GOVERNANCE_CONTRACT")
    scAddr := os.Getenv("XCHAIN_SECURITY_CONFIG_CONTRACT")
    
    return &OnChainConfigSync{
        governanceContract:     common.HexToAddress(govAddr),
        securityConfigContract: common.HexToAddress(scAddr),
    }, nil
}

func (s *OnChainConfigSync) SyncSecurityParams() (*SecurityConfig, error) {
    config := &SecurityConfig{}
    config.AllowedMREnclave = s.fetchWhitelist()
    config.KeyMigrationThreshold = s.fetchKeyMigrationThreshold()
    config.AdmissionStrict = s.fetchAdmissionPolicy()
    config.VotingThreshold = s.fetchVotingThreshold()
    return config, nil
}
```

**安全保证**
- 合约地址固定在 Manifest 中，影响 MRENCLAVE
- 安全参数通过链上共识保证一致性
- 节点定期同步最新参数

### 4.4 非安全参数

通过命令行配置的运行时参数。

```bash
# 命令行参数示例
./geth \
    --xchain.block.interval=15 \
    --xchain.rpc.port=8545 \
    --xchain.log.level=info
```

### 4.5 参数合并机制

**处理流程**
1. 从环境变量加载 Manifest 参数
2. 解析命令行参数
3. 读取链上参数
4. 合并参数：Manifest > 链上 > 命令行

```go
// config/param_validator.go
package config

type ParamCategory uint8

const (
    ParamCategorySecurity ParamCategory = 0x01
    ParamCategoryRuntime  ParamCategory = 0x02
)

type ParamDefinition struct {
    Name      string
    Category  ParamCategory
    EnvKey    string
    CliFlag   string
    Required  bool
    Default   string
    Validator func(string) error
}

var SecurityParams = []ParamDefinition{
    {
        Name:     "encrypted_path",
        Category: ParamCategorySecurity,
        EnvKey:   "XCHAIN_ENCRYPTED_PATH",
        CliFlag:  "xchain.encrypted-path",
        Required: true,
    },
    {
        Name:     "secret_path",
        Category: ParamCategorySecurity,
        EnvKey:   "XCHAIN_SECRET_PATH",
        CliFlag:  "xchain.secret-path",
        Required: true,
    },
}

type ParamValidator struct {
    manifestParams map[string]string
    chainParams    map[string]interface{}
    cliParams      map[string]string
    mergedParams   map[string]interface{}
}

func (pv *ParamValidator) MergeAndValidate() error {
    // 优先级：Manifest > 链上参数 > 命令行参数
    for name, value := range pv.manifestParams {
        pv.mergedParams[name] = value
    }
    
    for name, value := range pv.chainParams {
        if _, exists := pv.manifestParams[name]; !exists {
            pv.mergedParams[name] = value
        }
    }
    
    for cliFlag, cliValue := range pv.cliParams {
        isSecurityParam := false
        for _, param := range SecurityParams {
            if param.CliFlag == cliFlag {
                isSecurityParam = true
                if _, exists := pv.manifestParams[param.Name]; !exists {
                    if _, exists := pv.chainParams[param.Name]; !exists {
                        pv.mergedParams[param.Name] = cliValue
                    }
                }
                break
            }
        }
        if !isSecurityParam {
            pv.mergedParams[cliFlag] = cliValue
        }
    }
    
    return nil
}
```

### 4.6 启动时参数处理流程

```go
// cmd/geth/main.go
package main

import (
    "github.com/ethereum/go-ethereum/config"
)

func initializeParams() (*config.ParamValidator, error) {
    validator := config.NewParamValidator()
    
    // 1. 加载 Manifest 参数
    if err := validator.LoadManifestParams(); err != nil {
        return nil, err
    }
    
    // 2. 从链上合约同步安全参数
    if err := validator.LoadChainParams(); err != nil {
        return nil, err
    }
    
    // 3. 加载命令行参数
    if err := validator.LoadCliParams(os.Args[1:]); err != nil {
        return nil, err
    }
    
    // 4. 合并参数（Manifest > 链上 > 命令行）
    if err := validator.MergeAndValidate(); err != nil {
        return nil, err
    }
    
    return validator, nil
}
```

## 5. 核心接口定义

### 5.1 EncryptedPartition 接口

```go
// storage/encrypted_partition.go
package storage

type EncryptedPartition interface {
    WriteSecret(id string, data []byte) error
    ReadSecret(id string) ([]byte, error)
    DeleteSecret(id string) error
    ListSecrets() ([]string, error)
    SecureDelete(filePath string) error
}
```

### 5.2 SyncManager 接口

```go
// storage/sync_manager.go
package storage

import (
    "context"
    "github.com/ethereum/go-ethereum/common"
)

type SyncManager interface {
    RequestSync(peerID common.Hash, secretTypes []SecretDataType) (common.Hash, error)
    HandleSyncRequest(request *SyncRequest) (*SyncResponse, error)
    VerifyAndApplySync(response *SyncResponse) error
    AddPeer(peerID common.Hash, mrenclave [32]byte, quote []byte) error
    RemovePeer(peerID common.Hash) error
    GetSyncStatus(peerID common.Hash) (SyncStatus, error)
    StartHeartbeat(ctx context.Context) error
}
```

### 5.3 AutoMigrationManager 接口

实现自动密钥迁移，与治理模块集成，根据 PermissionLevel 控制迁移频率。

```go
// storage/auto_migration_manager.go
package storage

import (
    "context"
    "github.com/ethereum/go-ethereum/common"
)

type PermissionLevel uint8

const (
    PermissionBasic    PermissionLevel = 0x01  // 基础权限（7天），日限10次
    PermissionStandard PermissionLevel = 0x02  // 标准权限（30天），日限100次
    PermissionFull     PermissionLevel = 0x03  // 完全权限（永久），无限制
)

type AutoMigrationManager interface {
    StartMonitoring(ctx context.Context) error
    CheckAndMigrate() (bool, error)
    GetMigrationStatus() (*MigrationStatus, error)
    VerifyPermissionLevel(mrenclave [32]byte) (PermissionLevel, error)
    EnforceMigrationLimit() error
}
```

### 5.4 ParameterValidator 接口

注意：`MergeAndValidate` 方法处理三类参数：Manifest（环境变量）、链上参数（从 SecurityConfigContract 读取）、命令行参数。优先级：Manifest > 链上 > 命令行。

```go
// storage/parameter_validator.go
package storage

type ParameterValidator interface {
    ValidateManifestParams(manifestParams map[string]string) error
    ValidateChainParams(chainParams map[string]interface{}) error
    MergeAndValidate(
        manifestParams map[string]string,
        chainParams map[string]interface{},
        cmdLineParams map[string]interface{},
    ) (map[string]interface{}, error)
    CheckSecurityParams() error
}
```

## 6. 数据结构定义

### 6.1 存储配置

```go
// storage/config.go
package storage

type StorageConfig struct {
    EncryptedPath string  // 加密分区路径（Manifest）
    DataPath      string  // 普通数据路径
    SecretPath    string  // 秘密数据路径（Manifest）
    CacheSize     int     // 缓存大小（运行时）
    SyncInterval  int     // 同步间隔（运行时）
}

type SecretDataType uint8

const (
    SecretTypePrivateKey   SecretDataType = 0x01
    SecretTypeSealingKey   SecretDataType = 0x02
    SecretTypeNodeIdentity SecretDataType = 0x03
    SecretTypeSharedSecret SecretDataType = 0x04
)

type SecretData struct {
    Type      SecretDataType
    ID        []byte
    Data      []byte
    CreatedAt uint64
    ExpiresAt uint64
    Metadata  map[string]string
}
```

### 6.2 加密分区实现

Gramine 透明处理加密，应用层使用标准文件 I/O。

```go
// storage/encrypted_partition_impl.go
package storage

import (
    "fmt"
    "os"
    "path/filepath"
    "sync"
)

type EncryptedPartitionImpl struct {
    mu       sync.RWMutex
    basePath string
}

func NewEncryptedPartition(basePath string) (*EncryptedPartitionImpl, error) {
    if _, err := os.Stat(basePath); os.IsNotExist(err) {
        return nil, fmt.Errorf("encrypted partition path does not exist: %s", basePath)
    }
    
    return &EncryptedPartitionImpl{
        basePath: basePath,
    }, nil
}
```

// WriteSecret 写入秘密数据
// 应用只需调用标准文件写入，Gramine 会透明地自动加密数据
func (ep *EncryptedPartition) WriteSecret(id string, data []byte) error {
    ep.mu.Lock()
    defer ep.mu.Unlock()
    
    filePath := filepath.Join(ep.basePath, id)
    
    // 标准的文件写入操作
    // Gramine 在底层透明地加密数据，应用无感知
    file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
    if err != nil {
        return fmt.Errorf("failed to open file: %w", err)
    }
    defer file.Close()
    
    // 应用写入明文，Gramine 自动加密后存储到磁盘
    if _, err := file.Write(data); err != nil {
        return fmt.Errorf("failed to write data: %w", err)
    }
    
    return nil
}

// ReadSecret 读取秘密数据
// 应用只需调用标准文件读取，Gramine 会透明地自动解密数据
func (ep *EncryptedPartition) ReadSecret(id string) ([]byte, error) {
    ep.mu.RLock()
    defer ep.mu.RUnlock()
    
    filePath := filepath.Join(ep.basePath, id)
    
    // 标准的文件读取操作
    // Gramine 在底层透明地解密数据，应用直接获得明文
    data, err := os.ReadFile(filePath)
    if err != nil {
        return nil, fmt.Errorf("failed to read secret: %w", err)
    }
    
    return data, nil
}

// DeleteSecret 删除秘密数据
func (ep *EncryptedPartition) DeleteSecret(id string) error {
    ep.mu.Lock()
    defer ep.mu.Unlock()
    
    filePath := filepath.Join(ep.basePath, id)
    
    // 安全删除：先覆盖再删除
    if err := ep.secureDelete(filePath); err != nil {
        return err
    }
    
    return nil
}

// secureDelete 安全删除文件
func (ep *EncryptedPartition) secureDelete(filePath string) error {
    // 获取文件大小
    info, err := os.Stat(filePath)
    if err != nil {
        return err
    }
    
    // 用随机数据覆盖
    file, err := os.OpenFile(filePath, os.O_WRONLY, 0600)
    if err != nil {
        return err
    }
    
    randomData := make([]byte, info.Size())
    if _, err := io.ReadFull(rand.Reader, randomData); err != nil {
        file.Close()
        return err
    }
    
    if _, err := file.Write(randomData); err != nil {
        file.Close()
        return err
    }
    file.Close()
    
    // 删除文件
    return os.Remove(filePath)
}

// ListSecrets 列出所有秘密数据 ID
func (ep *EncryptedPartition) ListSecrets() ([]string, error) {
    ep.mu.RLock()
    defer ep.mu.RUnlock()
    
    entries, err := os.ReadDir(ep.basePath)
    if err != nil {
        return nil, err
    }
    
    ids := make([]string, 0, len(entries))
    for _, entry := range entries {
        if !entry.IsDir() {
            ids = append(ids, entry.Name())
        }
    }
    
    return ids, nil
}
```

## 7. 实现要点

### 7.1 加密分区初始化

Gramine manifest 配置加密分区：

```toml
# geth.manifest.template
fs.mounts = [
    { path = "/data/encrypted", uri = "file:/data/encrypted", type = "encrypted" },
]
```

应用层使用标准文件 I/O，Gramine 自动加解密：

```go
// 写入私钥（Gramine 自动加密）
os.WriteFile("/data/encrypted/key.bin", keyData, 0600)

// 读取私钥（Gramine 自动解密）
data, _ := os.ReadFile("/data/encrypted/key.bin")
```

### 7.2 秘密数据同步实现

**核心流程**
1. RA-TLS 握手，双向验证 MRENCLAVE
2. 验证对端在白名单中
3. ECDH 建立共享密钥
4. 加密传输秘密数据
5. 写入目标节点加密分区

**关键点**
- 传输前必须验证 MRENCLAVE 白名单
- 使用 RA-TLS 端到端加密
- Gramine 透明处理本地加解密

### 7.3 AutoMigrationManager 实现

**监控逻辑**
1. 定期从 SecurityConfigContract 读取白名单
2. 检测新增 MRENCLAVE
3. 验证 PermissionLevel（Basic/Standard/Full）
4. 根据权限级别限制迁移频率
5. 在 UpgradeCompleteBlock 前完成迁移

**权限限制**
```go
func (am *AutoMigration) getDailyLimit(level PermissionLevel) int {
    switch level {
    case PermissionBasic:
        return 10   // 日限10次
    case PermissionStandard:
        return 100  // 日限100次
    case PermissionFull:
        return -1   // 无限制
    }
    return 0
}
```

### 7.4 侧信道攻击防护

**常量时间操作**
- 使用 `crypto/subtle` 包进行常量时间比较
- 避免基于秘密数据的条件分支
- 密钥比较必须用 `subtle.ConstantTimeCompare`

**内存安全**
- 使用后立即清零敏感数据缓冲区
- 使用 `runtime.KeepAlive` 防止过早回收

加密分区由 Gramine 提供**透明加密**功能，应用无需处理加解密操作。

**关键点：**
- Gramine 在 manifest 中配置加密分区路径
- 应用只需使用标准文件 I/O（os.ReadFile, os.WriteFile 等）
- Gramine 在底层自动加密/解密，对应用完全透明
- 应用无需管理密钥、无需调用加密 API

```go
// storage/encrypted_partition_impl.go
package storage

import (
    "fmt"
    "os"
    "path/filepath"
    "sync"
)

type GramineEncryptedPartition struct {
    mu       sync.RWMutex
    basePath string
    // 无需 key 字段 - Gramine 透明处理所有加密
}

// NewEncryptedPartition 创建加密分区管理器
func NewGramineEncryptedPartition(basePath string) (*GramineEncryptedPartition, error) {
    if _, err := os.Stat(basePath); os.IsNotExist(err) {
        return nil, fmt.Errorf("path does not exist: %s", basePath)
    }
    return &GramineEncryptedPartition{basePath: basePath}, nil
}

func (gep *GramineEncryptedPartition) WriteSecret(id string, data []byte) error {
    path := filepath.Join(gep.basePath, id)
    return os.WriteFile(path, data, 0600)  // Gramine 自动加密
}

func (gep *GramineEncryptedPartition) ReadSecret(id string) ([]byte, error) {
    path := filepath.Join(gep.basePath, id)
    return os.ReadFile(path)  // Gramine 自动解密
}
```

## 8. 安全检查清单

**部署前检查**
- [ ] Manifest 中合约地址正确配置
- [ ] 加密分区路径已配置
- [ ] SecurityConfigContract 地址与 ARCHITECTURE.md 一致
- [ ] 参数合并逻辑优先级正确（Manifest > 链上 > 命令行）

**运行时检查**
- [ ] 秘密数据同步前验证 MRENCLAVE 白名单
- [ ] RA-TLS 连接建立成功
- [ ] PermissionLevel 正确限制迁移频率
- [ ] 所有密钥操作使用常量时间实现

**测试覆盖**
- [ ] 参数合并测试（三类参数）
- [ ] MRENCLAVE 白名单验证测试
- [ ] 秘密数据同步端到端测试
- [ ] AutoMigrationManager 权限级别测试
- [ ] 侧信道攻击防护测试

## 9. 与 ARCHITECTURE.md 的对应关系

本模块实现了 ARCHITECTURE.md 第 5 章"数据存储与同步"的内容，提供详细的实现指导。

**主要扩展**
- AutoMigrationManager 实现细节（ARCHITECTURE.md 定义了接口）
- PermissionLevel 机制的完整实现
- 参数处理的三层合并逻辑（Manifest、链上、命令行）
- 侧信道攻击防护的具体代码实现

**保持一致**
- 所有接口定义与 ARCHITECTURE.md 对齐
- PermissionLevel 常量值与 ARCHITECTURE.md 相同
- 合约地址配置方式与 ARCHITECTURE.md 一致
- Gramine 透明加密的使用方式与 ARCHITECTURE.md 一致
