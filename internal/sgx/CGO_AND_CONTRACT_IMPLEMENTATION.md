# SGX 模块实现说明

## CGO 实现

### 概述

SGX 模块现在提供了真实的 CGO 绑定，调用 Gramine RA-TLS 库的原生 C 函数。

### 文件结构

- `attestor_ratls.go` - 非 CGO 构建的桩实现（用于测试）
- `attestor_ratls_cgo.go` - CGO 构建的实际实现（生产环境）
- `verifier_ratls.go` - 非 CGO 构建的桩实现（用于测试）
- `verifier_ratls_cgo.go` - CGO 构建的实际实现（生产环境）

### 构建方式

#### 测试/开发环境（无 CGO）

```bash
# CGO 默认禁用或未安装 Gramine 库时
CGO_ENABLED=0 go build ./internal/sgx/...
CGO_ENABLED=0 go test ./internal/sgx/...
```

这将使用桩实现，不需要 Gramine 库。

#### 生产环境（启用 CGO）

```bash
# 确保 Gramine RA-TLS 库已安装
export CGO_ENABLED=1
export CGO_CFLAGS="-I/path/to/gramine/include"
export CGO_LDFLAGS="-L/path/to/gramine/lib -lra_tls_attest -lra_tls_verify -lsgx_dcap_ql"

go build -tags cgo ./internal/sgx/...
```

这将使用实际的 C 函数绑定，调用 Gramine RA-TLS 库。

### CGO 实现细节

#### Attestor (attestor_ratls_cgo.go)

调用的 C 函数：
- `ra_tls_create_key_and_crt_der()` - 生成 RA-TLS 证书和密钥
- `ra_tls_free_key_and_crt_der()` - 释放 C 分配的内存

特性：
- 使用 P-384 椭圆曲线（符合 Gramine 规范）
- 自动内存管理（C 到 Go 的转换和释放）
- SGX 环境检测（非 SGX 环境返回测试值）

#### Verifier (verifier_ratls_cgo.go)

调用的 C 函数：
- `ra_tls_verify_callback_der()` - 验证 RA-TLS 证书
- `ra_tls_set_measurement_callback()` - 设置自定义度量值验证回调
- `custom_verify_measurements()` - C 回调函数实现

特性：
- 完整的密码学验证（包括 SGX Quote 签名）
- 支持 MRENCLAVE/MRSIGNER 白名单
- 自定义验证回调支持

---

## 合约调用实现

### 概述

环境变量管理器现在实现了真实的链上合约调用逻辑，通过条件编译支持测试模式和生产模式。

### 文件

- `contracts.go` - 合约 ABI 定义和调用实现
- `env_manager.go` - 使用合约调用获取安全配置

### 测试模式

通过环境变量 `SGX_TEST_MODE=true` 启用测试模式：

```bash
export SGX_TEST_MODE=true
```

测试模式下：
- 合约调用错误被忽略
- 返回模拟的默认值
- 不需要实际的以太坊连接

### 生产模式

在生产环境中，不设置 `SGX_TEST_MODE` 或设置为 `false`：

```bash
unset SGX_TEST_MODE
# 或
export SGX_TEST_MODE=false
```

生产模式下：
- 执行真实的合约调用
- 需要有效的以太坊客户端连接
- 合约调用错误会导致失败

### 合约接口

#### SecurityConfigContract

```solidity
// 获取允许的 MRENCLAVE 列表
function getAllowedMREnclaves() view returns (bytes32[])

// 获取允许的 MRSIGNER 列表
function getAllowedMRSigners() view returns (bytes32[])

// 获取 ISV 产品 ID
function getISVProdID() view returns (uint16)

// 获取 ISV 安全版本号
function getISVSVN() view returns (uint16)

// 获取证书有效期
function getCertValidityPeriod() view returns (uint256 notBefore, uint256 notAfter)

// 获取准入策略
function getAdmissionPolicy() view returns (bool)
```

#### GovernanceContract

```solidity
// 获取密钥迁移阈值
function getKeyMigrationThreshold() view returns (uint256)
```

### 使用示例

