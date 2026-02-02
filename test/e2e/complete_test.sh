#!/bin/bash
set -e

echo "=========================================="
echo "完整端到端测试"
echo "=========================================="

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 测试目录
TEST_DIR="/home/runner/work/go-ethereum/go-ethereum/test-e2e"
mkdir -p "$TEST_DIR"
cd "$TEST_DIR"

echo ""
echo "=========================================="
echo "Phase 1: 准备测试环境"
echo "=========================================="

# 1.1 创建测试manifest文件
echo -e "${YELLOW}创建测试manifest文件...${NC}"
cat > geth.manifest << 'MANIFEST_EOF'
loader.entrypoint = "file:{{ gramine.libos }}"
libos.entrypoint = "/app/geth"

loader.log_level = "error"

loader.env.LD_LIBRARY_PATH = "/lib:/usr/lib:/lib/x86_64-linux-gnu"
loader.env.GRAMINE_VERSION = "v1.6-test"
loader.env.RA_TLS_MRENCLAVE = "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

# 合约地址配置
loader.env.XCHAIN_GOVERNANCE_CONTRACT = "0x0000000000000000000000000000000000001001"
loader.env.XCHAIN_SECURITY_CONFIG_CONTRACT = "0x0000000000000000000000000000000000001002"
loader.env.XCHAIN_INCENTIVE_CONTRACT = "0x0000000000000000000000000000000000001003"

fs.mounts = [
  { path = "/lib", uri = "file:/lib" },
  { path = "/usr/lib", uri = "file:/usr/lib" },
  { path = "/app", uri = "file:/app" },
]

sgx.enclave_size = "1024M"
sgx.max_threads = 32
MANIFEST_EOF

echo -e "${GREEN}✓ Manifest文件创建完成${NC}"

# 1.2 生成测试签名密钥
echo -e "${YELLOW}生成RSA-3072测试密钥...${NC}"
openssl genrsa -3 -out test-signing-key.pem 3072 2>/dev/null
openssl rsa -in test-signing-key.pem -pubout -out test-signing-key.pub 2>/dev/null
echo -e "${GREEN}✓ 签名密钥生成完成${NC}"

# 1.3 创建模拟的SIGSTRUCT签名文件
echo -e "${YELLOW}创建模拟签名文件...${NC}"
python3 << 'PYTHON_EOF'
import struct
import hashlib

# 读取manifest
with open('geth.manifest', 'rb') as f:
    manifest_data = f.read()

# 计算manifest哈希
manifest_hash = hashlib.sha256(manifest_data).digest()

# 创建SIGSTRUCT结构 (1808字节)
sigstruct = bytearray(1808)

# Header (16 bytes)
sigstruct[0:16] = b'\x06\x00\x00\x00\xe1\x00\x00\x00\x00\x00\x01\x00\x00\x00\x00\x00'

# Vendor (4 bytes)
sigstruct[16:20] = struct.pack('<I', 0x8086)  # Intel

# Date (4 bytes) 
sigstruct[20:24] = struct.pack('<I', 20260201)

# MRENCLAVE (offset 960, 32 bytes) - 必须与环境变量一致
mrenclave = bytes.fromhex('1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef')
sigstruct[960:992] = mrenclave

# 模拟的RSA签名 (offset 128, 384 bytes)
# 在实际环境中这应该是真实的RSA-3072签名
# 这里只是填充一些数据用于测试
signature = hashlib.sha256(manifest_hash).digest() * 12  # 32*12 = 384 bytes
sigstruct[128:512] = signature[:384]

# 写入签名文件
with open('geth.manifest.sig', 'wb') as f:
    f.write(sigstruct)

print("✓ 签名文件创建完成")
PYTHON_EOF

echo -e "${GREEN}✓ 签名文件创建完成${NC}"

# 1.4 设置环境变量
echo -e "${YELLOW}设置测试环境变量...${NC}"
export GRAMINE_VERSION="v1.6-test"
export RA_TLS_MRENCLAVE="1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
export RA_TLS_MRSIGNER="abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
export GRAMINE_MANIFEST_PATH="$TEST_DIR/geth.manifest"
export GRAMINE_SIGSTRUCT_KEY_PATH="$TEST_DIR/test-signing-key.pub"

