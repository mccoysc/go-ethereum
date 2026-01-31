# SGX 证明模块开发文档

## 模块概述

SGX 证明模块是 X Chain 安全基础设施的核心组件，负责实现 Intel SGX 远程证明功能，确保节点运行在可信执行环境中，并验证节点代码的完整性。

**重要说明**：RA-TLS 证书生成和验证功能应直接使用原生 Gramine 项目的 ra-tls 实现（https://github.com/gramineproject/gramine 的 `tools/sgx/ra-tls/` 目录），而不是自行实现。Gramine 的 RA-TLS 库提供了完整的证书生成（`ra_tls_create_key_and_crt_der`）和验证（`ra_tls_verify_callback_der`）功能。

## 负责团队

**安全/SGX 团队**

## 模块职责

1. 集成 Gramine RA-TLS 库进行证书生成和验证
2. 管理 MRENCLAVE/MRSIGNER 白名单
3. 配置 RA-TLS 环境变量
4. 实现侧信道攻击防护

## 依赖关系

```
+------------------+
|  SGX 证明模块    |
+------------------+
        |
        v
+---------------------------+
|  Gramine RA-TLS 库        |
|  (libra_tls_attest.so)    |
|  (libra_tls_verify.so)    |
+---------------------------+
        |
        v
+------------------+
|  Gramine 运行时  |
+------------------+
        |
        v
+------------------+
|  Intel SGX DCAP  |
+------------------+
```

### 上游依赖
- Gramine LibOS（提供 `/dev/attestation` 接口）
- Gramine RA-TLS 库（证书生成和验证）
- Intel SGX DCAP 库（Quote 验证）
- mbedTLS（密码学操作）

### 下游依赖（被以下模块使用）
- P2P 网络模块（RA-TLS 握手）
- 共识引擎模块（区块验证）
- 治理模块（白名单管理）

## Gramine RA-TLS 库使用

RA-TLS 实现来自原生 Gramine 项目：https://github.com/gramineproject/gramine/tree/master/tools/sgx/ra-tls

### RA-TLS 核心 API

Gramine 的 RA-TLS 库提供以下核心函数（定义在 `ra_tls.h`）：

```c
// 证书生成（Attester 端）
int ra_tls_create_key_and_crt_der(
    uint8_t** der_key,       // 输出：DER 格式私钥
    size_t* der_key_size,    // 输出：私钥大小
    uint8_t** der_crt,       // 输出：DER 格式证书（嵌入 SGX Quote）
    size_t* der_crt_size     // 输出：证书大小
);

// 证书验证（Verifier 端）
int ra_tls_verify_callback_der(
    uint8_t* der_crt,        // 输入：DER 格式证书
    size_t der_crt_size      // 输入：证书大小
);

// 设置自定义度量值验证回调
void ra_tls_set_measurement_callback(verify_measurements_cb_t f_cb);
```

### 证书算法说明

**官方 Gramine RA-TLS 限制**：官方 Gramine 的 `ra_tls_create_key_and_crt_der()` 函数固定使用 **NIST P-384 (SECP384R1)** 椭圆曲线生成密钥对，不支持配置其他算法。

```c
// 官方 Gramine ra_tls_create_key_and_crt_der() 说明（摘自 ra_tls.h）：
// "The function first generates a random ECDSA keypair with NIST P-384 (SECP384R1) elliptic curve."
```

**以太坊兼容性说明**：由于以太坊使用 secp256k1 曲线，而官方 Gramine RA-TLS 使用 SECP384R1，X Chain 需要在应用层处理密钥转换：

1. **RA-TLS 证书**：使用官方 Gramine 生成的 SECP384R1 证书进行节点间 TLS 通信和 SGX 远程证明
2. **以太坊地址**：在应用层单独生成 secp256k1 密钥对用于以太坊交易签名
3. **密钥绑定**：通过 SGX Quote 的 report_data 字段绑定两个密钥（将以太坊公钥哈希嵌入 Quote）

