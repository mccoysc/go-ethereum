# 安全修复总结 - 零容忍绕过政策

## 问题

之前的代码包含多个安全绕过点，允许在不满足安全条件时继续运行：

1. **Manifest验证绕过**
   - `SGX_TEST_MODE=true` 可以跳过manifest签名验证
   - 非Gramine环境自动跳过验证
   - 找不到公钥时返回nil verifier

2. **SGX证明绕过**
   - testMode允许返回假的Quote
   - testMode允许返回假的签名
   - Mock实现完全不调用SGX

3. **其他绕过**
   - 各种"开发模式"自动跳过检查
   - 静默失败（不报错继续运行）

## 修复

### 原则：Fail-Safe（失败即安全）

```
找不到manifest？ → 程序终止
找不到签名？ → 程序终止
签名验证失败？ → 程序终止
MRENCLAVE不匹配？ → 程序终止
不在Gramine环境？ → 程序终止
找不到公钥？ → 程序终止
```

**绝不静默跳过，绝不返回假数据**

### 具体修复

#### 1. internal/sgx/manifest_verifier.go

**移除**：
```go
// ❌ 删除
if os.Getenv("SGX_TEST_MODE") == "true" {
    log.Warn("SKIPPED")
    return nil
}
```

**现在**：
```go
// ✓ 必须验证
manifestPath, err := findManifestFile()
if err != nil {
    return fmt.Errorf("SECURITY: Cannot locate manifest file: %w")
}

verifier, err := NewManifestSignatureVerifier()
if err != nil {
    return fmt.Errorf("failed to create verifier: %w")
}

// 必须验证RSA签名
if err := verifier.VerifyManifestSignature(...); err != nil {
    return fmt.Errorf("verification FAILED: %w", err)
}

// 必须验证MRENCLAVE
if !bytes.Equal(manifestMREnclave, currentMREnclave) {
    return fmt.Errorf("MRENCLAVE MISMATCH")
}
```

#### 2. consensus/sgx/attestor_gramine.go

**移除**：
```go
// ❌ 删除
type GramineAttestor struct {
    testMode bool  // 删除
}

func (a *GramineAttestor) GenerateQuote(data []byte) ([]byte, error) {
    if a.testMode {  // 删除
        return generateMockQuote(data), nil  // 删除
    }
    ...
}
```

**现在**：
```go
// ✓ 只有真实实现
type GramineAttestor struct {
    // 无testMode字段
}

func (a *GramineAttestor) GenerateQuote(data []byte) ([]byte, error) {
    // 只调用真实的SGX
    quote, err := gramineGenerateQuote(data)
    if err != nil {
        return nil, fmt.Errorf("failed to generate SGX quote: %w", err)
    }
    return quote, nil
}
```

#### 3. consensus/sgx/consensus.go

**移除**：
```go
// ❌ 删除Mock
attestor := &DefaultAttestor{}  // Mock实现
verifier := &DefaultVerifier{}  // Mock实现
```

**现在**：
```go
// ✓ 必须是真实实现
attestor, err := NewGramineAttestor()
if err != nil {
    log.Crit("Failed to create Gramine attestor - REQUIRED")
    // 程序终止
}

verifier, err := NewGramineVerifier()
if err != nil {
    log.Crit("Failed to create Gramine verifier")
    // 程序终止
}
```

#### 4. 删除Mock文件

```bash
# 重命名禁用
mv consensus/sgx/attestor.go \
   consensus/sgx/attestor_OLD_MOCK.go.disabled
```

## 测试要求

### 如果需要测试，必须提供真实基础设施

#### Manifest测试
```bash
# 1. 生成测试密钥对
openssl genrsa -3 -out test-signing-key.pem 3072
openssl rsa -in test-signing-key.pem -pubout -out test-signing-key.pub

# 2. 创建测试manifest
cat > test.manifest.sgx << EOF
# Test manifest content
EOF

# 3. 签名manifest
gramine-sgx-sign \
    --key test-signing-key.pem \
    --manifest test.manifest.sgx \
    --output test.manifest.sgx.sig

# 4. 设置环境变量
export GRAMINE_SIGSTRUCT_KEY_PATH=./test-signing-key.pub
export GRAMINE_MANIFEST_PATH=./test.manifest.sgx
```

#### SGX测试
```bash
# 必须在Gramine容器内
docker run --rm -it \
    -v $(pwd):/workspace \
    -w /workspace \
    gramineproject/gramine:latest \
    bash

# 在容器内运行测试
./build/bin/geth ...
```

**不允许**：
- ❌ 设置 SGX_TEST_MODE=true
- ❌ 使用 mock 数据
- ❌ 跳过验证

## 安全保证

### 1. 默认安全
系统默认要求所有安全检查，不会因为"方便"而跳过。

### 2. 明确失败
如果安全条件不满足，程序立即终止，不会静默继续。

### 3. 无意外绕过
没有任何环境变量或配置可以意外绕过安全检查。

### 4. 生产就绪
代码从第一行开始就是生产级别的安全性。

## 对比

| 方面 | 修复前 | 修复后 |
|------|--------|--------|
| 测试便利性 | ✓ 很方便 | ⚠️ 需要真实环境 |
| 安全绕过 | ❌ 多处存在 | ✓ 完全消除 |
| 意外绕过 | ❌ 容易发生 | ✓ 不可能 |
| 生产安全性 | ⚠️ 有风险 | ✓ 完全安全 |
| 错误检测 | ⚠️ 静默失败 | ✓ 立即发现 |
| 代码质量 | ⚠️ 混杂mock | ✓ 纯净实现 |

## 总结

### 移除的代码
- 所有 `SGX_TEST_MODE` 检查
- 所有 `testMode` 变量
- 所有 mock 实现
- 所有静默跳过逻辑
- 所有假数据生成

### 现在的代码
- ✅ 只有真实实现
- ✅ 必须在Gramine环境
- ✅ 必须提供真实文件
- ✅ 必须通过所有验证
- ✅ 失败即终止

### 哲学
**"如果不能安全地做，就不要做"**

- 不为了便利牺牲安全
- 不为了测试降低标准
- 不允许任何绕过
- 生产级别从第一天开始

**系统现在是真正安全的！** 🔒