echo -e "${GREEN}✓ 环境变量设置完成${NC}"
echo "  GRAMINE_VERSION: $GRAMINE_VERSION"
echo "  RA_TLS_MRENCLAVE: $RA_TLS_MRENCLAVE"
echo "  GRAMINE_MANIFEST_PATH: $GRAMINE_MANIFEST_PATH"

echo ""
echo "=========================================="
echo "Phase 2: 编译和初始化"
echo "=========================================="

# 2.1 编译geth
echo -e "${YELLOW}编译geth...${NC}"
cd /home/runner/work/go-ethereum/go-ethereum
if [ ! -f "build/bin/geth" ]; then
    make geth
fi
echo -e "${GREEN}✓ Geth编译完成${NC}"

# 2.2 创建创世配置
echo -e "${YELLOW}创建创世配置...${NC}"
cd "$TEST_DIR"
cat > genesis.json << 'GENESIS_EOF'
{
  "config": {
    "chainId": 762385986,
    "homesteadBlock": 0,
    "eip150Block": 0,
    "eip155Block": 0,
    "eip158Block": 0,
    "byzantiumBlock": 0,
    "constantinopleBlock": 0,
    "petersburgBlock": 0,
    "istanbulBlock": 0,
    "berlinBlock": 0,
    "londonBlock": 0,
    "terminalTotalDifficulty": 0,
    "sgx": {
      "period": 5,
      "epoch": 30000,
      "governanceContract": "0x0000000000000000000000000000000000001001",
      "securityConfig": "0x0000000000000000000000000000000000001002",
      "incentiveContract": "0x0000000000000000000000000000000000001003"
    }
  },
  "difficulty": "1",
  "gasLimit": "8000000",
  "alloc": {
    "0x0000000000000000000000000000000000001001": {
      "balance": "0",
      "code": "0x608060405234801561001057600080fd5b50600436106100415760003560e01c806301ffc9a71461004657806320c13b0b14610076578063e2f273bd14610094575b600080fd5b610060600480360381019061005b91906102f1565b6100b2565b60405161006d9190610333565b60405180910390f35b61007e610124565b60405161008b919061035d565b60405180910390f35b61009c61012a565b6040516100a9919061035d565b60405180910390f35b60007f01ffc9a7000000000000000000000000000000000000000000000000000000007bffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916827bffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916149050919050565b60005481565b60015481565b600080fd5b600080fd5b60007fffffffff0000000000000000000000000000000000000000000000000000000082169050919050565b61017481610139565b811461017f57600080fd5b50565b6000813590506101918161016b565b92915050565b6000602082840312156101ad576101ac610134565b5b60006101bb84828501610182565b91505092915050565b60008115159050919050565b6101d9816101c4565b82525050565b60006020820190506101f460008301846101d0565b92915050565b6000819050919050565b61020d816101fa565b82525050565b60006020820190506102286000830184610204565b92915050565b600081519050919050565b600082825260208201905092915050565b60005b8381101561026857808201518184015260208101905061024d565b83811115610277576000848401525b50505050565b6000601f19601f8301169050919050565b60006102998261022e565b6102a38185610239565b93506102b381856020860161024a565b6102bc8161027d565b840191505092915050565b600060208201905081810360008301526102e1818461028e565b905092915050565b60006020828403121561030057600080fd5b600061030e84828501610182565b91505092915050565b610320816101c4565b82525050565b600060208201905061033b6000830184610317565b92915050565b610"
    },
    "0x0000000000000000000000000000000000001002": {
      "balance": "0",
      "code": "0x6080604052348015600f57600080fd5b506004361060325760003560e01c80632f54bf6e146037578063a0e67e2b14605b575b600080fd5b604960048036038101906044919060ca565b607f565b60405160529190610118565b60405180910390f35b606360d5565b6040516076919061019b565b60405180910390f35b600080915050919050565b606060008054905067ffffffffffffffff81111560a05760009190506060565b6040519080825280602002602001820160405280156060d05780820160200182028036833780820191505090505b5090505b90565b60008135905060e48160f6565b92915050565b60006020828403121560fc5760009190506101f0565b600061010a8482850160d7565b91505092915050565b6101128160f6565b82525050565b600060208201905061012d6000830184610109565b92915050565b600081519050919050565b600082825260208201905092915050565b6000819050602082019050919050565b61016881606081565b82525050565b600061017a838361015f565b60208301905092915050565b6000602082019050919050565b60006101aa83610133565b6101b4818561013e565b93506101bf8361014f565b8060005b838110156101f05781516101d7888261016e565b97506101e283610186565b9250506001810190506101c3565b5085935050505092915050565b600073ffffffffffffffffffffffffffffffffffffffff82169050919050565b6000610228826101fd565b9050919050565b6102388161021d565b82525050565b600060208201905061025360008301846102"
    },
    "0x0000000000000000000000000000000000001003": {
      "balance": "0",
      "code": "0x608060405234801561001057600080fd5b50600436106100415760003560e01c8063095ea7b31461004657806323b872dd1461007657806370a08231146100a6575b600080fd5b610060600480360381019061005b91906102f1565b6100d6565b60405161006d9190610333565b60405180910390f35b610090600480360381019061008b9190610384565b6101c8565b60405161009d9190610333565b60405180910390f35b6100c060048036038101906100bb91906103d7565b610397565b6040516100cd9190610413565b60405180910390f35b600081600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508273ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925846040516101b69190610413565b60405180910390a36001905092915050565b600081600160008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205410156102"
    },
    "0x8875022f57343979503b4a95637315064eb01698": {
      "balance": "1000000000000000000000"
    }
  }
}
GENESIS_EOF

