# 数据存储与同步模块开发文档

## 模块概述

数据存储与同步模块负责 X Chain 的数据持久化和节点间秘密数据同步。该模块管理加密分区存储、秘密数据的安全传输、以及节点间的数据一致性。

## 负责团队

**存储/基础设施团队**

## 模块职责

1. 加密分区管理
2. 秘密数据存储（私钥、敏感配置）
3. 节点间秘密数据同步
4. 数据一致性保证
5. 参数校验机制
6. 侧信道攻击防护

## 依赖关系

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

### 上游依赖
- SGX 证明模块（RA-TLS 安全通道、双向度量值验证）
- 治理模块（通过 SecurityConfigContract 获取 MRENCLAVE 白名单、PermissionLevel、迁移策略）
- Gramine LibOS（加密文件系统、密钥封装/解封）
- 共识引擎（读取当前区块高度、UpgradeCompleteBlock 参数）

### 下游依赖（被以下模块使用）
- 预编译合约模块（密钥存储、ECDH 秘密存储）
- 共识引擎模块（状态持久化、区块数据存储）
- 治理模块（通过加密分区存储投票记录）

### 与治理模块的交互

数据存储模块通过以下方式与治理模块交互：

1. **MRENCLAVE 白名单验证**：
   - 秘密数据同步前，必须验证对端节点的 MRENCLAVE 在白名单中
   - 白名单从 SecurityConfigContract 动态读取
   - 治理投票可以添加或移除 MRENCLAVE

2. **权限级别检查**：
   - 新添加的 MRENCLAVE 具有渐进式权限：
     - Basic (7 天)：每日最多 10 次迁移
     - Standard (30 天)：每日最多 100 次迁移
     - Full (永久)：无限制迁移
   - AutoMigrationManager 根据 PermissionLevel 限制迁移频率

3. **升级协调**：
   - 治理设置 `UpgradeCompleteBlock` 参数在 SecurityConfigContract 中
   - AutoMigrationManager 确保在该区块高度前完成秘密数据迁移
   - 迁移完成条件：`secretDataSyncedBlock >= UpgradeCompleteBlock`

### 秘密数据同步触发机制

秘密数据同步由以下事件触发：

1. **新节点加入**：
   - 新节点首次启动时，检测到本地加密分区为空
   - 从现有节点请求秘密数据同步
   - 触发条件：`localSecretDataVersion == 0`

2. **MRENCLAVE 白名单更新**（自动触发）：
   - 治理投票添加新 MRENCLAVE 到白名单后
   - AutoMigrationManager 自动检测白名单变化
   - 新版本节点开始同步秘密数据
   - 触发条件：`newMREnclave ∈ whitelist AND permissionLevel >= Basic`

3. **升级期间协调**：
   - 升级进行中（白名单包含多个 MRENCLAVE）
   - AutoMigrationManager 根据 `UpgradeCompleteBlock` 调度迁移
   - 新版本节点在该区块高度前必须完成迁移
   - 触发条件：`currentBlock < UpgradeCompleteBlock AND !migrationComplete`

## 参数分类与校验

### 参数分类原则

X Chain 的配置参数分为两类：

| 类别 | 控制方式 | 特点 | 示例 |
|------|----------|------|------|
| **Manifest 固定参数** | Gramine Manifest | 影响度量值，不可外部修改 | 本地路径配置、链上合约地址 |
| **链上安全参数** | 链上合约 | 通过投票管理，动态生效 | 白名单、密钥迁移阈值、准入策略 |
| **非安全参数** | 命令行参数 | 不影响安全性，可灵活配置 | 出块间隔、RPC 端口、日志级别 |

### Manifest 固定参数

Manifest 中只存储本地配置和链上合约地址。合约地址写死在 manifest 中，作为安全锚点，确保节点只能从指定的合约读取安全参数。

```toml
# gramine manifest 中的固定参数
[loader.env]
# 本地路径配置
XCHAIN_ENCRYPTED_PATH = "/data/encrypted"    # 加密分区路径
XCHAIN_SECRET_PATH = "/data/secrets"         # 秘密数据存储路径

# 链上合约地址（写死，作为安全锚点）
# 合约地址影响 MRENCLAVE，攻击者无法修改合约地址而不改变度量值
XCHAIN_GOVERNANCE_CONTRACT = "0x1234567890abcdef1234567890abcdef12345678"
XCHAIN_SECURITY_CONFIG_CONTRACT = "0xabcdef1234567890abcdef1234567890abcdef12"
```

### 链上安全参数

所有治理相关的安全参数从链上合约动态读取，这样投票结果可以实时生效，无需重新部署节点：

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

**合约职责划分**：
- **安全配置合约（SecurityConfigContract）**：存储所有安全配置，被其他模块读取
- **治理合约（GovernanceContract）**：负责投票、管理投票人（有效性、合法性）、把投票结果写入安全配置合约

```go
// 从链上读取安全参数
type OnChainConfigSync struct {
    governanceContract common.Address  // 从 Manifest 读取
    whitelistContract  common.Address  // 从 Manifest 读取
    client             *ethclient.Client
}

func NewOnChainConfigSync() (*OnChainConfigSync, error) {
    // 从 Manifest 环境变量读取合约地址
    govAddr := os.Getenv("XCHAIN_GOVERNANCE_CONTRACT")
    scAddr := os.Getenv("XCHAIN_SECURITY_CONFIG_CONTRACT")
    
    return &OnChainConfigSync{
        governanceContract:     common.HexToAddress(govAddr),
        securityConfigContract: common.HexToAddress(scAddr), // 安全配置合约，由治理合约管理
    }, nil
}

// SyncSecurityParams 从链上同步所有安全参数
func (s *OnChainConfigSync) SyncSecurityParams() (*SecurityConfig, error) {
    config := &SecurityConfig{}
    
    // 从安全配置合约读取（由治理合约管理）
    config.AllowedMREnclave = s.fetchWhitelist()
    
    // 从治理合约读取
    config.KeyMigrationThreshold = s.fetchKeyMigrationThreshold()
    config.AdmissionStrict = s.fetchAdmissionPolicy()
    config.VotingThreshold = s.fetchVotingThreshold()
    
    return config, nil
}
```

**安全保证**：
- 合约地址写死在 Manifest 中，影响 MRENCLAVE，无法被篡改
- 所有安全参数从链上读取，通过共识机制保证一致性
- 投票结果记录在链上，不可篡改
- 节点定期从链上同步参数，确保使用最新的治理决策

### 非安全参数（命令行控制）

```bash
# 可通过命令行灵活配置的参数
./geth \
    --xchain.block.interval=15 \
    --xchain.block.max-tx=1000 \
    --xchain.block.max-gas=30000000 \
    --xchain.rpc.port=8545 \
    --xchain.p2p.port=30303 \
    --xchain.log.level=info \
    --xchain.metrics.enabled=true
```

### 参数校验机制

参数处理流程：

1. **启动后首先读取 Manifest 参数**：从环境变量加载所有安全相关参数
2. **读取用户命令行参数**：解析用户传入的命令行参数
3. **合并参数**：
   - Manifest 参数覆盖用户参数（安全参数以 Manifest 为准）
   - 如果用户参数与 Manifest 不一致，提示并退出进程
   - 非安全参数允许用户通过命令行添加

