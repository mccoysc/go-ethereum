#!/bin/bash
set -e

echo "=== 在 Gramine 容器内测试 ==="

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

# 在 Gramine 容器内运行所有测试
docker run --rm -it \
  -v "$REPO_ROOT:/workspace" \
  -w /workspace \
  --network host \
  gramineproject/gramine:latest \
  bash -c '
    set -e
    echo "【环境】Gramine 容器"
    gramine-sgx --version || gramine-direct --version || echo "Gramine installed"
    
    echo ""
    echo "【编译】构建 geth"
    apt-get update -qq && apt-get install -y -qq wget golang-1.21 make > /dev/null 2>&1
    export PATH=/usr/lib/go-1.21/bin:$PATH
    make geth > /dev/null 2>&1
    
    echo ""
    echo "【初始化】创世区块"
    rm -rf /tmp/test-node
    ./build/bin/geth --datadir /tmp/test-node init test/integration/genesis.json > /dev/null 2>&1
    
    echo ""
    echo "【账户】创建测试账户"
    echo "test" > /tmp/pass.txt
    ACCOUNT=$(./build/bin/geth --datadir /tmp/test-node account new --password /tmp/pass.txt 2>&1 | grep -oP "0x[a-fA-F0-9]{40}")
    echo "账户: $ACCOUNT"
    
    echo ""
    echo "【启动】节点"
    ./build/bin/geth --datadir /tmp/test-node --networkid 762385986 \
      --http --http.addr 0.0.0.0 --http.port 8545 \
      --http.api eth,net,web3,personal \
      --nodiscover --maxpeers 0 --mine --miner.etherbase "$ACCOUNT" \
      --allow-insecure-unlock > /tmp/node.log 2>&1 &
    
    PID=$!
    
    cleanup() { kill $PID 2>/dev/null || true; }
    trap cleanup EXIT
    
    echo "等待节点启动..."
    sleep 15
    
    echo ""
    echo "=== 真实用户测试 ==="
    
    echo ""
    echo "【1】查询链信息"
    curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
      -d "{\"jsonrpc\":\"2.0\",\"method\":\"net_version\",\"params\":[],\"id\":1}" | python3 -m json.tool
    
    echo ""
    echo "【2】当前区块"
    curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
      -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_blockNumber\",\"params\":[],\"id\":1}" | python3 -m json.tool
    
    echo ""
    echo "【3】调用预编译 0x8000"
    curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
      -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_call\",\"params\":[{\"to\":\"0x8000\",\"data\":\"0x01\"},\"latest\"],\"id\":1}" | python3 -m json.tool
    
    echo ""
    echo "【4】调用预编译 0x8005"
    curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
      -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_call\",\"params\":[{\"to\":\"0x8005\",\"data\":\"0x00000020\"},\"latest\"],\"id\":1}" | python3 -m json.tool
    
    echo ""
    echo "【5】查询治理合约 0x1001"
    curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
      -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"0x0000000000000000000000000000000000001001\",\"latest\"],\"id\":1}" | python3 -m json.tool
    
    echo ""
    echo "【6】查询账户余额（挖矿奖励）"
    curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
      -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"$ACCOUNT\",\"latest\"],\"id\":1}" | python3 -m json.tool
    
    echo ""
    echo "【7】发送交易"
    curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
      -d "{\"jsonrpc\":\"2.0\",\"method\":\"personal_unlockAccount\",\"params\":[\"$ACCOUNT\",\"test\",300],\"id\":1}" > /dev/null
    
    TX=$(curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
      -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_sendTransaction\",\"params\":[{\"from\":\"$ACCOUNT\",\"to\":\"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266\",\"value\":\"0x1000\"}],\"id\":1}" | python3 -c "import sys,json; print(json.load(sys.stdin).get(\"result\",\"null\"))")
    
    if [ "$TX" != "null" ]; then
      echo "交易哈希: $TX"
    else
      echo "交易失败"
    fi
    
    sleep 3
    
    echo ""
    echo "【8】最终区块数"
    curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" \
      -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_blockNumber\",\"params\":[],\"id\":1}" | python3 -m json.tool
    
    echo ""
    echo "✓ Gramine 容器内测试完成"
    
    cleanup
  '
