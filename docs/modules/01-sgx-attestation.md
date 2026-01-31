# SGX 证明模块开发文档

## 模块概述

SGX 证明模块是 X Chain 安全基础设施的核心组件，负责实现 Intel SGX 远程证明功能，确保节点运行在可信执行环境中，并验证节点代码的完整性。

## 负责团队

**安全/SGX 团队**

## 模块职责

1. 生成 SGX Quote（远程证明数据）
2. 验证其他节点的 SGX Quote
3. 管理 MRENCLAVE/MRSIGNER 白名单
4. 集成 Intel DCAP 库
5. 实现侧信道攻击防护

## 依赖关系

```
+------------------+
|  SGX 证明模块    |
+------------------+
        |
        v
+------------------+
|  Gramine 运行时  |
+------------------+
        |
        v
+------------------+
|  Intel SGX SDK   |
+------------------+
```

### 上游依赖
- Gramine LibOS（提供 `/dev/attestation` 接口）
- Intel SGX DCAP 库

### 下游依赖（被以下模块使用）
- P2P 网络模块（RA-TLS 握手）
- 共识引擎模块（区块验证）
- 治理模块（白名单管理）

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
