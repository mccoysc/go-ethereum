#!/bin/bash
set -e
cd "$(dirname "$0")/../.."

echo "=== 快速集成测试 ==="

# 清理
rm -rf test-node
./build/bin/geth --datadir test-node init test/integration/genesis.json > /dev/null 2>&1

# 创建账户  
echo "test" > test-node/pass.txt
ACCOUNT=$(./build/bin/geth --datadir test-node account new --password test-node/pass.txt 2>&1 | grep -oP '0x[a-fA-F0-9]{40}')
echo "账户: $ACCOUNT"

# 启动节点
./build/bin/geth --datadir test-node --networkid 762385986 \
  --http --http.addr 127.0.0.1 --http.port 8545 \
  --http.api eth,net,web3,personal \
  --nodiscover --maxpeers 0 --mine --miner.etherbase "$ACCOUNT" \
  --allow-insecure-unlock > test-node/node.log 2>&1 &
PID=$!

cleanup() { kill $PID 2>/dev/null || true; }
trap cleanup EXIT

# 等待
sleep 10

# 测试
echo ""
echo "【测试 1】网络信息"
curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"net_version","params":[],"id":1}' | jq .

echo ""
echo "【测试 2】区块高度"
curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq .

echo ""
echo "【测试 3】预编译合约 0x8005"
curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x8005","data":"0x00000020"},"latest"],"id":1}' | jq .

echo ""  
echo "【测试 4】账户余额"
curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
  -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"$ACCOUNT\",\"latest\"],\"id\":1}" | jq .

echo ""
echo "✓ 测试完成"
sleep 5
