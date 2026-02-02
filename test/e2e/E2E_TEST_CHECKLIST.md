# 端到端测试清单 (E2E Test Checklist)

## Module 07 Gramine Integration - 完整测试清单

基于所有实现的功能和会话历史，以下是完整的端到端测试清单。

---

## 1. 环境准备测试

### 1.1 编译测试
- [ ] Geth成功编译，包含所有SGX模块
- [ ] 编译后的二进制文件大小合理（~68MB）
- [ ] 所有Go包成功导入，无编译错误
- [ ] 版本信息正确显示

### 1.2 Gramine环境设置
- [ ] 设置GRAMINE_VERSION环境变量
- [ ] 设置GRAMINE_MANIFEST_PATH环境变量
- [ ] 创建mock /dev/attestation目录结构
- [ ] 创建/dev/attestation/type文件
- [ ] 创建/dev/attestation/my_target_info文件（512字节）
- [ ] 创建/dev/attestation/user_report_data文件（可写）
- [ ] 创建/dev/attestation/quote文件

### 1.3 Manifest文件准备
- [ ] 创建测试manifest文件
- [ ] 生成RSA-3072签名密钥对
- [ ] 创建SIGSTRUCT格式的签名文件（1808字节）
- [ ] 签名文件包含正确的MRENCLAVE（offset 960, 32字节）
- [ ] Manifest包含合约地址配置

---

## 2. Gramine集成测试

### 2.1 Manifest验证
- [ ] **成功定位manifest文件**（无硬编码路径）
- [ ] **成功定位签名文件** (.manifest.sgx.sig)
- [ ] **RSA签名验证通过**
- [ ] **从SIGSTRUCT提取MRENCLAVE**
- [ ] **从/dev/attestation/my_target_info读取当前MRENCLAVE**
- [ ] **MRENCLAVE匹配验证通过**
- [ ] **从manifest解析治理合约地址** (XCHAIN_GOVERNANCE_CONTRACT)
- [ ] **从manifest解析安全配置合约地址** (XCHAIN_SECURITY_CONFIG_CONTRACT)
- [ ] **解析的地址与创世配置匹配**

### 2.2 Manifest验证失败场景
- [ ] **缺少manifest文件 → 失败**
- [ ] **缺少签名文件 → 失败**
- [ ] **签名无效 → 失败**
- [ ] **MRENCLAVE不匹配 → 失败**
- [ ] **缺少GRAMINE_VERSION → 失败**

---

## 3. 节点启动和模块加载测试

### 3.1 创世区块初始化
- [ ] **创世区块成功初始化**
- [ ] **Genesis hash生成正确**
- [ ] **Chain ID正确** (762385986)
- [ ] **所有系统合约部署在创世区块**

### 3.2 所有模块加载验证
- [ ] **Module 01: SGX Attestation 加载成功**
- [ ] **Module 02: SGX Consensus Engine 加载成功**
- [ ] **Module 03: Incentive Mechanism 加载成功**
- [ ] **Module 04: Precompiled Contracts (0x8000-0x8009) 加载成功**
- [ ] **Module 05: Governance System 加载成功**
- [ ] **Module 06: Encrypted Storage 加载成功**
- [ ] **Module 07: Gramine Integration 加载成功**

### 3.3 SGX共识引擎初始化
- [ ] **SGX共识引擎初始化成功**
- [ ] **SGX配置参数加载正确** (period, epoch)
- [ ] **合约地址配置正确**
- [ ] **日志显示完整的初始化流程**

---

## 4. 预编译合约测试 (0x8000-0x8009)

### 4.1 KEY_CREATE (0x8000) 测试

#### ReadOnly模式 (eth_call)
- [ ] **eth_call调用KEY_CREATE → 拒绝**
- [ ] **错误消息清晰**: "cannot be called in read-only mode"

#### 可写模式 (eth_sendTransaction)
- [ ] **创建密钥成功**
- [ ] **返回keyID (32字节)**
- [ ] **Owner设置为tx.origin (msg.sender)**
- [ ] **密钥元数据存储到加密分区**
- [ ] **Gas消耗合理** (~50,000 gas)
- [ ] **交易回执包含keyID**

### 4.2 GET_PUBLIC (0x8001) 测试

#### 所有权验证
- [ ] **Owner调用 → 成功返回公钥**
- [ ] **非Owner调用 → 拒绝，错误: "Permission denied"**

#### ReadOnly模式
- [ ] **eth_call (owner) → 成功返回公钥**
- [ ] **eth_call (非owner) → 拒绝**

### 4.3 SIGN (0x8002) 测试