```go
// 密钥绑定示例
type NodeKeys struct {
    // RA-TLS 密钥（SECP384R1，由 Gramine 生成）
    RATLSCert *tls.Certificate
    
    // 以太坊密钥（secp256k1，应用层生成）
    EthPrivateKey *ecdsa.PrivateKey
    EthAddress    common.Address
}

// 在 SGX Quote 中绑定以太坊公钥
func (n *NodeKeys) GenerateQuoteWithEthBinding() ([]byte, error) {
    // 计算以太坊公钥哈希作为 report_data
    ethPubKeyHash := crypto.Keccak256(crypto.FromECDSAPub(&n.EthPrivateKey.PublicKey))
    
    // 生成包含以太坊公钥哈希的 SGX Quote
    return generateQuote(ethPubKeyHash[:64])
}
```

### 安全参数配置架构

X Chain 的安全参数分为两类：

| 类别 | 存储位置 | 特点 |
|------|----------|------|
| **Manifest 固定参数** | Gramine Manifest | 影响 MRENCLAVE，不可篡改 |
| **链上安全参数** | 链上合约 | 通过投票管理，动态生效 |

#### Manifest 固定参数

Manifest 中只存储本地配置和**链上合约地址**。合约地址写死在 manifest 中，作为安全锚点：

```toml
# Gramine manifest 中的固定参数
[loader.env]
# 链上合约地址（写死，作为安全锚点）
# 合约地址影响 MRENCLAVE，攻击者无法修改合约地址而不改变度量值
XCHAIN_SECURITY_CONFIG_CONTRACT = "0xabcdef1234567890abcdef1234567890abcdef12"
XCHAIN_GOVERNANCE_CONTRACT = "0x1234567890abcdef1234567890abcdef12345678"
```

#### 链上安全参数（动态读取）

所有治理相关的安全参数从链上合约动态读取，投票结果实时生效：

| 参数 | 链上合约 | 说明 |
|------|----------|------|
| MRENCLAVE 白名单 | SecurityConfigContract | 允许的 enclave 代码度量值 |
| MRSIGNER 白名单 | SecurityConfigContract | 允许的签名者度量值 |
| 密钥迁移阈值 | SecurityConfigContract | 密钥迁移所需的最小节点数 |
| 节点准入策略 | SecurityConfigContract | 是否严格验证 Quote |
| 分叉配置 | SecurityConfigContract | 硬分叉升级相关配置 |
| 数据迁移策略 | SecurityConfigContract | 加密数据迁移相关配置 |

**合约职责划分**：
- **安全配置合约（SecurityConfigContract）**：存储所有安全配置，被其他模块读取
- **治理合约（GovernanceContract）**：负责投票、管理投票人（有效性、合法性）、把投票结果写入安全配置合约

```go
// 从链上读取安全参数
type OnChainSecurityConfig struct {
    whitelistContract  common.Address  // 从 Manifest 读取
    governanceContract common.Address  // 从 Manifest 读取
    client             *ethclient.Client
    localCache         *SecurityCache
}

func NewOnChainSecurityConfig() (*OnChainSecurityConfig, error) {
    // 从 Manifest 环境变量读取合约地址（写死的安全锚点）
    scAddr := os.Getenv("XCHAIN_SECURITY_CONFIG_CONTRACT")
    govAddr := os.Getenv("XCHAIN_GOVERNANCE_CONTRACT")
    
    return &OnChainSecurityConfig{
        securityConfigContract: common.HexToAddress(scAddr), // 安全配置合约，由治理合约管理
        governanceContract:     common.HexToAddress(govAddr),
    }, nil
}

// SyncFromChain 从链上同步所有安全参数
func (c *OnChainSecurityConfig) SyncFromChain() error {
    // 从安全配置合约读取（由治理合约管理）
    c.localCache.AllowedMREnclave = c.fetchWhitelist()
    
    // 从治理合约读取
    c.localCache.KeyMigrationThreshold = c.fetchKeyMigrationThreshold()
    c.localCache.AdmissionStrict = c.fetchAdmissionPolicy()
    
    return nil
}

// IsAllowedMREnclave 验证时使用本地缓存
func (c *OnChainSecurityConfig) IsAllowedMREnclave(mrenclave []byte) bool {
    key := fmt.Sprintf("%x", mrenclave)
    return c.localCache.AllowedMREnclave[key]
}
```

