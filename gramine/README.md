# Gramine 开发工作流

此目录包含用于快速开发测试 X Chain 节点的 Gramine 配置和脚本。

## 重要：编译环境一致性

**所有编译必须在 Gramine 官方镜像环境中进行**，以确保依赖库版本一致，避免运行时问题。

## 快速开始

### 1. 在 Gramine 环境中编译 geth

```bash
cd gramine
./build-in-gramine.sh
```

这会在 `gramineproject/gramine:latest` 容器中编译 geth，确保与运行环境完全一致。

### 2. 本地集成测试（推荐）

在 Gramine 镜像容器中直接运行 geth（不使用 gramine-sgx 包装）：

```bash
./run-local.sh
```

**优点**：
- ✅ 在真实运行环境中测试
- ✅ 确保依赖库兼容
- ✅ 功能验证，SGX 使用 mock
- ✅ 快速迭代开发

### 3. 生成 Gramine manifest

```bash
./rebuild-manifest.sh dev
```

### 4. 运行节点

#### 模拟模式（gramine-direct）

使用 `gramine-direct` 在模拟器中运行，**无需 SGX 硬件**：

```bash
./run-dev.sh direct
```

#### SGX 模式（gramine-sgx）

使用 `gramine-sgx` 在真实 SGX enclave 中运行：

```bash
./run-dev.sh sgx
```

## 完整测试工作流

### 测试层级（按顺序）

1. **本地集成测试**（在 Gramine 容器中直接运行）
   ```bash
   ./build-in-gramine.sh    # 在 Gramine 环境编译
   ./run-local.sh           # 在 Gramine 容器测试
   ```
   - 验证功能正确性
   - 确保依赖兼容性
   - SGX 功能使用 mock

2. **gramine-direct 测试**（Gramine 模拟器）
   ```bash
   ./rebuild-manifest.sh dev
   ./run-dev.sh direct
   ```
   - 验证 Gramine 集成
   - 无需 SGX 硬件

3. **gramine-sgx 测试**（真实 SGX）
   ```bash
   ./rebuild-manifest.sh dev
   ./run-dev.sh sgx
   ```
   - 完整 SGX 功能测试
   - 需要 SGX 硬件

4. **Docker 集成测试**
   ```bash
   ./build-docker.sh
   docker run ghcr.io/mccoysc/xchain-node:dev direct
   ```

## 开发工作流

### 典型的开发迭代流程

1. **修改代码**
   ```bash
   vim ../consensus/sgx/consensus.go
   ```

2. **在 Gramine 环境重新编译**
   ```bash
   ./build-in-gramine.sh
   ```

3. **本地集成测试**
   ```bash
   ./run-local.sh
   ```

4. **通过后，测试 Gramine 集成**
   ```bash
   ./rebuild-manifest.sh dev
   ./run-dev.sh direct
   ```

### 为什么必须在 Gramine 环境编译？

❌ **错误做法**（本地编译）:
```bash
make geth  # 在本地环境编译
./run-dev.sh sgx  # 可能因依赖不兼容而失败
```

✅ **正确做法**（Gramine 环境编译）:
```bash
./build-in-gramine.sh  # 在 Gramine 容器中编译
./run-dev.sh sgx       # 依赖完全兼容
```

**原因**：
- Gramine 镜像使用特定版本的 glibc 和系统库
- 本地编译的二进制可能链接不同版本的库
- 会导致运行时错误或未定义行为

## 文件说明

- `geth.manifest.template` - Gramine manifest 模板文件
- `rebuild-manifest.sh` - 快速重新生成和签名 manifest
- `run-dev.sh` - 运行节点（支持 direct/sgx 模式）
- `setup-signing-key.sh` - 生成签名密钥
- `enclave-key.pem` - 签名密钥（自动生成，**不要提交到 Git**）
- `geth.manifest` - 生成的 manifest（自动生成）
- `geth.manifest.sgx` - 签名的 manifest（自动生成）
- `MRENCLAVE.txt` - MRENCLAVE 值（自动生成）

## 开发模式 vs 生产模式

### 开发模式（默认）

```bash
./rebuild-manifest.sh dev
```

**特性**：
- 使用 **MRSIGNER sealing**（基于签名者而非代码）
- Debug 模式启用
- 每次重新编译后**数据不需要迁移**（同一个签名密钥）
- 适合快速迭代开发

### 生产模式

```bash
./rebuild-manifest.sh prod
```

**特性**：
- 使用 **MRENCLAVE sealing**（基于代码度量值）
- Debug 模式关闭
- 每次代码改变后**需要数据迁移**
- 最高安全性

## 运行模式对比

| 特性 | gramine-direct | gramine-sgx |
|------|----------------|-------------|
| **需要 SGX 硬件** | ❌ 不需要 | ✅ 需要 |
| **启动速度** | 快 | 较慢 |
| **SGX 保护** | ❌ 无 | ✅ 有 |
| **远程证明** | ❌ 不支持 | ✅ 支持 |
| **加密分区** | 工作但不安全 | 完全安全 |
| **适用场景** | 功能开发、快速测试 | 安全测试、生产环境 |

## 常见问题

### Q: 为什么使用 MRSIGNER 而不是 MRENCLAVE？

**A**: 在开发模式下：
- **MRENCLAVE** 基于代码的哈希值，每次重新编译代码都会改变
- **MRSIGNER** 基于签名密钥，只要使用同一个密钥签名就不会改变
- 使用 MRSIGNER 可以避免每次重新编译后都要迁移加密数据

### Q: gramine-direct 模式安全吗？

**A**: 不安全，仅用于开发测试：
- 没有真实的 SGX 保护
- 没有远程证明
- 加密分区的密钥不受 SGX 保护

**生产环境必须使用 gramine-sgx 模式！**

### Q: 如何切换回生产模式？

**A**: 
```bash
./rebuild-manifest.sh prod
./run-dev.sh sgx
```

### Q: 签名密钥丢失了怎么办？

**A**: 
- 开发环境：重新生成密钥，但会丢失加密数据
- 生产环境：必须妥善备份密钥！

## 故障排除

### 问题：gramine-direct 命令找不到

```bash
# 安装 Gramine
sudo apt install gramine
```

### 问题：/dev/sgx_enclave 不存在

```bash
# 检查 SGX 支持
cpuid | grep SGX

# 安装 SGX 驱动
# 参考: https://github.com/intel/linux-sgx-driver
```

### 问题：权限不足

```bash
# 某些操作可能需要 sudo
sudo ./run-dev.sh sgx
```

## 相关文档

- [完整文档](../docs/modules/07-gramine-integration.md) - Gramine 集成模块详细文档
- [ARCHITECTURE.md](../ARCHITECTURE.md) - X Chain 整体架构
- [Gramine 官方文档](https://gramine.readthedocs.io/)
