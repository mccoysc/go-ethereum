#!/bin/bash
# run-dev.sh
# 快速运行 X Chain 节点（开发测试模式）
# 支持 gramine-direct（模拟器）和 gramine-sgx（真实 SGX）

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# 运行模式
RUN_MODE="${1:-direct}"  # direct 或 sgx

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== X Chain 节点快速启动（开发模式）===${NC}"

# 检查 geth 二进制
GETH_BIN="${REPO_ROOT}/build/bin/geth"
if [ ! -f "${GETH_BIN}" ]; then
    echo -e "${RED}错误: geth 二进制文件不存在${NC}"
    echo "请先编译: make geth"
    exit 1
fi

# 创建软链接到 /app/geth（manifest 中的路径）
mkdir -p /app
if [ ! -L /app/geth ]; then
    sudo ln -sf "${GETH_BIN}" /app/geth
    echo -e "${GREEN}✓ 已创建软链接: /app/geth -> ${GETH_BIN}${NC}"
fi

# 检查 manifest 文件
cd "${SCRIPT_DIR}"
if [ ! -f "geth.manifest.sgx" ]; then
    echo -e "${YELLOW}Manifest 文件不存在，正在生成...${NC}"
    ./rebuild-manifest.sh dev
fi

# 创建必要的数据目录
mkdir -p /data/encrypted /data/secrets /data/wallet /app/logs

# 配置运行参数
NETWORK_ID="${XCHAIN_NETWORK_ID:-762385986}"
DATA_DIR="${XCHAIN_DATA_DIR:-/data/wallet/chaindata}"
RPC_PORT="${XCHAIN_RPC_PORT:-8545}"
WS_PORT="${XCHAIN_WS_PORT:-8546}"
P2P_PORT="${XCHAIN_P2P_PORT:-30303}"

echo ""
echo "运行配置:"
echo "  模式: ${RUN_MODE}"
echo "  Network ID: ${NETWORK_ID}"
echo "  Data Dir: ${DATA_DIR}"
echo "  RPC Port: ${RPC_PORT}"
echo ""

# 根据模式选择运行命令
if [ "${RUN_MODE}" = "direct" ]; then
    echo -e "${YELLOW}使用 gramine-direct 模拟模式运行${NC}"
    echo "说明: 此模式在模拟器中运行，无需 SGX 硬件，适合快速开发测试"
    echo ""
    
    exec gramine-direct geth \
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
        
elif [ "${RUN_MODE}" = "sgx" ]; then
    echo -e "${YELLOW}使用 gramine-sgx SGX 模式运行${NC}"
    echo "说明: 此模式在真实 SGX enclave 中运行，需要 SGX 硬件支持"
    echo ""
    
    # 检查 SGX 设备
    if [ ! -c /dev/sgx_enclave ]; then
        echo -e "${RED}警告: SGX 设备不存在 (/dev/sgx_enclave)${NC}"
        echo "请确保:"
        echo "  1. CPU 支持 SGX"
        echo "  2. BIOS 中已启用 SGX"
        echo "  3. 已安装 SGX 驱动"
        echo ""
        echo "继续使用 gramine-sgx 可能会失败..."
        read -p "是否继续? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
    
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
else
    echo -e "${RED}错误: 未知的运行模式 '${RUN_MODE}'${NC}"
    echo "用法: $0 [direct|sgx]"
    echo ""
    echo "  direct - 使用 gramine-direct 在模拟器中运行（无需 SGX）"
    echo "  sgx    - 使用 gramine-sgx 在真实 SGX 环境中运行"
    exit 1
fi
