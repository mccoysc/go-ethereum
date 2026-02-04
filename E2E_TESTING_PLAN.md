# E2E测试计划 (End-to-End Testing Plan)

## 概述

根据用户要求："单元测试完了后，需要以端到端测试验证，而不是go层面的单元测试过了就算过了"

本文档定义完整的测试策略，从单元测试到端到端验证。

## 测试层次

### 1. 单元测试 (Unit Tests)
**目的**: 测试单个组件的隔离功能  
**工具**: Go test framework  
**位置**: `*_test.go` 文件  
**状态**: 进行中

### 2. 集成测试 (Integration Tests)  
**目的**: 测试组件间交互  
**工具**: Go test + 真实依赖  
**状态**: 待实现

### 3. 端到端测试 (E2E Tests)
**目的**: 验证完整系统在真实环境中的行为  
**工具**: geth命令行 + RPC调用  
**状态**: 待实现

---

## 主线任务E2E测试

### 任务1: 确保正常出块

#### 单元测试 (当前)
```bash
go test ./consensus/sgx -run TestBlockProduction -v
```

#### E2E测试
```bash
# 1. 初始化测试链
geth --datadir /tmp/sgx-testnet init genesis-sgx.json

# 2. 启动SGX共识节点
geth --datadir /tmp/sgx-testnet \
     --sgx \
     --http \
     --http.api eth,web3,admin \
     --http.corsdomain "*" \
     --nodiscover \
     --allow-insecure-unlock

# 3. 创建账户
geth --datadir /tmp/sgx-testnet account new

# 4. 通过控制台发送交易
geth attach /tmp/sgx-testnet/geth.ipc
> eth.sendTransaction({from: eth.accounts[0], to: "0x...", value: web3.toWei(1, "ether")})

# 5. 验证区块生产
> eth.blockNumber  # 应该递增
> eth.getBlock(1)  # 应该包含交易
> eth.getBlock(1).extraData  # 应该包含SGX Quote

# 6. 验证区块间隔符合按需出块逻辑
# 7. 验证区块包含正确的SGX签名
```

**验收标准**:
- [ ] 区块号持续递增
- [ ] 交易被正确打包进区块
- [ ] 区块包含有效的SGX Quote
- [ ] 按需出块逻辑工作（有交易时出块，无交易时不出）
- [ ] 区块能被其他节点接受和验证

---

### 任务2: 密码学预编译接口测试

#### 单元测试
```bash
go test ./core/vm -run TestSGXPrecompiles -v
```

#### 合约测试
部署测试合约 `test/contracts/SGXCryptoTest.sol`:
```solidity
contract SGXCryptoTest {
    // 测试加密
    function testEncrypt(bytes memory data) public returns (bytes memory) {
        (bool success, bytes memory result) = 
            address(0x100).staticcall(abi.encode(data));
        require(success, "Encrypt failed");
        return result;
    }
    
    // 测试解密
    function testDecrypt(bytes memory encrypted) public returns (bytes memory) {
        (bool success, bytes memory result) = 
            address(0x101).staticcall(abi.encode(encrypted));
        require(success, "Decrypt failed");
        return result;
    }
    
    // 测试权限控制
    function testUnauthorizedDecrypt(bytes memory encrypted) public returns (bool) {
        // 应该失败，因为不是授权的合约
        (bool success, ) = address(0x101).staticcall(abi.encode(encrypted));
        return success;  // 预期为false
    }
}
```

#### E2E测试脚本
```bash
#!/bin/bash
# test/e2e/crypto_precompiles.sh

# 部署测试合约
CONTRACT=$(geth attach --exec "eth.contract(...).new()" /tmp/sgx-testnet/geth.ipc)

# 测试加密/解密
geth attach --exec "
    var plaintext = '0x48656c6c6f';  // 'Hello'
    var encrypted = contract.testEncrypt(plaintext);
    var decrypted = contract.testDecrypt(encrypted);
    console.log('Match:', plaintext == decrypted);
" /tmp/sgx-testnet/geth.ipc

# 测试权限控制
geth attach --exec "
    var result = contract.testUnauthorizedDecrypt(encrypted);
    console.log('Unauthorized blocked:', !result);
" /tmp/sgx-testnet/geth.ipc
```

**验收标准**:
- [ ] 加密/解密往返成功
- [ ] 未授权调用被正确拒绝
- [ ] 签名生成和验证正确
- [ ] Quote生成和验证正确
- [ ] 所有密码学操作的gas计量正确

---

### 任务3: 秘密数据同步验证

#### 多节点设置
```bash
# 节点1
geth --datadir /tmp/node1 --sgx --port 30303 --http --http.port 8545

# 节点2
geth --datadir /tmp/node2 --sgx --port 30304 --http --http.port 8546 \
     --bootnodes "enode://..."
```