**安全保证**：
- 合约地址写死在 Manifest 中，影响 MRENCLAVE，无法被篡改
- 所有安全参数从链上读取，通过共识机制保证一致性
- 投票结果记录在链上，不可篡改
- 节点定期从链上同步参数，确保使用最新的治理决策
- 本节点的 MRENCLAVE 由代码决定，无法伪造
- 其他节点的 MRENCLAVE 通过 SGX Quote 验证，由 Intel 签名保证真实性

### RA-TLS 环境变量管理（动态从合约读取）

**重要安全原则**：所有 RA-TLS 安全相关环境变量**禁止**从静态配置读取，必须在 geth 启动时从链上合约读取，然后动态设置/覆盖进程的环境变量。

#### Manifest 中禁止配置的环境变量

以下环境变量属于安全相关配置，**禁止**在 Manifest 中静态配置：

| 环境变量 | 说明 | 来源 |
|----------|------|------|
| `RA_TLS_MRENCLAVE` | 期望的 MRENCLAVE 值 | 从 SecurityConfigContract 读取 |
| `RA_TLS_MRSIGNER` | 期望的 MRSIGNER 值 | 从 SecurityConfigContract 读取 |
| `RA_TLS_ISV_PROD_ID` | 期望的 ISV 产品 ID | 从 SecurityConfigContract 读取 |
| `RA_TLS_ISV_SVN` | 期望的 ISV 安全版本号 | 从 SecurityConfigContract 读取 |
| `RA_TLS_ALLOW_OUTDATED_TCB_INSECURE` | 允许过期 TCB | 从 SecurityConfigContract 读取 |
| `RA_TLS_ALLOW_HW_CONFIG_NEEDED` | 允许硬件配置需要更新 | 从 SecurityConfigContract 读取 |
| `RA_TLS_ALLOW_SW_HARDENING_NEEDED` | 允许软件加固需要 | 从 SecurityConfigContract 读取 |
| `RA_TLS_ALLOW_DEBUG_ENCLAVE_INSECURE` | 允许调试 enclave | 从 SecurityConfigContract 读取 |
| `RA_TLS_CERT_TIMESTAMP_NOT_BEFORE` | 证书有效期开始 | 从 SecurityConfigContract 读取 |
| `RA_TLS_CERT_TIMESTAMP_NOT_AFTER` | 证书有效期结束 | 从 SecurityConfigContract 读取 |

#### Manifest 中允许配置的环境变量

只有合约地址可以写死在 Manifest 中（作为安全锚点，影响 MRENCLAVE）：

| 环境变量 | 说明 |
|----------|------|
| `XCHAIN_SECURITY_CONFIG_CONTRACT` | 安全配置合约地址（写死） |
| `XCHAIN_GOVERNANCE_CONTRACT` | 治理合约地址（写死） |

#### 启动时环境变量动态设置流程

geth 启动时必须执行以下流程：

```go
// internal/sgx/env_manager.go
package sgx

import (
    "os"
    "github.com/ethereum/go-ethereum/common"
)

// RATLSEnvManager 管理 RA-TLS 环境变量
type RATLSEnvManager struct {
    securityConfigContract common.Address
    client                 *ethclient.Client
}

// 需要从合约读取并设置的环境变量列表
var securityEnvVars = []string{
    "RA_TLS_MRENCLAVE",
    "RA_TLS_MRSIGNER",
    "RA_TLS_ISV_PROD_ID",
    "RA_TLS_ISV_SVN",
    "RA_TLS_ALLOW_OUTDATED_TCB_INSECURE",
    "RA_TLS_ALLOW_HW_CONFIG_NEEDED",
    "RA_TLS_ALLOW_SW_HARDENING_NEEDED",
    "RA_TLS_ALLOW_DEBUG_ENCLAVE_INSECURE",
    "RA_TLS_CERT_TIMESTAMP_NOT_BEFORE",
    "RA_TLS_CERT_TIMESTAMP_NOT_AFTER",
}

// InitFromContract 从合约读取安全参数并设置环境变量
func (m *RATLSEnvManager) InitFromContract() error {
    // 1. 先清除所有安全相关环境变量（防止静态配置被使用）
    for _, envVar := range securityEnvVars {
        os.Unsetenv(envVar)
    }
    
    // 2. 从链上合约读取安全参数
    config, err := m.fetchSecurityConfig()
    if err != nil {
        return fmt.Errorf("failed to fetch security config from contract: %w", err)
    }
    
    // 3. 设置环境变量（覆盖任何可能的静态配置）
    for _, mrenclave := range config.AllowedMREnclave {
        // 注意：官方 Gramine 只支持单个 MRENCLAVE 值
        // 如需支持多个，需要使用 ra_tls_set_measurement_callback
        os.Setenv("RA_TLS_MRENCLAVE", mrenclave)
        break // 只设置第一个，其余通过回调验证
    }
    
    if config.AllowOutdatedTCB {
        os.Setenv("RA_TLS_ALLOW_OUTDATED_TCB_INSECURE", "1")
    }
    
    // ... 设置其他环境变量
    
    return nil
}

// fetchSecurityConfig 从 SecurityConfigContract 读取配置
func (m *RATLSEnvManager) fetchSecurityConfig() (*SecurityConfig, error) {
    // 调用合约读取安全配置
    // ...
}
```

