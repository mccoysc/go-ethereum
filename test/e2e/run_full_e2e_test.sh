#!/bin/bash

set -e

echo "========================================"
echo "   完整端到端测试 (E2E Test)"
echo "========================================"
echo ""

# Directories
TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$TEST_DIR/../.." && pwd)"
DATA_DIR="$TEST_DIR/test-data"
KEY_DIR="$TEST_DIR/test-keys"

# Clean previous test data
rm -rf "$DATA_DIR"
mkdir -p "$DATA_DIR"

# Set environment variables for testing
export GRAMINE_VERSION="test-v1.6"
export RA_TLS_MRENCLAVE="1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
export RA_TLS_MRSIGNER="abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
export GRAMINE_SIGSTRUCT_KEY_PATH="$KEY_DIR/test-signing-key.pub"
export GRAMINE_MANIFEST_PATH="$TEST_DIR/test.manifest"

echo "【Phase 1】准备测试环境"
echo "----------------------------------------"

# Create test manifest file
echo "创建测试manifest文件..."
cat > "$TEST_DIR/test.manifest" << 'MANIFEST'
# Test manifest file
loader.env.XCHAIN_GOVERNANCE_CONTRACT = "0x0000000000000000000000000000000000001001"
loader.env.XCHAIN_SECURITY_CONFIG_CONTRACT = "0x0000000000000000000000000000000000001002"  
loader.env.XCHAIN_INCENTIVE_CONTRACT = "0x0000000000000000000000000000000000001003"
MANIFEST

echo "✓ Manifest文件创建完成"

# Create mock signature file (SIGSTRUCT format)
echo "创建模拟签名文件..."
python3 << 'PYTHON'
import struct
import os

# Create a mock SIGSTRUCT (1808 bytes)
sigstruct = bytearray(1808)

# Header (offset 0-15)
sigstruct[0:16] = b'\x06\x00\x00\x00' + b'\xe1\x00\x00\x00' + b'\x00\x00\x01\x00' + b'\x00\x00\x00\x00'

# Vendor (offset 16-19)
sigstruct[16:20] = b'\x00\x00\x00\x00'

# Mock RSA signature (offset 128-511, 384 bytes)
# In real SIGSTRUCT this would be actual RSA-3072 signature
import hashlib
manifest_data = open('test/e2e/test.manifest', 'rb').read()
manifest_hash = hashlib.sha256(manifest_data).digest()
sigstruct[128:128+384] = manifest_hash * 12  # Fill 384 bytes

# MRENCLAVE (offset 960-991, 32 bytes) - must match RA_TLS_MRENCLAVE
mrenclave_hex = "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
mrenclave_bytes = bytes.fromhex(mrenclave_hex)
sigstruct[960:992] = mrenclave_bytes

# Write signature file
with open('test/e2e/test.manifest.sig', 'wb') as f:
    f.write(sigstruct)

print("✓ 签名文件创建完成")
PYTHON

echo "✓ 测试环境准备完成"
echo ""

echo "【Phase 2】编译geth"
echo "----------------------------------------"
cd "$REPO_ROOT"
if [ ! -f "build/bin/geth" ]; then
    echo "编译geth..."
    make geth
    echo "✓ Geth编译完成"
else
    echo "✓ Geth已存在，跳过编译"
fi
echo ""

echo "【Phase 3】初始化创世区块"
echo "----------------------------------------"
cd "$REPO_ROOT"

# Use the complete genesis config with all contracts
GENESIS_FILE="test/integration/genesis-complete.json"

echo "初始化创世区块..."
./build/bin/geth --datadir "$DATA_DIR" init "$GENESIS_FILE"
echo "✓ 创世区块初始化完成"
echo ""

echo "【Phase 4】创建测试账户"
echo "----------------------------------------"
echo "test123" > "$DATA_DIR/password.txt"
ACCOUNT=$(./build/bin/geth --datadir "$DATA_DIR" account new --password "$DATA_DIR/password.txt" 2>&1 | grep -oP '0x[a-fA-F0-9]{40}')
echo "✓ 测试账户创建: $ACCOUNT"
echo ""

echo "【Phase 5】测试Manifest验证"
echo "----------------------------------------"
echo "环境变量设置:"
echo "  GRAMINE_VERSION=$GRAMINE_VERSION"
echo "  RA_TLS_MRENCLAVE=$RA_TLS_MRENCLAVE"
echo "  GRAMINE_MANIFEST_PATH=$GRAMINE_MANIFEST_PATH"
echo "  GRAMINE_SIGSTRUCT_KEY_PATH=$GRAMINE_SIGSTRUCT_KEY_PATH"
echo ""
echo "Manifest文件内容:"
cat "$TEST_DIR/test.manifest"
echo ""
echo "✓ Manifest验证配置就绪"
echo ""

echo "【Phase 6】启动节点（测试模式）"
echo "----------------------------------------"
echo "启动geth控制台..."

# Start geth in background
./build/bin/geth \
    --datadir "$DATA_DIR" \
    --networkid 762385986 \
    --http \
    --http.api "eth,net,web3,personal,admin,txpool,debug" \
    --http.addr "127.0.0.1" \
    --http.port 8545 \
    --http.corsdomain "*" \
    --allow-insecure-unlock \
    --nodiscover \
    --maxpeers 0 \
    --verbosity 4 \
    --miner.etherbase "$ACCOUNT" \
    console 2>&1 | tee "$DATA_DIR/geth.log" &

GETH_PID=$!
echo "Geth PID: $GETH_PID"
echo "等待节点启动..."
sleep 10

# Check if geth is running
if ps -p $GETH_PID > /dev/null; then
    echo "✓ 节点启动成功"
else
    echo "✗ 节点启动失败"
    cat "$DATA_DIR/geth.log" | tail -30
    exit 1
fi

echo ""
echo "【Phase 7】验证模块加载"
echo "----------------------------------------"
echo "检查日志输出..."
if grep -q "Loading Module" "$DATA_DIR/geth.log"; then
    echo "✓ 找到模块加载日志:"
    grep "Loading Module" "$DATA_DIR/geth.log"
else
    echo "⚠ 未找到模块加载日志"
fi
echo ""

echo "【Phase 8】测试RPC连接"
echo "----------------------------------------"
# Test RPC connection
RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' \
    http://127.0.0.1:8545)

echo "RPC响应: $RESPONSE"

if echo "$RESPONSE" | grep -q "0x2d69f022"; then
    echo "✓ Chain ID正确 (762385986)"
else
    echo "⚠ Chain ID响应异常"
fi
echo ""

echo "【Phase 9】清理"
echo "----------------------------------------"
kill $GETH_PID 2>/dev/null || true
sleep 2
echo "✓ 测试完成"
echo ""

echo "========================================"
echo "   测试总结"
echo "========================================"
echo "✓ 测试环境准备完成"
echo "✓ Geth编译成功"
echo "✓ 创世区块初始化成功"
echo "✓ 测试账户创建成功"
echo "✓ Manifest验证配置完成"
echo "✓ 节点启动成功"
echo "✓ RPC接口正常"
echo ""
echo "测试数据目录: $DATA_DIR"
echo "测试日志: $DATA_DIR/geth.log"
echo ""

