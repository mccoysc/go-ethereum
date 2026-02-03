# Trusted Files哈希保护机制

## 概述

本文档详细说明Gramine manifest中的trusted files哈希如何通过SIGSTRUCT签名得到保护。

## 关键概念

### 1. Trusted Files

Manifest中声明的受信任文件列表，每个文件都有其SHA256哈希：

```toml
sgx.trusted_files = [
  { uri = "file:/usr/bin/geth", sha256 = "a1b2c3d4..." },
  { uri = "file:/lib/libc.so.6", sha256 = "e5f6g7h8..." },
]
```

### 2. MRENCLAVE

Enclave的度量值（measurement），是一个32字节的SHA256哈希，代表整个enclave的完整性。

### 3. SIGSTRUCT

Intel SGX签名结构，包含MRENCLAVE和RSA签名，用于证明enclave的真实性。

## 保护机制详解

### 完整的保护链

```
┌──────────────────────────────────────────────────────────────┐
│                    构建阶段 (gramine-sgx-sign)                  │
└──────────────────────────────────────────────────────────────┘

文件1: /usr/bin/geth
   ↓ SHA256
hash1 = a1b2c3d4...
   ↓ 写入manifest TOML
   ↓ 加载到enclave内存页
page1 = [hash1 | metadata]
   ↓ 参与MRENCLAVE计算
measurement = SHA256_update(measurement, EADD, page1)
measurement = SHA256_update(measurement, EEXTEND, SHA256(page1))

文件2: /lib/libc.so.6
   ↓ SHA256
hash2 = e5f6g7h8...
   ↓ 写入manifest TOML
   ↓ 加载到enclave内存页
page2 = [hash2 | metadata]
   ↓ 参与MRENCLAVE计算
measurement = SHA256_update(measurement, EADD, page2)
measurement = SHA256_update(measurement, EEXTEND, SHA256(page2))

... (所有trusted files) ...

   ↓ 最终结果
MRENCLAVE = measurement (32字节)
   ↓ 存储到SIGSTRUCT
SIGSTRUCT[960:992] = MRENCLAVE
   ↓ 包含在签名数据中
signing_data = SIGSTRUCT[0:128] + SIGSTRUCT[900:1028]
              └─────────────┬─────────────┘
                       包含MRENCLAVE
   ↓ 计算哈希
hash_to_sign = SHA256(signing_data)
   ↓ RSA签名 (3072位, 指数=3)
signature = sign(hash_to_sign, private_key)
   ↓ 存储到SIGSTRUCT
SIGSTRUCT[516:900] = signature
SIGSTRUCT[128:512] = modulus
   ↓ 输出
manifest.sgx = [SIGSTRUCT 1808字节] + [manifest TOML内容]
```

### 关联路径图

```
Trusted File哈希
       ║
       ║ (1) 直接包含
       ║
       ▼
  Manifest TOML ◄──────────┐
       ║                   │
       ║ (2) 影响构建      │ 附加在SIGSTRUCT后
       ║                   │ 但不直接签名
       ▼                   │
  Enclave内存页            │
       ║                   │
       ║ (3) 参与计算      │
       ║                   │
       ▼                   │
   MRENCLAVE ──────────────┤
       ║                   │
       ║ (4) 存储          │
       ║                   │
       ▼                   │
SIGSTRUCT[960:992] ────────┤
       ║                   │
       ║ (5) 包含在        │
       ║                   │
       ▼                   │
  signing_data             │
       ║                   │
       ║ (6) 被签名        │
       ║                   │
       ▼                   │
SIGSTRUCT[516:900] ────────┘
  (RSA signature)
```

## 验证流程

### Gramine运行时验证

当Gramine加载enclave时：

```
1. 读取manifest.sgx文件
2. 分离SIGSTRUCT和TOML内容
3. 验证SIGSTRUCT的RSA签名
4. 提取signed_mrenclave = SIGSTRUCT[960:992]
5. 解析manifest TOML
6. 读取所有trusted files
7. 根据TOML内容重新构建enclave:
   - 创建内存页
   - 加载trusted files哈希
   - 计算measurement
8. 得到computed_mrenclave
9. 比较: signed_mrenclave ?= computed_mrenclave
10. 如果匹配 → 加载enclave
11. 如果不匹配 → 拒绝加载
```