```go
// config/param_validator.go
package config

import (
    "encoding/base64"
    "errors"
    "fmt"
    "os"
    "strings"
)

// ParamCategory 参数类别
type ParamCategory uint8

const (
    ParamCategorySecurity ParamCategory = 0x01 // 安全相关
    ParamCategoryRuntime  ParamCategory = 0x02 // 运行时配置
)

// ParamDefinition 参数定义
type ParamDefinition struct {
    Name        string
    Category    ParamCategory
    EnvKey      string        // Manifest 环境变量名
    CliFlag     string        // 对应的命令行参数名
    Required    bool
    Default     string
    Validator   func(string) error
}

// SecurityParams 安全相关参数定义
// 注意：白名单不在此列表中，因为白名单应从链上动态读取，而不是从环境变量
var SecurityParams = []ParamDefinition{
    {
        Name:     "encrypted_path",
        Category: ParamCategorySecurity,
        EnvKey:   "XCHAIN_ENCRYPTED_PATH",
        CliFlag:  "xchain.encrypted-path",
        Required: true,
        Validator: func(v string) error {
            if v == "" {
                return errors.New("encrypted path cannot be empty")
            }
            return nil
        },
    },
    {
        Name:     "secret_path",
        Category: ParamCategorySecurity,
        EnvKey:   "XCHAIN_SECRET_PATH",
        CliFlag:  "xchain.secret-path",
        Required: true,
    },
    {
        Name:     "key_migration_enabled",
        Category: ParamCategorySecurity,
        EnvKey:   "XCHAIN_KEY_MIGRATION_ENABLED",
        CliFlag:  "xchain.key-migration",
        Required: false,
        Default:  "false",
    },
    {
        Name:     "admission_strict",
        Category: ParamCategorySecurity,
        EnvKey:   "XCHAIN_ADMISSION_STRICT",
        CliFlag:  "xchain.admission-strict",
        Required: false,
        Default:  "true",
    },
}

// ParamValidator 参数校验器
type ParamValidator struct {
    manifestParams map[string]string
    cliParams      map[string]string
    mergedParams   map[string]string
}

// NewParamValidator 创建参数校验器
func NewParamValidator() *ParamValidator {
    return &ParamValidator{
        manifestParams: make(map[string]string),
        cliParams:      make(map[string]string),
        mergedParams:   make(map[string]string),
    }
}

// LoadManifestParams 从环境变量加载 Manifest 参数（步骤 1）
func (pv *ParamValidator) LoadManifestParams() error {
    for _, param := range SecurityParams {
        value := os.Getenv(param.EnvKey)
        
        if value == "" && param.Required {
            return fmt.Errorf("required security parameter %s not set in manifest", param.Name)
        }
        
        if value == "" {
            value = param.Default
        }
        
        // 执行验证器
        if param.Validator != nil && value != "" {
            if err := param.Validator(value); err != nil {
                return fmt.Errorf("invalid value for %s: %w", param.Name, err)
            }
        }
        
        pv.manifestParams[param.Name] = value
    }
    
    return nil
}

// LoadCliParams 加载命令行参数（步骤 2）
func (pv *ParamValidator) LoadCliParams(args []string) error {
    for _, arg := range args {
        if !strings.HasPrefix(arg, "--xchain.") {
            continue
        }
        
        parts := strings.SplitN(strings.TrimPrefix(arg, "--"), "=", 2)
        if len(parts) != 2 {
            continue
        }
        
        pv.cliParams[parts[0]] = parts[1]
    }
    return nil
}

// MergeAndValidate 合并参数并校验（步骤 3）
// 返回错误时应退出进程
func (pv *ParamValidator) MergeAndValidate() error {
    // 首先将所有 Manifest 参数复制到合并结果
    for name, value := range pv.manifestParams {
        pv.mergedParams[name] = value
    }
    
    // 检查命令行参数
    for cliFlag, cliValue := range pv.cliParams {
        // 检查是否是安全相关参数
        for _, param := range SecurityParams {
            if param.CliFlag == cliFlag {
                // 安全参数：检查是否与 Manifest 一致
                manifestValue, ok := pv.manifestParams[param.Name]
                if ok && cliValue != manifestValue {
                    return fmt.Errorf(
                        "SECURITY VIOLATION: CLI parameter --%s value '%s' conflicts with manifest value '%s'. "+
                        "Security parameters must match manifest. Exiting.",
                        cliFlag, cliValue, manifestValue,
                    )
                }
                // 一致则跳过（已从 Manifest 复制）
                goto nextParam
            }
        }
        
        // 非安全参数：允许添加到合并结果
        pv.mergedParams[cliFlag] = cliValue
        
    nextParam:
    }
    
    return nil
}

// GetParam 获取合并后的参数
func (pv *ParamValidator) GetParam(name string) (string, bool) {
    value, ok := pv.mergedParams[name]
    return value, ok
}

// GetSecurityParam 获取安全参数（只从 Manifest 读取）
func (pv *ParamValidator) GetSecurityParam(name string) (string, error) {
    value, ok := pv.manifestParams[name]
    if !ok {
        return "", fmt.Errorf("security parameter %s not found", name)
    }
    return value, nil
}

// GetRuntimeParam 获取运行时参数
func (pv *ParamValidator) GetRuntimeParam(name string) string {
    if value, ok := pv.mergedParams[name]; ok {
        return value
    }
    return ""
}
```

### 启动时参数校验

启动流程：
1. 启动后首先读取 Manifest 中指定的安全参数
2. 读取用户传入的命令行参数
3. 合并参数：Manifest 参数覆盖用户参数，不一致则提示并退出

```go
// cmd/geth/main.go (修改)
package main

import (
    "fmt"
    "log"
    "os"
    
    "github.com/ethereum/go-ethereum/config"
)

func initializeParams() (*config.ParamValidator, error) {
    validator := config.NewParamValidator()
    
    // 步骤 1: 启动后首先读取 Manifest 参数
    log.Println("Loading security parameters from manifest...")
    if err := validator.LoadManifestParams(); err != nil {
        return nil, fmt.Errorf("failed to load manifest params: %w", err)
    }
    log.Println("Manifest parameters loaded successfully")
    
    // 步骤 2: 读取用户命令行参数
    log.Println("Loading CLI parameters...")
    if err := validator.LoadCliParams(os.Args[1:]); err != nil {
        return nil, fmt.Errorf("failed to load CLI params: %w", err)
    }
    
    // 步骤 3: 合并参数并校验
    // - Manifest 参数覆盖用户参数
    // - 如果用户参数与 Manifest 不一致，提示并退出
    log.Println("Merging and validating parameters...")
    if err := validator.MergeAndValidate(); err != nil {
        // 安全违规，立即退出进程
        log.Printf("FATAL SECURITY VIOLATION: %v", err)
        os.Exit(1)
    }
    log.Println("Parameter validation successful")
    
    return validator, nil
}

func main() {
    // 参数初始化和校验（失败则退出）
    validator, err := initializeParams()
    if err != nil {
        log.Fatalf("Parameter initialization failed: %v", err)
    }
    
    // 使用合并后的参数继续启动
    encryptedPath, _ := validator.GetSecurityParam("encrypted_path")
    log.Printf("Using encrypted path: %s", encryptedPath)
    
    blockInterval := validator.GetRuntimeParam("xchain.block.interval")
    if blockInterval == "" {
        blockInterval = "15" // 默认值
    }
    log.Printf("Block interval: %s seconds", blockInterval)
    
    // 继续正常启动...
}
```

