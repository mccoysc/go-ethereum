# 预编译合约模块开发文档

## 模块概述

预编译合约模块实现 X Chain 的 SGX 密钥管理功能，通过预编译合约（地址 0x8000-0x80FF）提供密钥创建、签名、加密等密码学操作。所有私钥都在 SGX enclave 内部生成和使用，永不离开可信执行环境。

## 负责团队

**智能合约/EVM 团队**

## 模块职责

1. 实现 SGX 预编译合约接口
2. 密钥创建与管理
3. 签名与验证操作
4. 加密与解密操作
5. 密钥派生（ECDH、KDF）
6. 权限管理机制
7. 随机数生成

## 依赖关系

```
+------------------+
|  预编译合约模块  |
+------------------+
        |
        +---> SGX 证明模块（身份验证）
        |
        +---> 数据存储模块（密钥持久化）
        |
        +---> EVM 核心（合约执行）
```

### 上游依赖
- SGX 证明模块（验证调用者身份）
- 数据存储模块（加密分区存储私钥）
- go-ethereum EVM 框架

### 下游依赖（被以下模块使用）
- 用户智能合约（通过 CALL 调用）
- 钱包应用（密钥管理）

## 预编译合约地址分配

| 地址 | 名称 | 功能 |
|------|------|------|
| 0x8000 | SGX_KEY_CREATE | 创建新密钥对 |
| 0x8001 | SGX_KEY_GET_PUBLIC | 获取公钥 |
| 0x8002 | SGX_SIGN | ECDSA 签名 |
| 0x8003 | SGX_VERIFY | 签名验证 |
| 0x8004 | SGX_ECDH | ECDH 密钥交换 |
| 0x8005 | SGX_RANDOM | 安全随机数生成 |
| 0x8006 | SGX_ENCRYPT | 对称加密 |
| 0x8007 | SGX_DECRYPT | 对称解密 |
| 0x8008 | SGX_KEY_DERIVE | 密钥派生 |

## 核心接口定义

### 预编译合约基础接口

```go
// core/vm/contracts_sgx.go
package vm

import (
    "github.com/ethereum/go-ethereum/common"
)

// SGXPrecompile SGX 预编译合约接口
type SGXPrecompile interface {
    // RequiredGas 计算所需 Gas
    RequiredGas(input []byte) uint64
    
    // Run 执行合约
    Run(input []byte) ([]byte, error)
}

// SGXPrecompileWithContext 带上下文的预编译合约接口
type SGXPrecompileWithContext interface {
    SGXPrecompile
    
    // RunWithContext 带上下文执行
    RunWithContext(ctx *SGXContext, input []byte) ([]byte, error)
}

// SGXContext SGX 执行上下文
type SGXContext struct {
    // 调用者地址
    Caller common.Address
    
    // 交易发起者
    Origin common.Address
    
    // 区块号
    BlockNumber uint64
    
    // 时间戳
    Timestamp uint64
    
    // 密钥存储
    KeyStore KeyStore
    
    // 权限管理器
    PermissionManager PermissionManager
}
```

### 密钥存储接口

```go
// core/vm/sgx_keystore.go
package vm

import (
    "crypto/ecdsa"
    
    "github.com/ethereum/go-ethereum/common"
)

// KeyType 密钥类型
type KeyType uint8

const (
    KeyTypeECDSA   KeyType = 0x01 // secp256k1
    KeyTypeEd25519 KeyType = 0x02 // Ed25519
    KeyTypeAES256  KeyType = 0x03 // AES-256
)

// KeyMetadata 密钥元数据
type KeyMetadata struct {
    KeyID       common.Hash    // 密钥 ID
    Owner       common.Address // 所有者
    KeyType     KeyType        // 密钥类型
    CreatedAt   uint64         // 创建时间
    CreatedBy   common.Address // 创建者
    Permissions []Permission   // 权限列表
}

// KeyStore 密钥存储接口
type KeyStore interface {
    // CreateKey 创建新密钥
    CreateKey(owner common.Address, keyType KeyType) (common.Hash, error)
    
    // GetPublicKey 获取公钥
    GetPublicKey(keyID common.Hash) ([]byte, error)
    
    // Sign 使用密钥签名
    Sign(keyID common.Hash, hash []byte) ([]byte, error)
    
    // ECDH 执行 ECDH 密钥交换
    ECDH(keyID common.Hash, peerPubKey []byte) ([]byte, error)
    
    // Encrypt 加密数据
    Encrypt(keyID common.Hash, plaintext []byte) ([]byte, error)
    
    // Decrypt 解密数据
    Decrypt(keyID common.Hash, ciphertext []byte) ([]byte, error)
    
    // DeriveKey 派生子密钥
    DeriveKey(keyID common.Hash, path []byte) (common.Hash, error)
    
    // GetMetadata 获取密钥元数据
    GetMetadata(keyID common.Hash) (*KeyMetadata, error)
    
    // DeleteKey 删除密钥
    DeleteKey(keyID common.Hash) error
}
```

