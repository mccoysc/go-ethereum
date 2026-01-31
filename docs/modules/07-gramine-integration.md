# Gramine 集成模块开发文档

## 模块概述

Gramine 集成模块负责 X Chain 节点在 Intel SGX 环境中的部署和运行。该模块管理 Gramine manifest 配置、安全参数嵌入、加密文件系统挂载、以及 Docker 镜像构建流程。

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
[fs.mounts]
# 根文件系统
[[fs.mounts]]
type = "chroot"
path = "/"
uri = "file:/"

# 临时文件系统
[[fs.mounts]]
type = "tmpfs"
path = "/tmp"

# 加密分区 - 用于存储私钥
[[fs.mounts]]
type = "encrypted"
path = "/data/encrypted"
uri = "file:/data/encrypted"
key_name = "_sgx_mrenclave"

# 加密分区 - 用于存储秘密数据
[[fs.mounts]]
type = "encrypted"
path = "/data/secrets"
uri = "file:/data/secrets"
key_name = "_sgx_mrenclave"

# 普通目录 - 用于存储证书（非私钥）
[[fs.mounts]]
type = "chroot"
path = "/data/certs"
uri = "file:/data/certs"

# 区块链数据目录
[[fs.mounts]]
type = "chroot"
path = "/data/chaindata"
uri = "file:/data/chaindata"

# 允许的文件
[sgx.allowed_files]
# 配置文件
config = "file:/app/config.toml"
genesis = "file:/app/genesis.json"

# 日志文件
log = "file:/data/logs/geth.log"

# 网络相关
hosts = "file:/etc/hosts"
resolv = "file:/etc/resolv.conf"
nsswitch = "file:/etc/nsswitch.conf"
```

### 安全参数说明

Manifest 中只存储本地配置和链上合约地址。合约地址写死在 manifest 中，作为安全锚点：

| 参数 | 说明 | 格式 |
|------|------|------|
| `XCHAIN_ENCRYPTED_PATH` | 加密分区路径 | 绝对路径 |
| `XCHAIN_SECRET_PATH` | 秘密数据路径 | 绝对路径 |
| `XCHAIN_GOVERNANCE_CONTRACT` | 治理合约地址（写死） | 以太坊地址 |
| `XCHAIN_SECURITY_CONFIG_CONTRACT` | 安全配置合约地址（写死，由治理合约管理） | 以太坊地址 |

**重要说明**：
- 白名单（MRENCLAVE/MRSIGNER）不应存储在 Manifest 环境变量中
- 所有治理相关的安全参数（白名单、密钥迁移阈值、准入策略等）从链上合约动态读取
- 合约地址影响 MRENCLAVE，攻击者无法修改合约地址而不改变度量值

### 加密分区配置

加密分区使用 Gramine 的 `type = "encrypted"` 挂载类型：

```toml
# 使用 MRENCLAVE 派生的密钥加密
[[fs.mounts]]
type = "encrypted"
path = "/data/encrypted"
uri = "file:/data/encrypted"
key_name = "_sgx_mrenclave"
```

**重要说明**：
- `key_name = "_sgx_mrenclave"` 使用 MRENCLAVE 派生的密钥，确保只有相同代码的 enclave 能解密
- 私钥必须存储在加密分区中
- 证书可以存储在普通目录中（非敏感数据）

## Docker 镜像构建

### 构建流程

Manifest 文件必须在 Docker 镜像构建时预编译和签名，而不是在容器启动时：

```dockerfile
# Dockerfile
FROM gramineproject/gramine:v1.6-jammy AS builder

