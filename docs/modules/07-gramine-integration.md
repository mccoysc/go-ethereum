# Gramine 集成模块开发文档

## 模块概述

Gramine 集成模块负责 X Chain 节点在 Intel SGX 环境中的部署和运行。该模块管理 Gramine manifest 配置、安全参数嵌入、加密文件系统挂载、以及 Docker 镜像构建流程。

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

1. Gramine manifest 模板配置
2. 安全参数嵌入（影响度量值）
3. 加密文件系统配置
4. Docker 镜像构建流程
5. 启动脚本和部署配置
6. SGX 硬件支持检测

## 依赖关系

```
+----------------------+
|   Gramine 集成模块   |
+----------------------+
        |
        +---> SGX 证明模块（RA-TLS 配置）
        |
        +---> 数据存储模块（加密分区）
        |
        +---> 治理模块（度量值白名单）
```

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
[fs.mounts]...
```