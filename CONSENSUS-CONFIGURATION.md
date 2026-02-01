# 共识引擎配置说明

## 概述

Geth 现在支持三种共识引擎，可以通过 `genesis.json` 配置选择：

1. **SGX PoA** - 基于 SGX 的权威证明（包含模块 01-07）
2. **Clique PoA** - 标准的权威证明
3. **Ethash PoW** - 标准的工作量证明（仅用于测试）

## 配置方式

### 1. SGX PoA 共识

在 genesis.json 中配置 `sgx` 字段：

```json
{
  "config": {
    "chainId": 762385986,
    "homesteadBlock": 0,
    "eip150Block": 0,
    "eip155Block": 0,
    "eip158Block": 0,
    "byzantiumBlock": 0,
    "constantinopleBlock": 0,
    "petersburgBlock": 0,
    "istanbulBlock": 0,
    "berlinBlock": 0,
    "londonBlock": 0,
    "sgx": {
      "period": 5,
      "epoch": 30000,
      "governanceContract": "0x0000000000000000000000000000000000001001",
      "securityConfig": "0x0000000000000000000000000000000000001002",
      "incentiveContract": "0x0000000000000000000000000000000000001003"
    }
  }
}
```

**启用的模块**：
- ✅ Module 01: SGX 证明模块
- ✅ Module 02: SGX 共识引擎
- ✅ Module 03: 激励机制
- ✅ Module 04: 预编译合约 (0x8000-0x8009)
- ✅ Module 05: 治理系统
- ✅ Module 06: 加密存储
- ✅ Module 07: Gramine 集成

**启动日志**：
```
INFO [time] === Initializing SGX Consensus Engine ===
INFO [time] Loading Module 01: SGX Attestation
INFO [time] Loading Module 02: SGX Consensus Engine
INFO [time] Loading Module 03: Incentive Mechanism
INFO [time] Loading Module 04: Precompiled Contracts (0x8000-0x8009)
INFO [time] Loading Module 05: Governance System
INFO [time] Loading Module 06: Encrypted Storage
INFO [time] Loading Module 07: Gramine Integration
INFO [time] === SGX Consensus Engine Initialized ===
```

### 2. Clique PoA 共识

在 genesis.json 中配置 `clique` 字段：

```json
{
  "config": {
    "chainId": 1337,
    "clique": {
      "period": 15,
      "epoch": 30000
    }
  }
}
```

**启用的模块**：
- ❌ SGX 模块全部禁用
- ✅ 标准以太坊预编译合约

**启动日志**：
```
INFO [time] Using Clique PoA consensus (SGX modules disabled)
```

### 3. Ethash PoW 共识

在 genesis.json 中配置 `ethash` 字段：

```json
{
  "config": {
    "chainId": 1,
    "ethash": {}
  }
}
```

**启用的模块**：
- ❌ SGX 模块全部禁用
- ✅ 标准以太坊预编译合约

**启动日志**：
```
INFO [time] Using Ethash PoW consensus (SGX modules disabled)
```

## SGX 预编译合约

以下合约**只在 SGX 共识模式下可用**：

| 地址 | 合约名称 | 功能 | Gas 费用 |
|------|---------|------|---------|
| 0x8000 | SGX_KEY_CREATE | 创建密钥 | 50,000 |
| 0x8001 | SGX_KEY_GET_PUBLIC | 获取公钥 | 3,000 |
| 0x8002 | SGX_SIGN | 签名 | 10,000 |
| 0x8003 | SGX_VERIFY | 验证签名 | 5,000 |
| 0x8004 | SGX_ECDH | ECDH 密钥交换 | 20,000 |
| 0x8005 | SGX_RANDOM | 生成随机数 | 基础 2,000 |
| 0x8006 | SGX_ENCRYPT | 加密数据 | 5,000 + 数据费 |
| 0x8007 | SGX_DECRYPT | 解密数据 | 5,000 + 数据费 |
| 0x8008 | SGX_KEY_DERIVE | 派生密钥 | 10,000 |
| 0x8009 | SGX_KEY_DELETE | 删除密钥 | 5,000 |

## 配置参数说明

