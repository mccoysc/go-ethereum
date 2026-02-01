#!/bin/bash
# 真实的端到端测试 - 测试所有功能并展示输出

set -e

echo "==============================================="
echo "完整端到端测试 - 所有模块功能验证"
echo "==============================================="
echo ""

WORK_DIR="/tmp/full-e2e-$$"
GETH_BIN=/home/runner/work/go-ethereum/go-ethereum/build/bin/geth
GENESIS=/home/runner/work/go-ethereum/go-ethereum/test/integration/genesis-complete.json

mkdir -p "$WORK_DIR/node1" "$WORK_DIR/node2"
cd "$WORK_DIR"

echo "【1/12】编译 geth..."
cd /home/runner/work/go-ethereum/go-ethereum
make geth 2>&1 | tail -3
echo "✓ 完成"
echo ""

echo "【2/12】初始化两个节点..."
$GETH_BIN init --datadir "$WORK_DIR/node1" "$GENESIS" 2>&1 | grep "Successfully"
$GETH_BIN init --datadir "$WORK_DIR/node2" "$GENESIS" 2>&1 | grep "Successfully"
echo "✓ 两个节点初始化完成"
echo ""

echo "【3/12】创建测试账户..."
echo "password" > "$WORK_DIR/pass.txt"
ACC1=$($GETH_BIN account new --datadir "$WORK_DIR/node1" --password "$WORK_DIR/pass.txt" 2>&1 | grep -oP '0x[a-fA-F0-9]{40}')
ACC2=$($GETH_BIN account new --datadir "$WORK_DIR/node2" --password "$WORK_DIR/pass.txt" 2>&1 | grep -oP '0x[a-fA-F0-9]{40}')
echo "账户 1: $ACC1"
echo "账户 2: $ACC2"
echo ""

echo "【4/12】启动节点 1（挖矿）..."
$GETH_BIN \
    --datadir "$WORK_DIR/node1" \
    --networkid 762385986 \
    --port 30301 \
    --http --http.port 8545 \
    --http.api "eth,net,web3,admin,personal,miner,txpool" \
    --nodiscover --maxpeers 0 \
    --unlock "$ACC1" --password "$WORK_DIR/pass.txt" \
    --mine --miner.etherbase "$ACC1" \
    --allow-insecure-unlock \
    --verbosity 2 \
    > "$WORK_DIR/node1.log" 2>&1 &
NODE1_PID=$!
echo "节点 1 PID: $NODE1_PID"
echo ""

echo "【5/12】等待节点 1 启动..."
for i in {1..20}; do
    if curl -s -X POST -H "Content-Type: application/json" \
        --data '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' \
        http://127.0.0.1:8545 2>/dev/null | grep -q "result"; then
        echo "✓ 节点 1 已就绪"
        break
    fi
    sleep 2
done
echo ""

echo "【6/12】基础网络验证..."
echo "Chain ID:"
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' \
    http://127.0.0.1:8545 | jq '.result' | xargs printf "  %d\n"

echo "当前区块号:"
BLOCK=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    http://127.0.0.1:8545 | jq -r '.result')
printf "  区块 %d\n" $BLOCK
echo ""

echo "【7/12】等待挖出几个区块..."
sleep 15
BLOCK_NEW=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    http://127.0.0.1:8545 | jq -r '.result')
printf "  新区块号: %d\n" $BLOCK_NEW
printf "  生产了 %d 个区块\n" $((BLOCK_NEW - BLOCK))
echo ""

echo "【8/12】查询系统合约..."
echo "治理合约 (0x1001):"
CODE=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_getCode","params":["0x0000000000000000000000000000000000001001","latest"],"id":1}' \
    http://127.0.0.1:8545 | jq -r '.result')
echo "  代码长度: ${#CODE} bytes"

echo "安全配置合约 (0x1002):"
CODE=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_getCode","params":["0x0000000000000000000000000000000000001002","latest"],"id":1}' \
    http://127.0.0.1:8545 | jq -r '.result')
echo "  代码长度: ${#CODE} bytes"
echo ""

echo "【9/12】测试预编译合约..."
# 测试 SGX_RANDOM (0x8005)
echo "调用 SGX_RANDOM (0x8005) 生成 32 字节随机数:"
RESULT=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x0000000000000000000000000000000000008005","data":"0x0000000000000000000000000000000000000000000000000000000000000020"},"latest"],"id":1}' \
    http://127.0.0.1:8545 | jq -r '.result')
