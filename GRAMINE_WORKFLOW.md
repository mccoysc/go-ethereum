# X Chain Gramine 开发工作流完整指南

## 📖 概述

本指南提供了 X Chain 节点的完整开发、测试和部署工作流，解决了以下核心问题：

1. ✅ **快速迭代** - 无需每次重建 Docker 镜像
2. ✅ **环境一致** - 编译和运行都在 Gramine 环境
3. ✅ **无 SGX 开发** - 支持模拟器和本地测试
4. ✅ **自动发布** - CI/CD 集成到 GitHub

## 🚀 快速开始（30 秒）

```bash
cd gramine
./build-in-gramine.sh    # 在 Gramine 容器中编译
./run-local.sh           # 本地测试（Gramine 容器）
```

## 📂 文档结构

- **本文档** - 总体介绍和工作流概览
- `gramine/QUICKSTART.md` - 命令速查表和快速参考
- `gramine/README.md` - 详细开发指南
- `docs/modules/07-gramine-integration.md` - 完整技术文档

## 🎯 核心工作流

### 1. 本地开发迭代（最常用）

```bash
# 修改代码
vim consensus/sgx/consensus.go

# 在 Gramine 环境重新编译（重要！确保依赖一致）
cd gramine
./build-in-gramine.sh

# 本地集成测试（在 Gramine 容器中直接运行 geth）
./run-local.sh

# 验证通过后，测试 Gramine 集成
./rebuild-manifest.sh dev
./run-dev.sh direct
```

**时间**: 2-3 分钟（vs 传统方式 6-11 分钟）

### 2. 完整测试流程

```bash
# 层级 1: 本地集成测试
./build-in-gramine.sh
./run-local.sh
# ✅ 验证功能、确保依赖兼容、SGX mock

# 层级 2: Gramine direct 测试
./rebuild-manifest.sh dev
./run-dev.sh direct
# ✅ 验证 Gramine 集成、无需 SGX 硬件

# 层级 3: Gramine SGX 测试
./run-dev.sh sgx
# ✅ 真实 SGX 环境、完整功能

# 层级 4: Docker 测试
./build-docker.sh
docker run ghcr.io/mccoysc/xchain-node:dev direct
# ✅ 生产环境模拟
```

### 3. 版本发布流程

```bash
# 1. 切换到生产模式
./rebuild-manifest.sh prod

# 2. 完整测试
./run-dev.sh sgx

# 3. 构建 Docker 镜像
./build-docker.sh v1.0.0

# 4. 推送到 GitHub Container Registry
./push-docker.sh v1.0.0
```

**镜像**: `ghcr.io/mccoysc/xchain-node:v1.0.0`

## 💡 关键概念

### 为什么必须在 Gramine 环境编译？

❌ **错误做法**:
```bash
make geth                # 在本地编译
./run-dev.sh sgx        # 运行时依赖错误！
```

✅ **正确做法**:
```bash
./build-in-gramine.sh   # 在 Gramine 容器编译
./run-dev.sh sgx        # 完美运行
```

**原因**: Gramine 镜像使用特定版本的 glibc 和系统库，本地编译的二进制可能链接不兼容的库。

### 开发模式 vs 生产模式

| 特性 | 开发模式 | 生产模式 |
|------|---------|---------|
| Sealing | MRSIGNER | MRENCLAVE |
| 重编译后 | 数据无需迁移 | 数据需要迁移 |
| 安全性 | 中等 | 最高 |
| 适用场景 | 快速迭代 | 生产部署 |
| 命令 | `./rebuild-manifest.sh dev` | `./rebuild-manifest.sh prod` |

### 测试模式对比

| 模式 | 命令 | 需要 SGX | 速度 | 用途 |
|------|------|----------|------|------|
| 本地集成 | `./run-local.sh` | ❌ | 最快 | 功能验证 |
| gramine-direct | `./run-dev.sh direct` | ❌ | 快 | Gramine 集成 |
| gramine-sgx | `./run-dev.sh sgx` | ✅ | 慢 | 完整测试 |

## 📋 常用命令

### 编译
```bash
./build-in-gramine.sh      # 在 Gramine 容器中编译
```

### 测试
```bash
./run-local.sh             # 本地集成测试（推荐开始）
./run-dev.sh direct        # Gramine 模拟器
./run-dev.sh sgx           # SGX 真实环境
```

### Manifest
```bash
./rebuild-manifest.sh dev  # 开发模式（MRSIGNER）
./rebuild-manifest.sh prod # 生产模式（MRENCLAVE）
```

### Docker
```bash
./build-docker.sh v1.0.0   # 构建镜像
./push-docker.sh v1.0.0    # 推送到 ghcr.io
```

## 🐛 常见问题

### Q: 为什么不能用 `make geth` 编译？
A: 必须在 Gramine 环境编译以确保依赖一致性。使用 `./build-in-gramine.sh`。

### Q: 没有 SGX 硬件如何开发？
A: 使用 `./run-local.sh` 或 `./run-dev.sh direct`，都无需 SGX。

### Q: 如何快速测试代码改动？
A: 
```bash
./build-in-gramine.sh  # 重新编译（2分钟）
./run-local.sh         # 直接测试（秒级）
```

### Q: 重新编译后数据丢失？
A: 开发模式使用 MRSIGNER sealing，数据不会丢失。生产模式需要数据迁移。

### Q: Docker 镜像在哪里？
A: `ghcr.io/mccoysc/xchain-node:latest`

## 🔗 相关链接

- [快速参考](gramine/QUICKSTART.md) - 命令速查表
- [开发指南](gramine/README.md) - 详细文档
- [技术文档](docs/modules/07-gramine-integration.md) - 完整规范
- [GitHub Actions](.github/workflows/docker-build.yml) - CI/CD 配置

## 📊 性能提升

| 操作 | 传统方式 | 新方式 | 提升 |
|------|---------|--------|------|
| 开发迭代 | 6-11 分钟 | 2-3 分钟 | 66-73% |
| 更新 manifest | 5-10 分钟 | 5 秒 | 99% |
| 功能测试 | 需要 SGX | 无需硬件 | - |

## 🎯 下一步

1. 阅读 [快速参考](gramine/QUICKSTART.md) 了解命令
2. 尝试 `./build-in-gramine.sh && ./run-local.sh`
3. 查看 [README](gramine/README.md) 了解详细信息

---

**祝开发顺利！** 如有问题，请查阅文档或提交 Issue。
