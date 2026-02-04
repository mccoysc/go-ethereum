#!/bin/bash

# Test if block producer heartbeat works (60 second max interval)

set -e

echo "=== Testing Block Producer Heartbeat (60s MaxBlockInterval) ==="
echo

# Setup
DATADIR=/tmp/sgx-heartbeat-test
GENESIS=/tmp/sgx-heartbeat-genesis.json

# Cleanup
rm -rf $DATADIR $GENESIS
rm -f geth.log

# Set environment variables
export SGX_TEST_MODE=true
export GRAMINE_VERSION=test
export GOVERNANCE_CONTRACT=0x1234567890123456789012345678901234567890
export SECURITY_CONFIG_CONTRACT=0x2345678901234567890123456789012345678901

# Build if not exists
if [ ! -f ./geth-testenv ]; then
    echo "Building geth-testenv..."
    go build -tags testenv -o ./geth-testenv ./cmd/geth
fi

# Create genesis
echo "1. Creating genesis..."
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
  "alloc": {}
}
EOF

# Initialize
echo "2. Initializing genesis..."
./geth-testenv --datadir $DATADIR init $GENESIS

# Start geth in background
echo "3. Starting geth node..."
./geth-testenv --datadir $DATADIR \
    --http --http.api eth,web3,net,debug \
    --http.addr 0.0.0.0 --http.port 8545 \
    --http.corsdomain "*" \
    --nodiscover --maxpeers 0 \
    --networkid 1337 \
    --verbosity 4 \
    > geth.log 2>&1 &

GETH_PID=$!
echo "Geth started with PID: $GETH_PID"

# Wait for geth to start
sleep 5

# Check initial block number
echo "4. Checking initial block number..."
BLOCK_NUM=$(curl -s -X POST --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    http://localhost:8545 | jq -r '.result')
echo "Initial block number: $BLOCK_NUM"

# Wait for heartbeat (60 seconds + buffer)
echo "5. Waiting 65 seconds for heartbeat block production..."
for i in {1..13}; do
    sleep 5
    BLOCK_NUM=$(curl -s -X POST --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
        http://localhost:8545 | jq -r '.result')
    echo "   After $((i*5))s: Block number = $BLOCK_NUM"
done

# Final check
echo
echo "6. Final block number:"
FINAL_BLOCK=$(curl -s -X POST --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    http://localhost:8545 | jq -r '.result')
echo "Block number: $FINAL_BLOCK"

# Check logs for block production
echo
echo "7. Checking logs for block production:"
grep -i "block.*produced\|block.*sealed\|Attempting to produce block" geth.log | head -20 || echo "No block production found in logs"

# Cleanup
kill $GETH_PID 2>/dev/null || true

echo
if [ "$FINAL_BLOCK" != "0x0" ]; then
    echo "✓ SUCCESS: Heartbeat block production working!"
    exit 0
else
    echo "✗ FAILED: No blocks produced after 65 seconds"
    echo
    echo "Last 50 lines of geth.log:"
    tail -50 geth.log
    exit 1
fi