#### 多 MRENCLAVE 白名单支持

由于官方 Gramine 的 `RA_TLS_MRENCLAVE` 环境变量只支持单个值，X Chain 需要使用自定义回调函数支持多个 MRENCLAVE：

```go
// 使用 ra_tls_set_measurement_callback 支持多 MRENCLAVE 白名单
func setupMeasurementCallback(allowedMREnclaves []string) {
    // 注册自定义验证回调
    // 回调函数签名（官方 Gramine）：
    // int (*verify_measurements_cb_t)(const char* mrenclave, const char* mrsigner,
    //                                  const char* isv_prod_id, const char* isv_svn);
    
    // 在回调中检查 mrenclave 是否在白名单中
    // 白名单数据来自 SecurityConfigContract
}
```

**重要说明**：
- 官方 Gramine 不支持 `RA_TLS_CERT_ALGORITHM` 或 `RA_TLS_CERT_CONFIG_B64` 环境变量
- 证书算法固定为 SECP384R1，无法通过环境变量配置
- 所有安全参数必须从链上合约动态读取，禁止静态配置

### 证书和私钥存储

根据安全要求：
- **证书**：可以存储在普通目录
- **私钥**：必须存储在加密分区

```toml
# Gramine manifest 配置
[[fs.mounts]]
type = "encrypted"
path = "/app/keys"           # 私钥存储路径（加密分区）
uri = "file:/data/keys"
key_name = "_sgx_mrenclave"

[[fs.mounts]]
type = "chroot"
path = "/app/certs"          # 证书存储路径（普通目录）
uri = "file:/data/certs"
```

## 核心接口定义

### Attestor 接口

```go
// internal/sgx/attestor.go
package sgx

import (
    "crypto/tls"
)

// Attestor SGX 证明器接口
type Attestor interface {
    // GenerateQuote 生成 SGX Quote
    // reportData: 用户自定义数据（通常是公钥哈希），最大 64 字节
    // 返回: SGX Quote 二进制数据
    GenerateQuote(reportData []byte) ([]byte, error)
    
    // GenerateCertificate 生成 RA-TLS 证书
    // 证书中嵌入 SGX Quote，用于 TLS 握手时的远程证明
    GenerateCertificate() (*tls.Certificate, error)
    
    // GetMREnclave 获取本地 enclave 的 MRENCLAVE
    // MRENCLAVE 是 enclave 代码和初始数据的 SHA256 哈希
    GetMREnclave() []byte
    
    // GetMRSigner 获取本地 enclave 的 MRSIGNER
    // MRSIGNER 是签名者公钥的哈希
    GetMRSigner() []byte
}
```

### Verifier 接口

```go
// internal/sgx/verifier.go
package sgx

import (
    "crypto/x509"
)

// Verifier SGX Quote 验证器接口
type Verifier interface {
    // VerifyQuote 验证 SGX Quote 的有效性
    // 包括签名验证、TCB 状态检查等
    VerifyQuote(quote []byte) error
    
    // VerifyCertificate 验证 RA-TLS 证书
    // 从证书中提取 Quote 并验证
    VerifyCertificate(cert *x509.Certificate) error
    
    // IsAllowedMREnclave 检查 MRENCLAVE 是否在白名单中
    IsAllowedMREnclave(mrenclave []byte) bool
    
    // AddAllowedMREnclave 添加 MRENCLAVE 到白名单
    AddAllowedMREnclave(mrenclave []byte)
    
    // RemoveAllowedMREnclave 从白名单移除 MRENCLAVE
    RemoveAllowedMREnclave(mrenclave []byte)
}
```

