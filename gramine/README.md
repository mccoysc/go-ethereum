# Gramine 开发工作流

此目录包含用于快速开发测试 X Chain 节点的 Gramine 配置和脚本。

**完整文档请查看**: [docs/modules/07-gramine-integration.md](../docs/modules/07-gramine-integration.md)

## 重要：编译环境一致性

**所有编译必须在 Gramine 官方镜像环境中进行**，以确保依赖库版本一致，避免运行时问题。

## 快速开始

```bash
# 1. 在 Gramine 环境中编译
./build-in-gramine.sh

# 2. 本地集成测试（在 Gramine 容器中直接运行）
./run-local.sh

# 3. Gramine 模拟器测试
./rebuild-manifest.sh dev
./run-dev.sh direct
```

## 文件说明

| 文件 | 用途 |
|------|------|
| `build-in-gramine.sh` | ⭐ 在 Gramine 容器中编译 geth |
| `run-local.sh` | ⭐ 本地集成测试（Gramine 容器直接运行） |
| `rebuild-manifest.sh` | 快速重新生成和签名 manifest |
| `run-dev.sh` | Gramine 运行（direct/sgx 模式） |
| `build-docker.sh` | 构建 Docker 镜像 |
| `push-docker.sh` | 推送到 GitHub Container Registry |
| `setup-signing-key.sh` | 管理签名密钥 |
| `start-xchain.sh` | Docker 容器启动脚本 |
| `geth.manifest.template` | Gramine manifest 模板 |
| `genesis-local.json` | 本地测试创世配置 |

## 快速参考

### 开发迭代
```bash
vim ../consensus/sgx/consensus.go  # 修改代码
./build-in-gramine.sh              # 重新编译（2分钟）
./run-local.sh                      # 测试（秒级）
```

### 测试层级
```bash
./run-local.sh           # 层级1: 本地集成（最快）
./run-dev.sh direct      # 层级2: Gramine 模拟器
./run-dev.sh sgx         # 层级3: SGX 真实环境
```

### 发布流程
```bash
./rebuild-manifest.sh prod   # 生产模式
./build-docker.sh v1.0.0     # 构建镜像
./push-docker.sh v1.0.0      # 推送到 ghcr.io
```

## 详细文档

完整的开发工作流、最佳实践、故障排除等详细信息，请查看：

📚 **[07-gramine-integration.md](../docs/modules/07-gramine-integration.md)**

包含：
- 完整的开发工作流说明
- 四层测试体系详解
- 开发模式 vs 生产模式
- Manifest 配置详解
- Docker 构建和发布
- 故障排除和最佳实践

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