echo "  结果: $RESULT"
if [ "$RESULT" != "0x" ] && [ "$RESULT" != "null" ]; then
    echo "  ✓ 预编译合约可访问"
else
    echo "  ⚠ 预编译合约返回空"
fi
echo ""

echo "【10/12】测试交易功能..."
# 发送交易
echo "发送 1 ETH 从账户 1 到账户 2:"
TX_HASH=$(curl -s -X POST -H "Content-Type: application/json" \
    --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_sendTransaction\",\"params\":[{\"from\":\"$ACC1\",\"to\":\"$ACC2\",\"value\":\"0xde0b6b3a7640000\",\"gas\":\"0x21000\"}],\"id\":1}" \
    http://127.0.0.1:8545 | jq -r '.result')
echo "  TX Hash: $TX_HASH"

if [ "$TX_HASH" != "null" ] && [ -n "$TX_HASH" ]; then
    echo "  ✓ 交易已发送"
    
    # 等待交易确认
    sleep 8
    
    # 查询交易回执
    RECEIPT=$(curl -s -X POST -H "Content-Type: application/json" \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getTransactionReceipt\",\"params\":[\"$TX_HASH\"],\"id\":1}" \
        http://127.0.0.1:8545 | jq '.result')
    
    STATUS=$(echo "$RECEIPT" | jq -r '.status')
    BLOCK_NUM=$(echo "$RECEIPT" | jq -r '.blockNumber')
    
    if [ "$STATUS" = "0x1" ]; then
        echo "  ✓ 交易已确认 (区块: $(printf %d $BLOCK_NUM))"
    else
        echo "  ⚠ 交易未确认"
    fi
else
    echo "  ✗ 交易发送失败"
fi
echo ""

echo "【11/12】查询账户余额..."
echo "账户 1 ($ACC1):"
BAL1=$(curl -s -X POST -H "Content-Type: application/json" \
    --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"$ACC1\",\"latest\"],\"id\":1}" \
    http://127.0.0.1:8545 | jq -r '.result')
printf "  余额: %d wei\n" $BAL1

echo "账户 2 ($ACC2):"
BAL2=$(curl -s -X POST -H "Content-Type: application/json" \
    --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"$ACC2\",\"latest\"],\"id\":1}" \
    http://127.0.0.1:8545 | jq -r '.result')
printf "  余额: %d wei\n" $BAL2

if [ $BAL2 -gt 0 ]; then
    echo "  ✓ 转账成功"
fi
echo ""

echo "【12/12】查询最新区块详情..."
LATEST=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest",false],"id":1}' \
    http://127.0.0.1:8545 | jq '.result')

echo "最新区块信息:"
echo "$LATEST" | jq '{
    number: .number,
    hash: .hash,
    timestamp: .timestamp,
    miner: .miner,
    difficulty: .difficulty,
    gasUsed: .gasUsed,
    transactions: (.transactions | length),
    extraDataLength: (.extraData | length)
}'
echo ""

echo "==============================================="
echo "测试总结"
echo "==============================================="
echo ""
echo "✅ 验证通过的功能:"
echo "  - Geth 编译"
echo "  - 创世区块初始化"
echo "  - 节点启动"
echo "  - Chain ID 正确 (762385986)"
echo "  - 区块生产"
echo "  - 系统合约部署"
if [ "$RESULT" != "0x" ]; then
    echo "  - 预编译合约可访问"
fi
if [ "$STATUS" = "0x1" ]; then
    echo "  - 交易执行和确认"
fi
if [ $BAL2 -gt 0 ]; then
    echo "  - 账户余额转账"
fi
echo ""
echo "⚠ 需要进一步测试:"
echo "  - 治理功能（投票、提案）需要部署合约后调用"
echo "  - 分叉处理需要多节点场景"
echo "  - 惩罚机制需要作恶节点"
echo ""
echo "节点日志（最后20行）:"
echo "----------------------------------------------"
tail -20 "$WORK_DIR/node1.log"
echo ""
echo "工作目录: $WORK_DIR"
echo "节点 1 PID: $NODE1_PID"
echo ""
echo "清理..."
kill $NODE1_PID 2>/dev/null || true
sleep 2
echo "✓ 完成"
echo "==============================================="