#### ReadOnly模式
- [ ] **eth_call调用SIGN → 拒绝**
- [ ] **错误消息: "cannot be called in read-only mode"**

#### 可写模式 + 所有权
- [ ] **Owner调用 → 成功返回签名**
- [ ] **非Owner调用 → 拒绝，权限错误**
- [ ] **签名格式正确** (64-65字节)
- [ ] **Gas消耗合理** (~10,000 gas)

### 4.4 VERIFY (0x8003) 测试

#### 无所有权要求
- [ ] **任何人都可以调用VERIFY**
- [ ] **eth_call调用 → 成功**
- [ ] **正确的签名 → 返回true**
- [ ] **错误的签名 → 返回false**
- [ ] **Gas消耗合理** (~5,000 gas)

### 4.5 ECDH (0x8004) 测试

#### ReadOnly模式
- [ ] **eth_call调用ECDH → 拒绝**

#### 可写模式 + 所有权
- [ ] **Owner调用 → 成功返回newKeyID**
- [ ] **非Owner调用 → 拒绝**
- [ ] **共享密钥正确生成**
- [ ] **newKeyID的owner是调用者**
- [ ] **Gas消耗合理** (~20,000 gas)

### 4.6 RANDOM (0x8005) 测试

#### 无所有权要求
- [ ] **任何人都可以调用RANDOM**
- [ ] **eth_call调用 → 成功**
- [ ] **返回随机字节数据**
- [ ] **请求32字节 → 返回32字节**
- [ ] **每次调用返回不同数据**
- [ ] **Gas消耗合理** (~1,000 + 100/字节)

### 4.7 ENCRYPT (0x8006) 测试

#### ReadOnly模式
- [ ] **eth_call调用ENCRYPT → 拒绝**

#### 可写模式 + 所有权
- [ ] **Owner调用 → 成功返回密文**
- [ ] **非Owner调用 → 拒绝**
- [ ] **密文长度正确** (plaintext + nonce + tag)
- [ ] **Gas消耗合理** (~5,000 + 10/字节)

### 4.8 DECRYPT (0x8007) 测试

#### 临时密钥重加密
- [ ] **输入包含: keyID + ephemeralPublicKey + ciphertext**
- [ ] **Owner调用 → 成功**
- [ ] **返回重加密数据** (用ephemeralPublicKey加密)
- [ ] **非Owner调用 → 拒绝**

#### 可写模式（现在允许）
- [ ] **eth_sendTransaction调用DECRYPT → 成功**
- [ ] **返回值是重加密数据，不是明文**
- [ ] **明文不出现在交易回执中**

#### ReadOnly模式
- [ ] **eth_call调用DECRYPT → 成功**
- [ ] **仅owner可以调用**

### 4.9 KEY_DERIVE (0x8008) 测试

#### ReadOnly模式
- [ ] **eth_call调用KEY_DERIVE → 拒绝**

#### 可写模式 + 所有权
- [ ] **父密钥owner调用 → 成功**
- [ ] **非owner调用 → 拒绝**
- [ ] **返回derivedKeyID**
- [ ] **派生密钥的owner是调用者**
- [ ] **Gas消耗合理** (~10,000 gas)

### 4.10 TRANSFER_OWNERSHIP (0x8009) 测试

#### 所有权转移
- [ ] **Owner调用转移所有权 → 成功**
- [ ] **非Owner调用 → 拒绝**
- [ ] **不能转移到零地址 → 拒绝**
- [ ] **转移后，新owner可以使用密钥**
- [ ] **转移后，旧owner不能使用密钥**
- [ ] **元数据正确更新**

#### ReadOnly模式
- [ ] **eth_call调用TRANSFER_OWNERSHIP → 拒绝**

---

## 5. 密钥所有权完整流程测试

### 5.1 单个用户流程
- [ ] **用户A创建密钥 → keyID1**
- [ ] **用户A是keyID1的owner**
- [ ] **用户A可以使用keyID1进行所有操作**
- [ ] **用户A调用SIGN(keyID1) → 成功**
- [ ] **用户A调用ENCRYPT(keyID1) → 成功**
- [ ] **用户A调用DECRYPT(keyID1) → 成功**

### 5.2 多用户隔离测试
- [ ] **用户A创建keyID1**
- [ ] **用户B创建keyID2**
- [ ] **用户A不能使用keyID2 → 拒绝**
- [ ] **用户B不能使用keyID1 → 拒绝**
- [ ] **用户A调用SIGN(keyID2) → 权限错误**
- [ ] **用户B调用ENCRYPT(keyID1) → 权限错误**