### 权限管理接口

```go
// core/vm/sgx_permission.go
package vm

import (
    "github.com/ethereum/go-ethereum/common"
)

// PermissionType 权限类型
type PermissionType uint8

const (
    PermissionSign    PermissionType = 0x01 // 签名权限
    PermissionDecrypt PermissionType = 0x02 // 解密权限
    PermissionDerive  PermissionType = 0x04 // 派生权限
    PermissionAdmin   PermissionType = 0x80 // 管理权限
)

// Permission 权限定义
type Permission struct {
    Grantee    common.Address // 被授权者
    Type       PermissionType // 权限类型
    ExpiresAt  uint64         // 过期时间（0 表示永不过期）
    MaxUses    uint64         // 最大使用次数（0 表示无限制）
    UsedCount  uint64         // 已使用次数
}

// PermissionManager 权限管理器接口
type PermissionManager interface {
    // GrantPermission 授予权限
    GrantPermission(keyID common.Hash, permission Permission) error
    
    // RevokePermission 撤销权限
    RevokePermission(keyID common.Hash, grantee common.Address, permType PermissionType) error
    
    // CheckPermission 检查权限
    CheckPermission(keyID common.Hash, caller common.Address, permType PermissionType) bool
    
    // GetPermissions 获取所有权限
    GetPermissions(keyID common.Hash) ([]Permission, error)
    
    // UsePermission 使用权限（增加计数）
    UsePermission(keyID common.Hash, caller common.Address, permType PermissionType) error
}
```

## 预编译合约实现

### SGX_KEY_CREATE (0x8000)

```go
// core/vm/sgx_key_create.go
package vm

import (
    "errors"
    "fmt"
    
    "github.com/ethereum/go-ethereum/common"
)

// SGXKeyCreate 密钥创建预编译合约
type SGXKeyCreate struct {
    keyStore KeyStore
}

// 输入格式: keyType (1 byte)
// 输出格式: keyID (32 bytes)

func (c *SGXKeyCreate) RequiredGas(input []byte) uint64 {
    return 100000 // 密钥生成消耗较多 Gas
}

func (c *SGXKeyCreate) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
    // 1. 解析输入
    if len(input) < 1 {
        return nil, errors.New("invalid input: missing key type")
    }
    keyType := KeyType(input[0])
    
    // 2. 验证密钥类型
    if keyType != KeyTypeECDSA && keyType != KeyTypeEd25519 && keyType != KeyTypeAES256 {
        return nil, fmt.Errorf("unsupported key type: %d", keyType)
    }
    
    // 3. 创建密钥
    keyID, err := c.keyStore.CreateKey(ctx.Caller, keyType)
    if err != nil {
        return nil, fmt.Errorf("failed to create key: %w", err)
    }
    
    // 4. 返回密钥 ID
    return keyID.Bytes(), nil
}

func (c *SGXKeyCreate) Run(input []byte) ([]byte, error) {
    return nil, errors.New("context required")
}
```

### SGX_KEY_GET_PUBLIC (0x8001)

```go
// core/vm/sgx_key_get_public.go
package vm

import (
    "errors"
    
    "github.com/ethereum/go-ethereum/common"
)

// SGXKeyGetPublic 获取公钥预编译合约
type SGXKeyGetPublic struct {
    keyStore KeyStore
}

// 输入格式: keyID (32 bytes)
// 输出格式: publicKey (variable length)

func (c *SGXKeyGetPublic) RequiredGas(input []byte) uint64 {
    return 3000
}

func (c *SGXKeyGetPublic) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
    // 1. 解析输入
    if len(input) < 32 {
        return nil, errors.New("invalid input: missing key ID")
    }
    keyID := common.BytesToHash(input[:32])
    
    // 2. 获取公钥（公钥可以公开访问，无需权限检查）
    pubKey, err := c.keyStore.GetPublicKey(keyID)
    if err != nil {
        return nil, err
    }
    
    return pubKey, nil
}

func (c *SGXKeyGetPublic) Run(input []byte) ([]byte, error) {
    return nil, errors.New("context required")
}
```

### SGX_SIGN (0x8002)

```go
// core/vm/sgx_sign.go
package vm

import (
    "errors"
    
    "github.com/ethereum/go-ethereum/common"
)

// SGXSign 签名预编译合约
type SGXSign struct {
    keyStore          KeyStore
    permissionManager PermissionManager
}

// 输入格式: keyID (32 bytes) + hash (32 bytes)
// 输出格式: signature (65 bytes for ECDSA: r + s + v)

func (c *SGXSign) RequiredGas(input []byte) uint64 {
    return 10000
}

func (c *SGXSign) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
    // 1. 解析输入
    if len(input) < 64 {
        return nil, errors.New("invalid input: need keyID + hash")
    }
    keyID := common.BytesToHash(input[:32])
    hash := input[32:64]
    
    // 2. 检查权限
    if !c.permissionManager.CheckPermission(keyID, ctx.Caller, PermissionSign) {
        return nil, errors.New("permission denied: no sign permission")
    }
    
    // 3. 使用权限（增加计数）
    if err := c.permissionManager.UsePermission(keyID, ctx.Caller, PermissionSign); err != nil {
        return nil, err
    }
    
    // 4. 执行签名
    signature, err := c.keyStore.Sign(keyID, hash)
    if err != nil {
        return nil, err
    }
    
    return signature, nil
}

func (c *SGXSign) Run(input []byte) ([]byte, error) {
    return nil, errors.New("context required")
}
```