## 关键数据结构

### SGX Quote 结构

```go
// SGXQuote SGX Quote 数据结构
type SGXQuote struct {
    Version       uint16   // Quote 版本
    SignType      uint16   // 签名类型 (EPID/DCAP)
    MRENCLAVE     [32]byte // Enclave 代码度量值
    MRSIGNER      [32]byte // 签名者度量值
    ISVProdID     uint16   // 产品 ID
    ISVSVN        uint16   // 安全版本号
    ReportData    [64]byte // 用户自定义数据
    TCBStatus     uint8    // TCB 状态
    Signature     []byte   // Quote 签名
}

// TCB 状态常量
const (
    TCB_UP_TO_DATE          uint8 = 0x00
    TCB_OUT_OF_DATE         uint8 = 0x01
    TCB_REVOKED             uint8 = 0x02
    TCB_CONFIGURATION_NEEDED uint8 = 0x03
)
```

## 实现指南

### 1. Quote 生成实现

通过 Gramine 提供的 `/dev/attestation` 接口生成 Quote：

```go
// internal/sgx/attestor_impl.go
package sgx

import (
    "crypto/ecdsa"
    "crypto/elliptic"
    "crypto/rand"
    "crypto/tls"
    "crypto/x509"
    "crypto/x509/pkix"
    "encoding/binary"
    "fmt"
    "math/big"
    "os"
    "time"
)

type GramineAttestor struct {
    privateKey *ecdsa.PrivateKey
    mrenclave  []byte
    mrsigner   []byte
}

func NewGramineAttestor() (*GramineAttestor, error) {
    // 生成 TLS 密钥对
    privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    if err != nil {
        return nil, fmt.Errorf("failed to generate key: %w", err)
    }
    
    attestor := &GramineAttestor{
        privateKey: privateKey,
    }
    
    // 读取本地 MRENCLAVE
    mrenclave, err := os.ReadFile("/dev/attestation/my_target_info")
    if err != nil {
        return nil, fmt.Errorf("failed to read MRENCLAVE: %w", err)
    }
    attestor.mrenclave = mrenclave[:32]
    
    return attestor, nil
}

func (a *GramineAttestor) GenerateQuote(reportData []byte) ([]byte, error) {
    // 1. 写入 user_report_data
    if len(reportData) > 64 {
        return nil, fmt.Errorf("reportData too long: max 64 bytes")
    }
    
    paddedData := make([]byte, 64)
    copy(paddedData, reportData)
    
    err := os.WriteFile("/dev/attestation/user_report_data", paddedData, 0600)
    if err != nil {
        return nil, fmt.Errorf("failed to write user_report_data: %w", err)
    }
    
    // 2. 读取 Quote
    quote, err := os.ReadFile("/dev/attestation/quote")
    if err != nil {
        return nil, fmt.Errorf("failed to read quote: %w", err)
    }
    
    return quote, nil
}

func (a *GramineAttestor) GenerateCertificate() (*tls.Certificate, error) {
    // 1. 生成公钥哈希作为 reportData
    pubKeyBytes := elliptic.Marshal(a.privateKey.Curve, a.privateKey.X, a.privateKey.Y)
    
    // 2. 生成包含公钥哈希的 Quote
    quote, err := a.GenerateQuote(pubKeyBytes[:64])
    if err != nil {
        return nil, err
    }
    
    // 3. 创建自签名证书，将 Quote 嵌入扩展字段
    template := &x509.Certificate{
        SerialNumber: big.NewInt(1),
        Subject: pkix.Name{
            CommonName: "X-Chain-Node",
        },
        NotBefore: time.Now(),
        NotAfter:  time.Now().Add(24 * time.Hour),
        KeyUsage:  x509.KeyUsageDigitalSignature,
        ExtraExtensions: []pkix.Extension{
            {
                Id:       SGXQuoteOID, // 自定义 OID
                Critical: false,
                Value:    quote,
            },
        },
    }
    
    certDER, err := x509.CreateCertificate(rand.Reader, template, template, 
        &a.privateKey.PublicKey, a.privateKey)
    if err != nil {
        return nil, err
    }
    
    return &tls.Certificate{
        Certificate: [][]byte{certDER},
        PrivateKey:  a.privateKey,
    }, nil
}

func (a *GramineAttestor) GetMREnclave() []byte {
    return a.mrenclave
}

func (a *GramineAttestor) GetMRSigner() []byte {
    return a.mrsigner
}

// SGXQuoteOID 是 SGX Quote 在 X.509 证书中的 OID
var SGXQuoteOID = asn1.ObjectIdentifier{1, 2, 840, 113741, 1, 13, 1}
```

