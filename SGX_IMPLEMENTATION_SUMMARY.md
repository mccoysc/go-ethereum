# SGX 实现总结

## 核心原则

### SGX 的真正作用

1. **远程证明（Remote Attestation）**
   - 证明代码在真实的 SGX Enclave 内运行
   - 使用区块哈希作为 userData 生成 Quote
   - Quote 包含：MRENCLAVE、userData、时间戳等

2. **RA-TLS（Remote Attestation TLS）**
   - 建立带远程证明的安全连接
   - 验证对方运行在 SGX 内

3. **硬件加密内存**
   - SGX 硬件自动加密 RAM
   - 应用无需处理

4. **Gramine 透明加密文件系统**
   - Gramine 自动加密/解密文件
   - 应用只需正常读写文件

## 应用层 **不需要** 做的事

❌ **不需要手动加密/解密文件**
```go
// 错误示例
encrypted := encrypt(data, key)  // ❌ 不需要
os.WriteFile(path, encrypted, 0600)

// 正确示例
os.WriteFile(path, data, 0600)  // ✓ Gramine 自动加密
```

❌ **不需要手动 seal/unseal 密钥**
```go
// 错误示例
sealed := sgx_seal(key)  // ❌ 不需要

// 正确示例
// 直接写入加密分区，Gramine 使用 MRENCLAVE 自动 seal
os.WriteFile("/data/encrypted/key.dat", key, 0600)  // ✓
```

❌ **不需要手动加密内存**
```go
// SGX 硬件自动加密所有 Enclave 内存
// 应用代码完全不需要处理
```

## 应用层 **需要** 做的事

✅ **生成 SGX 远程证明**
```go
// consensus/sgx/consensus.go - Seal()方法
func (e *SGXEngine) Seal(...) error {
    sealHash := e.SealHash(header)
    
    // 用区块哈希作为 userData 生成 Quote
    quote, err := e.attestor.GenerateQuote(sealHash.Bytes())
    
    // 在 Enclave 内签名（私钥永不离开 SGX）
    signature, err := e.attestor.SignInEnclave(sealHash.Bytes())
    
    // 包含 SGX 证明的区块头
    extra := &SGXExtra{
        SGXQuote:      quote,      // 远程证明报告
        ProducerID:    producerID, // 出块者身份
        AttestationTS: timestamp,  // 时间戳
        Signature:     signature,  // 签名
    }
    ...
}
```

## 代码实现对比

### 密钥存储（正确实现）

```go
// core/vm/sgx_keystore_impl.go

// 保存私钥 - 直接写入加密分区
func (ks *EncryptedKeyStore) savePrivateKey(keyID common.Hash, privKey interface{}) error {
    keyPath := filepath.Join(ks.encryptedPath, keyID.Hex()+".key")
    // ✓ 直接写入 - Gramine 自动加密
    return os.WriteFile(keyPath, data, 0600)
}

// 读取私钥 - 直接从加密分区读取
func (ks *EncryptedKeyStore) loadPrivateKey(keyID common.Hash, keyType KeyType) (interface{}, error) {
    keyPath := filepath.Join(ks.encryptedPath, keyID.Hex()+".key")
    // ✓ 直接读取 - Gramine 自动解密
    data, err := os.ReadFile(keyPath)
    ...
}
```

**注意**：
- ✓ `ks.encryptedPath` 指向 Gramine manifest 中配置的加密分区
- ✓ 使用标准的 `os.WriteFile` 和 `os.ReadFile`
- ✓ Gramine 使用 MRENCLAVE/MRSIGNER 派生的密钥自动 seal/unseal

### Gramine Manifest 配置

```
# geth.manifest.template

# 加密分区配置
fs.mounts = [
  # 使用 Gramine 内置的 seal key，无需应用自定义
  # 开发模式：_sgx_mrsigner（重编译后数据无需迁移）
  # 生产模式：_sgx_mrenclave（最高安全性，只有相同代码能访问）
  { type = "encrypted", 
    path = "/data/encrypted", 
    uri = "file:/data/encrypted", 
    key_name = "_sgx_mrenclave" },  # 或 "_sgx_mrsigner"
    
  { type = "encrypted", 
    path = "/app/wallet", 
    uri = "file:/data/wallet", 
    key_name = "_sgx_mrenclave" },
]
```