### SGX_VERIFY (0x8003)

```go
// core/vm/sgx_verify.go
package vm

import (
    "crypto/ecdsa"
    "errors"
    
    "github.com/ethereum/go-ethereum/crypto"
)

// SGXVerify 签名验证预编译合约
type SGXVerify struct{}

// 输入格式: hash (32 bytes) + signature (65 bytes) + publicKey (64 bytes)
// 输出格式: valid (1 byte: 0x01 = valid, 0x00 = invalid)

func (c *SGXVerify) RequiredGas(input []byte) uint64 {
    return 3000
}

func (c *SGXVerify) Run(input []byte) ([]byte, error) {
    // 1. 解析输入
    if len(input) < 161 { // 32 + 65 + 64
        return nil, errors.New("invalid input length")
    }
    
    hash := input[:32]
    signature := input[32:97]
    pubKeyBytes := input[97:161]
    
    // 2. 恢复公钥
    pubKey, err := crypto.UnmarshalPubkey(append([]byte{0x04}, pubKeyBytes...))
    if err != nil {
        return []byte{0x00}, nil // 无效公钥
    }
    
    // 3. 验证签名
    sigWithoutV := signature[:64]
    valid := crypto.VerifySignature(
        crypto.FromECDSAPub(pubKey),
        hash,
        sigWithoutV,
    )
    
    if valid {
        return []byte{0x01}, nil
    }
    return []byte{0x00}, nil
}
```

### SGX_ECDH (0x8004)

```go
// core/vm/sgx_ecdh.go
package vm

import (
    "errors"
    
    "github.com/ethereum/go-ethereum/common"
)

// SGXECDH ECDH 密钥交换预编译合约
type SGXECDH struct {
    keyStore          KeyStore
    permissionManager PermissionManager
}

// 输入格式: keyID (32 bytes) + peerPublicKey (64 bytes)
// 输出格式: sharedSecret (32 bytes)

func (c *SGXECDH) RequiredGas(input []byte) uint64 {
    return 15000
}

func (c *SGXECDH) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
    // 1. 解析输入
    if len(input) < 96 {
        return nil, errors.New("invalid input: need keyID + peerPublicKey")
    }
    keyID := common.BytesToHash(input[:32])
    peerPubKey := input[32:96]
    
    // 2. 检查权限（ECDH 需要派生权限）
    if !c.permissionManager.CheckPermission(keyID, ctx.Caller, PermissionDerive) {
        return nil, errors.New("permission denied: no derive permission")
    }
    
    // 3. 执行 ECDH
    sharedSecret, err := c.keyStore.ECDH(keyID, peerPubKey)
    if err != nil {
        return nil, err
    }
    
    return sharedSecret, nil
}

func (c *SGXECDH) Run(input []byte) ([]byte, error) {
    return nil, errors.New("context required")
}
```

### SGX_RANDOM (0x8005)

```go
// core/vm/sgx_random.go
package vm

import (
    "crypto/rand"
    "errors"
    "io"
)

// SGXRandom 安全随机数生成预编译合约
type SGXRandom struct{}

// 输入格式: length (32 bytes, big-endian uint256)
// 输出格式: random bytes (variable length)

const MaxRandomLength = 1024 // 最大随机数长度

func (c *SGXRandom) RequiredGas(input []byte) uint64 {
    if len(input) < 32 {
        return 100
    }
    
    // 根据请求长度计算 Gas
    length := bytesToUint64(input[24:32]) // 取低 8 字节
    if length > MaxRandomLength {
        length = MaxRandomLength
    }
    
    return 100 + length*10
}

func (c *SGXRandom) Run(input []byte) ([]byte, error) {
    // 1. 解析长度
    if len(input) < 32 {
        return nil, errors.New("invalid input: missing length")
    }
    
    length := bytesToUint64(input[24:32])
    if length == 0 {
        return nil, errors.New("invalid length: must be > 0")
    }
    if length > MaxRandomLength {
        length = MaxRandomLength
    }
    
    // 2. 生成随机数
    randomBytes := make([]byte, length)
    if _, err := io.ReadFull(rand.Reader, randomBytes); err != nil {
        return nil, err
    }
    
    return randomBytes, nil
}

func bytesToUint64(b []byte) uint64 {
    var result uint64
    for _, v := range b {
        result = result<<8 | uint64(v)
    }
    return result
}
```

### SGX_ENCRYPT (0x8006)