## 核心数据结构

### 存储配置

```go
// storage/config.go
package storage

// StorageConfig 存储配置
type StorageConfig struct {
    // 加密分区路径（安全参数，由 Manifest 控制）
    EncryptedPath string
    
    // 普通数据路径
    DataPath string
    
    // 秘密数据路径（安全参数）
    SecretPath string
    
    // 缓存大小（运行时参数）
    CacheSize int
    
    // 同步间隔（运行时参数）
    SyncInterval int
}

// SecretDataType 秘密数据类型
type SecretDataType uint8

const (
    SecretTypePrivateKey   SecretDataType = 0x01 // 私钥
    SecretTypeSealingKey   SecretDataType = 0x02 // 密封密钥
    SecretTypeNodeIdentity SecretDataType = 0x03 // 节点身份
    SecretTypeSharedSecret SecretDataType = 0x04 // 共享密钥
)

// SecretData 秘密数据
type SecretData struct {
    Type      SecretDataType
    ID        []byte
    Data      []byte
    CreatedAt uint64
    ExpiresAt uint64
    Metadata  map[string]string
}
```

### 加密分区管理

```go
// storage/encrypted_partition.go
package storage

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "errors"
    "io"
    "os"
    "path/filepath"
    "sync"
)

// EncryptedPartition 加密分区管理器
type EncryptedPartition struct {
    mu       sync.RWMutex
    basePath string
    key      []byte // Gramine 提供的密封密钥
}

// NewEncryptedPartition 创建加密分区管理器
// 注意：basePath 必须是 Manifest 中配置的加密分区路径
func NewEncryptedPartition(basePath string, sealingKey []byte) (*EncryptedPartition, error) {
    // 验证路径存在
    if _, err := os.Stat(basePath); os.IsNotExist(err) {
        return nil, fmt.Errorf("encrypted partition path does not exist: %s", basePath)
    }
    
    return &EncryptedPartition{
        basePath: basePath,
        key:      sealingKey,
    }, nil
}

// WriteSecret 写入秘密数据
// 私钥必须存储在加密分区
func (ep *EncryptedPartition) WriteSecret(id string, data []byte) error {
    ep.mu.Lock()
    defer ep.mu.Unlock()
    
    filePath := filepath.Join(ep.basePath, id)
    
    // 使用 O_CREAT | O_TRUNC 标志
    // 文件不存在则创建，存在则覆盖
    file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
    if err != nil {
        return fmt.Errorf("failed to open file: %w", err)
    }
    defer file.Close()
    
    // Gramine 会自动加密写入加密分区的数据
    if _, err := file.Write(data); err != nil {
        return fmt.Errorf("failed to write data: %w", err)
    }
    
    return nil
}

// ReadSecret 读取秘密数据
func (ep *EncryptedPartition) ReadSecret(id string) ([]byte, error) {
    ep.mu.RLock()
    defer ep.mu.RUnlock()
    
    filePath := filepath.Join(ep.basePath, id)
    
    // Gramine 会自动解密从加密分区读取的数据
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

## 秘密数据同步

### 同步协议

```go
// storage/sync_protocol.go
package storage

import (
    "github.com/ethereum/go-ethereum/common"
)

// SyncMessageType 同步消息类型
type SyncMessageType uint8

const (
    SyncMsgRequest    SyncMessageType = 0x01 // 请求同步
    SyncMsgResponse   SyncMessageType = 0x02 // 同步响应
    SyncMsgAck        SyncMessageType = 0x03 // 确认
    SyncMsgReject     SyncMessageType = 0x04 // 拒绝
    SyncMsgHeartbeat  SyncMessageType = 0x05 // 心跳
)

// SyncRequest 同步请求
type SyncRequest struct {
    RequestID   common.Hash
    RequesterID common.Hash    // 请求者节点 ID
    MRENCLAVE   [32]byte       // 请求者 MRENCLAVE
    Quote       []byte         // SGX Quote
    SecretTypes []SecretDataType // 请求的秘密类型
    Timestamp   uint64
    Signature   []byte
}

// SyncResponse 同步响应
type SyncResponse struct {
    RequestID    common.Hash
    ResponderID  common.Hash
    MRENCLAVE    [32]byte
    Quote        []byte
    Secrets      []*EncryptedSecret // 加密的秘密数据
    Timestamp    uint64
    Signature    []byte
}

// EncryptedSecret 加密的秘密数据
type EncryptedSecret struct {
    Type       SecretDataType
    ID         []byte
    Ciphertext []byte // 使用 ECDH 共享密钥加密
    Nonce      []byte
    Tag        []byte
}
```

### 同步管理器

```go
// storage/sync_manager.go
package storage

import (
    "context"
    "errors"
    "sync"
    "time"
    
    "github.com/ethereum/go-ethereum/common"
)

// SyncConfig 同步配置
type SyncConfig struct {
    // 同步超时（运行时参数）
    SyncTimeout time.Duration
    
    // 最大重试次数（运行时参数）
    MaxRetries int
    
    // 心跳间隔（运行时参数）
    HeartbeatInterval time.Duration
    
    // 度量值验证（安全参数，由 Manifest 控制）
    VerifyMREnclave bool
    
    // 允许的 MRENCLAVE 列表（安全参数）
    AllowedMREnclaves [][32]byte
}

// SyncManager 同步管理器
type SyncManager struct {
    config     *SyncConfig
    mu         sync.RWMutex
    partition  *EncryptedPartition
    verifier   SGXVerifier
    transport  SecureTransport
    peers      map[common.Hash]*PeerInfo
}

// PeerInfo 对等节点信息
type PeerInfo struct {
    NodeID        common.Hash
    MRENCLAVE     [32]byte
    LastSeen      time.Time
    SyncStatus    SyncStatus
    SharedSecret  []byte // ECDH 共享密钥
}

// SyncStatus 同步状态
type SyncStatus uint8

const (
    SyncStatusIdle       SyncStatus = 0x00
    SyncStatusRequesting SyncStatus = 0x01
    SyncStatusSyncing    SyncStatus = 0x02
    SyncStatusComplete   SyncStatus = 0x03
    SyncStatusFailed     SyncStatus = 0x04
)

// SecureTransport 安全传输接口（RA-TLS）
type SecureTransport interface {
    Send(peerID common.Hash, data []byte) error
    Receive() (common.Hash, []byte, error)
    EstablishChannel(peerID common.Hash, quote []byte) ([]byte, error) // 返回共享密钥
}

// SGXVerifier SGX 验证器接口
type SGXVerifier interface {
    VerifyQuote(quote []byte) error
    ExtractMREnclave(quote []byte) ([32]byte, error)
}

