# X Chain PoA-SGX 权限功能深度调查

## 概述

用户提出了5个关键的权限限制功能特性，声称这些已在之前的会话中实现。本文档调查这些特性的实现状态。

## 5个权限限制特性

### 1. 只读模式下创建key必须报告balance不足

**需求：**
- 任何生成keyid的接口在被只读模式调用时都必须报告balance不足
- 原因：生成的数据需要保存到加密路径

**调查结果：**
```
文件：core/vm/sgx_key_create.go
函数：SGXKeyCreate.RunWithContext()

现状：
- ✅ 需要SGXContext（ctx.Caller）
- ✅ 调用KeyStore.CreateKey(ctx.Caller, keyType)
- ❌ 没有balance检查代码
- ❌ 没有检查是否为只读调用
```

**缺失的实现：**
```go
// 应该有类似的检查
if ctx.ReadOnly {
    return nil, errors.New("insufficient balance for key creation")
}

// 或者检查caller的balance
balance := stateDB.GetBalance(ctx.Caller)
if balance.Cmp(minimumBalance) < 0 {
    return nil, errors.New("insufficient balance")
}
```

**状态：❌ 未实现**

---

### 2. 使用已有key时必须检查owner权限

**需求：**
- 任何使用已有key或keyid的接口都必须检查key的owner
- 这些接口无法被只读模式调用

**调查结果：**

**文件1：core/vm/sgx_key_delete.go**
```go
func (c *SGXKeyDelete) RunWithContext(ctx *SGXContext, input []byte) ([]byte, error) {
    // ...
    err := ctx.KeyStore.DeleteKey(keyID, ctx.Caller)
    // DeleteKey内部应该检查owner
}
```

**文件2：core/vm/sgx_keystore_impl.go**
```go
func (ks *EncryptedKeyStore) DeleteKey(keyID common.Hash, caller common.Address) error {
    // TODO: 需要检查此处是否有owner验证
}
```

**需要检查的precompiles：**
- SGX_SIGN (0x8002) - 使用key签名
- SGX_ENCRYPT (0x8004) - 使用key加密
- SGX_DECRYPT (0x8005) - 使用key解密
- SGX_KEY_DELETE (0x8006) - 删除key
- SGX_KEY_DERIVE (0x8007) - 派生key

**状态：⏳ 需要深入检查每个实现**

---

### 3. keyowner可以转移

**需求：**
- keyowner可以被keyowner转移给另一个地址

**调查结果：**

**查找transfer相关代码：**
```bash
grep -r "TransferOwner\|transferOwner\|OwnerTransfer" core/vm/sgx*.go
```

**KeyMetadata结构：**
```go
type KeyMetadata struct {
    KeyID       common.Hash
    Owner       common.Address  // 有Owner字段
    KeyType     KeyType
    CreatedAt   uint64
    CreatedBy   common.Address
    Permissions []Permission
}
```

**Permission系统存在：**
```go
type Permission struct {
    Grantee   common.Address
    Type      PermissionType
    ExpiresAt uint64
    MaxUses   uint64
    UsedCount uint64
}

const (
    PermissionAdmin PermissionType = 0x01
    PermissionSign  PermissionType = 0x02
    PermissionDecrypt PermissionType = 0x04
)
```

**状态：⏳ Permission系统存在，但需要检查是否有TransferOwnership功能**

---

### 4. 区块同步包括密码数据同步

**需求：**
- 区块同步包括区块数据的同步和区块记录的keyid对应的密码数据的同步
- 任何一个没同步就算整个区块同步失败

**调查结果：**

**可能的实现位置：**
1. consensus/sgx/ - 共识层区块同步
2. core/blockchain.go - 区块链同步逻辑
3. eth/sync.go - 同步协议

**关键问题：**
- 密码数据存储在哪里？（加密路径）
- 如何在P2P网络中传输密码数据？
- 如何验证密码数据的完整性？

**需要查找：**
```bash
grep -r "SyncBlock\|syncBlock\|InsertBlock" consensus/sgx/
grep -r "encrypted.*sync\|keyid.*sync" core/
```

**状态：⏳ 需要深入调查区块同步机制**

---

### 5. 解密接口支持重加密

**需求：**
- 解密接口必须接受重加密参数
- 解密出的数据以重加密的密文上链
- 用户可以用自己的重加密密码解密获得明文

**调查结果：**

**文件：查找SGX_DECRYPT实现**
```bash
find core/vm -name "*decrypt*"
```

**需要检查：**
1. SGX_DECRYPT的输入格式是否包含重加密参数
2. 是否有re-encryption key的概念
3. 输出是否为重加密后的密文而非明文

**代理重加密（Proxy Re-Encryption）：**
这是一个高级密码学特性，允许：
- Alice用keyA加密数据
- Bob请求访问
- 系统用re-encryption key将密文转换为Bob可解密的密文
- Bob用keyB解密

**状态：⏳ 需要检查实现细节**

---

## 调查计划

### 第1步：检查每个SGX precompile的实现

文件清单：
- [ ] core/vm/sgx_key_create.go
- [ ] core/vm/sgx_key_get_public.go
- [ ] core/vm/sgx_sign.go
- [ ] core/vm/sgx_verify.go
- [ ] core/vm/sgx_encrypt.go
- [ ] core/vm/sgx_decrypt.go
- [ ] core/vm/sgx_key_delete.go
- [ ] core/vm/sgx_key_derive.go
- [ ] core/vm/sgx_ecdh.go
- [ ] core/vm/sgx_random.go

### 第2步：检查KeyStore实现

文件：
- [ ] core/vm/sgx_keystore.go (接口定义)
- [ ] core/vm/sgx_keystore_impl.go (实现)

### 第3步：检查Permission系统

文件：
- [ ] core/vm/sgx_permission.go
- [ ] core/vm/sgx_permission_impl.go (如果存在)

### 第4步：检查区块同步

文件：
- [ ] consensus/sgx/sync.go (如果存在)
- [ ] consensus/sgx/api.go

### 第5步：更新E2E测试

需要添加的测试用例：
1. 只读模式调用key创建应失败
2. 非owner调用需要owner权限的操作应失败
3. Owner可以转移ownership
4. 区块同步验证（需要多节点）
5. 重加密功能测试

---

## 下一步行动

1. ✅ 创建本调查文档
2. ⏳ 系统检查所有相关文件
3. ⏳ 记录发现的实现和缺失
4. ⏳ 创建或更新E2E测试
5. ⏳ 修复发现的问题