**Gramine 官方内置 key_name**（无需应用自定义）：

| key_name | 派生基础 | 用途 | 数据迁移 |
|----------|---------|------|---------|
| `_sgx_mrenclave` | MRENCLAVE | 生产环境（推荐） | 代码改变需要迁移 |
| `_sgx_mrsigner` | MRSIGNER | 开发环境（推荐） | 代码改变无需迁移 |
| `_sgx_mrenclave_legacy` | MRENCLAVE | 兼容旧版本 | 代码改变需要迁移 |
| `_sgx_mrsigner_legacy` | MRSIGNER | 兼容旧版本 | 代码改变无需迁移 |

**选择建议**：
- **开发/测试**：使用 `_sgx_mrsigner`（重新编译后数据仍可访问）
- **生产部署**：使用 `_sgx_mrenclave`（最高安全性，只有完全相同的代码能访问）

**Gramine 内置 key_name**：
- `_sgx_mrenclave`：使用 MRENCLAVE 派生（生产推荐）
- `_sgx_mrsigner`：使用 MRSIGNER 派生（开发方便）
- `_sgx_mrenclave_legacy`：旧版 MRENCLAVE 格式（向后兼容）
- `_sgx_mrsigner_legacy`：旧版 MRSIGNER 格式（向后兼容）
- **无需应用生成或管理任何密钥**

**Gramine 自动完成**：
1. 调用 SGX `sgx_get_seal_key()` 使用 MRENCLAVE/MRSIGNER 派生密钥
2. 写入时：AES-GCM 加密后存储到宿主机文件系统
3. 读取时：AES-GCM 解密后返回给应用
4. 应用层完全透明，使用标准文件 I/O

## 区块 Seal 流程

### 标准以太坊处理

```go
// 1. Prepare - 设置区块头
Difficulty = 1
Timestamp = now
Extra = []  // 预留空间

// 2. Execute - 执行交易
state.Apply(transactions)

// 3. Finalize - 计算状态根
header.Root = state.IntermediateRoot()

// 4. Seal - 密封区块
sealHash = header.Hash()  // 不包含签名
signature = sign(sealHash)
header.Extra = signature
```

### PoA-SGX 扩展

```go
// 1-3: 完全相同

// 4. Seal - 添加 SGX 远程证明
sealHash = header.Hash()  // 标准计算

// === 仅此处是 SGX 特有 ===
quote = GenerateSGXQuote(sealHash)      // userData = sealHash
signature = SignInEnclave(sealHash)     // 私钥在 Enclave 内
producerID = GetProducerID()

header.Extra = {
    SGXQuote,      // 远程证明
    ProducerID,    // 身份
    Timestamp,     // 时间
    Signature      // 签名
}
```

**除 Seal 方法外，所有其他代码与以太坊完全一致！**

## SGX Quote 内容

```
SGX Quote 结构：
├── MRENCLAVE     (32 bytes)  - 代码度量值
├── MRSIGNER      (32 bytes)  - 签名者公钥哈希
├── Attributes    (16 bytes)  - 安全属性
├── Report Data   (64 bytes)  - 用户数据（区块哈希）
└── Signature     (variable)  - Intel 签名
```

**验证流程**：
1. 验证 Intel 签名（证明 Quote 真实）
2. 检查 MRENCLAVE 是否在白名单（证明代码正确）
3. 提取 Report Data，验证是否等于区块哈希（证明绑定）

## 安全边界

| 层级 | 职责 | 实现方式 |
|------|------|---------|
| **应用层** | 业务逻辑 | 标准 Go 代码 |
| **SGX Seal()** | 生成远程证明 | `GenerateQuote(blockHash)` |
| **Gramine** | 文件透明加密 | Encrypted filesystem |
| **SGX 硬件** | 内存加密、密钥派生 | CPU 自动完成 |

## 部署和测试

### 开发模式（使用 _sgx_mrsigner）