### 2. Quote 验证实现

```go
// internal/sgx/verifier_impl.go
package sgx

import (
    "bytes"
    "crypto/x509"
    "encoding/binary"
    "errors"
    "sync"
)

type DCAPVerifier struct {
    mu              sync.RWMutex
    allowedMREnclave map[string]bool
    allowedMRSigner  map[string]bool
    allowOutdatedTCB bool
}

func NewDCAPVerifier(allowOutdatedTCB bool) *DCAPVerifier {
    return &DCAPVerifier{
        allowedMREnclave: make(map[string]bool),
        allowedMRSigner:  make(map[string]bool),
        allowOutdatedTCB: allowOutdatedTCB,
    }
}

func (v *DCAPVerifier) VerifyQuote(quote []byte) error {
    // 1. 解析 Quote 结构
    parsedQuote, err := parseQuote(quote)
    if err != nil {
        return fmt.Errorf("failed to parse quote: %w", err)
    }
    
    // 2. 验证 Quote 签名（调用 DCAP 库）
    if err := v.verifyQuoteSignature(quote); err != nil {
        return fmt.Errorf("quote signature verification failed: %w", err)
    }
    
    // 3. 检查 TCB 状态
    if !v.allowOutdatedTCB && parsedQuote.TCBStatus != TCB_UP_TO_DATE {
        return fmt.Errorf("TCB status not up to date: %d", parsedQuote.TCBStatus)
    }
    
    // 4. 检查 MRENCLAVE 白名单
    if !v.IsAllowedMREnclave(parsedQuote.MRENCLAVE[:]) {
        return fmt.Errorf("MRENCLAVE not in allowed list: %x", parsedQuote.MRENCLAVE)
    }
    
    return nil
}

func (v *DCAPVerifier) VerifyCertificate(cert *x509.Certificate) error {
    // 1. 从证书扩展中提取 Quote
    var quote []byte
    for _, ext := range cert.Extensions {
        if ext.Id.Equal(SGXQuoteOID) {
            quote = ext.Value
            break
        }
    }
    
    if quote == nil {
        return errors.New("no SGX quote found in certificate")
    }
    
    // 2. 验证 Quote
    if err := v.VerifyQuote(quote); err != nil {
        return err
    }
    
    // 3. 验证证书公钥与 Quote 中的 reportData 匹配
    parsedQuote, _ := parseQuote(quote)
    pubKeyBytes := elliptic.Marshal(cert.PublicKey.(*ecdsa.PublicKey).Curve,
        cert.PublicKey.(*ecdsa.PublicKey).X,
        cert.PublicKey.(*ecdsa.PublicKey).Y)
    
    if !bytes.Equal(parsedQuote.ReportData[:len(pubKeyBytes)], pubKeyBytes) {
        return errors.New("certificate public key does not match quote reportData")
    }
    
    return nil
}

func (v *DCAPVerifier) IsAllowedMREnclave(mrenclave []byte) bool {
    v.mu.RLock()
    defer v.mu.RUnlock()
    return v.allowedMREnclave[string(mrenclave)]
}

func (v *DCAPVerifier) AddAllowedMREnclave(mrenclave []byte) {
    v.mu.Lock()
    defer v.mu.Unlock()
    v.allowedMREnclave[string(mrenclave)] = true
}

func (v *DCAPVerifier) RemoveAllowedMREnclave(mrenclave []byte) {
    v.mu.Lock()
    defer v.mu.Unlock()
    delete(v.allowedMREnclave, string(mrenclave))
}

func (v *DCAPVerifier) verifyQuoteSignature(quote []byte) error {
    // 调用 Intel DCAP 库验证签名
    // 这里需要链接 libsgx_dcap_ql 库
    // TODO: 实现 CGO 调用
    return nil
}

func parseQuote(quote []byte) (*SGXQuote, error) {
    if len(quote) < 432 {
        return nil, errors.New("quote too short")
    }
    
    q := &SGXQuote{}
    q.Version = binary.LittleEndian.Uint16(quote[0:2])
    q.SignType = binary.LittleEndian.Uint16(quote[2:4])
    copy(q.MRENCLAVE[:], quote[112:144])
    copy(q.MRSIGNER[:], quote[176:208])
    q.ISVProdID = binary.LittleEndian.Uint16(quote[304:306])
    q.ISVSVN = binary.LittleEndian.Uint16(quote[306:308])
    copy(q.ReportData[:], quote[368:432])
    
    return q, nil
}
```

