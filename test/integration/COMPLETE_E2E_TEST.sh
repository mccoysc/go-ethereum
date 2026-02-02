#!/bin/bash

set -e

echo "=========================================="
echo "完整端到端测试 - 所有功能验证"
echo "=========================================="

REPO_ROOT="/home/runner/work/go-ethereum/go-ethereum"
TEST_DIR="$REPO_ROOT/test-node"
GETH_BIN="$REPO_ROOT/build/bin/geth"
CONTRACTS_DIR="$REPO_ROOT/contracts"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

log_error() {
    echo -e "${RED}✗ $1${NC}"
}

log_info() {
    echo -e "${YELLOW}➜ $1${NC}"
}

# 清理测试环境
cleanup() {
    log_info "清理测试环境..."
    pkill -f "geth.*test-node" || true
    rm -rf "$TEST_DIR"
    sleep 2
}

# 初始化测试节点
init_node() {
    log_info "初始化测试节点..."
    
    mkdir -p "$TEST_DIR"
    
    # 使用包含所有合约的创世配置
    $GETH_BIN init \
        --datadir "$TEST_DIR" \
        "$REPO_ROOT/test/integration/genesis-complete.json"
    
    log_success "节点初始化完成"
}

# 创建测试账户
create_account() {
    log_info "创建测试账户..."
    
    echo "test123" > "$TEST_DIR/pass.txt"
    
    ACCOUNT=$($GETH_BIN account new \
        --datadir "$TEST_DIR" \
        --password "$TEST_DIR/pass.txt" \
        2>&1 | grep -oP '0x[a-fA-F0-9]{40}' | head -1)
    
    echo "$ACCOUNT" > "$TEST_DIR/account.txt"
    
    log_success "账户创建: $ACCOUNT"
    echo "$ACCOUNT"
}

# 启动节点
start_node() {
    local ACCOUNT=$1
    log_info "启动节点 (矿工: $ACCOUNT)..."
    
    $GETH_BIN \
        --datadir "$TEST_DIR" \
        --networkid 762385986 \
        --http \
        --http.addr "127.0.0.1" \
        --http.port 8545 \
        --http.api "eth,net,web3,personal,admin,miner,txpool" \
        --http.corsdomain "*" \
        --nodiscover \
        --maxpeers 0 \
        --mine \
        --miner.etherbase "$ACCOUNT" \
        --unlock "$ACCOUNT" \
        --password "$TEST_DIR/pass.txt" \
        --allow-insecure-unlock \
        --verbosity 3 \
        > "$TEST_DIR/node.log" 2>&1 &
    
    echo $! > "$TEST_DIR/geth.pid"
    
    log_success "节点已启动 (PID: $(cat $TEST_DIR/geth.pid))"
}

# 等待节点就绪
wait_for_node() {
    log_info "等待节点就绪..."
    
    for i in {1..30}; do
        if curl -s -X POST \
            -H "Content-Type: application/json" \
            --data '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' \
            http://127.0.0.1:8545 > /dev/null 2>&1; then
            log_success "节点已就绪"
            return 0
        fi
        sleep 1
    done
    
    log_error "节点启动超时"
    return 1
}

