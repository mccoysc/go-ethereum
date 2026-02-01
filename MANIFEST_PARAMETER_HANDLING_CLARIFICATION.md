# Manifest 参数处理机制澄清

## 问题背景

对于模块 06 文档以及根目录下的主架构文档提到的安全参数的问题，如果模块启动时会读 manifest 且以 manifest 为准，是不是就该忽略命令行传入或者用户外部环境指定的同名参数/变量，根本无需与用户的比对？

## 解答

**是的，应该直接忽略，无需比对。**

## 设计理念

### 方案 A（不推荐，文档已修正）

- 启动时读取 Manifest 参数
- 读取命令行参数
- **比对两者，如果不一致则退出进程**

### 方案 B（推荐，文档已采用）

- 启动时读取 Manifest 参数
- 读取命令行参数
- **Manifest 参数直接作为最终值，命令行同名参数被忽略，无需比对**

## 设计理由

1. **Manifest 的不可篡改性**：
   - Manifest 参数嵌入在 enclave 镜像中
   - 影响 MRENCLAVE 度量值
   - 外部无法篡改

2. **简化实现**：
   - 无需比对逻辑
   - 避免不必要的进程退出
   - 代码更简洁

3. **权威性原则**：
   - Manifest 是唯一权威来源
   - 用户输入不应影响安全参数
   - 更符合"以 Manifest 为准"的原则

## 文档更改摘要

### 1. 参数处理流程

**方案 A（文档原描述，不推荐）**：
```
1. 加载 Manifest 参数
2. 加载 CLI 参数
3. 合并参数：Manifest 覆盖 CLI，不一致则退出进程
```

**方案 B（文档已修正，推荐）**：
```
1. 加载 Manifest 参数
2. 加载 CLI 参数
3. 合并参数：Manifest 直接作为最终值，CLI 同名参数被忽略
```

### 2. MergeAndValidate 函数

**方案 A（文档原描述）**：
```go
// 检查是否与 Manifest 一致
if ok && cliValue != manifestValue {
    return fmt.Errorf("SECURITY VIOLATION: ...")
}
```

**方案 B（文档已修正）**：
```go
// 如果 Manifest 中已存在，直接忽略 CLI 参数
if _, exists := pv.manifestParams[param.Name]; exists {
    goto nextParam
}
```

### 3. 章节标题

- "参数校验机制" → "参数处理机制"
- "启动时参数校验" → "启动时参数处理"
- "参数校验实现" → "参数处理实现"

### 4. 注意事项

**方案 A（文档原描述）**：
> 参数校验：安全参数必须与 Manifest 一致，不一致则退出进程

**方案 B（文档已修正）**：
> 参数处理：安全参数以 Manifest 为准，命令行传入的同名参数被忽略，无需比对

## 影响范围

### 已修正的文档

- `/docs/modules/06-data-storage-sync.md`：所有相关章节已修正

### 未实现的代码

**重要说明**：本次更改仅涉及文档设计，暂未涉及具体实现代码。

根据文档，以下代码需要实现（待后续实现）：
- `/config/param_validator.go`：参数处理器实现
- `/config/param_validator_test.go`：单元测试
- `/cmd/geth/main.go`：集成参数处理器

## 后续工作

如果需要实现代码，应遵循以下原则：

1. **Manifest 优先**：安全参数始终使用 Manifest 中的值
2. **静默忽略**：CLI 同名参数被静默忽略，无警告
3. **无需比对**：不进行一致性检查，不退出进程
4. **清晰日志**：记录 Manifest 参数加载情况，便于调试

## 示例场景

### 场景 1：Manifest 和 CLI 都设置了安全参数

```bash
# Manifest 中设置
XCHAIN_ENCRYPTED_PATH=/data/encrypted

# 用户运行
geth --encrypted-path=/custom/path

# 结果：使用 /data/encrypted（Manifest 值）
# 行为：静默忽略 CLI 参数，无警告
```

### 场景 2：只有 CLI 设置了安全参数

```bash
# Manifest 中未设置
# XCHAIN_ENCRYPTED_PATH=

# 用户运行
geth --encrypted-path=/custom/path

# 结果：使用 /custom/path（CLI 值）
# 行为：Manifest 未定义时，可以使用 CLI 参数
```

### 场景 3：非安全参数

```bash
# 用户运行
geth --datadir=/tmp/data --rpc-port=8545

# 结果：使用 CLI 值
# 行为：非安全参数正常处理
```

## 总结

这次文档修正澄清了 Manifest 参数处理的设计理念：

- ✅ **简化**：去除不必要的比对和退出逻辑
- ✅ **明确**：Manifest 是唯一权威来源
- ✅ **一致**：所有文档术语统一为"处理"而非"校验"
- ✅ **实用**：避免用户因参数不一致而导致进程意外退出

---

**文档修正日期**：2026-02-01  
**相关 PR**：copilot/clarify-manifest-parameter-handling