```go
// core/vm/sgx_encrypt.go
package vm

import (
    "errors"
    
    "github.com/ethereum/go-ethereum/common"
)

// SGXEncrypt 加密预编译合约
type SGXEncrypt struct {
    keyStore KeyStore
}

// 输入格式: keyID (32 bytes) + plaintext (variable)
// 输出格式: nonce (12 bytes) + ciphertext (variable) + tag (16 bytes)

func (c *SGXEncrypt) RequiredGas(input []byte) uint64 {
    if len(input) < 32 {
        return 5000
    }
    plaintextLen := len(input) - 32
    return 5000 + uint64(plaintextLen)*10
}

func (c *SGXEncrypt) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
    // 1. 解析输入
    if len(input) < 33 {
        return nil, errors.New("invalid input: need keyID + plaintext")
    }
    keyID := common.BytesToHash(input[:32])
    plaintext := input[32:]
    
    // 2. 加密（加密不需要特殊权限，任何人都可以用公钥加密）
    ciphertext, err := c.keyStore.Encrypt(keyID, plaintext)
    if err != nil {
        return nil, err
    }
    
    return ciphertext, nil
}

func (c *SGXEncrypt) Run(input []byte) ([]byte, error) {
    return nil, errors.New("context required")
}
```

### SGX_DECRYPT (0x8007)

```go
// core/vm/sgx_decrypt.go
package vm

import (
    "errors"
    
    "github.com/ethereum/go-ethereum/common"
)

// SGXDecrypt 解密预编译合约
type SGXDecrypt struct {
    keyStore          KeyStore
    permissionManager PermissionManager
}

// 输入格式: keyID (32 bytes) + ciphertext (variable)
// 输出格式: plaintext (variable)

func (c *SGXDecrypt) RequiredGas(input []byte) uint64 {
    if len(input) < 32 {
        return 5000
    }
    ciphertextLen := len(input) - 32
    return 5000 + uint64(ciphertextLen)*10
}

func (c *SGXDecrypt) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
    // 1. 解析输入
    if len(input) < 60 { // 32 + 12 (nonce) + 16 (tag) minimum
        return nil, errors.New("invalid input: ciphertext too short")
    }
    keyID := common.BytesToHash(input[:32])
    ciphertext := input[32:]
    
    // 2. 检查权限
    if !c.permissionManager.CheckPermission(keyID, ctx.Caller, PermissionDecrypt) {
        return nil, errors.New("permission denied: no decrypt permission")
    }
    
    // 3. 使用权限
    if err := c.permissionManager.UsePermission(keyID, ctx.Caller, PermissionDecrypt); err != nil {
        return nil, err
    }
    
    // 4. 解密
    plaintext, err := c.keyStore.Decrypt(keyID, ciphertext)
    if err != nil {
        return nil, err
    }
    
    return plaintext, nil
}

func (c *SGXDecrypt) Run(input []byte) ([]byte, error) {
    return nil, errors.New("context required")
}
```

### SGX_KEY_DERIVE (0x8008)

```go
// core/vm/sgx_key_derive.go
package vm

import (
    "errors"
    
    "github.com/ethereum/go-ethereum/common"
)

// SGXKeyDerive 密钥派生预编译合约
type SGXKeyDerive struct {
    keyStore          KeyStore
    permissionManager PermissionManager
}

// 输入格式: parentKeyID (32 bytes) + derivationPath (variable)
// 输出格式: childKeyID (32 bytes)

func (c *SGXKeyDerive) RequiredGas(input []byte) uint64 {
    return 50000
}

func (c *SGXKeyDerive) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
    // 1. 解析输入
    if len(input) < 33 {
        return nil, errors.New("invalid input: need parentKeyID + path")
    }
    parentKeyID := common.BytesToHash(input[:32])
    path := input[32:]
    
    // 2. 检查权限
    if !c.permissionManager.CheckPermission(parentKeyID, ctx.Caller, PermissionDerive) {
        return nil, errors.New("permission denied: no derive permission")
    }
    
    // 3. 派生密钥
    childKeyID, err := c.keyStore.DeriveKey(parentKeyID, path)
    if err != nil {
        return nil, err
    }
    
    return childKeyID.Bytes(), nil
}

func (c *SGXKeyDerive) Run(input []byte) ([]byte, error) {
    return nil, errors.New("context required")
}
```

## 密钥存储实现

### 加密分区存储

根据安全要求，私钥必须存储在加密分区中：

