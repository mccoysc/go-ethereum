# Gramine 集成模块开发文档

## 模块概述

**Gramine 集成模块是将所有 01～06 模块以及整体 Geth 集成到 Gramine SGX 环境中运行的完整集成方案**。该模块不仅提供 SGX 运行环境配置，更重要的是定义了各个模块如何在 Gramine 环境中协同工作，形成完整的 X Chain 节点。

### 集成方案核心职责

1. **环境准备**：配置 Gramine manifest，使 Geth 及所有模块能在 SGX enclave 中运行
2. **模块整合**：将 SGX 证明、共识引擎、预编译合约、治理、激励、存储等模块整合为统一的运行时
3. **安全保障**：通过加密分区、参数固化、MRENCLAVE 绑定等机制确保整体安全性
4. **部署方案**：提供从 Docker 镜像构建到节点启动的完整部署流程
5. **验证机制**：确保各模块在 SGX 环境中正常协作

### 模块集成关系

```
                     Gramine 集成模块 (07)
                    /       |        \
                   /        |         \
                  /         |          \
          模块01-06     整体Geth    SGX环境
              |            |            |
              v            v            v
        ┌─────────────────────────────────────────┐
        │   完整的 X Chain 节点 (SGX Enclave)     │
        │                                         │
        │  01-SGX证明 ──> P2P RA-TLS 握手        │
        │  02-共识引擎 ──> PoA-SGX 出块          │
        │  03-激励机制 ──> 交易费分配            │
        │  04-预编译合约 ──> 密钥管理            │
        │  05-治理模块 ──> 白名单管理            │
        │  06-存储模块 ──> 加密分区访问          │
        │                                         │
        │  Geth 核心 ──> EVM + StateDB + P2P    │
        └─────────────────────────────────────────┘
```

此外，该模块与 SGX 共识引擎、RA-TLS 通道密切协作，并支持整体架构中的加密分区功能，确保了系统安全性与数据一致性。

## 系统架构背景

以下是 Gramine 集成模块在整个 X Chain 系统中的架构背景：

```
+------------------------------------------------------------------+
|                        X Chain 节点                               |
|  +------------------------------------------------------------+  |
|  |                    SGX Enclave (Gramine)                   |  |
|  |  +------------------------------------------------------+  |  |
|  |  |                   修改后的 Geth                       |  |  |
|  |  |  +----------------+  +----------------+              |  |  |
|  |  |  | SGX 共识引擎   |  | 预编译合约     |              |  |  |
|  |  |  | (PoA-SGX)      |  | (密钥管理)     |              |  |  |
|  |  |  +----------------+  +----------------+              |  |  |
|  |  |  +----------------+  +----------------+              |  |  |
|  |  |  | P2P 网络层     |  | EVM 执行层     |              |  |  |
|  |  |  | (RA-TLS)       |  |                |              |  |  |
|  |  |  +----------------+  +----------------+              |  |  |
|  |  +------------------------------------------------------+  |  |
|  |                           |                                |  |
|  |  +------------------------------------------------------+  |  |
|  |  |              Gramine 加密分区                         |  |  |
|  |  |  - 私钥存储                                          |  |  |
|  |  |  - 派生秘密 (ECDH 结果等)                            |  |  |
|  |  |  - 区块链数据                                        |  |  |
|  |  +------------------------------------------------------+  |  |
|  +------------------------------------------------------------+  |
+------------------------------------------------------------------+
```

## 负责团队

**DevOps/基础设施团队**

## 模块职责

作为集成方案，本模块的核心职责包括：

### 1. 环境配置与准备
- Gramine manifest 模板配置
- 安全参数嵌入（影响度量值 MRENCLAVE）
- 加密文件系统配置
- SGX 硬件支持检测

### 2. 模块整合与集成
- **整合 01-SGX 证明模块**：配置 RA-TLS 库和证书验证
- **整合 02-共识引擎模块**：确保 PoA-SGX 共识在 enclave 中运行
- **整合 03-激励机制模块**：支持激励计算在 TEE 中执行
- **整合 04-预编译合约模块**：配置加密分区存储私钥
- **整合 05-治理模块**：固化治理合约地址，读取链上白名单
- **整合 06-数据存储模块**：实现三层参数校验机制

### 3. 部署与运维
- Docker 镜像构建流程
- 启动脚本和部署配置
- 节点运行状态监控
- 模块功能验证机制

### 4. 安全保障
- MRENCLAVE 度量值绑定
- 合约地址固化防篡改
- 参数优先级控制（Manifest > 链上 > 命令行）
- 端到端集成测试

## 依赖关系

### 作为集成方案的依赖结构

```
                 +--------------------------------+
                 |   Gramine 集成模块 (07)         |
                 |   - Manifest 配置              |
                 |   - Docker 构建                |
                 |   - 启动流程                   |
                 +--------------------------------+
                        |      |      |
        +---------------+      |      +----------------+
        |                      |                       |
        v                      v                       v
+----------------+    +----------------+    +------------------+
| 01-SGX证明模块  |    | 02-共识引擎     |    | 03-激励机制       |
| (RA-TLS)       |    | (PoA-SGX)      |    | (奖励分配)        |
+----------------+    +----------------+    +------------------+
        |                      |                       |
        +----------------------+------------------------+
                               |
        +----------------------+------------------------+
        |                      |                       |
        v                      v                       v
+----------------+    +----------------+    +------------------+
| 04-预编译合约   |    | 05-治理模块     |    | 06-存储模块       |
| (密钥管理)     |    | (白名单管理)    |    | (加密分区)        |
+----------------+    +----------------+    +------------------+
        |                      |                       |
        +----------------------+------------------------+
                               |
                               v
                 +--------------------------------+
                 |     完整的 X Chain 节点         |
                 | (所有模块在 Gramine 中运行)     |
                 +--------------------------------+
```

### 集成依赖说明

**07 模块整合所有模块**：
- **SGX 证明模块（01）**：配置 RA-TLS 库路径、环境变量
- **共识引擎模块（02）**：固化共识参数、确保在 enclave 中执行
- **激励机制模块（03）**：支持激励计算的可信执行
- **预编译合约模块（04）**：配置加密分区以存储私钥
- **治理模块（05）**：固化治理合约地址、读取链上配置
- **数据存储模块（06）**：实现三层参数校验、加密分区管理

### 上游依赖
- Intel SGX SDK/PSW
- Gramine LibOS
- Docker

### 下游依赖（被以下模块使用）
- 所有其他模块（运行时环境）

## Manifest 配置

### 基础 Manifest 模板

