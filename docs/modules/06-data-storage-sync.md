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
        +---> SGX 证明模块（RA-TLS 传输）
        |
        +---> 治理模块（度量值验证）
        |
        +---> Gramine 运行时（加密文件系统）
```

### 上游依赖
- SGX 证明模块（RA-TLS 安全通道）
- 治理模块（度量值白名单）
- Gramine LibOS（加密文件系统）

### 下游依赖（被以下模块使用）
- 预编译合约模块（密钥存储）
- 共识引擎模块（状态持久化）

## 参数分类与校验

### 参数分类原则

X Chain 的配置参数分为两类：

| 类别 | 控制方式 | 特点 | 示例 |
|------|----------|------|------|
| **安全相关参数** | Gramine Manifest | 影响度量值，不可外部修改 | 度量值白名单、加密分区路径、密钥迁移配置 |
| **非安全参数** | 命令行参数 | 不影响安全性，可灵活配置 | 出块间隔、RPC 端口、日志级别 |

### 安全相关参数（Manifest 控制）

```toml
# gramine manifest 中的安全参数
[loader.env]
# 度量值白名单（JSON 格式）
XCHAIN_MRENCLAVE_WHITELIST = '["abc123...", "def456..."]'

# 加密分区路径
XCHAIN_ENCRYPTED_PATH = "/data/encrypted"

# 秘密数据存储路径
XCHAIN_SECRET_PATH = "/data/secrets"

# 密钥迁移配置
XCHAIN_KEY_MIGRATION_ENABLED = "true"
XCHAIN_KEY_MIGRATION_THRESHOLD = "2"

# 节点准入控制
XCHAIN_ADMISSION_STRICT = "true"
XCHAIN_ADMISSION_VERIFY_QUOTE = "true"
```

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
var SecurityParams = []ParamDefinition{
    {
        Name:     "mrenclave_whitelist",
        Category: ParamCategorySecurity,
        EnvKey:   "XCHAIN_MRENCLAVE_WHITELIST",
        CliFlag:  "xchain.whitelist",
        Required: true,
        Validator: func(v string) error {
            // 白名单使用 Base64 编码的 CSV 格式
            decoded, err := base64.StdEncoding.DecodeString(v)
            if err != nil {
                return fmt.Errorf("invalid Base64 encoding: %w", err)
            }
            lines := strings.Split(string(decoded), "\n")
            if len(lines) == 0 {
                return errors.New("whitelist cannot be empty")
            }
            return nil
        },
    },
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

## 注意事项

1. **参数校验**：安全参数必须与 Manifest 一致，不一致则退出进程
2. **私钥存储**：私钥必须存储在加密分区，不能存储在普通目录
3. **同步安全**：同步前必须验证对方的 SGX Quote 和 MRENCLAVE
4. **常量时间**：所有密码学比较操作使用常量时间实现
5. **安全删除**：删除秘密数据时先覆盖再删除
