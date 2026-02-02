#!/bin/bash
echo "开始测试..."
export PATH=/usr/local/go/bin:$PATH

# 1. 检查 Go
if [ ! -f "/usr/local/go/bin/go" ]; then
    echo "安装 Go..."
    wget -q https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
    tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
    rm go1.21.6.linux-amd64.tar.gz
fi

go version

# 2. 编译 geth
echo "编译 geth..."
cd /workspace
make geth
ls -lh build/bin/geth

# 3. 初始化创世区块
echo "初始化创世区块..."
DATADIR=/tmp/test-node
rm -rf $DATADIR
./build/bin/geth --datadir $DATADIR init test/integration/genesis-complete.json

echo "测试完成!"
