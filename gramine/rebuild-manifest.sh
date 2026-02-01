#!/bin/bash
# rebuild-manifest.sh
# 快速重新生成和签名 Gramine manifest
# 用于开发测试时快速迭代

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# 默认参数
MODE="${1:-dev}"  # dev 或 prod
DEBUG="${2:-true}"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== 快速重新生成 Gramine Manifest ===${NC}"
echo "模式: ${MODE}"
echo "调试: ${DEBUG}"

# 配置参数
if [ "$MODE" = "prod" ]; then
    LOG_LEVEL="error"
    SEAL_KEY="_sgx_mrenclave"  # 生产模式：使用 MRENCLAVE
    ATTESTATION_TYPE="dcap"
    DEBUG_FLAG="false"
else
    LOG_LEVEL="warning"
    SEAL_KEY="_sgx_mrsigner"   # 开发模式：使用 MRSIGNER，避免数据迁移
    ATTESTATION_TYPE="none"
    DEBUG_FLAG="true"
fi

# 合约地址（从创世配置计算）
GOVERNANCE_CONTRACT="${GOVERNANCE_CONTRACT:-0x0000000000000000000000000000000000001001}"
SECURITY_CONFIG_CONTRACT="${SECURITY_CONFIG_CONTRACT:-0x0000000000000000000000000000000000001002}"

# 架构检测
ARCH_LIBDIR="/lib/x86_64-linux-gnu"

echo -e "${YELLOW}步骤 1/3: 生成 manifest 文件${NC}"
cd "${SCRIPT_DIR}"

gramine-manifest \
    -Dlog_level=${LOG_LEVEL} \
    -Darch_libdir=${ARCH_LIBDIR} \
    -Dgovernance_contract=${GOVERNANCE_CONTRACT} \
    -Dsecurity_config_contract=${SECURITY_CONFIG_CONTRACT} \
    -Ddebug=${DEBUG_FLAG} \
    -Dseal_key=${SEAL_KEY} \
    -Dattestation_type=${ATTESTATION_TYPE} \
    geth.manifest.template geth.manifest

echo -e "${GREEN}✓ Manifest 生成完成${NC}"

echo -e "${YELLOW}步骤 2/3: 签名 manifest${NC}"

# 检查签名密钥
if [ ! -f "${SCRIPT_DIR}/enclave-key.pem" ]; then
    echo "签名密钥不存在，正在生成..."
    openssl genrsa -3 -out "${SCRIPT_DIR}/enclave-key.pem" 3072
    echo -e "${GREEN}✓ 签名密钥已生成: ${SCRIPT_DIR}/enclave-key.pem${NC}"
fi

gramine-sgx-sign \
    --manifest geth.manifest \
    --output geth.manifest.sgx \
    --key enclave-key.pem

echo -e "${GREEN}✓ Manifest 签名完成${NC}"

echo -e "${YELLOW}步骤 3/3: 提取 MRENCLAVE${NC}"
MRENCLAVE=$(gramine-sgx-sigstruct-view geth.manifest.sgx | grep "mr_enclave" | awk '{print $2}')
echo "${MRENCLAVE}" > MRENCLAVE.txt

echo -e "${GREEN}=== 完成 ===${NC}"
echo "MRENCLAVE: ${MRENCLAVE}"
echo "Seal Key: ${SEAL_KEY}"
echo ""
echo "生成的文件:"
echo "  - geth.manifest      (生成的 manifest)"
echo "  - geth.manifest.sgx  (签名的 manifest)"
echo "  - MRENCLAVE.txt      (MRENCLAVE 值)"
echo ""

if [ "$MODE" = "dev" ]; then
    echo -e "${YELLOW}开发模式说明:${NC}"
    echo "  - 使用 MRSIGNER sealing（重新编译后数据不需要迁移）"
    echo "  - Debug 模式启用"
    echo "  - 可使用 gramine-direct 或 gramine-sgx 运行"
    echo ""
    echo "快速测试命令:"
    echo "  ./run-dev.sh direct  # 模拟模式，无需 SGX 硬件"
    echo "  ./run-dev.sh sgx     # SGX 模式，需要 SGX 硬件"
fi
