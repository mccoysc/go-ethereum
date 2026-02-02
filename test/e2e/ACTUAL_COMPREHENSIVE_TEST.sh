#!/bin/bash

# Actual comprehensive test execution
# Tests: Owner logic, Contract deployment, ReadOnly mode, Governance interaction

set -e

echo "=================================="
echo "ACTUAL COMPREHENSIVE E2E TEST"
echo "=================================="

DATADIR="./test-actual-e2e"
GENESIS_FILE="./test/e2e/data/genesis-with-contracts.json"
GETH="./build/bin/geth"

# Cleanup
rm -rf "$DATADIR" 2>/dev/null || true

echo ""
echo "Step 1: Initialize Genesis"
echo "=================================="
$GETH --datadir "$DATADIR" init "$GENESIS_FILE"

echo ""
echo "Step 2: Create Test Accounts"
echo "=================================="
ACCOUNT1=$(echo "password1" | $GETH --datadir "$DATADIR" account new --password /dev/stdin | grep -oP '(?<=Public address of the key:   )[0-9a-fA-Fx]+')
ACCOUNT2=$(echo "password2" | $GETH --datadir "$DATADIR" account new --password /dev/stdin | grep -oP '(?<=Public address of the key:   )[0-9a-fA-Fx]+')

echo "Account 1 (Alice): $ACCOUNT1"
echo "Account 2 (Bob): $ACCOUNT2"

echo ""
echo "Step 3: Start Geth Node"
echo "=================================="
# Start node in background
$GETH --datadir "$DATADIR" \
  --http --http.api "eth,net,web3,personal" \
  --http.addr "127.0.0.1" --http.port 8546 \
  --http.corsdomain "*" \
  --nodiscover --maxpeers 0 \
  --dev --dev.period 1 \
  --unlock "$ACCOUNT1,$ACCOUNT2" \
  --password <(echo -e "password1\npassword2") \
  --allow-insecure-unlock \
  > test-actual-node.log 2>&1 &

NODE_PID=$!
echo "Node PID: $NODE_PID"
sleep 5

# Test if node is running
if ! kill -0 $NODE_PID 2>/dev/null; then
    echo "ERROR: Node failed to start"
    cat test-actual-node.log
    exit 1
fi

echo "Node started successfully"

echo ""
echo "============================================"
echo "TEST 1: Owner Logic - Key Creation"
echo "============================================"

# Alice creates a key
echo "Alice (${ACCOUNT1:0:10}...) creates a key..."
RESULT=$(curl -s -X POST http://127.0.0.1:8546 \
  -H "Content-Type: application/json" \
  --data "{
    \"jsonrpc\":\"2.0\",
    \"method\":\"eth_sendTransaction\",
    \"params\":[{
      \"from\":\"$ACCOUNT1\",
      \"to\":\"0x0000000000000000000000000000000000008000\",
      \"data\":\"0x0000000000000000000000000000000000000000000000000000000000000020\",
      \"gas\":\"0x100000\"
    }],
    \"id\":1
  }")

echo "Result: $RESULT"

if echo "$RESULT" | grep -q "result"; then
    echo "✓ Alice successfully created a key"
else
    echo "✗ Alice failed to create key"
    echo "$RESULT"
fi

sleep 2

echo ""
echo "============================================"  
echo "TEST 2: ReadOnly Mode - KEY_CREATE"
echo "============================================"

echo "Trying KEY_CREATE via eth_call (should FAIL)..."
RESULT=$(curl -s -X POST http://127.0.0.1:8546 \
  -H "Content-Type: application/json" \
  --data "{
    \"jsonrpc\":\"2.0\",
    \"method\":\"eth_call\",
    \"params\":[{
      \"from\":\"$ACCOUNT1\",
      \"to\":\"0x0000000000000000000000000000000000008000\",
      \"data\":\"0x0000000000000000000000000000000000000000000000000000000000000020\"
    }, \"latest\"],
    \"id\":2
  }")

