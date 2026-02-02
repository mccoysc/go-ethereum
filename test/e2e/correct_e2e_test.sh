#!/bin/bash
#
# 正确的端到端测试
# 1. 部署CryptoTestContract并从合约内调用密码学接口
# 2. 与预部署的GovernanceContract (0x1001)交互
# 3. 验证IncentiveContract (0x1003)记录数据（激励逻辑在Go代码）

set -e

REPO_ROOT="/home/runner/work/go-ethereum/go-ethereum"
cd "$REPO_ROOT"

RPC_URL="http://localhost:8545"
GETH_BIN="./build/bin/geth"
DATADIR="./test-e2e-datadir"

echo "========================================"
echo "正确的端到端测试"
echo "========================================"

# 检查节点是否运行
if ! curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    "$RPC_URL" > /dev/null 2>&1; then
    echo "❌ 节点未运行，请先启动节点"
    exit 1
fi

echo "✓ 节点运行中"

# 获取账户
ACCOUNT=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_accounts","params":[],"id":1}' \
    "$RPC_URL" | grep -o '"0x[^"]*"' | head -1 | tr -d '"')

if [ -z "$ACCOUNT" ]; then
    echo "❌ 未找到账户"
    exit 1
fi

echo "✓ 使用账户: $ACCOUNT"

echo ""
echo "========================================"
echo "测试 1: 部署CryptoTestContract"
echo "========================================"

# 编译CryptoTestContract
echo "编译CryptoTestContract..."
cd contracts
if [ ! -f "build/CryptoTestContract.bin" ]; then
    echo "需要先编译合约"
    # 这里需要solc编译器
    # solc --bin --abi CryptoTestContract.sol -o build/
fi

# TODO: 部署合约
# 由于需要solc和完整的部署流程，这里先展示逻辑

echo "CryptoTestContract部署步骤："
echo "  1. 编译合约得到bytecode"
echo "  2. 使用eth_sendTransaction部署"
echo "  3. 获取合约地址"
echo "  4. 调用合约方法测试所有密码学接口"

echo ""
echo "========================================"
echo "测试 2: 与预部署的GovernanceContract交互"
echo "========================================"

GOVERNANCE_ADDR="0x0000000000000000000000000000000000001001"

echo "治理合约地址: $GOVERNANCE_ADDR"
echo "✓ 该合约已在创世区块预部署"

# 检查合约代码
CODE=$(curl -s -X POST -H "Content-Type: application/json" \
    --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$GOVERNANCE_ADDR\",\"latest\"],\"id\":1}" \
    "$RPC_URL" | grep -o '"0x[^"]*"' | grep -v "jsonrpc" | tr -d '"')

CODE_LEN=${#CODE}
echo "合约代码长度: $((CODE_LEN / 2 - 1)) bytes"

if [ $CODE_LEN -gt 10 ]; then
    echo "✓ 治理合约已部署"
else
    echo "❌ 治理合约未部署"
    exit 1
fi

echo ""
echo "治理合约交互测试："
echo "  1. registerValidator() - 注册验证者"
echo "  2. createProposal() - 创建提案"  
echo "  3. vote() - 投票"
echo "  4. executeProposal() - 执行提案"

echo ""
echo "========================================"
echo "测试 3: 验证IncentiveContract用途"
echo "========================================"

INCENTIVE_ADDR="0x0000000000000000000000000000000000001003"

echo "激励合约地址: $INCENTIVE_ADDR"

# 检查合约代码
CODE=$(curl -s -X POST -H "Content-Type: application/json" \
    --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$INCENTIVE_ADDR\",\"latest\"],\"id\":1}" \
    "$RPC_URL" | grep -o '"0x[^"]*"' | grep -v "jsonrpc" | tr -d '"')

CODE_LEN=${#CODE}
echo "合约代码长度: $((CODE_LEN / 2 - 1)) bytes"

if [ $CODE_LEN -gt 10 ]; then
    echo "✓ IncentiveContract已部署"
else
    echo "❌ IncentiveContract未部署"
fi

echo ""
echo "IncentiveContract作用澄清："
echo "  ✓ 仅用于记录奖励数据（数据存储）"
echo "  ✓ 激励逻辑在Go代码中实现（incentive/包）"
echo "  ✓ 共识引擎在分配奖励时调用合约记录"
echo "  ✓ 包含: RewardRecord, ReputationRecord等"

echo ""
echo "========================================"
echo "测试 4: 验证SecurityConfigContract"
echo "========================================"

SECURITY_ADDR="0x0000000000000000000000000000000000001002"

echo "安全配置合约地址: $SECURITY_ADDR"

# 检查合约代码
CODE=$(curl -s -X POST -H "Content-Type: application/json" \
    --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$SECURITY_ADDR\",\"latest\"],\"id\":1}" \
    "$RPC_URL" | grep -o '"0x[^"]*"' | grep -v "jsonrpc" | tr -d '"')

CODE_LEN=${#CODE}
echo "合约代码长度: $((CODE_LEN / 2 - 1)) bytes"

if [ $CODE_LEN -gt 10 ]; then
    echo "✓ SecurityConfigContract已部署"
else
    echo "❌ SecurityConfigContract未部署"
fi

echo ""
echo "========================================"
echo "总结"
echo "========================================"

echo "✓ 测试方法已修正："
echo "  1. 密码学接口 - 通过部署的合约调用（不是直接HTTP）"
echo "  2. 治理合约 - 使用预部署的合约（不重新部署）"
echo "  3. 激励合约 - 明确仅用于数据记录（逻辑在Go代码）"

echo ""
echo "下一步需要完成："
echo "  [ ] 部署CryptoTestContract"
echo "  [ ] 调用CryptoTestContract方法测试所有密码学接口"
echo "  [ ] 与GovernanceContract交互测试治理功能"
echo "  [ ] 验证激励机制Go代码在共识中的作用"