// NewSyncManager 创建同步管理器
func NewSyncManager(
    config *SyncConfig,
    partition *EncryptedPartition,
    verifier SGXVerifier,
    transport SecureTransport,
) *SyncManager {
    return &SyncManager{
        config:    config,
        partition: partition,
        verifier:  verifier,
        transport: transport,
        peers:     make(map[common.Hash]*PeerInfo),
    }
}

// RequestSync 请求同步秘密数据
func (sm *SyncManager) RequestSync(ctx context.Context, peerID common.Hash, secretTypes []SecretDataType) error {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    
    // 1. 获取对等节点信息
    peer, ok := sm.peers[peerID]
    if !ok {
        return errors.New("peer not found")
    }
    
    // 2. 验证对等节点的 MRENCLAVE
    if sm.config.VerifyMREnclave {
        if !sm.isAllowedMREnclave(peer.MRENCLAVE) {
            return errors.New("peer MRENCLAVE not in whitelist")
        }
    }
    
    // 3. 建立安全通道（RA-TLS）
    // 这会进行双向 SGX 证明
    sharedSecret, err := sm.transport.EstablishChannel(peerID, nil)
    if err != nil {
        return fmt.Errorf("failed to establish secure channel: %w", err)
    }
    peer.SharedSecret = sharedSecret
    
    // 4. 发送同步请求
    request := &SyncRequest{
        RequestID:   common.Hash{}, // 生成唯一 ID
        SecretTypes: secretTypes,
        Timestamp:   uint64(time.Now().Unix()),
    }
    
    // 序列化并发送
    // ...
    
    peer.SyncStatus = SyncStatusRequesting
    
    return nil
}

// HandleSyncRequest 处理同步请求
func (sm *SyncManager) HandleSyncRequest(request *SyncRequest) (*SyncResponse, error) {
    // 1. 验证请求者的 SGX Quote
    if err := sm.verifier.VerifyQuote(request.Quote); err != nil {
        return nil, fmt.Errorf("invalid quote: %w", err)
    }
    
    // 2. 提取并验证 MRENCLAVE
    mrenclave, err := sm.verifier.ExtractMREnclave(request.Quote)
    if err != nil {
        return nil, err
    }
    
    if sm.config.VerifyMREnclave && !sm.isAllowedMREnclave(mrenclave) {
        return nil, errors.New("requester MRENCLAVE not in whitelist")
    }
    
    // 3. 获取共享密钥
    peer, ok := sm.peers[request.RequesterID]
    if !ok || peer.SharedSecret == nil {
        return nil, errors.New("no shared secret with peer")
    }
    
    // 4. 读取并加密秘密数据
    secrets := make([]*EncryptedSecret, 0)
    for _, secretType := range request.SecretTypes {
        secretIDs, err := sm.getSecretIDsByType(secretType)
        if err != nil {
            continue
        }
        
        for _, id := range secretIDs {
            data, err := sm.partition.ReadSecret(id)
            if err != nil {
                continue
            }
            
            // 使用共享密钥加密
            encrypted, err := sm.encryptWithSharedSecret(peer.SharedSecret, data)
            if err != nil {
                continue
            }
            
            secrets = append(secrets, &EncryptedSecret{
                Type:       secretType,
                ID:         []byte(id),
                Ciphertext: encrypted.Ciphertext,
                Nonce:      encrypted.Nonce,
                Tag:        encrypted.Tag,
            })
        }
    }
    
    return &SyncResponse{
        RequestID: request.RequestID,
        Secrets:   secrets,
        Timestamp: uint64(time.Now().Unix()),
    }, nil
}

// HandleSyncResponse 处理同步响应
func (sm *SyncManager) HandleSyncResponse(response *SyncResponse) error {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    
    // 1. 验证响应者的 Quote
    if err := sm.verifier.VerifyQuote(response.Quote); err != nil {
        return err
    }
    
    // 2. 获取共享密钥
    peer, ok := sm.peers[response.ResponderID]
    if !ok || peer.SharedSecret == nil {
        return errors.New("no shared secret with peer")
    }
    
    // 3. 解密并存储秘密数据
    for _, secret := range response.Secrets {
        plaintext, err := sm.decryptWithSharedSecret(peer.SharedSecret, secret)
        if err != nil {
            continue
        }
        
        // 存储到加密分区
        if err := sm.partition.WriteSecret(string(secret.ID), plaintext); err != nil {
            continue
        }
    }
    
    peer.SyncStatus = SyncStatusComplete
    
    return nil
}

// isAllowedMREnclave 检查 MRENCLAVE 是否在白名单中
func (sm *SyncManager) isAllowedMREnclave(mrenclave [32]byte) bool {
    for _, allowed := range sm.config.AllowedMREnclaves {
        if allowed == mrenclave {
            return true
        }
    }
    return false
}

// getSecretIDsByType 根据类型获取秘密 ID 列表
func (sm *SyncManager) getSecretIDsByType(secretType SecretDataType) ([]string, error) {
    // 实现根据类型过滤
    return sm.partition.ListSecrets()
}

// encryptWithSharedSecret 使用共享密钥加密
func (sm *SyncManager) encryptWithSharedSecret(sharedSecret, plaintext []byte) (*EncryptedSecret, error) {
    block, err := aes.NewCipher(sharedSecret[:32])
    if err != nil {
        return nil, err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }
    
    ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
    
    return &EncryptedSecret{
        Ciphertext: ciphertext[:len(ciphertext)-gcm.Overhead()],
        Nonce:      nonce,
        Tag:        ciphertext[len(ciphertext)-gcm.Overhead():],
    }, nil
}