```go
import (
    "github.com/ethereum/go-ethereum/ethclient"
    "github.com/ethereum/go-ethereum/internal/sgx"
)

// 连接到以太坊节点
client, err := ethclient.Dial("http://localhost:8545")
if err != nil {
    log.Fatal(err)
}

// 创建环境变量管理器
// 合约地址从环境变量读取
manager, err := sgx.NewRATLSEnvManager(client)
if err != nil {
    log.Fatal(err)
}

// 从合约初始化配置
// 在测试模式下会使用默认值
err = manager.InitFromContract()
if err != nil {
    log.Fatal(err)
}

// 启动定期刷新（可选）
manager.StartPeriodicRefresh(5 * time.Minute)
```

### 配置

需要在环境中设置合约地址：

```bash
export XCHAIN_SECURITY_CONFIG_CONTRACT="0xabcdef1234567890abcdef1234567890abcdef12"
export XCHAIN_GOVERNANCE_CONTRACT="0x1234567890abcdef1234567890abcdef12345678"
```

这些地址通常在 Gramine Manifest 中固定配置，影响 MRENCLAVE。

---

## 条件编译策略

### CGO 条件编译

| Build Tag | 文件 | 用途 |
|-----------|------|------|
| `!cgo` | `*_ratls.go` | 测试/开发环境桩实现 |
| `cgo` | `*_ratls_cgo.go` | 生产环境实际实现 |

### 合约调用条件

不使用 build tag，而是使用运行时环境变量：

| 环境变量 | 值 | 行为 |
|----------|-----|------|
| `SGX_TEST_MODE` | `true` | 使用模拟值，忽略合约调用错误 |
| `SGX_TEST_MODE` | `false` 或未设置 | 执行真实合约调用 |

### 设计优势

1. **灵活性**：可以在同一个二进制文件中通过环境变量切换测试/生产模式
2. **可测试性**：不需要真实的以太坊连接即可运行测试
3. **安全性**：生产环境会实际验证合约数据
4. **兼容性**：CGO 和非 CGO 版本都可以正常编译和测试

---

## 测试

### 运行所有测试

```bash
# 测试模式（不需要 Gramine 库或以太坊连接）
CGO_ENABLED=0 SGX_TEST_MODE=true go test ./internal/sgx/... -v

# 查看覆盖率
CGO_ENABLED=0 SGX_TEST_MODE=true go test ./internal/sgx/... -cover
```

### 预期结果

所有测试应该通过：
- Attestor 测试
- Verifier 测试
- 常量时间操作测试
- Quote 解析测试
- Instance ID 测试
- 环境变量管理器测试

---

## 部署清单

### 开发/测试环境

✅ 无需特殊配置
✅ 使用桩实现
✅ 设置 `SGX_TEST_MODE=true`

### 生产环境

1. ✅ 安装 Gramine 和 RA-TLS 库
2. ✅ 配置 CGO 环境变量
3. ✅ 在 Manifest 中设置合约地址
4. ✅ 不设置 `SGX_TEST_MODE` 或设置为 `false`
5. ✅ 使用 `CGO_ENABLED=1` 构建

### 验证

```bash
# 检查 CGO 是否正确链接
go build -tags cgo -x ./internal/sgx/... 2>&1 | grep "ra_tls"

# 运行测试
CGO_ENABLED=0 SGX_TEST_MODE=true go test ./internal/sgx/... -v
```

---

## 故障排除

### CGO 链接错误

```
cannot find -lra_tls_attest
```

**解决方案**：
- 确保 Gramine RA-TLS 库已安装
- 设置正确的 `CGO_LDFLAGS` 路径

### 合约调用失败

```
contract call failed: ...
```

**解决方案**：
- 检查以太坊客户端连接
- 验证合约地址是否正确
- 确认合约已部署
- 或启用 `SGX_TEST_MODE=true` 用于测试

### 测试失败

**解决方案**：
- 确保使用 `CGO_ENABLED=0` 进行测试
- 设置 `SGX_TEST_MODE=true`
- 检查环境变量是否正确设置

---

**最后更新**: 2026-01-31
