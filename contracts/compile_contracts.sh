#!/bin/bash
set -e

echo "=== 编译系统合约 ==="

# 创建输出目录
mkdir -p build

# 检查 solc
if ! command -v solc &> /dev/null; then
    echo "安装 solc..."
    wget -q https://github.com/ethereum/solidity/releases/download/v0.8.19/solc-static-linux -O /tmp/solc
    chmod +x /tmp/solc
    SOLC=/tmp/solc
else
    SOLC=solc
fi

echo "Solc version:"
$SOLC --version | head -2

# 编译合约
echo ""
echo "编译 SecurityConfigContract..."
$SOLC --bin --abi --optimize SecurityConfigContract.sol -o build/ --overwrite

echo "编译 GovernanceContract..."
$SOLC --bin --abi --optimize GovernanceContract.sol -o build/ --overwrite

echo "编译 IncentiveContract..."
$SOLC --bin --abi --optimize IncentiveContract.sol -o build/ --overwrite

echo ""
echo "=== 编译完成 ==="
ls -lh build/