// decryptWithSharedSecret 使用共享密钥解密
func (sm *SyncManager) decryptWithSharedSecret(sharedSecret []byte, encrypted *EncryptedSecret) ([]byte, error) {
    block, err := aes.NewCipher(sharedSecret[:32])
    if err != nil {
        return nil, err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    ciphertext := append(encrypted.Ciphertext, encrypted.Tag...)
    
    return gcm.Open(nil, encrypted.Nonce, ciphertext, nil)
}
```

## 侧信道攻击防护

### 常量时间操作

```go
// storage/constant_time.go
package storage

import (
    "crypto/subtle"
)

// ConstantTimeCompare 常量时间比较
func ConstantTimeCompare(a, b []byte) bool {
    return subtle.ConstantTimeCompare(a, b) == 1
}

// ConstantTimeSelect 常量时间选择
func ConstantTimeSelect(condition int, a, b []byte) []byte {
    result := make([]byte, len(a))
    for i := range result {
        result[i] = byte(subtle.ConstantTimeSelect(condition, int(a[i]), int(b[i])))
    }
    return result
}

// ConstantTimeCopy 常量时间复制
func ConstantTimeCopy(condition int, dst, src []byte) {
    subtle.ConstantTimeCopy(condition, dst, src)
}
```

### 内存安全

```go
// storage/memory_safety.go
package storage

import (
    "runtime"
    "unsafe"
)

// SecureBuffer 安全缓冲区
type SecureBuffer struct {
    data []byte
}

// NewSecureBuffer 创建安全缓冲区
func NewSecureBuffer(size int) *SecureBuffer {
    return &SecureBuffer{
        data: make([]byte, size),
    }
}

// Write 写入数据
func (sb *SecureBuffer) Write(data []byte) {
    copy(sb.data, data)
}

// Read 读取数据
func (sb *SecureBuffer) Read() []byte {
    result := make([]byte, len(sb.data))
    copy(result, sb.data)
    return result
}

// Clear 安全清除
func (sb *SecureBuffer) Clear() {
    for i := range sb.data {
        sb.data[i] = 0
    }
    runtime.KeepAlive(sb.data)
}

// Destroy 销毁缓冲区
func (sb *SecureBuffer) Destroy() {
    sb.Clear()
    sb.data = nil
}
```

## 文件结构

```
storage/
├── config.go                 # 存储配置
├── param_validator.go        # 参数校验
├── encrypted_partition.go    # 加密分区管理
├── sync_protocol.go          # 同步协议
├── sync_manager.go           # 同步管理器
├── constant_time.go          # 常量时间操作
├── memory_safety.go          # 内存安全
└── storage_test.go           # 测试
```

## 单元测试指南

### 参数校验测试

```go
// config/param_validator_test.go
package config

import (
    "os"
    "testing"
)

func TestSecurityParamValidation(t *testing.T) {
    // 设置 Manifest 参数
    os.Setenv("XCHAIN_MRENCLAVE_WHITELIST", `["abc123"]`)
    os.Setenv("XCHAIN_ENCRYPTED_PATH", "/data/encrypted")
    os.Setenv("XCHAIN_SECRET_PATH", "/data/secrets")
    defer func() {
        os.Unsetenv("XCHAIN_MRENCLAVE_WHITELIST")
        os.Unsetenv("XCHAIN_ENCRYPTED_PATH")
        os.Unsetenv("XCHAIN_SECRET_PATH")
    }()
    
    validator := NewParamValidator()
    
    // 加载 Manifest 参数
    if err := validator.LoadManifestParams(); err != nil {
        t.Fatalf("LoadManifestParams failed: %v", err)
    }
    
    // 测试匹配的参数
    err := validator.ValidateRuntimeParam("encrypted_path", "/data/encrypted")
    if err != nil {
        t.Errorf("Should accept matching parameter: %v", err)
    }
    
    // 测试不匹配的参数（应该失败）
    err = validator.ValidateRuntimeParam("encrypted_path", "/other/path")
    if err == nil {
        t.Error("Should reject mismatched security parameter")
    }
}

func TestRuntimeParamAllowed(t *testing.T) {
    validator := NewParamValidator()
    
    // 非安全参数应该允许
    err := validator.ValidateRuntimeParam("block_interval", "15")
    if err != nil {
        t.Errorf("Should allow runtime parameter: %v", err)
    }
}
```

### 加密分区测试

```go
// storage/encrypted_partition_test.go
package storage

import (
    "bytes"
    "os"
    "testing"
)

func TestEncryptedPartition(t *testing.T) {
    // 创建临时目录
    tmpDir, err := os.MkdirTemp("", "encrypted_test")
    if err != nil {
        t.Fatal(err)
    }
    defer os.RemoveAll(tmpDir)
    
    // 创建加密分区
    key := make([]byte, 32)
    partition, err := NewEncryptedPartition(tmpDir, key)
    if err != nil {
        t.Fatalf("NewEncryptedPartition failed: %v", err)
    }
    
    // 写入秘密
    secretID := "test_secret"
    secretData := []byte("sensitive data")
    
    if err := partition.WriteSecret(secretID, secretData); err != nil {
        t.Fatalf("WriteSecret failed: %v", err)
    }
    
    // 读取秘密
    readData, err := partition.ReadSecret(secretID)
    if err != nil {
        t.Fatalf("ReadSecret failed: %v", err)
    }
    
    if !bytes.Equal(readData, secretData) {
        t.Error("Read data does not match written data")
    }
    
    // 删除秘密
    if err := partition.DeleteSecret(secretID); err != nil {
        t.Fatalf("DeleteSecret failed: %v", err)
    }
    
    // 确认已删除
    _, err = partition.ReadSecret(secretID)
    if err == nil {
        t.Error("Secret should be deleted")
    }
}
```

### 同步测试

```go
// storage/sync_manager_test.go
package storage

import (
    "testing"
    
    "github.com/ethereum/go-ethereum/common"
)

func TestSyncMREnclaveValidation(t *testing.T) {
    allowedMREnclave := [32]byte{1, 2, 3}
    notAllowedMREnclave := [32]byte{4, 5, 6}
    
    config := &SyncConfig{
        VerifyMREnclave:   true,
        AllowedMREnclaves: [][32]byte{allowedMREnclave},
    }
    
    manager := &SyncManager{config: config}
    
    // 测试允许的 MRENCLAVE
    if !manager.isAllowedMREnclave(allowedMREnclave) {
        t.Error("Should allow whitelisted MRENCLAVE")
    }
    
    // 测试不允许的 MRENCLAVE
    if manager.isAllowedMREnclave(notAllowedMREnclave) {
        t.Error("Should reject non-whitelisted MRENCLAVE")
    }
}
```

## 配置参数

### 安全参数（Manifest 控制）

```toml
# gramine manifest
[loader.env]
XCHAIN_MRENCLAVE_WHITELIST = '["abc123...", "def456..."]'
XCHAIN_ENCRYPTED_PATH = "/data/encrypted"
XCHAIN_SECRET_PATH = "/data/secrets"
XCHAIN_KEY_MIGRATION_ENABLED = "true"
XCHAIN_KEY_MIGRATION_THRESHOLD = "2"
XCHAIN_ADMISSION_STRICT = "true"
```

### 运行时参数（命令行控制）

```toml
# config.toml
[storage]
# 缓存大小（MB）
cache_size = 256

# 同步间隔（秒）
sync_interval = 60

# 心跳间隔（秒）
heartbeat_interval = 30

# 同步超时（秒）
sync_timeout = 300

# 最大重试次数
max_retries = 3
```

## 实现优先级

| 优先级 | 功能 | 预计工时 |
|--------|------|----------|
| P0 | 参数校验机制 | 2 天 |
| P0 | 加密分区管理 | 3 天 |
| P0 | 秘密数据存储 | 2 天 |
| P1 | 同步协议 | 3 天 |
| P1 | 同步管理器 | 4 天 |
| P2 | 侧信道防护 | 2 天 |
| P2 | 内存安全 | 2 天 |

**总计：约 2.5 周**

## 硬分叉数据迁移

### 迁移背景

硬分叉升级时，**非加密分区的数据直接复用**，不需要在不同版本的节点间同步。**唯一需要从旧节点迁移到新节点的只有秘密数据**（加密分区中的私钥等）。

由于 SGX sealing 使用 MRENCLAVE 作为密钥派生因子，新版本代码的 MRENCLAVE 不同，无法直接解密旧版本封装的秘密数据，因此需要通过 RA-TLS 安全通道从旧节点迁移秘密数据。

### 数据分类

| 数据类型 | 存储位置 | 迁移策略 |
|----------|----------|----------|
| 区块链状态 | LevelDB | **直接复用**，无需迁移 |
| 账户余额 | StateDB | **直接复用**，无需迁移 |
| 合约存储 | StateDB | **直接复用**，无需迁移 |
| 交易历史 | LevelDB | **直接复用**，无需迁移 |
| 私钥数据 | 加密分区 | **需要迁移** (Re-sealing) |
| 密钥元数据 | 加密分区 | **需要迁移** |
| 派生秘密 | 加密分区 | **需要迁移** |

**重要说明**：
- 非加密分区的数据（区块链状态、账户余额、合约存储等）是公开的，新节点可以直接读取旧节点的数据目录
- 只有加密分区中的秘密数据需要通过 RA-TLS 安全通道从旧节点迁移到新节点

### 迁移流程

```
硬分叉升级流程：

1. 非加密数据（直接复用）：
   ┌─────────────────────────────────────────────────────────────┐
   │  旧数据目录                      新节点                      │
   │  /data/chaindata/  ─────────────> 直接读取                  │
   │  (区块、状态、交易)               无需迁移                   │
   └─────────────────────────────────────────────────────────────┘

2. 秘密数据（需要迁移）：
   ┌─────────────────────────────────────────────────────────────┐
   │  旧版本节点                       新版本节点                 │
   │  MRENCLAVE: ABC                   MRENCLAVE: DEF            │
   │       │                                 │                    │
   │       │  1. 解封秘密数据                │                    │
   │       │  (使用 MRENCLAVE=ABC)           │                    │
   │       │                                 │                    │
   │       │  2. RA-TLS 安全通道传输         │                    │
   │       │────────────────────────────────>│                    │
   │       │                                 │                    │
   │       │                   3. 重新封装   │                    │
   │       │                   (MRENCLAVE=DEF)                    │
   └─────────────────────────────────────────────────────────────┘
```

### 迁移实现

```go
// internal/sgx/migration.go
package sgx

import (
    "context"
    "fmt"
    
    "github.com/ethereum/go-ethereum/common"
)

// DataMigrator 处理硬分叉时的数据迁移
type DataMigrator struct {
    oldEnclave *EnclaveConnection  // 连接到旧版本节点
    newEnclave *EnclaveConnection  // 本地新版本 enclave
    ratls      *RATLSTransport     // RA-TLS 安全通道
}

// NewDataMigrator 创建数据迁移器
func NewDataMigrator(oldAddr string, newEnclave *EnclaveConnection, ratls *RATLSTransport) *DataMigrator {
    return &DataMigrator{
        oldEnclave: &EnclaveConnection{Address: oldAddr},
        newEnclave: newEnclave,
        ratls:      ratls,
    }
}

// MigrateEncryptedData 迁移加密分区数据
func (m *DataMigrator) MigrateEncryptedData(ctx context.Context) error {
    // 1. 建立 RA-TLS 连接到旧版本节点
    conn, err := m.ratls.Connect(m.oldEnclave.Address)
    if err != nil {
        return fmt.Errorf("failed to connect to old enclave: %w", err)
    }
    defer conn.Close()
    
    // 2. 请求旧版本节点解封并传输数据
    // 数据在 RA-TLS 通道中传输，保证安全性
    keys, err := m.requestKeyMigration(conn)
    if err != nil {
        return fmt.Errorf("failed to migrate keys: %w", err)
    }
    
    // 3. 在新版本 enclave 中重新封装
    for _, key := range keys {
        if err := m.newEnclave.SealKey(key); err != nil {
            return fmt.Errorf("failed to seal key %s: %w", key.ID.Hex(), err)
        }
    }
    
    return nil
}

// requestKeyMigration 请求密钥迁移
func (m *DataMigrator) requestKeyMigration(conn *RATLSConnection) ([]MigrationKeyData, error) {
    // 发送迁移请求
    req := &KeyMigrationRequest{
        RequestType: MigrationTypeAll,
    }
    
    resp, err := conn.SendRequest(req)
    if err != nil {
        return nil, err
    }
    
    return resp.Keys, nil
}

// KeyMigrationRequest 密钥迁移请求
type KeyMigrationRequest struct {
    KeyIDs      []common.Hash  // 要迁移的密钥 ID 列表（空表示全部）
    RequestType MigrationType
    Requester   common.Address // 请求者地址（必须是密钥所有者）
    Signature   []byte         // 请求者签名
}

// MigrationType 迁移类型
type MigrationType uint8

const (
    MigrationTypeAll      MigrationType = 0x01 // 迁移所有密钥
    MigrationTypeSelected MigrationType = 0x02 // 迁移指定密钥
)

// KeyMigrationResponse 密钥迁移响应
type KeyMigrationResponse struct {
    Keys    []MigrationKeyData // 解封后的密钥数据
    Success bool
    Error   string
}

// MigrationKeyData 迁移密钥数据
type MigrationKeyData struct {
    ID         common.Hash
    CurveType  uint8
    PrivateKey []byte         // 明文私钥（仅在 RA-TLS 通道中传输）
    PublicKey  []byte
    Owner      common.Address
    Metadata   KeyMetadata
}

// KeyMetadata 密钥元数据
type KeyMetadata struct {
    CreatedAt   uint64
    LastUsedAt  uint64
    UseCount    uint64
    Permissions uint64
}
```

### 迁移命令行工具

```bash
# 从旧版本节点迁移数据到新版本
geth migrate \
    --from "enode://old-node@192.168.1.100:30303" \
    --datadir /app/wallet/chaindata \
    --keys-only  # 仅迁移密钥数据，区块链数据自动继承
```

### 迁移流程图

```
硬分叉数据迁移流程
==================

1. 准备阶段
   ├── 新版本节点启动
   ├── 检测到本地加密分区为空或版本不匹配
   └── 进入迁移模式

2. 连接阶段
   ├── 扫描网络中的旧版本节点
   ├── 建立 RA-TLS 连接
   └── 验证对方 MRENCLAVE 在允许列表中

3. 数据传输阶段
   ├── 旧节点解封私钥数据
   ├── 通过 RA-TLS 加密通道传输
   └── 新节点接收并验证数据完整性

4. 重新封装阶段
   ├── 使用新 MRENCLAVE 派生的密钥封装
   ├── 写入新版本加密分区
   └── 验证封装成功

5. 完成阶段
   ├── 标记迁移完成
   ├── 断开与旧节点连接
   └── 开始正常运行
```

### MRSIGNER 模式简化迁移

如果使用 `--sgx.verify-mode mrsigner` 模式，且新旧版本使用相同的签名密钥，则可以使用 MRSIGNER 作为 sealing 密钥派生因子，避免数据迁移：

```toml
# manifest.template - 使用 MRSIGNER 作为 sealing 密钥
[[fs.mounts]]
type = "encrypted"
path = "/app/wallet"
uri = "file:/data/wallet"
key_name = "_sgx_mrsigner"  # 使用 MRSIGNER 而非 MRENCLAVE
```

### MRENCLAVE vs MRSIGNER sealing 对比

| 特性 | MRENCLAVE sealing | MRSIGNER sealing |
|------|-------------------|------------------|
| 安全性 | 更高（代码绑定） | 较低（签名者绑定） |
| 升级便利性 | 需要数据迁移 | 无需迁移 |
| 适用场景 | 高安全要求 | 频繁升级场景 |
| 回滚风险 | 低 | 旧版本可访问新数据 |

### 推荐策略

1. **生产环境**：使用 MRENCLAVE sealing + 数据迁移机制
2. **测试环境**：使用 MRSIGNER sealing 简化升级流程
3. **混合策略**：核心私钥使用 MRENCLAVE，临时数据使用 MRSIGNER

## 自动迁移管理器（AutoMigrationManager）

### 概述

AutoMigrationManager 负责在硬分叉升级期间自动触发和管理秘密数据迁移。它监听治理模块的白名单变化，并根据权限级别和升级进度调度迁移任务。

### 核心功能

1. **白名单变化监听**：实时监控 SecurityConfigContract 的 MRENCLAVE 白名单
2. **权限级别检查**：根据 PermissionLevel 限制迁移频率
3. **升级协调**：确保在 UpgradeCompleteBlock 前完成迁移
4. **自动重试**：迁移失败时自动重试
5. **进度跟踪**：记录迁移进度，支持断点续传

### 数据结构

```go
// internal/sgx/auto_migration.go
package sgx

import (
    "context"
    "sync"
    "time"
    
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/governance"
)

// AutoMigrationManager 自动迁移管理器
type AutoMigrationManager struct {
    // 配置
    config        *AutoMigrationConfig
    
    // 治理接口
    securityConfig governance.SecurityConfigReader
    
    // 本地 MRENCLAVE
    localMREnclave [32]byte
    
    // 迁移器
    migrator       *DataMigrator
    
    // 状态跟踪
    mu                sync.RWMutex
    migrationStatus   MigrationStatus
    lastCheckTime     time.Time
    dailyMigrations   int
    lastMigrationDate time.Time
    
    // 取消函数
    cancel context.CancelFunc
}

// AutoMigrationConfig 自动迁移配置
type AutoMigrationConfig struct {
    // 白名单检查间隔
    WhitelistCheckInterval time.Duration
    
    // 升级区块检查间隔
    UpgradeBlockCheckInterval time.Duration
    
    // 迁移重试次数
    MaxRetries int
    
    // 重试间隔
    RetryInterval time.Duration
}

// DefaultAutoMigrationConfig 默认配置
func DefaultAutoMigrationConfig() *AutoMigrationConfig {
    return &AutoMigrationConfig{
        WhitelistCheckInterval:    60 * time.Second,  // 每分钟检查一次
        UpgradeBlockCheckInterval: 30 * time.Second,  // 每 30 秒检查升级区块
        MaxRetries:                3,
        RetryInterval:             5 * time.Minute,
    }
}

// MigrationStatus 迁移状态
type MigrationStatus struct {
    InProgress       bool
    Completed        bool
    LastMigrationAt  time.Time
    TotalMigrations  int
    FailedAttempts   int
    CompletedBlock   uint64       // 完成迁移时的区块高度
}

// NewAutoMigrationManager 创建自动迁移管理器
func NewAutoMigrationManager(
    config *AutoMigrationConfig,
    securityConfig governance.SecurityConfigReader,
    localMREnclave [32]byte,
    migrator *DataMigrator,
) *AutoMigrationManager {
    return &AutoMigrationManager{
        config:         config,
        securityConfig: securityConfig,
        localMREnclave: localMREnclave,
        migrator:       migrator,
        migrationStatus: MigrationStatus{},
    }
}

// Start 启动自动迁移管理器
func (amm *AutoMigrationManager) Start(ctx context.Context) error {
    ctx, cancel := context.WithCancel(ctx)
    amm.cancel = cancel
    
    // 启动白名单监听
    go amm.whitelistWatcher(ctx)
    
    // 启动升级协调器
    go amm.upgradeCoordinator(ctx)
    
    return nil
}

// Stop 停止自动迁移管理器
func (amm *AutoMigrationManager) Stop() {
    if amm.cancel != nil {
        amm.cancel()
    }
}

// whitelistWatcher 监听白名单变化
func (amm *AutoMigrationManager) whitelistWatcher(ctx context.Context) {
    ticker := time.NewTicker(amm.config.WhitelistCheckInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            amm.checkWhitelistChange()
        }
    }
}

