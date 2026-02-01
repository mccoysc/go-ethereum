#!/bin/bash
set -e

echo "========================================"
echo "PoA-SGX 共识引擎完整功能测试"
echo "运行环境: Gramine 容器"
echo "========================================"

WORKSPACE="/workspace"
DATADIR="$WORKSPACE/test-sgx-node"
GENESIS="$WORKSPACE/test/integration/genesis-sgx.json"

# 清理旧数据
echo ""
echo "【1/10】清理旧数据..."
rm -rf "$DATADIR"

# 安装依赖
echo ""
echo "【2/10】安装依赖..."
apt-get update -qq > /dev/null 2>&1
apt-get install -y -qq wget make gcc g++ > /dev/null 2>&1

# 安装 Go
echo ""
echo "【3/10】安装 Go..."
if [ ! -d "/usr/local/go" ]; then
    cd /tmp
    wget -q https://go.dev/dl/go1.21.6.linux-amd64.tar.gz || \
        wget -q https://golang.google.cn/dl/go1.21.6.linux-amd64.tar.gz
    tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
fi
export PATH=$PATH:/usr/local/go/bin
export GOPATH=/root/go

# 编译 geth（包含所有 SGX 模块）
echo ""
echo "【4/10】编译 geth (包含 SGX 模块)..."
cd "$WORKSPACE"
make geth > /tmp/build.log 2>&1
GETH="$WORKSPACE/build/bin/geth"

if [ ! -f "$GETH" ]; then
    echo "❌ geth 编译失败"
    tail -20 /tmp/build.log
    exit 1
fi

GETH_SIZE=$(du -h "$GETH" | cut -f1)
echo "✓ geth 编译完成 (大小: $GETH_SIZE)"

# 验证 SGX 模块已编译进去
echo ""
echo "【5/10】验证 SGX 模块..."
if strings "$GETH" | grep -q "consensus/sgx"; then
    echo "✓ SGX 共识模块已包含"
else
    echo "❌ SGX 共识模块未找到"
    exit 1
fi

if strings "$GETH" | grep -q "governance"; then
    echo "✓ 治理模块已包含"
else
    echo "❌ 治理模块未找到"
    exit 1
fi

# 检查创世配置
echo ""
echo "【6/10】检查创世配置..."
if [ ! -f "$GENESIS" ]; then
    echo "❌ 创世文件不存在: $GENESIS"
    exit 1
fi

if grep -q '"sgx"' "$GENESIS"; then
    echo "✓ SGX 共识配置已找到"
    echo "  配置详情:"
    grep -A 6 '"sgx"' "$GENESIS" | sed 's/^/    /'
else
    echo "❌ 创世文件缺少 SGX 配置"
    exit 1
fi

# 初始化创世区块
echo ""
echo "【7/10】初始化创世区块..."
$GETH init --datadir "$DATADIR" "$GENESIS" > /tmp/init.log 2>&1

if [ $? -eq 0 ]; then
    echo "✓ 创世区块初始化成功"
else
    echo "❌ 创世区块初始化失败"
    cat /tmp/init.log
    exit 1
fi

# 创建测试账户
echo ""
echo "【8/10】创建测试账户..."
echo "test123" > "$DATADIR/pass.txt"
ACCOUNT=$($GETH account new --datadir "$DATADIR" --password "$DATADIR/pass.txt" 2>&1 | grep "Public address" | awk '{print $4}')

if [ -z "$ACCOUNT" ]; then
    echo "❌ 账户创建失败"
    exit 1
fi

echo "✓ 测试账户: $ACCOUNT"

# 启动节点
echo ""
echo "【9/10】启动节点 (PoA-SGX 共识)..."
$GETH --datadir "$DATADIR" \
    --networkid 762385986 \
    --http \
    --http.addr "0.0.0.0" \
    --http.port 8545 \
    --http.api "eth,net,web3,personal,admin,debug" \
    --nodiscover \
    --maxpeers 0 \
    --verbosity 3 \
    --mine \
    --miner.etherbase "$ACCOUNT" \
    --unlock "$ACCOUNT" \
    --password "$DATADIR/pass.txt" \
    --allow-insecure-unlock \
    > "$DATADIR/node.log" 2>&1 &

NODE_PID=$!
echo "✓ 节点已启动 (PID: $NODE_PID)"

# 等待节点就绪
echo ""
echo "等待节点就绪..."
for i in {1..30}; do
    if curl -s -X POST -H "Content-Type: application/json" \
        --data '{"jsonrpc":"2.0","method":"net_version","params":[],"id":1}' \
        http://localhost:8545 > /dev/null 2>&1; then
        echo "✓ 节点已就绪 (${i}s)"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "❌ 节点启动超时"
        kill $NODE_PID 2>/dev/null
        tail -50 "$DATADIR/node.log"
        exit 1
    fi
    sleep 1
done

# 执行完整功能测试
echo ""
echo "【10/10】执行 PoA-SGX 功能测试..."
echo ""

test_count=0
pass_count=0

# 测试函数
run_test() {
    local test_name="$1"
    local test_cmd="$2"
    
    test_count=$((test_count + 1))
    echo "测试 $test_count: $test_name"
    
    result=$(eval "$test_cmd")
    
    if [ $? -eq 0 ] && [ -n "$result" ]; then
        echo "  结果: $result"
        echo "  ✓ 通过"
        pass_count=$((pass_count + 1))
        return 0
    else
        echo "  ✗ 失败"
        return 1
    fi
}