# 安装依赖
RUN apt-get update && apt-get install -y \
    build-essential \
    golang-go \
    && rm -rf /var/lib/apt/lists/*

# 复制源代码
WORKDIR /build
COPY . .

# 编译 geth
RUN go build -o geth ./cmd/geth

# 复制到应用目录
RUN mkdir -p /app && cp geth /app/

# 复制 manifest 模板
COPY deploy/gramine/geth.manifest.template /app/

# 生成签名密钥（仅用于构建，不包含在最终镜像中）
RUN gramine-sgx-gen-private-key /tmp/signing_key.pem

# 设置构建参数
# 注意：白名单不再作为构建参数，而是从链上合约动态读取
ARG GOVERNANCE_CONTRACT=""
ARG SECURITY_CONFIG_CONTRACT=""
ARG LOG_LEVEL="error"

# 编译 manifest（步骤 1）
# 只嵌入本地配置和链上合约地址
RUN gramine-manifest \
    -Dlog_level=${LOG_LEVEL} \
    -Dgovernance_contract="${GOVERNANCE_CONTRACT}" \
    -Dsecurity_config_contract="${SECURITY_CONFIG_CONTRACT}" \
    -Darch_libdir=/lib/x86_64-linux-gnu \
    /app/geth.manifest.template /app/geth.manifest

# 签名 manifest（步骤 2）
RUN gramine-sgx-sign \
    --manifest /app/geth.manifest \
    --output /app/geth.manifest.sgx \
    --key /tmp/signing_key.pem

# 生成 SIGSTRUCT
RUN gramine-sgx-sigstruct-view /app/geth.sig

# 最终镜像
FROM gramineproject/gramine:v1.6-jammy

# 复制预编译的应用和 manifest
COPY --from=builder /app/geth /app/
COPY --from=builder /app/geth.manifest /app/
COPY --from=builder /app/geth.manifest.sgx /app/
COPY --from=builder /app/geth.sig /app/

# 创建数据目录
RUN mkdir -p /data/encrypted /data/secrets /data/certs /data/chaindata /data/logs

# 复制启动脚本
COPY deploy/scripts/entrypoint.sh /app/

# 设置权限
RUN chmod +x /app/entrypoint.sh

WORKDIR /app
ENTRYPOINT ["/app/entrypoint.sh"]
```

### 启动脚本

```bash
#!/bin/bash
# entrypoint.sh
# X Chain 节点启动脚本

set -e

# 检查 SGX 设备
check_sgx_device() {
    if [ ! -e /dev/sgx_enclave ] && [ ! -e /dev/sgx/enclave ]; then
        echo "ERROR: SGX device not found"
        echo "Please ensure SGX is enabled and the container has access to SGX devices"
        exit 1
    fi
    echo "SGX device found"
}

# 检查 AESM 服务
check_aesm_service() {
    if [ ! -S /var/run/aesmd/aesm.socket ]; then
        echo "WARNING: AESM socket not found, starting local AESM..."
        # 如果需要本地 AESM，可以在这里启动
    else
        echo "AESM service available"
    fi
}

# 初始化数据目录
init_data_dirs() {
    echo "Initializing data directories..."
    
    # 确保目录存在
    mkdir -p /data/encrypted
    mkdir -p /data/secrets
    mkdir -p /data/certs
    mkdir -p /data/chaindata
    mkdir -p /data/logs
    
    # 设置权限
    chmod 700 /data/encrypted
    chmod 700 /data/secrets
    chmod 755 /data/certs
    chmod 755 /data/chaindata
    chmod 755 /data/logs
}

# 主函数
main() {
    echo "Starting X Chain node..."
    
    # 环境检查
    check_sgx_device
    check_aesm_service
    init_data_dirs
    
    # 显示 MRENCLAVE 信息
    echo "MRENCLAVE: $(gramine-sgx-sigstruct-view /app/geth.sig 2>/dev/null | grep mr_enclave | awk '{print $2}')"
    
    # 启动 geth（使用预编译的 manifest）
    # 注意：manifest 已在镜像构建时编译和签名，这里直接使用
    exec gramine-sgx /app/geth "$@"
}

main "$@"
```

### Docker Compose 配置

```yaml
# docker-compose.yml
version: '3.8'

services:
  xchain-node:
    image: xchain/geth:latest
    container_name: xchain-node
    
    # SGX 设备访问
    devices:
      - /dev/sgx_enclave:/dev/sgx_enclave
      - /dev/sgx_provision:/dev/sgx_provision
    
    # 挂载 AESM socket
    volumes:
      - /var/run/aesmd:/var/run/aesmd
      - xchain-data:/data
    
    # 网络配置
    ports:
      - "8545:8545"   # RPC
      - "8546:8546"   # WebSocket
      - "30303:30303" # P2P
    
    # 运行时参数（非安全参数）
    command:
      - --xchain.block.interval=15
      - --xchain.block.max-tx=1000
      - --xchain.rpc.port=8545
      - --xchain.p2p.port=30303
      - --xchain.log.level=info
    
    # 资源限制
    deploy:
      resources:
        limits:
          memory: 4G
        reservations:
          memory: 2G
    
    restart: unless-stopped

volumes:
  xchain-data:
```

## 参数分类与处理

### Manifest 固定参数

这些参数在镜像构建时嵌入 manifest，影响 MRENCLAVE 度量值：

```bash
# 构建时设置固定参数（只有本地配置和链上合约地址）
# 注意：白名单不再作为构建参数，而是从链上合约动态读取
docker build \
    --build-arg GOVERNANCE_CONTRACT="0x1234567890abcdef1234567890abcdef12345678" \
    --build-arg SECURITY_CONFIG_CONTRACT="0xabcdef1234567890abcdef1234567890abcdef12" \
    -t xchain/geth:latest .
```

**重要说明**：
- 合约地址写死在 manifest 中，作为安全锚点
- 所有治理相关的安全参数（白名单、密钥迁移阈值、准入策略等）从链上合约动态读取
- 这样投票结果可以实时生效，无需重新构建镜像

### 运行时参数（命令行控制）

这些参数在容器启动时通过命令行传递：

```bash
# 运行时设置非安全参数
docker run xchain/geth:latest \
    --xchain.block.interval=15 \
    --xchain.block.max-tx=1000 \
    --xchain.rpc.port=8545
```

### 参数校验流程

```
+------------------+     +------------------+     +------------------+
|  镜像构建时      |     |  容器启动时      |     |  geth 进程启动   |
|  嵌入安全参数    | --> |  传入运行时参数  | --> |  参数校验        |
+------------------+     +------------------+     +------------------+
                                                         |
                                                         v
                                                 +------------------+
                                                 |  比对安全参数    |
                                                 |  不一致则退出    |
                                                 +------------------+
```

## 链上安全参数配置

### 安全参数架构

X Chain 的安全参数分为两类：

| 类别 | 存储位置 | 特点 |
|------|----------|------|
| **Manifest 固定参数** | Gramine Manifest | 影响 MRENCLAVE，不可篡改 |
| **链上安全参数** | 链上合约 | 通过投票管理，动态生效 |

### 链上安全参数

所有治理相关的安全参数从链上合约动态读取：

| 参数 | 链上合约 | 说明 |
|------|----------|------|
| MRENCLAVE 白名单 | SecurityConfigContract | 允许的 enclave 代码度量值 |
| MRSIGNER 白名单 | SecurityConfigContract | 允许的签名者度量值 |
| 密钥迁移阈值 | SecurityConfigContract | 密钥迁移所需的最小节点数 |
| 节点准入策略 | SecurityConfigContract | 是否严格验证 Quote |
| 分叉配置 | SecurityConfigContract | 硬分叉升级相关配置 |
| 数据迁移策略 | SecurityConfigContract | 加密数据迁移相关配置 |
| 投票阈值 | GovernanceContract | 提案通过所需的投票比例 |

**注意**：所有安全、准入、秘密数据管理策略等相关配置都存储在安全配置合约中，但对安全配置合约的管理（修改配置）由治理合约通过投票实现。

**重要说明**：
- 白名单不再存储在 Manifest 环境变量中
- 节点启动后从链上合约动态读取安全参数
- 投票结果实时生效，无需重新构建镜像

## 硬件支持检测

### SGX 支持检测脚本

```bash
#!/bin/bash
# check_sgx_support.sh
# 检测 SGX 硬件支持

check_cpu_support() {
    echo "Checking CPU SGX support..."
    
    if grep -q sgx /proc/cpuinfo; then
        echo "  CPU supports SGX"
        return 0
    else
        echo "  ERROR: CPU does not support SGX"
        return 1
    fi
}

check_bios_enabled() {
    echo "Checking BIOS SGX settings..."
    
    # 检查 SGX 是否在 BIOS 中启用
    if [ -e /dev/sgx_enclave ] || [ -e /dev/sgx/enclave ]; then
        echo "  SGX is enabled in BIOS"
        return 0
    else
        echo "  WARNING: SGX device not found, may not be enabled in BIOS"
        return 1
    fi
}

check_driver_loaded() {
    echo "Checking SGX driver..."
    
    if lsmod | grep -q sgx; then
        echo "  SGX driver loaded"
        return 0
    else
        echo "  WARNING: SGX driver not loaded"
        return 1
    fi
}

check_aesm_service() {
    echo "Checking AESM service..."
    
    if systemctl is-active --quiet aesmd; then
        echo "  AESM service is running"
        return 0
    elif [ -S /var/run/aesmd/aesm.socket ]; then
        echo "  AESM socket available"
        return 0
    else
        echo "  WARNING: AESM service not running"
        return 1
    fi
}

check_dcap_support() {
    echo "Checking DCAP support..."
    
    if [ -e /dev/sgx_provision ]; then
        echo "  DCAP provisioning device available"
        return 0
    else
        echo "  WARNING: DCAP provisioning device not found"
        return 1
    fi
}

main() {
    echo "=== SGX Support Check ==="
    echo ""
    
    local errors=0
    
    check_cpu_support || ((errors++))
    check_bios_enabled || ((errors++))
    check_driver_loaded || ((errors++))
    check_aesm_service || ((errors++))
    check_dcap_support || ((errors++))
    
    echo ""
    if [ $errors -eq 0 ]; then
        echo "=== All checks passed ==="
        exit 0
    else
        echo "=== $errors check(s) failed ==="
        exit 1
    fi
}

main
```

## 单元测试

### Manifest 配置测试

```go
// gramine/manifest_test.go
package gramine

import (
    "encoding/base64"
    "os"
    "os/exec"
    "strings"
    "testing"
)

func TestManifestGeneration(t *testing.T) {
    // 准备测试数据
    whitelist := "abc123,1.0.0,test\ndef456,1.0.1,test2"
    whitelistBase64 := base64.StdEncoding.EncodeToString([]byte(whitelist))
    
    // 创建临时 manifest 模板
    template := `
libos.entrypoint = "/app/geth"
[loader.env]
XCHAIN_MRENCLAVE_WHITELIST = "{{ mrenclave_whitelist_base64 }}"
`
    
    tmpTemplate, err := os.CreateTemp("", "geth.manifest.template")
    if err != nil {
        t.Fatalf("Failed to create temp template: %v", err)
    }
    defer os.Remove(tmpTemplate.Name())
    
    if _, err := tmpTemplate.WriteString(template); err != nil {
        t.Fatalf("Failed to write template: %v", err)
    }
    tmpTemplate.Close()
    
    // 生成 manifest
    tmpManifest, err := os.CreateTemp("", "geth.manifest")
    if err != nil {
        t.Fatalf("Failed to create temp manifest: %v", err)
    }
    defer os.Remove(tmpManifest.Name())
    
    cmd := exec.Command("gramine-manifest",
        "-Dmrenclave_whitelist_base64="+whitelistBase64,
        tmpTemplate.Name(),
        tmpManifest.Name(),
    )
    
    if output, err := cmd.CombinedOutput(); err != nil {
        t.Fatalf("Failed to generate manifest: %v\nOutput: %s", err, output)
    }
    
    // 验证生成的 manifest
    content, err := os.ReadFile(tmpManifest.Name())
    if err != nil {
        t.Fatalf("Failed to read manifest: %v", err)
    }
    
    if !strings.Contains(string(content), whitelistBase64) {
        t.Error("Manifest does not contain whitelist")
    }
}

func TestWhitelistEncoding(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected int // 预期的条目数
    }{
        {
            name:     "Single entry",
            input:    "abc123,1.0.0,test",
            expected: 1,
        },
        {
            name:     "Multiple entries",
            input:    "abc123,1.0.0,test\ndef456,1.0.1,test2",
            expected: 2,
        },
        {
            name:     "Empty",
            input:    "",
            expected: 0,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 编码
            encoded := base64.StdEncoding.EncodeToString([]byte(tt.input))
            
            // 解码
            decoded, err := base64.StdEncoding.DecodeString(encoded)
            if err != nil {
                t.Fatalf("Failed to decode: %v", err)
            }
            
            // 验证
            if string(decoded) != tt.input {
                t.Errorf("Decoded content mismatch: got %s, want %s", decoded, tt.input)
            }
            
            // 计算条目数
            lines := strings.Split(string(decoded), "\n")
            count := 0
            for _, line := range lines {
                if line != "" {
                    count++
                }
            }
            
            if count != tt.expected {
                t.Errorf("Entry count mismatch: got %d, want %d", count, tt.expected)
            }
        })
    }
}
```

### 参数校验测试

```go
// gramine/param_validation_test.go
package gramine

import (
    "os"
    "testing"
)

func TestSecurityParamValidation(t *testing.T) {
    // 设置 manifest 环境变量
    os.Setenv("XCHAIN_ENCRYPTED_PATH", "/data/encrypted")
    os.Setenv("XCHAIN_SECRET_PATH", "/data/secrets")
    os.Setenv("XCHAIN_MRENCLAVE_WHITELIST", "YWJjMTIzLDEuMC4wLHRlc3Q=") // base64
    defer func() {
        os.Unsetenv("XCHAIN_ENCRYPTED_PATH")
        os.Unsetenv("XCHAIN_SECRET_PATH")
        os.Unsetenv("XCHAIN_MRENCLAVE_WHITELIST")
    }()
    
    validator := NewParamValidator()
    
    // 步骤 1: 加载 manifest 参数
    if err := validator.LoadManifestParams(); err != nil {
        t.Fatalf("Failed to load manifest params: %v", err)
    }
    
    // 步骤 2: 加载命令行参数
    cliArgs := []string{
        "--xchain.block.interval=15",
        "--xchain.rpc.port=8545",
    }
    if err := validator.LoadCliParams(cliArgs); err != nil {
        t.Fatalf("Failed to load CLI params: %v", err)
    }
    
    // 步骤 3: 合并和校验
    if err := validator.MergeAndValidate(); err != nil {
        t.Fatalf("Merge and validate failed: %v", err)
    }
    
    // 验证安全参数来自 manifest
    encPath, err := validator.GetSecurityParam("encrypted_path")
    if err != nil {
        t.Fatalf("Failed to get security param: %v", err)
    }
    if encPath != "/data/encrypted" {
        t.Errorf("Unexpected encrypted path: %s", encPath)
    }
    
    // 验证运行时参数
    interval := validator.GetRuntimeParam("xchain.block.interval")
    if interval != "15" {
        t.Errorf("Unexpected block interval: %s", interval)
    }
}

func TestSecurityParamConflict(t *testing.T) {
    // 设置 manifest 环境变量
    os.Setenv("XCHAIN_ENCRYPTED_PATH", "/data/encrypted")
    os.Setenv("XCHAIN_SECRET_PATH", "/data/secrets")
    os.Setenv("XCHAIN_MRENCLAVE_WHITELIST", "YWJjMTIzLDEuMC4wLHRlc3Q=")
    defer func() {
        os.Unsetenv("XCHAIN_ENCRYPTED_PATH")
        os.Unsetenv("XCHAIN_SECRET_PATH")
        os.Unsetenv("XCHAIN_MRENCLAVE_WHITELIST")
    }()
    
    validator := NewParamValidator()
    
    // 加载 manifest 参数
    if err := validator.LoadManifestParams(); err != nil {
        t.Fatalf("Failed to load manifest params: %v", err)
    }
    
    // 尝试通过命令行覆盖安全参数
    cliArgs := []string{
        "--xchain.encrypted-path=/malicious/path", // 尝试覆盖安全参数
    }
    if err := validator.LoadCliParams(cliArgs); err != nil {
        t.Fatalf("Failed to load CLI params: %v", err)
    }
    
    // 合并时应该检测到冲突
    err := validator.MergeAndValidate()
    if err == nil {
        t.Fatal("Expected security violation error, got nil")
    }
    
    // 验证错误信息
    if !strings.Contains(err.Error(), "SECURITY VIOLATION") {
        t.Errorf("Expected SECURITY VIOLATION error, got: %v", err)
    }
}
```

### Docker 构建测试

```bash
#!/bin/bash
# test_docker_build.sh
# 测试 Docker 镜像构建

set -e

echo "=== Testing Docker Build ==="

# 准备测试白名单
WHITELIST="abc123def456,1.0.0,test"
WHITELIST_BASE64=$(echo -n "$WHITELIST" | base64 -w0)

# 构建测试镜像
echo "Building test image..."
docker build \
    --build-arg MRENCLAVE_WHITELIST="$WHITELIST_BASE64" \
    --build-arg KEY_MIGRATION_ENABLED="false" \
    --build-arg ADMISSION_STRICT="true" \
    -t xchain/geth:test \
    -f Dockerfile.test \
    .

# 验证镜像包含预编译的 manifest
echo "Verifying manifest files..."
docker run --rm xchain/geth:test ls -la /app/ | grep -E "\.manifest|\.sig"

# 验证 manifest 内容
echo "Checking manifest content..."
docker run --rm xchain/geth:test cat /app/geth.manifest | grep -q "XCHAIN_MRENCLAVE_WHITELIST"

echo "=== Docker Build Test Passed ==="
```

## 配置参数

### 构建时参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `MRENCLAVE_WHITELIST` | 度量值白名单（Base64 CSV） | 必填 |
| `KEY_MIGRATION_ENABLED` | 启用密钥迁移 | "false" |
| `KEY_MIGRATION_THRESHOLD` | 密钥迁移阈值 | "2" |
| `ADMISSION_STRICT` | 严格准入控制 | "true" |
| `RATLS_WHITELIST` | RA-TLS 白名单（Base64 CSV） | 可选 |
| `LOG_LEVEL` | 日志级别 | "error" |

### 运行时参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--xchain.block.interval` | 出块间隔（秒） | 15 |
| `--xchain.block.max-tx` | 每块最大交易数 | 1000 |
| `--xchain.block.max-gas` | 每块最大 Gas | 30000000 |
| `--xchain.rpc.port` | RPC 端口 | 8545 |
| `--xchain.p2p.port` | P2P 端口 | 30303 |
| `--xchain.log.level` | 日志级别 | info |
| `--xchain.metrics.enabled` | 启用指标 | false |

## 部署检查清单

### 部署前检查

- [ ] SGX 硬件支持已启用
- [ ] SGX 驱动已安装
- [ ] AESM 服务已运行
- [ ] DCAP 配置已完成
- [ ] PCCS 服务可访问

### 镜像构建检查

- [ ] 安全参数已正确设置
- [ ] Manifest 已预编译
- [ ] Manifest 已签名
- [ ] MRENCLAVE 值已记录

### 运行时检查

- [ ] SGX 设备已挂载到容器
- [ ] AESM socket 已挂载
- [ ] 数据卷已配置
- [ ] 网络端口已映射

## 实现优先级

1. **P0 - 核心功能**
   - Manifest 模板配置
   - 安全参数嵌入
   - Docker 镜像构建流程

2. **P1 - 安全功能**
   - 加密分区配置
   - 参数校验机制
   - 白名单格式处理

3. **P2 - 运维功能**
   - 启动脚本
   - 硬件检测
   - Docker Compose 配置

## 预计工时

- 核心功能开发：1 周
- 安全功能开发：1 周
- 运维功能开发：0.5 周
- 测试和文档：0.5 周

**总计：约 3 周**