```go
// core/vm/sgx_keystore_impl.go
package vm

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/ecdsa"
    "crypto/elliptic"
    "crypto/rand"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "sync"
    
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/crypto"
)

// EncryptedKeyStore 加密密钥存储
type EncryptedKeyStore struct {
    mu sync.RWMutex
    
    // 加密分区路径（私钥存储）
    encryptedPath string
    
    // 普通路径（公钥和元数据存储）
    publicPath string
    
    // 内存缓存
    keys     map[common.Hash]*keyEntry
    metadata map[common.Hash]*KeyMetadata
}

type keyEntry struct {
    PrivateKey *ecdsa.PrivateKey
    KeyType    KeyType
}

// NewEncryptedKeyStore 创建加密密钥存储
// encryptedPath: 加密分区路径（如 /data/keys）- 存储私钥
// publicPath: 普通路径（如 /app/public）- 存储公钥和元数据
func NewEncryptedKeyStore(encryptedPath, publicPath string) (*EncryptedKeyStore, error) {
    // 确保目录存在
    if err := os.MkdirAll(encryptedPath, 0700); err != nil {
        return nil, fmt.Errorf("failed to create encrypted path: %w", err)
    }
    if err := os.MkdirAll(publicPath, 0755); err != nil {
        return nil, fmt.Errorf("failed to create public path: %w", err)
    }
    
    ks := &EncryptedKeyStore{
        encryptedPath: encryptedPath,
        publicPath:    publicPath,
        keys:          make(map[common.Hash]*keyEntry),
        metadata:      make(map[common.Hash]*KeyMetadata),
    }
    
    // 加载已有密钥
    if err := ks.loadKeys(); err != nil {
        return nil, err
    }
    
    return ks, nil
}

// CreateKey 创建新密钥
func (ks *EncryptedKeyStore) CreateKey(owner common.Address, keyType KeyType) (common.Hash, error) {
    ks.mu.Lock()
    defer ks.mu.Unlock()
    
    var privateKey *ecdsa.PrivateKey
    var err error
    
    switch keyType {
    case KeyTypeECDSA:
        privateKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
        if err != nil {
            return common.Hash{}, err
        }
    default:
        return common.Hash{}, fmt.Errorf("unsupported key type: %d", keyType)
    }
    
    // 生成密钥 ID
    pubKeyBytes := crypto.FromECDSAPub(&privateKey.PublicKey)
    keyID := crypto.Keccak256Hash(pubKeyBytes)
    
    // 存储私钥到加密分区
    if err := ks.savePrivateKey(keyID, privateKey); err != nil {
        return common.Hash{}, err
    }
    
    // 创建元数据
    metadata := &KeyMetadata{
        KeyID:     keyID,
        Owner:     owner,
        KeyType:   keyType,
        CreatedAt: uint64(time.Now().Unix()),
        CreatedBy: owner,
        Permissions: []Permission{
            {
                Grantee: owner,
                Type:    PermissionSign | PermissionDecrypt | PermissionDerive | PermissionAdmin,
            },
        },
    }
    
    // 存储元数据到普通路径
    if err := ks.saveMetadata(keyID, metadata); err != nil {
        return common.Hash{}, err
    }
    
    // 更新缓存
    ks.keys[keyID] = &keyEntry{
        PrivateKey: privateKey,
        KeyType:    keyType,
    }
    ks.metadata[keyID] = metadata
    
    return keyID, nil
}

// savePrivateKey 保存私钥到加密分区
func (ks *EncryptedKeyStore) savePrivateKey(keyID common.Hash, privateKey *ecdsa.PrivateKey) error {
    keyPath := filepath.Join(ks.encryptedPath, keyID.Hex()+".key")
    
    // 序列化私钥
    keyBytes := crypto.FromECDSA(privateKey)
    
    // 写入加密分区（Gramine 会自动加密）
    return os.WriteFile(keyPath, keyBytes, 0600)
}

// saveMetadata 保存元数据到普通路径
func (ks *EncryptedKeyStore) saveMetadata(keyID common.Hash, metadata *KeyMetadata) error {
    metaPath := filepath.Join(ks.publicPath, keyID.Hex()+".meta")
    
    data, err := json.Marshal(metadata)
    if err != nil {
        return err
    }
    
    return os.WriteFile(metaPath, data, 0644)
}

// GetPublicKey 获取公钥
func (ks *EncryptedKeyStore) GetPublicKey(keyID common.Hash) ([]byte, error) {
    ks.mu.RLock()
    defer ks.mu.RUnlock()
    
    entry, ok := ks.keys[keyID]
    if !ok {
        return nil, errors.New("key not found")
    }
    
    return crypto.FromECDSAPub(&entry.PrivateKey.PublicKey), nil
}

// Sign 签名
func (ks *EncryptedKeyStore) Sign(keyID common.Hash, hash []byte) ([]byte, error) {
    ks.mu.RLock()
    defer ks.mu.RUnlock()
    
    entry, ok := ks.keys[keyID]
    if !ok {
        return nil, errors.New("key not found")
    }
    
    return crypto.Sign(hash, entry.PrivateKey)
}

// ECDH 密钥交换
func (ks *EncryptedKeyStore) ECDH(keyID common.Hash, peerPubKey []byte) ([]byte, error) {
    ks.mu.RLock()
    defer ks.mu.RUnlock()
    
    entry, ok := ks.keys[keyID]
    if !ok {
        return nil, errors.New("key not found")
    }
    
    // 解析对方公钥
    peerKey, err := crypto.UnmarshalPubkey(append([]byte{0x04}, peerPubKey...))
    if err != nil {
        return nil, err
    }
    
    // 执行 ECDH
    x, _ := entry.PrivateKey.Curve.ScalarMult(peerKey.X, peerKey.Y, entry.PrivateKey.D.Bytes())
    
    // 返回共享密钥的哈希
    return crypto.Keccak256(x.Bytes()), nil
}

// loadKeys 加载已有密钥
func (ks *EncryptedKeyStore) loadKeys() error {
    // 从加密分区加载私钥
    files, err := os.ReadDir(ks.encryptedPath)
    if err != nil {
        if os.IsNotExist(err) {
            return nil
        }
        return err
    }
    
    for _, file := range files {
        if filepath.Ext(file.Name()) != ".key" {
            continue
        }
        
        keyIDHex := file.Name()[:len(file.Name())-4]
        keyID := common.HexToHash(keyIDHex)
        
        // 读取私钥
        keyPath := filepath.Join(ks.encryptedPath, file.Name())
        keyBytes, err := os.ReadFile(keyPath)
        if err != nil {
            continue
        }
        
        privateKey, err := crypto.ToECDSA(keyBytes)
        if err != nil {
            continue
        }
        
        ks.keys[keyID] = &keyEntry{
            PrivateKey: privateKey,
            KeyType:    KeyTypeECDSA,
        }
        
        // 加载元数据
        metaPath := filepath.Join(ks.publicPath, keyIDHex+".meta")
        metaBytes, err := os.ReadFile(metaPath)
        if err == nil {
            var metadata KeyMetadata
            if json.Unmarshal(metaBytes, &metadata) == nil {
                ks.metadata[keyID] = &metadata
            }
        }
    }
    
    return nil
}
```

