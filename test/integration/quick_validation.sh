#!/bin/bash
echo "=== 快速验证测试 ==="

DATADIR=/tmp/test-quick
rm -rf $DATADIR

echo "1. 初始化创世块..."
./build/bin/geth --datadir $DATADIR init test/integration/genesis-complete.json 2>&1 | grep -i "Successfully\|genesis"

echo ""
echo "2. 创建测试账户..."
echo "test123" > /tmp/pass.txt
ACCOUNT=$(./build/bin/geth --datadir $DATADIR account new --password /tmp/pass.txt 2>&1 | grep -oP '0x[a-fA-F0-9]{40}' | head -1)
echo "账户: $ACCOUNT"

echo ""
echo "3. 启动节点（后台）..."
./build/bin/geth --datadir $DATADIR \
  --networkid 762385986 \
  --http \
  --http.addr 127.0.0.1 \
  --http.port 8545 \
  --http.api "eth,net,web3" \
  --nodiscover \
  --maxpeers 0 \
  > /tmp/geth-test.log 2>&1 &
GETH_PID=$!
echo "Geth PID: $GETH_PID"

echo ""
echo "4. 等待节点就绪..."
sleep 5

echo ""
echo "5. 测试 RPC 调用..."
CHAIN_ID=$(curl -s -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' \
  http://127.0.0.1:8545 | grep -oP '"result":"0x[^"]+' | cut -d'"' -f4)

if [ -n "$CHAIN_ID" ]; then
    CHAIN_ID_DEC=$((CHAIN_ID))
    echo "✓ Chain ID: $CHAIN_ID_DEC"
    if [ "$CHAIN_ID_DEC" -eq 762385986 ]; then
        echo "✓ Chain ID 正确！"
    else
        echo "✗ Chain ID 错误"
    fi
else
    echo "✗ RPC 调用失败"
fi

echo ""
echo "6. 检查系统合约..."
GOV_CODE=$(curl -s -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_getCode","params":["0x0000000000000000000000000000000000001001","latest"],"id":1}' \
  http://127.0.0.1:8545 | grep -oP '"result":"0x[^"]+' | cut -d'"' -f4)

if [ "${#GOV_CODE}" -gt 10 ]; then
    echo "✓ 治理合约 (0x1001) 已部署，代码长度: ${#GOV_CODE}"
else
    echo "✗ 治理合约未部署"
fi

echo ""
echo "7. 清理..."
kill $GETH_PID 2>/dev/null
wait $GETH_PID 2>/dev/null
rm -rf $DATADIR /tmp/pass.txt

echo ""
echo "=== 测试完成 ==="
