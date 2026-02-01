# X Chain 开发工作流快速参考

## 🚀 快速开始（3 步）

```bash
cd gramine
./build-in-gramine.sh    # 1. 在 Gramine 环境编译
./run-local.sh           # 2. 本地测试
```

## 📋 命令速查表

### 编译

| 命令 | 说明 | 环境 |
|------|------|------|
| `./build-in-gramine.sh` | ⭐ 在 Gramine 容器编译 | Gramine 镜像 |
| `make geth` | ❌ 本地编译（不推荐） | 本地环境 |

### 测试

| 命令 | 模式 | 需要 SGX | 说明 |
|------|------|----------|------|
| `./run-local.sh` | 本地集成 | ❌ | 在 Gramine 容器直接运行 geth |
| `./run-dev.sh direct` | gramine-direct | ❌ | Gramine 模拟器 |
| `./run-dev.sh sgx` | gramine-sgx | ✅ | 真实 SGX enclave |

### Manifest

| 命令 | 模式 | Sealing |
|------|------|---------|
| `./rebuild-manifest.sh dev` | 开发 | MRSIGNER（无需数据迁移） |
| `./rebuild-manifest.sh prod` | 生产 | MRENCLAVE（最高安全） |

### Docker

| 命令 | 说明 |
|------|------|
| `./build-docker.sh v1.0.0` | 构建镜像（自动在 Gramine 环境编译） |
| `./push-docker.sh v1.0.0` | 推送到 ghcr.io |

## 🔄 典型工作流

### 日常开发
```bash
# 编辑代码
vim ../consensus/sgx/consensus.go

# 在 Gramine 环境重新编译
./build-in-gramine.sh

# 本地测试
./run-local.sh

# Gramine 测试
./rebuild-manifest.sh dev
./run-dev.sh direct
```

### 发布版本
```bash
# 完整测试
./build-in-gramine.sh
./run-local.sh
./rebuild-manifest.sh prod
./run-dev.sh sgx

# 构建和发布
./build-docker.sh v1.0.0
./push-docker.sh v1.0.0
```

## 💡 关键要点

1. ✅ **必须在 Gramine 环境编译**
   - 避免依赖不兼容
   - 使用 `./build-in-gramine.sh`

2. ✅ **先本地测试，再 Gramine 测试**
   - `./run-local.sh` → `./run-dev.sh direct` → `./run-dev.sh sgx`

3. ✅ **开发用 MRSIGNER，生产用 MRENCLAVE**
   - 开发：`./rebuild-manifest.sh dev`（避免数据迁移）
   - 生产：`./rebuild-manifest.sh prod`（最高安全）

4. ✅ **Docker 自动处理编译**
   - `./build-docker.sh` 自动在 Gramine 环境编译
   - 不需要手动 `make geth`

## 🐛 故障排除

### 运行时依赖错误
```bash
# 问题：本地编译的 geth 在 Gramine 环境报错
# 解决：必须在 Gramine 环境重新编译
./build-in-gramine.sh
```

### SGX 设备不存在
```bash
# 问题：/dev/sgx_enclave 不存在
# 解决：使用 gramine-direct 或 run-local.sh
./run-dev.sh direct  # 或
./run-local.sh
```

### Docker 权限问题
```bash
# 添加用户到 docker 组
sudo usermod -aG docker $USER
newgrp docker
```

## 📦 文件说明

- `build-in-gramine.sh` - 在 Gramine 容器中编译（重要！）
- `run-local.sh` - 本地集成测试（Gramine 容器直接运行）
- `run-dev.sh` - Gramine 模式运行（direct/sgx）
- `rebuild-manifest.sh` - 快速重生成 manifest
- `build-docker.sh` - 构建 Docker 镜像
- `push-docker.sh` - 推送到 GitHub Registry
- `start-xchain.sh` - Docker 容器启动脚本

## 🌐 拉取和运行镜像

```bash
# 拉取
docker pull ghcr.io/mccoysc/xchain-node:latest

# SGX 模式
docker run -d --name xchain \
  --device=/dev/sgx_enclave \
  --device=/dev/sgx_provision \
  -v /var/run/aesmd:/var/run/aesmd \
  -v $(pwd)/data:/data \
  -p 8545:8545 -p 8546:8546 -p 30303:30303 \
  ghcr.io/mccoysc/xchain-node:latest sgx

# Direct 模式
docker run -d --name xchain \
  -v $(pwd)/data:/data \
  -p 8545:8545 -p 8546:8546 -p 30303:30303 \
  ghcr.io/mccoysc/xchain-node:latest direct
```