## 权限管理实现

```go
// core/vm/sgx_permission_impl.go
package vm

import (
    "errors"
    "sync"
    "time"
    
    "github.com/ethereum/go-ethereum/common"
)

// InMemoryPermissionManager 内存权限管理器
type InMemoryPermissionManager struct {
    mu          sync.RWMutex
    permissions map[common.Hash][]Permission
}

// NewInMemoryPermissionManager 创建权限管理器
func NewInMemoryPermissionManager() *InMemoryPermissionManager {
    return &InMemoryPermissionManager{
        permissions: make(map[common.Hash][]Permission),
    }
}

// GrantPermission 授予权限
func (pm *InMemoryPermissionManager) GrantPermission(keyID common.Hash, permission Permission) error {
    pm.mu.Lock()
    defer pm.mu.Unlock()
    
    perms := pm.permissions[keyID]
    
    // 检查是否已存在相同权限
    for i, p := range perms {
        if p.Grantee == permission.Grantee && p.Type == permission.Type {
            // 更新现有权限
            perms[i] = permission
            pm.permissions[keyID] = perms
            return nil
        }
    }
    
    // 添加新权限
    pm.permissions[keyID] = append(perms, permission)
    return nil
}

// RevokePermission 撤销权限
func (pm *InMemoryPermissionManager) RevokePermission(keyID common.Hash, grantee common.Address, permType PermissionType) error {
    pm.mu.Lock()
    defer pm.mu.Unlock()
    
    perms := pm.permissions[keyID]
    for i, p := range perms {
        if p.Grantee == grantee && p.Type == permType {
            // 移除权限
            pm.permissions[keyID] = append(perms[:i], perms[i+1:]...)
            return nil
        }
    }
    
    return errors.New("permission not found")
}

// CheckPermission 检查权限
func (pm *InMemoryPermissionManager) CheckPermission(keyID common.Hash, caller common.Address, permType PermissionType) bool {
    pm.mu.RLock()
    defer pm.mu.RUnlock()
    
    perms := pm.permissions[keyID]
    for _, p := range perms {
        if p.Grantee != caller {
            continue
        }
        
        // 检查权限类型
        if p.Type&permType == 0 {
            continue
        }
        
        // 检查过期时间
        if p.ExpiresAt > 0 && uint64(time.Now().Unix()) > p.ExpiresAt {
            continue
        }
        
        // 检查使用次数
        if p.MaxUses > 0 && p.UsedCount >= p.MaxUses {
            continue
        }
        
        return true
    }
    
    return false
}

// UsePermission 使用权限
func (pm *InMemoryPermissionManager) UsePermission(keyID common.Hash, caller common.Address, permType PermissionType) error {
    pm.mu.Lock()
    defer pm.mu.Unlock()
    
    perms := pm.permissions[keyID]
    for i, p := range perms {
        if p.Grantee == caller && p.Type&permType != 0 {
            perms[i].UsedCount++
            pm.permissions[keyID] = perms
            return nil
        }
    }
    
    return errors.New("permission not found")
}

// GetPermissions 获取所有权限
func (pm *InMemoryPermissionManager) GetPermissions(keyID common.Hash) ([]Permission, error) {
    pm.mu.RLock()
    defer pm.mu.RUnlock()
    
    perms, ok := pm.permissions[keyID]
    if !ok {
        return nil, nil
    }
    
    // 返回副本
    result := make([]Permission, len(perms))
    copy(result, perms)
    return result, nil
}
```