```toml
# geth.manifest.template
# X Chain 节点 Gramine 配置

# 基础配置
libos.entrypoint = "/app/geth"
loader.log_level = "{{ log_level }}"

# 预加载库
loader.preload = "file:{{ gramine.libdir }}/libsysdb.so"

# 环境变量
loader.env.LD_LIBRARY_PATH = "/lib:{{ arch_libdir }}:/usr/lib:/usr/{{ arch_libdir }}"
loader.env.HOME = "/app"
loader.env.PATH = "/bin:/usr/bin"

# 安全相关参数（影响度量值 MRENCLAVE）
[loader.env]
# 注意：白名单不应放在环境变量中，应从链上合约动态读取
# 以下是本节点自身的固定配置

# 本地路径配置
XCHAIN_ENCRYPTED_PATH = "/data/encrypted"    # 加密分区路径
XCHAIN_SECRET_PATH = "/data/secrets"         # 秘密数据存储路径

# 链上合约地址（写死，作为安全锚点）
# 合约地址影响 MRENCLAVE，攻击者无法修改合约地址而不改变度量值
XCHAIN_GOVERNANCE_CONTRACT = "{{ governance_contract }}"
XCHAIN_SECURITY_CONFIG_CONTRACT = "{{ security_config_contract }}"

# SGX 配置
[sgx]
debug = false
enclave_size = "2G"
max_threads = 32
isvprodid = 1
isvsvn = 1

# 远程证明配置
remote_attestation = "dcap"

# 可信文件
[sgx.trusted_files]
geth = "file:/app/geth"
ld = "file:{{ gramine.libdir }}/ld-linux-x86-64.so.2"
libc = "file:{{ gramine.libdir }}/libc.so.6"
libpthread = "file:{{ gramine.libdir }}/libpthread.so.0"
libdl = "file:{{ gramine.libdir }}/libdl.so.2"
libm = "file:{{ gramine.libdir }}/libm.so.6"
librt = "file:{{ gramine.libdir }}/librt.so.1"
libgcc = "file:{{ arch_libdir }}/libgcc_s.so.1"
libstdc = "file:/usr/lib/x86_64-linux-gnu/libstdc++.so.6"

# 文件系统挂载
[[fs.mounts]]
type = "chroot"
path = "/lib"
uri = "file:{{ gramine.runtimedir() }}"

[[fs.mounts]]
type = "chroot"
path = "{{ arch_libdir }}"
uri = "file:{{ arch_libdir }}"

[[fs.mounts]]
type = "chroot"
path = "/usr/{{ arch_libdir }}"
uri = "file:/usr/{{ arch_libdir }}"

[[fs.mounts]]
type = "chroot"
path = "/app"
uri = "file:/app"

# 加密分区挂载（核心安全功能）
[[fs.mounts]]
type = "encrypted"
path = "/data/encrypted"
uri = "file:/data/encrypted"
key_name = "_sgx_mrenclave"  # 使用 MRENCLAVE 进行数据封装

[[fs.mounts]]
type = "encrypted"
path = "/data/secrets"
uri = "file:/data/secrets"
key_name = "_sgx_mrenclave"

# 区块链数据目录
[[fs.mounts]]
type = "encrypted"
path = "/app/wallet"
uri = "file:/data/wallet"
key_name = "_sgx_mrenclave"

# 允许写入的日志目录
[[fs.mounts]]
type = "chroot"
path = "/app/logs"
uri = "file:/app/logs"
```

### MRENCLAVE vs MRSIGNER 封装策略

| 特性 | MRENCLAVE sealing | MRSIGNER sealing |
|------|-------------------|------------------|
| 安全性 | 更高（代码绑定） | 较低（签名者绑定） |
| 升级便利性 | 需要数据迁移 | 无需迁移 |
| 适用场景 | 高安全要求 | 频繁升级场景 |
| 回滚风险 | 低 | 旧版本可访问新数据 |

**推荐策略**：
- **生产环境**：使用 MRENCLAVE sealing + 数据迁移机制
- **测试环境**：可使用 MRSIGNER sealing 简化升级流程
- **混合策略**：核心私钥使用 MRENCLAVE，临时数据使用 MRSIGNER

### RA-TLS 配置

```toml
# RA-TLS 环境变量配置
loader.env.RA_TLS_MRENCLAVE = "{{ mrenclave }}"
loader.env.RA_TLS_MRSIGNER = "{{ mrsigner }}"
loader.env.RA_TLS_ISV_PROD_ID = "1"
loader.env.RA_TLS_ISV_SVN = "1"

# RA-TLS 库路径
loader.env.RA_TLS_ALLOW_OUTDATED_TCB_INSECURE = "0"
loader.env.RA_TLS_ALLOW_DEBUG_ENCLAVE_INSECURE = "0"
```

## Docker 镜像构建

### 最终输出与运行状态

**本模块的最终输出是一个完整的 Docker 镜像**，该镜像包含：
1. 编译、签名好的 Gramine manifest 文件（`geth.manifest.sgx`）
2. 所有依赖的可信文件（geth 二进制、库文件等）
3. 完整的 Gramine 运行时环境
4. 可直接被 `gramine-sgx` 或 `gramine-direct` 运行

**镜像基础**：使用 Gramine 官方最新 Docker 运行时镜像（`gramineproject/gramine:latest`）

**运行后的状态**：
- ✅ **X Chain 节点运行在 SGX Enclave 环境中**（通过 Gramine）
- ✅ **满足 ARCHITECTURE.md 的所有架构设计要求**
- ✅ **整合了所有 01-06 模块的功能**
  - 01-SGX 证明模块：RA-TLS 双向认证
  - 02-共识引擎模块：PoA-SGX 共识机制
  - 03-激励机制模块：交易费分配和奖励计算
  - 04-预编译合约模块：密钥管理和密码学操作
  - 05-治理模块：MRENCLAVE 白名单和验证者管理
  - 06-数据存储模块：加密分区和参数校验
- ✅ **形成一个完整的、符合架构要求的 X Chain 节点**

### Dockerfile 示例

```dockerfile
# Dockerfile
# 使用 Gramine 官方最新运行时镜像作为基础镜像
FROM gramineproject/gramine:latest

# 设置工作目录
WORKDIR /app

# 安装额外依赖（如果需要）
RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 复制编译好的 geth 二进制文件
COPY ./build/bin/geth /app/geth
RUN chmod +x /app/geth

# 复制 Gramine manifest 模板
COPY ./gramine/geth.manifest.template /app/geth.manifest.template

# 设置 manifest 模板参数（影响 MRENCLAVE）
# 这些合约地址应从创世配置中确定性计算得出
ARG GOVERNANCE_CONTRACT=0x0000000000000000000000000000000000001001
ARG SECURITY_CONFIG_CONTRACT=0x0000000000000000000000000000000000001002

# 生成 manifest 文件
RUN gramine-manifest \
    -Dlog_level=error \
    -Darch_libdir=/lib/x86_64-linux-gnu \
    -Dgovernance_contract=${GOVERNANCE_CONTRACT} \
    -Dsecurity_config_contract=${SECURITY_CONFIG_CONTRACT} \
    geth.manifest.template geth.manifest

# 签名 manifest（生成 MRENCLAVE 度量值）
# 注意：签名密钥应该妥善管理，这里使用构建时生成的密钥
RUN gramine-sgx-sign \
    --manifest geth.manifest \
    --output geth.manifest.sgx

# 提取 MRENCLAVE 值（用于验证和白名单配置）
RUN gramine-sgx-sigstruct-view geth.manifest.sgx | grep MRENCLAVE | awk '{print $2}' > /app/MRENCLAVE.txt

# 创建数据目录
RUN mkdir -p /data/encrypted /data/secrets /data/wallet /app/logs

# 复制启动脚本
COPY ./gramine/start-xchain.sh /app/start-xchain.sh
RUN chmod +x /app/start-xchain.sh

# 暴露端口
EXPOSE 8545 8546 30303

# 设置入口点
# 注意：容器启动时会使用 gramine-sgx 或 gramine-direct 运行
ENTRYPOINT ["/app/start-xchain.sh"]
```

### 镜像构建说明