```bash
# 优点：代码改动后不需要迁移数据（同一签名者的版本都能访问）
# 缺点：安全性稍低
# 使用 Gramine 内置 key_name，无需应用自定义
cd gramine
./rebuild-manifest.sh dev

# 脚本自动设置：key_name = "_sgx_mrsigner"
# Gramine 自动调用 SGX sgx_get_seal_key() 派生密钥
```

### 生产模式（使用 _sgx_mrenclave）

```bash
# 优点：最高安全性（只有相同代码能访问数据）
# 缺点：代码改动后需要数据迁移
# 使用 Gramine 内置 key_name，无需应用自定义
cd gramine
./rebuild-manifest.sh prod

# 脚本自动设置：key_name = "_sgx_mrenclave"
# Gramine 自动调用 SGX sgx_get_seal_key() 派生密钥
```

### Gramine 官方内置 key_name

根据 Gramine 官方文档，支持以下内置 key_name：

| key_name | 派生基础 | 安全性 | 代码更新 |
|----------|---------|--------|---------|
| `_sgx_mrenclave` | MRENCLAVE（代码度量） | 最高 | 需要迁移数据 |
| `_sgx_mrsigner` | MRSIGNER（签名者） | 中等 | 无需迁移 |
| `_sgx_mrenclave_legacy` | MRENCLAVE（旧版） | 最高 | 需要迁移数据 |
| `_sgx_mrsigner_legacy` | MRSIGNER（旧版） | 中等 | 无需迁移 |

**推荐选择**：
- 开发/测试：`_sgx_mrsigner`（方便迭代）
- 生产环境：`_sgx_mrenclave`（最高安全性）

**应用层无需**：
- ❌ 生成 seal key
- ❌ 管理 seal key
- ❌ 调用 SGX seal/unseal API
- ✅ 只需在 manifest 中指定 key_name

### 运行测试
```bash
# 编译
make geth

# 初始化（使用包含系统合约的创世配置）
geth init test/integration/genesis-complete.json

# 启动节点
geth --mine --miner.etherbase 0x...

# 验证 SGX Quote（在其他节点）
# 节点会自动验证区块头中的 SGX Quote
```

## 代码组织

```
go-ethereum/
├── consensus/sgx/
│   ├── consensus.go          # ✓ Seal()中生成远程证明
│   ├── attestor.go           # ✓ SGX Quote 生成接口
│   └── verify.go             # ✓ SGX Quote 验证
│
├── core/vm/
│   ├── sgx_*.go              # ✓ 预编译合约（密钥管理）
│   └── sgx_keystore_impl.go # ✓ 直接读写加密分区
│
├── storage/
│   └── encrypted_partition_impl.go  # ✓ 直接读写，Gramine加密
│
├── gramine/
│   └── geth.manifest.template       # ✓ 配置加密分区
│
└── contracts/
    ├── GovernanceContract.sol       # ✓ 治理（MRENCLAVE白名单）
    ├── SecurityConfigContract.sol   # ✓ 安全配置
    └── IncentiveContract.sol        # ✓ 激励
```

## 常见误区

❌ **误区 1**：需要在应用层手动调用 SGX seal API
✓ **正确**：Gramine 自动处理，应用只需写入加密分区

❌ **误区 2**：需要在代码中加密/解密数据
✓ **正确**：Gramine 透明处理，使用标准文件 I/O

❌ **误区 3**：需要在区块处理的很多地方加 SGX 代码
✓ **正确**：只在 Seal()方法中生成远程证明

❌ **误区 4**：需要手动管理 Enclave 内存加密
✓ **正确**：SGX 硬件自动加密所有 Enclave 内存

## 总结

### SGX 集成的精髓

1. **最小侵入**：仅在 Seal()方法中添加远程证明
2. **透明加密**：文件和内存加密完全自动
3. **标准代码**：除远程证明外，代码与以太坊一致
4. **安全保证**：通过 SGX Quote 证明区块来源

### 实现质量检查

✅ 应用层无手动加密代码
✅ 密钥存储使用标准文件 I/O
✅ Seal()方法生成 SGX Quote
✅ Quote 使用区块哈希作为 userData
✅ Gramine manifest 配置加密分区
✅ 除 Seal 外，代码与以太坊一致

**结论**：实现简洁、安全、符合 SGX 最佳实践！