### SGX 配置

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `period` | uint64 | 是 | 出块周期（秒） |
| `epoch` | uint64 | 是 | Epoch 长度（区块数） |
| `governanceContract` | address | 是 | 治理合约地址 |
| `securityConfig` | address | 是 | 安全配置合约地址 |
| `incentiveContract` | address | 是 | 激励合约地址 |

### Clique 配置

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `period` | uint64 | 是 | 出块周期（秒） |
| `epoch` | uint64 | 是 | Epoch 长度（区块数） |

## 使用示例

### 初始化并启动 SGX 节点

```bash
# 1. 初始化创世区块
geth init test/integration/genesis-sgx.json --datadir ./data

# 2. 启动节点
geth --datadir ./data \
     --http \
     --http.addr 0.0.0.0 \
     --http.port 8545 \
     --http.api eth,net,web3 \
     --mine \
     --miner.etherbase 0xa875022f57343979503b4a95637315064eb01698

# 3. 验证 SGX 模块已加载
# 查看日志确认看到 "Loading Module 01-07" 的信息
```

### 初始化并启动 Clique 节点

```bash
# 1. 初始化创世区块
geth init genesis-clique.json --datadir ./data

# 2. 启动节点
geth --datadir ./data \
     --http \
     --http.addr 0.0.0.0 \
     --http.port 8545 \
     --http.api eth,net,web3

# 3. 验证 Clique 模式
# 查看日志确认看到 "Using Clique PoA consensus (SGX modules disabled)"
```

## 运行时验证

### 检查当前共识引擎

```javascript
// 通过 geth console 连接
geth attach http://localhost:8545

// 检查链 ID
> eth.chainId()
762385986  // SGX 链
1337       // Clique 链

// 尝试调用 SGX 预编译合约（只在 SGX 模式下有效）
> eth.call({to: "0x8000000000000000000000000000000000000000", data: "0x00"})
// SGX 模式: 返回结果
// Clique 模式: 返回错误或空
```

### 检查 IsSGX 标志

可以通过查看节点日志或运行测试来确认：

```bash
# 运行 SGX 预编译合约测试
go test -v -run TestSGXPrecompilesConditionalActivation ./core/vm
```

## 注意事项

1. **互斥性**: 每个 genesis.json 只能配置一种共识引擎
   - ✅ 可以: `{"sgx": {...}}`
   - ✅ 可以: `{"clique": {...}}`
   - ❌ 错误: `{"sgx": {...}, "clique": {...}}`

2. **模块隔离**: 
   - SGX 模块只在 SGX 共识时加载
   - 其他共识不会加载任何 SGX 相关代码
   - 不会产生额外的性能开销

3. **向后兼容**:
   - 原有的 Clique 和 Ethash 配置仍然有效
   - SGX 是新增的可选共识引擎

4. **合约地址**: 
   - 预编译合约地址 0x8000-0x8009 在所有链上保留
   - 只在 SGX 模式下这些地址才有功能

## 测试验证

所有功能都有完整的单元测试覆盖：

```bash
# 测试 SGX 预编译合约条件激活
go test -v -run TestSGXPrecompilesConditionalActivation ./core/vm

# 测试所有 SGX 预编译合约
go test -v ./core/vm -run TestSGX

# 测试共识引擎
go test -v ./consensus/sgx
```

## 文件位置

- **Genesis 示例**: `test/integration/genesis-sgx.json`
- **共识配置**: `params/config.go` (SGXConfig 结构)
- **引擎创建**: `eth/ethconfig/config.go` (CreateConsensusEngine 函数)
- **预编译合约**: `core/vm/contracts.go` (PrecompiledContractsSGX)
- **规则定义**: `params/config.go` (Rules.IsSGX 字段)

## 总结

通过 genesis.json 配置，可以灵活选择共识引擎：

- **需要 SGX 功能** → 配置 `"sgx": {...}`，所有模块 01-07 自动加载
- **需要标准 PoA** → 配置 `"clique": {...}`，SGX 模块禁用
- **需要 PoW 测试** → 配置 `"ethash": {}`，SGX 模块禁用

所有配置都经过测试验证，确保模块正确加载和隔离。