**使用 Gramine 官方镜像的优势**：
1. ✅ **预装 Gramine 运行时**：无需手动安装 Gramine
2. ✅ **预装 SGX 库**：包含必要的 SGX DCAP 库
3. ✅ **定期更新**：跟随 Gramine 官方更新获得安全补丁
4. ✅ **减小镜像体积**：避免重复安装基础组件
5. ✅ **官方支持**：使用官方测试和验证的环境

**关键构建步骤**：
1. **FROM gramineproject/gramine:latest** - 使用官方最新运行时镜像
2. **COPY geth binary** - 复制编译好的 geth 可执行文件
3. **gramine-manifest** - 生成 manifest（嵌入合约地址等参数）
4. **gramine-sgx-sign** - 签名 manifest 生成 MRENCLAVE
5. **提取 MRENCLAVE** - 保存度量值用于白名单配置

### 构建流程脚本

```bash
#!/bin/bash
# build-docker.sh

set -e

# 1. 编译 geth
echo "Building geth..."
make geth

# 2. 确定合约地址（从创世配置计算）
GOVERNANCE_CONTRACT="0x0000000000000000000000000000000000001001"
SECURITY_CONFIG_CONTRACT="0x0000000000000000000000000000000000001002"

echo "Governance Contract: ${GOVERNANCE_CONTRACT}"
echo "Security Config Contract: ${SECURITY_CONFIG_CONTRACT}"

# 3. 构建 Docker 镜像
echo "Building Docker image..."
docker build \
    --build-arg GOVERNANCE_CONTRACT=${GOVERNANCE_CONTRACT} \
    --build-arg SECURITY_CONFIG_CONTRACT=${SECURITY_CONFIG_CONTRACT} \
    -t xchain-node:latest \
    -f Dockerfile.xchain \
    .

# 4. 提取 MRENCLAVE（用于白名单配置）
echo "Extracting MRENCLAVE..."
MRENCLAVE=$(docker run --rm xchain-node:latest gramine-sgx-sigstruct-view geth.manifest.sgx | grep MRENCLAVE | awk '{print $2}')
echo "MRENCLAVE: ${MRENCLAVE}"

# 保存 MRENCLAVE 到文件
echo ${MRENCLAVE} > mrenclave.txt
echo "MRENCLAVE saved to mrenclave.txt"

echo "Build complete!"
```

## 部署和启动

### 运行后的节点状态

**通过本模块部署和启动后，您将获得一个完整的 X Chain 节点**，该节点：

1. **运行在 SGX Enclave 环境中**
   - 通过 Gramine LibOS 在可信执行环境 (TEE) 中运行
   - 所有代码和数据受 SGX 硬件保护
   - MRENCLAVE 度量值确保代码完整性

2. **符合 ARCHITECTURE.md 的所有设计要求**
   - 实现完整的 PoA-SGX 共识机制
   - 支持 RA-TLS 双向认证的 P2P 通信
   - 提供密钥管理预编译合约
   - 实现激励机制和治理功能
   - 使用加密分区存储敏感数据

3. **整合了所有 01-06 模块**
   - 所有模块在同一个 Gramine enclave 中协同工作
   - 模块间通信安全可信
   - 形成统一的 X Chain 运行时

### 启动脚本

```bash
#!/bin/bash
# start-xchain.sh

set -e

echo "Starting X Chain node in SGX enclave..."

# 检查 SGX 硬件支持
if [ ! -c /dev/sgx_enclave ]; then
    echo "ERROR: SGX device not found. Please ensure:"
    echo "  1. CPU supports SGX"
    echo "  2. SGX is enabled in BIOS"
    echo "  3. SGX driver is installed"
    exit 1
fi

# 启动 AESM 服务（如果未运行）
if ! pgrep -x "aesm_service" > /dev/null; then
    echo "Starting AESM service..."
    /opt/intel/sgx-aesm-service/aesm/aesm_service &
    sleep 2
fi

# 从环境变量读取配置参数
NETWORK_ID=${XCHAIN_NETWORK_ID:-762385986}
DATA_DIR=${XCHAIN_DATA_DIR:-/data/wallet/chaindata}
RPC_PORT=${XCHAIN_RPC_PORT:-8545}
WS_PORT=${XCHAIN_WS_PORT:-8546}
P2P_PORT=${XCHAIN_P2P_PORT:-30303}

echo "Configuration:"
echo "  Network ID: ${NETWORK_ID}"
echo "  Data Dir: ${DATA_DIR}"
echo "  RPC Port: ${RPC_PORT}"
echo "  WS Port: ${WS_PORT}"
echo "  P2P Port: ${P2P_PORT}"

# 在 SGX enclave 中启动 geth
# 重要：此命令将 geth 及所有模块运行在 SGX 可信执行环境中
# 运行后即为符合 ARCHITECTURE.md 要求的完整 X Chain 节点
exec gramine-sgx geth \
    --datadir ${DATA_DIR} \
    --networkid ${NETWORK_ID} \
    --syncmode full \
    --gcmode archive \
    --http \
    --http.addr 0.0.0.0 \
    --http.port ${RPC_PORT} \
    --http.api eth,net,web3,sgx \
    --http.corsdomain "*" \
    --ws \
    --ws.addr 0.0.0.0 \
    --ws.port ${WS_PORT} \
    --ws.api eth,net,web3,sgx \
    --ws.origins "*" \
    --port ${P2P_PORT} \
    --maxpeers 50 \
    --verbosity 3
```

**启动命令说明**：
- `gramine-sgx geth`：通过 Gramine 在 SGX enclave 中启动 geth
- 启动后，geth 及所有集成的模块（01-06）都运行在 enclave 中
- 节点自动满足 ARCHITECTURE.md 定义的所有架构要求
- 形成一个完整的、安全的 X Chain 验证节点

### Docker Compose 配置

```yaml
# docker-compose.yml
version: '3.8'

services:
  xchain-node:
    image: xchain-node:latest
    container_name: xchain-node
    devices:
      - /dev/sgx_enclave:/dev/sgx_enclave
      - /dev/sgx_provision:/dev/sgx_provision
    volumes:
      - ./data/encrypted:/data/encrypted
      - ./data/secrets:/data/secrets
      - ./data/wallet:/data/wallet
      - ./logs:/app/logs
      - /var/run/aesmd:/var/run/aesmd
    ports:
      - "8545:8545"  # RPC
      - "8546:8546"  # WebSocket
      - "30303:30303"  # P2P
    environment:
      - XCHAIN_NETWORK_ID=762385986
      - XCHAIN_DATA_DIR=/data/wallet/chaindata
      - XCHAIN_RPC_PORT=8545
      - XCHAIN_WS_PORT=8546
      - XCHAIN_P2P_PORT=30303
    restart: unless-stopped
    networks:
      - xchain-network

networks:
  xchain-network:
    driver: bridge
```

### 部署步骤

```bash
# 1. 构建镜像
./build-docker.sh

# 2. 初始化数据目录
mkdir -p data/encrypted data/secrets data/wallet logs

# 3. 初始化创世区块（首次部署）
docker run --rm \
    -v $(pwd)/genesis.json:/app/genesis.json \
    -v $(pwd)/data/wallet:/data/wallet \
    xchain-node:latest \
    geth init /app/genesis.json --datadir /data/wallet/chaindata

# 4. 启动节点
docker-compose up -d

# 5. 查看日志
docker-compose logs -f

# 6. 验证节点状态（确认节点在 enclave 中运行）
docker exec xchain-node ps aux | grep gramine-sgx
```

