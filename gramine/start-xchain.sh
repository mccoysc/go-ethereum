#!/bin/bash
# start-xchain.sh
# Docker 容器启动脚本
# 支持 gramine-sgx 和 gramine-direct 模式

set -e

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}=== X Chain 节点启动 ===${NC}"
echo "版本: ${XCHAIN_VERSION:-unknown}"
echo "时间: $(date -u +"%Y-%m-%d %H:%M:%S UTC")"
echo ""

# 运行模式
RUN_MODE="${1:-sgx}"

# 配置参数
NETWORK_ID="${XCHAIN_NETWORK_ID:-762385986}"
DATA_DIR="${XCHAIN_DATA_DIR:-/data/wallet/chaindata}"
RPC_PORT="${XCHAIN_RPC_PORT:-8545}"
WS_PORT="${XCHAIN_WS_PORT:-8546}"
P2P_PORT="${XCHAIN_P2P_PORT:-30303}"

echo -e "${BLUE}配置:${NC}"
echo "  模式: ${RUN_MODE}"
echo "  Network ID: ${NETWORK_ID}"
echo "  Data Dir: ${DATA_DIR}"
echo "  RPC Port: ${RPC_PORT}"
echo "  WS Port: ${WS_PORT}"
echo "  P2P Port: ${P2P_PORT}"
echo ""

# 检查是否需要初始化
if [ ! -d "${DATA_DIR}/geth" ]; then
    echo -e "${YELLOW}首次运行，初始化创世区块...${NC}"
    /app/geth init --datadir "${DATA_DIR}" /app/genesis.json
    echo -e "${GREEN}✓ 创世区块初始化完成${NC}"
fi

# 显示 MRENCLAVE
if [ -f "/app/MRENCLAVE.txt" ]; then
    MRENCLAVE=$(cat /app/MRENCLAVE.txt)
    echo -e "${BLUE}MRENCLAVE: ${MRENCLAVE}${NC}"
fi

echo ""

# 根据模式启动
if [ "${RUN_MODE}" = "direct" ]; then
    echo -e "${YELLOW}使用 gramine-direct 模式（模拟器）${NC}"
    echo -e "${RED}警告: 此模式不提供 SGX 安全保障，仅用于测试${NC}"
    echo ""
    
    exec gramine-direct geth \
        --datadir "${DATA_DIR}" \
        --networkid "${NETWORK_ID}" \
        --syncmode full \
        --gcmode archive \
        --http \
        --http.addr 0.0.0.0 \
        --http.port "${RPC_PORT}" \
        --http.api eth,net,web3,sgx \
        --http.corsdomain "*" \
        --ws \
        --ws.addr 0.0.0.0 \
        --ws.port "${WS_PORT}" \
        --ws.api eth,net,web3,sgx \
        --ws.origins "*" \
        --port "${P2P_PORT}" \
        --maxpeers 50 \
        --verbosity 3
        
elif [ "${RUN_MODE}" = "sgx" ]; then
    echo -e "${GREEN}使用 gramine-sgx 模式（SGX Enclave）${NC}"
    
    # 检查 SGX 设备
    if [ ! -c /dev/sgx_enclave ]; then
        echo -e "${RED}错误: SGX 设备不存在 (/dev/sgx_enclave)${NC}"
        echo "请确保:"
        echo "  1. 运行在支持 SGX 的硬件上"
        echo "  2. Docker 容器映射了 SGX 设备"
        echo "  3. 添加: --device=/dev/sgx_enclave --device=/dev/sgx_provision"
        exit 1
    fi
    
    # 检查 AESM 服务
    if [ ! -S /var/run/aesmd/aesm.socket ]; then
        echo -e "${YELLOW}警告: AESM socket 不存在${NC}"
        echo "远程证明可能无法工作"
        echo "请确保挂载: -v /var/run/aesmd:/var/run/aesmd"
    fi
    
    echo ""
    echo -e "${GREEN}在 SGX Enclave 中启动节点...${NC}"
    echo ""
    
    exec gramine-sgx geth \
        --datadir "${DATA_DIR}" \
        --networkid "${NETWORK_ID}" \
        --syncmode full \
        --gcmode archive \
        --http \
        --http.addr 0.0.0.0 \
        --http.port "${RPC_PORT}" \
        --http.api eth,net,web3,sgx \
        --http.corsdomain "*" \
        --ws \
        --ws.addr 0.0.0.0 \
        --ws.port "${WS_PORT}" \
        --ws.api eth,net,web3,sgx \
        --ws.origins "*" \
        --port "${P2P_PORT}" \
        --maxpeers 50 \
        --verbosity 3
else
    echo -e "${RED}错误: 未知的运行模式 '${RUN_MODE}'${NC}"
    echo "用法: $0 [sgx|direct]"
    exit 1
fi