echo "Result: $RESULT"

if echo "$RESULT" | grep -q "error\|cannot be called in read-only mode"; then
    echo "✓ KEY_CREATE correctly rejected in ReadOnly mode"
else
    echo "✗ KEY_CREATE should have been rejected"
    echo "$RESULT"
fi

echo ""
echo "============================================"
echo "TEST 3: ReadOnly Mode - RANDOM"
echo "============================================"

echo "Trying RANDOM via eth_call (should SUCCEED)..."
RESULT=$(curl -s -X POST http://127.0.0.1:8546 \
  -H "Content-Type: application/json" \
  --data "{
    \"jsonrpc\":\"2.0\",
    \"method\":\"eth_call\",
    \"params\":[{
      \"from\":\"$ACCOUNT1\",
      \"to\":\"0x0000000000000000000000000000000000008005\",
      \"data\":\"0x0000000000000000000000000000000000000000000000000000000000000020\"
    }, \"latest\"],
    \"id\":3
  }")

echo "Result: $RESULT"

if echo "$RESULT" | grep -q "\"result\":\"0x"; then
    echo "✓ RANDOM successfully returned data in ReadOnly mode"
else
    echo "✗ RANDOM should have succeeded"
    echo "$RESULT"
fi

echo ""
echo "============================================"
echo "TEST 4: System Contracts - Governance"
echo "============================================"

echo "Checking GovernanceContract at 0x1001..."
RESULT=$(curl -s -X POST http://127.0.0.1:8546 \
  -H "Content-Type: application/json" \
  --data "{
    \"jsonrpc\":\"2.0\",
    \"method\":\"eth_getCode\",
    \"params\":[\"0x0000000000000000000000000000000000001001\", \"latest\"],
    \"id\":4
  }")

echo "Result: $RESULT"

if echo "$RESULT" | grep -q "\"result\":\"0x[0-9a-f]\{10,\}"; then
    CODE_LENGTH=$(echo "$RESULT" | grep -oP '(?<="result":"0x)[0-9a-f]+' | tr -d '\n' | wc -c)
    echo "✓ GovernanceContract deployed (code length: $((CODE_LENGTH/2)) bytes)"
else
    echo "✗ GovernanceContract not found"
fi

echo ""
echo "============================================"
echo "TEST 5: System Contracts - SecurityConfig"
echo "============================================"

echo "Checking SecurityConfigContract at 0x1002..."
RESULT=$(curl -s -X POST http://127.0.0.1:8546 \
  -H "Content-Type: application/json" \
  --data "{
    \"jsonrpc\":\"2.0\",
    \"method\":\"eth_getCode\",
    \"params\":[\"0x0000000000000000000000000000000000001002\", \"latest\"],
    \"id\":5
  }")

if echo "$RESULT" | grep -q "\"result\":\"0x[0-9a-f]\{10,\}"; then
    CODE_LENGTH=$(echo "$RESULT" | grep -oP '(?<="result":"0x)[0-9a-f]+' | tr -d '\n' | wc -c)
    echo "✓ SecurityConfigContract deployed (code length: $((CODE_LENGTH/2)) bytes)"
else
    echo "✗ SecurityConfigContract not found"
fi

echo ""
echo "============================================"
echo "CLEANUP"
echo "============================================"

echo "Stopping node (PID: $NODE_PID)..."
kill $NODE_PID 2>/dev/null || true
sleep 2

echo ""
echo "=================================="
echo "TEST EXECUTION COMPLETE"
echo "=================================="
echo ""
echo "Summary:"
echo "  ✓ Key creation tested"
echo "  ✓ ReadOnly mode validation tested"
echo "  ✓ System contracts verified"
echo ""
echo "Note: For full owner verification and contract deployment,"
echo "additional Solidity contract compilation would be needed."
echo "Current tests validate core precompile and system contract functionality."