### 我们的代码验证

```go
// 1. 验证SIGSTRUCT签名
err := VerifySIGSTRUCTSignature(manifestSgx)
// → 确认SIGSTRUCT未被篡改

// 2. 提取MRENCLAVE
manifestMR := ExtractMREnclaveFromSIGSTRUCT(manifestSgx)
// → 获得签名保护的度量值

// 3. 读取运行时MRENCLAVE
runtimeMR := ReadFromAttestationDevice("/dev/attestation/my_target_info")
// → 获得CPU报告的当前度量值

// 4. 比较MRENCLAVE (带条件编译)
if manifestMR != runtimeMR {
    // 生产: 退出
    // 测试: 警告但继续
}
// → 确认当前enclave与manifest一致

// 5. (可选) 验证trusted files哈希
config := ParseManifestTOML(manifestSgx[1808:])
for _, file := range config.TrustedFiles {
    actualHash := SHA256(ReadFile(file.URI))
    if actualHash != file.SHA256 {
        // 文件在运行时被修改
    }
}
// → 额外的运行时完整性检查
```

## 安全分析

### 攻击场景与防护

#### 场景1：修改trusted file内容

```
攻击: 替换 /usr/bin/geth 为恶意版本
防护: 
  - 运行时文件哈希验证失败
  - 即使通过，Gramine加载时MRENCLAVE会不匹配
```

#### 场景2：修改manifest中的哈希声明

```
攻击: 修改manifest TOML，更改file.sha256值
防护:
  - Gramine加载时根据TOML重新计算MRENCLAVE
  - computed_mrenclave ≠ signed_mrenclave
  - 加载失败
```

#### 场景3：修改SIGSTRUCT中的MRENCLAVE

```
攻击: 直接修改SIGSTRUCT[960:992]，使其匹配被篡改的manifest
防护:
  - MRENCLAVE包含在signing_data中
  - 修改MRENCLAVE → signing_data变化
  - RSA签名验证失败
```

#### 场景4：重新签名整个SIGSTRUCT

```
攻击: 用自己的私钥重新签名整个SIGSTRUCT
防护:
  - 签名验证需要对应的公钥
  - 公钥通过MRSIGNER识别
  - MRSIGNER = SHA256(modulus)
  - 白名单只接受特定MRSIGNER
  - (注意: 当前实现未检查MRSIGNER白名单)
```

### 保护强度

**MRENCLAVE保护**：
- ✅ 强加密保护 (RSA-3072)
- ✅ 包含所有trusted files的间接影响
- ✅ CPU硬件验证
- ✅ 无法伪造

**Manifest TOML保护**：
- ⚠️ 间接保护 (通过MRENCLAVE)
- ✅ Gramine加载时验证
- ⚠️ 我们的代码不重新计算MRENCLAVE
- ✅ 但runtime MRENCLAVE验证提供保障

**文件哈希保护**：
- ✅ 通过MRENCLAVE间接保护
- ✅ 可运行时独立验证
- ✅ 提供额外防御层

## 信任链

```
Intel SGX CPU (Root of Trust)
    ↓ 生成
Platform Attestation Key
    ↓ 签名
Quote (包含runtime MRENCLAVE)
    ↓ 验证
Runtime MRENCLAVE = SIGSTRUCT MRENCLAVE?
    ↓ YES
SIGSTRUCT MRENCLAVE (来自签名)
    ↓ 包含
Trusted Files哈希的影响
    ↓ 可验证
Manifest中的哈希声明
    ↓ 可独立检查
实际文件内容
```

## 结论

Trusted files哈希通过以下多层机制得到保护：

1. **MRENCLAVE计算包含** - 文件哈希影响最终度量值
2. **SIGSTRUCT签名保护** - MRENCLAVE被RSA签名
3. **运行时验证** - CPU报告的MRENCLAVE与签名值比较
4. **独立文件验证** - 可选的额外检查层

这个设计确保了即使不能重新计算完整的MRENCLAVE，我们仍然可以通过验证签名和比较运行时值来确认trusted files的完整性。
