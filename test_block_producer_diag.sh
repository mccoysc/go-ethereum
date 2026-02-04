#!/bin/bash
# Quick diagnostic: Check if block producer loop is running

set -e

DATADIR=/tmp/sgx-diag-test
GENESIS=/tmp/sgx-diag-genesis.json

# Cleanup
rm -rf $DATADIR $GENESIS
pkill -f geth-testenv || true
sleep 1

# Set environment
export SGX_TEST_MODE=true
export GRAMINE_VERSION=test
export GOVERNANCE_CONTRACT=0x1234567890123456789012345678901234567890
export SECURITY_CONFIG_CONTRACT=0x2345678901234567890123456789012345678901

# Build if needed
if [ ! -f ./geth-testenv ]; then
    echo "Building geth-testenv..."
    go build -tags testenv -o ./geth-testenv ./cmd/geth
fi

# Create genesis
cat > $GENESIS << 'EOF'
{
  "config": {
    "chainId": 1337,
    "homesteadBlock": 0,
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

# Init and start
./geth-testenv --datadir $DATADIR init $GENESIS > /dev/null 2>&1

echo "Starting geth..."
./geth-testenv --datadir $DATADIR \
    --http --http.port 8545 \
    --nodiscover --maxpeers 0 \
    --verbosity 5 \
    > geth-diag.log 2>&1 &

GETH_PID=$!
sleep 5

echo "Checking logs for block producer activity..."
echo

# Check for key log messages
if grep -q "SGX block producer started successfully" geth-diag.log; then
    echo "✓ Block producer started"
else
    echo "✗ Block producer NOT started"
fi

if grep -q "produceLoop" geth-diag.log; then
    echo "✓ produceLoop mentioned in logs"
else
    echo "✗ produceLoop NOT mentioned"
fi

if grep -q "Attempting to produce block" geth-diag.log; then
    echo "✓ Block production attempted"
    grep "Attempting to produce block" geth-diag.log | head -5
else
    echo "✗ No block production attempts found"
fi

echo
echo "Last 30 lines of log:"
tail -30 geth-diag.log

# Cleanup
kill $GETH_PID 2>/dev/null || true