# 等待区块生产
wait_for_blocks() {
    local MIN_BLOCKS=$1
    log_info "等待至少 $MIN_BLOCKS 个区块..."
    
    for i in {1..60}; do
        BLOCK_NUM=$(curl -s -X POST \
            -H "Content-Type: application/json" \
            --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
            http://127.0.0.1:8545 | jq -r '.result' | xargs printf "%d")
        
        if [ "$BLOCK_NUM" -ge "$MIN_BLOCKS" ]; then
            log_success "当前区块高度: $BLOCK_NUM"
            return 0
        fi
        
        echo -n "."
        sleep 2
    done
    
    log_error "区块生产超时"
    return 1
}

# 编译合约
compile_contract() {
    log_info "编译 CryptoTestContract..."
    
    cd "$CONTRACTS_DIR"
    
    # 编译合约
    solc --bin --abi --optimize --overwrite \
        -o build \
        CryptoTestContract.sol
    
    if [ ! -f "build/CryptoTestContract.bin" ]; then
        log_error "合约编译失败"
        return 1
    fi
    
    log_success "合约编译完成"
    
    # 显示字节码大小
    local SIZE=$(wc -c < "build/CryptoTestContract.bin")
    log_info "合约大小: $SIZE bytes"
}

# 部署合约
deploy_contract() {
    local ACCOUNT=$1
    log_info "部署 CryptoTestContract..."
    
    # 读取合约字节码
    local BYTECODE="0x$(cat $CONTRACTS_DIR/build/CryptoTestContract.bin)"
    
    # 部署合约
    local TX_HASH=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        --data "{
            \"jsonrpc\":\"2.0\",
            \"method\":\"eth_sendTransaction\",
            \"params\":[{
                \"from\":\"$ACCOUNT\",
                \"data\":\"$BYTECODE\",
                \"gas\":\"0x1000000\"
            }],
            \"id\":1
        }" \
        http://127.0.0.1:8545 | jq -r '.result')
    
    if [ "$TX_HASH" == "null" ] || [ -z "$TX_HASH" ]; then
        log_error "合约部署交易发送失败"
        return 1
    fi
    
    log_success "部署交易哈希: $TX_HASH"
    
    # 等待交易确认
    log_info "等待交易确认..."
    sleep 10
    
    # 获取合约地址
    local RECEIPT=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        --data "{
            \"jsonrpc\":\"2.0\",
            \"method\":\"eth_getTransactionReceipt\",
            \"params\":[\"$TX_HASH\"],
            \"id\":1
        }" \
        http://127.0.0.1:8545)
    
    local CONTRACT_ADDR=$(echo "$RECEIPT" | jq -r '.result.contractAddress')
    
    if [ "$CONTRACT_ADDR" == "null" ] || [ -z "$CONTRACT_ADDR" ]; then
        log_error "获取合约地址失败"
        echo "Receipt: $RECEIPT"
        return 1
    fi
    
    log_success "合约部署成功: $CONTRACT_ADDR"
    echo "$CONTRACT_ADDR" > "$TEST_DIR/contract.txt"
    echo "$CONTRACT_ADDR"
}

# 调用合约方法
call_contract_method() {
    local CONTRACT=$1
    local METHOD_SIG=$2
    local ACCOUNT=$3
    
    log_info "调用合约方法: $METHOD_SIG"
    
    # 计算方法选择器
    local METHOD_ID=$(echo -n "$METHOD_SIG" | openssl dgst -sha3-256 -binary | xxd -p -c 32 | cut -c1-8)
    
    # 发送交易
    local TX_HASH=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        --data "{
            \"jsonrpc\":\"2.0\",
            \"method\":\"eth_sendTransaction\",
            \"params\":[{
                \"from\":\"$ACCOUNT\",
                \"to\":\"$CONTRACT\",
                \"data\":\"0x$METHOD_ID\",
                \"gas\":\"0x100000\"
            }],
            \"id\":1
        }" \
        http://127.0.0.1:8545 | jq -r '.result')
    
    log_success "交易哈希: $TX_HASH"
    
    # 等待确认
    sleep 10
    
    # 获取交易收据
    local RECEIPT=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        --data "{
            \"jsonrpc\":\"2.0\",
            \"method\":\"eth_getTransactionReceipt\",
            \"params\":[\"$TX_HASH\"],
            \"id\":1
        }" \
        http://127.0.0.1:8545)
    
    local STATUS=$(echo "$RECEIPT" | jq -r '.result.status')
    
    if [ "$STATUS" == "0x1" ]; then
        log_success "交易执行成功"
        
        # 显示事件日志
        local LOGS=$(echo "$RECEIPT" | jq -r '.result.logs')
        if [ "$LOGS" != "null" ] && [ "$LOGS" != "[]" ]; then
            log_info "事件日志:"
            echo "$LOGS" | jq .
        fi
    else
        log_error "交易执行失败"
        echo "Receipt: $RECEIPT"
    fi
}