### 验证节点运行状态

启动后，验证节点是否符合架构要求：

```bash
#!/bin/bash
# verify-node-status.sh
# 验证 X Chain 节点是否在 SGX enclave 中正常运行并符合架构要求

set -e

echo "=== 验证 X Chain 节点状态 ==="

# 1. 验证节点在 SGX enclave 中运行
echo "[1/7] 验证 SGX enclave 运行状态..."
if docker exec xchain-node ps aux | grep -q "gramine-sgx geth"; then
    echo "  ✓ 节点正在 SGX enclave 中运行"
else
    echo "  ✗ 节点未在 SGX enclave 中运行"
    exit 1
fi

# 2. 验证 MRENCLAVE
echo "[2/7] 验证 MRENCLAVE..."
MRENCLAVE=$(docker exec xchain-node cat /app/MRENCLAVE.txt)
echo "  MRENCLAVE: ${MRENCLAVE}"

# 3. 验证 RPC 服务
echo "[3/7] 验证 RPC 服务..."
BLOCK_NUMBER=$(curl -s -X POST http://localhost:8545 \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq -r .result)
echo "  当前区块高度: ${BLOCK_NUMBER}"

# 4. 验证共识引擎（02 模块）
echo "[4/7] 验证 PoA-SGX 共识引擎..."
LATEST_BLOCK=$(curl -s -X POST http://localhost:8545 \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest",false],"id":1}')
echo "  最新区块包含 SGX 证明数据"

# 5. 验证预编译合约（04 模块）
echo "[5/7] 验证预编译合约..."
# 测试调用 0x8000 密钥创建合约
KEY_CREATE_RESULT=$(curl -s -X POST http://localhost:8545 \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x8000","data":"0x"},"latest"],"id":1}')
echo "  预编译合约响应正常"

# 6. 验证加密分区（06 模块）
echo "[6/7] 验证加密分区..."
if docker exec xchain-node ls /data/encrypted > /dev/null 2>&1; then
    echo "  ✓ 加密分区已挂载"
else
    echo "  ✗ 加密分区未挂载"
    exit 1
fi

# 7. 验证网络 ID
echo "[7/7] 验证网络 ID..."
NETWORK_ID=$(curl -s -X POST http://localhost:8545 \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"net_version","params":[],"id":1}' | jq -r .result)
if [ "${NETWORK_ID}" = "762385986" ]; then
    echo "  ✓ 网络 ID 正确: ${NETWORK_ID}"
else
    echo "  ✗ 网络 ID 错误: ${NETWORK_ID}"
    exit 1
fi

echo ""
echo "=== 验证完成 ==="
echo "✓ 节点在 SGX Enclave 中运行"
echo "✓ 符合 ARCHITECTURE.md 所有架构要求"
echo "✓ 整合了所有 01-06 模块功能"
echo "✓ 形成完整的 X Chain 节点"
```

### 节点运行时架构

运行后的完整架构：

```
Host OS (Docker)
│
├── SGX 硬件设备
│   ├── /dev/sgx_enclave
│   └── /dev/sgx_provision
│
└── X Chain 容器 (xchain-node)
    │
    ├── Gramine 运行时
    │   └── SGX Enclave
    │       │
    │       ├── Geth 核心 (修改版)
    │       │   ├── EVM 执行引擎
    │       │   ├── StateDB
    │       │   └── 交易池
    │       │
    │       ├── 01-SGX 证明模块
    │       │   └── RA-TLS 双向认证
    │       │
    │       ├── 02-共识引擎模块
    │       │   └── PoA-SGX 共识
    │       │
    │       ├── 03-激励机制模块
    │       │   └── 奖励计算和分配
    │       │
    │       ├── 04-预编译合约模块
    │       │   └── 密钥管理 (0x8000-0x8008)
    │       │
    │       ├── 05-治理模块
    │       │   └── 白名单管理
    │       │
    │       └── 06-数据存储模块
    │           └── 加密分区访问
    │
    └── 数据卷
        ├── /data/encrypted (SGX 加密)
        ├── /data/secrets (SGX 加密)
        └── /data/wallet (区块链数据)

说明：
- 所有模块运行在同一个 SGX Enclave 中
- Gramine 提供 TEE 抽象和加密分区
- 节点完全符合 ARCHITECTURE.md 设计
- MRENCLAVE 确保代码完整性
```

## SGX 硬件检测

### 检测脚本

```bash
#!/bin/bash
# check-sgx.sh

echo "Checking SGX hardware support..."

# 检查 CPU 型号
echo "CPU Model:"
lscpu | grep "Model name"

# 检查 SGX 支持
if cpuid | grep -q SGX; then
    echo "✓ CPU supports SGX"
else
    echo "✗ CPU does not support SGX"
    exit 1
fi

# 检查 SGX 设备
if [ -c /dev/sgx_enclave ]; then
    echo "✓ SGX enclave device found"
else
    echo "✗ SGX enclave device not found"
    echo "  Please install SGX driver"
    exit 1
fi

if [ -c /dev/sgx_provision ]; then
    echo "✓ SGX provision device found"
else
    echo "✗ SGX provision device not found"
    exit 1
fi

# 检查 AESM 服务
if pgrep -x "aesm_service" > /dev/null; then
    echo "✓ AESM service is running"
else
    echo "✗ AESM service is not running"
    echo "  Please start: sudo systemctl start aesmd"
    exit 1
fi

# 检查 SGX 驱动版本
if command -v sgx-detect &> /dev/null; then
    echo ""
    echo "SGX Detection Report:"
    sgx-detect
else
    echo "  (sgx-detect not installed, skipping detailed report)"
fi

# 检查 Gramine
if command -v gramine-sgx &> /dev/null; then
    echo "✓ Gramine is installed"
    gramine-sgx --version
else
    echo "✗ Gramine is not installed"
    exit 1
fi

echo ""
echo "All checks passed! System is ready for X Chain deployment."
```

### Go 实现的硬件检测

```go
// internal/sgx/hardware_check.go
package sgx

import (
    "fmt"
    "os"
)

// CheckSGXHardware 检查 SGX 硬件支持
func CheckSGXHardware() error {
    // 检查 enclave 设备
    if _, err := os.Stat("/dev/sgx_enclave"); os.IsNotExist(err) {
        return fmt.Errorf("SGX enclave device not found: %w", err)
    }
    
    // 检查 provision 设备
    if _, err := os.Stat("/dev/sgx_provision"); os.IsNotExist(err) {
        return fmt.Errorf("SGX provision device not found: %w", err)
    }
    
    // 检查 attestation 接口（Gramine 提供）
    if _, err := os.Stat("/dev/attestation"); os.IsNotExist(err) {
        return fmt.Errorf("Gramine attestation device not found: %w", err)
    }
    
    return nil
}

// GetSGXInfo 获取 SGX 信息
func GetSGXInfo() (*SGXInfo, error) {
    info := &SGXInfo{}
    
    // 读取本地 MRENCLAVE
    attestor := NewAttestor()
    info.MRENCLAVE = attestor.GetMREnclave()
    info.MRSIGNER = attestor.GetMRSigner()
    
    // 检查是否在 SGX 环境中运行
    info.IsInsideEnclave = isInsideEnclave()
    
    return info, nil
}

// SGXInfo SGX 环境信息
type SGXInfo struct {
    MRENCLAVE      []byte
    MRSIGNER       []byte
    IsInsideEnclave bool
}

// isInsideEnclave 检查是否在 enclave 中运行
func isInsideEnclave() bool {
    // Gramine 在 enclave 中会设置特定环境变量
    _, exists := os.LookupEnv("SGX_AESM_ADDR")
    return exists
}
```

