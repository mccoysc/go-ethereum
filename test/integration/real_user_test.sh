#!/bin/bash
# 真实用户场景集成测试
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}=== X Chain 真实用户场景测试 ===${NC}"

# 配置
NODE_DIR="${REPO_ROOT}/test-node"
GETH="${REPO_ROOT}/build/bin/geth"
GENESIS="${REPO_ROOT}/test/integration/genesis.json"
NETWORK_ID="762385986"
RPC_PORT="8545"
RPC_URL="http://localhost:${RPC_PORT}"

# 检查依赖
if [ ! -f "$GETH" ]; then
    echo -e "${RED}错误: geth 未编译，运行: make geth${NC}"
    exit 1
fi

if ! command -v jq &> /dev/null; then
    echo -e "${YELLOW}安装 jq...${NC}"
    sudo apt-get update && sudo apt-get install -y jq
fi

# 清理并初始化
echo -e "${BLUE}[准备] 初始化测试环境...${NC}"
rm -rf "$NODE_DIR"
mkdir -p "$NODE_DIR"
$GETH --datadir "$NODE_DIR" init "$GENESIS"

# 创建测试账户
echo "$TEST_PASSWORD" > "$NODE_DIR/password.txt"
ACCOUNT=$($GETH --datadir "$NODE_DIR" account new --password "$NODE_DIR/password.txt" 2>&1 | grep -oP '0x[a-fA-F0-9]{40}')
echo -e "${GREEN}✓ 测试账户: ${ACCOUNT}${NC}"

# 启动节点
echo -e "${BLUE}[启动] 启动节点...${NC}"
$GETH --datadir "$NODE_DIR" \
    --networkid "$NETWORK_ID" \
    --http --http.addr "0.0.0.0" --http.port "$RPC_PORT" \
    --http.api "eth,net,web3,personal,admin" \
    --nodiscover --maxpeers 0 \
    --mine --miner.etherbase "$ACCOUNT" \
    --allow-insecure-unlock \
    > "$NODE_DIR/node.log" 2>&1 &

NODE_PID=$!
echo "$NODE_PID" > "$NODE_DIR/node.pid"

cleanup() {
    echo -e "${YELLOW}清理...${NC}"
    kill $NODE_PID 2>/dev/null || true
}
trap cleanup EXIT

# 等待节点启动
for i in {1..30}; do
    if curl -s "$RPC_URL" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ 节点就绪${NC}"
        break
    fi
    sleep 1
done

echo ""
echo -e "${GREEN}=== 开始真实用户测试 ===${NC}"
echo ""

# 测试 1: 查询网络信息
echo -e "${BLUE}【场景 1】连接网络${NC}"
CHAIN_ID=$(curl -s -X POST "$RPC_URL" -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' | jq -r .result)
echo -e "  ✓ Chain ID: $((CHAIN_ID))"

# 测试 2: 查询余额
BALANCE=$(curl -s -X POST "$RPC_URL" -H "Content-Type: application/json" \
    -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"$ACCOUNT\",\"latest\"],\"id\":1}" | jq -r .result)
echo -e "  ✓ 账户余额: $BALANCE"
echo ""

# 测试 3: 调用预编译合约
echo -e "${BLUE}【场景 2】调用预编译合约${NC}"
RANDOM=$(curl -s -X POST "$RPC_URL" -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x8005","data":"0x00000020"},"latest"],"id":1}' | jq -r .result)
echo -e "  ✓ 随机数生成: ${RANDOM:0:18}..."
echo ""

sleep 3
BLOCK=$(curl -s -X POST "$RPC_URL" -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq -r .result)
echo -e "${GREEN}✓ 测试完成，产生 $((BLOCK)) 个区块${NC}"
echo ""
echo "日志: $NODE_DIR/node.log"

wait
