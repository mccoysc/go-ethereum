# SGX 证明模块实现差距分析

## 概述

当前实现与架构文档及模块设计文档要求存在重大差距，需要进行重构以完全满足规范要求。

## 关键问题

### 1. 未使用 Gramine 原生 RA-TLS 库 ⚠️ **CRITICAL**

**文档要求：**
> **重要说明**：RA-TLS 证书生成和验证功能应直接使用原生 Gramine 项目的 ra-tls 实现（https://github.com/gramineproject/gramine 的 `tools/sgx/ra-tls/` 目录），而不是自行实现。

**当前实现：**
- `attestor_impl.go`: 使用 Go 的 `crypto/x509` 自行实现证书生成
- 未调用 Gramine 的 `ra_tls_create_key_and_crt_der()` 函数
- 未使用 `ra_tls_verify_callback_der()` 进行验证

**应该的实现：**
```go
// 应通过 CGO 调用 Gramine 的 C 函数
/*
#cgo LDFLAGS: -lra_tls_attest -lra_tls_verify
#include <ra_tls.h>

int ra_tls_create_key_and_crt_der(uint8_t** der_key, size_t* der_key_size,
                                    uint8_t** der_crt, size_t* der_crt_size);
int ra_tls_verify_callback_der(uint8_t* der_crt, size_t der_crt_size);
void ra_tls_set_measurement_callback(verify_measurements_cb_t f_cb);
*/
import "C"
```

**影响：**
- 当前实现无法在真实 SGX 环境中生成有效的 RA-TLS 证书
- 证书不包含由 Intel 签名的真实 Quote
- 无法通过其他节点的验证

---

## 缺失的组件

### 2. RATLSEnvManager - 环境变量管理器

**文档要求：**
- 实现 `internal/sgx/env_manager.go`
- 从链上 SecurityConfigContract 动态读取安全参数
- 管理 RA-TLS 相关环境变量

**缺失的功能：**
```go
type RATLSEnvManager struct {
    securityConfigContract common.Address
    client                 *ethclient.Client
}

func (m *RATLSEnvManager) InitFromContract() error
func (m *RATLSEnvManager) StartPeriodicRefresh(refreshInterval time.Duration)
func (m *RATLSEnvManager) setupMeasurementCallback(allowedMREnclaves []string, allowedMRSigners []string)
```

**影响：**
- 无法从链上合约读取 MRENCLAVE/MRSIGNER 白名单
- 无法实现动态安全参数更新
- 缺少治理机制集成

---

### 3. Instance ID 提取功能

**文档要求：**
```go
// 从 SGX Quote 中提取 Instance ID（硬件唯一标识）
func ExtractInstanceID(quote []byte) (string, error) {
    // 从 Quote 中提取 EPID 或 DCAP 硬件标识
    // 该标识对于每个物理 SGX CPU 是唯一的
}
```

**用途：**
- 确保每个物理 CPU 只能注册一个验证者节点
- 防止同一硬件运行多个节点进行女巫攻击
- 在引导阶段用于区分不同的创始管理者

**当前状态：** 未实现

---

### 4. CGO 集成 Gramine 库

**文档要求：**
- 链接 `libra_tls_attest.so` 和 `libra_tls_verify.so`
- 链接 `libsgx_dcap_ql.so` 用于 Quote 验证
- 通过 CGO 调用 C 函数

**当前状态：** 
- 完全使用 Go 实现，无 CGO 代码
- 无法调用真实的 SGX/Gramine 函数

---

### 5. 证书算法不符合规范

**文档要求：**
> Gramine 的 `ra_tls_create_key_and_crt_der()` 函数使用 NIST P-384 (SECP384R1) 椭圆曲线生成密钥对

**当前实现：**
```go
// attestor_impl.go 第 45 行
privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
```
使用 P-256 而非文档要求的 P-384

---

## 已实现但需要调整的部分

### 6. Quote 结构解析

**当前实现：** ✓ 基本满足
- `quote.go` 正确定义了 SGXQuote 结构
- `ParseQuote()` 可以解析 Quote

**需要调整：**
- 添加 Instance ID 字段和提取逻辑
- 完善 TCB 状态解析（当前简化为固定值）

---

### 7. 常量时间操作

**当前实现：** ✓ 满足要求
- `constant_time.go` 实现了侧信道防护
- 包含时序测试验证

---

### 8. Mock 实现

**当前实现：** ✓ 满足要求
- `mock_attestor.go` 提供了非 SGX 环境的测试支持
- 可以在 CI/CD 中运行

---

## 文件结构对比

### 文档要求的结构：
```
internal/sgx/
├── attestor.go           # Attestor 接口定义
├── attestor_impl.go      # Gramine Attestor 实现 (CGO)
├── verifier.go           # Verifier 接口定义
├── verifier_impl.go      # DCAP Verifier 实现 (CGO)
├── quote.go              # Quote 解析和数据结构
├── constant_time.go      # 常量时间操作
├── constant_time_test.go # 常量时间测试
└── sidechannel_test.go   # 侧信道防护测试
```

### 当前实现的结构：
```
internal/sgx/
├── attestor.go           # ✓ 接口定义
├── attestor_impl.go      # ✗ 自定义实现（应该用 CGO 调用 Gramine）
├── verifier.go           # ✓ 接口定义
├── verifier_impl.go      # ✗ 自定义实现（应该用 CGO 调用 DCAP）
├── quote.go              # ✓ Quote 解析
├── constant_time.go      # ✓ 常量时间操作
├── constant_time_test.go # ✓ 常量时间测试
├── attestor_test.go      # 额外添加
├── verifier_test.go      # 额外添加
├── quote_test.go         # 额外添加
├── mock_attestor.go      # 额外添加（有用）
├── example_test.go       # 额外添加
└── README.md             # 额外添加

缺失：
├── env_manager.go        # ✗ 环境变量管理器
└── sidechannel_test.go   # ✗ 侧信道防护测试（已有 constant_time_test.go）
```

---

## 重构建议

### 阶段 1: 核心 CGO 集成（P0）
1. 创建 `attestor_cgo.go` 实现 CGO 调用
2. 调用 `ra_tls_create_key_and_crt_der()`
3. 调用 `ra_tls_verify_callback_der()`
4. 修改密钥算法为 P-384

### 阶段 2: 环境变量管理（P0）
1. 实现 `env_manager.go`
2. 集成链上合约读取
3. 实现回调函数机制

### 阶段 3: Instance ID 支持（P1）
1. 在 `quote.go` 添加 Instance ID 提取
2. 更新 Quote 结构体

### 阶段 4: 完善测试（P1）
1. 添加 CGO 模拟测试
2. 完善 TCB 验证测试

---

## 总结

当前实现**约 40% 符合规范**：
- ✓ 接口定义正确
- ✓ 常量时间操作完整
- ✓ Mock 支持良好
- ✗ **未使用 Gramine 原生库（关键问题）**
- ✗ 缺少环境变量管理器
- ✗ 缺少 Instance ID 支持
- ✗ 证书算法不符合规范

**风险等级：** 🔴 **高风险**
- 当前代码无法在真实 SGX 环境中工作
- 无法与其他符合规范的节点互操作

**建议：** 需要进行重大重构，特别是 CGO 集成部分。
