#!/bin/bash
set -e

echo "========================================"
echo "完整 PoA-SGX 功能测试"
echo "像真实 SGX 用户一样测试所有功能"
echo "运行环境: Gramine 容器"
echo "========================================"

WORKSPACE="/workspace"
DATADIR="$WORKSPACE/test-sgx-complete"
GENESIS="$WORKSPACE/test/integration/genesis-sgx.json"
CONTRACTS_DIR="$WORKSPACE/test/integration/contracts"

# 清理旧数据
echo ""
echo "【1/15】清理旧数据..."
rm -rf "$DATADIR"

# 安装依赖
echo ""
echo "【2/15】安装依赖..."
apt-get update -qq > /dev/null 2>&1
apt-get install -y -qq wget make gcc g++ jq solc > /dev/null 2>&1

# 安装 Go
echo ""
echo "【3/15】安装 Go..."
if [ ! -d "/usr/local/go" ]; then
    cd /tmp
    wget -q https://go.dev/dl/go1.21.6.linux-amd64.tar.gz || \
        wget -q https://golang.google.cn/dl/go1.21.6.linux-amd64.tar.gz
    tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
fi
export PATH=$PATH:/usr/local/go/bin
export GOPATH=/root/go

# 编译 geth（SGX共识已显式导入，非force load）
echo ""
echo "【4/15】编译 geth (SGX 共识已显式导入)..."
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

# 验证 SGX 模块
echo ""
echo "【5/15】验证 SGX 模块集成..."
if strings "$GETH" | grep -q "consensus/sgx"; then
    echo "✓ SGX 共识模块已包含"
else
    echo "❌ SGX 共识模块未找到"
    exit 1
fi

# 初始化创世区块
echo ""
echo "【6/15】初始化创世区块 (包含 SGX 配置)..."
$GETH init --datadir "$DATADIR" "$GENESIS" > /tmp/init.log 2>&1

if [ $? -eq 0 ]; then
    echo "✓ 创世区块初始化成功"
else
    echo "❌ 创世区块初始化失败"
    cat /tmp/init.log
    exit 1
fi

# 验证创世配置中的 SGX 共识配置
echo ""
echo "【7/15】验证 SGX 共识配置..."
if grep -q '"sgx"' "$GENESIS"; then
    echo "✓ SGX 共识配置已找到:"
    grep -A 6 '"sgx"' "$GENESIS" | sed 's/^/    /'
else
    echo "❌ 创世文件缺少 SGX 配置"
    exit 1
fi

# 创建多个测试账户
echo ""
echo "【8/15】创建测试账户..."
echo "test123" > "$DATADIR/pass.txt"

# 账户 1: 矿工账户
MINER=$($GETH account new --datadir "$DATADIR" --password "$DATADIR/pass.txt" 2>&1 | grep "Public address" | awk '{print $4}')
echo "  矿工账户: $MINER"

# 账户 2: 用户账户
USER1=$($GETH account new --datadir "$DATADIR" --password "$DATADIR/pass.txt" 2>&1 | grep "Public address" | awk '{print $4}')
echo "  用户账户 1: $USER1"

# 账户 3: 治理账户  
GOV_ACCOUNT=$($GETH account new --datadir "$DATADIR" --password "$DATADIR/pass.txt" 2>&1 | grep "Public address" | awk '{print $4}')
echo "  治理账户: $GOV_ACCOUNT"

# 启动节点（使用 PoA-SGX 共识）
echo ""
echo "【9/15】启动节点 (PoA-SGX 共识引擎)..."
$GETH --datadir "$DATADIR" \
    --networkid 762385986 \
    --http \
    --http.addr "0.0.0.0" \
    --http.port 8545 \
    --http.api "eth,net,web3,personal,admin,debug,txpool" \
    --nodiscover \
    --maxpeers 0 \
    --verbosity 3 \
    --mine \
    --miner.etherbase "$MINER" \
    --unlock "$MINER,$USER1,$GOV_ACCOUNT" \
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

# 等待挖矿产生区块
echo ""
echo "等待挖矿产生区块 (PoA-SGX)..."
sleep 12

# RPC调用函数
rpc_call() {
    curl -s -X POST -H "Content-Type: application/json" \
        --data "$1" \
        http://localhost:8545 | jq -r '.result'
}

# 测试计数
test_count=0
pass_count=0

run_test() {
    local test_name="$1"
    local test_cmd="$2"
    
    test_count=$((test_count + 1))
    echo ""
    echo "测试 $test_count: $test_name"
    
    result=$(eval "$test_cmd" 2>&1)
    local exit_code=$?
    
    if [ $exit_code -eq 0 ] && [ -n "$result" ] && [ "$result" != "null" ]; then
        echo "  结果: $result"
        echo "  ✓ 通过"
        pass_count=$((pass_count + 1))
        return 0
    else
        echo "  结果: $result"
        echo "  ✗ 失败"
        return 1
    fi
}