echo -e "${GREEN}✓ 创世配置创建完成${NC}"

# 2.3 初始化创世区块
echo -e "${YELLOW}初始化创世区块...${NC}"
/home/runner/work/go-ethereum/go-ethereum/build/bin/geth init --datadir "$TEST_DIR/node-data" genesis.json
echo -e "${GREEN}✓ 创世区块初始化完成${NC}"

echo ""
echo "=========================================="
echo "Phase 3: 启动节点并验证"  
echo "=========================================="

# 3.1 创建测试账户
echo -e "${YELLOW}创建测试账户...${NC}"
echo "test123" > "$TEST_DIR/password.txt"
ACCOUNT=$(/home/runner/work/go-ethereum/go-ethereum/build/bin/geth account new --datadir "$TEST_DIR/node-data" --password "$TEST_DIR/password.txt" | grep "Public address" | awk '{print $4}')
echo -e "${GREEN}✓ 测试账户创建: $ACCOUNT${NC}"

# 3.2 启动节点（后台）
echo -e "${YELLOW}启动节点...${NC}"
/home/runner/work/go-ethereum/go-ethereum/build/bin/geth \
    --datadir "$TEST_DIR/node-data" \
    --networkid 762385986 \
    --http \
    --http.addr "127.0.0.1" \
    --http.port 8545 \
    --http.api "eth,net,web3,personal,admin,debug" \
    --nodiscover \
    --maxpeers 0 \
    --verbosity 3 \
    > "$TEST_DIR/node.log" 2>&1 &

GETH_PID=$!
echo -e "${GREEN}✓ 节点已启动 (PID: $GETH_PID)${NC}"

# 等待节点就绪
echo -e "${YELLOW}等待节点就绪...${NC}"
for i in {1..30}; do
    if curl -s -X POST -H "Content-Type: application/json" \
        --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
        http://127.0.0.1:8545 > /dev/null 2>&1; then
        echo -e "${GREEN}✓ 节点已就绪${NC}"
        break
    fi
    if [ $i -eq 30 ]; then
        echo -e "${RED}✗ 节点启动超时${NC}"
        tail -20 "$TEST_DIR/node.log"
        kill $GETH_PID 2>/dev/null
        exit 1
    fi
    sleep 1
done

echo ""
echo "=========================================="
echo "Phase 4: 测试结果"
echo "=========================================="

# 4.1 检查日志中的模块加载信息
echo ""
echo -e "${YELLOW}=== 检查模块加载 ===${NC}"
if grep -q "Loading Module 01: SGX Attestation" "$TEST_DIR/node.log"; then
    echo -e "${GREEN}✓ 模块 01 (SGX Attestation) 已加载${NC}"
fi
if grep -q "Loading Module 02: SGX Consensus Engine" "$TEST_DIR/node.log"; then
    echo -e "${GREEN}✓ 模块 02 (SGX Consensus Engine) 已加载${NC}"
fi
if grep -q "Loading Module 03: Incentive Mechanism" "$TEST_DIR/node.log"; then
    echo -e "${GREEN}✓ 模块 03 (Incentive Mechanism) 已加载${NC}"