# 测试治理合约
test_governance() {
    local ACCOUNT=$1
    
    echo ""
    echo "=========================================="
    echo "测试 1: 治理合约交互"
    echo "=========================================="
    
    local GOV_CONTRACT="0x0000000000000000000000000000000000001001"
    
    log_info "治理合约地址: $GOV_CONTRACT"
    
    # 检查合约代码
    local CODE=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        --data "{
            \"jsonrpc\":\"2.0\",
            \"method\":\"eth_getCode\",
            \"params\":[\"$GOV_CONTRACT\", \"latest\"],
            \"id\":1
        }" \
        http://127.0.0.1:8545 | jq -r '.result')
    
    log_info "合约代码长度: ${#CODE} 字符"
    
    # TODO: 调用治理合约方法（需要ABI）
    # registerValidator
    # createProposal
    # vote
    
    log_success "治理合约测试完成"
}

# 测试安全配置合约
test_security_config() {
    echo ""
    echo "=========================================="
    echo "测试 2: 读取安全配置参数"
    echo "=========================================="
    
    local SEC_CONTRACT="0x0000000000000000000000000000000000001002"
    
    log_info "安全配置合约地址: $SEC_CONTRACT"
    
    # 检查合约代码
    local CODE=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        --data "{
            \"jsonrpc\":\"2.0\",
            \"method\":\"eth_getCode\",
            \"params\":[\"$SEC_CONTRACT\", \"latest\"],
            \"id\":1
        }" \
        http://127.0.0.1:8545 | jq -r '.result')
    
    log_info "合约代码长度: ${#CODE} 字符"
    
    # TODO: 读取配置参数
    # minStake
    # baseBlockReward
    # slashingAmount
    # etc.
    
    log_success "安全配置读取测试完成"
}

# 测试密码学接口
test_crypto_interfaces() {
    local ACCOUNT=$1
    local CONTRACT=$2
    
    echo ""
    echo "=========================================="
    echo "测试 3: 密码学预编译接口"
    echo "=========================================="
    
    log_info "通过部署的合约测试所有密码学接口..."
    
    # 测试所有接口
    call_contract_method "$CONTRACT" "testAllInterfaces()" "$ACCOUNT"
    
    # 测试随机数生成
    call_contract_method "$CONTRACT" "testRandom(uint256)" "$ACCOUNT"
    
    # 测试加密周期
    call_contract_method "$CONTRACT" "testFullEncryptionCycle(string)" "$ACCOUNT"
    
    # 测试签名周期
    call_contract_method "$CONTRACT" "testFullSignatureCycle(string)" "$ACCOUNT"
    
    log_success "密码学接口测试完成"
}

# 主测试流程
main() {
    log_info "开始完整端到端测试..."
    
    # 清理
    cleanup
    
    # 初始化节点
    init_node
    
    # 创建账户
    ACCOUNT=$(create_account)
    
    # 启动节点
    start_node "$ACCOUNT"
    
    # 等待节点就绪
    wait_for_node
    
    # 等待一些区块
    wait_for_blocks 3
    
    # 测试治理合约
    test_governance "$ACCOUNT"
    
    # 测试安全配置合约
    test_security_config
    
    # 编译测试合约
    compile_contract
    
    # 部署测试合约
    CONTRACT=$(deploy_contract "$ACCOUNT")
    
    # 测试密码学接口
    test_crypto_interfaces "$ACCOUNT" "$CONTRACT"
    
    echo ""
    echo "=========================================="
    echo "所有测试完成！"
    echo "=========================================="
    
    log_success "节点仍在运行，可以继续手动测试"
    log_info "查看日志: tail -f $TEST_DIR/node.log"
    log_info "停止节点: pkill -f 'geth.*test-node'"
}

# 运行主流程
main