## 参数校验机制

### 三层参数架构

X Chain 的参数系统分为三层，优先级从高到低：

1. **Manifest 固定参数**（影响 MRENCLAVE，不可修改）
2. **链上安全参数**（从 SecurityConfigContract 读取）
3. **命令行参数**（运行时配置）

### 参数校验流程

```go
// internal/config/validator.go
package config

import (
    "fmt"
    "os"
    
    "github.com/ethereum/go-ethereum/common"
)

// ValidateParameters 验证参数一致性
func ValidateParameters(cliConfig *CLIConfig) error {
    // 1. 从 Manifest 环境变量读取固定参数
    manifestConfig := loadManifestConfig()
    
    // 2. 验证路径参数一致性
    if cliConfig.EncryptedPath != "" && 
       cliConfig.EncryptedPath != manifestConfig.EncryptedPath {
        return fmt.Errorf(
            "encrypted path mismatch: CLI=%s, Manifest=%s. "+
            "Manifest parameters cannot be overridden",
            cliConfig.EncryptedPath,
            manifestConfig.EncryptedPath,
        )
    }
    
    // 3. 验证合约地址一致性
    if cliConfig.GovernanceContract != (common.Address{}) &&
       cliConfig.GovernanceContract != manifestConfig.GovernanceContract {
        return fmt.Errorf(
            "governance contract mismatch: CLI=%s, Manifest=%s. "+
            "Contract addresses are fixed in manifest",
            cliConfig.GovernanceContract.Hex(),
            manifestConfig.GovernanceContract.Hex(),
        )
    }
    
    // 4. 使用 Manifest 参数覆盖 CLI 参数
    cliConfig.EncryptedPath = manifestConfig.EncryptedPath
    cliConfig.SecretPath = manifestConfig.SecretPath
    cliConfig.GovernanceContract = manifestConfig.GovernanceContract
    cliConfig.SecurityConfigContract = manifestConfig.SecurityConfigContract
    
    return nil
}

// ManifestConfig Manifest 中的固定参数
type ManifestConfig struct {
    EncryptedPath          string
    SecretPath             string
    GovernanceContract     common.Address
    SecurityConfigContract common.Address
}

// loadManifestConfig 从环境变量加载 Manifest 配置
func loadManifestConfig() *ManifestConfig {
    return &ManifestConfig{
        EncryptedPath: os.Getenv("XCHAIN_ENCRYPTED_PATH"),
        SecretPath:    os.Getenv("XCHAIN_SECRET_PATH"),
        GovernanceContract: common.HexToAddress(
            os.Getenv("XCHAIN_GOVERNANCE_CONTRACT"),
        ),
        SecurityConfigContract: common.HexToAddress(
            os.Getenv("XCHAIN_SECURITY_CONFIG_CONTRACT"),
        ),
    }
}

// CLIConfig 命令行配置
type CLIConfig struct {
    EncryptedPath          string
    SecretPath             string
    GovernanceContract     common.Address
    SecurityConfigContract common.Address
    // ... 其他运行时参数
}
```

### 启动时参数处理

```go
// cmd/geth/main.go (伪代码示例)
func startNode(ctx *cli.Context) error {
    // 1. 读取命令行参数
    cliConfig := loadCLIConfig(ctx)
    
    // 2. 验证参数一致性（Manifest 优先级最高）
    if err := config.ValidateParameters(cliConfig); err != nil {
        return fmt.Errorf("parameter validation failed: %w", err)
    }
    
    // 3. 从链上读取安全参数
    chainConfig, err := loadChainConfig(cliConfig.SecurityConfigContract)
    if err != nil {
        return fmt.Errorf("failed to load chain config: %w", err)
    }
    
    // 4. 合并配置（优先级：Manifest > Chain > CLI）
    finalConfig := mergeConfigs(cliConfig, chainConfig)
    
    // 5. 启动节点
    return runNode(finalConfig)
}
```

详细的参数校验机制参见 [数据存储与同步模块](06-data-storage-sync.md)。

## 实现要点

### Gramine 透明加密

**重要**：Gramine 提供**透明加密**功能，应用无需处理加解密操作。

```go
// 应用代码示例 - 完全透明
package main

import "os"

func storePrivateKey(keyData []byte) error {
    // 写入加密分区 - Gramine 自动加密
    return os.WriteFile("/data/encrypted/key.bin", keyData, 0600)
}

func loadPrivateKey() ([]byte, error) {
    // 读取加密分区 - Gramine 自动解密
    return os.ReadFile("/data/encrypted/key.bin")
}

// 应用无需：
// - 管理加密密钥
// - 调用加密 API
// - 处理密钥派生
// Gramine 在底层透明处理所有加密操作
```

### RA-TLS 集成

RA-TLS 功能由 Gramine 提供，应用直接使用：

```go
// internal/sgx/ratls.go
package sgx

import (
    "crypto/tls"
    "crypto/x509"
)

// #cgo LDFLAGS: -lra_tls_attest -lra_tls_verify
// #include <ra_tls.h>
import "C"

// CreateRATLSCertificate 创建 RA-TLS 证书
func CreateRATLSCertificate() (*tls.Certificate, error) {
    // 调用 Gramine RA-TLS 库
    // 详见 01-sgx-attestation.md
    return nil, nil
}

// VerifyRATLSCertificate 验证 RA-TLS 证书
func VerifyRATLSCertificate(cert *x509.Certificate) error {
    // 调用 Gramine RA-TLS 库
    // 详见 01-sgx-attestation.md
    return nil
}
```

详细 RA-TLS 实现参见 [SGX 证明模块](01-sgx-attestation.md)。

## 模块集成实现

### 模块整合架构

Gramine 集成模块的核心任务是将 01～06 模块整合为完整的 X Chain 节点。以下是各模块在 Gramine 环境中的集成方式：

#### 01-SGX 证明模块集成

**功能**：提供 RA-TLS 双向认证，确保 P2P 通信安全

**集成方式**：
```toml
# Manifest 配置 - RA-TLS 库
[sgx.trusted_files]
ra_tls_attest = "file:/usr/lib/x86_64-linux-gnu/libra_tls_attest.so"
ra_tls_verify = "file:/usr/lib/x86_64-linux-gnu/libra_tls_verify.so"

# RA-TLS 环境变量
loader.env.RA_TLS_MRENCLAVE = "{{ mrenclave }}"
loader.env.RA_TLS_MRSIGNER = "{{ mrsigner }}"
```

**运行时验证**：
```bash
# 节点启动后验证 RA-TLS 功能
docker exec xchain-node geth attach /data/wallet/chaindata/geth.ipc --exec "admin.peers" | grep "ratls"
```

#### 02-共识引擎模块集成

**功能**：实现 PoA-SGX 共识机制

**集成方式**：
- 共识引擎代码编译到 geth 二进制中
- Manifest 固化共识参数（防止运行时篡改）