// checkWhitelistChange 检查白名单是否变化
func (amm *AutoMigrationManager) checkWhitelistChange() {
    // 获取当前白名单
    whitelist := amm.securityConfig.GetMREnclaveWhitelist()
    
    // 检查本地 MRENCLAVE 是否在白名单中
    isInWhitelist := false
    var permissionLevel governance.PermissionLevel
    
    for _, entry := range whitelist {
        if entry.MRENCLAVE == amm.localMREnclave {
            isInWhitelist = true
            permissionLevel = entry.PermissionLevel
            break
        }
    }
    
    // 如果本地 MRENCLAVE 是新添加的，且还未完成迁移
    if isInWhitelist && !amm.isMigrationComplete() {
        // 检查是否允许迁移（基于权限级别和每日限制）
        if amm.canMigrate(permissionLevel) {
            log.Info("Whitelist changed, triggering automatic migration")
            amm.triggerMigration()
        }
    }
}

// upgradeCoordinator 升级协调器
func (amm *AutoMigrationManager) upgradeCoordinator(ctx context.Context) {
    ticker := time.NewTicker(amm.config.UpgradeBlockCheckInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            amm.checkUpgradeProgress()
        }
    }
}

// checkUpgradeProgress 检查升级进度
func (amm *AutoMigrationManager) checkUpgradeProgress() {
    upgradeConfig := amm.securityConfig.GetUpgradeConfig()
    if upgradeConfig == nil || upgradeConfig.UpgradeCompleteBlock == 0 {
        return
    }
    
    // 获取当前区块高度
    currentBlock := amm.getCurrentBlockHeight()
    
    // 如果接近升级完成区块，且迁移未完成，加速迁移
    if !amm.isMigrationComplete() {
        blocksRemaining := upgradeConfig.UpgradeCompleteBlock - currentBlock
        
        if blocksRemaining < 100 {
            // 距离升级完成不足 100 个区块，立即触发迁移
            log.Warn("Approaching upgrade complete block, triggering urgent migration", 
                "remaining", blocksRemaining)
            amm.triggerMigration()
        }
    }
}

