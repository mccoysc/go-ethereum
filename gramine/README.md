# Gramine 开发工作流

此目录包含用于快速开发测试 X Chain 节点的 Gramine 配置和脚本。

## 快速开始

### 1. 编译 geth

```bash
cd ..
make geth
```

### 2. 生成 Gramine manifest

```bash
cd gramine
./rebuild-manifest.sh dev
```

### 3. 运行节点

#### 模拟模式（推荐用于开发测试）

使用 `gramine-direct` 在模拟器中运行，**无需 SGX 硬件**：

```bash
./run-dev.sh direct
```

**优点**：
- ✅ 无需 SGX 硬件
- ✅ 快速启动和测试
- ✅ 适合功能开发和调试

**限制**：
- ❌ 无真实 SGX 保护
- ❌ 无远程证明
- ❌ 加密分区仍然工作，但安全性降低

#### SGX 模式（用于完整测试）

使用 `gramine-sgx` 在真实 SGX enclave 中运行：

```bash
./run-dev.sh sgx
```

**要求**：
- ✅ CPU 支持 SGX
- ✅ BIOS 中启用 SGX
- ✅ 安装了 SGX 驱动

## 开发工作流

### 典型的开发迭代流程

1. **修改代码**
   ```bash
   # 编辑 go-ethereum 源代码
   vim ../consensus/sgx/consensus.go
   ```

2. **重新编译 geth**
   ```bash
   cd ..
   make geth
   cd gramine
   ```

3. **快速重新生成 manifest**（只需几秒钟）
   ```bash
   ./rebuild-manifest.sh dev
   ```

4. **测试运行**
   ```bash
   # 使用模拟模式快速测试
   ./run-dev.sh direct
   
   # 或使用 SGX 模式完整测试
   ./run-dev.sh sgx
   ```

### 为什么这种方式更快？

**传统方式**（慢）：
```bash
# 每次都要重建整个 Docker 镜像（5-10分钟）
docker build -t xchain-node:latest .
docker run xchain-node:latest
```

**新方式**（快）：
```bash
# 只重新编译 geth（30秒）+ 重新生成 manifest（5秒）
make geth
./rebuild-manifest.sh dev
./run-dev.sh direct
```

**时间节省**：从 5-10 分钟降低到 30-40 秒！

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