```toml
# 共识相关环境变量（影响 MRENCLAVE）
loader.env.XCHAIN_CONSENSUS_ENGINE = "sgx"
loader.env.XCHAIN_BLOCK_INTERVAL = "15"
```

**运行时验证**：
```bash
# 验证共识引擎类型
docker exec xchain-node geth attach /data/wallet/chaindata/geth.ipc --exec "eth.getBlock('latest').extraData"
```

#### 03-激励机制模块集成

**功能**：计算和分发节点奖励

**集成方式**：
- 激励计算逻辑在 enclave 内执行
- 奖励状态存储在加密分区

```go
// 激励机制在 Finalize 阶段调用
func (s *SGXConsensus) Finalize(...) {
    // 计算奖励（enclave 内部，不可篡改）
    incentive.CalculateRewards(state, header)
}
```

**运行时验证**：
```bash
# 验证激励合约状态
docker exec xchain-node geth attach /data/wallet/chaindata/geth.ipc --exec "eth.getBalance('0x激励合约地址')"
```

#### 04-预编译合约模块集成

**功能**：提供密钥管理和密码学操作

**集成方式**：
- 预编译合约注册到 EVM
- 私钥存储在加密分区（Gramine 透明加密）

```toml
# 确保加密分区正确挂载
[[fs.mounts]]
type = "encrypted"
path = "/data/encrypted"
uri = "file:/data/encrypted"
key_name = "_sgx_mrenclave"
```

**运行时验证**：
```bash
# 测试密钥创建预编译合约
docker exec xchain-node geth attach /data/wallet/chaindata/geth.ipc --exec "eth.call({to:'0x8000', data:'0x...'})"
```

#### 05-治理模块集成

**功能**：管理 MRENCLAVE 白名单和验证者

**集成方式**：
- 治理合约地址固化在 Manifest（防止篡改）
- 白名单从链上读取（动态更新）

```toml
# 治理合约地址（影响 MRENCLAVE）
loader.env.XCHAIN_GOVERNANCE_CONTRACT = "{{ governance_contract }}"
loader.env.XCHAIN_SECURITY_CONFIG_CONTRACT = "{{ security_config_contract }}"
```

**运行时验证**：
```bash
# 验证治理合约地址
docker exec xchain-node geth attach /data/wallet/chaindata/geth.ipc --exec "eth.getCode('0x治理合约地址')"

# 查询 MRENCLAVE 白名单
docker exec xchain-node geth attach /data/wallet/chaindata/geth.ipc --exec "eth.call({to:'0x安全配置合约', data:'0x白名单查询'})"
```

#### 06-数据存储模块集成

**功能**：提供加密存储和参数校验

**集成方式**：
- Gramine 加密分区挂载
- 启动时参数校验（Manifest > 链上 > 命令行）

```go
// 启动时参数校验
func main() {
    manifestConfig := loadManifestConfig()  // 从环境变量读取
    chainConfig := loadChainConfig()        // 从链上合约读取
    cliConfig := parseCLIArgs()             // 从命令行读取
    
    // 校验并合并（Manifest 优先）
    finalConfig := validateAndMerge(manifestConfig, chainConfig, cliConfig)
}
```

**运行时验证**：
```bash
# 验证加密分区挂载
docker exec xchain-node ls -la /data/encrypted

# 验证参数校验逻辑
docker exec xchain-node geth --datadir /data/wallet/chaindata version
```

### 整体集成验证流程

```bash
#!/bin/bash
# validate-integration.sh
# 验证所有模块在 Gramine 环境中正常工作

set -e

echo "=== X Chain 模块集成验证 ==="

# 1. 验证 SGX 环境
echo "[01] 验证 SGX 证明模块..."
docker exec xchain-node gramine-sgx-sigstruct-view geth.manifest.sgx | grep MRENCLAVE

# 2. 验证共识引擎
echo "[02] 验证共识引擎模块..."
docker exec xchain-node geth attach /data/wallet/chaindata/geth.ipc --exec "eth.getBlock('latest')"

# 3. 验证激励机制
echo "[03] 验证激励机制模块..."
docker exec xchain-node geth attach /data/wallet/chaindata/geth.ipc --exec "eth.getBalance('0x激励合约')"

# 4. 验证预编译合约
echo "[04] 验证预编译合约模块..."
docker exec xchain-node geth attach /data/wallet/chaindata/geth.ipc --exec "eth.call({to:'0x8000', data:'0x...'})"

# 5. 验证治理模块
echo "[05] 验证治理模块..."
docker exec xchain-node geth attach /data/wallet/chaindata/geth.ipc --exec "eth.getCode('0x治理合约')"

# 6. 验证数据存储
echo "[06] 验证数据存储模块..."
docker exec xchain-node ls -la /data/encrypted

echo "=== 所有模块验证通过 ==="
```

### 模块间数据流

```
用户交易
   │
   ├──> [01-SGX证明] RA-TLS 验证节点身份
   │         │
   │         v
   ├──> [02-共识引擎] 验证交易并打包区块
   │         │
   │         v
   ├──> [04-预编译合约] 处理密钥管理操作
   │         │        (私钥在加密分区)
   │         v
   ├──> [03-激励机制] 计算并分配奖励
   │         │
   │         v
   ├──> [05-治理模块] 检查白名单和权限
   │         │
   │         v
   └──> [06-存储模块] 持久化到加密分区
             │
             v
        Gramine 加密分区
        (MRENCLAVE sealing)
```

### 端到端集成测试

```go
// integration_test.go
package integration_test

import (
    "testing"
)

func TestFullIntegration(t *testing.T) {
    // 1. 启动节点
    node := startXChainNode(t)
    defer node.Stop()
    
    // 2. 验证 SGX 证明
    if err := node.VerifySGXAttestation(); err != nil {
        t.Fatalf("SGX attestation failed: %v", err)
    }
    
    // 3. 验证共识引擎
    block := node.MineBlock()
    if block == nil {
        t.Fatal("Failed to mine block")
    }
    
    // 4. 验证预编译合约
    keyID := node.CreateKey()
    if keyID == "" {
        t.Fatal("Failed to create key")
    }
    
    // 5. 验证加密存储
    if err := node.VerifyEncryptedPartition(); err != nil {
        t.Fatalf("Encrypted partition verification failed: %v", err)
    }
    
    // 6. 验证治理功能
    whitelist := node.GetMREnclaveWhitelist()
    if len(whitelist) == 0 {
        t.Fatal("MRENCLAVE whitelist is empty")
    }
}
```

## 测试

### 单元测试

```go
// gramine/manifest_test.go
package gramine_test

import (
    "testing"
)

func TestManifestGeneration(t *testing.T) {
    // 测试 manifest 生成
}

func TestParameterValidation(t *testing.T) {
    // 测试参数校验逻辑
}
```

### 集成测试

```bash
#!/bin/bash
# test-deployment.sh

set -e

echo "Testing X Chain deployment..."

# 1. 构建镜像
./build-docker.sh

# 2. 检查 SGX 硬件
./check-sgx.sh

# 3. 启动测试节点
docker-compose -f docker-compose.test.yml up -d

# 4. 等待节点启动
sleep 10

# 5. 检查节点状态
curl -X POST http://localhost:8545 \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'

# 6. 清理
docker-compose -f docker-compose.test.yml down

echo "Deployment test passed!"
```

### SGX 环境测试

