#!/bin/bash
# run-local.sh
# 在 Gramine 镜像环境中本地测试（直接运行 geth，不使用 gramine 包装）
# 用于功能开发和集成测试，SGX 功能使用 mock 数据

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== X Chain 本地集成测试（Gramine 容器环境）===${NC}"
echo -e "${BLUE}模式: 在 Gramine 镜像中直接运行 geth（非 gramine-sgx 包装）${NC}"
echo ""

# 检查 Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}错误: Docker 未安装${NC}"
    exit 1
fi

# 检查 geth 二进制
GETH_BIN="${REPO_ROOT}/build/bin/geth"
if [ ! -f "${GETH_BIN}" ]; then
    echo -e "${RED}错误: geth 二进制文件不存在${NC}"
    echo "请先在 Gramine 环境中编译: ./build-in-gramine.sh"
    exit 1
fi

echo -e "${GREEN}✓ 找到 geth: ${GETH_BIN}${NC}"

# 创建本地测试数据目录
LOCAL_DATA_DIR="${REPO_ROOT}/test-data/local"
mkdir -p "${LOCAL_DATA_DIR}/chaindata"
mkdir -p "${LOCAL_DATA_DIR}/keystore"
mkdir -p "${LOCAL_DATA_DIR}/logs"
mkdir -p "${LOCAL_DATA_DIR}/encrypted"
mkdir -p "${LOCAL_DATA_DIR}/secrets"

echo -e "${GREEN}✓ 测试数据目录: ${LOCAL_DATA_DIR}${NC}"

# 配置参数
NETWORK_ID="${XCHAIN_NETWORK_ID:-762385986}"
DATA_DIR="/data/chaindata"
RPC_PORT="${XCHAIN_RPC_PORT:-8545}"
WS_PORT="${XCHAIN_WS_PORT:-8546}"
P2P_PORT="${XCHAIN_P2P_PORT:-30303}"

# 合约地址（测试环境）
GOVERNANCE_CONTRACT="${GOVERNANCE_CONTRACT:-0x0000000000000000000000000000000000001001}"
SECURITY_CONFIG_CONTRACT="${SECURITY_CONFIG_CONTRACT:-0x0000000000000000000000000000000000001002}"

echo ""
echo -e "${YELLOW}运行配置:${NC}"
echo "  容器环境: gramineproject/gramine:latest"
echo "  Network ID: ${NETWORK_ID}"
echo "  Data Dir: ${DATA_DIR}"
echo "  RPC Port: ${RPC_PORT}"
echo "  WS Port: ${WS_PORT}"
echo "  P2P Port: ${P2P_PORT}"
echo ""
echo -e "${YELLOW}环境变量（SGX Mock）:${NC}"
echo "  XCHAIN_SGX_MODE: mock"
echo "  XCHAIN_GOVERNANCE_CONTRACT: ${GOVERNANCE_CONTRACT}"
echo "  XCHAIN_SECURITY_CONFIG_CONTRACT: ${SECURITY_CONFIG_CONTRACT}"
echo ""

# 检查是否需要初始化创世区块
GENESIS_MARKER="${LOCAL_DATA_DIR}/chaindata/genesis-initialized"
if [ ! -f "${GENESIS_MARKER}" ]; then
    echo -e "${YELLOW}首次运行，在容器中初始化创世区块...${NC}"
    
    # 检查创世配置文件
    GENESIS_FILE="${SCRIPT_DIR}/genesis-local.json"
    if [ ! -f "${GENESIS_FILE}" ]; then
        echo -e "${RED}错误: 找不到创世配置文件 ${GENESIS_FILE}${NC}"
        exit 1
    fi
    
    # 在 Gramine 容器中初始化
    docker run --rm \
        -v "${REPO_ROOT}/build/bin:/app/bin" \
        -v "${LOCAL_DATA_DIR}:/data" \
        -v "${GENESIS_FILE}:/genesis.json" \
        gramineproject/gramine:latest \
        /app/bin/geth init --datadir /data/chaindata /genesis.json
    
    touch "${GENESIS_MARKER}"
    echo -e "${GREEN}✓ 创世区块初始化完成${NC}"
    echo ""
fi

echo -e "${BLUE}=== 在 Gramine 容器中启动节点（本地测试模式）===${NC}"
echo ""
echo -e "${YELLOW}说明:${NC}"
echo "  - 在 Gramine 官方镜像容器中运行"
echo "  - 直接运行 geth（不使用 gramine-sgx/gramine-direct 包装）"
echo "  - SGX 功能使用 mock 数据"
echo "  - 用于功能开发和集成测试"
echo "  - 不提供真实的 SGX 安全保障"
echo ""
echo -e "${YELLOW}按 Ctrl+C 停止节点${NC}"
echo ""

# 在 Gramine 容器中直接运行 geth
docker run --rm -it \
    --name xchain-local-test \
    -v "${REPO_ROOT}/build/bin:/app/bin" \
    -v "${LOCAL_DATA_DIR}:/data" \
    -p "${RPC_PORT}:${RPC_PORT}" \
    -p "${WS_PORT}:${WS_PORT}" \
    -p "${P2P_PORT}:${P2P_PORT}" \
    -e XCHAIN_SGX_MODE=mock \
    -e XCHAIN_ENCRYPTED_PATH=/data/encrypted \
    -e XCHAIN_SECRET_PATH=/data/secrets \
    -e XCHAIN_GOVERNANCE_CONTRACT="${GOVERNANCE_CONTRACT}" \
    -e XCHAIN_SECURITY_CONFIG_CONTRACT="${SECURITY_CONFIG_CONTRACT}" \
    gramineproject/gramine:latest \
    /app/bin/geth \
        --datadir "${DATA_DIR}" \
        --networkid "${NETWORK_ID}" \
        --syncmode full \
        --gcmode archive \
        --http \
        --http.addr 0.0.0.0 \
        --http.port "${RPC_PORT}" \
        --http.api eth,net,web3,admin,debug,personal,sgx \
        --http.corsdomain "*" \
        --ws \
        --ws.addr 0.0.0.0 \
        --ws.port "${WS_PORT}" \
        --ws.api eth,net,web3,admin,debug,personal,sgx \
        --ws.origins "*" \
        --port "${P2P_PORT}" \
        --maxpeers 50 \
        --verbosity 4 \
        --nodiscover \
        --allow-insecure-unlock
