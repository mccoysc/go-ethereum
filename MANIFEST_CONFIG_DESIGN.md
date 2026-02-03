# Manifest配置读取设计

## 问题

如何安全地从Gramine manifest获取应用配置？

## 错误的方法（之前的实现）

```
1. 从磁盘读manifest.sgx文件
2. 尝试验证MRENCLAVE
3. 解析TOML内容
```

**问题**：
- 即使MRENCLAVE匹配，也不能保证TOML内容完整性
- 需要完整重新计算MRENCLAVE（复杂且易错）
- 过度复杂

## 正确的方法

### Manifest定义（geth.manifest.toml）

```toml
loader.env.GOVERNANCE_CONTRACT = "0xd9145CCE52D386f254917e481eB44e9943F39138"
loader.env.SECURITY_CONFIG_CONTRACT = "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"
loader.env.NODE_TYPE = "validator"
```

### 应用代码读取

```go
config, err := GetAppConfigFromEnvironment()
// config.GovernanceContract
// config.SecurityConfigContract
// config.NodeType
```

## 工作流程

```
1. Gramine启动
   ├─ 读取manifest.sgx
   ├─ 验证SIGSTRUCT签名
   ├─ 重新计算MRENCLAVE
   ├─ 验证MRENCLAVE匹配
   └─ 设置环境变量（loader.env中的）

2. 应用启动
   ├─ 从环境变量读取配置
   ├─ 检查RA_TLS_MRENCLAVE存在（确认在SGX中）
   └─ 使用配置
```

## 安全保证

- Gramine验证了manifest（签名+MRENCLAVE）
- 环境变量在SGX保护的内存中
- 应用不需要重新验证
- 不需要读取文件

## 实现

见`internal/sgx/manifest_parser.go`：
- `GetAppConfigFromEnvironment()` - 唯一的配置读取方式