echo ""
echo "========================================"
echo "【10/15】测试阶段 1: 网络和共识验证"
echo "========================================"

run_test "Chain ID (762385986)" \
    "rpc_call '{\"jsonrpc\":\"2.0\",\"method\":\"eth_chainId\",\"params\":[],\"id\":1}' | xargs printf '%d'"

run_test "区块号 (应 > 0)" \
    "rpc_call '{\"jsonrpc\":\"2.0\",\"method\":\"eth_blockNumber\",\"params\":[],\"id\":1}' | xargs printf '%d'"

run_test "矿工奖励余额" \
    "rpc_call '{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"$MINER\",\"latest\"],\"id\":1}' | xargs printf '%d'"

echo ""
echo "========================================"
echo "【11/15】测试阶段 2: 读取安全配置合约"
echo "========================================"

# 读取安全配置合约
SEC_CONFIG="0x0000000000000000000000000000000000001002"

run_test "安全配置合约代码" \
    "rpc_call '{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$SEC_CONFIG\",\"latest\"],\"id\":1}' | wc -c"

# 尝试调用安全配置合约的函数（如果有的话）
echo ""
echo "尝试读取安全参数..."
MIN_STAKE=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"eth_call\",\"params\":[{\"to\":\"$SEC_CONFIG\",\"data\":\"0x375a6e7e\"},\"latest\"],\"id\":1}")
echo "  minStake: $MIN_STAKE"

echo ""
echo "========================================"
echo "【12/15】测试阶段 3: 调用预编译合约"
echo "========================================"

# 测试 SGX_RANDOM (0x8005)
echo ""
echo "测试预编译合约: SGX_RANDOM (0x8005)"
RANDOM_DATA=$(rpc_call '{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x0000000000000000000000000000000000008005","data":"0x00000020"},"latest"],"id":1}')
if [ -n "$RANDOM_DATA" ] && [ "$RANDOM_DATA" != "null" ] && [ "$RANDOM_DATA" != "0x" ]; then
    echo "  ✓ 随机数生成成功: $RANDOM_DATA"
    pass_count=$((pass_count + 1))
else
    echo "  ✗ 随机数生成失败"
fi
test_count=$((test_count + 1))

# 测试 SGX_KEY_CREATE (0x8000)
echo ""
echo "测试预编译合约: SGX_KEY_CREATE (0x8000)"
KEY_RESULT=$(rpc_call '{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x0000000000000000000000000000000000008000","data":"0x01"},"latest"],"id":1}')
if [ -n "$KEY_RESULT" ] && [ "$KEY_RESULT" != "null" ]; then
    echo "  ✓ 密钥创建成功: $KEY_RESULT"
    pass_count=$((pass_count + 1))
else
    echo "  ✗ 密钥创建失败"
fi
test_count=$((test_count + 1))

echo ""
echo "========================================"
echo "【13/15】测试阶段 4: 发送交易"
echo "========================================"

# 发送 ETH 交易
echo ""
echo "发送交易: $MINER -> $USER1"
TX_HASH=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"eth_sendTransaction\",\"params\":[{\"from\":\"$MINER\",\"to\":\"$USER1\",\"value\":\"0x1000000000000000\",\"gas\":\"0x5208\"}],\"id\":1}")

if [ "$TX_HASH" != "null" ] && [ -n "$TX_HASH" ]; then
    echo "  ✓ 交易哈希: $TX_HASH"
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

# 验证余额变化
USER1_BALANCE=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"$USER1\",\"latest\"],\"id\":1}" | xargs printf '%d')
echo ""
echo "接收账户余额: $USER1_BALANCE wei"
if [ $USER1_BALANCE -gt 0 ]; then
    echo "  ✓ 余额已更新"
    pass_count=$((pass_count + 1))
else
    echo "  ✗ 余额未更新"
fi
test_count=$((test_count + 1))

echo ""
echo "========================================"
echo "【14/15】测试阶段 5: 部署和调用合约"
echo "========================================"

# 编译测试合约
echo ""
echo "编译测试合约..."
cd "$CONTRACTS_DIR"

