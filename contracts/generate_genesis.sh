#!/bin/bash
# 生成包含所有系统合约的创世区块配置

set -e

echo "生成包含完整系统合约的创世配置..."

CONTRACTS_DIR="/home/runner/work/go-ethereum/go-ethereum/contracts"
BUILD_DIR="$CONTRACTS_DIR/build"
OUTPUT="/home/runner/work/go-ethereum/go-ethereum/test/integration/genesis-complete.json"

# 创世 MRENCLAVE（示例值）
GENESIS_MRENCLAVE="0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

# 编译合约（如果还没编译）
if [ ! -f "$BUILD_DIR/SecurityConfigContract.bin" ]; then
    echo "编译合约..."
    if command -v solc >/dev/null 2>&1; then
        cd "$CONTRACTS_DIR"
        solc --bin --abi --optimize SecurityConfigContract.sol -o "$BUILD_DIR/" --overwrite
        solc --bin --abi --optimize GovernanceContract.sol -o "$BUILD_DIR/" --overwrite
        solc --bin --abi --optimize IncentiveContract.sol -o "$BUILD_DIR/" --overwrite
    else
        echo "错误: solc 未安装"
        exit 1
    fi
fi

# 读取编译后的字节码
SEC_BIN=$(cat "$BUILD_DIR/SecurityConfigContract.bin")
GOV_BIN=$(cat "$BUILD_DIR/GovernanceContract.bin")
INC_BIN=$(cat "$BUILD_DIR/IncentiveContract.bin")

# 生成创世配置
cat > "$OUTPUT" << EOF
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
  "gasLimit": "30000000",
  "timestamp": "0x0",
  "extraData": "0x",
  "alloc": {
    "0x0000000000000000000000000000000000001001": {
      "balance": "0",
      "code": "0x$GOV_BIN"
    },
    "0x0000000000000000000000000000000000001002": {
      "balance": "0",
      "code": "0x$SEC_BIN"
    },
    "0x0000000000000000000000000000000000001003": {
      "balance": "0",
      "code": "0x$INC_BIN"
    },
    "0x0000000000000000000000000000000000008000": {
      "balance": "0",
      "code": "0x00"
    },
    "0x0000000000000000000000000000000000008001": {
      "balance": "0",
      "code": "0x00"
    },
    "0x0000000000000000000000000000000000008002": {
      "balance": "0",
      "code": "0x00"
    },
    "0x0000000000000000000000000000000000008003": {
      "balance": "0",
      "code": "0x00"
    },
    "0x0000000000000000000000000000000000008004": {
      "balance": "0",
      "code": "0x00"
    },
    "0x0000000000000000000000000000000000008005": {
      "balance": "0",
      "code": "0x00"
    },
    "0x0000000000000000000000000000000000008006": {
      "balance": "0",
      "code": "0x00"
    },
    "0x0000000000000000000000000000000000008007": {
      "balance": "0",
      "code": "0x00"
    },
    "0x0000000000000000000000000000000000008008": {
      "balance": "0",
      "code": "0x00"
    },
    "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266": {
      "balance": "10000000000000000000000"
    },
    "0x70997970C51812dc3A010C7d01b50e0d17dc79C8": {
      "balance": "10000000000000000000000"
    }
  }
}
EOF

echo "✓ 创世配置生成完成: $OUTPUT"
echo ""
echo "配置包含："
echo "  - 治理合约 (0x1001)"
echo "  - 安全配置合约 (0x1002)"
echo "  - 激励合约 (0x1003)"
echo "  - 预编译合约 (0x8000-0x8008)"
echo "  - 测试账户（预充值）"
echo ""
echo "下一步: 使用此配置初始化节点并测试"