## 注册预编译合约

```go
// core/vm/contracts.go (修改)
package vm

import (
    "github.com/ethereum/go-ethereum/common"
)

// SGX 预编译合约地址
var (
    SGXKeyCreateAddr    = common.HexToAddress("0x8000")
    SGXKeyGetPublicAddr = common.HexToAddress("0x8001")
    SGXSignAddr         = common.HexToAddress("0x8002")
    SGXVerifyAddr       = common.HexToAddress("0x8003")
    SGXECDHAddr         = common.HexToAddress("0x8004")
    SGXRandomAddr       = common.HexToAddress("0x8005")
    SGXEncryptAddr      = common.HexToAddress("0x8006")
    SGXDecryptAddr      = common.HexToAddress("0x8007")
    SGXKeyDeriveAddr    = common.HexToAddress("0x8008")
)

// RegisterSGXPrecompiles 注册 SGX 预编译合约
func RegisterSGXPrecompiles(keyStore KeyStore, permMgr PermissionManager) map[common.Address]PrecompiledContract {
    return map[common.Address]PrecompiledContract{
        SGXKeyCreateAddr:    &SGXKeyCreate{keyStore: keyStore},
        SGXKeyGetPublicAddr: &SGXKeyGetPublic{keyStore: keyStore},
        SGXSignAddr:         &SGXSign{keyStore: keyStore, permissionManager: permMgr},
        SGXVerifyAddr:       &SGXVerify{},
        SGXECDHAddr:         &SGXECDH{keyStore: keyStore, permissionManager: permMgr},
        SGXRandomAddr:       &SGXRandom{},
        SGXEncryptAddr:      &SGXEncrypt{keyStore: keyStore},
        SGXDecryptAddr:      &SGXDecrypt{keyStore: keyStore, permissionManager: permMgr},
        SGXKeyDeriveAddr:    &SGXKeyDerive{keyStore: keyStore, permissionManager: permMgr},
    }
}
```

## 文件结构

```
core/vm/
├── contracts_sgx.go          # SGX 预编译合约接口
├── sgx_keystore.go           # 密钥存储接口
├── sgx_keystore_impl.go      # 密钥存储实现
├── sgx_permission.go         # 权限管理接口
├── sgx_permission_impl.go    # 权限管理实现
├── sgx_key_create.go         # 0x8000 密钥创建
├── sgx_key_get_public.go     # 0x8001 获取公钥
├── sgx_sign.go               # 0x8002 签名
├── sgx_verify.go             # 0x8003 验证
├── sgx_ecdh.go               # 0x8004 ECDH
├── sgx_random.go             # 0x8005 随机数
├── sgx_encrypt.go            # 0x8006 加密
├── sgx_decrypt.go            # 0x8007 解密
├── sgx_key_derive.go         # 0x8008 密钥派生
└── contracts_sgx_test.go     # 测试
```

## 单元测试指南

### 密钥创建测试

```go
// core/vm/sgx_key_create_test.go
package vm

import (
    "testing"
    
    "github.com/ethereum/go-ethereum/common"
)

func TestKeyCreate(t *testing.T) {
    keyStore, _ := NewEncryptedKeyStore("/tmp/test_keys", "/tmp/test_public")
    contract := &SGXKeyCreate{keyStore: keyStore}
    
    ctx := &SGXContext{
        Caller: common.HexToAddress("0x1234"),
    }
    
    // 创建 ECDSA 密钥
    input := []byte{byte(KeyTypeECDSA)}
    result, err := contract.RunWithContext(ctx, input)
    
    if err != nil {
        t.Fatalf("KeyCreate failed: %v", err)
    }
    
    if len(result) != 32 {
        t.Errorf("Expected 32 bytes keyID, got %d", len(result))
    }
}

func TestKeyCreateInvalidType(t *testing.T) {
    keyStore, _ := NewEncryptedKeyStore("/tmp/test_keys", "/tmp/test_public")
    contract := &SGXKeyCreate{keyStore: keyStore}
    
    ctx := &SGXContext{
        Caller: common.HexToAddress("0x1234"),
    }
    
    // 无效密钥类型
    input := []byte{0xFF}
    _, err := contract.RunWithContext(ctx, input)
    
    if err == nil {
        t.Error("Expected error for invalid key type")
    }
}
```

### 签名验证测试

