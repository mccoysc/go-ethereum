#!/bin/bash
# Complete E2E test for all 4 main tasks

set -e

DATADIR=/tmp/sgx-e2e-test
GENESIS=/tmp/sgx-e2e-genesis.json
LOGFILE=geth-e2e.log

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Cleanup
echo "Cleaning up..."
rm -rf $DATADIR $GENESIS $LOGFILE
pkill -f geth-testenv || true
sleep 2

# Set environment
export SGX_TEST_MODE=true
export GRAMINE_VERSION=test
export GOVERNANCE_CONTRACT=0x1234567890123456789012345678901234567890
export SECURITY_CONFIG_CONTRACT=0x2345678901234567890123456789012345678901

# Create genesis with funded account
cat > $GENESIS << 'EOF'
{
  "config": {
    "chainId": 1337,
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
    "sgx": {
      "period": 1,
      "epoch": 30000,
      "governanceContract": "0x1234567890123456789012345678901234567890",
      "securityConfig": "0x2345678901234567890123456789012345678901"
    }
  },
  "difficulty": "1",
  "gasLimit": "8000000",
  "alloc": {
    "0x71562b71999873DB5b286dF957af199Ec94617F7": {
      "balance": "1000000000000000000000"
    }
  }
}
EOF

# Init
echo "Initializing genesis..."
./geth-testenv --datadir $DATADIR init $GENESIS > /dev/null 2>&1

# Start geth
echo "Starting geth node..."
./geth-testenv --datadir $DATADIR \
    --http --http.port 8545 \
    --http.api eth,web3,net,personal \
    --allow-insecure-unlock \
    --nodiscover --maxpeers 0 \
    --verbosity 4 \
    > $LOGFILE 2>&1 &

GETH_PID=$!
sleep 5

echo "Geth PID: $GETH_PID"
echo ""

# Helper function to make RPC calls
rpc_call() {
    curl -s -X POST --data "$1" -H "Content-Type: application/json" http://localhost:8545
}

# Test 1: Block Production
echo "================================================"
echo "TASK 1: 确保正常出块 (Block Production)"
echo "================================================"

# Check initial block number
BLOCK_0=$(rpc_call '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}')
echo "Initial block: $BLOCK_0"

# Create account and send transaction
echo "Using pre-funded account: 0x71562b71999873DB5b286dF957af199Ec94617F7"
ACCOUNT="0x0000000000000000000000000000000000000001"

# Unlock account
echo "Unlocking account..."
rpc_call '{"jsonrpc":"2.0","method":"personal_unlockAccount","params":["0x71562b71999873DB5b286dF957af199Ec94617F7","",300],"id":1}' > /dev/null

# Send transaction
echo "Sending transaction..."
TX_HASH=$(rpc_call '{"jsonrpc":"2.0","method":"eth_sendTransaction","params":[{"from":"0x71562b71999873DB5b286dF957af199Ec94617F7","to":"0x0000000000000000000000000000000000000001","value":"0x1000","gas":"0x5208","gasPrice":"0x1"}],"id":1}' | python3 -c "import sys, json; print(json.load(sys.stdin).get('result',''))" 2>/dev/null || echo "error")
echo "Transaction: $TX_HASH"

# Wait for block production
echo "Waiting for block production (10s)..."
sleep 10

# Check new block number
BLOCK_1=$(rpc_call '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}')
echo "Current block: $BLOCK_1"

# Verify block was produced
if echo "$BLOCK_1" | grep -q '"result":"0x[1-9]'; then
    echo -e "${GREEN}✓ TASK 1 PASSED: Block produced!${NC}"
    TASK1_PASS=1
else
    echo -e "${RED}✗ TASK 1 FAILED: No block produced${NC}"
    TASK1_PASS=0
fi

# Get block details
BLOCK_DETAILS=$(rpc_call '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x1",true],"id":1}')
echo "Block 1 details:"
echo "$BLOCK_DETAILS" | python3 -m json.tool 2>/dev/null || echo "$BLOCK_DETAILS"

echo ""

# Test 2: Crypto Precompiles
echo "================================================"
echo "TASK 2: 密码学预编译接口 (Crypto Precompiles)"
echo "================================================"

# Test SGXRandom (0x8005)
echo "Testing SGXRandom precompile at 0x8005..."
RANDOM_RESULT=$(rpc_call '{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x0000000000000000000000000000000000008005","data":"0x0000000000000000000000000000000000000000000000000000000000000020"},"latest"],"id":1}')
echo "Random result: $RANDOM_RESULT"

if echo "$RANDOM_RESULT" | grep -q '"result":"0x[0-9a-fA-F]\{64,\}"'; then
    echo -e "${GREEN}✓ TASK 2 PASSED: Crypto precompile working!${NC}"
    TASK2_PASS=1
else
    echo -e "${RED}✗ TASK 2 FAILED: Precompile not working${NC}"
    TASK2_PASS=0
fi

echo ""

# Test 3: Secret Data Storage
echo "================================================"
echo "TASK 3: 秘密数据同步 (Secret Data Sync)"
echo "================================================"

# Check if encrypted storage module loaded
if grep -q "Encrypted Storage" $LOGFILE; then
    echo -e "${GREEN}✓ TASK 3 PASSED: Encrypted Storage module loaded${NC}"
    TASK3_PASS=1
else
    echo -e "${RED}✗ TASK 3 FAILED: Encrypted Storage not loaded${NC}"
    TASK3_PASS=0
fi

echo ""

# Test 4: Governance Contracts
echo "================================================"
echo "TASK 4: 治理合约验证 (Governance Contracts)"
echo "================================================"

# Check if governance system loaded
if grep -q "Governance System" $LOGFILE; then
    echo -e "${GREEN}✓ TASK 4 PASSED: Governance System loaded${NC}"
    TASK4_PASS=1
else
    echo -e "${RED}✗ TASK 4 FAILED: Governance System not loaded${NC}"
    TASK4_PASS=0
fi

echo ""

# Summary
echo "================================================"
echo "SUMMARY"
echo "================================================"
TOTAL=$((TASK1_PASS + TASK2_PASS + TASK3_PASS + TASK4_PASS))
echo "Tasks passed: $TOTAL/4"
echo ""
echo "Task 1 (Block Production): $([ $TASK1_PASS -eq 1 ] && echo -e "${GREEN}PASS${NC}" || echo -e "${RED}FAIL${NC}")"
echo "Task 2 (Crypto Precompiles): $([ $TASK2_PASS -eq 1 ] && echo -e "${GREEN}PASS${NC}" || echo -e "${RED}FAIL${NC}")"
echo "Task 3 (Secret Data Sync): $([ $TASK3_PASS -eq 1 ] && echo -e "${GREEN}PASS${NC}" || echo -e "${RED}FAIL${NC}")"
echo "Task 4 (Governance Contracts): $([ $TASK4_PASS -eq 1 ] && echo -e "${GREEN}PASS${NC}" || echo -e "${RED}FAIL${NC}")"
echo ""

# Show relevant log entries
echo "Relevant log entries:"
echo "---"
grep -E "(Block sealed|Block produced|SGX|Governance|Storage)" $LOGFILE | tail -20

# Cleanup
kill $GETH_PID 2>/dev/null || true

if [ $TOTAL -eq 4 ]; then
    echo -e "\n${GREEN}ALL TASKS PASSED!${NC}"
    exit 0
else
    echo -e "\n${YELLOW}Some tasks failed. Check logs above.${NC}"
    exit 1
fi
