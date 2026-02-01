#!/bin/bash
# run-local.sh
# 本地运行 X Chain 节点（无需 Gramine）
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

echo -e "${GREEN}=== X Chain 本地集成测试环境 ===${NC}"
echo -e "${BLUE}模式: 本地运行（无 Gramine）${NC}"
echo ""

# 检查 geth 二进制
GETH_BIN="${REPO_ROOT}/build/bin/geth"
if [ ! -f "${GETH_BIN}" ]; then
    echo -e "${RED}错误: geth 二进制文件不存在${NC}"
    echo "请先编译: make geth"
    exit 1
fi

echo -e "${GREEN}✓ 找到 geth: ${GETH_BIN}${NC}"

# 创建本地测试数据目录
LOCAL_DATA_DIR="${REPO_ROOT}/test-data/local"
mkdir -p "${LOCAL_DATA_DIR}/chaindata"
mkdir -p "${LOCAL_DATA_DIR}/keystore"
mkdir -p "${LOCAL_DATA_DIR}/logs"

echo -e "${GREEN}✓ 测试数据目录: ${LOCAL_DATA_DIR}${NC}"

# 配置参数
NETWORK_ID="${XCHAIN_NETWORK_ID:-762385986}"
DATA_DIR="${LOCAL_DATA_DIR}/chaindata"
RPC_PORT="${XCHAIN_RPC_PORT:-8545}"
WS_PORT="${XCHAIN_WS_PORT:-8546}"
P2P_PORT="${XCHAIN_P2P_PORT:-30303}"

# 合约地址（测试环境）
GOVERNANCE_CONTRACT="${GOVERNANCE_CONTRACT:-0x0000000000000000000000000000000000001001}"
SECURITY_CONFIG_CONTRACT="${SECURITY_CONFIG_CONTRACT:-0x0000000000000000000000000000000000001002}"

echo ""
echo -e "${YELLOW}运行配置:${NC}"
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
if [ ! -d "${DATA_DIR}/geth" ]; then
    echo -e "${YELLOW}首次运行，需要初始化创世区块${NC}"
    
    # 检查创世配置文件
    GENESIS_FILE="${SCRIPT_DIR}/genesis-local.json"
    if [ ! -f "${GENESIS_FILE}" ]; then
        echo -e "${YELLOW}创世配置文件不存在，使用默认配置${NC}"
        GENESIS_FILE="${REPO_ROOT}/genesis/xchain-testnet.json"
        
        if [ ! -f "${GENESIS_FILE}" ]; then
            echo -e "${RED}错误: 找不到创世配置文件${NC}"
            echo "请创建 ${SCRIPT_DIR}/genesis-local.json 或确保 genesis/xchain-testnet.json 存在"
            exit 1
        fi
    fi
    
    echo "使用创世配置: ${GENESIS_FILE}"
    "${GETH_BIN}" init --datadir "${DATA_DIR}" "${GENESIS_FILE}"
    echo -e "${GREEN}✓ 创世区块初始化完成${NC}"
    echo ""
fi

echo -e "${BLUE}=== 启动 X Chain 节点（本地模式）===${NC}"
echo ""
echo -e "${YELLOW}说明:${NC}"
echo "  - 本地运行，不使用 Gramine"
echo "  - SGX 功能使用 mock 数据"
echo "  - 用于功能开发和集成测试"
echo "  - 不提供真实的 SGX 安全保障"
echo ""
echo -e "${YELLOW}按 Ctrl+C 停止节点${NC}"
echo ""

# 设置环境变量启用 SGX mock 模式
export XCHAIN_SGX_MODE="mock"
export XCHAIN_ENCRYPTED_PATH="${LOCAL_DATA_DIR}/encrypted"
export XCHAIN_SECRET_PATH="${LOCAL_DATA_DIR}/secrets"
export XCHAIN_GOVERNANCE_CONTRACT="${GOVERNANCE_CONTRACT}"
export XCHAIN_SECURITY_CONFIG_CONTRACT="${SECURITY_CONFIG_CONTRACT}"

# 创建 mock 数据目录
mkdir -p "${XCHAIN_ENCRYPTED_PATH}"
mkdir -p "${XCHAIN_SECRET_PATH}"

# 启动节点
exec "${GETH_BIN}" \
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
    --log.file "${LOCAL_DATA_DIR}/logs/geth.log" \
    --nodiscover \
    --allow-insecure-unlock