## 侧信道攻击防护

SGX 证明模块必须实现侧信道攻击防护，特别是在处理敏感数据时。

### 常量时间操作

```go
// internal/sgx/constant_time.go
package sgx

import (
    "crypto/subtle"
)

// ConstantTimeCompare 常量时间比较
// 无论输入是否相等，执行时间相同
func ConstantTimeCompare(a, b []byte) bool {
    return subtle.ConstantTimeCompare(a, b) == 1
}

// ConstantTimeCopy 常量时间条件复制
// 如果 condition 为 true，将 src 复制到 dst
func ConstantTimeCopy(condition bool, dst, src []byte) {
    mask := byte(0)
    if condition {
        mask = 0xFF
    }
    
    for i := range dst {
        dst[i] = (dst[i] & ^mask) | (src[i] & mask)
    }
}

// ConstantTimeSelect 常量时间选择
// 如果 condition 为 true，返回 a，否则返回 b
func ConstantTimeSelect(condition bool, a, b []byte) []byte {
    result := make([]byte, len(a))
    mask := byte(0)
    if condition {
        mask = 0xFF
    }
    
    for i := range result {
        result[i] = (a[i] & mask) | (b[i] & ^mask)
    }
    return result
}
```

### 防护检查清单

实现密码学操作时，必须检查以下项目：

- [ ] 所有比较操作使用常量时间函数
- [ ] 没有基于秘密数据的条件分支
- [ ] 没有使用秘密值作为数组索引
- [ ] 没有基于秘密数据的循环次数
- [ ] 使用经过审计的密码学库
- [ ] 输入/输出长度不泄露信息
- [ ] 错误处理不泄露时序信息
- [ ] 内存访问模式不依赖秘密数据

## 文件结构

```
internal/sgx/
├── attestor.go           # Attestor 接口定义
├── attestor_impl.go      # Gramine Attestor 实现
├── verifier.go           # Verifier 接口定义
├── verifier_impl.go      # DCAP Verifier 实现
├── quote.go              # Quote 解析和数据结构
├── constant_time.go      # 常量时间操作
├── constant_time_test.go # 常量时间测试
└── sidechannel_test.go   # 侧信道防护测试
```

## 单元测试指南

### 测试用例

```go
// internal/sgx/attestor_test.go
package sgx

import (
    "testing"
)

func TestGenerateQuote(t *testing.T) {
    // 注意：此测试需要在 SGX 环境中运行
    attestor, err := NewGramineAttestor()
    if err != nil {
        t.Skipf("SGX not available: %v", err)
    }
    
    reportData := []byte("test report data")
    quote, err := attestor.GenerateQuote(reportData)
    if err != nil {
        t.Fatalf("GenerateQuote failed: %v", err)
    }
    
    if len(quote) < 432 {
        t.Errorf("Quote too short: %d bytes", len(quote))
    }
}

func TestGenerateCertificate(t *testing.T) {
    attestor, err := NewGramineAttestor()
    if err != nil {
        t.Skipf("SGX not available: %v", err)
    }
    
    cert, err := attestor.GenerateCertificate()
    if err != nil {
        t.Fatalf("GenerateCertificate failed: %v", err)
    }
    
    if len(cert.Certificate) == 0 {
        t.Error("No certificate generated")
    }
}
```