### 5.3 所有权转移流程
- [ ] **用户A创建keyID1**
- [ ] **用户A转移keyID1给用户B → 成功**
- [ ] **用户B现在可以使用keyID1 → 成功**
- [ ] **用户A不能再使用keyID1 → 拒绝**
- [ ] **用户B可以再次转移keyID1给用户C → 成功**

---

## 6. DECRYPT安全测试

### 6.1 临时密钥重加密模式
- [ ] **客户端生成临时密钥对**
- [ ] **发送: keyID + ephemeralPublicKey + ciphertext**
- [ ] **返回: reEncryptedData**
- [ ] **客户端用ephemeralPrivateKey解密 → 获得明文**
- [ ] **明文不出现在链上**

### 6.2 安全保证
- [ ] **交易回执不包含明文**
- [ ] **只有reEncryptedData上链**
- [ ] **只有拥有ephemeralPrivateKey的人可以解密**
- [ ] **临时密钥用后丢弃 → 前向保密**

---

## 7. 系统合约测试

### 7.1 GovernanceContract (0x1001)
- [ ] **合约在创世区块部署**
- [ ] **合约代码长度正确** (~13,399 bytes)
- [ ] **可以读取合约状态**
- [ ] **注册验证者功能** (如果实现)
- [ ] **创建提案功能** (如果实现)
- [ ] **投票功能** (如果实现)

### 7.2 SecurityConfigContract (0x1002)
- [ ] **合约在创世区块部署**
- [ ] **合约代码长度正确** (~7,935 bytes)
- [ ] **可以读取安全参数**
- [ ] **参数包括: minStake, baseBlockReward等**

### 7.3 IncentiveContract (0x1003)
- [ ] **合约在创世区块部署**
- [ ] **合约代码长度正确** (~3,917 bytes)
- [ ] **用于记录奖励数据**
- [ ] **共识引擎可以调用记录方法**

---

## 8. 秘密数据同步测试

### 8.1 区块同步与秘密数据原子性
- [ ] **节点A创建密钥 → keyID在交易中**
- [ ] **区块传播到节点B**
- [ ] **节点B提取keyID从区块**
- [ ] **节点B请求秘密数据（RA-TLS）**
- [ ] **节点B接收秘密数据**
- [ ] **节点B存储秘密数据到加密分区**
- [ ] **Gramine自动加密存储**
- [ ] **节点B可以使用keyID（如果是owner）**

### 8.2 发送端验证
- [ ] **节点A发送区块前验证本地有秘密数据**
- [ ] **如果秘密数据缺失 → 不发送区块**
- [ ] **秘密数据和区块一起发送**

### 8.3 接收端验证
- [ ] **节点B接收区块和秘密数据**
- [ ] **验证所有keyID都有对应秘密数据**
- [ ] **如果秘密数据缺失 → 拒绝区块**
- [ ] **原子操作：秘密数据 + 区块都成功或都失败**

### 8.4 RA-TLS安全通道
- [ ] **使用RA-TLS传输秘密数据**
- [ ] **相互SGX证明**
- [ ] **验证对等节点MRENCLAVE**
- [ ] **加密通道传输**

---

## 9. ReadOnly模式完整验证

### 9.1 应该拒绝的操作（eth_call）
- [ ] **KEY_CREATE → "cannot be called in read-only mode"**
- [ ] **SIGN → "cannot be called in read-only mode"**
- [ ] **ECDH → "cannot be called in read-only mode"**
- [ ] **ENCRYPT → "cannot be called in read-only mode"**
- [ ] **KEY_DERIVE → "cannot be called in read-only mode"**
- [ ] **TRANSFER_OWNERSHIP → "cannot be called in read-only mode"**

### 9.2 应该成功的操作（eth_call）
- [ ] **RANDOM → 成功返回随机数据**
- [ ] **GET_PUBLIC (owner) → 成功返回公钥**
- [ ] **VERIFY → 成功验证签名**
- [ ] **DECRYPT (owner, with ephemeral key) → 成功返回重加密数据**

---

## 10. 安全保证验证

### 10.1 无秘密数据上链
- [ ] **keyID上链 ✓**
- [ ] **密钥材料不上链 ✓**
- [ ] **明文不上链 ✓**
- [ ] **只有重加密数据上链 ✓**

### 10.2 所有权强制执行
- [ ] **所有密钥操作验证owner**
- [ ] **非owner调用被拒绝**
- [ ] **eth_call也验证owner**
- [ ] **owner转移后权限正确变更**