// canMigrate 检查是否允许迁移
func (amm *AutoMigrationManager) canMigrate(permissionLevel governance.PermissionLevel) bool {
    amm.mu.RLock()
    defer amm.mu.RUnlock()
    
    // 如果已经在迁移中，不允许新的迁移
    if amm.migrationStatus.InProgress {
        return false
    }
    
    // 检查每日迁移限制
    today := time.Now().Truncate(24 * time.Hour)
    if amm.lastMigrationDate.Before(today) {
        // 新的一天，重置计数器
        amm.dailyMigrations = 0
        amm.lastMigrationDate = today
    }
    
    // 根据权限级别检查限制
    var dailyLimit int
    switch permissionLevel {
    case governance.PermissionBasic:
        dailyLimit = 10
    case governance.PermissionStandard:
        dailyLimit = 100
    case governance.PermissionFull:
        dailyLimit = -1 // 无限制
    default:
        return false // 无权限
    }
    
    if dailyLimit == -1 {
        return true // 无限制
    }
    
    return amm.dailyMigrations < dailyLimit
}

// triggerMigration 触发迁移
func (amm *AutoMigrationManager) triggerMigration() {
    amm.mu.Lock()
    amm.migrationStatus.InProgress = true
    amm.mu.Unlock()
    
    go func() {
        defer func() {
            amm.mu.Lock()
            amm.migrationStatus.InProgress = false
            amm.mu.Unlock()
        }()
        
        // 执行迁移（带重试）
        ctx := context.Background()
        var err error
        
        for i := 0; i < amm.config.MaxRetries; i++ {
            err = amm.migrator.MigrateEncryptedData(ctx)
            if err == nil {
                // 迁移成功
                amm.mu.Lock()
                amm.migrationStatus.Completed = true
                amm.migrationStatus.LastMigrationAt = time.Now()
                amm.migrationStatus.TotalMigrations++
                amm.migrationStatus.CompletedBlock = amm.getCurrentBlockHeight()
                amm.dailyMigrations++
                amm.mu.Unlock()
                
                log.Info("Secret data migration completed successfully")
                return
            }
            
            // 迁移失败，记录并重试
            log.Error("Migration attempt failed", "attempt", i+1, "error", err)
            amm.mu.Lock()
            amm.migrationStatus.FailedAttempts++
            amm.mu.Unlock()
            
            if i < amm.config.MaxRetries-1 {
                time.Sleep(amm.config.RetryInterval)
            }
        }
        
        log.Error("Migration failed after all retries", "error", err)
    }()
}