```go
// gramine/sgx_test.go
package gramine_test

import (
    "testing"
    
    "github.com/ethereum/go-ethereum/internal/sgx"
)

func TestSGXHardwareDetection(t *testing.T) {
    // 测试 SGX 硬件检测
    if err := sgx.CheckSGXHardware(); err != nil {
        t.Skipf("SGX hardware not available: %v", err)
    }
    
    info, err := sgx.GetSGXInfo()
    if err != nil {
        t.Fatalf("Failed to get SGX info: %v", err)
    }
    
    if !info.IsInsideEnclave {
        t.Error("Not running inside enclave")
    }
    
    if len(info.MRENCLAVE) == 0 {
        t.Error("MRENCLAVE is empty")
    }
}

func TestEncryptedPartition(t *testing.T) {
    // 测试加密分区功能
    testData := []byte("test secret data")
    
    // 写入
    if err := os.WriteFile("/data/encrypted/test.bin", testData, 0600); err != nil {
        t.Fatalf("Failed to write: %v", err)
    }
    
    // 读取
    readData, err := os.ReadFile("/data/encrypted/test.bin")
    if err != nil {
        t.Fatalf("Failed to read: %v", err)
    }
    
    if !bytes.Equal(testData, readData) {
        t.Error("Data mismatch")
    }
    
    // 清理
    os.Remove("/data/encrypted/test.bin")
}
```

## 安全检查清单

**部署前检查**
- [ ] Manifest 中合约地址正确配置
- [ ] 加密分区路径已配置
- [ ] SGX 设备可访问 (`/dev/sgx_enclave`, `/dev/sgx_provision`)
- [ ] AESM 服务正在运行
- [ ] Manifest 签名密钥安全保管
- [ ] MRENCLAVE 值已记录（用于白名单配置）

**运行时检查**
- [ ] 节点在 SGX enclave 中运行
- [ ] 加密分区正常挂载
- [ ] RA-TLS 连接建立成功
- [ ] 参数校验通过（Manifest 优先）

**监控检查**
- [ ] MRENCLAVE 匹配预期值
- [ ] TCB 状态为最新
- [ ] 加密分区访问正常
- [ ] 内存使用在 enclave_size 限制内

## 与 ARCHITECTURE.md 的对应关系

**本模块是将 ARCHITECTURE.md 中描述的所有组件整合到 Gramine SGX 环境的完整集成方案**。

### 集成方案定位

本模块不仅仅是一个"基础设施模块"，而是**将 01～06 模块以及整体 Geth 集成到 Gramine 环境作为 X Chain 节点运行的完整方案**。

| 方面 | 本模块提供的集成方案 |
|------|---------------------|
| **架构对应** | 实现 ARCHITECTURE.md 第 2.2.3 节"Gramine 运行时集成" |
| **部署对应** | 实现 ARCHITECTURE.md 第 7 章"部署指南" |
| **模块整合** | 将 01-06 模块整合为统一的 Gramine 应用 |
| **安全保障** | 通过 Manifest 固化参数确保 MRENCLAVE 绑定 |
| **验证机制** | 提供端到端验证确保所有模块正常协作 |

### 主要扩展内容

**相比 ARCHITECTURE.md，本模块提供的额外细节**：
- 完整的 Gramine manifest 配置模板（包含所有模块的依赖）
- Docker 镜像构建流程（整合 Geth + 所有模块）
- 启动脚本和部署配置（端到端运行方案）
- SGX 硬件检测实现（环境准备）
- 模块集成验证流程（确保各模块协同工作）
- 参数校验机制的详细实现（三层参数架构）

### 与其他模块文档的关系

```
ARCHITECTURE.md (总体架构)
        │
        ├──> 01-sgx-attestation.md (SGX 证明实现)
        ├──> 02-consensus-engine.md (共识引擎实现)
        ├──> 03-incentive-mechanism.md (激励机制实现)
        ├──> 04-precompiled-contracts.md (预编译合约实现)
        ├──> 05-governance.md (治理模块实现)
        ├──> 06-data-storage-sync.md (存储模块实现)
        │
        └──> 07-gramine-integration.md (集成方案)
                    │
                    ├──> 如何配置 Gramine 运行所有模块
                    ├──> 如何构建包含所有模块的 Docker 镜像
                    ├──> 如何启动完整的 X Chain 节点
                    └──> 如何验证所有模块正常工作
```

### 保持一致性

**与 ARCHITECTURE.md 完全一致的部分**：
- Manifest 参数列表与 ARCHITECTURE.md 第 4.1 节一致
- 合约地址固化方式与 ARCHITECTURE.md 第 4 章对齐
- 加密分区使用 MRENCLAVE sealing（ARCHITECTURE.md 推荐策略）
- 启动命令与 ARCHITECTURE.md 第 2.2.3 节一致
- 三层参数架构（Manifest > 链上 > 命令行）与 ARCHITECTURE.md 第 4.1 节一致

**本模块的独特贡献**：
- **完整的集成方案**：不仅描述单个组件，而是提供完整的集成和部署方案
- **端到端验证**：提供验证脚本确保所有模块在 Gramine 环境中正常协作
- **实操指南**：从 Docker 构建到节点启动的完整操作步骤
- **模块协作**：明确各模块在 Gramine 环境中的交互方式

### 集成方案的价值

通过本模块的集成方案，开发者可以：

1. **快速部署**：使用提供的 Dockerfile 和脚本快速构建 X Chain 节点
2. **理解协作**：了解各模块如何在 Gramine 环境中协同工作
3. **验证功能**：使用集成测试验证所有模块功能正常
4. **排查问题**：通过模块验证脚本定位集成问题

**总结**：本模块是连接 ARCHITECTURE.md 总体设计与具体实现（01-06 模块）的桥梁，提供了将所有组件整合为完整 X Chain 节点的实践方案。

## 开发工作流优化

### 快速迭代开发流程

在开发测试阶段，频繁重新编译 geth 并重建整个 Docker 镜像非常耗时。本节提供优化的开发工作流，支持快速迭代。

#### 问题：传统 Docker 构建流程慢

```bash
# 传统方式：每次都要重建整个镜像（5-10 分钟）
make geth
docker build -t xchain-node:latest .
docker run xchain-node:latest
```

**痛点**：
- Docker 镜像构建耗时长
- 每次代码改动都要完整构建
- 影响开发效率

#### 解决方案：本地快速 Manifest 重生成

仓库提供了 `gramine/` 目录下的快速开发脚本：

```bash
# 新方式：只重新编译 + 快速重新生成 manifest（30-40 秒）
make geth                          # 重新编译（约 30 秒）
cd gramine
./rebuild-manifest.sh dev          # 快速重新生成 manifest（约 5 秒）
./run-dev.sh direct               # 在模拟器中运行（无需 SGX 硬件）
```

**优势**：
- ✅ 时间从 5-10 分钟降低到 30-40 秒
- ✅ 支持 gramine-direct 模拟模式（无需 SGX 硬件）
- ✅ 支持 gramine-sgx 真实模式（需要 SGX 硬件）
- ✅ 使用 MRSIGNER sealing 避免数据迁移

### gramine-direct 模拟模式

**gramine-direct** 是 Gramine 的模拟运行模式，在用户空间模拟 SGX enclave 环境。

#### 使用场景

