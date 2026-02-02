#!/bin/bash
set -e

# 一步步创建测试环境并运行测试

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="$(dirname "$SCRIPT_DIR")"
REPO_ROOT="$(dirname "$(dirname "$TEST_DIR")")"

echo "=========================================="
echo "  逐步E2E测试 - 增量模拟数据"
echo "=========================================="
echo ""

# Step 1: 创建真实的manifest和签名
echo "=== Step 1: 创建真实的manifest和签名文件 ==="
bash "${SCRIPT_DIR}/create_test_manifest.sh"

# 读取生成的环境变量
DATA_DIR="${TEST_DIR}/data"
MANIFEST_FILE="${DATA_DIR}/geth.manifest"
SIGNATURE_FILE="${DATA_DIR}/geth.manifest.sig"
PUBLIC_KEY="${DATA_DIR}/test-signing-key.pub"
MANIFEST_HASH=$(sha256sum "${MANIFEST_FILE}" | awk '{print $1}')

echo ""
echo "=== Step 2: 设置环境变量 ==="
export GRAMINE_VERSION="1.6"
export GRAMINE_MANIFEST_PATH="${MANIFEST_FILE}"
export GRAMINE_SIGSTRUCT_KEY_PATH="${PUBLIC_KEY}"
export RA_TLS_MRENCLAVE="${MANIFEST_HASH}"
export RA_TLS_MRSIGNER="0000000000000000000000000000000000000000000000000000000000000000"

echo "✓ GRAMINE_VERSION=${GRAMINE_VERSION}"
echo "✓ GRAMINE_MANIFEST_PATH=${GRAMINE_MANIFEST_PATH}"
echo "✓ GRAMINE_SIGSTRUCT_KEY_PATH=${GRAMINE_SIGSTRUCT_KEY_PATH}"
echo "✓ RA_TLS_MRENCLAVE=${RA_TLS_MRENCLAVE}"

echo ""
echo "=== Step 3: 创建模拟的Gramine伪文件系统 ==="
MOCK_DEV_DIR="${TEST_DIR}/data/mock-dev"
export MOCK_DEV_DIR
bash "${SCRIPT_DIR}/create_mock_attestation.sh" "${MOCK_DEV_DIR}"

echo ""
echo "=== Step 4: 编译geth ==="
cd "${REPO_ROOT}"
if [ ! -f "build/bin/geth" ]; then
    echo "编译geth..."
    make geth
    echo "✓ geth编译完成"
else
    echo "✓ 使用已编译的geth: build/bin/geth"
fi

echo ""
echo "=== Step 5: 初始化创世区块 ==="
DATADIR="${TEST_DIR}/data/test-chain"
rm -rf "${DATADIR}"
mkdir -p "${DATADIR}"

GENESIS_FILE="${TEST_DIR}/../integration/genesis-complete.json"
if [ ! -f "${GENESIS_FILE}" ]; then
    echo "错误: 找不到创世配置文件: ${GENESIS_FILE}"
    exit 1
fi

build/bin/geth --datadir "${DATADIR}" init "${GENESIS_FILE}"
echo "✓ 创世区块已初始化"

echo ""
echo "=== Step 6: 创建测试账户 ==="
ACCOUNT_PASSWORD="test123"
echo "${ACCOUNT_PASSWORD}" > "${DATADIR}/password.txt"

ACCOUNT=$(build/bin/geth --datadir "${DATADIR}" account new --password "${DATADIR}/password.txt" 2>&1 | grep -oP '0x[a-fA-F0-9]{40}')
echo "✓ 测试账户已创建: ${ACCOUNT}"

echo ""
echo "=== Step 7: 启动geth节点（后台） ==="
echo "注意: 当前在非SGX环境，会在尝试挖矿时访问/dev/attestation失败"
echo "启动节点以测试初始化逻辑..."

# 启动节点（不挖矿，只测试初始化）
build/bin/geth \
    --datadir "${DATADIR}" \
    --networkid 762385986 \
    --port 30303 \
    --http \
    --http.port 8545 \
    --http.addr "127.0.0.1" \
    --http.api "eth,web3,net,personal" \
    --nodiscover \
    --maxpeers 0 \
    --verbosity 4 \
    > "${TEST_DIR}/data/geth.log" 2>&1 &

GETH_PID=$!
echo "✓ Geth已启动 (PID: ${GETH_PID})"

# 等待节点启动
echo "等待节点初始化..."
sleep 5

echo ""
echo "=== Step 8: 检查日志 ==="
echo "检查SGX共识引擎初始化日志..."
if grep -q "Initializing SGX Consensus Engine" "${TEST_DIR}/data/geth.log"; then
    echo "✓ SGX共识引擎已初始化"
    grep "Initializing SGX Consensus Engine" "${TEST_DIR}/data/geth.log"
else
    echo "⚠ 未找到SGX共识引擎初始化日志"
fi

echo ""
echo "检查模块加载日志..."
grep "Loading Module" "${TEST_DIR}/data/geth.log" || echo "⚠ 未找到模块加载日志"

echo ""
echo "检查manifest读取日志..."
grep -i "manifest" "${TEST_DIR}/data/geth.log" | head -10 || echo "⚠ 未找到manifest相关日志"

echo ""
echo "=== Step 9: 测试RPC接口 ==="
sleep 2
echo "测试网络连接..."
CHAIN_ID=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' \
    http://127.0.0.1:8545 | grep -oP '0x[0-9a-f]+' || echo "")

if [ -n "${CHAIN_ID}" ]; then
    CHAIN_ID_DEC=$((${CHAIN_ID}))
    echo "✓ Chain ID: ${CHAIN_ID_DEC}"
else
    echo "⚠ 无法连接到RPC接口"
fi

echo ""
echo "=== Step 10: 停止节点 ==="
kill ${GETH_PID} 2>/dev/null || true
wait ${GETH_PID} 2>/dev/null || true
echo "✓ 节点已停止"

echo ""
echo "=========================================="
echo "  测试完成"
echo "=========================================="
echo ""
echo "日志文件: ${TEST_DIR}/data/geth.log"
echo "查看完整日志: cat ${TEST_DIR}/data/geth.log"
echo ""
echo "下一步:"
echo "1. 修改代码支持通过环境变量配置/dev/attestation路径"
echo "2. 重新测试使用模拟的attestation文件"
echo "3. 测试挖矿功能"
echo ""