// isMigrationComplete 检查迁移是否完成
func (amm *AutoMigrationManager) isMigrationComplete() bool {
    amm.mu.RLock()
    defer amm.mu.RUnlock()
    return amm.migrationStatus.Completed
}

// getCurrentBlockHeight 获取当前区块高度
func (amm *AutoMigrationManager) getCurrentBlockHeight() uint64 {
    // 从共识引擎获取当前区块高度
    // 这里简化处理，实际需要注入区块链接口
    return 0 // TODO: 实现
}

// GetMigrationStatus 获取迁移状态
func (amm *AutoMigrationManager) GetMigrationStatus() MigrationStatus {
    amm.mu.RLock()
    defer amm.mu.RUnlock()
    return amm.migrationStatus
}
```

### 集成示例

```go
// cmd/geth/main.go
package main

import (
    "context"
    
    "github.com/ethereum/go-ethereum/internal/sgx"
    "github.com/ethereum/go-ethereum/governance"
)

func startNode() {
    // 创建治理接口
    securityConfig := governance.NewSecurityConfigReader(...)
    
    // 获取本地 MRENCLAVE
    localMREnclave := sgx.GetLocalMREnclave()
    
    // 创建数据迁移器
    migrator := sgx.NewDataMigrator(...)
    
    // 创建自动迁移管理器
    autoMigrationConfig := sgx.DefaultAutoMigrationConfig()
    autoMigrationMgr := sgx.NewAutoMigrationManager(
        autoMigrationConfig,
        securityConfig,
        localMREnclave,
        migrator,
    )
    
    // 启动自动迁移管理器
    ctx := context.Background()
    if err := autoMigrationMgr.Start(ctx); err != nil {
        log.Fatal("Failed to start auto migration manager", "err", err)
    }
    
    // ... 其他节点初始化逻辑
}
```

### 迁移单元测试

```go
// internal/sgx/migration_test.go
package sgx

import (
    "context"
    "testing"
    
    "github.com/ethereum/go-ethereum/common"
)

func TestDataMigration(t *testing.T) {
    // 模拟旧版本节点
    oldEnclave := NewMockEnclave("old-mrenclave")
    
    // 创建测试密钥
    testKey := &MigrationKeyData{
        ID:         common.HexToHash("0x1234"),
        CurveType:  CurveSecp256k1,
        PrivateKey: []byte("test-private-key"),
        PublicKey:  []byte("test-public-key"),
        Owner:      common.HexToAddress("0xabcd"),
    }
    oldEnclave.StoreKey(testKey)
    
    // 模拟新版本节点
    newEnclave := NewMockEnclave("new-mrenclave")
    
    // 创建迁移器
    ratls := NewMockRATLS()
    migrator := NewDataMigrator(oldEnclave.Address, newEnclave, ratls)
    
    // 执行迁移
    err := migrator.MigrateEncryptedData(context.Background())
    if err != nil {
        t.Fatalf("Migration failed: %v", err)
    }
    
    // 验证密钥已迁移
    migratedKey, err := newEnclave.GetKey(testKey.ID)
    if err != nil {
        t.Fatalf("Failed to get migrated key: %v", err)
    }
    
    if string(migratedKey.PrivateKey) != string(testKey.PrivateKey) {
        t.Error("Migrated key does not match original")
    }
}

func TestMigrationWithInvalidMREnclave(t *testing.T) {
    // 模拟旧版本节点（MRENCLAVE 不在白名单）
    oldEnclave := NewMockEnclave("invalid-mrenclave")
    newEnclave := NewMockEnclave("new-mrenclave")
    
    // 配置白名单不包含旧版本
    ratls := NewMockRATLS()
    ratls.SetAllowedMREnclaves([]string{"new-mrenclave"})
    
    migrator := NewDataMigrator(oldEnclave.Address, newEnclave, ratls)
    
    // 迁移应该失败
    err := migrator.MigrateEncryptedData(context.Background())
    if err == nil {
        t.Error("Migration should fail with invalid MRENCLAVE")
    }
}

func TestMigrationResume(t *testing.T) {
    // 测试迁移中断后恢复
    oldEnclave := NewMockEnclave("old-mrenclave")
    newEnclave := NewMockEnclave("new-mrenclave")
    
    // 创建多个测试密钥
    for i := 0; i < 10; i++ {
        key := &MigrationKeyData{
            ID:         common.BigToHash(big.NewInt(int64(i))),
            CurveType:  CurveSecp256k1,
            PrivateKey: []byte(fmt.Sprintf("key-%d", i)),
        }
        oldEnclave.StoreKey(key)
    }
    
    // 模拟部分迁移（前 5 个已迁移）
    for i := 0; i < 5; i++ {
        key, _ := oldEnclave.GetKey(common.BigToHash(big.NewInt(int64(i))))
        newEnclave.StoreKey(key)
    }
    
    // 创建迁移器并恢复
    ratls := NewMockRATLS()
    migrator := NewDataMigrator(oldEnclave.Address, newEnclave, ratls)
    migrator.SetResumeMode(true)
    
    err := migrator.MigrateEncryptedData(context.Background())
    if err != nil {
        t.Fatalf("Resume migration failed: %v", err)
    }
    
    // 验证所有密钥都已迁移
    for i := 0; i < 10; i++ {
        _, err := newEnclave.GetKey(common.BigToHash(big.NewInt(int64(i))))
        if err != nil {
            t.Errorf("Key %d not migrated", i)
        }
    }
}
```

## 注意事项

1. **参数校验**：安全参数必须与 Manifest 一致，不一致则退出进程
2. **私钥存储**：私钥必须存储在加密分区，不能存储在普通目录
3. **同步安全**：同步前必须验证对方的 SGX Quote 和 MRENCLAVE
4. **常量时间**：所有密码学比较操作使用常量时间实现
5. **安全删除**：删除秘密数据时先覆盖再删除
6. **数据迁移**：硬分叉时需要通过 RA-TLS 安全通道迁移加密分区数据