#### E2E测试脚本
```bash
#!/bin/bash
# test/e2e/secret_data_sync.sh

# 1. 在节点1创建包含秘密数据的交易
geth attach http://localhost:8545 --exec "
    eth.sendTransaction({
        from: eth.accounts[0],
        to: '0x...',
        value: 0,
        data: '0x...'  // 包含秘密数据
    })
"

# 2. 等待区块传播
sleep 5

# 3. 在节点2查询区块
BLOCK_NUM=$(geth attach http://localhost:8546 --exec "eth.blockNumber")

# 4. 验证节点2能获取并解密秘密数据
geth attach http://localhost:8546 --exec "
    var block = eth.getBlock($BLOCK_NUM);
    var secretData = sgx.decryptBlockSecrets(block.hash);
    console.log('Secret data synced:', secretData != null);
"

# 5. 验证加密数据在区块中
# 6. 验证只有授权节点能解密
```

**验收标准**:
- [ ] 秘密数据随区块正确同步
- [ ] 授权节点能解密秘密数据
- [ ] 未授权节点无法解密
- [ ] 秘密数据不以明文形式传播

---

### 任务4: 治理合约功能验证

#### 合约部署
```bash
# 部署治理合约
geth attach --exec "
    var abi = [...];
    var bytecode = '0x...';
    var contract = eth.contract(abi);
    var tx = contract.new({from: eth.accounts[0], data: bytecode, gas: 3000000});
    var receipt = eth.getTransactionReceipt(tx.transactionHash);
    var governanceAddress = receipt.contractAddress;
" /tmp/sgx-testnet/geth.ipc
```

#### E2E测试脚本
```bash
#!/bin/bash
# test/e2e/governance_contract.sh

# 1. 测试白名单管理
geth attach --exec "
    // 添加MREnclave到白名单
    governance.addMREnclave('0x...', {from: admin});
    
    // 验证已添加
    var isWhitelisted = governance.isWhitelisted('0x...');
    console.log('Added to whitelist:', isWhitelisted);
    
    // 移除
    governance.removeMREnclave('0x...', {from: admin});
" /tmp/sgx-testnet/geth.ipc

# 2. 测试权限控制
geth attach --exec "
    // 非管理员尝试添加（应该失败）
    try {
        governance.addMREnclave('0x...', {from: nonAdmin});
        console.log('FAIL: Should have reverted');
    } catch(e) {
        console.log('PASS: Unauthorized access blocked');
    }
" /tmp/sgx-testnet/geth.ipc

# 3. 测试配置更新
geth attach --exec "
    governance.updateConfig(key, value, {from: admin});
    var retrieved = governance.getConfig(key);
    console.log('Config updated:', retrieved == value);
" /tmp/sgx-testnet/geth.ipc
```

**验收标准**:
- [ ] 白名单管理功能正常
- [ ] 权限控制正确实施
- [ ] 配置更新功能工作
- [ ] 与设计文档一致

---

## E2E测试执行流程

### 环境准备
```bash
# 1. 编译geth
make geth

# 2. 准备genesis文件
cat > genesis-sgx.json << EOF
{
  "config": {
    "chainId": 1337,
    "sgx": {
      "period": 0,
      "maxGasPerBlock": 30000000,
      "governanceContract": "0x...",
      "securityConfig": "0x..."
    }
  },
  "alloc": {
    "0x...": {"balance": "1000000000000000000000"}
  },
  "difficulty": "1",
  "gasLimit": "30000000"
}
EOF

# 3. 创建测试脚本目录
mkdir -p test/e2e
```

### 执行测试
```bash
# 运行所有E2E测试
./test/e2e/run_all.sh

# 或单独运行
./test/e2e/block_production.sh
./test/e2e/crypto_precompiles.sh  
./test/e2e/secret_data_sync.sh
./test/e2e/governance_contract.sh
```

### 验证报告
每个E2E测试应生成报告：
- 测试名称
- 执行步骤
- 实际结果
- 预期结果
- 通过/失败状态

---

## 测试环境

### 本地测试环境
- 单节点或小型网络
- 快速迭代
- 开发调试

### SGX测试环境
- 真实SGX硬件或模拟
- Gramine环境
- 完整安全特性

### 生产预演环境
- 多节点网络
- 负载测试
- 性能验证

---

## 成功标准

只有当以下所有条件满足时，任务才算完成：

1. **单元测试**: 所有Go测试通过
2. **集成测试**: 组件集成测试通过
3. **E2E测试**: 
   - 区块正常生产和传播
   - 密码学操作正确执行
   - 秘密数据正确同步
   - 治理合约按设计工作
4. **性能测试**: 满足性能要求
5. **安全审计**: 通过安全检查

**用户要求已明确理解和记录。** ✓