```go
// core/vm/sgx_sign_test.go
package vm

import (
    "testing"
    
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/crypto"
)

func TestSignAndVerify(t *testing.T) {
    keyStore, _ := NewEncryptedKeyStore("/tmp/test_keys", "/tmp/test_public")
    permMgr := NewInMemoryPermissionManager()
    
    // 创建密钥
    owner := common.HexToAddress("0x1234")
    keyID, _ := keyStore.CreateKey(owner, KeyTypeECDSA)
    
    // 授予签名权限
    permMgr.GrantPermission(keyID, Permission{
        Grantee: owner,
        Type:    PermissionSign,
    })
    
    // 签名
    signContract := &SGXSign{keyStore: keyStore, permissionManager: permMgr}
    ctx := &SGXContext{Caller: owner}
    
    hash := crypto.Keccak256([]byte("test message"))
    input := append(keyID.Bytes(), hash...)
    
    signature, err := signContract.RunWithContext(ctx, input)
    if err != nil {
        t.Fatalf("Sign failed: %v", err)
    }
    
    // 验证
    verifyContract := &SGXVerify{}
    pubKey, _ := keyStore.GetPublicKey(keyID)
    
    verifyInput := append(hash, signature...)
    verifyInput = append(verifyInput, pubKey[1:]...) // 去掉 0x04 前缀
    
    result, err := verifyContract.Run(verifyInput)
    if err != nil {
        t.Fatalf("Verify failed: %v", err)
    }
    
    if result[0] != 0x01 {
        t.Error("Signature verification failed")
    }
}
```

### 权限测试

```go
// core/vm/sgx_permission_test.go
package vm

import (
    "testing"
    "time"
    
    "github.com/ethereum/go-ethereum/common"
)

func TestPermissionGrant(t *testing.T) {
    pm := NewInMemoryPermissionManager()
    keyID := common.HexToHash("0x1234")
    grantee := common.HexToAddress("0x5678")
    
    // 授予权限
    pm.GrantPermission(keyID, Permission{
        Grantee: grantee,
        Type:    PermissionSign,
    })
    
    // 检查权限
    if !pm.CheckPermission(keyID, grantee, PermissionSign) {
        t.Error("Permission should be granted")
    }
    
    // 检查未授予的权限
    if pm.CheckPermission(keyID, grantee, PermissionDecrypt) {
        t.Error("Decrypt permission should not be granted")
    }
}

func TestPermissionExpiry(t *testing.T) {
    pm := NewInMemoryPermissionManager()
    keyID := common.HexToHash("0x1234")
    grantee := common.HexToAddress("0x5678")
    
    // 授予已过期的权限
    pm.GrantPermission(keyID, Permission{
        Grantee:   grantee,
        Type:      PermissionSign,
        ExpiresAt: uint64(time.Now().Add(-1 * time.Hour).Unix()),
    })
    
    // 检查权限（应该失败）
    if pm.CheckPermission(keyID, grantee, PermissionSign) {
        t.Error("Expired permission should not be valid")
    }
}

func TestPermissionMaxUses(t *testing.T) {
    pm := NewInMemoryPermissionManager()
    keyID := common.HexToHash("0x1234")
    grantee := common.HexToAddress("0x5678")
    
    // 授予限制使用次数的权限
    pm.GrantPermission(keyID, Permission{
        Grantee:  grantee,
        Type:     PermissionSign,
        MaxUses:  2,
        UsedCount: 0,
    })
    
    // 使用两次
    pm.UsePermission(keyID, grantee, PermissionSign)
    pm.UsePermission(keyID, grantee, PermissionSign)
    
    // 第三次应该失败
    if pm.CheckPermission(keyID, grantee, PermissionSign) {
        t.Error("Permission should be exhausted")
    }
}
```

## 配置参数

```toml
# config.toml
[precompile.sgx]
# 加密分区路径（存储私钥）
encrypted_path = "/data/keys"

# 普通路径（存储公钥和元数据）
public_path = "/app/public"

# Gas 配置
[precompile.sgx.gas]
key_create = 100000
key_get_public = 3000
sign = 10000
verify = 3000
ecdh = 15000
random_base = 100
random_per_byte = 10
encrypt_base = 5000
encrypt_per_byte = 10
decrypt_base = 5000
decrypt_per_byte = 10
key_derive = 50000

# 限制
[precompile.sgx.limits]
max_random_length = 1024
max_plaintext_length = 65536
max_keys_per_address = 1000
```

## 实现优先级

| 优先级 | 功能 | 预计工时 |
|--------|------|----------|
| P0 | 密钥存储接口和实现 | 3 天 |
| P0 | SGX_KEY_CREATE | 2 天 |
| P0 | SGX_KEY_GET_PUBLIC | 1 天 |
| P0 | SGX_SIGN | 2 天 |
| P0 | SGX_VERIFY | 1 天 |
| P1 | 权限管理系统 | 3 天 |
| P1 | SGX_ECDH | 2 天 |
| P1 | SGX_ENCRYPT/DECRYPT | 3 天 |
| P2 | SGX_RANDOM | 1 天 |
| P2 | SGX_KEY_DERIVE | 2 天 |

**总计：约 3 周**

## 注意事项

1. **私钥安全**：私钥必须存储在加密分区，永不以明文形式离开 enclave
2. **权限检查**：所有敏感操作必须先检查权限
3. **Gas 计算**：确保 Gas 计算合理，防止 DoS 攻击
4. **常量时间**：密码学操作使用常量时间实现，防止侧信道攻击
5. **输入验证**：严格验证所有输入，防止缓冲区溢出