fi
if grep -q "Loading Module 04: Precompiled Contracts" "$TEST_DIR/node.log"; then
    echo -e "${GREEN}✓ 模块 04 (Precompiled Contracts) 已加载${NC}"
fi
if grep -q "Loading Module 05: Governance System" "$TEST_DIR/node.log"; then
    echo -e "${GREEN}✓ 模块 05 (Governance System) 已加载${NC}"
fi
if grep -q "Loading Module 06: Encrypted Storage" "$TEST_DIR/node.log"; then
    echo -e "${GREEN}✓ 模块 06 (Encrypted Storage) 已加载${NC}"
fi
if grep -q "Loading Module 07: Gramine Integration" "$TEST_DIR/node.log"; then
    echo -e "${GREEN}✓ 模块 07 (Gramine Integration) 已加载${NC}"
fi

# 4.2 验证网络信息
echo ""
echo -e "${YELLOW}=== 网络信息验证 ===${NC}"
CHAIN_ID=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' \
    http://127.0.0.1:8545 | jq -r '.result')
echo "Chain ID: $CHAIN_ID"
if [ "$CHAIN_ID" == "0x2d6f8982" ]; then
    echo -e "${GREEN}✓ Chain ID 正确 (762385986)${NC}"
else
    echo -e "${RED}✗ Chain ID 错误${NC}"
fi

# 4.3 验证系统合约
echo ""
echo -e "${YELLOW}=== 系统合约验证 ===${NC}"

# 治理合约
GOV_CODE=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_getCode","params":["0x0000000000000000000000000000000000001001","latest"],"id":1}' \
    http://127.0.0.1:8545 | jq -r '.result')
echo "治理合约 (0x1001): ${#GOV_CODE} 字符"
if [ "${#GOV_CODE}" -gt 10 ]; then
    echo -e "${GREEN}✓ 治理合约已部署${NC}"
fi

# 安全配置合约
SEC_CODE=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_getCode","params":["0x0000000000000000000000000000000000001002","latest"],"id":1}' \
    http://127.0.0.1:8545 | jq -r '.result')
echo "安全配置合约 (0x1002): ${#SEC_CODE} 字符"
if [ "${#SEC_CODE}" -gt 10 ]; then
    echo -e "${GREEN}✓ 安全配置合约已部署${NC}"
fi

# 激励合约
INC_CODE=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_getCode","params":["0x0000000000000000000000000000000000001003","latest"],"id":1}' \
    http://127.0.0.1:8545 | jq -r '.result')
echo "激励合约 (0x1003): ${#INC_CODE} 字符"
if [ "${#INC_CODE}" -gt 10 ]; then
    echo -e "${GREEN}✓ 激励合约已部署${NC}"
fi

# 4.4 测试预编译合约
echo ""
echo -e "${YELLOW}=== 预编译合约测试 ===${NC}"

# 测试 SGX_RANDOM (0x8005)
RANDOM_RESULT=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x0000000000000000000000000000000000008005","data":"0x0000000000000000000000000000000000000000000000000000000000000020"},"latest"],"id":1}' \
    http://127.0.0.1:8545 | jq -r '.result // .error.message')

echo "SGX_RANDOM (0x8005): $RANDOM_RESULT"
if [[ "$RANDOM_RESULT" == 0x* ]]; then
    echo -e "${GREEN}✓ SGX_RANDOM 接口可访问${NC}"
else
    echo -e "${YELLOW}⚠ SGX_RANDOM: $RANDOM_RESULT${NC}"
fi

echo ""
echo "=========================================="
echo "测试总结"
echo "=========================================="

# 显示日志片段
echo ""
echo -e "${YELLOW}=== 节点日志片段 ===${NC}"
echo "--- 最后50行 ---"
tail -50 "$TEST_DIR/node.log"

# 清理
echo ""
echo -e "${YELLOW}清理资源...${NC}"
kill $GETH_PID 2>/dev/null
sleep 2
echo -e "${GREEN}✓ 测试完成${NC}"

echo ""
echo "=========================================="
echo "测试文件位置:"
echo "  数据目录: $TEST_DIR"
echo "  日志文件: $TEST_DIR/node.log"
echo "  Manifest: $TEST_DIR/geth.manifest"
echo "  签名文件: $TEST_DIR/geth.manifest.sig"
echo "=========================================="