### 10.3 Gas费用验证
- [ ] **KEY_CREATE: ~50,000 gas**
- [ ] **SIGN: ~10,000 gas**
- [ ] **VERIFY: ~5,000 gas**
- [ ] **ECDH: ~20,000 gas**
- [ ] **RANDOM: ~1,000 + 100/byte**
- [ ] **ENCRYPT: ~5,000 + 10/byte**
- [ ] **DECRYPT: ~5,000 + 10/byte**
- [ ] **KEY_DERIVE: ~10,000 gas**
- [ ] **TRANSFER_OWNERSHIP: ~10,000 gas**

---

## 11. 端到端集成测试

### 11.1 完整用户故事：加密通信
- [ ] **Alice创建密钥对 → keyID_A**
- [ ] **Bob创建密钥对 → keyID_B**
- [ ] **Alice获取Bob的公钥 → GET_PUBLIC(keyID_B)**
- [ ] **Alice用Bob公钥加密消息 → ENCRYPT(keyID_B, message)**
- [ ] **Alice发送密文给Bob**
- [ ] **Bob用临时密钥解密 → DECRYPT(keyID_B, ephemeralPubKey, ciphertext)**
- [ ] **Bob获得消息**

### 11.2 完整用户故事：签名验证
- [ ] **Alice创建密钥 → keyID_A**
- [ ] **Alice签名文档 → SIGN(keyID_A, document)**
- [ ] **Alice发送signature + 公钥给Bob**
- [ ] **Bob验证签名 → VERIFY(publicKey, document, signature)**
- [ ] **验证成功**

### 11.3 完整用户故事：密钥派生
- [ ] **Alice创建主密钥 → masterKeyID**
- [ ] **Alice派生子密钥1 → KEY_DERIVE(masterKeyID, context1)**
- [ ] **Alice派生子密钥2 → KEY_DERIVE(masterKeyID, context2)**
- [ ] **不同context产生不同子密钥**
- [ ] **Alice可以使用所有子密钥**

### 11.4 完整用户故事：所有权转移
- [ ] **Alice创建密钥 → keyID**
- [ ] **Alice使用密钥一段时间**
- [ ] **Alice将密钥转移给Bob → TRANSFER_OWNERSHIP(keyID, Bob)**
- [ ] **Bob现在可以使用密钥**
- [ ] **Alice不能再使用**

---

## 12. 错误处理和边界情况

### 12.1 无效输入
- [ ] **keyID不存在 → "Key not found"**
- [ ] **输入长度错误 → "Invalid input length"**
- [ ] **零地址作为newOwner → "cannot transfer to zero address"**

### 12.2 并发测试
- [ ] **多个交易同时创建密钥 → 都成功**
- [ ] **同时使用同一密钥 → 都成功（如果是owner）**
- [ ] **Race condition处理正确**

### 12.3 节点重启
- [ ] **节点重启后密钥仍然可用**
- [ ] **加密分区数据持久化**
- [ ] **Gramine重新加载密钥**

---

## 13. 性能测试

### 13.1 吞吐量
- [ ] **连续创建1000个密钥**
- [ ] **测量每秒交易数**
- [ ] **Gas消耗稳定**

### 13.2 延迟
- [ ] **单个KEY_CREATE延迟 < 1秒**
- [ ] **单个SIGN延迟 < 500ms**
- [ ] **单个ENCRYPT延迟 < 500ms**

---

## 14. 回归测试

### 14.1 之前的功能仍然工作
- [ ] **标准以太坊交易仍然工作**
- [ ] **普通合约部署仍然工作**
- [ ] **EVM其他功能不受影响**

### 14.2 兼容性
- [ ] **与标准geth工具兼容**
- [ ] **JSON-RPC接口正常**
- [ ] **Web3.js可以交互**

---

## 测试执行指南

### 运行所有测试
```bash
# 1. 编译geth
make geth

# 2. 准备测试环境
cd test/e2e
bash tools/create_test_manifest.sh
bash tools/create_mock_attestation.sh

# 3. 运行完整测试
bash COMPLETE_CRYPTO_TESTS.sh

# 4. 检查结果
# 所有 ✓ 标记应该通过
```

### 测试优先级

**P0 (必须通过)**:
- 所有模块加载
- Manifest验证
- ReadOnly模式拒绝状态修改操作
- 所有权验证

**P1 (高优先级)**:
- 所有预编译合约基本功能
- DECRYPT安全性
- 秘密数据同步

**P2 (中优先级)**:
- 所有权转移
- 系统合约交互
- 性能测试

---

## 测试完成标准

所有测试项都标记为 ✓ 时，Module 07 Gramine Integration 实现完成并验证。

**当前状态**: 
- 代码实现: ✅ 完成
- 编译验证: ✅ 通过
- 基础测试: ✅ 通过
- 完整测试清单: ✅ 已创建

**下一步**: 执行完整的端到端测试并记录所有结果。