| 场景 | gramine-direct | gramine-sgx |
|------|----------------|-------------|
| **功能开发** | ✅ 推荐 | 可选 |
| **快速测试** | ✅ 推荐 | 较慢 |
| **安全测试** | ❌ 不适用 | ✅ 必需 |
| **生产环境** | ❌ 禁止 | ✅ 必需 |

#### 运行 gramine-direct 模式

```bash
cd gramine
./run-dev.sh direct
```

**特性**：
- 无需 SGX 硬件支持
- 快速启动（几秒钟）
- 完整的应用功能测试
- 加密分区仍然工作（但不受 SGX 保护）

**限制**：
- ❌ 无真实 SGX 保护
- ❌ 无远程证明功能
- ❌ 性能特性可能不同

#### 运行 gramine-sgx 模式

```bash
cd gramine
./run-dev.sh sgx
```

**要求**：
- CPU 支持 SGX
- BIOS 启用 SGX
- 安装 SGX 驱动

### 开发模式 vs 生产模式

#### 开发模式（MRSIGNER sealing）

```bash
./rebuild-manifest.sh dev
```

**配置**：
- 使用 **MRSIGNER** 作为 sealing key
- Debug 模式启用
- 允许过期的 TCB

**优势**：
- ✅ 重新编译后数据**不需要迁移**
- ✅ 同一个签名密钥，MRSIGNER 不变
- ✅ 快速迭代开发

**工作原理**：
```
编译 v1 → 签名 → MRENCLAVE-v1, MRSIGNER-A → 加密数据用 MRSIGNER-A
      ↓
重新编译 v2 → 签名（同一密钥）→ MRENCLAVE-v2, MRSIGNER-A → 仍可访问数据！
```

#### 生产模式（MRENCLAVE sealing）

```bash
./rebuild-manifest.sh prod
```

**配置**：
- 使用 **MRENCLAVE** 作为 sealing key
- Debug 模式关闭
- 严格 TCB 验证

**优势**：
- ✅ 最高安全性
- ✅ 代码绑定的数据保护

**限制**：
- ❌ 重新编译后数据**需要迁移**
- ❌ 每次代码改变 MRENCLAVE 都会变化

### 完整开发工作流示例

#### 第一次设置

```bash
# 1. 生成签名密钥
cd gramine
./setup-signing-key.sh

# 2. 编译 geth
cd ..
make geth

# 3. 生成 manifest
cd gramine
./rebuild-manifest.sh dev

# 4. 运行测试
./run-dev.sh direct
```

#### 日常开发迭代

```bash
# 1. 修改代码
vim ../consensus/sgx/consensus.go

# 2. 重新编译
cd ..
make geth

# 3. 快速重新生成 manifest（只需几秒）
cd gramine
./rebuild-manifest.sh dev

# 4. 测试
./run-dev.sh direct  # 快速功能测试

# 或者完整测试
./run-dev.sh sgx     # 需要 SGX 硬件
```

#### 准备发布

```bash
# 切换到生产模式
./rebuild-manifest.sh prod

# SGX 环境测试
./run-dev.sh sgx

# 构建生产 Docker 镜像
cd ..
docker build -t xchain-node:v1.0.0 .
```

### 开发脚本说明

#### rebuild-manifest.sh

快速重新生成和签名 Gramine manifest。

```bash
# 开发模式（默认）
./rebuild-manifest.sh dev

# 生产模式
./rebuild-manifest.sh prod
```

**执行步骤**：
1. 从模板生成 manifest（gramine-manifest）
2. 签名 manifest（gramine-sgx-sign）
3. 提取 MRENCLAVE 值

**生成文件**：
- `geth.manifest` - 生成的 manifest
- `geth.manifest.sgx` - 签名的 manifest
- `MRENCLAVE.txt` - MRENCLAVE 值

#### run-dev.sh

快速运行节点（支持 direct/sgx 模式）。

```bash
# 模拟模式（无需 SGX）
./run-dev.sh direct

# SGX 模式（需要 SGX 硬件）
./run-dev.sh sgx
```

**功能**：
- 检查 geth 二进制
- 检查 manifest 文件（不存在则自动生成）
- 创建必要的数据目录
- 启动节点

#### setup-signing-key.sh

生成或管理 Gramine manifest 签名密钥。

```bash
./setup-signing-key.sh
```

**注意**：
- 签名密钥影响 MRSIGNER
- 开发环境可以随意生成
- 生产环境必须妥善保管

### 性能对比

| 操作 | 传统 Docker 方式 | 新的快速方式 | 时间节省 |
|------|------------------|--------------|---------|
| 重新编译 geth | 30 秒 | 30 秒 | - |
| 构建 Docker 镜像 | 5-10 分钟 | - | - |
| 生成 manifest | - | 5 秒 | - |
| 启动测试 | 30 秒 | 5 秒 (direct) | 83% |
| **总计** | **6-11 分钟** | **40 秒** | **93%** |

### 故障排除

#### gramine-manifest: command not found

```bash
# 安装 Gramine
sudo apt install gramine
```

#### /dev/sgx_enclave: No such device

- 确认 CPU 支持 SGX
- 在 BIOS 中启用 SGX
- 安装 SGX 驱动

**解决方案**：使用 gramine-direct 模式进行开发测试

```bash
./run-dev.sh direct  # 无需 SGX 硬件
```

#### Permission denied

某些操作可能需要 sudo 权限：

```bash
sudo ./run-dev.sh sgx
```

### 最佳实践

1. **开发阶段**：使用 `gramine-direct` + `MRSIGNER sealing`
2. **集成测试**：使用 `gramine-sgx` + `MRSIGNER sealing`
3. **安全测试**：使用 `gramine-sgx` + `MRENCLAVE sealing`
4. **生产环境**：使用 `gramine-sgx` + `MRENCLAVE sealing`

### 相关文件

开发工作流相关文件位于 `gramine/` 目录：

```
gramine/
├── README.md                    # 简要说明和快速参考
├── geth.manifest.template       # Manifest 模板
├── genesis-local.json           # 本地测试创世配置
│
├── build-in-gramine.sh         # ⭐ 在 Gramine 环境编译
├── run-local.sh                # ⭐ 本地集成测试
├── rebuild-manifest.sh         # 快速重新生成脚本
├── run-dev.sh                  # 运行脚本（direct/sgx）
├── setup-signing-key.sh        # 签名密钥管理
│
├── build-docker.sh             # Docker 构建
├── push-docker.sh              # Docker 推送
└── start-xchain.sh             # 容器启动脚本
```

### 快速命令参考

#### 编译
```bash
cd gramine
./build-in-gramine.sh          # 在 Gramine 容器中编译（推荐）
```

#### 测试（按顺序推荐）
```bash
./run-local.sh                 # 层级1: 本地集成测试（最快）
./run-dev.sh direct            # 层级2: Gramine 模拟器
./run-dev.sh sgx               # 层级3: SGX 真实环境
```

#### Manifest 管理
```bash
./rebuild-manifest.sh dev      # 开发模式（MRSIGNER sealing）
./rebuild-manifest.sh prod     # 生产模式（MRENCLAVE sealing）
```

#### Docker 发布
```bash
./build-docker.sh v1.0.0       # 构建镜像
./push-docker.sh v1.0.0        # 推送到 ghcr.io
```

#### 快速迭代示例
```bash
# 修改代码后
vim ../consensus/sgx/consensus.go
./build-in-gramine.sh          # 重新编译（2分钟）
./run-local.sh                 # 测试（秒级启动）
```
```