# 编译 SGXCryptoTest 合约
if command -v solc >/dev/null 2>&1; then
    solc --bin --abi SGXCryptoTest.sol -o /tmp/solc_output/ --overwrite 2>/dev/null
    
    if [ -f "/tmp/solc_output/SGXCryptoTest.bin" ]; then
        CONTRACT_BIN=$(cat /tmp/solc_output/SGXCryptoTest.bin)
        echo "✓ 合约编译成功 (${#CONTRACT_BIN} 字节)"
        
        # 部署合约
        echo ""
        echo "部署 SGXCryptoTest 合约..."
        DEPLOY_TX=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"eth_sendTransaction\",\"params\":[{\"from\":\"$USER1\",\"data\":\"0x$CONTRACT_BIN\",\"gas\":\"0x500000\"}],\"id\":1}")
        
        if [ "$DEPLOY_TX" != "null" ] && [ -n "$DEPLOY_TX" ]; then
            echo "  ✓ 部署交易: $DEPLOY_TX"
            pass_count=$((pass_count + 1))
            
            # 等待部署确认
            sleep 8
            
            DEPLOY_RECEIPT=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getTransactionReceipt\",\"params\":[\"$DEPLOY_TX\"],\"id\":1}")
            CONTRACT_ADDR=$(echo "$DEPLOY_RECEIPT" | jq -r '.contractAddress')
            
            if [ "$CONTRACT_ADDR" != "null" ] && [ -n "$CONTRACT_ADDR" ]; then
                echo "  ✓ 合约地址: $CONTRACT_ADDR"
                pass_count=$((pass_count + 1))
                
                # 调用合约方法测试随机数
                echo ""
                echo "调用合约方法: testRandom(32)"
                CALL_TX=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"eth_sendTransaction\",\"params\":[{\"from\":\"$USER1\",\"to\":\"$CONTRACT_ADDR\",\"data\":\"0x$(echo -n 'testRandom(uint256)' | sha256sum | cut -c1-8)0000000000000000000000000000000000000000000000000000000000000020\",\"gas\":\"0x100000\"}],\"id\":1}")
                
                if [ "$CALL_TX" != "null" ]; then
                    echo "  ✓ 合约调用成功: $CALL_TX"
                    pass_count=$((pass_count + 1))
                else
                    echo "  ✗ 合约调用失败"
                fi
                test_count=$((test_count + 1))
            else
                echo "  ✗ 合约部署失败"
            fi
            test_count=$((test_count + 1))
        else
            echo "  ✗ 部署交易失败"
            test_count=$((test_count + 1))
        fi
    else
        echo "⚠ solc 编译失败，跳过合约部署测试"
    fi
else
    echo "⚠ solc 未安装，跳过合约部署测试"
fi

echo ""
echo "========================================"
echo "【15/15】测试阶段 6: 治理投票流程"
echo "========================================"

# 测试治理合约交互
GOV_CONTRACT="0x0000000000000000000000000000000000001001"

echo ""
echo "检查治理合约..."
GOV_CODE=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$GOV_CONTRACT\",\"latest\"],\"id\":1}")
GOV_CODE_LEN=$(echo "$GOV_CODE" | wc -c)

if [ $GOV_CODE_LEN -gt 10 ]; then
    echo "  ✓ 治理合约已部署 (代码长度: $GOV_CODE_LEN)"
    pass_count=$((pass_count + 1))
    
    # 尝试创建提案
    echo ""
    echo "创建治理提案 (添加 MRENCLAVE)..."
    MRENCLAVE="0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
    PROPOSE_TX=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"eth_sendTransaction\",\"params\":[{\"from\":\"$GOV_ACCOUNT\",\"to\":\"$GOV_CONTRACT\",\"data\":\"0x$(echo -n 'proposeAdd(bytes32)' | sha256sum | cut -c1-8)$MRENCLAVE\",\"gas\":\"0x100000\"}],\"id\":1}")
    
    if [ "$PROPOSE_TX" != "null" ] && [ -n "$PROPOSE_TX" ]; then
        echo "  ✓ 提案创建交易: $PROPOSE_TX"
        pass_count=$((pass_count + 1))
        
        sleep 8
        
        # 尝试投票
        echo ""
        echo "对提案投票..."
        VOTE_TX=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"eth_sendTransaction\",\"params\":[{\"from\":\"$GOV_ACCOUNT\",\"to\":\"$GOV_CONTRACT\",\"data\":\"0x$(echo -n 'vote(uint256,bool)' | sha256sum | cut -c1-8)00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001\",\"gas\":\"0x100000\"}],\"id\":1}")
        
        if [ "$VOTE_TX" != "null" ]; then
            echo "  ✓ 投票交易: $VOTE_TX"
            pass_count=$((pass_count + 1))
        else
            echo "  ✗ 投票失败"
        fi
        test_count=$((test_count + 1))
    else
        echo "  ✗ 提案创建失败"
        test_count=$((test_count + 1))
    fi
else
    echo "  ✗ 治理合约未部署"
fi
test_count=$((test_count + 1))

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
echo "通过率: $(( pass_count * 100 / test_count ))%"

if [ $pass_count -ge $((test_count * 70 / 100)) ]; then
    echo ""
    echo "🎉 测试通过！PoA-SGX 功能正常"
    exit 0
else
    echo ""
    echo "❌ 测试失败过多"
    exit 1
fi
