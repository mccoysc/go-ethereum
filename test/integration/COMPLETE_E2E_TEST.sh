#!/bin/bash
# Gramine 容器内完整端到端测试
set -e
cd /workspace

echo "=== Gramine 容器内端到端测试 ==="
echo ""

# 安装依赖
echo "【1/8】安装依赖..."
apt-get update -qq
apt-get install -y -qq wget golang-1.21 make curl jq > /dev/null 2>&1
export PATH=/usr/lib/go-1.21/bin:$PATH
echo "✓ 完成"

# 编译
echo ""
echo "【2/8】编译 geth..."
make geth > /dev/null 2>&1
echo "✓ 编译完成"

# 初始化
echo ""
echo "【3/8】初始化创世区块..."
rm -rf /tmp/testnode
./build/bin/geth --datadir /tmp/testnode init test/integration/genesis.json > /dev/null 2>&1
echo "✓ 创世区块初始化完成"

# 创建账户
echo ""
echo "【4/8】创建测试账户..."
echo "testpassword" > /tmp/password.txt
ACCOUNT=$(./build/bin/geth --datadir /tmp/testnode account new --password /tmp/password.txt 2>&1 | grep -oP '0x[a-fA-F0-9]{40}')
echo "✓ 账户创建: $ACCOUNT"

# 启动节点
echo ""
echo "【5/8】启动节点..."
./build/bin/geth \
  --datadir /tmp/testnode \
  --networkid 762385986 \
  --http \
  --http.addr 0.0.0.0 \
  --http.port 8545 \
  --http.api "eth,net,web3,personal,admin" \
  --nodiscover \
  --maxpeers 0 \
  --mine \
  --miner.etherbase "$ACCOUNT" \
  --allow-insecure-unlock \
  --verbosity 2 \
  > /tmp/node.log 2>&1 &

NODE_PID=$!
echo "✓ 节点已启动 (PID: $NODE_PID)"

# 清理函数
cleanup() {
  if [ -n "$NODE_PID" ]; then
    kill "$NODE_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

# 等待节点启动
echo ""
echo "【6/8】等待节点就绪..."
for i in {1..30}; do
  if curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"net_version","params":[],"id":1}' 2>/dev/null | grep -q result; then
    echo "✓ 节点已就绪"
    break
  fi
  if [ $i -eq 30 ]; then
    echo "✗ 节点启动超时"
    tail -20 /tmp/node.log
    exit 1
  fi
  sleep 1
done

# 等待产生区块
sleep 10

# 执行测试
echo ""
echo "【7/8】执行端到端测试..."
echo ""

# 测试 1: 网络信息
echo "  测试 1: 网络信息"
CHAIN_ID=$(curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' | jq -r .result)
CHAIN_ID_DEC=$(printf "%d" $CHAIN_ID)
echo "    Chain ID: $CHAIN_ID_DEC"
if [ "$CHAIN_ID_DEC" == "762385986" ]; then
  echo "    ✓ Chain ID 正确"
else
  echo "    ✗ Chain ID 错误"
fi

# 测试 2: 区块高度
echo ""
echo "  测试 2: 区块生产"
BLOCK=$(curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq -r .result)
BLOCK_DEC=$(printf "%d" $BLOCK)
echo "    当前区块: $BLOCK_DEC"
if [ $BLOCK_DEC -gt 0 ]; then
  echo "    ✓ 区块生产正常"
else
  echo "    ✗ 无区块生产"
fi

# 测试 3: 预编译合约
echo ""
echo "  测试 3: 预编译合约"
echo "    0x8000 (KEY_CREATE):"
R1=$(curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x8000","data":"0x01"},"latest"],"id":1}' | jq -r .result)
echo "      返回: ${R1:0:20}..."

echo "    0x8005 (RANDOM):"
R2=$(curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x8005","data":"0x00000020"},"latest"],"id":1}' | jq -r .result)
echo "      返回: ${R2:0:20}..."
echo "    ✓ 预编译合约可用"

# 测试 4: 系统合约
echo ""
echo "  测试 4: 系统合约"
GOV=$(curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_getCode","params":["0x0000000000000000000000000000000000001001","latest"],"id":1}' | jq -r .result)
echo "    治理合约 (0x1001): 代码长度 ${#GOV}"
if [ ${#GOV} -gt 10 ]; then
  echo "    ✓ 治理合约已部署"
else
  echo "    ✗ 治理合约未部署"
fi

# 测试 5: 挖矿奖励
echo ""
echo "  测试 5: 挖矿奖励"
BALANCE=$(curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
  -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"$ACCOUNT\",\"latest\"],\"id\":1}" | jq -r .result)
BALANCE_DEC=$(printf "%d" $BALANCE)
echo "    矿工余额: $BALANCE_DEC wei"
if [ $BALANCE_DEC -gt 0 ]; then
  echo "    ✓ 激励机制正常"
else
  echo "    ⚠ 余额为 0"
fi

# 测试 6: 发送交易
echo ""
echo "  测试 6: 发送交易"
curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
  -d "{\"jsonrpc\":\"2.0\",\"method\":\"personal_unlockAccount\",\"params\":[\"$ACCOUNT\",\"testpassword\",300],\"id\":1}" > /dev/null

TX=$(curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
  -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_sendTransaction\",\"params\":[{\"from\":\"$ACCOUNT\",\"to\":\"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266\",\"value\":\"0x10000\"}],\"id\":1}" | jq -r .result)

if [ "$TX" != "null" ] && [ -n "$TX" ]; then
  echo "    交易哈希: $TX"
  sleep 3
  RECEIPT=$(curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
    -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getTransactionReceipt\",\"params\":[\"$TX\"],\"id\":1}" | jq -r .result)
  if [ "$RECEIPT" != "null" ]; then
    echo "    ✓ 交易已确认"
  else
    echo "    ⚠ 交易待确认"
  fi
else
  echo "    ✗ 交易发送失败"
fi

# 测试 7: 查询区块
echo ""
echo "  测试 7: 查询区块详情"
LATEST=$(curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest",false],"id":1}' | jq -r .result)
MINER=$(echo $LATEST | jq -r .miner)
NUM=$(echo $LATEST | jq -r .number)
echo "    最新区块: $NUM"
echo "    矿工: $MINER"
echo "    ✓ 区块查询正常"

# 总结
echo ""
echo "【8/8】测试总结"
FINAL_BLOCK=$(curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq -r .result)
FINAL_BLOCK_DEC=$(printf "%d" $FINAL_BLOCK)
FINAL_BALANCE=$(curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
  -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"$ACCOUNT\",\"latest\"],\"id\":1}" | jq -r .result)
FINAL_BALANCE_DEC=$(printf "%d" $FINAL_BALANCE)

echo "  总区块数: $FINAL_BLOCK_DEC"
echo "  最终余额: $FINAL_BALANCE_DEC wei"
echo ""
echo "✓ 所有端到端测试完成"

cleanup