### 常量时间测试

```go
// internal/sgx/constant_time_test.go
package sgx

import (
    "testing"
    "time"
)

func TestConstantTimeCompare(t *testing.T) {
    secret := []byte("secret_password_12345")
    
    inputs := [][]byte{
        []byte("wrong_password_12345"),  // 完全不同
        []byte("secret_password_12344"), // 最后一位不同
        []byte("aecret_password_12345"), // 第一位不同
        []byte("secret_password_12345"), // 完全相同
    }
    
    var times []time.Duration
    iterations := 10000
    
    for _, input := range inputs {
        start := time.Now()
        for i := 0; i < iterations; i++ {
            ConstantTimeCompare(input, secret)
        }
        times = append(times, time.Since(start))
    }
    
    // 验证所有执行时间在统计误差范围内
    avgTime := averageDuration(times)
    for i, d := range times {
        deviation := float64(d-avgTime) / float64(avgTime)
        if deviation > 0.05 { // 允许 5% 误差
            t.Errorf("Input %d has timing deviation: %.2f%%", i, deviation*100)
        }
    }
}

func averageDuration(durations []time.Duration) time.Duration {
    var total time.Duration
    for _, d := range durations {
        total += d
    }
    return total / time.Duration(len(durations))
}
```

### Mock 测试（非 SGX 环境）

```go
// internal/sgx/mock_attestor.go
package sgx

import (
    "crypto/ecdsa"
    "crypto/elliptic"
    "crypto/rand"
    "crypto/tls"
)

// MockAttestor 用于非 SGX 环境的测试
type MockAttestor struct {
    privateKey *ecdsa.PrivateKey
    mrenclave  []byte
    mrsigner   []byte
}

func NewMockAttestor() *MockAttestor {
    privateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    return &MockAttestor{
        privateKey: privateKey,
        mrenclave:  make([]byte, 32),
        mrsigner:   make([]byte, 32),
    }
}

func (m *MockAttestor) GenerateQuote(reportData []byte) ([]byte, error) {
    // 返回模拟的 Quote
    quote := make([]byte, 432)
    copy(quote[112:144], m.mrenclave)
    copy(quote[176:208], m.mrsigner)
    copy(quote[368:432], reportData)
    return quote, nil
}

func (m *MockAttestor) GenerateCertificate() (*tls.Certificate, error) {
    // 返回普通的自签名证书
    return nil, nil
}

func (m *MockAttestor) GetMREnclave() []byte {
    return m.mrenclave
}

func (m *MockAttestor) GetMRSigner() []byte {
    return m.mrsigner
}
```

## 配置参数

```toml
# config.toml
[sgx]
# 验证模式: "mrenclave" 或 "mrsigner"
verify_mode = "mrenclave"

# 允许的 MRENCLAVE 列表
mrenclave = [
    "abc123def456...",  # v1.0.0
    "789xyz...",        # v1.1.0
]

# 允许的 MRSIGNER 列表（如果使用 mrsigner 模式）
mrsigner = [
    "signer_hash...",
]

# 是否允许 TCB 过期
allow_outdated_tcb = false

# Quote 缓存时间（秒）
quote_cache_ttl = 3600
```

## 实现优先级

| 优先级 | 功能 | 预计工时 |
|--------|------|----------|
| P0 | Quote 生成（Gramine 集成） | 3 天 |
| P0 | Quote 验证（DCAP 集成） | 5 天 |
| P0 | MRENCLAVE 白名单管理 | 2 天 |
| P1 | RA-TLS 证书生成 | 3 天 |
| P1 | 侧信道防护实现 | 3 天 |
| P2 | Mock 测试框架 | 2 天 |

**总计：约 2 周**

## 注意事项

1. **硬件依赖**：此模块需要 Intel SGX 硬件支持，开发时可使用 Mock 实现
2. **DCAP 库链接**：需要正确配置 CGO 以链接 Intel DCAP 库
3. **Gramine 版本**：确保使用 Gramine 1.5+ 版本
4. **安全审计**：侧信道防护代码需要专业安全审计
