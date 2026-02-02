#!/bin/bash
# Simple test script for crypto precompiles and governance
set -e

echo "Testing Crypto Precompiles and Governance Contract"
echo "==================================================="

# Assume node is running or start minimal test
RPC_URL="http://localhost:8545"

echo ""
echo "1. Testing RPC Connection..."
chain_id=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' \
    $RPC_URL 2>/dev/null | grep -o '"result":"0x[^"]*"' | cut -d'"' -f4 || echo "")

if [ -z "$chain_id" ]; then
    echo "⚠ Node not running. Please start geth first."
    echo ""
    echo "To start node with SGX consensus:"
    echo "  export GRAMINE_VERSION=test-v1.6"
    echo "  export GRAMINE_MANIFEST_PATH=$PWD/test/e2e/data/geth.manifest"
    echo "  bash test/e2e/tools/create_mock_attestation.sh"
    echo "  ./build/bin/geth --datadir test-e2e-node --http --http.addr 127.0.0.1 ..."
    exit 0
fi

echo "✓ Connected to Chain ID: $chain_id ($(printf "%d" $chain_id))"

echo ""
echo "2. Testing SGX Crypto Precompiled Contracts..."

# Test SGX_RANDOM (0x8005) - can be called via eth_call
echo "  Testing SGX_RANDOM (0x8005)..."
random_result=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x0000000000000000000000000000000000008005","data":"0x0000000000000000000000000000000000000000000000000000000000000020"},"latest"],"id":1}' \
    $RPC_URL 2>/dev/null)

echo "  Response: $random_result"

if echo "$random_result" | grep -q '"result":"0x'; then
    random_hex=$(echo "$random_result" | grep -o '"result":"0x[^"]*"' | cut -d'"' -f4)
    echo "  ✓ SGX_RANDOM returned: $random_hex"
    echo "    Length: ${#random_hex} characters"
else
    echo "  ⚠ SGX_RANDOM call failed or returned error"
fi

# Check all 9 precompiles exist
echo ""
echo "  Checking all 9 SGX precompiled contracts..."
for addr_hex in 8000 8001 8002 8003 8004 8005 8006 8007 8008; do
    addr_dec=$((16#$addr_hex))
    padded_addr=$(printf "0x%040x" $addr_dec)
    
    code_result=$(curl -s -X POST -H "Content-Type: application/json" \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$padded_addr\",\"latest\"],\"id\":1}" \
        $RPC_URL 2>/dev/null | grep -o '"result":"[^"]*"')
    
    if echo "$code_result" | grep -q "0x0"; then
        echo "    0x$addr_hex: Exists (native precompile)"
    else
        echo "    0x$addr_hex: $code_result"
    fi
done

echo ""
echo "3. Testing System Contracts..."

# Governance Contract (0x1001)
gov_addr="0x0000000000000000000000000000000000001001"
echo "  Checking Governance Contract ($gov_addr)..."
gov_code=$(curl -s -X POST -H "Content-Type: application/json" \
    --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$gov_addr\",\"latest\"],\"id\":1}" \
    $RPC_URL | grep -o '"result":"0x[^"]*"' | cut -d'"' -f4)

echo "    Code length: ${#gov_code} characters"
if [ "${#gov_code}" -gt 100 ]; then
    echo "    ✓ Governance contract deployed ($(( (${#gov_code} - 2) / 2 )) bytes)"
else
    echo "    ⚠ Governance contract may not be deployed"
fi

# Security Config Contract (0x1002)
sec_addr="0x0000000000000000000000000000000000001002"
echo "  Checking Security Config Contract ($sec_addr)..."
sec_code=$(curl -s -X POST -H "Content-Type: application/json" \
    --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$sec_addr\",\"latest\"],\"id\":1}" \
    $RPC_URL | grep -o '"result":"0x[^"]*"' | cut -d'"' -f4)

echo "    Code length: ${#sec_code} characters"
if [ "${#sec_code}" -gt 100 ]; then
    echo "    ✓ Security Config contract deployed ($(( (${#sec_code} - 2) / 2 )) bytes)"
else
    echo "    ⚠ Security Config contract may not be deployed"
fi

# Incentive Contract (0x1003)
inc_addr="0x0000000000000000000000000000000000001003"
echo "  Checking Incentive Contract ($inc_addr)..."
inc_code=$(curl -s -X POST -H "Content-Type: application/json" \
    --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$inc_addr\",\"latest\"],\"id\":1}" \
    $RPC_URL | grep -o '"result":"0x[^"]*"' | cut -d'"' -f4)

echo "    Code length: ${#inc_code} characters"
if [ "${#inc_code}" -gt 100 ]; then
    echo "    ✓ Incentive contract deployed ($(( (${#inc_code} - 2) / 2 )) bytes)"
else
    echo "    ⚠ Incentive contract may not be deployed"
fi

echo ""
echo "==================================================="
echo "Test Summary"
echo "==================================================="
echo "✓ RPC connectivity verified"
echo "✓ Crypto precompiles checked (9 contracts)"
echo "✓ System contracts checked (3 contracts)"
echo ""
echo "Note: Some tests require actual transaction execution"
echo "      which needs mining and gas. Current tests are"
echo "      read-only operations via eth_call and eth_getCode."