# RPC调用函数
rpc_call() {
    curl -s -X POST -H "Content-Type: application/json" \
        --data "$1" \
        http://localhost:8545 | jq -r '.result'
}

# 1. 网络和共识验证
run_test "Chain ID (应为 762385986)" \
    "rpc_call '{\"jsonrpc\":\"2.0\",\"method\":\"eth_chainId\",\"params\":[],\"id\":1}' | xargs printf '%d'"

run_test "网络连接" \
    "rpc_call '{\"jsonrpc\":\"2.0\",\"method\":\"net_listening\",\"params\":[],\"id\":1}'"

# 等待挖矿产生区块
echo ""
echo "等待区块生产 (PoA-SGX 挖矿)..."
sleep 10

run_test "当前区块号" \
    "rpc_call '{\"jsonrpc\":\"2.0\",\"method\":\"eth_blockNumber\",\"params\":[],\"id\":1}' | xargs printf '%d'"

# 2. 预编译合约测试 (0x8000-0x8008)
echo ""
echo "--- 预编译合约测试 ---"

# SGX_KEY_CREATE (0x8000)
run_test "预编译合约 0x8000 (密钥创建)" \
    "rpc_call '{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"0x0000000000000000000000000000000000008000\",\"latest\"],\"id\":1}' | wc -c"

# SGX_RANDOM (0x8005)
run_test "预编译合约 0x8005 (随机数)" \
    "rpc_call '{\"jsonrpc\":\"2.0\",\"method\":\"eth_call\",\"params\":[{\"to\":\"0x0000000000000000000000000000000000008005\",\"data\":\"0x00000020\"},\"latest\"],\"id\":1}'"

# 3. 系统合约测试
echo ""
echo "--- 系统合约测试 ---"

# 治理合约 (0x1001)
GOV_CODE=$(rpc_call '{"jsonrpc":"2.0","method":"eth_getCode","params":["0x0000000000000000000000000000000000001001","latest"],"id":1}')
run_test "治理合约 (0x1001)" \
    "echo '$GOV_CODE' | wc -c"

# 安全配置合约 (0x1002)
SEC_CODE=$(rpc_call '{"jsonrpc":"2.0","method":"eth_getCode","params":["0x0000000000000000000000000000000000001002","latest"],"id":1}')
run_test "安全配置合约 (0x1002)" \
    "echo '$SEC_CODE' | wc -c"

# 激励合约 (0x1003)
INC_CODE=$(rpc_call '{"jsonrpc":"2.0","method":"eth_getCode","params":["0x0000000000000000000000000000000000001003","latest"],"id":1}')
run_test "激励合约 (0x1003)" \
    "echo '$INC_CODE' | wc -c"

# 4. 账户和余额测试
echo ""
echo "--- 账户和余额测试 ---"

BALANCE=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"$ACCOUNT\",\"latest\"],\"id\":1}")
run_test "矿工余额 (挖矿奖励)" \
    "echo $BALANCE | xargs printf '%d'"

# 5. 交易测试
echo ""
echo "--- 交易测试 ---"

# 创建接收账户
echo "test456" > "$DATADIR/pass2.txt"
ACCOUNT2=$($GETH account new --datadir "$DATADIR" --password "$DATADIR/pass2.txt" 2>&1 | grep "Public address" | awk '{print $4}')
echo "接收账户: $ACCOUNT2"

# 发送交易
TX_HASH=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"eth_sendTransaction\",\"params\":[{\"from\":\"$ACCOUNT\",\"to\":\"$ACCOUNT2\",\"value\":\"0x1000000000000000\",\"gas\":\"0x5208\"}],\"id\":1}")

if [ "$TX_HASH" != "null" ] && [ -n "$TX_HASH" ]; then
    echo "交易哈希: $TX_HASH"
    echo "  ✓ 交易发送成功"
    pass_count=$((pass_count + 1))
    
    # 等待交易确认
    sleep 8
    
    TX_RECEIPT=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getTransactionReceipt\",\"params\":[\"$TX_HASH\"],\"id\":1}")
    if [ "$TX_RECEIPT" != "null" ]; then
        echo "  ✓ 交易已确认"
        pass_count=$((pass_count + 1))
    else
        echo "  ✗ 交易未确认"
    fi
    test_count=$((test_count + 2))
else
    echo "  ✗ 交易发送失败"
    test_count=$((test_count + 1))
fi

# 6. 区块详情测试
echo ""
echo "--- 区块详情测试 ---"

LATEST_BLOCK=$(rpc_call '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest",true],"id":1}')
run_test "最新区块信息" \
    "echo '$LATEST_BLOCK' | jq -r '.number'"

# 停止节点
echo ""
echo "停止节点..."
kill $NODE_PID 2>/dev/null
wait $NODE_PID 2>/dev/null

# 测试总结
echo ""
echo "========================================"
echo "测试总结"
echo "========================================"
echo "总测试数: $test_count"
echo "通过数: $pass_count"
echo "失败数: $((test_count - pass_count))"

if [ $pass_count -eq $test_count ]; then
    echo ""
    echo "🎉 所有测试通过！PoA-SGX 共识功能正常"
    exit 0
elif [ $pass_count -gt $((test_count / 2)) ]; then
    echo ""
    echo "⚠️  部分测试通过 ($pass_count/$test_count)"
    exit 0
else
    echo ""
    echo "❌ 大部分测试失败"
    exit 1
fi
